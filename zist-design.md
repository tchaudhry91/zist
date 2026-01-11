# zist - ZSH History Aggregation

A local tool for aggregating and searching command history from multiple ZSH history files on a single machine.

## Overview

`zist` reads commands from multiple ZSH history files, aggregates them into a local SQLite database, and provides fast search. Cross-machine sync is the user's responsibility (rsync, MinIO, git, etc.).

**Key Features:**
- Aggregate multiple history files (zsh, claude, opencode, etc.)
- Local SQLite database (works offline, always fast)
- Interactive search (fzf-style TUI)
- Conversational search via LLM (RAG approach)
- Static binary distribution
- Sync strategy is entirely user-defined

## Architecture

### Local Aggregation Model

```
Machine: QE93
~/.zsh_history              → source: /home/tchaudhry/.zsh_history
~/.claude/claude_zsh_history → source: /home/tchaudhry/.claude/claude_zsh_history
~/.opencode_zsh_history     → source: /home/tchaudhry/.opencode_zsh_history

All aggregated into: ~/.zist/zist.db
```

**How it works:**
1. Each machine runs `zist collect` after every command (via ZSH hook)
2. Commands from all configured history files inserted into local SQLite
3. User handles cross-machine sync however they prefer
4. Search/LLM queries run against local database (always fast, works offline)

**Why no built-in sync?**
- Different users have different sync preferences (rsync, MinIO, git, Syncthing, etc.)
- Filesystem already guarantees unique file paths
- User is free to organize their history files however they want
- Simpler codebase, fewer edge cases, fewer failure modes

## Components

### 1. CLI Tool (Zig)

**Binary:** `zist`
**Configuration:** `~/.config/zist/config.ini`
**Database:** `~/.zist/zist.db` (SQLite)

**Commands:**

```bash
# Interactive search (fzf-style)
zist search [query]

# Conversational search (RAG)
zist ask "How did I fix that nginx issue?"

# Collection (called by ZSH hook)
zist collect

# Installation
zist install    # Sets up ZSH integration
zist uninstall
```

### 2. Database Schema (SQLite)

```sql
CREATE TABLE commands (
    source TEXT NOT NULL,           -- absolute filepath (e.g., "/home/user/.zsh_history")
    timestamp REAL NOT NULL,        -- unix timestamp with subsecond precision
    command TEXT NOT NULL,          -- the command executed
    duration INTEGER,               -- execution duration in seconds
    cwd TEXT,                       -- working directory
    exit_code INTEGER,              -- command exit code

    PRIMARY KEY (source, timestamp)  -- unique per source+timestamp, handles dedup
);

CREATE INDEX idx_timestamp ON commands(timestamp DESC);
CREATE INDEX idx_source ON commands(source);

-- Full-text search on command text
CREATE VIRTUAL TABLE commands_fts USING fts5(
    command,
    content='commands',
    content_rowid='rowid'
);
```

**Key Design Decisions:**
- `(source, timestamp)` as primary key enables automatic deduplication via `INSERT OR IGNORE`
- Timestamp stored as REAL for subsecond precision (see "Subsecond Timestamp Handling" below)
- Source is the absolute file path - filesystem guarantees uniqueness per machine
- No sync_state table - user handles cross-machine sync outside zist

### 3. Configuration

**~/.config/zist/config.ini:**

```ini
[collection]
# Paths to ZSH history files (supports ~ expansion)
# Comma-separated list; all files will be collected from
history_files = ~/.zsh_history, ~/.claude/claude_zsh_history, ~/.opencode_zsh_history

[llm]
# LLM API endpoint for conversational search (zist ask)
endpoint = http://localhost:11434

# API key (if required by your LLM provider)
api_key =

# Model to use
# Examples: llama2, codellama, mistral
model = llama2
```

**Configuration Notes:**
- File location: `~/.config/zist/config.ini`
- Created by `zist install` with defaults
- INI format chosen for simplicity (no external parser needed)
- Only `history_files` in `[collection]` section is required

## User Flows

### Collection Flow

```
1. User executes command in shell
2. Command completes
3. precmd() hook triggers
4. zist collect runs in background (&)
5. zist reads configured history files
6. zist parses commands (timestamp, duration, command text)
7. zist inserts into local SQLite database
8. Returns immediately (non-blocking)
```

**ZSH Integration:**

```bash
# Add to .zshrc (via `zist install`)
precmd() {
    zist collect &
}

# Interactive search keybinding
bindkey '^R' zist-search-widget
```

### Search Flow (Interactive)

```
1. User presses Ctrl+R (or runs 'zist search')
2. zist launches fzf-style TUI
3. User types search query: "docker"
4. As user types:
   - Query local SQLite with FTS5
   - Display results in real-time
5. User navigates with arrow keys
6. User presses Enter to select
7. Selected command is inserted into shell prompt
```

**Search UI:**

```
  3/120 matches
> docker█
  █ docker build -t myapp .          [~/.zsh_history] 2026-01-03 14:23
  █ docker ps -a                      [~/.claude/claude_zsh_history] 2026-01-02 10:15
  █ docker compose up -d              [~/.zsh_history] 2026-01-01 09:00
```

