# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Refactor

- remove 1,778 lines of dead code, stale artifacts, and unused modules
- delete Hyperliquid module (schema, migrations, CRUD — zero consumers)
- delete unused PoolService, ErrorBar, ConfirmModel components
- remove 14 unused error sentinels, ExitCode(), DexType enum
- simplify Router (drop unused rpcClient/getSOLPrice fields)
- consolidate duplicate sparkline rune arrays, unexport duplicate TruncateAddress

### Bug Fixes

- **review-loop**: iteration 1 — 2 findings fixed
- **review-loop**: iteration 1 — 5 findings fixed
- replace relative og:image with absolute URL + screenshot
### Performance

- **portfolio+db+tui**: eliminate N+1 queries, parallelize wallet refresh, cache token lookups
- **tui**: 9× faster injectBorderTitle, bypass footer Render, clean up dead fields
- **tui**: 9× faster injectBorderTitle, bypass footer style, eliminate concat allocs
- **tui**: single-alloc View() — cache sort/scroll, eliminate string+newline concats
- **tui**: pre-alloc Builder, cache selected row, eliminate all per-frame allocs
- **tui**: zero-alloc RenderRow for non-selected, cached title header
- **tui**: bypass lipgloss for non-selected rows, cache App styles, cache history rows
- **tui**: cache per-row styled content — eliminates 80 lipgloss.Render calls/frame
- **tui**: eliminate per-frame style allocs, cache separators and footers
- **tui+price**: cache visibleTokens, hoist styles, bound price fetcher concurrency
- **tui+tx**: PadLeft/PadRight eliminate fmt.Sprintf in render loops, pre-alloc Transaction.Serialize
- **tx+dex+activity**: zero-alloc compact-u16, RWMutex caches, targeted swap lookup
- **wallet+tui**: strconv for number formatting, PadRight, cached table header
- **wallet+tx+db**: parallel walletlist refresh, pre-alloc serialize, batch balance writes
## [1.1.0] — 2026-02-28

### Features

- **swapview**: add Jupiter token symbol search overlay
- **tokenfetch**: add j/k scroll with sliding window and ↕ indicator
- **transaction**: add ComputeBudget priority fee instructions
### Bug Fixes

- **history**: fetch all swap records to fix merge across pages
- **rpc**: handle HTTP 429 with Retry-After backoff before endpoint rotation
- **send**: remove $1 USD minimum filter from token list
- **tokenlist**: j/k navigate cursor in saved-tokens mode without leaking into search
- **wallet**: deduplicate concurrent RefreshPortfolio calls per address
## [1.0.0] — 2026-02-27

### Features

- history pagination with Helius RPC, page cache, and landing update
- wire help modal in walletlist; hide i/d/r from footer, show in ? overlay
- filter tokens below $1 USD value from send token list
- migrate send/swap/walletstatus to FooterProvider; add enter→token-detail in walletstatus
- add sigil favicon to landing
- replace menu with wallets (m/w), swap moves to x
- simplify footers to nav+help, h=history, </> hide/unhide tokens
- UI polish — accent bar cursor, table separators, cyan value columns, compact footer, border title, floating notifications
- interactive menu, global q quit, persist last view, 2-state filter toggle, remove receive view
- hide tokens and dust filter in wallet status
### Bug Fixes

- history footer s→status (not send); wire w/s/x/t nav keys; add TestFooterExactBindings and TestNavKeys
- complete walletlist footer (add i/d/r/S/enter keys); fix send focusCurrentStep value-receiver bug; add TestFooterExactBindings for walletlist
- swapview focusCurrent returns (Model, tea.Cmd) to preserve blur/focus mutations; add tab-cycle tests
- remove Width from footerStyle to prevent footer truncation; add TestFooterExactBindings
- replace m with w in walletstatus footer, update stale test assertions
- transparent favicon background
- remove menu from restorable views — only wallet-status restores on launch
- only save state for restorable views (wallet-status, menu)
- history shows all rows and j/k navigation works
- add h key for back navigation in history and tokenlist views
- always do live fetch on Init instead of relying on stale cache
- show tokens on first load and display wallet name in header
- keys work during refresh, aligned walletlist table, skip list with 1 wallet
- ANSI-safe table column alignment and tokenfetch height cap
### Refactor

- FilterMode enum, clipboard copy, token-list key in walletstatus
## [0.23.1] — 2026-02-25

