# zist

Local ZSH history aggregation tool. Collect commands from multiple ZSH history files, store them in a local SQLite database, and search instantly with fuzzy matching.

## Why zist?

- **Multiple machines**: Collect history from all your machines into one database
- **Shared access**: Sync with rsync, git, or any file sharing mechanism
- **Instant search**: Query 10,000+ commands in milliseconds with SQLite FTS5
- **Ctrl+X for fuzzy search**: Interactive fuzzy search with fzf and preview pane
- **Automatic deduplication**: `(source, timestamp)` primary key prevents duplicates

## Features

- **Collect** from files or directories (recursive search)
- **Search** with full-text search, fuzzy matching, and time filtering
- **Preview pane** shows source file and timestamp while browsing
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
# Set up history sync directory
mkdir -p ~/.histories/$(hostname)
mv ~/.zsh_history ~/.histories/$(hostname)/
ln -s ~/.histories/$(hostname)/.zsh_history ~/.zsh_history

# Collect from default location (~/.histories)
zist collect

# Or collect from specific files/directories
zist collect ~/.zsh_history ~/other_histories/

# Search commands (requires fzf) - shows preview pane with source/timestamp
zist search docker

# Search with time filter
zist search --since 2024-01-01 git

# Interactive search (type before Ctrl+X)
docker<Ctrl+X>  # opens fzf with "docker" as query

# Check version
zist --version
```

## Commands

### collect

Collect commands from ZSH history files.

```bash
zist collect [--db PATH] [--quiet] [PATH...]
```

- **PATH**: History file or directory to search (default: `~/.histories`)
- **--db**: Database path (default: `~/.zist/zist.db`)
- **--quiet**: Suppress output (useful for scripts/automation)

Directories are searched recursively for `*zsh_history` files.

### search

Search command history interactively with fzf.

```bash
zist search [--db PATH] [--limit N] [--since DATE] [--until DATE] [QUERY]
```

- **QUERY**: Initial search query for fzf (optional)
- **--db**: Database path (default: `~/.zist/zist.db`)
- **--limit**: Maximum number of results (default: 500)
- **--since**: Only show commands after this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)
- **--until**: Only show commands before this date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)

The search displays a **preview pane** showing the source file and timestamp for the highlighted command.

## ZSH Integration

Install Ctrl+X binding:

```bash
zist install
source ~/.zshrc
```

Now press **Ctrl+X** to search across all aggregated history with fuzzy matching.

**What it does:**
- Uses `$LBUFFER` (what you typed before Ctrl+X) as initial query
- Opens fzf with all commands from database (with preview pane)
- Places selected command in buffer for editing
- precmd hook automatically collects from `~/.histories` after each command

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

-- Full-text search index
CREATE VIRTUAL TABLE commands_fts USING fts5(
    command,
    content='commands',
    content_rowid='rowid'
);

-- Triggers keep FTS index in sync automatically
CREATE TRIGGER commands_ai AFTER INSERT ON commands ...
CREATE TRIGGER commands_ad AFTER DELETE ON commands ...
CREATE TRIGGER commands_au AFTER UPDATE ON commands ...
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