### Conversational Search Flow (RAG)

**RAG = Retrieval Augmented Generation**

```bash
$ zist ask "How did I fix that nginx issue last week?"
```

**Flow:**

```
1. Extract keywords from question: "nginx", "last week"
2. Query SQLite for relevant commands:
   SELECT * FROM commands
   WHERE command LIKE '%nginx%'
     AND timestamp > (now() - 7 days)
   ORDER BY timestamp
3. Build LLM prompt:
   "User asks: How did I fix that nginx issue?

    Relevant commands from history:
    [2025-12-28 14:23] docker logs nginx-container
    [2025-12-28 14:24] docker exec -it nginx-container bash
    [2025-12-28 14:25] vim /etc/nginx/nginx.conf
    [2025-12-28 14:30] systemctl restart nginx

    Explain what the user did."
4. Send to local Ollama
5. Stream LLM response back to user
```

**LLM Response:**

```
On December 28th, you debugged an nginx issue by:
1. Checking container logs for errors
2. Accessing the nginx container shell
3. Editing the nginx configuration file
4. Restarting nginx to apply changes

This appears to be a configuration fix workflow.
```

**Key Points:**
- No pre-detection of workflows needed
- LLM analyzes command sequences on-demand
- Flexible - answers any question about history
- Privacy-preserving (local Ollama)

## Cross-Machine Sync (User Responsibility)

zist does not include built-in sync. Users choose their own sync strategy:

### Option 1: rsync

```bash
# Sync history files between machines
rsync -avz machine-a:/home/user/.zsh_history machine-b:/home/user/
rsync -avz machine-a:/home/user/.claude/claude_zsh_history machine-b:/home/user/.claude/
```

### Option 2: MinIO/S3

```bash
# Push your history files to MinIO/S3
aws s3 cp ~/.zsh_history s3://my-bucket/history/

# Pull from any machine
aws s3 cp s3://my-bucket/history/.zsh_history ~/.zsh_history
```

### Option 3: Git

```bash
# Initialize a git repo in your history directory
cd ~/.zist/history
git init
git add .
git commit -m "Add history"

# Push to GitHub/GitLab for cross-machine access
git remote add origin git@github.com:user/history.git
git push origin main

# Pull on other machines
git pull origin main
```

### Option 4: Syncthing/Dropbox/etc.

Any file syncing tool works - just point it at your history files.

## Implementation Notes

### Zig Dependencies

