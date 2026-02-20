---
date: 2026-02-20
topic: "Full Port: Odin → Go + Bubble Tea TUI"
status: validated
---

# Hound: Odin → Go + Bubble Tea Full Port

## Problem Statement

Hound is a Solana wallet management and accounting tool written in Odin (~100+ source files, two build targets: CLI + macOS menubar). We're porting the entire application to **Go with Bubble Tea** as an interactive TUI.

**Primary use case:** Store wallets and view portfolio balances for accounting purposes.

**Why port:**
- Odin ecosystem is limited for long-term maintenance
- Bubble Tea provides a rich interactive TUI experience
- Go has mature crypto libraries and better cross-platform support
- Drop macOS menubar — consolidate to one TUI binary

**Key constraint:** The SQLite database at `~/.config/hound/hound.db` must remain compatible. Same schema, same encryption parameters. Users can migrate seamlessly.

## Constraints

- Database backward compatibility with existing Odin `hound.db` files
- Same Argon2id parameters (19MB, 2 iterations, 1 thread) for keypair encryption
- Same BIP39/BIP32/SLIP-0010 derivation paths
- Same AES-256-GCM nonce (12 bytes) and tag (16 bytes) sizes
- No CGO — pure Go dependencies only (modernc.org/sqlite, not mattn/go-sqlite3)
- Sensitive data (seeds, passwords, private keys) must be zeroed after use

## Priority Ordering

1. **Wallet storage** — import, list, switch, delete (encrypted BIP39/BIP44 keypairs)
2. **Portfolio viewing** — balance fetching, USD valuation, multi-wallet aggregation
3. **Token management** — add/list tracked tokens, price fetching
4. **Swap execution** — quote, confirm, execute via Jupiter Ultra
5. **History** — swap transaction log

## Approach

Layered architecture mirroring the Odin codebase's clean separation, with Go idioms:

- **Cobra** for CLI entry point and non-interactive mode (`--json`)
- **Bubble Tea** for all interactive TUI flows
- **Pure Go business logic** with no TUI dependency — testable and reusable
- **Same SQLite schema** — zero migration needed from Odin version

Alternatives considered:
- **Tview** — rejected: less composable, imperative API doesn't match the Odin state/handler/view pattern
- **CLI-only (no TUI)** — rejected: the accounting use case benefits from interactive navigation between wallets

## Architecture

