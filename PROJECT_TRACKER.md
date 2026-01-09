# zist - Project Tracker

**Last Updated:** 2026-01-09

---

## Project Overview

P2P command history synchronization tool written in Zig.
See `zist-design.md` for full design specification.

---

## Phase 1: Core MVP (No Sync)

### Setup & Infrastructure
- [x] Zig project structure (`build.zig`, directory layout)
- [ ] SQLite integration (zig-sqlite dependency)
- [ ] Database schema creation
- [~] INI configuration parser (IN PROGRESS)
  - [x] Config struct definitions (Collection, Sync, LLM)
  - [x] expandHome helper function
  - [x] File reading and line parsing
  - [x] Section parsing ([section])
  - [x] Comment handling (# comments)
  - [ ] Key-value parsing (key = value)
  - [ ] Array parsing (comma-separated values)
  - [x] Test infrastructure with @embedFile

### Core Features
- [ ] ZSH history parser
  - [ ] Basic line parsing (`: timestamp:duration;command`)
  - [ ] Multi-line command handling
  - [ ] Subsecond timestamp generation
- [ ] `zist collect` command
  - [ ] Parse history file
  - [ ] Insert into SQLite with deduplication
  - [ ] Handle errors gracefully
- [ ] `zist search` command
  - [ ] SQLite FTS5 queries
  - [ ] Shell out to fzf for UI
  - [ ] Return selected command to shell
- [ ] `zist install` command
  - [ ] Modify .zshrc with precmd hook
  - [ ] Create default config.ini
  - [ ] Set up directory structure

---

## Phase 2: P2P Sync

- [ ] JSON serialization/deserialization for sync protocol
- [ ] `zist serve-sync` command
  - [ ] Read JSON from stdin
  - [ ] Query for missing commands
  - [ ] Write JSON to stdout
- [ ] `zist sync` command
  - [ ] Read local sync_state
  - [ ] SSH to each peer
  - [ ] Exchange data
  - [ ] Update local DB and sync_state
- [ ] Sync state management
- [ ] Error handling (peer unreachable, network failures)

---

## Phase 3: Conversational Search (RAG)

- [ ] Ollama HTTP client integration
- [ ] `zist ask` command
  - [ ] Keyword extraction from question
  - [ ] Query relevant commands
  - [ ] Build LLM prompt
  - [ ] Stream response from Ollama

---

## Phase 4: Polish & Distribution

- [ ] Cross-platform builds (Linux, macOS, Windows)
- [ ] GitHub Actions CI/CD
- [ ] Documentation (README, installation guide)
- [ ] Package releases

---

## Currently In Progress

**INI Configuration Parser** (`src/config.zig`)
- Config struct with Collection, Sync, LLM sections defined
- expandHome() helper implemented
- Section enum for tracking active section
- parse() reads file, delegates to parseFromString()
- parseFromString() parses lines, handles comments, identifies sections
- Need to implement: key-value parsing per section

---

## Recently Completed

**2026-01-09:**
- ✅ Project structure synced and reviewed

**Previous session:**
- ✅ Zig project structure set up (build.zig, src/)
- ✅ Basic CLI skeleton (main.zig with help/version)
- ✅ Library module structure (root.zig exports config)
- ✅ Config struct definitions started
- ✅ bufferedPrint utility function

**2026-01-04:**
- ✅ Design document finalized
- ✅ P2P sync protocol designed (SSH-based)
- ✅ Database schema designed
- ✅ Configuration format decided (INI)
- ✅ Subsecond timestamp handling designed
- ✅ RAG approach for conversational search

---

## Code Reviews & Notes

_Agent will add code review notes here when code is submitted_

---

## Known Issues / Blockers

_None currently_

---

## Questions / Design Decisions Needed

_Add questions here as they come up during implementation_

---

## Next Session Goals

**Continue INI parser implementation:**
1. Complete the `load()` function in `config.zig`:
   - Read file contents
   - Parse lines (skip comments, handle sections)
   - Parse key-value pairs
   - Populate Config struct fields
2. Add tests for INI parser
3. Test with sample config.ini file

**After INI parser:**
4. Add zig-sqlite dependency
5. Create database schema (run SQL to create tables)
6. Start ZSH history parser

---

## Resources & References

- Zig SQLite: https://github.com/vrischmann/zig-sqlite
- Zig Language Reference: https://ziglang.org/documentation/master/
- ZSH History Format: Extended history (`: timestamp:duration;command`)
