# AGENTS.md - Guidelines for AI Assistants Working on zist

This document provides guidelines for AI coding assistants working on the zist repository.

## Project Overview

zist is a local ZSH history aggregation tool written in Go. It collects commands from multiple ZSH history files, stores them in a SQLite database with FTS5 full-text search, and provides instant fuzzy search via fzf.

## Build Commands

All build tasks are defined in Taskfile.yml. Use `task` to run commands:

### Primary Commands
```bash
task build          # Build zist binary (requires fts5 build tag)
task test           # Run full test suite
task check          # Run fmt check, vet, and tests
task run -- <args>  # Build and run with arguments
```

### Development Commands
```bash
task fmt            # Format code with gofmt
task vet            # Run go vet
task clean          # Remove build artifacts
task ci             # CI pipeline (same as check)
```

### Running Single Tests
```bash
# Run all tests
go test -v -tags fts5 ./...

# Run specific test file
go test -v -tags fts5 ./... -run TestFunctionName

# Run tests matching pattern
go test -v -tags fts5 -run "TestParse" ./...

# Run tests with timeout
go test -v -tags fts5 -timeout 30s ./...
```

### Database Tasks
```bash
task db-shell       # Open SQLite shell with database
task db-backup      # Backup database to current directory
task db-reset       # Delete and reset database
```

### Release Build
```bash
task release        # Build for linux-x64, linux-arm64, macos-intel, macos-arm, windows
```

### Dependency Management
```bash
task deps           # Download dependencies
task deps-update    # Update all dependencies
task deps-verify    # Verify dependencies
```

## Code Style Guidelines

### General Principles
- Follow standard Go conventions (Effective Go, Go Code Review Comments)
- No comments in code unless explicitly required by the task
- Keep functions short and focused
- Use early returns to reduce nesting

### Formatting
- Run `task fmt` before committing or submitting PRs
- Use gofmt with default settings (no custom style)
- Ensure code passes `task fmt-check`

### Naming Conventions
- **Files**: snake_case.go (e.g., `history.go`, `database_test.go`)
- **Packages**: single word, lowercase (e.g., `package main`)
- **Types**: PascalCase (e.g., `Command`, `History`, `SearchResult`)
- **Functions**: PascalCase (e.g., `ParseHistoryFile`, `InsertCommands`)
- **Variables**: camelCase (e.g., `dbPath`, `insertedCount`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE for magic values
- **Interfaces**: PascalCase, often -er suffix (e.g., `Reader`)

### Imports
- Standard library imports first, then third-party
- Group imports with blank line between groups:
  ```go
  import (
      "context"
      "fmt"
      "os"

      _ "github.com/mattn/go-sqlite3"
      "github.com/peterbourgon/ff/v4"
      "github.com/peterbourgon/ff/v4/ffhelp"
  )
  ```
- Use blank import (`_`) for side-effects only (e.g., sqlite3 driver)

### Error Handling
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors early, avoid else blocks after error checks
- Use sentinel errors only when appropriate
- Handle all errors explicitly; avoid `_` discard

### Function Signatures
- Context (ctx context.Context) as first parameter for operations
- Database path as string parameter, expand tilde in function
- Multiple return values: (result, error) or (result1, result2, error)

### Types and Structs
- Use structs for data containers with field comments:
  ```go
  type Command struct {
      Source    string  // Absolute file path
      Timestamp float64 // Unix timestamp with subsecond precision
      Command   string  // The command text
      Duration  int     // Execution duration in seconds
      CWD       string  // Working directory (optional)
      ExitCode  int     // Exit code (optional)
  }
  ```
- Use maps for key-value lookups
- Prefer slices over arrays

### Testing
- Use table-driven tests with test structs
- Test file naming: `*_test.go` alongside implementation
- Subtests with `t.Run()` for grouped assertions
- Use `t.TempDir()` for temporary test directories
- Tests must include FTS5 build tag: `-tags fts5`
- Example test pattern:
  ```go
  func TestFunction(t *testing.T) {
      tests := []struct {
          name string
          input Type
          want  Type
      }{
          {"name", input, want},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              got := Function(tt.input)
              if got != tt.want {
                  t.Errorf("Function(%v) = %v, want %v", tt.input, got, tt.want)
              }
          })
      }
  }
  ```

### Database Operations
- Use prepared statements for all queries
- Close resources with defer (database connections, rows, statements)
- Transactions for batch operations
- SQLite FTS5 for full-text search
- Foreign keys enabled: `?_foreign_keys=on`

### Concurrency
- Pass context to cancel long-running operations
- Use goroutines for non-blocking I/O
- Close pipes and stdin properly in goroutines

### Third-Party Libraries
- **peterbourgon/ff/v4**: CLI flag parsing with subcommands
- **mattn/go-sqlite3**: SQLite driver (requires CGO, C compiler)
- Build tags required: `-tags fts5` for SQLite FTS5 support

## Project Structure

```
zist/
├── main.go           # CLI entry point, command handlers
├── database.go       # Database operations, schema, queries
├── history.go        # ZSH history file parsing
├── *_test.go         # Test files
├── Taskfile.yml      # Build automation
├── go.mod            # Go module definition
└── README.md         # User documentation
```

## Common Patterns

### CLI Command Pattern
```go
func runCommand(ctx context.Context, flags *FlagSet, args []string) error {
    db, err := InitDB(*dbPath)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()
    // ... operation
    return nil
}
```

### Path Expansion
```go
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
```

## Important Notes

- SQLite requires CGO for go-sqlite3 driver
- Always use `-tags fts5` when building or testing
- The binary is output to `bin/` directory
- Default database location: `~/.zist/zist.db`
- fzf must be installed for search functionality
- Tests use `t.TempDir()` for isolation