```
cmd/hound/main.go              ← Entry point (Cobra → Bubble Tea)
├── internal/tui/               ← Bubble Tea models (all UI)
│   ├── app.go                  ← Root model + view router
│   ├── wallet/                 ← Wallet views
│   │   ├── list.go             ← Home screen: wallet table with cached balances
│   │   ├── import.go           ← Multi-step import wizard
│   │   ├── status.go           ← Portfolio table with live refresh
│   │   └── delete.go           ← Deletion confirmation flow
│   ├── tokens/                 ← Token views
│   │   ├── list.go             ← Token table with pool stats
│   │   ├── fetch.go            ← Extended token info
│   │   └── add.go              ← Add token form
│   ├── swap/                   ← Swap flow
│   │   └── swap.go             ← Quote → confirm → execute
│   ├── history/                ← History view
│   │   └── history.go          ← Swap history table
│   └── components/             ← Shared TUI components
│       ├── table.go            ← Styled table wrapper
│       ├── password.go         ← Masked password input with strength indicator
│       ├── confirm.go          ← Yes/No confirmation dialog
│       ├── spinner.go          ← Loading spinner with message
│       ├── error.go            ← Error bar component
│       └── help.go             ← Help overlay
├── internal/database/          ← SQLite layer
│   ├── database.go             ← Open, close, migrate, pragmas
│   ├── tokens.go               ← Token CRUD
│   ├── wallets.go              ← Wallet CRUD
│   ├── balances.go             ← Balance CRUD
│   ├── keypairs.go             ← Encrypted keypair CRUD
│   ├── pools.go                ← Pool CRUD
│   ├── swap_history.go         ← Swap history CRUD
│   └── hyperliquid.go          ← Hyperliquid wallet CRUD
├── internal/wallet/            ← Wallet operations
│   ├── manager.go              ← WalletManager (orchestrates portfolio refresh)
│   ├── balance.go              ← Balance fetching + portfolio assembly
│   └── operations.go           ← Add, validate, aggregate portfolios
├── internal/keystore/          ← Cryptographic operations
│   ├── bip39.go                ← Mnemonic → 64-byte seed (PBKDF2-HMAC-SHA512)
│   ├── bip32.go                ← SLIP-0010 Ed25519 HD key derivation
│   ├── keypair.go              ← Ed25519 keypair derivation + Base58 address
│   ├── aes.go                  ← AES-256-GCM encrypt/decrypt
│   ├── argon2.go               ← Argon2id key derivation + password hashing
│   └── secure.go               ← Secure memory zeroing utilities
├── internal/blockchain/        ← Solana RPC client
│   ├── rpc.go                  ← JSON-RPC client with endpoint failover
│   ├── solana.go               ← getAccountInfo, getBalance, token accounts
│   └── oracle.go               ← SOL/USD price (Jupiter → CoinGecko fallback)
├── internal/dex/               ← DEX integrations
│   ├── dexscreener.go          ← DexScreener API client
│   ├── jupiter.go              ← Jupiter Price API v3
│   ├── router.go               ← Multi-DEX price routing
│   ├── pool.go                 ← Pool discovery, filtering, ranking
│   └── decoders/               ← On-chain pool decoders
│       ├── orca.go             ← Orca Whirlpool (sqrt_price Q64.64)
│       ├── raydium_amm.go      ← Raydium AMM V4 (reserve ratio)
│       ├── raydium_clmm.go     ← Raydium CLMM
│       └── meteora.go          ← Meteora DLMM (bin pricing)
├── internal/swap/              ← Swap execution
│   ├── client.go               ← Jupiter Ultra API (quote + execute)
│   ├── transaction.go          ← Transaction signing (Ed25519)
│   └── cache.go                ← Quote cache (90s TTL)
├── internal/services/          ← Business logic facades
│   ├── price.go                ← Price fetching with fallback chain
│   ├── pool.go                 ← Pool discovery + persistence
│   ├── token_info.go           ← Extended token info aggregation
│   ├── keystore.go             ← Import/unlock/update keypair workflows
│   └── swap.go                 ← Swap orchestration
├── internal/models/            ← Shared types
│   ├── wallet.go               ← Wallet, WalletType, TokenBalance, PortfolioBalance
│   ├── token.go                ← Token, PoolInfo, TokenExtendedInfo
│   ├── swap.go                 ← SwapQuote, SwapTransactionResult, RouteStep
│   └── errors.go               ← Sentinel errors + custom error types
└── internal/config/            ← Configuration
    └── config.go               ← Database path, RPC endpoint, app settings
```

## Components

### Root App Model (`internal/tui/app.go`)

The root Bubble Tea model that:
- Holds the current active view (one of the sub-models)
- Handles global keybindings: `q`/`ctrl+c` quit, `esc` back, `?` help overlay
- Routes navigation messages between views
- Holds shared dependencies (database handle, wallet manager, config)

### Wallet List (Home Screen) — `internal/tui/wallet/list.go`

Default view on startup. Shows all wallets with cached USD totals from the `balances` table.

**Data source:** `database.GetAllWallets()` + `database.GetBalancesForWallet()` (cached, no network)

**Keybindings:**
- `Enter` / `s` → navigate to wallet status (live portfolio)
- `i` → wallet import wizard
- `d` → delete wallet
- `w` → switch primary wallet
- `t` → token list
- `h` → swap history
- `a` → aggregated view (all wallets combined)

### Wallet Import Wizard — `internal/tui/wallet/import.go`

Multi-step state machine:

| Step | Input | Component |
|------|-------|-----------|
| 1. Seed phrase | Text area (12 or 24 words) | `bubbles/textarea` |
| 2. Wallet type | Selection list (4 options) | `bubbles/list` |
| 3. Account index | Numeric input (default 0) | `bubbles/textinput` |
| 4. Address preview | Derived address + confirm/reject | Custom confirm dialog |
| 5. Password | Masked input + strength bar + confirmation | Custom password component |
| 6. Label | Text input (default "Imported Wallet") | `bubbles/textinput` |
| 7. Success | Summary display | Static view |

`Esc` goes back one step. Sensitive buffers zeroed on exit via `defer`.

### Wallet Status (Portfolio) — `internal/tui/wallet/status.go`

