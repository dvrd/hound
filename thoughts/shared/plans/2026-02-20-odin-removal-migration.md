# Odin Removal & Go Project Restructure — Implementation Plan

**Goal:** Remove all Odin code and make Go the sole implementation at repo root.

**Architecture:** Three sequential phases — move Go to root (`git mv`), delete all Odin artifacts, rewrite project files. Each phase is a single commit. No Go source code changes except the version string in `cmd/hound/main.go`.

**Design:** `thoughts/shared/designs/2026-02-20-odin-removal-migration-design.md`

---

## Dependency Graph

```
Phase 1 (sequential): Move go/* to repo root — single script
Phase 2 (sequential): Delete all Odin code — single script
Phase 3 (parallel):   3.1, 3.2, 3.3, 3.4, 3.5, 3.6 [rewrite project files — independent]
Phase 4 (sequential): Verify everything works — single validation
```

**NOTE:** This is a file restructure, not a feature build. Phases 1 and 2 are `git mv`/`git rm` operations that must be executed as shell scripts. Phase 3 contains independent file rewrites that CAN run in parallel. There are no Go test files to write — the existing Go tests at `internal/` validate correctness after the move.

---

## Phase 1: Move Go to Root

**This phase moves `go/*` contents to repo root using `git mv` for history preservation.**

### Task 1.1: Move Go files to root
**Type:** Shell script (git operations)
**Test:** none (verified by Phase 4)
**Depends:** none

Run these commands sequentially. If `git mv` fails because a destination exists, fall back to `mv` + `git add`.

```bash
#!/bin/bash
set -euo pipefail

# Move Go module files to root
git mv go/go.mod ./go.mod
git mv go/go.sum ./go.sum

# Move Go source directories to root
git mv go/cmd ./cmd
git mv go/internal ./internal

# Remove the now-empty go/ directory
rmdir go

# Verify the move worked — go.mod must be at root
if [ ! -f go.mod ]; then
  echo "FATAL: go.mod not at root after move"
  exit 1
fi

echo "Phase 1 complete: Go files moved to root"
```

**Verify:**
```bash
go build ./...
go test ./... -count=1 -short -timeout 60s
go vet ./...
```

**Commit:** `refactor: move Go source from go/ subdirectory to repo root`

---

## Phase 2: Delete All Odin Code

**This phase removes everything Odin-related. Must run AFTER Phase 1 commit.**

### Task 2.1: Delete Odin artifacts
**Type:** Shell script (git operations)
**Test:** none (verified by Phase 4)
**Depends:** 1.1

```bash
#!/bin/bash
set -euo pipefail

# Delete Odin source tree (113 .odin files + vendored libs)
git rm -rf src/

# Delete Odin test files (29 test files + compiled binaries)
git rm -rf tests/

# Delete Ruby/Python scripts (version bumping, app bundling, icons, hooks)
git rm -rf scripts/

# Delete macOS app resources (icons, Info.plist template)
git rm -rf resources/

# Delete stale documentation
git rm -f DEVELOPMENT.md
git rm -f VERSIONING.md

# Delete misc stale files
git rm -f .env.example
git rm -f .gitmodules

# Delete stale test artifacts (may not be tracked — use rm -f as fallback)
git rm -f test_hyperliquid.db-shm 2>/dev/null || rm -f test_hyperliquid.db-shm
git rm -f test_hyperliquid.db-wal 2>/dev/null || rm -f test_hyperliquid.db-wal

echo "Phase 2 complete: All Odin artifacts removed"
```

**Verify:**
```bash
# Go should still build and pass — we only deleted non-Go files
go build ./...
go test ./... -count=1 -short -timeout 60s
go vet ./...

# Confirm nothing Odin remains
test ! -d src && test ! -d tests && test ! -d scripts && test ! -d resources
echo "Odin cleanup verified"
```

**Commit:** `chore: remove all Odin source, tests, scripts, and resources`

---

## Phase 3: Update Project Files (parallel — 6 implementers)

All tasks in this phase are INDEPENDENT and can run simultaneously. They depend on Phase 2 completing.

