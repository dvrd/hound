---
date: 2026-02-20
topic: "Implementation Plan: Odin → Go + Bubble Tea Port"
design: thoughts/shared/designs/2026-02-20-go-bubbletea-port-design.md
status: ready
---

# Implementation Plan: Hound Go + Bubble Tea Port

## Overview

Full port of Hound from Odin to Go + Bubble Tea TUI. The Go project lives at `go/` within the existing repo. Each task creates ONE file + its test file.

## Phase 1: Foundation (Batch 1 — all parallel)

### Task 1.1: Go Module Setup
- **Create:** `go/go.mod`, `go/go.sum`
- **Action:** `go mod init github.com/dvrd/hound` then `go get` all dependencies:
  - `github.com/charmbracelet/bubbletea`
  - `github.com/charmbracelet/lipgloss`
  - `github.com/charmbracelet/bubbles`
  - `github.com/spf13/cobra`
  - `modernc.org/sqlite`
  - `golang.org/x/crypto`
  - `golang.org/x/term`
  - `github.com/tyler-smith/go-bip39`
  - `github.com/mr-tron/base58`
- **Verify:** `go build ./...` succeeds

### Task 1.2: Error Types
- **Create:** `go/internal/models/errors.go` + `go/internal/models/errors_test.go`
- **Content:** All sentinel errors (`ErrWalletNotFound`, `ErrWeakPassword`, `ErrInvalidSeedPhrase`, `ErrCryptoFailed`, `ErrQuoteExpired`, `ErrHighPriceImpact`, `ErrInsufficientBalance`, `ErrRPCConnectionFailed`, `ErrDatabaseCorrupted`, `ErrNetworkTimeout`, `ErrRateLimited`, `ErrTokenNotFound`, `ErrNoPoolsFound`, `ErrWalletAlreadyExists`, `ErrKeyNotFound`, `ErrMigrationFailed`). Custom error types: `WalletNotFoundError{Address}`, `TokenNotConfiguredError{Symbol}`. Exit code mapping function.
- **Test:** Verify `errors.Is()` works, exit code mapping is correct.

### Task 1.3: Wallet Models
- **Create:** `go/internal/models/wallet.go` + `go/internal/models/wallet_test.go`
- **Content:** `WalletType` iota enum (Legacy, BIP44Standard, BIP44Change, SolanaCLI). `Wallet` struct (Address, Label, IsPrimary, WalletType, DerivationPath, AccountIndex). `TokenBalance` struct (Mint, Symbol, Amount, Decimals, USDPrice, USDValue, Change24h). `PortfolioBalance` struct (WalletAddress, SOLBalance, TokenBalances, TotalUSD). `GetDerivationPath(walletType, accountIndex)` function. `ParseWalletType(s)` function. `WalletType.String()` method.
- **Test:** Derivation path generation for all 4 types, parse/string round-trip.

### Task 1.4: Token Models
- **Create:** `go/internal/models/token.go` + `go/internal/models/token_test.go`
- **Content:** `Token` struct (Symbol, Name, ContractAddress, Chain, Decimals, Pools, IsQuoteToken, USDPrice). `PoolInfo` struct (Dex, PoolAddress, QuoteToken, PoolType, LiquidityUSD, Volume24h, FeePercent, DiscoveredAt). `PoolStats` struct (PoolCount, TotalLiquidity). `TokenExtendedInfo` struct (all fields from Odin). `TopHolder` struct. `GetTokenDecimals(token)` with fallback logic (USDC=6, USDT=6, SOL=9, default=9).
- **Test:** Token decimals fallback, struct initialization.

### Task 1.5: Swap Models
- **Create:** `go/internal/models/swap.go` + `go/internal/models/swap_test.go`
- **Content:** `SwapQuote` struct (InputMint, OutputMint, InAmount, OutAmount, Rate, SlippageBps, PriceImpactPct, RoutePlan, NetworkFee, FetchedAt, RawResponse). `RouteStep` struct (DexLabel, InputMint, OutputMint, InAmount, OutAmount, FeeAmount, Percent). `SwapTransactionResult` struct (Signature, Slot, BlockTime, Status, ActualInAmount, ActualOutAmount, Fees, Dex, ErrorMessage). `SwapFlags` struct (DryRun, SlippageBps, WalletAddr). `IsExpired()` method on SwapQuote (90s TTL).
- **Test:** Quote expiry logic.

### Task 1.6: Config
- **Create:** `go/internal/config/config.go` + `go/internal/config/config_test.go`
- **Content:** `Config` struct (DatabasePath, RPCEndpoint, BackupEndpoints). `DefaultConfig()` — reads `HOUND_RPC_ENDPOINT` env (default `https://api.mainnet-beta.solana.com`), computes DB path as `$HOME/.config/hound/hound.db` (fallback `/tmp/hound.db`). `EnsureConfigDir()` — creates `~/.config/hound/` if missing.
- **Test:** Default config values, env override, path construction.