Live portfolio view with balance table.

**Data flow:**
1. On enter: show cached balances immediately (from `balances` table)
2. Trigger async refresh via `tea.Cmd` → `wallet.FetchPortfolioBalance()`
3. On response: update table, persist to DB

**Table columns:** Token, Balance, Price, Value (USD), 24h Change

**Keybindings:**
- `r` → force refresh (with spinner)
- `a` → toggle all tokens (including zero balance)
- `j` → dump JSON to stdout (for piping)
- `1`/`2`/`3` → sort by value/symbol/balance

### Token List — `internal/tui/tokens/list.go`

Table of tracked tokens with pool stats.

**Columns:** Symbol, Name, Pools, Liquidity, Auto-discovered indicator

### Swap Flow — `internal/tui/swap/swap.go`

Multi-phase state machine:
1. **Input** — from token, to token, amount (3 text inputs)
2. **Quoting** — spinner while fetching Jupiter Ultra quote
3. **Review** — styled quote display with price impact warning
4. **Password** — unlock wallet for signing
5. **Executing** — spinner while signing + submitting
6. **Result** — success/failure with transaction signature + Solscan link

### History — `internal/tui/history/history.go`

Paginated table of swap history from `swap_history` table.

**Columns:** Date, Trade (FROM → TO), Rate, Status, DEX

### Shared Components

**Password Input** (`components/password.go`):
- Masked input (dots)
- Real-time strength indicator (weak/fair/strong/very strong)
- Confirmation step (enter twice)
- Uses `golang.org/x/term` for raw mode

**Styled Table** (`components/table.go`):
- Wraps `bubbles/table` with lipgloss styling
- Dynamic column widths based on content
- Smart number formatting (same precision rules as Odin: ≥1000→2dp, ≥1→4dp, ≥0.01→6dp, <0.01→8dp)

**Error Bar** (`components/error.go`):
- Renders at bottom of view
- Red background, white text
- Auto-dismisses after 5 seconds or on any keypress
- Shows actionable error messages (same catalog as Odin version)

## Data Flow

### Wallet Import
```
User input → WalletImportModel.Update()
  → keystore.DeriveKeypairBIP44(seed, walletType, accountIndex)
  → keystore.DeriveKey(password, salt)  // Argon2id
  → keystore.Encrypt(privateKey, aesKey)  // AES-256-GCM
  → database.InsertEncryptedKeypair(...)
  → database.InsertWallet(...)
  → WalletImportModel.View() shows success
```

### Portfolio Refresh
```
User presses 'r' → WalletStatusModel.Update()
  → tea.Cmd: wallet.FetchPortfolioBalance(address)
    → blockchain.GetBalance(address)  // SOL lamports
    → blockchain.GetTokenAccountsByOwner(address)  // SPL tokens
    → For each token:
      → dex.Router.FetchPrice(token)  // on-chain → DexScreener → Jupiter fallback
    → Assemble PortfolioBalance
  → WalletStatusModel.Update() receives result
  → database.UpdateBalance(...) for each token
  → WalletStatusModel.View() renders table
```

### Swap Execution
```
User confirms swap → SwapModel.Update()
  → tea.Cmd: swap.GetQuote(inputMint, outputMint, amount, taker)
    → Jupiter Ultra API: GET /ultra/v1/order
  → SwapModel.Update() receives quote, renders review
  → User confirms → password input
  → tea.Cmd: keystore.Unlock(address, password)
    → database.GetEncryptedKeypair(address)
    → keystore.DeriveKey(password, storedSalt)
    → keystore.Decrypt(ciphertext, aesKey)
  → tea.Cmd: swap.Execute(signedTx, requestId)
    → Sign transaction (Ed25519)
    → Jupiter Ultra API: POST /ultra/v1/execute
  → database.InsertSwapHistory(...)
  → SwapModel.View() renders result
```

## Error Handling

### Error Types (Go idiomatic)

Sentinel errors for known conditions:
- `ErrWalletNotFound`, `ErrWalletAlreadyExists`
- `ErrWeakPassword`, `ErrInvalidSeedPhrase`, `ErrCryptoFailed`
- `ErrQuoteExpired`, `ErrHighPriceImpact`, `ErrInsufficientBalance`
- `ErrRPCConnectionFailed`, `ErrRPCInvalidResponse`
- `ErrDatabaseCorrupted`, `ErrMigrationFailed`
- `ErrNetworkTimeout`, `ErrRateLimited`
- `ErrTokenNotFound`, `ErrNoPoolsFound`

