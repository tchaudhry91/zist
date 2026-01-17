package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
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

	db, err := sql.Open("sqlite3", expandedPath+"?_foreign_keys=on")
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

	insertSQL := `INSERT OR IGNORE INTO commands (source, timestamp, command, duration, cwd, exit_code)
	              VALUES (?, ?, ?, ?, ?, ?)`
	insertFTSSQL := `INSERT OR IGNORE INTO commands_fts (rowid, command)
	                SELECT rowid, command FROM commands WHERE rowid = ?`

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	ftsStmt, err := tx.Prepare(insertFTSSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to prepare FTS insert statement: %w", err)
	}
	defer ftsStmt.Close()

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
			rowID, _ := result.LastInsertId()
			if _, err := ftsStmt.Exec(rowID); err != nil {
				return 0, 0, fmt.Errorf("failed to insert into FTS: %w", err)
			}
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
	Command string
	Source  string
}

func SearchCommands(db *sql.DB, query string) ([]SearchResult, error) {
	var results []SearchResult

	var rows *sql.Rows
	var err error

	if query == "" {
		rows, err = db.Query(`
			SELECT command, source FROM commands
			ORDER BY timestamp DESC
			LIMIT 100
		`)
	} else {
		ftsQuery := buildFTSQuery(query)
		rows, err = db.Query(`
			SELECT command, source FROM commands
			WHERE rowid IN (SELECT rowid FROM commands_fts WHERE commands_fts MATCH ?)
			ORDER BY timestamp DESC
			LIMIT 100
		`, ftsQuery)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search commands: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Command, &result.Source); err != nil {
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
