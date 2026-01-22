package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		usr, err := user.Current()
		if err != nil {
			return path
		}
		return filepath.Join(usr.HomeDir, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func InitDB(dbPath string) (*sql.DB, error) {
	expandedPath := expandTilde(dbPath)

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", expandedPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := CreateSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}

func CreateSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS commands (
			source TEXT NOT NULL,
			timestamp REAL NOT NULL,
			command TEXT NOT NULL,
			duration INTEGER,
			cwd TEXT,
			exit_code INTEGER,
			PRIMARY KEY (source, timestamp)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_timestamp ON commands(timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_source ON commands(source);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS commands_fts USING fts5(
			command,
			content='commands',
			content_rowid='rowid'
		);`,
		// Triggers to keep FTS index in sync automatically
		`CREATE TRIGGER IF NOT EXISTS commands_ai AFTER INSERT ON commands BEGIN
			INSERT INTO commands_fts(rowid, command) VALUES (new.rowid, new.command);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS commands_ad AFTER DELETE ON commands BEGIN
			INSERT INTO commands_fts(commands_fts, rowid, command) VALUES ('delete', old.rowid, old.command);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS commands_au AFTER UPDATE ON commands BEGIN
			INSERT INTO commands_fts(commands_fts, rowid, command) VALUES ('delete', old.rowid, old.command);
			INSERT INTO commands_fts(rowid, command) VALUES (new.rowid, new.command);
		END;`,
		// Wizard cache table for natural language → command mappings
		`CREATE TABLE IF NOT EXISTS wizard_cache (
			query_normalized TEXT PRIMARY KEY,
			query_original TEXT NOT NULL,
			command TEXT NOT NULL,
			run_count INTEGER DEFAULT 1,
			last_used REAL NOT NULL,
			created_at REAL NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_wizard_last_used ON wizard_cache(last_used DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_wizard_run_count ON wizard_cache(run_count DESC);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query '%s': %w", query, err)
		}
	}

	return nil
}

func InsertCommands(db *sql.DB, commands []Command) (int, int, error) {
	if len(commands) == 0 {
		return 0, 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// FTS index is updated automatically via triggers
	insertSQL := `INSERT OR IGNORE INTO commands (source, timestamp, command, duration, cwd, exit_code)
	              VALUES (?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0

	for _, cmd := range commands {
		result, err := stmt.Exec(cmd.Source, cmd.Timestamp, cmd.Command, cmd.Duration, cmd.CWD, cmd.ExitCode)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert command: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected > 0 {
			inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, len(commands) - inserted, nil
}

func InsertCommandsBatch(db *sql.DB, commands []Command, batchSize int) (int, int, error) {
	if len(commands) == 0 {
		return 0, 0, nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	totalInserted := 0
	totalIgnored := 0

	for i := 0; i < len(commands); i += batchSize {
		end := i + batchSize
		if end > len(commands) {
			end = len(commands)
		}

		batch := commands[i:end]
		inserted, ignored, err := InsertCommands(db, batch)
		if err != nil {
			return totalInserted, totalIgnored, fmt.Errorf("failed to insert batch %d-%d: %w", i, end-1, err)
		}

		totalInserted += inserted
		totalIgnored += ignored
	}

	return totalInserted, totalIgnored, nil
}

func GetDBStats(db *sql.DB) (map[string]int64, error) {
	stats := make(map[string]int64)

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM commands").Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to count commands: %w", err)
	}
	stats["total_commands"] = count

	if err := db.QueryRow("SELECT COUNT(DISTINCT source) FROM commands").Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to count sources: %w", err)
	}
	stats["total_sources"] = count

	rows, err := db.Query("SELECT source, COUNT(*) as count FROM commands GROUP BY source ORDER BY count DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query source stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		var sourceCount int64
		if err := rows.Scan(&source, &sourceCount); err != nil {
			continue
		}
		stats["source_"+source] = sourceCount
	}

	return stats, nil
}

type SearchResult struct {
	Command   string
	Source    string
	Timestamp float64
}

type SearchOptions struct {
	Query string
	Limit int
	Since float64 // Unix timestamp, 0 means no filter
	Until float64 // Unix timestamp, 0 means no filter
}

func SearchCommands(db *sql.DB, opts SearchOptions) ([]SearchResult, error) {
	var results []SearchResult

	if opts.Limit <= 0 {
		opts.Limit = 500
	}

	var queryBuilder strings.Builder
	var args []interface{}

	queryBuilder.WriteString("SELECT command, source, timestamp FROM commands WHERE 1=1")

	// FTS filter
	if opts.Query != "" {
		ftsQuery := buildFTSQuery(opts.Query)
		queryBuilder.WriteString(" AND rowid IN (SELECT rowid FROM commands_fts WHERE commands_fts MATCH ?)")
		args = append(args, ftsQuery)
	}

	// Time range filters
	if opts.Since > 0 {
		queryBuilder.WriteString(" AND timestamp >= ?")
		args = append(args, opts.Since)
	}
	if opts.Until > 0 {
		queryBuilder.WriteString(" AND timestamp <= ?")
		args = append(args, opts.Until)
	}

	queryBuilder.WriteString(" ORDER BY timestamp DESC LIMIT ?")
	args = append(args, opts.Limit)

	rows, err := db.Query(queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search commands: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Command, &result.Source, &result.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

func buildFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	parts := strings.Fields(query)
	for i, part := range parts {
		parts[i] = escapeFTS(part) + "*"
	}
	return strings.Join(parts, " ")
}

func escapeFTS(s string) string {
	s = strings.ReplaceAll(s, "\"", "\"\"")
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, ":", "")
	return s
}

// FrequentCommand represents a command and its usage count
type FrequentCommand struct {
	Command string
	Count   int
}

// SearchByPrefix returns commands starting with the given prefix (for history fallback)
func SearchByPrefix(db *sql.DB, prefix string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `SELECT command, source, timestamp FROM commands
		WHERE command LIKE ? || '%'
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := db.Query(query, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search by prefix: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Command, &result.Source, &result.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// GetFrequentCommands returns the most frequently used commands matching a pattern
func GetFrequentCommands(db *sql.DB, pattern string, limit int) ([]FrequentCommand, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	var args []interface{}

	if pattern != "" {
		query = `SELECT command, COUNT(*) as count FROM commands
			WHERE command LIKE '%' || ? || '%'
			GROUP BY command
			ORDER BY count DESC
			LIMIT ?`
		args = []interface{}{pattern, limit}
	} else {
		query = `SELECT command, COUNT(*) as count FROM commands
			GROUP BY command
			ORDER BY count DESC
			LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get frequent commands: %w", err)
	}
	defer rows.Close()

	var results []FrequentCommand
	for rows.Next() {
		var result FrequentCommand
		if err := rows.Scan(&result.Command, &result.Count); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// GetRecentCommands returns the last N commands globally
func GetRecentCommands(db *sql.DB, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT command, source, timestamp FROM commands
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent commands: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Command, &result.Source, &result.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// WizardCacheEntry represents a cached query→command mapping
type WizardCacheEntry struct {
	QueryNormalized string
	QueryOriginal   string
	Command         string
	RunCount        int
	LastUsed        float64
	CreatedAt       float64
}

// NormalizeQuery normalizes a query for cache lookup (lowercase, trim whitespace)
func NormalizeQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

// GetWizardCache looks up a cached command for the given query
func GetWizardCache(db *sql.DB, query string) (*WizardCacheEntry, error) {
	normalized := NormalizeQuery(query)

	row := db.QueryRow(`SELECT query_normalized, query_original, command, run_count, last_used, created_at
		FROM wizard_cache WHERE query_normalized = ?`, normalized)

	var entry WizardCacheEntry
	err := row.Scan(&entry.QueryNormalized, &entry.QueryOriginal, &entry.Command,
		&entry.RunCount, &entry.LastUsed, &entry.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wizard cache: %w", err)
	}

	return &entry, nil
}

// SetWizardCache stores or updates a query→command mapping
func SetWizardCache(db *sql.DB, query, command string) error {
	normalized := NormalizeQuery(query)
	now := float64(time.Now().Unix())

	_, err := db.Exec(`INSERT INTO wizard_cache (query_normalized, query_original, command, run_count, last_used, created_at)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(query_normalized) DO UPDATE SET
			command = excluded.command,
			run_count = run_count + 1,
			last_used = excluded.last_used`,
		normalized, query, command, now, now)

	if err != nil {
		return fmt.Errorf("failed to set wizard cache: %w", err)
	}

	return nil
}

// ListWizardCache returns all cached mappings, ordered by most recently used
func ListWizardCache(db *sql.DB, limit int) ([]WizardCacheEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`SELECT query_normalized, query_original, command, run_count, last_used, created_at
		FROM wizard_cache ORDER BY last_used DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list wizard cache: %w", err)
	}
	defer rows.Close()

	var entries []WizardCacheEntry
	for rows.Next() {
		var entry WizardCacheEntry
		if err := rows.Scan(&entry.QueryNormalized, &entry.QueryOriginal, &entry.Command,
			&entry.RunCount, &entry.LastUsed, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan wizard cache entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// ClearWizardCache removes all cached mappings
func ClearWizardCache(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM wizard_cache`)
	if err != nil {
		return fmt.Errorf("failed to clear wizard cache: %w", err)
	}
	return nil
}

// DeleteWizardCacheEntry removes a specific cached mapping
func DeleteWizardCacheEntry(db *sql.DB, query string) error {
	normalized := NormalizeQuery(query)
	_, err := db.Exec(`DELETE FROM wizard_cache WHERE query_normalized = ?`, normalized)
	if err != nil {
		return fmt.Errorf("failed to delete wizard cache entry: %w", err)
	}
	return nil
}

// SearchHistoryByKeywords searches history for commands containing the given keywords
// Uses AND for multiple keywords to get more relevant results
func SearchHistoryByKeywords(db *sql.DB, keywords []string, limit int) ([]SearchResult, error) {
	if len(keywords) == 0 || limit <= 0 {
		return nil, nil
	}

	// Filter to meaningful keywords (longer than 2 chars)
	var filtered []string
	for _, kw := range keywords {
		if kw = strings.TrimSpace(kw); len(kw) > 2 {
			filtered = append(filtered, kw)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Build LIKE conditions - use AND for better relevance
	var conditions []string
	var args []interface{}
	for _, kw := range filtered {
		conditions = append(conditions, "command LIKE ?")
		args = append(args, "%"+kw+"%")
	}

	// Try AND first (more specific), fall back to OR if no results
	query := fmt.Sprintf(`SELECT command, source, timestamp FROM commands
		WHERE %s
		GROUP BY command
		ORDER BY COUNT(*) DESC, timestamp DESC
		LIMIT ?`, strings.Join(conditions, " AND "))
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search history by keywords: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Command, &result.Source, &result.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	// If AND returned no results and we have multiple keywords, try OR
	if len(results) == 0 && len(filtered) > 1 {
		args = args[:0]
		for _, kw := range filtered {
			args = append(args, "%"+kw+"%")
		}
		args = append(args, limit)

		query = fmt.Sprintf(`SELECT command, source, timestamp FROM commands
			WHERE %s
			GROUP BY command
			ORDER BY COUNT(*) DESC, timestamp DESC
			LIMIT ?`, strings.Join(conditions, " OR "))

		rows2, err := db.Query(query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to search history by keywords: %w", err)
		}
		defer rows2.Close()

		for rows2.Next() {
			var result SearchResult
			if err := rows2.Scan(&result.Command, &result.Source, &result.Timestamp); err != nil {
				return nil, fmt.Errorf("failed to scan result: %w", err)
			}
			results = append(results, result)
		}
	}

	return results, nil
}
