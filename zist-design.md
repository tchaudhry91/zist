# zist - ZSH History Synchronization

A peer-to-peer tool for synchronizing and searching ZSH command history across multiple machines.

## Overview

`zist` enables you to search and recall commands executed across all your machines from any machine. Each machine runs a Zig CLI that maintains a local SQLite database and syncs with peer machines.

**Key Features:**
- Peer-to-peer sync (no central server needed)
- Local SQLite database on each machine
- Interactive search (fzf-style TUI)
- Conversational search via LLM (RAG approach)
- Static binary distribution

## Architecture

### Peer-to-Peer Design

```
┌──────────────────┐          ┌──────────────────┐
│   QE93           │          │   QECarbon       │
│   (desktop)      │◄────────►│   (laptop)       │
│                  │   SSH    │                  │
│ ~/.zsh_history   │          │ ~/.zsh_history   │
│ ~/.zist/zist.db  │          │ ~/.zist/zist.db  │
└────────┬─────────┘          └────────┬─────────┘
         │                              │
         │           SSH                │
         ├──────────────────────────────┤
         ↓              ↓               ↓
    ┌─────────┐   ┌─────────┐   ┌─────────┐
    │  main   │◄─►│  dev    │◄─►│  auir   │
    │ (always)│   │ (always)│   │ (always)│
    │  on)    │   │  on)    │   │  on)    │
    └─────────┘   └─────────┘   └─────────┘

    Servers sync with each other (cron every 5 min)
    Desktops/laptops sync on-demand or at startup
```

**How it works:**
1. Each machine runs `zist collect` after every command (via ZSH hook)
2. Commands stored in local SQLite database (`~/.zist/zist.db`)
3. Machines sync via SSH using `zist serve-sync` protocol
4. Servers sync automatically (cron), workstations sync on-demand
5. Search/LLM queries run against local database (always fast, works offline)

**Benefits:**
- No central server to maintain
- Works offline (local search always available)
- Transitive sync: servers share all data, desktops catch up when online
- Simple deployment: just SSH (no daemon, no ports)
- Automatic deduplication via `(machine, timestamp)` primary key

## Components

### 1. CLI Tool (Zig)

**Binary:** `zist`
**Configuration:** `~/.zist/config.toml`
**Database:** `~/.zist/zist.db` (SQLite)

**Commands:**

```bash
# Interactive search (fzf-style)
zist search [query]

# Conversational search (RAG)
zist ask "How did I fix that nginx issue?"

# Collection (called by ZSH hook)
zist collect

# Sync with peers
zist sync

# Server mode (called via SSH)
zist serve-sync    # Read JSON from stdin, respond to stdout

# Installation
zist install    # Sets up ZSH integration
zist uninstall
```

### 2. Database Schema (SQLite)

```sql
CREATE TABLE commands (
    machine TEXT NOT NULL,            -- hostname of machine (e.g., "QE93", "main")
    timestamp REAL NOT NULL,          -- unix timestamp with subsecond precision
    command TEXT NOT NULL,            -- the command executed
    source TEXT NOT NULL,             -- history file path (e.g., "~/.zsh_history")
    duration INTEGER,                 -- execution duration in seconds
    cwd TEXT,                         -- working directory
    exit_code INTEGER,                -- command exit code

    PRIMARY KEY (machine, timestamp)  -- unique per machine+timestamp, handles dedup
);

CREATE INDEX idx_timestamp ON commands(timestamp DESC);
CREATE INDEX idx_machine ON commands(machine);
CREATE INDEX idx_source ON commands(source);

-- Full-text search on command text
CREATE VIRTUAL TABLE commands_fts USING fts5(
    command,
    content='commands',
    content_rowid='rowid'
);

-- Track what we've received from each origin machine
CREATE TABLE sync_state (
    origin_machine TEXT PRIMARY KEY,  -- machine where commands originated
    last_timestamp REAL               -- highest timestamp seen from that machine
);
```

**Key Design Decisions:**
- `(machine, timestamp)` as primary key enables automatic deduplication via `INSERT OR IGNORE`
- Timestamp stored as REAL for subsecond precision (see "Subsecond Timestamp Handling" below)
- `sync_state` tracks per-origin-machine, not per-peer (handles transitive sync)

### 3. Configuration

**~/.config/zist/config.ini:**

```ini
[collection]
# Paths to ZSH history files (supports ~ expansion)
# Comma-separated list; all files will be collected from
history_files = ~/.zsh_history, ~/.claude/claude_zsh_history

# Machine name for command attribution
# Use 'auto' to detect from hostname, or set explicitly
machine_name = auto

[sync]
# Comma or space-separated list of peer machines (SSH hostnames)
# These should be SSH-accessible (configured in ~/.ssh/config)
peers = main,dev,auir

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
- All settings have sensible defaults except `peers` (must be configured)

**Example scenarios:**
- **QE93** (desktop): Syncs with main, dev, auir (servers always available)
- **QECarbon** (laptop): Syncs when online
- **main, dev, auir** (servers): Sync with each other via cron every 5 min
- All machines can sync with any peer - data propagates transitively

## User Flows

### Collection Flow

```
1. User executes command in shell
2. Command completes
3. precmd() hook triggers
4. zist collect runs in background (&)
5. zist reads last command from ~/.zsh_history
6. zist parses command (timestamp, duration, command text)
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