## Phase 2: Database Layer (Batch 2 — depends on Phase 1)

### Task 2.1: Database Core
- **Create:** `go/internal/database/database.go` + `go/internal/database/database_test.go`
- **Content:** `Database` struct with `*sql.DB` handle and path. `Open(path)` — opens SQLite, sets PRAGMAs (foreign_keys=ON, journal_mode=WAL, synchronous=NORMAL, busy_timeout=5000). `Close()`. `CreateSchema()` — all 6 CREATE TABLE statements + indexes, identical to Odin. `MigrateSchema()` — ALTER TABLE migrations for wallet_type/derivation_path/account_index columns on wallets, and liquidity_usd/volume_24h/fee_percent/discovered_at on pools. `IntegrityCheck()` — PRAGMA integrity_check.
- **Test:** Open in-memory DB, create schema, integrity check, idempotent migration.
- **Reference:** Odin `src/lib/database/db.odin` lines 56-210

### Task 2.2: Token CRUD
- **Create:** `go/internal/database/tokens.go` + `go/internal/database/tokens_test.go`
- **Content:** `InsertToken(token)`, `GetTokenBySymbol(symbol)` (COLLATE NOCASE), `GetTokenByContractAddress(addr)`, `GetAllTokens()` (ORDER BY symbol), `UpdateTokenPrice(symbol, price)`.
- **Test:** Insert, get by symbol (case insensitive), get by address, list all, update price.
- **Reference:** Odin `db.odin` lines 230-560

### Task 2.3: Pool CRUD
- **Create:** `go/internal/database/pools.go` + `go/internal/database/pools_test.go`
- **Content:** `InsertPool(tokenSymbol, pool)` (looks up token_id first), `GetPoolsForToken(tokenID)`, `DeletePoolsForToken(tokenSymbol)`, `GetPoolStats(tokenSymbol)` (count + sum liquidity).
- **Test:** Insert pool, get pools, delete, stats aggregation.
- **Reference:** Odin `db.odin` lines 290-1320

### Task 2.4: Wallet CRUD
- **Create:** `go/internal/database/wallets.go` + `go/internal/database/wallets_test.go`
- **Content:** `InsertWallet(wallet)`, `GetAllWallets()` (ORDER BY is_primary DESC, added_at ASC), `GetPrimaryWallet()`, `GetWalletByAddress(addr)`, `SetPrimaryWallet(addr)` (transactional: unset all, set one), `DeleteWallet(addr)` (transactional: delete keypair + wallet).
- **Test:** Insert, list, get primary, switch primary (transactional), delete cascade.
- **Reference:** Odin `db.odin` lines 845-1780

### Task 2.5: Balance CRUD
- **Create:** `go/internal/database/balances.go` + `go/internal/database/balances_test.go`
- **Content:** `UpdateBalance(walletAddr, mint, symbol, amount, usdPrice, usdValue)` (INSERT OR REPLACE), `GetBalancesForWallet(walletAddr)` (ORDER BY usd_value DESC).
- **Test:** Upsert balance, retrieve sorted by value.
- **Reference:** Odin `db.odin` lines 1099-1216

### Task 2.6: Encrypted Keypair CRUD
- **Create:** `go/internal/database/keypairs.go` + `go/internal/database/keypairs_test.go`
- **Content:** `EncryptedKeypairData` struct (Address, EncryptedPrivateKey, Salt[16], Nonce[12], Tag[16], PasswordHash[32], Label, IsPrimary, CreatedAt, LastUsed). `InsertEncryptedKeypair(data)`, `GetEncryptedKeypair(addr)`, `UpdateEncryptedKeypair(data)`, `UpdateKeypairLastUsed(addr)`.
- **Test:** Insert, retrieve (verify all BLOB fields round-trip correctly), update.
- **Reference:** Odin `db.odin` lines 1334-1617

### Task 2.7: Swap History CRUD
- **Create:** `go/internal/database/swap_history.go` + `go/internal/database/swap_history_test.go`
- **Content:** `SwapHistoryEntry` struct. `InsertSwapHistory(entry)`, `GetSwapHistory(walletAddr, limit)`, `GetSwapHistoryCount(walletAddr)`.
- **Test:** Insert, retrieve with limit, count.
- **Reference:** Odin `src/lib/database/swap_history.odin`

### Task 2.8: Hyperliquid CRUD
- **Create:** `go/internal/database/hyperliquid.go` + `go/internal/database/hyperliquid_test.go`
- **Content:** `EncryptedHyperliquidData` struct. `SaveHyperliquidWallet(data)`, `LoadHyperliquidWallets()`, `LoadHyperliquidWalletCredentials(addr)`, `SetActiveHyperliquidWallet(addr)`, `GetActiveHyperliquidWallet()`.
- **Test:** Save, load list, load credentials, switch active.
- **Reference:** Odin `db.odin` lines 1790-2300

## Phase 3: Keystore (Batch 3 — depends on Phase 1)

> Can run in parallel with Phase 2

