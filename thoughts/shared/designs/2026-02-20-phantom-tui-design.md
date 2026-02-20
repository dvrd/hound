---
date: 2026-02-20
topic: "Phantom-like Core Wallet Features (Tier 1)"
status: validated
---

# Phantom-like Core Wallet Features (Tier 1)

## Problem Statement

Hound can hold crypto and swap via Jupiter, but lacks the three most fundamental wallet operations: **send**, **receive**, and **generate**. It also has no on-chain transaction history (only swap records from our DB) and requires manual refresh to see balance changes. Without these, Hound is a swap terminal, not a wallet.

## Constraints

- **No CGO** — all dependencies must be pure Go
- **No new external services** — all transaction building happens locally; only Solana RPC for submission
- **Reuse existing patterns** — wizard steps, spinner, NavigateMsg, ViewFactory, deps injection
- **Memory safety** — zero private keys, seeds, and passwords after use (existing `ZeroBytes` pattern)
- **Existing `swap/transaction.go` is untouched** — it serves Jupiter blob signing only

## Approach

Build a **transaction construction module** (`internal/transaction/`) from scratch that can serialize any Solana transaction. Layer SOL transfers, SPL transfers, and ATA creation on top as composable instruction builders. Wire new RPC methods into the existing `blockchain/` package. Build Send and Receive as new TUI views following established wizard/async patterns. Enhance the existing History view to show on-chain activity.

