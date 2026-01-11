# zist - Project Tracker

**Last Updated:** 2026-01-11

---

## Project Overview

Local ZSH history aggregation tool written in Zig.
See `zist-design.md` for full design specification.

---

## Phase 1: Core MVP

### Setup & Infrastructure
- [x] Zig project structure (`build.zig`, directory layout)
- [ ] SQLite integration (zig-sqlite dependency)
- [ ] Database schema creation
- [x] Basic CLI argument parsing (help, version, collect, search)
- [x] History files as CLI arguments (no config file)

### Core Features
- [~] ZSH history parser (IN PROGRESS)
  - [x] Scaffold with History/Command structs
  - [x] Memory management (deinit, ownership pattern)
  - [x] Test infrastructure with sample.zsh_history
  - [ ] Basic line parsing (`: timestamp:duration;command`)
  - [ ] Multi-line command handling
  - [ ] Subsecond timestamp generation
- [ ] `zist collect` command
  - [ ] Parse history files passed as CLI args
  - [ ] Insert into SQLite with deduplication
  - [ ] Handle errors gracefully
- [ ] `zist search` command
  - [ ] SQLite FTS5 queries
  - [ ] Shell out to fzf for UI
  - [ ] Return selected command to shell

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

**ZSH History Parser** (`src/history.zig`)
- Scaffold complete with History/Command structs
- TODO: Implement parse_from_string() logic

---

## Recently Completed

**2026-01-11 (Design Simplification - Final):**
- ✅ Removed INI configuration parser entirely
- ✅ Simplified architecture: CLI args instead of config file
- ✅ Updated main.zig to accept history files as arguments
- ✅ Removed sync references from design document
- ✅ Updated PROJECT_TRACKER.md to reflect local-only architecture
- ✅ Updated AGENTS.md with new design context
- ✅ Build passes, all tests passing
- ✅ Ready to implement history parser

**2026-01-11 (Earlier):**
- ✅ ZSH history parser scaffold
  - History/Command structs with ownership pattern
  - Sample test data (sample.zsh_history)

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

**ZSH History Parser (continue):**
1. Implement parse_from_string() logic
2. Parse `: timestamp:duration;command` format
3. Handle multi-line commands (continuation lines)
4. Enable test assertions

**After history parser:**
5. Add zig-sqlite dependency
6. Create database schema
7. Implement `zist collect` command (save to SQLite)

---

## Resources & References

- Zig SQLite: https://github.com/vrischmann/zig-sqlite
- Zig Language Reference: https://ziglang.org/documentation/master/
- ZSH History Format: Extended history (`: timestamp:duration;command`)