### Task 3.1: Secure Memory Utilities
- **Create:** `go/internal/keystore/secure.go` + `go/internal/keystore/secure_test.go`
- **Content:** `ZeroBytes(b []byte)` — zeroes every byte. `ZeroString(s *string)` — zeroes underlying bytes (unsafe). `SecureBuffer` type with `Reset()` method.
- **Test:** Verify bytes are zeroed after call.

### Task 3.2: Argon2id Key Derivation
- **Create:** `go/internal/keystore/argon2.go` + `go/internal/keystore/argon2_test.go`
- **Content:** Constants: `Argon2MemoryKB=19456`, `Argon2Iterations=2`, `Argon2Parallelism=1`, `Argon2SaltBytes=16`, `Argon2KeyBytes=32`. `GenerateSalt()` → 16 random bytes via `crypto/rand`. `DeriveKey(password, salt)` → 32-byte key via `argon2.IDKey()`. `HashPassword(password, salt)` → 32-byte hash (same function, same output).
- **Test:** Deterministic output with known password+salt. Salt uniqueness. Parameter verification.
- **Reference:** Odin `src/lib/keystore/argon2.odin`

### Task 3.3: AES-256-GCM Encryption
- **Create:** `go/internal/keystore/aes.go` + `go/internal/keystore/aes_test.go`
- **Content:** Constants: `AESKeyBytes=32`, `GCMNonceBytes=12`, `GCMTagBytes=16`. `EncryptedData` struct (Ciphertext, Nonce[12], Tag[16]). `GenerateNonce()` → 12 random bytes. `Encrypt(plaintext, key, nonce)` → EncryptedData. `Decrypt(data EncryptedData, key)` → plaintext. **CRITICAL:** Go's `cipher.AEAD.Seal()` appends tag to ciphertext. Must split last 16 bytes as tag for DB storage. On decrypt, must rejoin ciphertext+tag before calling `Open()`.
- **Test:** Round-trip encrypt/decrypt. Wrong key returns error. Tag split/join correctness.
- **Reference:** Odin `src/lib/keystore/aes_gcm.odin`

### Task 3.4: BIP39 Mnemonic to Seed
- **Create:** `go/internal/keystore/bip39.go` + `go/internal/keystore/bip39_test.go`
- **Content:** `MnemonicToSeed(words []string)` → 64-byte seed. Validates 12 or 24 words. Uses `github.com/tyler-smith/go-bip39` under the hood. `ValidateMnemonic(words)` → error.
- **Test:** Known BIP39 test vectors. 12-word and 24-word. Invalid word count rejection.
- **Reference:** Odin `src/lib/keystore/bip39.odin`

### Task 3.5: SLIP-0010 Ed25519 HD Key Derivation
- **Create:** `go/internal/keystore/bip32.go` + `go/internal/keystore/bip32_test.go`
- **Content:** `HDKey` struct (Key[32], ChainCode[32], Depth). `DeriveMasterKey(seed)` → HDKey via `HMAC-SHA512("ed25519 seed", seed)`. `DeriveChildKey(parent, index)` → HDKey (hardened only, `index | 0x80000000`). `ParseDerivationPath(path)` → []uint32 (validates all hardened). `DeriveFromPath(seed, path)` → HDKey. All use `crypto/hmac` + `crypto/sha512`.
- **Test:** SLIP-0010 test vectors from the spec. Path parsing (valid and invalid). Rejection of non-hardened indices.
- **Reference:** Odin `src/lib/keystore/bip32.odin`

### Task 3.6: Ed25519 Keypair + Base58 Address
- **Create:** `go/internal/keystore/keypair.go` + `go/internal/keystore/keypair_test.go`
- **Content:** `Keypair` struct (PublicKey ed25519.PublicKey, PrivateKey ed25519.PrivateKey). `DeriveKeypairBIP44(seed, walletType, accountIndex)` → Keypair (uses bip32.DeriveFromPath + ed25519). `DeriveKeypairLegacy(seedPhrase)` → Keypair (SHA-256 of joined words → Ed25519 seed). `KeypairToAddress(kp)` → string (Base58 of public key bytes). `ZeroKeypair(kp)` — zeroes private key bytes.
- **Test:** Known seed → known address for each wallet type. Legacy derivation. Address encoding.
- **Reference:** Odin `src/lib/keystore/keypair.odin`

### Task 3.7: Password Validation
- **Create:** `go/internal/keystore/password.go` + `go/internal/keystore/password_test.go`
- **Content:** `ValidatePasswordStrength(password)` → error. Requirements: ≥12 chars, has uppercase, has lowercase, has digit, has special character. `ReadPasswordSecure(prompt)` → string (uses `golang.org/x/term` to disable echo). `ReadPasswordWithConfirmation(prompt1, prompt2)` → string.
- **Test:** Valid and invalid passwords. Each requirement tested independently.
- **Reference:** Odin `src/lib/utils/password.odin` + `src/cli/commands/wallet.odin:600-660`