**Why not extend `swap/transaction.go`?** That code patches a pre-built base64 blob from Jupiter — it doesn't parse or construct messages. The transaction builder needs compact-u16 encoding, message serialization, instruction composition, and account ordering. Completely different responsibility.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    TUI Layer                             │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐             │
│  │ Send View│  │Receive View│ │History    │ (enhanced)  │
│  │ (wizard) │  │(display)   │ │(on-chain) │             │
│  └────┬─────┘  └───────────┘  └────┬─────┘             │
│       │                             │                    │
├───────┼─────────────────────────────┼────────────────────┤
│       │      Service Layer          │                    │
│  ┌────▼─────────────┐  ┌───────────▼──────────┐        │
│  │ TransferService   │  │ ActivityService       │        │
│  │ - SendSOL()       │  │ - GetTransactions()   │        │
│  │ - SendSPL()       │  │ - merges on-chain +   │        │
│  │ - builds + signs  │  │   swap history         │        │
│  │ - submits via RPC │  │                        │        │
│  └────┬──────────────┘  └───────────┬────────────┘      │
│       │                             │                    │
├───────┼─────────────────────────────┼────────────────────┤
│       │      Core Layer             │                    │
│  ┌────▼──────────────┐  ┌──────────▼─────────────┐     │
│  │ transaction/       │  │ blockchain/solana.go    │     │
│  │ - Message builder  │  │ + getLatestBlockhash    │     │
│  │ - Instruction      │  │ + sendTransaction       │     │
│  │   composers        │  │ + getSignaturesForAddr  │     │
│  │ - compact-u16      │  │ + getTransaction        │     │
│  │ - Signer           │  │ + getMinBalanceForRent  │     │
│  └───────────────────┘  └────────────────────────┘      │
└─────────────────────────────────────────────────────────┘
```

## Components

### 1. Transaction Builder (`internal/transaction/`)

The core primitive. Constructs Solana transactions from scratch.

**Subcomponents:**

- **`encoding.go`** — compact-u16 encoder/decoder. Solana uses a variable-length integer encoding where values 0-127 are 1 byte, 128-16383 are 2 bytes, 16384+ are 3 bytes. Needed for signature count, account count, instruction count, and data lengths.

- **`types.go`** — Core types:
  - `AccountMeta` — pubkey (32 bytes) + is_signer (bool) + is_writable (bool)
  - `Instruction` — program_id (32 bytes) + accounts ([]AccountMeta) + data ([]byte)
  - `Message` — header + account keys + recent blockhash + instructions
  - `Transaction` — signatures + message

- **`message.go`** — Message builder:
  - Accepts a list of `Instruction`s and a fee payer pubkey
  - Deduplicates and sorts account keys per Solana's required ordering: writable signers → readonly signers → writable non-signers → readonly non-signers
  - Computes the 3-byte header (num_required_signatures, num_readonly_signed, num_readonly_unsigned)
  - Serializes to the binary format: header + compact-u16 num_accounts + account keys + blockhash + compact-u16 num_instructions + serialized instructions
  - Each instruction serializes as: program_id_index (1 byte) + compact-u16 num_accounts + account_indices + compact-u16 data_len + data

- **`transaction.go`** — Transaction wrapper:
  - `NewTransaction(message, signers)` — creates signature slots, signs message bytes with each signer's ed25519 key
  - `Serialize()` — compact-u16 num_signatures + signature bytes + serialized message → `[]byte`
  - `ToBase64()` — base64.StdEncoding of serialized bytes (for `sendTransaction` RPC)

- **`programs.go`** — Well-known program IDs as `[32]byte` constants:
  - System Program: `11111111111111111111111111111111`
  - Token Program: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
  - Associated Token Account Program: `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`
  - Sysvar Rent: `SysvarRent111111111111111111111111111111111`

- **`system.go`** — SystemProgram instruction builders:
  - `Transfer(from, to [32]byte, lamports uint64) Instruction` — instruction index 2, data is 4-byte LE index + 8-byte LE lamports

- **`token.go`** — TokenProgram instruction builders:
  - `TokenTransfer(source, destination, owner [32]byte, amount uint64) Instruction` — instruction index 3, data is 1-byte index + 8-byte LE amount
  - `TokenTransferChecked(source, mint, destination, owner [32]byte, amount uint64, decimals uint8) Instruction` — instruction index 12, preferred over Transfer for safety (validates decimals)

- **`ata.go`** — Associated Token Account helpers:
  - `DeriveATA(wallet, mint [32]byte) ([32]byte, error)` — computes PDA via SHA-256 of `[wallet, TOKEN_PROGRAM_ID, mint, ATA_PROGRAM_ID]` seeds, then verifies the result is off the Ed25519 curve (valid PDA). Pure computation, no RPC needed.
  - `CreateATAInstruction(funder, wallet, mint [32]byte) Instruction` — builds the ATA program's create instruction (7 accounts: funder, ATA address, wallet, mint, system program, token program, rent sysvar)

### 2. New RPC Methods (`internal/blockchain/solana.go`)

Five new methods added to the existing file, using the existing `RPCClient.Call` generic caller:

- **`GetLatestBlockhash(client) (string, uint64, error)`** — returns base58 blockhash + lastValidBlockHeight. Called before every transaction to get a recent blockhash.

- **`SendTransaction(client, base64Tx string) (string, error)`** — submits a signed transaction, returns signature string. Uses `skipPreflight: false` for safety (validates before sending). Encoding: base64.

- **`GetSignaturesForAddress(client, address string, limit int, before string) ([]SignatureInfo, error)`** — returns transaction signatures for an address, newest first. `before` cursor enables pagination. Returns signature, slot, blockTime, err, memo.

- **`GetTransaction(client, signature string) (*TransactionDetail, error)`** — returns full transaction details for a signature. Encoding: jsonParsed for human-readable instruction data. Returns parsed instructions, pre/post balances, fee, timestamp.

- **`GetMinimumBalanceForRentExemption(client, dataSize uint64) (uint64, error)`** — returns lamports needed to make an account rent-exempt. Needed to know the cost of creating an ATA (165 bytes of data → ~0.00203 SOL).

### 3. Transfer Service (`internal/services/transfer.go`)

Orchestrates the full send flow. Follows the same service pattern as `SwapService`.

**`TransferService`** struct with dependencies: `RPCClient`, `KeystoreService`, `Database`.

- **`SendSOL(fromAddr, toAddr string, lamports uint64, password string) (string, error)`**:
  1. Unlock keypair via KeystoreService (password → ed25519.PrivateKey)
  2. Fetch latest blockhash via RPC
  3. Build SystemProgram.Transfer instruction
  4. Build Message (fee payer = sender)
  5. Create and sign Transaction
  6. Submit via SendTransaction RPC
  7. Zero private key
  8. Return transaction signature

- **`SendSPL(fromAddr, toAddr, mint string, amount uint64, decimals uint8, password string) (string, error)`**:
  1. Unlock keypair
  2. Derive sender's ATA (should exist — they hold the token)
  3. Derive recipient's ATA
  4. Check if recipient's ATA exists via `GetAccountInfo`
  5. If ATA doesn't exist: prepend `CreateATAInstruction` (sender pays rent)
  6. Build TokenTransferChecked instruction
  7. Build Message with all instructions, fetch blockhash
  8. Sign and submit
  9. Zero private key
  10. Return signature

- **`EstimateFee(numInstructions int) uint64`** — returns estimated fee in lamports. Solana's base fee is 5000 lamports per signature. For ATA creation, add rent-exemption cost (~2,039,280 lamports). This is a local calculation, not an RPC call.

### 4. Activity Service (`internal/services/activity.go`)

Fetches and merges on-chain transaction history with local swap records.

**`ActivityService`** struct with dependencies: `RPCClient`, `Database`.

- **`GetActivity(address string, limit int, before string) ([]ActivityItem, error)`**:
  1. Fetch on-chain signatures via `GetSignaturesForAddress`
  2. For each signature, fetch transaction details via `GetTransaction`
  3. Classify each transaction: SOL transfer, SPL transfer, swap, program interaction, unknown
  4. Merge with local swap history from DB (match by signature)
  5. Return unified list sorted by timestamp descending

- **`ActivityItem`** type:
  - `Signature` — transaction signature
  - `Type` — "sol_transfer" | "spl_transfer" | "swap" | "program_interaction" | "unknown"
  - `Direction` — "sent" | "received" | "self"
  - `Amount` — human-readable amount (e.g., "1.5 SOL")
  - `Counterparty` — the other address (truncated for display)
  - `Fee` — fee in SOL
  - `Timestamp` — block time
  - `Status` — "confirmed" | "failed"
  - `Slot` — block slot number

**Transaction classification logic:**
- If instructions contain SystemProgram Transfer → "sol_transfer"
- If instructions contain TokenProgram Transfer/TransferChecked → "spl_transfer"
- If signature matches a swap record in DB → "swap"
- Otherwise → "program_interaction" or "unknown"

### 5. Send View (`internal/tui/views/send/`)

Multi-step wizard following the walletimport pattern exactly.

**Steps** (7 total):
1. **StepSelectToken** — List of tokens from portfolio (SOL + SPL tokens with balances). Cursor selection with j/k/arrows. Shows symbol, balance, USD value.
2. **StepRecipient** — Text input for recipient address. Validates: 32-44 chars, base58 characters only, not the sender's own address.
3. **StepAmount** — Text input for amount. Shows available balance and USD equivalent. Validates: positive number, not exceeding balance (minus fee for SOL). "MAX" keyword sends entire balance minus fees.
4. **StepReview** — Summary screen: token, amount, recipient (truncated), estimated fee, total cost. No input, just enter to confirm or esc to go back.
5. **StepPassword** — Password input (masked). Validates against stored hash.
6. **StepSending** — Spinner + async `TransferService.SendSOL/SendSPL` call. Non-cancellable.
7. **StepResult** — Shows success (signature, explorer link) or error. Any key exits.

**Navigation:** Esc goes back one step. At StepSelectToken, esc emits `NavigateBackMsg`. Enter advances. Validation errors display inline via `m.err`.

**ViewFactory registration:** Name `"send"`, data is `string` (wallet address). Falls back to primary wallet if empty (same pattern as swap view).

### 6. Receive View (`internal/tui/views/receive/`)

Simple single-screen view — no wizard needed.

**Display:**
- Wallet label and address (full, not truncated)
- "Press c to copy address to clipboard" instruction
- Visual separator
- Helpful text: "Send SOL or SPL tokens to this address"

**Clipboard:** Use Go's `os/exec` to call `pbcopy` on macOS, `xclip`/`xsel` on Linux, `clip.exe` on Windows/WSL. Detect platform at runtime. If clipboard command fails, show "Clipboard not available — copy address manually".

**ViewFactory registration:** Name `"receive"`, data is `string` (wallet address).

### 7. Enhanced History View (`internal/tui/views/history/`)

Modify the existing history view to show on-chain activity instead of just swap records.

**Changes:**
- Replace `Database` dependency with `ActivityService`
- Fetch activity items instead of swap records
- Display: icon/type indicator, direction arrow (↑ sent / ↓ received), amount, counterparty, timestamp, status
- Pagination: use `before` cursor from last item's signature for "load more"
- Keep swap history entries (merged by ActivityService)
- Color coding: green for received, red for sent, purple for swaps

**Display format per row:**
```
↑ Sent 1.5 SOL → 7xKp...3mFq     2 min ago    confirmed
↓ Received 100 USDC ← 9aBc...dEf  1 hour ago   confirmed
⇄ Swapped 0.5 SOL → 25 USDC       3 hours ago  confirmed
```

### 8. Wallet Generate (extend walletimport)

Add a "Create New Wallet" option to the wallet import flow rather than a separate view.

**Changes to `walletimport`:**
- Add `StepChoice` as the new step 0: "Import existing wallet" or "Create new wallet" (cursor selection)
- If "Create new": generate mnemonic via `bip39.NewMnemonic(128)` (12 words) or `bip39.NewMnemonic(256)` (24 words)
- Add `StepShowMnemonic`: display generated words in a numbered grid, warn user to write them down, require confirmation ("I have saved my recovery phrase")
- After confirmation, flow continues to existing StepWalletType → StepPassword → etc.
- The generated mnemonic is stored in `m.words` just like imported ones — the rest of the flow is identical

**BIP39 generation:** The existing `internal/keystore/bip39.go` only has `ValidateMnemonic` and `MnemonicToSeed`. Add `GenerateMnemonic(bitSize int) (string, error)` that generates entropy via `crypto/rand` and converts to mnemonic words using the BIP39 English wordlist. The wordlist must be embedded (2048 words, ~11KB).

### 9. Wallet Rename

Simple operation — add to wallet status view.

**Changes to `walletstatus`:**
- Add `r` keybinding: "rename wallet"
- Pressing `r` enters a rename sub-mode: show text input overlay, enter saves, esc cancels
- Calls `Database.UpdateWalletLabel(address, newLabel)` — new DB method
- Refresh display after rename

**New DB method in `database/wallets.go`:**
- `UpdateWalletLabel(address, label string) error` — simple UPDATE statement

### 10. Auto-Refresh (`walletstatus` view)

Add periodic balance refresh to the portfolio view.

**Changes to `walletstatus`:**
- Add `tea.Tick` command that fires every 30 seconds
- On tick: trigger the same balance refresh that `r` key does (call `WalletManager.RefreshPortfolio`)
- Show "Last updated: X seconds ago" in the footer
- Manual `r` resets the timer
- Tick only runs while the view is active (no background polling)

## Data Flow

### Send SOL Flow
```
User enters send view → selects SOL → enters recipient → enters amount
→ review screen (fee: 5000 lamports = 0.000005 SOL)
→ enters password → spinner
→ TransferService.SendSOL():
    KeystoreService.UnlockKeypair(addr, password) → ed25519.PrivateKey
    GetLatestBlockhash() → blockhash
    transaction.SystemTransfer(from, to, lamports) → Instruction
    transaction.NewMessage(feePayer, [instruction], blockhash) → Message
    transaction.NewTransaction(message, [privateKey]) → Transaction
    transaction.ToBase64() → base64Tx
    SendTransaction(base64Tx) → signature
    ZeroBytes(privateKey)
