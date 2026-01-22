# zist

Local ZSH history aggregation tool. Collect commands from multiple ZSH history files, store them in a local SQLite database, and search instantly with fuzzy matching.

## Why zist?

- **Multiple history files**: Collect from several sources simultaneously
- **Instant search**: Query 10,000+ commands in milliseconds with SQLite FTS5
- **Ctrl+X for fuzzy search**: Interactive fuzzy search with fzf and preview pane
- **Automatic deduplication**: `(source, timestamp)` primary key prevents duplicates
- **AI assistant history**: Collect from Claude Desktop and OpenCode

## Features

- **Collect** from files or directories (recursive search)
- **Search** with full-text search, fuzzy matching, and time filtering
- **Preview pane** shows source file and timestamp while browsing
- **Interactive** ZSH integration (Ctrl+X)
- **Batch inserts** with transactions
- **Metadata storage**: duration, cwd, exit code
- **Subsecond timestamps** for duplicate deduplication

## Requirements

- fzf (for search functionality)

## Installation

### Pre-built binaries (recommended)

Download the latest release from [GitHub releases](https://github.com/tchaudhry91/zist/releases):

```bash
# Linux (x64)
curl -L https://github.com/tchaudhry91/zist/releases/latest/download/zist-linux-x64 -o zist
chmod +x zist
sudo mv zist /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/tchaudhry91/zist/releases/latest/download/zist-macos-intel -o zist
chmod +x zist
sudo mv zist /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/tchaudhry91/zist/releases/latest/download/zist-macos-arm -o zist
chmod +x zist
sudo mv zist /usr/local/bin/
```

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
# Collect from your ZSH history file
zist collect ~/.zsh_history

# Or collect from multiple history files at once
zist collect ~/.zsh_history ~/.claude/claude_zsh_history ~/.opencode_zsh_history

# Or collect from a directory (recursively finds all *zsh_history files)
zist collect ~/.histories/

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

- **PATH**: History file or directory to search (paths can be mixed)
- **--db**: Database path (default: `~/.zist/zist.db`)
- **--quiet**: Suppress output (useful for scripts/automation)

Directories are searched recursively for `*zsh_history` files.

**Example - Collect from multiple sources:**
```bash
zist collect ~/.zsh_history ~/.claude/claude_zsh_history ~/.opencode_zsh_history
```

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

### wizard

Generate shell commands from natural language using an LLM.

```bash
zist wizard [--db PATH] [--query QUERY] [--pwd PATH] [--llm-api-url URL] [--model NAME] [--key KEY] [--timeout DURATION] [--cache QUERY] [--cache-command CMD] [--list-cache] [--clear-cache]
```

- **--query**: Natural language query to convert to shell command
- **--pwd**: Current working directory (default: actual PWD)
- **--db**: Database path (default: `~/.zist/zist.db`)
- **--llm-api-url**: LLM API endpoint (overridden by `ZIST_LLM_API_URL` env var)
- **--model**: Model name (overridden by `ZIST_MODEL` env var)
- **--key**: API key (overridden by `ZIST_LLM_API_KEY` env var)
- **--timeout**: LLM request timeout (default: 30s)
- **--cache**: Cache a query→command mapping (use with --cache-command)
- **--cache-command**: Command to cache (use with --cache)
- **--list-cache**: List all cached query→command mappings
- **--clear-cache**: Clear all cached mappings

## Configuration

zist can be configured using environment variables.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ZIST_LLM_API_URL` | LLM API endpoint URL | `http://localhost:11434/v1` |
| `ZIST_MODEL` | Model name to use | `qwen2.5-coder:3b` |
| `ZIST_LLM_API_KEY` | API key for hosted LLM providers | `ollama` |

### Example Configuration

**Shell export (temporary):**
```bash
export ZIST_LLM_API_URL=http://localhost:11434/v1
export ZIST_MODEL=qwen2.5-coder:3b
```

**Add to ~/.zshrc or ~/.bashrc (permanent):**
```bash
# For Ollama
export ZIST_LLM_API_URL=http://localhost:11434/v1
export ZIST_MODEL=qwen2.5-coder:3b

# Or for OpenRouter
export ZIST_LLM_API_URL=https://openrouter.ai/api/v1
export ZIST_LLM_API_KEY=sk-or-v1-your-key-here
export ZIST_MODEL=deepseek/deepseek-coder
```

**Command-line flags override environment variables:**
```bash
# Even with env vars set, you can override per-command
ZIST_LLM_API_URL=http://localhost:11434/v1 zist wizard --query "test"

# Or override with flags
zist wizard --query "test" --llm-api-url https://api.openai.com/v1 --model gpt-4o
```

## Collect History from AI Assistants

zist can capture commands run by your AI assistants. Here's how to enable it:

### Claude Code

Add this to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '\": \\(now | floor):0;\\(.tool_input.command)\"' >> ~/.claude/claude_zsh_history"
          }
        ]
      }
    ]
  }
}
```

Then collect the history:
```bash
zist collect ~/.claude/claude_zsh_history
```

### OpenCode

Create a plugin at `~/.opencode/plugins/bash-history.ts`:

```typescript
import { appendFileSync } from "fs"
import { homedir } from "os"