Wrapped errors with context: `fmt.Errorf("fetching balance for %s: %w", addr, err)`

### TUI Error Display

Errors from `tea.Cmd` results bubble up to the active model's `Update`. The model sets an error state that renders via the shared `ErrorBar` component. Each error maps to the same user-facing message catalog from the Odin version (actionable, with suggestions).

### Non-Interactive Errors

When running with `--json` flag (Cobra bypasses Bubble Tea), errors go to stderr as structured JSON: `{"error": "wallet_not_found", "message": "...", "exit_code": 1}`.

Exit codes preserved from Odin:
- `0` — Success
- `1` — General/user error
- `2` — Usage error
- `69` — Service unavailable
- `70` — Internal error
- `74` — I/O/database error
- `78` — Configuration error

## Database Schema

Identical to Odin version. Tables:

### `tokens`
| Column | Type | Constraints |
|--------|------|-------------|
| id | INTEGER | PRIMARY KEY AUTOINCREMENT |
| symbol | TEXT | NOT NULL UNIQUE COLLATE NOCASE |
| name | TEXT | NOT NULL |
| contract_address | TEXT | NOT NULL UNIQUE |
| chain | TEXT | NOT NULL DEFAULT 'solana' |
| is_quote_token | INTEGER | DEFAULT 0 |
| usd_price | REAL | DEFAULT 0.0 |
| discovered_at | INTEGER | NOT NULL |
| last_updated | INTEGER | NOT NULL |
| cache_ttl | INTEGER | DEFAULT 86400 |

### `pools`
| Column | Type | Constraints |
|--------|------|-------------|
| id | INTEGER | PRIMARY KEY AUTOINCREMENT |
| token_id | INTEGER | FK → tokens(id) ON DELETE CASCADE |
| dex | TEXT | NOT NULL |
| pool_address | TEXT | NOT NULL |
| quote_token | TEXT | NOT NULL |
| pool_type | TEXT | NOT NULL |
| liquidity_usd | REAL | DEFAULT 0.0 |
| volume_24h | REAL | DEFAULT 0.0 |
| fee_percent | REAL | DEFAULT 0.0 |
| discovered_at | INTEGER | DEFAULT 0 |

### `wallets`
| Column | Type | Constraints |
|--------|------|-------------|
| address | TEXT | PRIMARY KEY |
| label | TEXT | NOT NULL |
| is_primary | INTEGER | DEFAULT 0 |
| added_at | INTEGER | NOT NULL |
| wallet_type | TEXT | DEFAULT 'Legacy' |
| derivation_path | TEXT | DEFAULT 'legacy-sha256' |
| account_index | INTEGER | DEFAULT 0 |

### `balances`
| Column | Type | Constraints |
|--------|------|-------------|
| wallet_address | TEXT | FK → wallets(address) ON DELETE CASCADE |
| mint | TEXT | NOT NULL |
| symbol | TEXT | nullable |
| amount | REAL | NOT NULL |
| usd_price | REAL | NOT NULL |
| usd_value | REAL | NOT NULL |
| updated_at | INTEGER | NOT NULL |
PK: (wallet_address, mint)

### `encrypted_keypairs`
| Column | Type | Constraints |
|--------|------|-------------|
| address | TEXT | PRIMARY KEY |
| encrypted_private_key | BLOB | NOT NULL |
| salt | BLOB | NOT NULL (16 bytes) |
| nonce | BLOB | NOT NULL (12 bytes) |
| tag | BLOB | NOT NULL (16 bytes) |
| password_hash | BLOB | NOT NULL (32 bytes) |
| label | TEXT | NOT NULL |
| is_primary | INTEGER | DEFAULT 0 |
| created_at | INTEGER | NOT NULL |
| last_used | INTEGER | nullable |

