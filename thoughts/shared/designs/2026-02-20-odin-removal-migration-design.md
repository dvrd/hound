---
date: 2026-02-20
topic: "Odin Removal & Go Project Restructure"
status: validated
---

# Odin Removal & Go Project Restructure

## Problem Statement

The Go + Bubble Tea port is complete and committed. The repo still contains the
full Odin codebase (~113 .odin files), vendored libraries (http, sqlite3, appkit),
Ruby/Python scripts, macOS menubar app, and Odin-centric project files. The goal
is to make Go the sole implementation — no Odin code remains.

## Constraints

- **Zero Go code changes** — module path is already `github.com/dvrd/hound`, all
  imports are module-relative, runtime paths use `$HOME`
- **Preserve git history** — use `git mv` where possible for traceability
- **Keep VERSION file** — shared versioning mechanism, just replace Ruby bumping
  with shell/Taskfile
- **Keep LICENSE** — unchanged
- **Single commit per logical phase** — clean history

## Approach

Three-phase migration executed sequentially:

### Phase 1: Move Go to Root

Move `go/*` contents to repo root. This makes `go.mod`, `cmd/`, `internal/` top-level.

- `git mv go/go.mod .`
- `git mv go/go.sum .`
- `git mv go/cmd .`
- `git mv go/internal .`
- Remove empty `go/` directory

### Phase 2: Delete All Odin Code

Remove everything that's Odin-only:

- `src/` — entire tree (cli, lib, menubar, appkit, http, sqlite3, version)
- `tests/` — all Odin test files and compiled binaries
- `scripts/` — Ruby/Python scripts (version bumping, app bundling, icons, hooks)
- `resources/` — macOS app icons, Info.plist template
- `DEVELOPMENT.md` — references Odin project structure
- `VERSIONING.md` — references Odin version.odin, stale task names
- `.env.example` — single var, document in README instead
- `.gitmodules` — empty file
- Stale test artifacts: `test_hyperliquid.db-shm`, `test_hyperliquid.db-wal`

### Phase 3: Update Project Files

**Taskfile.yml** — Replace entirely:
- Remove all Odin tasks (build, debug, run, test variants, menubar, hooks)
- Promote Go tasks: `go:build` → `build`, `go:test` → `test`, etc.
- Remove `dir: go` directives (now at root)
- Add version bumping as shell commands (no Ruby dependency)
- Add `install` task for `/usr/local/bin`
- Add `test:watch` using `find` + `entr`
- Keep `clean` task

**README.md** — Rewrite for Go:
- Remove Odin prerequisites, menubar sections
- Update build instructions to `task build`
- Update binary name from `hound-go` to `hound`
- Keep wallet compatibility table, CLI commands, security notes
- Add Go prerequisites (Go 1.25+, Task runner)
- Document `HOUND_RPC_ENDPOINT` env var (replaces .env.example)

**.gitignore** — Clean up:
- Remove `# Odin` section and `.odin/`
- Remove `*.app/`, `*.dmg` (no menubar)
- Keep Go entries (`go/bin/` → just `bin/`), add standard Go ignores
- Add `*.db`, `*.db-shm`, `*.db-wal` for test artifacts

**Version handling**:
- Keep `VERSION` file at root
- Version bumping via Taskfile shell commands (awk/sed) — no Ruby
- Go binary reads version via `-ldflags -X` at build time instead of hardcoded string
- Pre-commit hook: rewrite as simple shell script (no Ruby)

## Components

| Component | Action | Notes |
|-----------|--------|-------|
| `go/*` | Move to root | `git mv` for history |
| `src/` | Delete | 113 .odin files + vendored libs |
| `tests/` | Delete | 29 test files + compiled binaries |
| `scripts/` | Delete | 10 Ruby/Python scripts |
| `resources/` | Delete | macOS app assets |
| `Taskfile.yml` | Rewrite | Go-only tasks, shell version bumping |
| `README.md` | Rewrite | Go-focused documentation |
| `.gitignore` | Update | Remove Odin, add Go patterns |
| `DEVELOPMENT.md` | Delete | Odin-specific |
| `VERSIONING.md` | Delete | Stale references |
| `.env.example` | Delete | Document in README |
| `.gitmodules` | Delete | Empty |
| `VERSION` | Keep | Shared version source |
| `LICENSE` | Keep | Unchanged |

## Data Flow

N/A — this is a file restructure, not a feature.

## Error Handling

- If `git mv` fails (e.g., destination exists), fall back to `mv` + `git add`
- Verify `go build ./...` and `go test ./...` pass after each phase

## Testing Strategy

After each phase:
1. `go build ./...` — compilation check
2. `go test ./... -count=1 -short -timeout 60s` — all 21 packages pass
3. `go vet ./...` — static analysis
4. Verify binary runs: `./bin/hound --help`

## Open Questions

None — all decisions made.