### Bug Fixes

- resample panic when n=1 (division by zero in sparkline)
## [0.23.0] — 2026-02-25

### Features

- user-configurable slippage, min received from Jupiter, swap review completeness
- price history sparkline, swap route display, preload error handling
- remove thoughts directory
- pinned footer via FooterProvider interface + portfolio preloading at startup
- add token full name to walletstatus and balances DB
- make TUI responsive to terminal size — proportional columns, capped rows, adaptive inputs
- apply Batch 4 'Make It Complete' — all 14 remaining audit fixes
- wire send/receive/activity into ViewFactory, add integration tests (Phase 10)
- add send/receive views, activity service, wallet generate, rename, auto-refresh (Phases 4-9)
- add transaction builder, new RPC methods, and transfer service (Phases 1-3)
- complete Go + Bubble Tea port of Hound
- implement comprehensive token information fetching
### Bug Fixes

- correct inaccurate landing page claims from accuracy audit
- tech debt — shared format helpers, partial error surfacing, reuse httpClient, populate wallet in New()
- keep portfolio visible during background refresh
- load portfolio on enter and resolve token names from Jupiter
- force full repaint on terminal resize with tea.ClearScreen
- use inner dimensions in App.View() to prevent 4-row overflow
- validate password strength at entry step, not at import time
- apply Batch 1 'Make It Work' — 8 critical/high bug fixes
- migrate Odin swap_history schema (timestamp→created_at)
### Performance

- apply Batch 3 'Make It Fast' — 5 performance fixes across 19 files
### Refactor

- remove Odin compatibility migrations, reset database
- move Go source from go/ subdirectory to repo root
- restructure CLI to use 'hound tokens' command hierarchy [**BREAKING**]
### Security

- apply Batch 2 'Make It Safe' — 6 security fixes, dual-salt key derivation
## [0.20.7] — 2025-11-26

### Features

- add Meteora DLMM support to DEX router
- show full formatted JSON in all debug and error logs
- add pretty-printed JSON logging for execute endpoint
- add pretty-printed JSON logging for Jupiter responses
- display Jupiter API error message directly to user
### Bug Fixes

- wallet status showing wrong type (Legacy vs BIP44_Standard)
- dynamic column width for wallet list Status column
- correct Meteora DLMM decoder offsets and prevent overflow
- correct execute API field name to signedTransaction
- use json.marshal for execute request body to ensure proper escaping
- critical memory corruption in jupiter_response string
- improve swap transaction error handling and messages
- resolve memory test race conditions and segfaults
## [0.20.6] — 2025-11-25

### Refactor

- complete arena memory migration - CLI, output, wallet, router, menubar (Phase 3)
## [0.20.5] — 2025-11-25

### Refactor

- implement request arena for service layer and HTTP client (Phase 2)
## [0.20.4] — 2025-11-25

### Refactor

- implement secure arena memory management for cryptography (Phase 1)
## [0.20.3] — 2025-11-25

### Features

- add --compact flag to wallet list command
## [0.20.2] — 2025-11-25

### Features

- add wallet delete command and fix memory arena
## [0.20.1] — 2025-11-25

### Features

- implement BIP39/BIP32/BIP44 cryptographic foundations (v0.20.0)
## [0.20.0] — 2025-11-25

### Bug Fixes

- resolve nil slice assertions in wallet commands
## [0.19.1] — 2025-11-24

### Features

- add wallet update-password command
- add HTTP client retry logic and enhanced logging
- add configurable RPC endpoint support for faster performance
- add wallet subcommands (list, status, switch, help)
- implement Phase 3 - transaction execution for swaps (v0.18.0)
### Bug Fixes

- improve error handling for insufficient balance
- correct AES-GCM authentication tag storage
- sync encrypted_keypairs to wallets table on import
- hide tokens with no price in wallet status
- improve error messages and memory management in wallet import
- migrate menubar swap to Jupiter Ultra API (lite-api.jup.ag)
### Refactor

- migrate all build scripts from Bash to Ruby
- migrate git hooks to Ruby with auto-bump on master
## [0.17.1] — 2025-11-21

### Bug Fixes

- logging output not being shown
### Refactor

- migrate version management to Ruby with semver bumping
## [0.17.0] — 2025-11-21

### Features