### Task 3.1: Rewrite Taskfile.yml
**File:** `Taskfile.yml`
**Test:** none (config file — verified manually)
**Depends:** 2.1

Replace the entire file. Key changes:
- All Odin tasks removed
- Go tasks promoted: `go:build` -> `build`, `go:test` -> `test`, etc.
- `dir: go` directives removed (Go is now at root)
- Binary output: `hound-go` -> `hound`
- Version bumping: Ruby scripts -> shell commands (awk/sed)
- Added: `install`, `test:watch`, `hooks:install`
- ldflags inject version from VERSION file at build time

```yaml
version: '3'

vars:
  VERSION:
    sh: cat VERSION
  LDFLAGS: -s -w -X main.version=v{{.VERSION}}

tasks:
  build:
    desc: Build the hound binary
    cmds:
      - mkdir -p bin
      - go build -ldflags "{{.LDFLAGS}}" -o bin/hound ./cmd/hound

  run:
    desc: 'Build and run with arguments (usage: task run -- wallet status)'
    cmds:
      - task: build
      - ./bin/hound {{.CLI_ARGS}}

  test:
    desc: Run all tests
    cmds:
      - go test ./... -count=1 -short -timeout 60s

  test:verbose:
    desc: Run tests with verbose output
    cmds:
      - go test ./... -count=1 -short -timeout 60s -v

  test:watch:
    desc: Run tests in watch mode (requires entr)
    cmds:
      - find cmd internal -name '*.go' | entr -c go test ./... -count=1 -short -timeout 60s

  lint:
    desc: Run go vet on all packages
    cmds:
      - go vet ./...

  tidy:
    desc: Tidy module dependencies
    cmds:
      - go mod tidy

  clean:
    desc: Remove build artifacts
    cmds:
      - rm -rf bin

  install:
    desc: Install to /usr/local/bin
    cmds:
      - task: build
      - sudo cp bin/hound /usr/local/bin/
      - sudo chmod +x /usr/local/bin/hound

  version:
    desc: Display current version
    cmds:
      - echo "v$(cat VERSION)"

  version:major:
    desc: Bump major version (X.0.0) and create git tag
    cmds:
      - |
        current=$(cat VERSION)
        major=$(echo "$current" | cut -d. -f1)
        new="$((major + 1)).0.0"
        echo "$new" > VERSION
        git add VERSION
        git commit -m "chore: bump version to v${new}"
        git tag "v${new}"
        echo "Bumped to v${new}"

  version:minor:
    desc: Bump minor version (0.X.0) and create git tag
    cmds:
      - |
        current=$(cat VERSION)
        major=$(echo "$current" | cut -d. -f1)
        minor=$(echo "$current" | cut -d. -f2)
        new="${major}.$((minor + 1)).0"
        echo "$new" > VERSION
        git add VERSION
        git commit -m "chore: bump version to v${new}"
        git tag "v${new}"
        echo "Bumped to v${new}"

  version:patch:
    desc: Bump patch version (0.0.X) and create git tag
    cmds:
      - |
        current=$(cat VERSION)
        major=$(echo "$current" | cut -d. -f1)
        minor=$(echo "$current" | cut -d. -f2)
        patch=$(echo "$current" | cut -d. -f3)
        new="${major}.${minor}.$((patch + 1))"
        echo "$new" > VERSION
        git add VERSION
        git commit -m "chore: bump version to v${new}"
        git tag "v${new}"
        echo "Bumped to v${new}"

  hooks:install:
    desc: Install git pre-commit hook
    cmds:
      - cp hooks/pre-commit .git/hooks/pre-commit
      - chmod +x .git/hooks/pre-commit
      - echo "Pre-commit hook installed"
```

**Verify:** `task build && ./bin/hound version`
**Commit:** (included in Phase 3 commit)

---

### Task 3.2: Update cmd/hound/main.go version handling
**File:** `cmd/hound/main.go`
**Test:** none (existing Go tests cover the binary; version is a print statement)
**Depends:** 2.1

Replace the hardcoded version string with an `ldflags`-injectable variable. The `version` var is set at build time via `-X main.version=v0.22.0` from the Taskfile. Falls back to `"dev"` for `go run` without ldflags.