## Phase 4: Keystore Service (Batch 4 — depends on Phase 2 + 3)

### Task 4.1: Keystore Service
- **Create:** `go/internal/services/keystore.go` + `go/internal/services/keystore_test.go`
- **Content:** `KeystoreService` struct (db *database.Database). `ImportKeypair(db, seedPhrase, password, label, isPrimary, walletType, accountIndex)` → (address string, err error). Full flow: derive keypair → generate salt → derive AES key → hash password → generate nonce → encrypt private key → insert keypair + wallet into DB. `UnlockKeypair(db, address, password)` → (ed25519.PrivateKey, error). Full flow: get encrypted data → derive key → verify password hash → decrypt. `UpdatePassword(db, seedPhrase, newPassword)` → error. All sensitive data zeroed via defer.
- **Test:** Import → unlock round-trip. Wrong password rejection. Password update flow.
- **Reference:** Odin `src/lib/services/keystore_service.odin` + `src/cli/commands/wallet.odin:370-460`

## Phase 5: Blockchain Clients (Batch 5 — depends on Phase 1)

> Can run in parallel with Phases 2-4

### Task 5.1: Solana RPC Client
- **Create:** `go/internal/blockchain/rpc.go` + `go/internal/blockchain/rpc_test.go`
- **Content:** `RPCClient` struct (Endpoint, BackupEndpoints, currentIndex, requestID, httpClient). `NewRPCClient(endpoint, backups)`. `Call(method, params)` → json.RawMessage. Endpoint failover: try primary, on failure rotate to backups, max attempts = 1 + len(backups). JSON-RPC 2.0 request/response handling.
- **Test:** Mock HTTP server. Successful call. Failover on primary failure. Error response handling.
- **Reference:** Odin `src/lib/wallet/rpc_client.odin:370-478`

### Task 5.2: Solana Account/Balance Methods
- **Create:** `go/internal/blockchain/solana.go` + `go/internal/blockchain/solana_test.go`
- **Content:** `GetBalance(client, address)` → uint64 (lamports). `GetTokenAccountsByOwner(client, address)` → []TokenAccount. `GetAccountInfo(client, address)` → []byte (base64 decoded). `GetTokenAccountBalance(client, vaultAddr)` → (amount, decimals, uiAmount). `GetTokenSupply(client, mintAddr)` → (amount, decimals). `GetTokenLargestAccounts(client, mintAddr)` → []AccountBalance. `TokenAccount` struct (Pubkey, Mint, Owner, Amount, Decimals, UIAmount).
- **Test:** Mock RPC responses for each method. JSON parsing of deeply nested responses.
- **Reference:** Odin `src/lib/wallet/rpc_client.odin:98-370` + `src/lib/blockchain/solana_rpc.odin`

### Task 5.3: SOL/USD Oracle
- **Create:** `go/internal/blockchain/oracle.go` + `go/internal/blockchain/oracle_test.go`
- **Content:** `SOLMint = "So11111111111111111111111111111111111111112"`. `solPriceCache` with 30s TTL. `GetSOLPriceCached()` → (float64, error). Fallback chain: cache → Jupiter Price API → CoinGecko API. Price validation: $50-$1000 range. `fetchJupiterSOLPrice()`, `fetchCoinGeckoPrice()`.
- **Test:** Cache hit/miss. Jupiter success. Jupiter fail → CoinGecko fallback. Price validation rejection.
- **Reference:** Odin `src/lib/blockchain/sol_oracle.odin`

### Task 5.4: DexScreener Client
- **Create:** `go/internal/dex/dexscreener.go` + `go/internal/dex/dexscreener_test.go`
- **Content:** `DexScreenerClient` struct. `FetchPrice(contractAddr)` → (price, change24h, error). `FetchPoolsForToken(contractAddr)` → []PairData. In-memory caches: 5min for 24h change, 1hr for pools. `FetchWithRetry(addr)` — max 3 attempts, exponential backoff 1s→2s→4s on rate limit. `DexScreenerResponse`, `PairData` response structs.
- **Test:** Mock API. Price parsing (string→float64). Cache behavior. Retry on rate limit.
- **Reference:** Odin `src/lib/dex/price_fetcher.odin` + `src/lib/dex/pool_discovery.odin`

### Task 5.5: Jupiter Price Client
- **Create:** `go/internal/dex/jupiter.go` + `go/internal/dex/jupiter_test.go`
- **Content:** `JupiterClient` struct. `FetchPrice(mint)` → (price, change24h, error). 60s cache. `FetchPriceWithRetry(mint)` — max 3 attempts, exponential backoff. `LookupTokenMetadata(mintAddr)` → TokenMetadata (via `/tokens/v2/search`).
- **Test:** Mock API. Cache behavior. Retry logic.
- **Reference:** Odin `src/lib/blockchain/jupiter_client.odin` + `src/lib/wallet/token_metadata.odin`