export async function BashHistoryPlugin() {
  const historyFile = `${homedir()}/.opencode_zsh_history`

  return {
    "tool.execute.before": async (input, output) => {
      if (input?.tool === "bash" && output?.args?.command) {
        const timestamp = Math.floor(Date.now() / 1000)
        appendFileSync(historyFile, `: ${timestamp}:0;${output.args.command}\n`)
      }
    },
  }
}
```

Then collect the history:
```bash
zist collect ~/.opencode_zsh_history
```

## ZSH Integration

Install keybindings:

```bash
zist install
source ~/.zshrc
```

**Keybindings:**
- **Ctrl+X** - Fuzzy search history (uses what you typed as query)
- **Ctrl+G** - AI wizard (natural language → command)

### History Search (Ctrl+X)

Press Ctrl+X to search across all aggregated history with fuzzy matching:

- Uses `$LBUFFER` (what you typed before Ctrl+X) as initial query
- Opens fzf with all commands from database (with preview pane)
- Places selected command in buffer for editing
- precmd hook automatically collects from `~/.histories` after each command

### AI Wizard (Ctrl+G)

Press Ctrl+G to convert natural language to shell commands using an LLM.

zist works with **any OpenAI-compatible API**, including Ollama, OpenAI, OpenRouter, Together, Groq, and more.

**Option 1: Ollama (local, free)**
```bash
# Install Ollama (https://ollama.com)
curl https://ollama.com/install.sh | sh

# Pull a code model
ollama pull qwen2.5-coder:3b

# Configure zist
export ZIST_LLM_API_URL=http://localhost:11434/v1
export ZIST_MODEL=qwen2.5-coder:3b
```

**Option 2: OpenRouter (cloud, pay-per-token)**
```bash
# Get an API key from https://openrouter.ai
# Configure zist
export ZIST_LLM_API_URL=https://openrouter.ai/api/v1
export ZIST_LLM_API_KEY=sk-or-v1-...  # Your OpenRouter key
export ZIST_MODEL=deepseek/deepseek-coder
```

**Option 3: OpenAI**
```bash
# Configure zist
export ZIST_LLM_API_URL=https://api.openai.com/v1
export ZIST_LLM_API_KEY=sk-...  # Your OpenAI key
export ZIST_MODEL=gpt-4o
```

**Command-line usage:**
```bash
zist wizard --query "list all running docker containers"
zist wizard --query "find all files larger than 100MB"
zist wizard --query "compress a directory into tar.gz"
```

**Wizard features:**
- Caches query→command mappings after execution to speed up repeated queries
- Learns from your command history for better suggestions
- Uses your current working directory for context

**Cache management:**
```bash
zist wizard --list-cache      # View cached mappings
zist wizard --clear-cache     # Clear all cache
```

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
