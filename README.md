# zist

Local ZSH history aggregation tool. Collect commands from multiple ZSH history files, store them in a local SQLite database, and search instantly with fuzzy matching.

## Why zist?

- **Multiple machines**: Collect history from all your machines into one database
- **Shared access**: Sync with rsync, git, or any file sharing mechanism
- **Instant search**: Query 10,000+ commands in milliseconds with SQLite FTS5
- **Ctrl+X for fuzzy search**: Interactive fuzzy search with fzf
- **Automatic deduplication**: `(source, timestamp)` primary key prevents duplicates

## Features

- **Collect** from files or directories
- **Search** with full-text search and fuzzy matching
- **Interactive** ZSH integration (Ctrl+X)
- **Batch inserts** with transactions
- **Metadata storage**: duration, cwd, exit code
- **Subsecond timestamps** for duplicate deduplication

## Requirements

- Go 1.25+
- SQLite with FTS5 support
- fzf (for search functionality)

## Installation

### From source

```bash
git clone git@github.com:tchaudhry91/zist.git
cd zist
task build
task install-user
```

### Dependencies

```bash
# fzf (required for search)
brew install fzf     # macOS
sudo apt install fzf  # Ubuntu/Debian
sudo dnf install fzf  # Fedora

# Add to PATH
export PATH="$HOME/go/bin:$PATH"
```

## Quick Start

```bash
# Collect history
zist collect ~/.zsh_history

# Collect multiple files
zist collect ~/.zsh_history ~/.claude/claude_zsh_history

# Collect all *zsh_history files from directory
zist collect ~/synced_hists/

# Search commands (requires fzf)
zist search docker

# Interactive search (type before Ctrl+X)
docker<Ctrl+X>  # opens fzf with "docker" as query
```

## Commands

### collect

Collect commands from ZSH history files.

```bash
zist collect [--db PATH] HISTORY_FILE... | DIRECTORY...
```

- **HISTORY_FILE**: ZSH history file to parse
- **DIRECTORY**: Find all `*zsh_history` files in directory
- **--db**: Database path (default: `~/.zist/zist.db`)

### search

Search command history interactively with fzf.

```bash
zist search [--db PATH] [QUERY]
```

- **QUERY**: Initial search query for fzf (optional)
- **--db**: Database path (default: `~/.zist/zist.db`)

Note: fzf displays commands with their source file (e.g., `git status|||~/.zsh_history`).
Only the command is returned on selection.

## ZSH Integration

Install Ctrl+X binding:

```bash
zist install
source ~/.zshrc
```

Now press **Ctrl+X** to search across all aggregated history with fuzzy matching.

**What it does:**
- Uses `$LBUFFER` (what you typed before Ctrl+X) as initial query
- Opens fzf with all commands from database
- Places selected command in buffer for editing
- precmd hook saves executed command to history

**Uninstall:**

```bash
zist uninstall
```

## Database Schema

```sql
CREATE TABLE commands (
    source      TEXT NOT NULL,   -- absolute file path
    timestamp   REAL NOT NULL,   -- Unix timestamp with subsecond
    command     TEXT NOT NULL,   -- command text
    duration    INTEGER,         -- execution duration in seconds
    cwd         TEXT,            -- working directory
    exit_code   INTEGER,         -- command exit code
    PRIMARY KEY (source, timestamp)
);

CREATE INDEX idx_timestamp ON commands(timestamp DESC);
CREATE INDEX idx_source ON commands(source);

CREATE VIRTUAL TABLE commands_fts USING fts5(
    command,
    content='commands',
    content_rowid='rowid'
);
```

## Development

### Build

```bash
task build        # Build binary
task check        # Run fmt, vet, test
task test         # Run tests
```

### Database

```bash
task db-shell       # Open SQLite shell
task db-backup      # Backup database
task db-reset       # Delete database
```

### Release

```bash
task release    # Build for: linux-x64, linux-arm64, macos-intel, macos-arm, windows
```

## License

MIT

## Author

Tanmay Chaudhry