### Task 5.6: Pool Filtering and Ranking
- **Create:** `go/internal/dex/pool.go` + `go/internal/dex/pool_test.go`
- **Content:** `FilterPools(pairs)` → filtered (min $1K liquidity, max 1% fee, Solana chain only). `RankPools(pools)` → sorted by score. Score = `0.8 * liquidity - 0.2 * fee * 10000`. `PairToPoolInfo(pair)` → PoolInfo conversion. `TopPoolsToStore = 3`.
- **Test:** Filtering thresholds. Ranking order. Edge cases (zero liquidity, high fee).
- **Reference:** Odin `src/lib/dex/pool_ranking.odin` + `src/lib/services/pool_service.odin`

### Task 5.7: DEX Pool Decoders — Orca Whirlpool
- **Create:** `go/internal/dex/decoders/orca.go` + `go/internal/dex/decoders/orca_test.go`
- **Content:** `DecodeOrcaWhirlpool(accountData []byte)` → (sqrtPrice, tokenMintA, tokenMintB, error). Account size: 653 bytes. `SqrtPriceToPrice(sqrtPrice uint128, decimalsA, decimalsB int)` → float64. Q64.64 fixed-point math.
- **Test:** Known pool bytes → expected price.
- **Reference:** Odin `src/lib/blockchain/orca_decoder.odin`

### Task 5.8: DEX Pool Decoders — Raydium AMM V4
- **Create:** `go/internal/dex/decoders/raydium_amm.go` + `go/internal/dex/decoders/raydium_amm_test.go`
- **Content:** `DecodeRaydiumAMM(accountData []byte)` → (tokenVaultA, tokenVaultB, tokenMintA, tokenMintB, error). Account size: 752 bytes. Price from vault reserves: `quote_actual / base_actual` adjusted for decimals.
- **Test:** Known pool bytes → expected vault addresses.
- **Reference:** Odin `src/lib/blockchain/raydium_decoder.odin`

### Task 5.9: DEX Pool Decoders — Raydium CLMM
- **Create:** `go/internal/dex/decoders/raydium_clmm.go` + `go/internal/dex/decoders/raydium_clmm_test.go`
- **Content:** `DecodeRaydiumCLMM(accountData []byte)` → decoded struct with sqrt_price and embedded decimals.
- **Test:** Known pool bytes → expected price.
- **Reference:** Odin `src/lib/blockchain/raydium_clmm_decoder.odin`

### Task 5.10: DEX Pool Decoders — Meteora DLMM
- **Create:** `go/internal/dex/decoders/meteora.go` + `go/internal/dex/decoders/meteora_test.go`
- **Content:** `DecodeMeteoraDLMM(accountData []byte)` → decoded struct. Price formula: `(1 + binStep/10000) ^ (activeId - 8388608)`.
- **Test:** Known pool bytes → expected price.
- **Reference:** Odin `src/lib/blockchain/meteora_dlmm_decoder.odin`

### Task 5.11: Multi-DEX Price Router
- **Create:** `go/internal/dex/router.go` + `go/internal/dex/router_test.go`
- **Content:** `DexType` enum (OrcaWhirlpool, RaydiumCLMM, RaydiumAMMV4, MeteoraMLMM, JupiterAPI). `Router` struct (rpcClient, jupiterClient, oracleFunc). `FetchPrice(token)` → (price, change24h, error). Routes through pools in priority order, falls back to Jupiter API. Quote token → USD conversion: sol/wsol → oracle, usdc/usdt → 1.0.
- **Test:** Mock all decoders. Priority-based routing. Fallback chain.
- **Reference:** Odin `src/lib/dex/router.odin`

## Phase 6: Wallet Operations (Batch 6 — depends on Phases 2, 4, 5)

### Task 6.1: Balance Fetcher
- **Create:** `go/internal/wallet/balance.go` + `go/internal/wallet/balance_test.go`
- **Content:** `BalanceFetcher` struct (rpcClient, priceRouter, db). `FetchPortfolioBalance(address)` → PortfolioBalance. Flow: getBalance (SOL) → getTokenAccountsByOwner → for each token: lookup in config/DB → fetch price via router → assemble TokenBalance. Smart number formatting functions: `FormatBalance(amount)`, `FormatPrice(price)`, `FormatLargeNumber(n)`.
- **Test:** Mock RPC + router. Portfolio assembly. Number formatting (all precision tiers).
- **Reference:** Odin `src/lib/wallet/balance_fetcher.odin`

### Task 6.2: Wallet Manager
- **Create:** `go/internal/wallet/manager.go` + `go/internal/wallet/manager_test.go`
- **Content:** `WalletManager` struct (db, rpcClient, balanceFetcher, portfolioCache). `AddWallet(wallet)` → error (validates address, inserts). `GetWallets()` → []Wallet. `RefreshPortfolio(address)` → PortfolioBalance (fetches + persists + caches). `RefreshAllPortfolios()` → map[string]PortfolioBalance. `AggregatePortfolios(portfolios)` → PortfolioBalance (sums across wallets). `ResolveWallet(identifier)` → Wallet (by address, partial address, or label).
- **Test:** Add + list. Refresh with mock fetcher. Aggregation logic. Resolve by partial address and label.
- **Reference:** Odin `src/lib/wallet/wallet_manager.odin` + `wallet_operations.odin`