### Sync Flow (SSH-based P2P)

**Triggered by:** User runs `zist sync`, cron job, or shell startup

**Example: QE93 syncs with main**

```
1. QE93 reads local sync_state table:
   {
     "auir": 1500.000,
     "main": 2000.000,
     "qecarbon": 1800.500
   }

2. QE93 gathers new local commands (from QE93) since last sync

3. QE93 connects via SSH and exchanges data:

   ssh main "zist serve-sync" <<EOF
   {
     "state": {
       "auir": 1500.000,
       "main": 2000.000,
       "qecarbon": 1800.500
     },
     "commands": [
       {"machine": "QE93", "timestamp": 3000.000, "command": "ls", "source": "~/.zsh_history", ...},
       {"machine": "QE93", "timestamp": 3001.000, "command": "docker ps", "source": "~/.claude/claude_zsh_history", ...}
     ]
   }
   EOF

4. main receives request:
   - Inserts QE93's commands via INSERT OR IGNORE (dedup automatic)
   - Queries for commands QE93 is missing:
     * Commands from auir where timestamp > 1500.000
     * Commands from main where timestamp > 2000.000
     * Commands from qecarbon where timestamp > 1800.500
     * Commands from dev (not in state) where timestamp > 0
   - Returns JSON response with missing commands

5. main responds:
   {
     "commands": [
       {"machine": "dev", "timestamp": 100.000, "command": "cd /tmp", "source": "~/.zsh_history", ...},
       {"machine": "auir", "timestamp": 1501.200, "command": "docker ps", "source": "~/.zsh_history", ...},
       ...
     ],
     "received": 2
   }

6. QE93 processes response:
   - INSERT OR IGNORE each command (automatic dedup)
   - Update sync_state table with new highest timestamps
   - Log sync summary

7. Repeat for each configured peer
```

**Key Features:**
- **Bidirectional:** Push local commands, pull missing commands in one exchange
- **Deduplication:** `INSERT OR IGNORE` handles duplicates automatically
- **Transitive sync:** Peers share commands from all origins they know about
- **No daemon:** `zist serve-sync` runs, responds, exits
- **SSH security:** Uses existing SSH authentication, no new ports

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
  █ docker build -t myapp .          [server-a] 2026-01-03 14:23
  █ docker ps -a                      [laptop]   2026-01-02 10:15
  █ docker compose up -d              [server-b] 2026-01-01 09:00
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

## Implementation Notes

### Zig Dependencies