**External libraries needed:**
- `zig-sqlite` (https://github.com/vrischmann/zig-sqlite) - SQLite bindings

**Built-in (std library):**
- `std.json` - JSON parsing (not needed for sync anymore, but kept for future)
- `std.fs` - File I/O for reading history and config
- `std.http.Client` - HTTP client for Ollama API
- `std.ChildProcess` - Execute shell commands (fzf fallback)
- `std.mem` - String parsing and manipulation

**No external library (write yourself):**
- INI parser (~100-150 LOC)
- ZSH history parser
- CLI argument parsing (or use `zig-clap` if preferred)

**Total external dependencies: 1 (just zig-sqlite)**

### ZSH History Format

ZSH extended history format:
```
: 1704384000:0;ls -la
: 1704384015:5;docker build -t app .
: 1704384020:0;git commit -m "initial commit"
```

Format: `: <timestamp>:<duration>;<command>`

**Parsing considerations:**
- Multi-line commands (continuation with `\`)
- Commands containing semicolons
- Corrupted/partial lines

### Subsecond Timestamp Handling

**Problem:** ZSH history only has second-resolution timestamps. Multiple commands in the same second from the same file would have identical `(source, timestamp)` primary keys.

**Solution:** Add subsecond precision based on order in history file.

```zig
// Pseudocode for collection
var commands_at_timestamp = HashMap(u64, []Command){};

// Read history file and group by timestamp
for line in history_file {
    (timestamp, duration, command) = parse(line);

    if (!commands_at_timestamp[timestamp]) {
        commands_at_timestamp[timestamp] = [];
    }
    commands_at_timestamp[timestamp].append({duration, command});
}

// Insert with subsecond precision
for (timestamp, commands) in commands_at_timestamp {
    for (i, cmd) in enumerate(commands) {
        // Add milliseconds based on order: .000, .001, .002, etc.
        precise_timestamp = timestamp + (i * 0.001);

        INSERT OR IGNORE INTO commands (source, timestamp, command, duration, ...)
        VALUES (source_path, precise_timestamp, cmd.command, cmd.duration, ...);
    }
}
```

**Why this works:**
- Order in history file is deterministic (ZSH writes chronologically)
- Same file always generates same subsecond timestamps for same commands
- Preserves execution order within the same second
- `(source, timestamp)` is unique per machine by construction

**Example:**
```
History file:
: 1704384000:5;ls -la        → DB: ("~/.zsh_history", 1704384000.000, "ls -la")
: 1704384000:2;git status    → DB: ("~/.zsh_history", 1704384000.001, "git status")
: 1704384000:1;docker ps     → DB: ("~/.zsh_history", 1704384000.002, "docker ps")

All three commands at the same second get unique timestamps.
```

### Search Performance

**Target:** < 100ms for interactive search

**Optimizations:**
- SQLite FTS5 for full-text search
- Index on timestamp for time-based queries
- Limit results to reasonable number (100 default)

### Configuration Parsing (INI)

**Format:** Simple INI format (no external library needed)

**Parser implementation (Zig):**
```zig
// Simple state machine
// ~100-150 lines of code
// Parse sections: [section]
// Parse key-value: key = value
// Handle comments: # comment
// Trim whitespace
// Split arrays on comma or space
```

**Key features needed:**
- Section tracking: `[collection]`, `[llm]`, etc.
- Key-value parsing: split on `=`
- Comment handling: ignore lines starting with `#`
- Array parsing: split `history_files = file1, file2, file3` on comma
- Whitespace trimming
- Tilde expansion for home directory

**Complexity:** Low - straightforward string parsing, good Zig learning exercise

### TUI Implementation

**Decision:** Custom Zig TUI (no external dependencies)
- Full control over behavior
- No `fzf` dependency
- Smaller binary

**Alternative:** Shell out to `fzf` if user prefers
- Can be config option

## Distribution

### Static Binaries

**Zig builds static binaries natively:**

```bash
# Cross-compile for all platforms from one machine
zig build -Dtarget=x86_64-linux-musl -Doptimize=ReleaseFast
zig build -Dtarget=x86_64-macos -Doptimize=ReleaseFast
zig build -Dtarget=aarch64-macos -Doptimize=ReleaseFast
zig build -Dtarget=x86_64-windows -Doptimize=ReleaseFast
```

**GitHub Releases:**
```
zist-v1.0.0/
  ├── zist-linux-x64      (~500KB)
  ├── zist-macos-intel    (~500KB)
  ├── zist-macos-arm      (~500KB)
  └── zist-windows.exe    (~500KB)
```

Users: Download → `chmod +x` → Run

### Package Managers (Future)

- Homebrew (macOS/Linux)
- AUR (Arch Linux)
- apt/yum repos

## Scale Estimates

**Target environment:**
- Single machine
- Multiple history files (~3-10 files)
- ~10k-100k commands total
- Average: ~50 bytes per command

**Storage:**
- Commands: ~5-10MB
- With indexes: ~15-30MB per machine

**Performance:**
- Collection: < 10ms per file (non-blocking background)
- Search: < 100ms (local SQLite query)

## Implementation Roadmap

### Phase 1: Core MVP
- [x] Design document
- [ ] SQLite schema implementation
- [ ] ZSH history parser (Zig)
- [ ] Collection command (`zist collect`)
- [ ] Local search command (`zist search`)
- [ ] Custom TUI or fzf integration
- [ ] ZSH integration (`zist install`)
- [ ] Static binary builds

### Phase 2: Conversational Search (RAG)
- [ ] Ollama integration
- [ ] Ask command (`zist ask`)
- [ ] Prompt engineering for command analysis
- [ ] Keyword extraction from questions
- [ ] Streaming LLM responses

### Phase 3: Polish & Distribution
- [ ] Cross-platform testing
- [ ] Documentation and README
- [ ] GitHub repository setup
- [ ] CI/CD for releases
- [ ] Package manager submissions

### Future (Optional)
- [ ] Vector embeddings for semantic search
- [ ] Web UI for browsing history
- [ ] Export workflows as scripts
- [ ] Bash/Fish support
- [ ] Privacy filters (exclude sensitive patterns)

## Decisions Made

**Architecture:**
- ✅ Local aggregation only (no built-in sync)
- ✅ User handles cross-machine sync (rsync, MinIO, git, etc.)
- ✅ SQLite for local storage
- ✅ `(source, timestamp)` primary key with subsecond precision for dedup
- ✅ `INSERT OR IGNORE` for automatic deduplication
- ✅ RAG approach for conversational search (no pre-workflow detection)

**Implementation:**
- ✅ CLI in Zig (static binaries, full project in Zig)
- ✅ INI format for configuration (simple, no external parser)
- ✅ Local Ollama for LLM features (privacy-preserving)

**Removed from original design:**
- ❌ SSH P2P sync protocol
- ❌ `zist sync` command
- ❌ `zist serve-sync` command
- ❌ sync_state table
- ❌ JSON exchange format for sync

## Open Questions

### Technical Decisions
- TUI: Custom Zig or shell out to fzf? (Start with custom, fallback to fzf optional)
- Keybinding: Ctrl+R (override default) or different key?

### Open Source
- License: MIT, Apache 2.0, GPL?
- Target audience: Solo developers, DevOps professionals, small teams?
- Contribution model: Issues/PRs welcome? Roadmap-driven?

---

*Document created: 2026-01-04*
*Last updated: 2026-01-11*
*Status: Design phase - simplified to local aggregation only*