**Change 1:** Add a package-level `version` variable after the imports (before `var jsonOutput bool`):

Find this exact code in `cmd/hound/main.go`:
```go
var jsonOutput bool
```

Replace with:
```go
// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

var jsonOutput bool
```

**Change 2:** Replace the hardcoded version print in `versionCmd()`:

Find this exact code:
```go
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hound v0.22.0-go")
		},
	}
}
```

Replace with:
```go
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hound " + version)
		},
	}
}
```

**Verify:** `go build -ldflags "-X main.version=v0.22.0" -o bin/hound ./cmd/hound && ./bin/hound version`
Expected output: `hound v0.22.0`

**Commit:** (included in Phase 3 commit)

---

### Task 3.3: Rewrite .gitignore
**File:** `.gitignore`
**Test:** none (config file)
**Depends:** 2.1

Replace the entire file. Removes Odin section, macOS app bundle entries, `go/bin/` prefix. Adds standard Go ignores and SQLite test artifact patterns.

```gitignore
# Build outputs
bin/
*.exe

# Go
*.test
__debug_bin*

# SQLite test artifacts
*.db
*.db-shm
*.db-wal

# Logs
*.log

# OS
.DS_Store
Thumbs.db

# Editor
.vscode/
.idea/
*.swp
*~

# Config (user-specific)
.hound/

# Agent/tool directories (not source)
PRPs/
docs/
.logs/
.opencode/
```

**Verify:** `git status` should not show unexpected untracked files
**Commit:** (included in Phase 3 commit)

---

### Task 3.4: Rewrite README.md
**File:** `README.md`
**Test:** none (documentation)
**Depends:** 2.1

Replace the entire file. Key changes:
- Remove all Odin references (prerequisites, build commands, menubar sections)
- Update prerequisites to Go 1.25+ and Task runner
- Binary name: `hound` (not `hound-go`)
- Document `HOUND_RPC_ENDPOINT` env var (replaces `.env.example`)
- Keep wallet compatibility table, CLI commands, security notes

```markdown
# Hound

A Solana toolkit for token price tracking, wallet management, and portfolio monitoring via CLI.

## What Does It Do?

- **Track Token Prices**: Real-time Solana token prices from multiple DEXs
- **Manage Wallets**: Secure wallet import with encrypted storage (compatible with Phantom, Solflare, Ledger)
- **Monitor Portfolio**: View all your holdings and their USD values
- **Swap Tokens**: Execute token swaps via Jupiter aggregator

## Installation

### Prerequisites
- [Go 1.25+](https://go.dev/dl/)
- [Task runner](https://taskfile.dev/): `brew install go-task`

### Build
```bash
git clone https://github.com/dvrd/hound.git
cd hound
task build
```

The binary will be at `./bin/hound`

### Install system-wide
```bash
task install
```

## Quick Start

### 1. Import Your Wallet

```bash
./bin/hound wallet import
```

Follow the prompts to:
- Choose wallet standard (BIP44 for Phantom/Solflare compatibility)
- Enter your 12 or 24-word seed phrase
- Set a password (12+ chars, must include uppercase, lowercase, digit, special character)

Your wallet is encrypted and stored at `~/.config/hound/hound.db`

### 2. Check Your Balance

```bash
./bin/hound wallet status
```

Shows:
- SOL balance
- All SPL token balances
- Current USD values
- 24h price changes

### 3. Track Token Prices

```bash
# Add a token
./bin/hound tokens list

# Check all tracked tokens
./bin/hound tokens list --json
```

## CLI Commands

### Wallet Commands

```bash
# Import wallet
./bin/hound wallet import

# List all wallets
./bin/hound wallet list

# Check balances (detailed view)
./bin/hound wallet status

# Show all tokens including zero balance
./bin/hound wallet status --all

# View specific wallet
./bin/hound wallet status [address|label]
```

### Token Commands

```bash
# List tracked tokens
./bin/hound tokens list
```

### Swap Commands

```bash
# Get swap quote and execute (via TUI)
./bin/hound
# Navigate to swap view from the TUI
```

### History

```bash
# View swap history
./bin/hound history
```

### JSON Output

All list commands support `--json` for scripting:

```bash
./bin/hound wallet list --json
./bin/hound wallet status --json
./bin/hound tokens list --json
./bin/hound history --json
```

### Other Commands

```bash
# Show version
./bin/hound version