→ result screen (signature + explorer link)
```

### Send SPL Flow (with ATA creation)
```
Same as SOL until TransferService.SendSPL():
    UnlockKeypair → privateKey
    DeriveATA(sender, mint) → senderATA
    DeriveATA(recipient, mint) → recipientATA
    GetAccountInfo(recipientATA) → exists?
    if !exists:
        instructions = [CreateATAInstruction(sender, recipient, mint)]
        fee += rentExemption (~0.00203 SOL)
    instructions += [TokenTransferChecked(senderATA, mint, recipientATA, sender, amount, decimals)]
    GetLatestBlockhash → blockhash
    NewMessage(sender, instructions, blockhash) → Message
    NewTransaction(message, [privateKey]) → Transaction
    SendTransaction(ToBase64()) → signature
```

## Error Handling

**Transaction errors:**
- `ErrInsufficientBalance` — checked client-side before building tx (compare balance minus fee)
- `ErrInsufficientBalanceForRent` — when sending SOL, ensure remaining balance covers rent exemption for the account (0 is fine — closing account)
- `ErrInvalidRecipient` — base58 decode fails or address is 0-bytes
- `ErrSendToSelf` — recipient equals sender
- `ErrTransactionFailed` — RPC returns error after submission (show RPC error message)
- `ErrBlockhashExpired` — transaction took too long; prompt user to retry

**ATA creation errors:**
- If recipient ATA already exists but `GetAccountInfo` fails transiently → retry once, then fail with clear message
- If ATA creation instruction fails on-chain → the whole transaction is atomic, nothing partial happens

**Network errors:**
- RPC failover already handled by `RPCClient` (rotates endpoints on failure)
- Timeout: existing 10-second HTTP timeout per call

**All errors surface to the TUI** via the existing `m.err` pattern — displayed inline on the current step, user can retry or go back.

## Testing Strategy

### Transaction Builder (`internal/transaction/`)
- **Unit tests for compact-u16** — encode/decode round-trip for values 0, 127, 128, 16383, 16384, max
- **Unit tests for message serialization** — build a known SOL transfer, compare serialized bytes against a reference (captured from a real Solana transaction)
- **Unit tests for account deduplication and ordering** — verify the 4-group sort (writable signers, readonly signers, writable non-signers, readonly non-signers)
- **Unit tests for ATA derivation** — derive ATA for known wallet+mint, compare against known address
- **Unit tests for instruction data** — verify SystemProgram.Transfer data layout (4-byte LE index + 8-byte LE lamports)

### RPC Methods
- **Mock HTTP server tests** — same pattern as existing `blockchain/` tests. Verify request JSON, return canned responses.

### Transfer Service
- **Integration-style tests** with mocked RPC + mocked keystore — verify the full flow produces a valid base64 transaction
- **Error path tests** — insufficient balance, invalid recipient, ATA creation failure

### Activity Service
- **Mock tests** — verify classification logic (SOL transfer vs SPL vs swap vs unknown)
- **Merge tests** — verify on-chain + DB swap records merge correctly by signature

### TUI Views
- **Same pattern as existing view tests** — create model, send messages, verify state transitions and output strings

## Open Questions

None — all technical decisions are resolved. The design is ready for implementation.