### `hyperliquid_wallets`
| Column | Type | Constraints |
|--------|------|-------------|
| address | TEXT | PRIMARY KEY |
| label | TEXT | NOT NULL UNIQUE |
| api_wallet_name | TEXT | NOT NULL |
| encrypted_api_key | BLOB | NOT NULL |
| encrypted_api_secret | BLOB | NOT NULL |
| salt | BLOB | NOT NULL (16 bytes) |
| nonce_key | BLOB | NOT NULL (12 bytes) |
| nonce_secret | BLOB | NOT NULL (12 bytes) |
| tag_key | BLOB | NOT NULL (16 bytes) |
| tag_secret | BLOB | NOT NULL (16 bytes) |
| password_hash | BLOB | NOT NULL (32 bytes) |
| is_active | INTEGER | DEFAULT 0 |
| created_at | INTEGER | NOT NULL |
| last_used | INTEGER | nullable |

### `swap_history`
(Ported from `src/lib/database/swap_history.odin`)

PRAGMAs: `foreign_keys=ON`, `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`

## Keystore Implementation Details

### BIP39: Mnemonic → Seed
- Input: 12 or 24 word mnemonic
- Salt: `"mnemonic"` (no passphrase)
- Algorithm: PBKDF2-HMAC-SHA512, 2048 iterations
- Output: 64-byte seed
- Go: `github.com/tyler-smith/go-bip39` handles this natively

### BIP32/SLIP-0010: Seed → Ed25519 Keypair
- Master key: `HMAC-SHA512(key="ed25519 seed", data=seed)` → left 32 = key, right 32 = chain code
- Child derivation (hardened only): `HMAC-SHA512(key=parent_chain_code, data=0x00||parent_key||index_be32)`
- Paths: `m/44'/501'/0'/0'` (BIP44_Standard), `m/44'/501'/0'` (BIP44_Change), `m/44'/501'` (Solana CLI)
- Legacy: `SHA-256(seed_phrase_string)` → 32 bytes → Ed25519 seed
- Go: Custom ~120 line implementation (no suitable library for Ed25519 SLIP-0010)

### Argon2id: Password → AES Key
- Memory: 19,456 KB (19 MB)
- Iterations: 2
- Parallelism: 1
- Salt: 16 bytes (crypto/rand)
- Output: 32 bytes (AES-256 key)
- Go: `argon2.IDKey(password, salt, 2, 19456, 1, 32)`

