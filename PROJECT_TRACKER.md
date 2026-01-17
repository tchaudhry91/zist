# zist - Project Tracker

**Last Updated:** 2026-01-17

---

## Project Overview

Local ZSH history aggregation tool written in Go.
See `zist-design.md` for full design specification.

**Changed from Zig:** Moved to Go for faster iteration (already know Go, want to ship quickly).

---

## Phase 1: Core MVP

### Setup & Infrastructure
- [x] Go module initialized (`go.mod`)
- [x] SQLite integration (mattn/go-sqlite3)
- [x] Database schema creation (with FTS5 for search)
- [x] CLI argument parsing (ff/v4 - flags-first, simpler than cobra)
- [x] History files as CLI arguments (no config file)

### Core Features
- [x] ZSH history parser (Go)
  - [x] Parse `: timestamp:duration;command` format
  - [x] Multi-line command handling
  - [x] Subsecond timestamp generation
  - [x] All unit tests passing
- [x] `zist collect` command
  - [x] Parse history files passed as CLI args
  - [x] Insert into SQLite with deduplication
  - [x] Handle errors gracefully
  - [x] Batch inserts (500 at a time) for performance
  - [x] Transactions for atomicity
  - [x] Database stats reporting
- [x] Database layer
  - [x] `database.go` with InitDB, CreateSchema, InsertCommands
  - [x] SQLite schema with FTS5 table for search
  - [x] INSERT OR IGNORE for automatic deduplication
  - [x] ~ path expansion for database path
  - [x] All unit tests passing (4 test cases)
- [x] `zist search` command
  - [x] SQLite FTS5 queries (with prefix wildcards and escaping)
  - [x] Shell out to fzf for UI
  - [x] Return selected command to shell
  - [x] Result limit (100 rows)

---

## Phase 2: Conversational Search (RAG)

- [ ] Ollama HTTP client integration
- [ ] `zist ask` command
  - [ ] Keyword extraction from question
  - [ ] Query relevant commands
  - [ ] Build LLM prompt
  - [ ] Stream response from Ollama

---

## Phase 3: Polish & Distribution

- [ ] Cross-platform builds (Linux, macOS, Windows)
- [ ] GitHub Actions CI/CD
- [ ] Documentation (README, installation guide)
- [ ] Package releases

---

## Currently In Progress

**Search Implementation:** FTS5 search working with proper wildcard escaping. ZSH integration complete with precmd hook.

## Code Reviews & Notes (2026-01-17)

**Fixed in this session:**
1. **ZSH Precmd Hook**: Added `precmd()` function to integration that runs `zist collect &` after every command
2. **Idempotent Install**: Simplified check to look for "# zist integration" marker
3. **Uninstall Fix**: Updated to properly remove both Ctrl+X binding and precmd hook
4. **FTS5 Query Injection**: Added `buildFTSQuery()` with escaping for special characters
5. **Query Wildcards**: Changed from `*git*` (invalid) to `git*` (valid prefix wildcard)
6. **Result Limits**: Added `LIMIT 100` to prevent large result sets

## Recently Completed

**2026-01-17 (SQLite Integration - Complete):**
- ✅ Added `mattn/go-sqlite3` dependency
- ✅ Created `database.go` with InitDB, CreateSchema, InsertCommands
- ✅ SQLite schema with FTS5 for full-text search
- ✅ Batch inserts (500 at a time) for performance (~9600 commands in <1s)
- ✅ Transactions for atomicity
- ✅ INSERT OR IGNORE for automatic deduplication (primary key: source, timestamp)
- ✅ ~ path expansion for database paths
- ✅ Database stats reporting (total commands, sources)
- ✅ All unit tests passing (4 database tests)
- ✅ Tested with actual history files:
  - `~/.zsh_history`: 9264 commands
  - `~/.claude/claude_zsh_history`: 79 commands
  - `~/.opencode_zsh_history`: 267 commands
  - Total: ~9610 commands (~1.9MB database)
- ✅ Build tag: `-tags "fts5"` required for FTS5 support

**2026-01-17 (ZSH History Parser - Complete):**
- ✅ Created `history.go` with complete parser
- ✅ Parse `: timestamp:duration;command` format correctly
- ✅ Handle multi-line commands (continuation lines)
- ✅ Subsecond timestamp generation for duplicate timestamps
- ✅ Parse duration field
- ✅ Handle edge cases (empty lines, malformed input)
- ✅ All unit tests passing (5 test cases)
- ✅ Tested with actual history files (~9500 commands parsed)

**2026-01-17 (CLI with ff/v4 - Complete):**
- ✅ Migrated from Cobra to ff/v4 (flags-first, simpler)
- ✅ Simple directory structure (all `.go` files at root)
- ✅ Main-driven approach with functions called from root
- ✅ Module: `github.com/tchaudhry91/zist`
- ✅ Binary builds and runs successfully

**2026-01-17 (Language migration to Go - Complete):**
- ✅ Made decision to migrate from Zig to Go
- ✅ Removed all Zig code and artifacts (~350MB freed)
- ✅ Preserved design document (language-agnostic)
- ✅ Initialized Go module

**Module Name:** `github.com/tchaudhry91/zist` (updated from `github.com/tchaudhry/zist`)

**Previous Zig progress (archived):**
- Had started ZSH history parser scaffolding
- Will need to reimplement in Go

**Previous Zig progress (archived):**
- Had started ZSH history parser scaffolding
- Will need to reimplement in Go

---

## Known Issues / Blockers

_None currently_

---

## Questions / Design Decisions Needed

_Add questions here as they come up during implementation_

---

## Next Session Goals

**Phase 1 Complete (MVP Ready):**
- ✅ ZSH history parser
- ✅ Database layer with FTS5 search
- ✅ `zist collect` command
- ✅ `zist search` command
- ✅ ZSH integration (Ctrl+X + precmd hook)

**Next Steps:**
1. Cross-platform builds (Linux, macOS, Windows)
2. Documentation and README polish
3. CI/CD for releases
4. Phase 2: `zist ask` (RAG/LLM)

---

## Resources & References

- Go SQLite: https://github.com/mattn/go-sqlite3
- Cobra CLI: https://github.com/spf13/cobra
- ZSH History Format: Extended history (`: timestamp:duration;command`)
- Zist Design: `zist-design.md`