## Phase 7: Services Layer (Batch 7 — depends on Phase 6)

### Task 7.1: Price Service
- **Create:** `go/internal/services/price.go` + `go/internal/services/price_test.go`
- **Content:** `PriceService` struct (router, dexscreener, jupiter). `FetchPriceWithFallback(token)` → (price, change24h, error). If pools → try on-chain → fallback API. If no pools → API directly. `FetchMultiplePrices(tokens)` → map[string]PriceData (partial results OK).
- **Test:** Fallback chain. Partial failure handling.
- **Reference:** Odin `src/lib/services/price_service.odin`

### Task 7.2: Pool Service
- **Create:** `go/internal/services/pool.go` + `go/internal/services/pool_test.go`
- **Content:** `PoolService` struct (dexscreener, db). `DiscoverAndStorePools(token, forceRefresh)` → (bestPool, error). Flow: fetch from DexScreener → filter → rank → store top 3 in DB. If forceRefresh, delete old pools first.
- **Test:** Discovery flow. Force refresh deletes old. Top 3 storage.
- **Reference:** Odin `src/lib/services/pool_service.odin` + `src/lib/config/token_config.odin:196`

### Task 7.3: Token Info Service
- **Create:** `go/internal/services/token_info.go` + `go/internal/services/token_info_test.go`
- **Content:** `TokenInfoService` struct (dexscreener, rpcClient). `FetchExtendedTokenInfo(mintOrSymbol, db)` → TokenExtendedInfo. Aggregates DexScreener market data + Solana RPC supply/holders.
- **Test:** Aggregation of multi-pair data. Holder percentage calculation.
- **Reference:** Odin `src/lib/services/token_info_service.odin`

## Phase 8: Swap Layer (Batch 8 — depends on Phases 5, 6)

### Task 8.1: Jupiter Ultra Swap Client
- **Create:** `go/internal/swap/client.go` + `go/internal/swap/client_test.go`
- **Content:** `SwapClient` struct (httpClient). `GetQuote(inputMint, outputMint, amount, taker)` → SwapQuote. Calls `GET /ultra/v1/order`. 90s quote cache (key: `input:output:amount`). Parses routePlan, transaction (base64), requestId.
- **Test:** Mock API. Quote parsing. Cache hit/miss.
- **Reference:** Odin `src/lib/swap/client.odin`

### Task 8.2: Transaction Signing
- **Create:** `go/internal/swap/transaction.go` + `go/internal/swap/transaction_test.go`
- **Content:** `SignTransaction(txBase64, privateKey)` → signedTxBase64. Flow: base64 decode → validate (≥65 bytes, 1 signature slot) → extract message (bytes[65:]) → Ed25519 sign → copy signature into bytes[1:65] → base64 encode. `SubmitTransaction(signedTx, requestId)` → SwapTransactionResult. Calls `POST /ultra/v1/execute`.
- **Test:** Known transaction bytes → valid signature placement. Submit with mock API.
- **Reference:** Odin `src/lib/services/transaction_service.odin`

### Task 8.3: Swap Service
- **Create:** `go/internal/services/swap.go` + `go/internal/services/swap_test.go`
- **Content:** `SwapService` struct (swapClient, keystoreService, db). `ExecuteSwap(quote, walletAddr, password)` → SwapTransactionResult. Full flow: check expiry → unlock keypair → sign → submit → save history.
- **Test:** Full flow with mocks. Expired quote rejection. Password failure.
- **Reference:** Odin `src/cli/commands/wallet.odin:427-481`

## Phase 9: TUI Components (Batch 9 — depends on Phase 1)

> Can start as soon as Phase 1 is done. No business logic dependency.

### Task 9.1: Styled Table Component
- **Create:** `go/internal/tui/components/table.go` + `go/internal/tui/components/table_test.go`
- **Content:** `StyledTable` wrapping `bubbles/table` with lipgloss. Dynamic column widths. Smart number formatting. Header/row/footer styling. Alternating row colors.
- **Test:** Column width calculation. Number formatting output.

### Task 9.2: Password Input Component
- **Create:** `go/internal/tui/components/password.go` + `go/internal/tui/components/password_test.go`
- **Content:** `PasswordInput` model. Masked display (dots). Real-time strength indicator bar (weak/fair/strong/very strong based on length + char classes). Confirmation mode (enter twice, compare). Emits `PasswordEnteredMsg` on completion.
- **Test:** Strength calculation. Confirmation match/mismatch.

### Task 9.3: Confirm Dialog Component
- **Create:** `go/internal/tui/components/confirm.go` + `go/internal/tui/components/confirm_test.go`
- **Content:** `ConfirmDialog` model. Shows question + [y/N] or [Y/n]. Emits `ConfirmMsg{Confirmed bool}`. Supports custom yes/no labels.
- **Test:** Yes/no key handling. Default selection.

