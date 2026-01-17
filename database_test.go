package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("InitDB() returned nil db")
	}
}

func TestInsertCommands(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	defer os.Remove(dbPath)

	commands := []Command{
		{Source: "/file1", Timestamp: 1000.0, Command: "ls", Duration: 0},
		{Source: "/file1", Timestamp: 1000.001, Command: "pwd", Duration: 0},
		{Source: "/file2", Timestamp: 2000.0, Command: "git status", Duration: 1},
	}

	inserted, ignored, err := InsertCommands(db, commands)
	if err != nil {
		t.Fatalf("InsertCommands() error = %v", err)
	}

	if inserted != 3 {
		t.Errorf("InsertCommands() inserted = %d, want 3", inserted)
	}

	if ignored != 0 {
		t.Errorf("InsertCommands() ignored = %d, want 0", ignored)
	}

	inserted2, ignored2, err := InsertCommands(db, commands)
	if err != nil {
		t.Fatalf("InsertCommands() second call error = %v", err)
	}

	if inserted2 != 0 {
		t.Errorf("InsertCommands() second call inserted = %d, want 0 (duplicate)", inserted2)
	}

	if ignored2 != 3 {
		t.Errorf("InsertCommands() second call ignored = %d, want 3", ignored2)
	}
}

func TestInsertCommandsBatch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()
	defer os.Remove(dbPath)

	commands := make([]Command, 0, 25)
	for i := 0; i < 25; i++ {
		commands = append(commands, Command{
			Source:    "/file",
			Timestamp: float64(1000 + int64(i)*10),
			Command:   "test command",
			Duration:  0,
		})
	}

	inserted, ignored, err := InsertCommandsBatch(db, commands, 10)
	if err != nil {
		t.Fatalf("InsertCommandsBatch() error = %v", err)
	}

	if inserted != 25 {
		t.Errorf("InsertCommandsBatch() inserted = %d, want 25", inserted)
	}

	if ignored != 0 {
		t.Errorf("InsertCommandsBatch() ignored = %d, want 0", ignored)
	}
}

func TestGetDBStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()

	commands := []Command{
		{Source: "/file1", Timestamp: 1000.0, Command: "cmd1"},
		{Source: "/file1", Timestamp: 1001.0, Command: "cmd2"},
		{Source: "/file2", Timestamp: 2000.0, Command: "cmd3"},
	}

	_, _, err = InsertCommands(db, commands)
	if err != nil {
		t.Fatalf("InsertCommands() error = %v", err)
	}

	stats, err := GetDBStats(db)
	if err != nil {
		t.Fatalf("GetDBStats() error = %v", err)
	}

	if stats["total_commands"] != 3 {
		t.Errorf("GetDBStats() total_commands = %d, want 3", stats["total_commands"])
	}

	if stats["total_sources"] != 2 {
		t.Errorf("GetDBStats() total_sources = %d, want 2", stats["total_sources"])
	}
}

func TestSearchCommands(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer db.Close()

	commands := []Command{
		{Source: "/file1", Timestamp: 1000.0, Command: "ls -la"},
		{Source: "/file1", Timestamp: 1001.0, Command: "git status"},
		{Source: "/file1", Timestamp: 1002.0, Command: "git commit"},
		{Source: "/file2", Timestamp: 2000.0, Command: "echo hello"},
	}

	_, _, err = InsertCommands(db, commands)
	if err != nil {
		t.Fatalf("InsertCommands() error = %v", err)
	}

	t.Run("all commands", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 4 {
			t.Errorf("SearchCommands() with empty query returned %d results, want 4", len(results))
		}

		if results[0].Command != "echo hello" {
			t.Errorf("SearchCommands()[0].Command = %s, want 'echo hello' (most recent)", results[0].Command)
		}

		if results[0].Source != "/file2" {
			t.Errorf("SearchCommands()[0].Source = %s, want '/file2'", results[0].Source)
		}
	})

	t.Run("fts search", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Query: "git"})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("SearchCommands('git') returned %d results, want 2", len(results))
		}

		foundGitStatus := false
		foundGitCommit := false
		for _, r := range results {
			if r.Command == "git status" && r.Source == "/file1" {
				foundGitStatus = true
			}
			if r.Command == "git commit" && r.Source == "/file1" {
				foundGitCommit = true
			}
		}

		if !foundGitStatus {
			t.Errorf("SearchCommands('git') did not find 'git status' from /file1")
		}
		if !foundGitCommit {
			t.Errorf("SearchCommands('git') did not find 'git commit' from /file1")
		}
	})

	t.Run("no results", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Query: "nonexistent"})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 0 {
			t.Errorf("SearchCommands('nonexistent') returned %d results, want 0", len(results))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Limit: 2})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("SearchCommands() with limit=2 returned %d results, want 2", len(results))
		}
	})

	t.Run("with since filter", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Since: 1500.0})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 1 {
			t.Errorf("SearchCommands() with since=1500 returned %d results, want 1", len(results))
		}

		if len(results) > 0 && results[0].Command != "echo hello" {
			t.Errorf("SearchCommands() with since filter returned wrong command: %s", results[0].Command)
		}
	})

	t.Run("with until filter", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Until: 1001.5})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("SearchCommands() with until=1001.5 returned %d results, want 2", len(results))
		}
	})

	t.Run("with since and until", func(t *testing.T) {
		results, err := SearchCommands(db, SearchOptions{Since: 1000.5, Until: 1002.5})
		if err != nil {
			t.Fatalf("SearchCommands() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("SearchCommands() with time range returned %d results, want 2", len(results))
		}
	})
}

func TestExpandTilde(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"tilde path", "~/test.db"},
		{"absolute path", "/tmp/test.db"},
		{"relative path", "test.db"},
		{"tilde only", "~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)
			if len(result) == 0 {
				t.Errorf("expandTilde(%q) returned empty", tt.input)
			}
			if tt.name == "tilde path" && result == tt.input {
				t.Errorf("expandTilde(%q) should expand tilde, got %q", tt.input, result)
			}
		})
	}
}