- Phase 2 - Swap Quote & Dry-Run (v0.17.0)
## [0.16.0] — 2025-11-21

### Features

- Phase 1 - Secure Keystore Implementation (v0.16.0)
- add real 24h price change tracking to wallet command
## [0.15.0] — 2025-11-21

### Features

- add Phase 3 wallet command enhancements (v0.15.0)
## [0.14.0] — 2025-11-21

### Features

- add wallet command subcommands and address parameter (v0.14.0)
## [0.13.0] — 2025-11-21

### Features

- implement wallet command to display portfolio balances
### Refactor

- consolidate menubar_main into menubar package
- implement Phase 4 backend-frontend separation for MenuBar
## [0.11.3] — 2025-11-20

### Refactor

- move config, swap, transaction to lib/ and vendor to src/
## [0.11.2] — 2025-11-20

### Refactor

- consolidate all wallet logic into src/lib/wallet/
- update imports for renamed directories
- rename src/jupiter_swap/ to src/swap/
- rename src/wallet_manager/ to src/wallet/
- rename src/token_config/ to src/config/
- update Taskfile.yml CLI build paths
- update imports in src/cli/ after move (9 files)
- move cli/ to src/cli/ (10 files)
- update core/ → src/lib/ imports in src/ (10 files)
- update core/ → src/lib/ imports in cli/ (8 files)
- update vendor imports in src/lib/ (8 files)
- move core/ to src/lib/ (22 files)
- update error messages to reflect SQLite database instead of JSON
## [0.10.1] — 2025-11-20

### Bug Fixes

- update version script to generate in correct package location
## [0.11.1] — 2025-11-20

### Bug Fixes

- update service_ctx pointers after wallet manager struct copy
## [0.11.0] — 2025-11-20

### Features

- add version to help message
### Bug Fixes

- correct version package structure and update build paths
### Refactor

- complete Phase 4 MenuBar backend-frontend separation
- change version from flag to command
## [0.10.0] — 2025-11-20

### Features

- implement Phase 2 service layer architecture
## [0.9.0] — 2025-11-20

### Refactor

- create core/ package and standardize documentation
## [0.8.2] — 2025-11-19

### Bug Fixes

- remove white background from wolf icon for transparency
## [0.8.1] — 2025-11-19

### Features

- update app icon to black wolf with red eyes
## [0.8.0] — 2025-11-19

### Features

- implement Phase 3 database/wallet arena migration and fix memory leaks
- implement Phase 1 arena foundation and Phase 2 RPC migration
## [0.7.1] — 2025-11-19

### Features

- add Phase 2 - Jupiter swap transaction building and export
### Bug Fixes

- use fmt.aprintf instead of fmt.tprintf to avoid memory corruption
- migrate token auto-discovery to Jupiter Token API V2
## [0.7.0] — 2025-11-19

### Features

- implement Orca Whirlpool CLMM on-chain price fetching
## [0.6.0] — 2025-11-19

### Features

- implement automatic token metadata discovery via Jupiter Token List API
- add 'hound add' command for easy token management
- implement Phase 5.3 CLI enhancements (Tasks 7-11)
- implement Phase 5.3 pool metadata infrastructure (Tasks 1-6)
- integrate wallet portfolio tracking into menubar app
- implement Phase 1 watch-only wallet foundation and menubar app
- implement Phase 4.4 Multi-DEX support with priority-based routing
- add comprehensive logging with core:log throughout application
- implement Phase 4.3 hybrid 24-hour price change tracking
- add pre-commit hook for automatic version synchronization
- add semantic versioning system with automated management
- implement Phase 4.2 SOL price oracle with 30-second caching and menu bar infrastructure
- add comprehensive test suite and TigerBeetle-inspired development philosophy
- implement Phase 4.1 on-chain pricing infrastructure (Raydium AMM MVP)
- implement Phase 3 multi-token support via JSON configuration
- implement Phase 2 error handling with granular error types
- implement Hound CLI MVP - Solana token price fetcher
### Bug Fixes

- resolve use-after-free bug causing RPC connection failures in menubar app
- defer initial network request to prevent menubar app crash
- prevent pool duplicates on refresh
- correct AppKit bindings to use @(objc_class) attribute pattern
- implement native DNS resolution for macOS using getaddrinfo()
### Refactor

- rename tokens.db to hound.db for broader scope
- improve logging verbosity and clarity