### Task 9.4: Spinner Component
- **Create:** `go/internal/tui/components/spinner.go` + `go/internal/tui/components/spinner_test.go`
- **Content:** `LoadingSpinner` model wrapping `bubbles/spinner`. Configurable message. Styled with lipgloss.
- **Test:** Message rendering.

### Task 9.5: Error Bar Component
- **Create:** `go/internal/tui/components/error.go` + `go/internal/tui/components/error_test.go`
- **Content:** `ErrorBar` model. Red background, white text. Auto-dismiss after 5 seconds (via `tea.Tick`). Dismiss on any keypress. Maps sentinel errors to user-friendly messages (same catalog as Odin `errors.odin`).
- **Test:** Error message mapping. Auto-dismiss timing.

### Task 9.6: Help Overlay Component
- **Create:** `go/internal/tui/components/help.go` + `go/internal/tui/components/help_test.go`
- **Content:** `HelpOverlay` model. Shows keybinding reference for current view. Toggle with `?`. Semi-transparent overlay styling.
- **Test:** Toggle behavior. Key list rendering.

## Phase 10: TUI Views — Core (Batch 10 — depends on Phases 6, 9)

### Task 10.1: Root App Model
- **Create:** `go/internal/tui/app.go` + `go/internal/tui/app_test.go`
- **Content:** `App` model. Holds current view (interface), shared deps (db, walletManager, config). Global keybindings: `q`/`ctrl+c` quit, `esc` back, `?` help. Navigation messages: `NavigateMsg{View}`. Initializes with WalletListModel as home.
- **Test:** Navigation routing. Global key handling. View switching.

### Task 10.2: Wallet List (Home Screen)
- **Create:** `go/internal/tui/wallet/list.go` + `go/internal/tui/wallet/list_test.go`
- **Content:** `WalletListModel`. Shows all wallets with cached USD totals (from balances table, no network). Keybindings: Enter/s→status, i→import, d→delete, w→switch, t→tokens, h→history, a→aggregate. Shows total across all wallets. Highlights primary wallet with ★.
- **Test:** Rendering with mock data. Key routing.

### Task 10.3: Wallet Import Wizard
- **Create:** `go/internal/tui/wallet/import.go` + `go/internal/tui/wallet/import_test.go`
- **Content:** `WalletImportModel`. 7-step state machine: seed phrase (textarea) → wallet type (list) → account index (textinput) → address preview (confirm) → password (password component) → label (textinput) → success. Esc goes back one step. Sensitive data zeroed on exit. Uses `tea.Cmd` for keypair derivation (non-blocking).
- **Test:** State transitions. Back navigation. Validation at each step.

### Task 10.4: Wallet Status (Portfolio)
- **Create:** `go/internal/tui/wallet/status.go` + `go/internal/tui/wallet/status_test.go`
- **Content:** `WalletStatusModel`. Shows cached balances immediately on enter. Triggers async refresh via `tea.Cmd`. Table columns: Token, Balance, Price, Value (USD), 24h Change. Keybindings: r→refresh, a→show all, j→JSON dump, 1/2/3→sort. Spinner during refresh. Error bar on failure.
- **Test:** Cached data display. Refresh flow. Sort modes.

### Task 10.5: Wallet Delete Flow
- **Create:** `go/internal/tui/wallet/delete.go` + `go/internal/tui/wallet/delete_test.go`
- **Content:** `WalletDeleteModel`. Shows wallet details + WARNING. Requires typing full address to confirm. Cannot delete last wallet. Auto-promotes another wallet to primary if deleting primary.
- **Test:** Confirmation matching. Last wallet rejection. Primary promotion.

## Phase 11: TUI Views — Extended (Batch 11 — depends on Phase 10)

### Task 11.1: Token List View
- **Create:** `go/internal/tui/tokens/list.go` + `go/internal/tui/tokens/list_test.go`
- **Content:** `TokenListModel`. Table: Symbol, Name, Pools, Liquidity, Auto-discovered indicator (✨). Keybindings: Enter→fetch details, a→add token, esc→back.
- **Test:** Rendering with pool stats.

### Task 11.2: Token Fetch View
- **Create:** `go/internal/tui/tokens/fetch.go` + `go/internal/tui/tokens/fetch_test.go`
- **Content:** `TokenFetchModel`. Extended info display: market data, 24h trading, price changes, top holders. Async fetch via `tea.Cmd`. Spinner while loading.
- **Test:** Data rendering. Loading state.

### Task 11.3: Token Add View
- **Create:** `go/internal/tui/tokens/add.go` + `go/internal/tui/tokens/add_test.go`
- **Content:** `TokenAddModel`. Form: symbol, name, contract address. Validates address length (32-44). Checks for duplicates. Optional pool discovery after add.
- **Test:** Validation. Duplicate detection.