### AES-256-GCM: Encrypt/Decrypt Private Key
- Key: 32 bytes (from Argon2id)
- Nonce: 12 bytes (crypto/rand)
- Tag: 16 bytes (appended by Go's GCM implementation)
- Go: `crypto/aes` + `crypto/cipher` (GCM mode)
- Note: Go's `cipher.AEAD.Seal()` appends the tag to ciphertext. The Odin version stores them separately. The Go port must split/join accordingly for DB compatibility.

### Password Hash (stored for verification)
- Same Argon2id with same parameters
- 32-byte output stored in `encrypted_keypairs.password_hash`
- On unlock: re-derive and compare byte-by-byte before attempting decryption

## External API Reference

| Service | Base URL | Endpoints | Cache TTL |
|---------|----------|-----------|-----------|
| Solana RPC | configurable (default: `https://api.mainnet-beta.solana.com`) | `getBalance`, `getTokenAccountsByOwner`, `getAccountInfo`, `getTokenAccountBalance`, `getTokenSupply`, `getTokenLargestAccounts` | None |
| DexScreener | `https://api.dexscreener.com` | `GET /latest/dex/tokens/{address}` | 5min (change), 1hr (pools) |
| Jupiter Price | `https://lite-api.jup.ag` | `GET /price/v3?ids={mint}` | 60s |
| Jupiter Ultra | `https://lite-api.jup.ag` | `GET /ultra/v1/order?...`, `POST /ultra/v1/execute` | 90s (quotes) |
| Jupiter Token | `https://lite-api.jup.ag` | `GET /tokens/v2/search?query={mint}` | None |
| CoinGecko | `https://api.coingecko.com` | `GET /api/v3/simple/price?ids=solana&vs_currencies=usd` | 30s |

## Non-Interactive Mode

Cobra handles `--json` flags. When passed, commands bypass Bubble Tea and output JSON to stdout:
- `hound wallet status --json` → portfolio JSON
- `hound wallet list --json` → wallet list JSON
- `hound tokens list --json` → token list JSON

This preserves pipe-friendly behavior: `hound wallet status --json | jq '.total_value_usd'`

## Go Dependencies

```
github.com/charmbracelet/bubbletea         # TUI framework
github.com/charmbracelet/lipgloss          # Styling
github.com/charmbracelet/bubbles           # Table, text input, spinner, viewport, list
github.com/spf13/cobra                     # CLI entry point
modernc.org/sqlite                         # Pure Go SQLite (no CGO)
golang.org/x/crypto                        # Argon2id, PBKDF2
golang.org/x/term                          # Terminal raw mode
github.com/tyler-smith/go-bip39            # BIP39 mnemonic → seed
github.com/mr-tron/base58                  # Solana Base58 encoding
crypto/ed25519                             # Ed25519 (stdlib)
crypto/aes + crypto/cipher                 # AES-256-GCM (stdlib)
crypto/rand                                # Secure random (stdlib)
crypto/hmac + crypto/sha512                # HMAC-SHA512 for SLIP-0010 (stdlib)
crypto/sha256                              # SHA-256 for legacy derivation (stdlib)
```

## Testing Strategy

| Layer | Approach | Tools |
|-------|----------|-------|
| Database | Integration tests with in-memory SQLite | `testing` + `modernc.org/sqlite` |
| Keystore | BIP39/BIP32 test vectors from specs | `testing` with known vectors |
| Keystore | Cross-compatibility: decrypt Odin-encrypted keypairs | Golden file tests |
| RPC Client | Mock HTTP server with recorded responses | `net/http/httptest` |
| DexScreener/Jupiter | Mock HTTP server | `net/http/httptest` |
| DEX Decoders | Known pool account bytes → expected prices | Table-driven tests |
| TUI Models | Programmatic Update/View testing | `bubbletea` test helpers |
| Wallet Operations | Integration tests with mock RPC | Dependency injection |
| End-to-end | Full import → status flow | In-memory DB + mock RPC |

## What We're Dropping

- macOS menubar app (`src/menubar/`)
- AppKit/ObjC bindings (`src/appkit/`)
- Custom 4-arena memory allocator (`src/lib/memory/`)
- Vendored HTTP client (`src/http/`)
- Vendored SQLite bindings (`src/sqlite3/`)
- libsodium FFI dependency

## Open Questions

1. **Swap history table schema** — need to verify exact columns from `src/lib/database/swap_history.odin` (partially analyzed)
2. **go-bip39 word list compatibility** — verify the Go library uses the same BIP39 English word list as the Odin implementation
3. **AES-GCM tag storage** — Go's `cipher.AEAD.Seal()` appends tag to ciphertext; Odin stores them separately. Need adapter logic to split/join for DB compatibility.

## Implementation Phases

### Phase 1: Foundation (3-4 days)
- Go module setup, project structure
- Models package (all types)
- Database layer (schema, migrations, all CRUD)
- Config (database path, RPC endpoint)
- Database compatibility test (read existing Odin DB)

### Phase 2: Keystore (3-4 days)
- BIP39 mnemonic → seed
- SLIP-0010 Ed25519 HD key derivation
- Legacy SHA-256 derivation
- AES-256-GCM encrypt/decrypt with DB-compatible tag handling
- Argon2id key derivation
- Base58 address encoding
- Cross-compatibility tests with Odin-encrypted data

### Phase 3: Blockchain & Pricing (5-6 days)
- Solana RPC client with failover
- DexScreener client with caching
- Jupiter Price API client with caching
- SOL/USD oracle with fallback chain
- DEX pool decoders (Orca, Raydium AMM/CLMM, Meteora)
- Multi-DEX price router
- Balance fetcher + portfolio assembly

### Phase 4: Bubble Tea TUI — Core (5-6 days)
- Root app model + navigation
- Shared components (table, password input, confirm, spinner, error bar)
- Wallet list (home screen)
- Wallet import wizard
- Wallet status (portfolio view)
- Wallet switch/delete

### Phase 5: Bubble Tea TUI — Extended (3-4 days)
- Token list/fetch/add views
- Swap flow (quote → confirm → execute)
- History view
- Non-interactive JSON mode (Cobra)
- Help overlay

### Phase 6: Polish (2-3 days)
- Lipgloss theming
- Edge cases and error handling
- Cross-platform testing (macOS, Linux)
- README and documentation

**Total estimated effort: 3-4 weeks**