**External libraries needed:**
- `zig-sqlite` (https://github.com/vrischmann/zig-sqlite) - SQLite bindings

**Built-in (std library):**
- `std.json` - JSON parsing/serialization for sync protocol
- `std.fs` - File I/O for reading history and config
- `std.http.Client` - HTTP client for Ollama API
- `std.ChildProcess` - Execute SSH commands
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

**Problem:** ZSH history only has second-resolution timestamps. Multiple commands in the same second would have identical `(machine, timestamp)` primary keys.

**Solution:** Add subsecond precision based on order in history file.

```zig
// Pseudocode for collection
var commands_at_timestamp = {};

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

        INSERT OR IGNORE INTO commands (machine, timestamp, command, duration, ...)
        VALUES (machine_name, precise_timestamp, cmd.command, cmd.duration, ...);
    }
}
```

**Why this works:**
- Order in history file is deterministic (ZSH writes chronologically)
- Same machine always generates same subsecond timestamps for same commands
- When syncing, `(machine, timestamp)` is unique across all peers
- Preserves execution order within the same second

**Example:**
```
History file:
: 1704384000:5;ls -la        → DB: ("QE93", 1704384000.000, "ls -la")
: 1704384000:2;git status    → DB: ("QE93", 1704384000.001, "git status")
: 1704384000:1;docker ps     → DB: ("QE93", 1704384000.002, "docker ps")

All three commands at the same second get unique timestamps.
Same data synced to all peers maintains same timestamps.
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
- Section tracking: `[collection]`, `[sync]`, etc.
- Key-value parsing: split on `=`
- Comment handling: ignore lines starting with `#`
- Array parsing: split `peers = main,dev,auir` on comma
- Whitespace trimming

**Complexity:** Low - straightforward string parsing, good Zig learning exercise

### TUI Implementation

**Decision:** Custom Zig TUI (no external dependencies)
- Full control over behavior
- No `fzf` dependency
- Smaller binary

**Alternative:** Shell out to `fzf` if user prefers
- Can be config option

### P2P Sync Protocol

**Design:** SSH-based bidirectional exchange using `zist serve-sync` command

**Protocol Format:**

**Request (from client to peer):**
```json
{
  "state": {
    "origin_machine_1": 1234567890.123,
    "origin_machine_2": 1234567900.456,
    ...
  },
  "commands": [
    {
      "machine": "client_machine",
      "timestamp": 1234567910.000,
      "command": "ls -la",
      "source": "~/.zsh_history",
      "duration": 0,
      "cwd": "/home/user",
      "exit_code": 0
    },
    ...
  ]
}
```

**Response (from peer to client):**
```json
{
  "commands": [
    {
      "machine": "some_origin",
      "timestamp": 1234567920.000,
      "command": "docker ps",
      "source": "~/.zsh_history",
      "duration": 1,
      "cwd": "/home/user",
      "exit_code": 0
    },
    ...
  ],
  "received": 5
}
```

**Implementation Details:**

**Client side (`zist sync`):**
1. Read local `sync_state` table (highest timestamp per origin machine)
2. Gather new local commands since last sync
3. For each peer in config:
   - SSH to peer: `ssh peer "zist serve-sync"`
   - Send state + commands as JSON to stdin
   - Receive missing commands from stdout
   - `INSERT OR IGNORE` received commands
   - Update `sync_state` with new highest timestamps

**Server side (`zist serve-sync`):**
1. Read JSON request from stdin
2. Insert client's commands: `INSERT OR IGNORE INTO commands ...`
3. Query for commands client is missing:
   ```sql
   -- For each origin in client's state
   SELECT * FROM commands
   WHERE machine = ? AND timestamp > ?

   -- For origins not in client's state (new peers)
   SELECT * FROM commands
   WHERE machine NOT IN (client_state_keys)
   ```
4. Format response as JSON
5. Write to stdout and exit

**Conflict Resolution:**
- No conflicts possible! `(machine, timestamp)` is globally unique
- `INSERT OR IGNORE` silently skips duplicates
- Multiple peers can have same command, dedup is automatic

**Fault Tolerance:**
- If peer unreachable: skip to next peer, try again later
- Partial sync okay: other peers will fill gaps
- No transaction required: each command insert is independent

**Authentication:**
- Uses SSH (existing authentication mechanism)
- No passwords or tokens needed
- SSH keys already configured on machines

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
- Cargo (just for distribution)

## Scale Estimates

**Target environment:**
- 10-12 machines
- ~10k commands per machine
- Total: ~120k commands

**Storage:**
- Average command length: ~50 bytes
- Commands: ~6MB
- With indexes: ~20-30MB per machine

**Performance:**
- Collection: < 10ms (non-blocking background)
- Search: < 100ms (local SQLite query)
- Sync: Depends on network and delta size

## Implementation Roadmap

### Phase 1: Core MVP (No Sync)
- [x] Design document
- [ ] SQLite schema implementation
- [ ] ZSH history parser (Zig)
- [ ] Collection command (`zist collect`)
- [ ] Local search command (`zist search`)
- [ ] Custom TUI or fzf integration
- [ ] ZSH integration (`zist install`)
- [ ] Static binary builds

### Phase 2: P2P Sync
- [ ] Design P2P sync protocol
- [ ] Implement peer discovery/configuration
- [ ] Sync command (`zist sync`)
- [ ] Conflict resolution
- [ ] Fault tolerance and retries

### Phase 3: Conversational Search (RAG)
- [ ] Ollama integration
- [ ] Ask command (`zist ask`)
- [ ] Prompt engineering for command analysis
- [ ] Keyword extraction from questions
- [ ] Streaming LLM responses

### Phase 4: Polish & Distribution
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
- ✅ Peer-to-peer sync via SSH (no central server, no daemon)
- ✅ SQLite for local storage on each machine
- ✅ `(machine, timestamp)` primary key with subsecond precision for dedup
- ✅ `INSERT OR IGNORE` for automatic conflict resolution
- ✅ Transitive sync: servers as "data hubs", workstations sync on-demand
- ✅ RAG approach for conversational search (no pre-workflow detection)

**Implementation:**
- ✅ CLI in Zig (static binaries, full project in Zig)
- ✅ INI format for configuration (simple, no external parser)
- ✅ `zist serve-sync` command for SSH-based protocol
- ✅ JSON exchange format for sync protocol (using `std.json`)
- ✅ Local Ollama for LLM features (privacy-preserving)

## Open Questions

### Technical Decisions
- TUI: Custom Zig or shell out to fzf? (Start with custom, fallback to fzf optional)
- Keybinding: Ctrl+R (override default) or different key?
- Sync trigger on workstations: Manual, on terminal start, or triggered by search?
- Cron frequency on servers: Every 5 min? Configurable?

### Open Source
- License: MIT, Apache 2.0, GPL?
- Target audience: Solo developers, DevOps professionals, small teams?
- Contribution model: Issues/PRs welcome? Roadmap-driven?

---

*Document created: 2026-01-04*
*Last updated: 2026-01-04*
*Status: Design phase - P2P architecture*