### Task 11.4: Swap View
- **Create:** `go/internal/tui/swap/swap.go` + `go/internal/tui/swap/swap_test.go`
- **Content:** `SwapModel`. Multi-phase: input (3 fields) → quoting (spinner) → review (styled quote with price impact) → password → executing (spinner) → result. Dry-run mode. High price impact warning (>5%).
- **Test:** State transitions. Dry-run short-circuit.

### Task 11.5: History View
- **Create:** `go/internal/tui/history/history.go` + `go/internal/tui/history/history_test.go`
- **Content:** `HistoryModel`. Paginated table: Date, Trade (FROM → TO), Rate, Status, DEX. Date formatted as `DDd HHh:MMm`. Keybindings: n→next page, p→prev page.
- **Test:** Date formatting. Pagination logic.

## Phase 12: CLI Entry Point + Non-Interactive Mode (Batch 12 — depends on Phase 10)

### Task 12.1: Cobra Root Command + TUI Launch
- **Create:** `go/cmd/hound/main.go`
- **Content:** Cobra root command. If no `--json` flag → launch Bubble Tea with `App` model. Subcommands: `wallet`, `tokens`, `history`, `version`. Each subcommand checks `--json` flag: if set, runs non-interactive and prints JSON to stdout; if not, launches Bubble Tea at the appropriate view.
- **Verify:** `go build -o bin/hound-go ./cmd/hound` succeeds.

### Task 12.2: Non-Interactive JSON Output
- **Create:** `go/internal/tui/json_output.go` + `go/internal/tui/json_output_test.go`
- **Content:** `RunNonInteractive(command, args, flags)` → exits with JSON on stdout, errors on stderr. Supports: `wallet status --json`, `wallet list --json`, `tokens list --json`. Same JSON format as Odin version.
- **Test:** JSON output format matches expected schema.

## Phase 13: Integration + Polish (Batch 13 — depends on all)

### Task 13.1: Cross-Compatibility Test
- **Create:** `go/tests/compat_test.go`
- **Content:** Test that Go can read an existing Odin-created `hound.db`. Test that Go can decrypt a keypair encrypted by the Odin version. Requires golden test fixtures (copy of a real DB with known test data).
- **Verify:** `go test ./tests/...`

### Task 13.2: Lipgloss Theme
- **Create:** `go/internal/tui/theme.go`
- **Content:** Centralized color palette, border styles, text styles. Consistent look across all views.

### Task 13.3: Taskfile Integration
- **Modify:** `Taskfile.yml`
- **Content:** Add `go:build`, `go:test`, `go:run`, `go:lint` tasks.

## Dependency Graph

```
Phase 1 (models, config, errors) ──┬──→ Phase 2 (database) ──────────┐
                                    ├──→ Phase 3 (keystore) ──────────┤
                                    ├──→ Phase 5 (blockchain clients) ─┤
                                    └──→ Phase 9 (TUI components) ─────┤
                                                                       │
Phase 2 + 3 ──→ Phase 4 (keystore service) ───────────────────────────┤
Phase 2 + 4 + 5 ──→ Phase 6 (wallet operations) ─────────────────────┤
Phase 6 ──→ Phase 7 (services) ───────────────────────────────────────┤
Phase 5 + 6 ──→ Phase 8 (swap) ──────────────────────────────────────┤
Phase 6 + 9 ──→ Phase 10 (TUI core views) ───────────────────────────┤
Phase 10 ──→ Phase 11 (TUI extended views) ───────────────────────────┤
Phase 10 ──→ Phase 12 (CLI entry + JSON mode) ────────────────────────┤
All ──→ Phase 13 (integration + polish) ──────────────────────────────┘
```

## Parallel Execution Opportunities

| Batch | Tasks | Can Run With |
|-------|-------|-------------|
| Batch 1 | 1.1–1.6 | All parallel |
| Batch 2 | 2.1–2.8 | Batch 3, 5, 9 |
| Batch 3 | 3.1–3.7 | Batch 2, 5, 9 |
| Batch 4 | 4.1 | Batch 5, 9 |
| Batch 5 | 5.1–5.11 | Batch 2, 3, 9 |
| Batch 6 | 6.1–6.2 | Batch 8, 9 |
| Batch 7 | 7.1–7.3 | Batch 8, 9 |
| Batch 8 | 8.1–8.3 | Batch 9, 10 |
| Batch 9 | 9.1–9.6 | Batch 2–8 |
| Batch 10 | 10.1–10.5 | — |
| Batch 11 | 11.1–11.5 | Batch 12 |
| Batch 12 | 12.1–12.2 | Batch 11 |
| Batch 13 | 13.1–13.3 | — |

## Verification

After each phase:
- `go build ./...` must succeed
- `go test ./...` must pass
- `go vet ./...` must be clean

Final verification:
- Build binary: `go build -o bin/hound-go ./cmd/hound`
- Run TUI: `./bin/hound-go`
- Import a wallet and view portfolio
- Cross-compatibility with existing Odin database