# Show help
./bin/hound --help
./bin/hound wallet --help
```

## Development

### Run tests
```bash
task test

# Verbose
task test:verbose

# Watch mode (requires entr)
task test:watch
```

### Lint
```bash
task lint
```

### Version bumping
```bash
task version:patch   # 0.0.X — bug fixes
task version:minor   # 0.X.0 — new features
task version:major   # X.0.0 — breaking changes
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HOUND_RPC_ENDPOINT` | Solana RPC endpoint URL | Public mainnet-beta |

### Database Location
```
~/.config/hound/hound.db
```

Contains encrypted wallets and token configuration.

### Supported Networks
- Solana Mainnet-Beta

### Supported DEXs
- **Orca**: Whirlpool, Concentrated Liquidity
- **Raydium**: AMM V4, CLMM
- **Jupiter**: Aggregator (for swaps)

## Wallet Compatibility

Hound supports industry-standard BIP44 wallets:

| Wallet | Compatible | How to Import |
|--------|-----------|---------------|
| Phantom | Yes | Select "BIP44 Standard" during import |
| Solflare | Yes | Select "BIP44 Standard" during import |
| Ledger | Yes | Select "BIP44 Standard" during import |
| Backpack | Yes | Select "BIP44 Standard" during import |
| Trust Wallet | Yes | Select "BIP44-Change" during import |
| solana-keygen | Yes | Select "Solana CLI" during import |

**Your derived address will match your Phantom/Solflare wallet** when using BIP44 Standard.

## Common Issues

### "Wallet import failed"
Check that:
- Seed phrase is 12 or 24 words
- Password meets requirements (12+ chars with uppercase, lowercase, digit, special)
- `~/.config/hound/` directory has correct permissions

### "Insufficient balance"
Make sure you have:
- Enough input tokens for the swap
- Enough SOL to pay transaction fees (~0.000005 SOL)

## Security Notes

- **Encrypted Storage**: Wallets encrypted with AES-256-GCM + Argon2id
- **Password Requirements**: 12+ characters with uppercase, lowercase, digit, and special character
- **Memory Safety**: Sensitive data is zeroed after use
- **Never Share**: Don't share your seed phrase or password with anyone

**Important**:
- This is alpha software — use at your own risk
- Always verify addresses and amounts before swaps
- Never use publicly known test mnemonics with real funds
- Keep backups of your seed phrases securely offline

---

**Version**: See `VERSION` file

**License**: MIT
```

**Verify:** Visual inspection — no Odin/menubar references remain
**Commit:** (included in Phase 3 commit)

---

### Task 3.5: Update VERSION file
**File:** `VERSION`
**Test:** none
**Depends:** 2.1

Update the version to match the Go port's current version. The Go binary currently prints `v0.22.0-go`. After migration, it should be `v0.22.0` (drop the `-go` suffix since Go is now the only implementation).

```
0.22.0
```

**Verify:** `cat VERSION` outputs `0.22.0`
**Commit:** (included in Phase 3 commit)

---

### Task 3.6: Create pre-commit hook (shell)
**File:** `hooks/pre-commit`
**Test:** none (git hook — verified manually)
**Depends:** 2.1

Create a `hooks/` directory at repo root with a shell pre-commit hook that replaces the Ruby version. Same behavior: warns when committing to master/main without a VERSION file change. No Ruby dependency.

First, create the `hooks/` directory:
```bash
mkdir -p hooks
```

Then write `hooks/pre-commit`:

```bash
#!/bin/sh
# Pre-commit hook: Reminder for version management on master/main branch

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

if [ "$branch" = "master" ] || [ "$branch" = "main" ]; then
  # Check if VERSION is in the staged files
  if ! git diff --cached --name-only | grep -q '^VERSION$'; then
    echo ""
    echo "${YELLOW}════════════════════════════════════════════════${NC}"
    echo "${YELLOW}  Committing to ${branch} without version bump  ${NC}"
    echo "${YELLOW}════════════════════════════════════════════════${NC}"
    echo ""
    echo "${BLUE}Consider bumping version before committing:${NC}"
    echo "  ${GREEN}task version:patch${NC} - Bug fixes (0.0.X)"
    echo "  ${GREEN}task version:minor${NC} - New features (0.X.0)"
    echo "  ${GREEN}task version:major${NC} - Breaking changes (X.0.0)"
    echo ""
    echo "${BLUE}Or abort this commit with:${NC} Ctrl+C"
    echo ""
    printf "Continuing in 3 seconds..."
    sleep 1
    printf " 2..."
    sleep 1
    printf " 1..."
    sleep 1
    echo ""
    echo ""
  fi
fi

exit 0
```

**Verify:** `task hooks:install && cat .git/hooks/pre-commit`
**Commit:** (included in Phase 3 commit)

---

## Phase 4: Final Verification

**This runs AFTER all Phase 3 tasks are committed.**

### Task 4.1: Full verification
**Type:** Shell commands (no file changes)
**Depends:** 3.1, 3.2, 3.3, 3.4, 3.5, 3.6

```bash
#!/bin/bash
set -euo pipefail

echo "=== Build check ==="
go build ./...

echo "=== Test suite ==="
go test ./... -count=1 -short -timeout 60s

echo "=== Static analysis ==="
go vet ./...

echo "=== Binary runs ==="
./bin/hound version

echo "=== Task runner works ==="
task build
task version

echo "=== Odin artifacts gone ==="
test ! -d src
test ! -d tests
test ! -d scripts
test ! -d resources
test ! -f DEVELOPMENT.md
test ! -f VERSIONING.md
test ! -f .env.example
test ! -f .gitmodules
test ! -d go

echo "=== Go files at root ==="
test -f go.mod
test -f go.sum
test -d cmd
test -d internal

echo "=== All checks passed ==="
```

**Commit:** `chore: update project files for Go-only repository`

This commit includes all Phase 3 changes: Taskfile.yml, cmd/hound/main.go, .gitignore, README.md, VERSION, hooks/pre-commit.

---

## Execution Summary

| Phase | Tasks | Type | Commit |
|-------|-------|------|--------|
| 1 | 1.1 | `git mv` operations | `refactor: move Go source from go/ subdirectory to repo root` |
| 2 | 2.1 | `git rm` operations | `chore: remove all Odin source, tests, scripts, and resources` |
| 3 | 3.1-3.6 (parallel) | File rewrites | `chore: update project files for Go-only repository` |
| 4 | 4.1 | Verification only | (no commit) |

**Total commits:** 3
**Total files created/modified:** 6 (Taskfile.yml, main.go, .gitignore, README.md, VERSION, hooks/pre-commit)
**Total files/dirs deleted:** ~160+ files across src/, tests/, scripts/, resources/, plus 4 standalone files

---

## Key Decisions Made

1. **Version injection via ldflags** — Design says "reads version via `-ldflags -X` at build time." I'm implementing this as a `var version = "dev"` package variable in main.go, overridden by `-X main.version=vX.Y.Z` in the Taskfile build command. The `"dev"` fallback ensures `go run ./cmd/hound version` still works without ldflags.

2. **VERSION bumped to 0.22.0** — The Go binary currently prints `v0.22.0-go`. After migration, the `-go` suffix is meaningless (there's no Odin to distinguish from). VERSION file updated from `0.21.0` to `0.22.0` to match.

3. **Pre-commit hook location** — Design says "rewrite as shell script." I'm placing it at `hooks/pre-commit` (not `.git/hooks/`) so it's tracked in version control. The `task hooks:install` command copies it into `.git/hooks/`.

4. **docs/ directory kept in .gitignore** — The `docs/` directory contains wallet standards and FAQ docs that are already gitignored. Keeping the gitignore entry since these are generated/reference docs.

5. **No Go code changes beyond version** — The module path is already `github.com/dvrd/hound` (confirmed in go.mod). All imports use `github.com/dvrd/hound/internal/...` which resolves correctly whether go.mod is at root or in `go/`. Zero import path changes needed.
