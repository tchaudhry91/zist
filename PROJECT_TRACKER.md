# zist - Project Tracker

**Last Updated:** 2026-01-04

---

## Project Overview

P2P command history synchronization tool written in Zig.
See `zist-design.md` for full design specification.

---

## Phase 1: Core MVP (No Sync)

### Setup & Infrastructure
- [ ] Zig project structure (`build.zig`, directory layout)
- [ ] SQLite integration (zig-sqlite dependency)
- [ ] Database schema creation
- [ ] INI configuration parser

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

**None** - Ready to start implementation

---

## Recently Completed

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

**Suggested first steps:**
1. Set up Zig project structure (build.zig, src/ directory)
2. Add zig-sqlite dependency
3. Create database schema (run SQL to create tables)
4. Write simple INI parser
5. Test: Read config.ini and print values

---

## Resources & References

- Zig SQLite: https://github.com/vrischmann/zig-sqlite
- Zig Language Reference: https://ziglang.org/documentation/master/
- ZSH History Format: Extended history (`: timestamp:duration;command`)
