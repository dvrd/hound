# Phantom-like Core Wallet Features — Implementation Plan

**Goal:** Add send, receive, generate, rename, activity history, and auto-refresh to make Hound a full wallet, not just a swap terminal.

**Architecture:** Build a transaction construction module (`internal/transaction/`) from scratch, layer new RPC methods on top, create TransferService and ActivityService, then wire into TUI views following existing wizard/async patterns. Each phase is independently testable and builds on the prior.

**Design:** `thoughts/shared/designs/2026-02-20-phantom-tui-design.md`

---

## Dependency Graph

```
Phase 1  (parallel): 1.1–1.8  [transaction builder — no deps]
Phase 2  (parallel): 2.1–2.2  [new RPC methods — depends on Phase 1 for types only]
Phase 3  (serial):   3.1      [transfer service — depends on Phase 1 + 2]
Phase 4  (parallel): 4.1–4.2  [send view + messages — depends on Phase 3]
Phase 5  (parallel): 5.1      [receive view — no deps on Phase 3/4]
Phase 6  (parallel): 6.1–6.2  [activity service + enhanced history — depends on Phase 2]
Phase 7  (parallel): 7.1–7.2  [wallet generate — independent]
Phase 8  (parallel): 8.1–8.2  [wallet rename — independent]
Phase 9  (serial):   9.1      [auto-refresh — independent]
Phase 10 (serial):   10.1     [ViewFactory wiring + integration — depends on all]
```

---

## Phase 1: Transaction Builder (`internal/transaction/`)

All tasks in this phase have NO dependencies and run simultaneously.
This is the core primitive — pure computation, no RPC, no I/O.

### Task 1.1: Compact-u16 Encoding
**File:** `internal/transaction/encoding.go`
**Test:** `internal/transaction/encoding_test.go`
**Depends:** none

**What to implement:**
- `EncodeCompactU16(value uint16) []byte` — Solana's variable-length integer encoding:
  - Values 0–127: 1 byte (value as-is)
  - Values 128–16383: 2 bytes (low 7 bits + continuation bit, then remaining bits)
  - Values 16384–65535: 3 bytes (7 bits + continuation, 7 bits + continuation, remaining)
- `DecodeCompactU16(data []byte) (uint16, int, error)` — returns value and bytes consumed
- The encoding is little-endian with 7-bit groups and a high-bit continuation flag

**Test requirements:**
- Round-trip encode/decode for boundary values: 0, 1, 127, 128, 255, 16383, 16384, 65535
- Verify exact byte output for known values:
  - 0 → `[0x00]`
  - 127 → `[0x7f]`
  - 128 → `[0x80, 0x01]`
  - 16383 → `[0xff, 0x7f]`
  - 16384 → `[0x80, 0x80, 0x01]`
- Error case: empty input to DecodeCompactU16

**Verify:** `go test ./internal/transaction/ -run TestCompactU16 -v`
**Commit:** `feat(transaction): add compact-u16 variable-length encoding`

---

### Task 1.2: Core Types
**File:** `internal/transaction/types.go`
**Test:** `internal/transaction/types_test.go`
**Depends:** none

**What to implement:**
- `type Pubkey [32]byte` — with `String() string` (base58 encoding via `github.com/mr-tron/base58`) and `PubkeyFromBase58(s string) (Pubkey, error)`
- `type AccountMeta struct { Pubkey Pubkey; IsSigner bool; IsWritable bool }`
- `type Instruction struct { ProgramID Pubkey; Accounts []AccountMeta; Data []byte }`
- `type MessageHeader struct { NumRequiredSignatures uint8; NumReadonlySignedAccounts uint8; NumReadonlyUnsignedAccounts uint8 }`
- `type Message struct { Header MessageHeader; AccountKeys []Pubkey; RecentBlockhash Pubkey; Instructions []CompiledInstruction }`
- `type CompiledInstruction struct { ProgramIDIndex uint8; AccountIndices []uint8; Data []byte }`
- `type Transaction struct { Signatures [][]byte; Message Message; serializedMessage []byte }`

**Test requirements:**
- `PubkeyFromBase58` round-trip with known Solana addresses (System Program `11111111111111111111111111111111`)
- `PubkeyFromBase58` error for invalid base58 strings
- `Pubkey.String()` returns correct base58
- `AccountMeta` construction and field access

**Verify:** `go test ./internal/transaction/ -run TestTypes -v`
**Commit:** `feat(transaction): add core Solana transaction types`

---

### Task 1.3: Program IDs
**File:** `internal/transaction/programs.go`
**Test:** `internal/transaction/programs_test.go`
**Depends:** 1.2 (uses Pubkey type)

**What to implement:**
- Package-level `var` declarations (not const — `[32]byte` can't be const in Go):
  - `SystemProgramID` — base58 `11111111111111111111111111111111`
  - `TokenProgramID` — base58 `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
  - `ATAProgramID` — base58 `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`
  - `SysvarRentID` — base58 `SysvarRent111111111111111111111111111111111`
  - `SOLMint` — base58 `So11111111111111111111111111111111111111112` (wrapped SOL)
- Use `mustPubkeyFromBase58(s string) Pubkey` helper that panics on invalid input (init-time only)

**Test requirements:**
- Verify each program ID's `String()` matches the expected base58 address
- Verify byte representation is exactly 32 bytes

**Verify:** `go test ./internal/transaction/ -run TestPrograms -v`
**Commit:** `feat(transaction): add well-known Solana program ID constants`

---

### Task 1.4: Message Builder
**File:** `internal/transaction/message.go`
**Test:** `internal/transaction/message_test.go`
**Depends:** 1.1, 1.2, 1.3

**What to implement:**
- `NewMessage(feePayer Pubkey, instructions []Instruction, recentBlockhash Pubkey) Message`
  1. Collect all unique accounts from instructions + fee payer
  2. Fee payer is always first, always signer+writable
  3. Deduplicate accounts by pubkey; merge flags (if any instruction marks it signer, it's signer; same for writable)
  4. Sort into 4 groups: writable signers → readonly signers → writable non-signers → readonly non-signers
  5. Program IDs are always readonly non-signers (never signer, never writable)
  6. Compute header: `NumRequiredSignatures` = count of signers, `NumReadonlySignedAccounts` = count of readonly signers, `NumReadonlyUnsignedAccounts` = count of readonly non-signers
  7. Build `CompiledInstruction` for each instruction: look up program ID index and account indices in the sorted account list
  8. Set `RecentBlockhash`
- `(m Message) Serialize() []byte`
  - Format: `header (3 bytes) + compact-u16(num_accounts) + account_keys (32 bytes each) + blockhash (32 bytes) + compact-u16(num_instructions) + compiled_instructions`
  - Each compiled instruction: `program_id_index (1 byte) + compact-u16(num_accounts) + account_indices (1 byte each) + compact-u16(data_len) + data`

**Test requirements:**
- Build a simple SOL transfer message (1 instruction, 2 accounts + system program), verify:
  - Header: `{NumRequiredSignatures: 1, NumReadonlySignedAccounts: 0, NumReadonlyUnsignedAccounts: 1}`
  - Account order: sender (writable signer), recipient (writable non-signer), system program (readonly non-signer)
  - Serialized bytes length is correct
- Account deduplication: instruction referencing same account twice → only appears once
- Multiple instructions sharing accounts → correct dedup and ordering
- Fee payer always at index 0

**Verify:** `go test ./internal/transaction/ -run TestMessage -v`
**Commit:** `feat(transaction): add Solana message builder with account dedup and ordering`

---

### Task 1.5: Transaction Wrapper
**File:** `internal/transaction/transaction.go`
**Test:** `internal/transaction/transaction_test.go`
**Depends:** 1.4

**What to implement:**
- `NewTransaction(msg Message, signers []ed25519.PrivateKey) (*Transaction, error)`
  1. Serialize the message
  2. Verify number of signers matches `msg.Header.NumRequiredSignatures`
  3. For each signer, `ed25519.Sign(privateKey, serializedMessage)` → 64-byte signature
  4. Signatures are ordered to match the account key order (signer at index 0 → signature at index 0, etc.)
  5. Store serialized message for later use
- `(t *Transaction) Serialize() []byte`
  - Format: `compact-u16(num_signatures) + signatures (64 bytes each) + serialized_message`
- `(t *Transaction) ToBase64() string`
  - `base64.StdEncoding.EncodeToString(t.Serialize())`

**Test requirements:**
- Create a transaction with a known ed25519 keypair, verify:
  - Signature is 64 bytes
  - `ed25519.Verify(pubkey, message, signature)` returns true
  - Serialized output starts with compact-u16 encoding of 1 (one signature)
  - `ToBase64()` produces valid base64 that decodes back to the serialized bytes
- Error case: wrong number of signers

**Verify:** `go test ./internal/transaction/ -run TestTransaction -v`
**Commit:** `feat(transaction): add transaction signing and serialization`

---

### Task 1.6: System Program Instructions
**File:** `internal/transaction/system.go`
**Test:** `internal/transaction/system_test.go`
**Depends:** 1.2, 1.3

**What to implement:**
- `SystemTransfer(from, to Pubkey, lamports uint64) Instruction`
  - ProgramID: `SystemProgramID`
  - Accounts: `[{from, signer: true, writable: true}, {to, signer: false, writable: true}]`
  - Data: 12 bytes — `uint32(2)` as little-endian (transfer instruction index) + `uint64(lamports)` as little-endian

**Test requirements:**
- Verify instruction data is exactly 12 bytes
- Verify first 4 bytes decode to `uint32(2)` (LE)
- Verify next 8 bytes decode to the lamports value (LE)
- Verify accounts: from is signer+writable, to is writable+not-signer
- Verify ProgramID is SystemProgramID
- Test with lamports = 0, 1, 1_000_000_000 (1 SOL), max uint64

**Verify:** `go test ./internal/transaction/ -run TestSystem -v`
**Commit:** `feat(transaction): add SystemProgram transfer instruction builder`

---

### Task 1.7: Token Program Instructions
**File:** `internal/transaction/token.go`
**Test:** `internal/transaction/token_test.go`
**Depends:** 1.2, 1.3

**What to implement:**
- `TokenTransfer(source, destination, owner Pubkey, amount uint64) Instruction`
  - ProgramID: `TokenProgramID`
  - Accounts: `[{source, writable: true}, {destination, writable: true}, {owner, signer: true}]`
  - Data: 9 bytes — `uint8(3)` (transfer instruction index) + `uint64(amount)` LE
- `TokenTransferChecked(source, mint, destination, owner Pubkey, amount uint64, decimals uint8) Instruction`
  - ProgramID: `TokenProgramID`
  - Accounts: `[{source, writable: true}, {mint, readonly}, {destination, writable: true}, {owner, signer: true}]`
  - Data: 10 bytes — `uint8(12)` (transferChecked index) + `uint64(amount)` LE + `uint8(decimals)`

**Test requirements:**
- `TokenTransfer`: verify data layout (1 + 8 bytes), account flags, program ID
- `TokenTransferChecked`: verify data layout (1 + 8 + 1 bytes), 4 accounts with correct flags
- Test with amount = 1_000_000 (1 USDC with 6 decimals), decimals = 6

**Verify:** `go test ./internal/transaction/ -run TestToken -v`
**Commit:** `feat(transaction): add SPL Token transfer instruction builders`

---

### Task 1.8: ATA Derivation and Creation
**File:** `internal/transaction/ata.go`
**Test:** `internal/transaction/ata_test.go`
**Depends:** 1.2, 1.3

**What to implement:**
- `DeriveATA(wallet, mint Pubkey) (Pubkey, error)`
  - Compute PDA: SHA-256 hash of seeds `[wallet_bytes, TOKEN_PROGRAM_ID_bytes, mint_bytes]` with program ID `ATA_PROGRAM_ID`
  - PDA derivation: try bump seed from 255 down to 0. For each bump:
    1. Concatenate: `seeds... + [bump] + program_id + "ProgramDerivedAddress"`
    2. SHA-256 hash the concatenation
    3. Check if result is NOT on the Ed25519 curve (valid PDA must be off-curve)
    4. If off-curve, return the 32-byte hash as the PDA
  - Use `crypto/sha256` (stdlib). For the Ed25519 curve check, attempt to decompress the point — if it fails, the point is off-curve (valid PDA). Use the existing `github.com/mr-tron/base58` for any base58 needs.
  - **Ed25519 on-curve check:** A 32-byte value is on the Ed25519 curve if it can be decoded as a valid curve point. The simplest pure-Go approach: use `crypto/ed25519` internal point decompression. Since Go's stdlib doesn't expose this directly, implement a minimal check: try `edwards25519.NewGeneratorPoint().SetBytes(candidate)` from `filippo.io/edwards25519` — BUT that adds a dependency. Instead, since `golang.org/x/crypto` is already in go.mod, use the `edwards25519` field operations from `crypto/ed25519/internal` — BUT that's internal. **Decision:** Use a lookup table approach. Pre-compute nothing. Instead, use the fact that for Solana's `findProgramAddress`, the standard approach is to try bump 255→0 and the first valid one (off-curve) is the canonical bump. For the curve check, use `filippo.io/edwards25519` which is a transitive dependency of `golang.org/x/crypto` and is already in go.sum. Call `new(edwards25519.Point).SetBytes(candidate)` — if it returns an error, the point is off-curve (valid PDA).

- `CreateATAInstruction(funder, wallet, mint Pubkey) Instruction`
  - ProgramID: `ATAProgramID`
  - Accounts (7): `[{funder, signer+writable}, {ata_address, writable}, {wallet, readonly}, {mint, readonly}, {SystemProgramID, readonly}, {TokenProgramID, readonly}, {SysvarRentID, readonly}]`
  - Data: empty (the ATA program's create instruction has no data)
  - Must call `DeriveATA(wallet, mint)` internally to compute the ATA address

**Test requirements:**
- `DeriveATA` with known wallet + USDC mint → verify against known ATA address
  - Wallet: `7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU` (well-known test address)
  - USDC Mint: `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v`
  - Expected ATA: compute and hardcode from a reference implementation
- `CreateATAInstruction`: verify 7 accounts, correct flags, empty data, correct program ID
- Error case: `DeriveATA` should never fail for valid pubkeys (bump 255 almost always works)

**Verify:** `go test ./internal/transaction/ -run TestATA -v`
**Commit:** `feat(transaction): add ATA derivation and creation instruction`

---

## Phase 2: New RPC Methods

All tasks depend on Phase 1 completing (for types). Tasks within this phase run in parallel.

### Task 2.1: New RPC Method Functions
**File:** `internal/blockchain/solana.go` (MODIFY — append new functions)
**Test:** `internal/blockchain/solana_test.go` (MODIFY — append new tests)
**Depends:** Phase 1 (for `SignatureInfo`, `TransactionDetail` types — but these are defined locally in blockchain package, so only conceptual dependency)

**What to implement — add these functions to the existing file:**

1. **New types** (add at top of file, after existing types):
```go
// SignatureInfo represents a transaction signature from getSignaturesForAddress.
type SignatureInfo struct {
    Signature string
    Slot      uint64
    BlockTime *int64
    Err       interface{}
    Memo      *string
}

// TransactionDetail represents parsed transaction data from getTransaction.
type TransactionDetail struct {
    Signature    string
    Slot         uint64
    BlockTime    *int64
    Fee          uint64
    Instructions []ParsedInstruction
    PreBalances  []uint64
    PostBalances []uint64
    Err          interface{}
}

// ParsedInstruction represents a parsed instruction from a transaction.
type ParsedInstruction struct {
    ProgramID string
    Program   string // "system", "spl-token", etc.
    Type      string // "transfer", "transferChecked", etc.
    Info      map[string]interface{}
}
```

2. **`GetLatestBlockhash(client *RPCClient) (string, uint64, error)`**
   - Call `getLatestBlockhash` with `[{"commitment": "finalized"}]`
   - Parse: `result.value.blockhash` (string) and `result.value.lastValidBlockHeight` (uint64)
   - Return blockhash string, lastValidBlockHeight, error

3. **`SendTransaction(client *RPCClient, base64Tx string) (string, error)`**
   - Call `sendTransaction` with `[base64Tx, {"encoding": "base64", "skipPreflight": false, "preflightCommitment": "confirmed"}]`
   - Result is a string (the transaction signature)
   - Return signature, error

4. **`GetSignaturesForAddress(client *RPCClient, address string, limit int, before string) ([]SignatureInfo, error)`**
   - Build opts map: `{"limit": limit}`. If `before != ""`, add `"before": before`
   - Call `getSignaturesForAddress` with `[address, opts]`
   - Parse array of `SignatureInfo`
   - Return slice, error

5. **`GetTransaction(client *RPCClient, signature string) (*TransactionDetail, error)`**
   - Call `getTransaction` with `[signature, {"encoding": "jsonParsed", "maxSupportedTransactionVersion": 0}]`
   - Parse the deeply nested response: `result.transaction.message.instructions`, `result.meta.fee`, `result.meta.preBalances`, `result.meta.postBalances`, `result.blockTime`, `result.slot`
   - For each instruction, extract `programId`, `program`, `parsed.type`, `parsed.info`
   - Return `*TransactionDetail`, error. Return `nil, nil` if transaction not found (null result)

6. **`GetMinimumBalanceForRentExemption(client *RPCClient, dataSize uint64) (uint64, error)`**
   - Call `getMinimumBalanceForRentExemption` with `[dataSize]`
   - Result is a uint64 (lamports)
   - Return lamports, error

**Test requirements (mock HTTP server pattern — same as existing tests):**
- `TestGetLatestBlockhash`: mock returns blockhash + height, verify parsing
- `TestSendTransaction`: mock returns signature string, verify params include base64 encoding
- `TestGetSignaturesForAddress`: mock returns array of 2 signatures, verify parsing including optional fields
- `TestGetSignaturesForAddress_WithBefore`: verify `before` cursor is included in params
- `TestGetTransaction`: mock returns full jsonParsed response with system transfer, verify instruction parsing
- `TestGetTransaction_NotFound`: mock returns null result, verify nil return
- `TestGetMinimumBalanceForRentExemption`: mock returns lamports value

**Verify:** `go test ./internal/blockchain/ -run "TestGetLatestBlockhash|TestSendTransaction|TestGetSignatures|TestGetTransaction|TestGetMinimumBalance" -v`
**Commit:** `feat(blockchain): add RPC methods for transaction submission and history`

---

### Task 2.2: New Error Sentinels
**File:** `internal/models/errors.go` (MODIFY — add new sentinel errors)
**Test:** `internal/models/errors_test.go` (MODIFY — add new test cases)
**Depends:** none

**What to implement — add to the existing `var` block:**
```go
// Transfer errors
ErrInvalidRecipient          = errors.New("invalid recipient address")
ErrSendToSelf                = errors.New("cannot send to own address")
ErrInsufficientBalanceForRent = errors.New("insufficient balance for rent exemption")
ErrTransactionFailed         = errors.New("transaction failed on-chain")
ErrBlockhashExpired          = errors.New("blockhash expired — please retry")
```

**Also add to `ExitCode` and `UserMessage` functions:**
- `ErrInvalidRecipient` → exit code 1, message "Invalid recipient address. Check the address and try again."
- `ErrSendToSelf` → exit code 1, message "Cannot send to your own address."
- `ErrInsufficientBalanceForRent` → exit code 1, message "Insufficient SOL to cover rent exemption for new account."
- `ErrTransactionFailed` → exit code 70, message "Transaction failed on-chain. Check explorer for details."
- `ErrBlockhashExpired` → exit code 69, message "Transaction expired. Please try again."

**Test requirements:**
- Verify each new error has correct exit code
- Verify each new error has a non-empty user message
- Verify `errors.Is` works for wrapped errors

**Verify:** `go test ./internal/models/ -v`
**Commit:** `feat(models): add transfer-related error sentinels`

---

## Phase 3: Transfer Service

### Task 3.1: Transfer Service
**File:** `internal/services/transfer.go`
**Test:** `internal/services/transfer_test.go`
**Depends:** Phase 1 (transaction builder), Phase 2 (RPC methods, error sentinels)

**What to implement:**

```go
// TransferService orchestrates SOL and SPL token transfers.
type TransferService struct {
    keystoreService *KeystoreService
    db              *database.Database
}

// NewTransferService creates a new TransferService.
func NewTransferService(keystoreService *KeystoreService, db *database.Database) *TransferService

// SendSOL sends SOL from one address to another.
// Returns the transaction signature.
func (s *TransferService) SendSOL(rpcClient *blockchain.RPCClient, fromAddr, toAddr string, lamports uint64, password string) (string, error)
```

**`SendSOL` implementation:**
1. Validate recipient: `PubkeyFromBase58(toAddr)` — if error, return `ErrInvalidRecipient`
2. Validate not self: if `fromAddr == toAddr`, return `ErrSendToSelf`
3. Unlock keypair: `s.keystoreService.UnlockKeypair(s.db, fromAddr, password)` → `ed25519.PrivateKey`
4. Defer zero private key: `defer func() { for i := range privKey { privKey[i] = 0 } }()`
5. Get latest blockhash: `blockchain.GetLatestBlockhash(rpcClient)`
6. Parse from/to as `transaction.Pubkey`
7. Build instruction: `transaction.SystemTransfer(fromPubkey, toPubkey, lamports)`
8. Parse blockhash: `transaction.PubkeyFromBase58(blockhash)` (blockhash is base58-encoded)
9. Build message: `transaction.NewMessage(fromPubkey, []Instruction{transferIx}, blockhashPubkey)`
10. Sign: `transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})`
11. Encode: `tx.ToBase64()`
12. Submit: `blockchain.SendTransaction(rpcClient, base64Tx)`
13. Return signature

```go
// SendSPL sends SPL tokens from one address to another.
// Creates recipient's ATA if it doesn't exist (sender pays rent).
func (s *TransferService) SendSPL(rpcClient *blockchain.RPCClient, fromAddr, toAddr, mint string, amount uint64, decimals uint8, password string) (string, error)
```

**`SendSPL` implementation:**
1. Validate recipient and not-self (same as SendSOL)
2. Unlock keypair, defer zero
3. Parse all pubkeys: from, to, mint
4. Derive sender ATA: `transaction.DeriveATA(fromPubkey, mintPubkey)`
5. Derive recipient ATA: `transaction.DeriveATA(toPubkey, mintPubkey)`
6. Check if recipient ATA exists: `blockchain.GetAccountInfo(rpcClient, recipientATA.String())`
7. Build instructions slice:
   - If ATA doesn't exist: prepend `transaction.CreateATAInstruction(fromPubkey, toPubkey, mintPubkey)`
   - Append `transaction.TokenTransferChecked(senderATA, mintPubkey, recipientATA, fromPubkey, amount, decimals)`
8. Get blockhash, build message, sign, submit (same as SendSOL)
9. Return signature

```go
// EstimateFee returns the estimated fee in lamports for a transfer.
// baseFee is 5000 lamports per signature. If createATA is true, adds rent exemption cost.
func (s *TransferService) EstimateFee(createATA bool) uint64
```

**`EstimateFee` implementation:**
- Base fee: 5000 lamports
- If `createATA`: add 2_039_280 lamports (rent exemption for 165-byte token account)
- Return total

**Test requirements:**
- **`TestSendSOL_InvalidRecipient`**: pass garbage address, expect `ErrInvalidRecipient`
- **`TestSendSOL_SendToSelf`**: same from/to, expect `ErrSendToSelf`
- **`TestSendSOL_WrongPassword`**: wrong password, expect `ErrCryptoFailed`
- **`TestSendSOL_Success`**: mock RPC server that returns blockhash + accepts sendTransaction, verify signature returned
  - Setup: import a test wallet using `KeystoreService.ImportKeypair` with the test mnemonic
  - Mock: `getLatestBlockhash` returns a known blockhash, `sendTransaction` returns a fake signature
  - Verify: returned signature matches mock response
- **`TestSendSPL_CreatesATA`**: mock `getAccountInfo` returns nil (ATA doesn't exist), verify `sendTransaction` is called
- **`TestEstimateFee`**: verify 5000 without ATA, 5000+2039280 with ATA

**Verify:** `go test ./internal/services/ -run TestTransfer -v`
**Commit:** `feat(services): add TransferService for SOL and SPL token sends`

---

## Phase 4: Send View

### Task 4.1: Send View TUI Messages
**File:** `internal/tui/messages.go` (MODIFY — add new message types)
**Test:** none (message types are just structs)
**Depends:** none

**What to implement — append to existing file:**
```go
// TransferSentMsg is sent when a transfer completes.
type TransferSentMsg struct {
    Signature string
    Err       error
}
```

**Verify:** `go build ./internal/tui/`
**Commit:** `feat(tui): add TransferSentMsg for send flow`

---

### Task 4.2: Send View
**File:** `internal/tui/views/send/send.go`
**Test:** `internal/tui/views/send/send_test.go`
**Depends:** 3.1 (TransferService), 4.1 (messages)

**What to implement:**

A 7-step wizard following the `walletimport` pattern exactly:

```go
package send

type Step int

const (
    StepSelectToken  Step = iota // 0 — cursor selection of SOL or SPL token
    StepRecipient                // 1 — text input for recipient address
    StepAmount                   // 2 — text input for amount
    StepReview                   // 3 — summary, enter to confirm
    StepPassword                 // 4 — masked password input
    StepSending                  // 5 — spinner + async transfer
    StepResult                   // 6 — success/error display
)

type Model struct {
    step           Step
    tokens         []models.TokenBalance // from portfolio
    tokenCursor    int
    selectedToken  models.TokenBalance
    recipientInput textinput.Model
    amountInput    textinput.Model
    passwordInput  textinput.Model
    recipient      string
    amount         uint64       // in smallest unit (lamports or token base units)
    amountDisplay  string       // human-readable
    isSOL          bool
    createATA      bool         // whether recipient ATA needs creation
    estimatedFee   uint64
    signature      string
    spinner        components.SpinnerModel
    err            error

    // Dependencies
    walletAddr     string
    transferSvc    *services.TransferService
    rpcClient      *blockchain.RPCClient
    portfolio      models.PortfolioBalance
    width, height  int
}
```

**Constructor:** `New(walletAddr string, transferSvc *services.TransferService, rpcClient *blockchain.RPCClient, portfolio models.PortfolioBalance) Model`

**Navigation:**
- `esc` at StepSelectToken → `NavigateBackMsg`
- `esc` at other steps → go back one step
- `enter` advances to next step (with validation)
- At StepResult, any key → `NavigateBackMsg`

**Step logic:**
- **StepSelectToken**: Show SOL + all SPL tokens with balances > 0. j/k navigation. Enter selects.
- **StepRecipient**: Text input. Validate: base58 decode succeeds, 32-44 chars, not sender's address.
- **StepAmount**: Text input. Validate: positive number, not exceeding balance (minus fee for SOL). Support "MAX" keyword. Convert to lamports/base units.
- **StepReview**: Display token, amount, recipient (first 4...last 4), fee estimate, total. Enter confirms.
- **StepPassword**: Masked input. Enter triggers send.
- **StepSending**: Spinner. Fire async cmd that calls `transferSvc.SendSOL` or `transferSvc.SendSPL`. Handle `TransferSentMsg`.
- **StepResult**: Show signature + explorer link `https://solscan.io/tx/{signature}` on success, or error message.

**Test requirements:**
- **`TestNew`**: verify initial step is StepSelectToken
- **`TestSelectToken`**: send j/k keys, verify cursor moves
- **`TestSelectTokenEnter`**: send enter, verify advances to StepRecipient
- **`TestRecipientValidation`**: enter invalid address, verify error displayed
- **`TestRecipientSelfSend`**: enter own address, verify error
- **`TestAmountValidation`**: enter 0, negative, exceeds balance — verify errors
- **`TestAmountMAX`**: enter "MAX", verify amount set to balance minus fee
- **`TestReviewDisplay`**: verify view contains token, amount, recipient, fee
- **`TestEscNavigatesBack`**: at StepSelectToken, esc returns NavigateBackMsg
- **`TestEscGoesBackOneStep`**: at StepRecipient, esc goes to StepSelectToken
- **`TestTransferSentMsg_Success`**: send TransferSentMsg with signature, verify StepResult shows signature
- **`TestTransferSentMsg_Error`**: send TransferSentMsg with error, verify error displayed

**Verify:** `go test ./internal/tui/views/send/ -v`
**Commit:** `feat(tui): add send view with 7-step wizard flow`

---

## Phase 5: Receive View

### Task 5.1: Receive View
**File:** `internal/tui/views/receive/receive.go`
**Test:** `internal/tui/views/receive/receive_test.go`
**Depends:** none (just displays an address)

**What to implement:**

Simple single-screen view — no wizard:

```go
package receive

type Model struct {
    walletAddr string
    walletLabel string
    copied     bool
    copyErr    string
    width, height int
}
```

**Constructor:** `New(walletAddr, walletLabel string) Model`

**Display:**
- Title: "Receive" (styled with `tui.StyleTitle`)
- Wallet label and full address (not truncated)
- Instruction: "Press c to copy address to clipboard"
- After copy: "Address copied!" (green) or "Clipboard not available — copy address manually" (warning)
- Footer: "Send SOL or SPL tokens to this address"
- `esc` → `NavigateBackMsg`

**Clipboard:** Use `os/exec` to detect and call platform clipboard:
```go
func copyToClipboard(text string) error {
    // Try pbcopy (macOS), then xclip, then xsel, then clip.exe (WSL)
    for _, cmd := range [][]string{
        {"pbcopy"},
        {"xclip", "-selection", "clipboard"},
        {"xsel", "--clipboard", "--input"},
        {"clip.exe"},
    } {
        c := exec.Command(cmd[0], cmd[1:]...)
        c.Stdin = strings.NewReader(text)
        if err := c.Run(); err == nil {
            return nil
        }
    }
    return fmt.Errorf("no clipboard command available")
}
```

**Test requirements:**
- **`TestNew`**: verify view contains wallet address
- **`TestViewContainsAddress`**: full address displayed
- **`TestViewContainsLabel`**: wallet label displayed
- **`TestEscNavigatesBack`**: esc returns NavigateBackMsg
- **`TestCopyKey`**: send 'c' key, verify `copied` flag set (clipboard may fail in test env — that's OK, test the state transition)

**Verify:** `go test ./internal/tui/views/receive/ -v`
**Commit:** `feat(tui): add receive view with clipboard copy`

---

## Phase 6: Activity Service + Enhanced History

### Task 6.1: Activity Service
**File:** `internal/services/activity.go`
**Test:** `internal/services/activity_test.go`
**Depends:** 2.1 (RPC methods)

**What to implement:**

```go
package services

// ActivityItem represents a unified transaction history entry.
type ActivityItem struct {
    Signature    string
    Type         string // "sol_transfer", "spl_transfer", "swap", "program_interaction", "unknown"
    Direction    string // "sent", "received", "self"
    Amount       string // human-readable: "1.5 SOL", "100 USDC"
    Counterparty string // the other address (truncated for display)
    Fee          uint64 // fee in lamports
    Timestamp    int64  // unix timestamp
    Status       string // "confirmed", "failed"
    Slot         uint64
}

// ActivityService fetches and merges on-chain transaction history with local swap records.
type ActivityService struct {
    db *database.Database
}

func NewActivityService(db *database.Database) *ActivityService

// GetActivity fetches on-chain activity for an address and merges with local swap history.
func (s *ActivityService) GetActivity(rpcClient *blockchain.RPCClient, address string, limit int, before string) ([]ActivityItem, error)
```

**`GetActivity` implementation:**
1. Fetch signatures: `blockchain.GetSignaturesForAddress(rpcClient, address, limit, before)`
2. For each signature, fetch details: `blockchain.GetTransaction(rpcClient, sig.Signature)`
3. Classify each transaction using `classifyTransaction(detail, address)`:
   - If instructions contain program="system" + type="transfer" → "sol_transfer"
   - If instructions contain program="spl-token" + type="transfer" or "transferChecked" → "spl_transfer"
   - Otherwise → "program_interaction" or "unknown"
4. Determine direction:
   - For SOL transfers: compare pre/post balances — if address balance decreased → "sent", increased → "received"
   - For SPL transfers: check if source owner matches address → "sent", else "received"
5. Fetch local swap history: `s.db.GetSwapHistory(address, limit)`
6. Merge: for any ActivityItem whose signature matches a swap record, override Type to "swap" and enrich with swap details
7. Sort by timestamp descending
8. Return merged list

**Helper functions:**
- `classifyTransaction(detail *blockchain.TransactionDetail, address string) ActivityItem`
- `truncateAddress(addr string) string` — returns `"7xKp...3mFq"` (first 4 + last 4)
- `formatAmount(lamports uint64) string` — returns `"1.5 SOL"` etc.

**Test requirements:**
- **`TestClassifySOLTransfer`**: mock transaction with system transfer instruction → type "sol_transfer"
- **`TestClassifySPLTransfer`**: mock transaction with spl-token transferChecked → type "spl_transfer"
- **`TestClassifyUnknown`**: mock transaction with unknown program → type "unknown"
- **`TestDirectionSent`**: address balance decreased → "sent"
- **`TestDirectionReceived`**: address balance increased → "received"
- **`TestMergeWithSwapHistory`**: activity item signature matches swap record → type becomes "swap"
- **`TestTruncateAddress`**: verify format "xxxx...xxxx"
- **`TestGetActivity_Empty`**: no signatures → empty list

**Verify:** `go test ./internal/services/ -run TestActivity -v`
**Commit:** `feat(services): add ActivityService for on-chain transaction history`

---

### Task 6.2: Enhanced History View
**File:** `internal/tui/views/history/history.go` (MODIFY)
**Test:** `internal/tui/views/history/history_test.go` (MODIFY)
**Depends:** 6.1 (ActivityService)

**What to implement — modify the existing history view:**

1. **Change constructor** to accept `ActivityService` + `RPCClient` instead of `Database`:
```go
func New(walletAddr string, activitySvc *services.ActivityService, rpcClient *blockchain.RPCClient) Model
```

2. **Change Model fields:**
   - Replace `entries []models.SwapHistoryEntry` with `items []services.ActivityItem`
   - Replace `db *database.Database` with `activitySvc *services.ActivityService` and `rpcClient *blockchain.RPCClient`
   - Add `lastSignature string` for pagination cursor

3. **Change message type:**
```go
type ActivityLoadedMsg struct {
    Items []services.ActivityItem
    Err   error
}
```

4. **Change `loadHistory`** to call `activitySvc.GetActivity(rpcClient, address, pageSize, before)`

5. **Change View rendering** — each row shows:
   - Direction icon: `↑` (sent, red), `↓` (received, green), `⇄` (swap, purple)
   - Type + amount: "Sent 1.5 SOL", "Received 100 USDC", "Swapped 0.5 SOL → 25 USDC"
   - Counterparty: `→ 7xKp...3mFq` or `← 9aBc...dEf`
   - Relative time: "2 min ago", "1 hour ago"
   - Status: "confirmed" (green) or "failed" (red)

6. **Pagination**: use `before` cursor from last item's signature instead of page offset

**Test requirements — update existing tests + add new ones:**
- **Update `TestNew`**: title should still contain "History" (not "Swap History")
- **Update `TestViewContainsEntries`**: use `ActivityLoadedMsg` instead of `HistoryLoadedMsg`
- **Add `TestActivityLoadedMsg`**: send ActivityLoadedMsg with items, verify view renders
- **Add `TestDirectionIcons`**: verify ↑ for sent, ↓ for received, ⇄ for swap
- **Add `TestColorCoding`**: verify sent is red-styled, received is green-styled
- **Keep existing navigation tests** (esc, j/k, pagination)

**Verify:** `go test ./internal/tui/views/history/ -v`
**Commit:** `feat(tui): enhance history view with on-chain activity and direction indicators`

---

## Phase 7: Wallet Generate

### Task 7.1: BIP39 Mnemonic Generation
**File:** `internal/keystore/bip39.go` (MODIFY — add GenerateMnemonic)
**Test:** `internal/keystore/bip39_test.go` (CREATE — the file doesn't exist yet)
**Depends:** none

**What to implement — add to existing file:**

```go
// GenerateMnemonic generates a new BIP39 mnemonic phrase.
// bitSize must be 128 (12 words) or 256 (24 words).
func GenerateMnemonic(bitSize int) (string, error) {
    if bitSize != 128 && bitSize != 256 {
        return "", fmt.Errorf("bitSize must be 128 or 256, got %d", bitSize)
    }
    entropy, err := bip39.NewEntropy(bitSize)
    if err != nil {
        return "", fmt.Errorf("generate entropy: %w", err)
    }
    mnemonic, err := bip39.NewMnemonic(entropy)
    if err != nil {
        return "", fmt.Errorf("generate mnemonic: %w", err)
    }
    return mnemonic, nil
}
```

The `go-bip39` library already has `NewEntropy` and `NewMnemonic` — we just wrap them.

**Test requirements:**
- **`TestGenerateMnemonic_12Words`**: bitSize=128, verify result has 12 words, passes `ValidateMnemonic`
- **`TestGenerateMnemonic_24Words`**: bitSize=256, verify result has 24 words, passes `ValidateMnemonic`
- **`TestGenerateMnemonic_InvalidBitSize`**: bitSize=64, expect error
- **`TestGenerateMnemonic_Uniqueness`**: generate 2 mnemonics, verify they're different (probabilistic but essentially guaranteed)
- **`TestGenerateMnemonic_RoundTrip`**: generate → validate → convert to seed → verify seed is 64 bytes

**Verify:** `go test ./internal/keystore/ -v`
**Commit:** `feat(keystore): add BIP39 mnemonic generation`

---

### Task 7.2: Wallet Import — Add Generate Flow
**File:** `internal/tui/views/walletimport/walletimport.go` (MODIFY)
**Test:** `internal/tui/views/walletimport/walletimport_test.go` (MODIFY or CREATE)
**Depends:** 7.1 (GenerateMnemonic)

**What to implement — modify the existing wizard:**

1. **Add new steps** — insert `StepChoice` at the beginning and `StepShowMnemonic` after it:
```go
const (
    StepChoice         Step = iota // NEW: "Import existing" or "Create new"
    StepSeedPhrase                 // existing (only for import flow)
    StepShowMnemonic               // NEW: display generated mnemonic (only for create flow)
    StepWalletType                 // existing
    StepAccountIndex               // existing
    StepPassword                   // existing
    StepConfirmPassword            // existing
    StepLabel                      // existing
    StepImporting                  // existing
    StepSuccess                    // existing
)
```

2. **Add fields to Model:**
```go
isGenerate     bool     // true if "Create new wallet" was chosen
generateChoice int      // cursor for choice step
mnemonicConfirmed bool  // user confirmed they saved the phrase
```

3. **StepChoice logic:**
   - Display two options: "Import existing wallet" / "Create new wallet"
   - j/k to navigate, enter to select
   - If "Import existing": `m.step = StepSeedPhrase` (existing flow)
   - If "Create new": generate mnemonic via `keystore.GenerateMnemonic(128)`, store in `m.words`, set `m.isGenerate = true`, advance to `StepShowMnemonic`

4. **StepShowMnemonic logic:**
   - Display words in a numbered 3-column grid (4 rows × 3 cols for 12 words)
   - Warning: "Write down these words and store them safely. You will NOT be able to see them again."
   - Instruction: "Press Enter to confirm you have saved your recovery phrase"
   - Enter → advance to `StepWalletType`
   - Esc → go back to `StepChoice` (and clear generated words)

5. **Update step numbering** — `totalSteps` and step indicator need adjustment based on flow (import vs generate)

6. **Update esc handling** — at `StepSeedPhrase`, esc goes to `StepChoice` (not NavigateBackMsg). At `StepChoice`, esc → `NavigateBackMsg`.

**Test requirements:**
- **`TestInitialStepIsChoice`**: verify initial step is StepChoice
- **`TestChoiceImport`**: select "Import existing", verify goes to StepSeedPhrase
- **`TestChoiceGenerate`**: select "Create new", verify goes to StepShowMnemonic, words are populated
- **`TestShowMnemonicDisplay`**: verify view contains numbered words
- **`TestShowMnemonicEnter`**: enter advances to StepWalletType
- **`TestShowMnemonicEsc`**: esc goes back to StepChoice
- **`TestEscAtChoiceNavigatesBack`**: esc at StepChoice returns NavigateBackMsg
- **Keep existing tests** for the import flow (they may need step number adjustments)

**Verify:** `go test ./internal/tui/views/walletimport/ -v`
**Commit:** `feat(tui): add wallet generation flow to import wizard`

---

## Phase 8: Wallet Rename

### Task 8.1: Database — UpdateWalletLabel
**File:** `internal/database/wallets.go` (MODIFY — add method)
**Test:** `internal/database/wallets_test.go` (MODIFY — add test)
**Depends:** none

**What to implement — add to existing file:**

```go
// UpdateWalletLabel updates the label for a wallet.
func (d *Database) UpdateWalletLabel(address, label string) error {
    result, err := d.db.Exec(
        `UPDATE wallets SET label = ? WHERE address = ?`,
        label, address,
    )
    if err != nil {
        return fmt.Errorf("updating wallet label %q: %w", address, err)
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("checking rows affected for wallet label update %q: %w", address, err)
    }
    if rows == 0 {
        return fmt.Errorf("updating wallet label %q: %w", address, models.ErrWalletNotFound)
    }
    return nil
}
```

Also update the label in `encrypted_keypairs` table for consistency:
```go
// After the wallets update, also update encrypted_keypairs label
_, _ = d.db.Exec(`UPDATE encrypted_keypairs SET label = ? WHERE address = ?`, label, address)
```

**Test requirements:**
- **`TestUpdateWalletLabel`**: insert wallet, update label, verify via GetWalletByAddress
- **`TestUpdateWalletLabel_NotFound`**: update non-existent wallet, expect ErrWalletNotFound
- **`TestUpdateWalletLabel_Empty`**: update with empty label — should succeed (no validation at DB level)

**Verify:** `go test ./internal/database/ -run TestUpdateWalletLabel -v`
**Commit:** `feat(database): add UpdateWalletLabel method`

---

### Task 8.2: Wallet Status — Add Rename
**File:** `internal/tui/views/walletstatus/walletstatus.go` (MODIFY)
**Test:** `internal/tui/views/walletstatus/walletstatus_test.go` (MODIFY)
**Depends:** 8.1 (UpdateWalletLabel)

**What to implement — modify the existing wallet status view:**

1. **Add fields to Model:**
```go
renaming      bool            // in rename sub-mode
renameInput   textinput.Model // text input for new label
db            *database.Database
```

2. **Update constructor** to accept `*database.Database`:
```go
func New(walletMgr *wallet.WalletManager, address string, db *database.Database) Model
```

3. **Add rename keybinding** — in the `tea.KeyMsg` handler, add:
```go
case "R": // capital R to avoid conflict with refresh 'r'
    m.renaming = true
    m.renameInput = textinput.New()
    m.renameInput.Placeholder = "New wallet name"
    m.renameInput.CharLimit = 32
    m.renameInput.Width = 30
    m.renameInput.SetValue(m.portfolio.WalletAddress) // pre-fill with current label if available
    m.renameInput.Focus()
    return m, m.renameInput.Focus()
```

4. **Handle rename sub-mode** — when `m.renaming`:
   - `enter`: validate non-empty, call `db.UpdateWalletLabel(m.address, newLabel)`, set `m.renaming = false`, refresh display
   - `esc`: cancel rename, set `m.renaming = false`
   - Other keys: delegate to `m.renameInput.Update(msg)`

5. **Update View** — when `m.renaming`, show rename overlay:
```
Rename wallet:
[text input]
Enter to save, Esc to cancel
```

6. **Update status bar** to show `[R]ename` hint

**Test requirements:**
- **`TestRenameKeyBinding`**: press 'R', verify `renaming` is true
- **`TestRenameEscCancels`**: enter rename mode, press esc, verify `renaming` is false
- **`TestRenameEnterSaves`**: enter rename mode, type new name, press enter — verify DB updated (need mock or in-memory DB)
- **`TestRenameEmptyRejects`**: enter rename mode, press enter with empty input — verify error

**Verify:** `go test ./internal/tui/views/walletstatus/ -v`
**Commit:** `feat(tui): add wallet rename to status view`

---

## Phase 9: Auto-Refresh

### Task 9.1: Auto-Refresh in Wallet Status
**File:** `internal/tui/views/walletstatus/walletstatus.go` (MODIFY)
**Test:** `internal/tui/views/walletstatus/walletstatus_test.go` (MODIFY)
**Depends:** none (modifies existing view)

**What to implement:**

1. **Add fields to Model:**
```go
lastRefresh time.Time
tickActive  bool
```

2. **Add tick message type:**
```go
type autoRefreshTickMsg struct{}
```

3. **Update Init** to start the tick:
```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Init(),
        m.loadPortfolio(),
        m.scheduleRefresh(),
    )
}

func (m Model) scheduleRefresh() tea.Cmd {
    return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
        return autoRefreshTickMsg{}
    })
}
```

4. **Handle tick in Update:**
```go
case autoRefreshTickMsg:
    if !m.loading { // don't refresh if already loading
        m.loading = true
        m.lastRefresh = time.Now()
        m.spinner = components.NewSpinner("Refreshing...")
        return m, tea.Batch(m.spinner.Init(), m.refreshPortfolio(), m.scheduleRefresh())
    }
    return m, m.scheduleRefresh() // reschedule even if skipped
```

5. **Update manual refresh** (`r` key) to reset the timer:
```go
case "r":
    m.loading = true
    m.lastRefresh = time.Now()
    m.spinner = components.NewSpinner("Refreshing portfolio...")
    return m, tea.Batch(m.spinner.Init(), m.refreshPortfolio(), m.scheduleRefresh())
```

6. **Update View footer** to show last refresh time:
```go
if !m.lastRefresh.IsZero() {
    elapsed := time.Since(m.lastRefresh)
    footer += fmt.Sprintf(" | Last updated: %s ago", formatDuration(elapsed))
}
```

Helper:
```go
func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return fmt.Sprintf("%ds", int(d.Seconds()))
    }
    if d < time.Hour {
        return fmt.Sprintf("%dm", int(d.Minutes()))
    }
    return fmt.Sprintf("%dh", int(d.Hours()))
}
```

**Test requirements:**
- **`TestAutoRefreshTickMsg`**: send `autoRefreshTickMsg`, verify loading state triggered
- **`TestAutoRefreshSkipsWhenLoading`**: set loading=true, send tick, verify no double-load
- **`TestManualRefreshResetsTimer`**: press 'r', verify lastRefresh updated
- **`TestLastRefreshDisplay`**: set lastRefresh, verify view contains "Last updated"

**Verify:** `go test ./internal/tui/views/walletstatus/ -v`
**Commit:** `feat(tui): add 30-second auto-refresh to wallet status view`

---

## Phase 10: ViewFactory Wiring + Integration

### Task 10.1: ViewFactory Wiring
**File:** `cmd/hound/main.go` (MODIFY)
**Test:** none (integration — verified by `go build`)
**Depends:** All previous phases

**What to implement:**

1. **Add new services to `deps` struct:**
```go
type deps struct {
    // ... existing fields ...
    transferSvc  *services.TransferService
    activitySvc  *services.ActivityService
}
```

2. **Initialize new services in `initDeps`:**
```go
transferSvc := services.NewTransferService(keystoreSvc, db)
activitySvc := services.NewActivityService(db)
```

3. **Add new view cases to `makeViewFactory`:**
```go
case "send":
    addr, _ := data.(string)
    if addr == "" {
        if pw, err := d.walletMgr.GetPrimaryWallet(); err == nil {
            addr = pw.Address
        }
    }
    // Need portfolio for token selection
    portfolio, _ := d.walletMgr.GetCachedPortfolio(addr)
    m := send.New(addr, d.transferSvc, d.rpcClient, portfolio)
    return m

case "receive":
    addr, _ := data.(string)
    label := ""
    if w, err := d.db.GetWalletByAddress(addr); err == nil {
        label = w.Label
    }
    m := receive.New(addr, label)
    return m
```

4. **Update history view case:**
```go
case "history":
    addr, _ := data.(string)
    m := history.New(addr, d.activitySvc, d.rpcClient)
    return m
```

5. **Update wallet-status case** to pass DB:
```go
case "wallet-status":
    addr, _ := data.(string)
    m := walletstatus.New(d.walletMgr, addr, d.db)
    return m
```

6. **Add imports** for new packages:
```go
"github.com/dvrd/hound/internal/tui/views/send"
"github.com/dvrd/hound/internal/tui/views/receive"
```

7. **Update NavigateMsg view names** in the `messages.go` comment to include `"send"`, `"receive"`

**Verify:** `go build ./cmd/hound/`
**Commit:** `feat(main): wire send, receive, activity views into ViewFactory`

---

### Task 10.2: Integration Testing
**File:** `internal/transaction/integration_test.go`
**Test:** (this IS the test)
**Depends:** All previous phases

**What to implement:**

End-to-end test that builds a complete SOL transfer transaction and verifies the serialized output:

```go
func TestFullSOLTransferTransaction(t *testing.T) {
    // 1. Generate a deterministic keypair from known seed
    seed := make([]byte, 32)
    for i := range seed { seed[i] = byte(i) }
    privKey := ed25519.NewKeyFromSeed(seed)
    pubKey := privKey.Public().(ed25519.PublicKey)

    var fromPubkey transaction.Pubkey
    copy(fromPubkey[:], pubKey)

    // 2. Create a recipient
    recipientSeed := make([]byte, 32)
    for i := range recipientSeed { recipientSeed[i] = byte(i + 32) }
    recipientPriv := ed25519.NewKeyFromSeed(recipientSeed)
    recipientPub := recipientPriv.Public().(ed25519.PublicKey)
    var toPubkey transaction.Pubkey
    copy(toPubkey[:], recipientPub)

    // 3. Build transfer instruction
    ix := transaction.SystemTransfer(fromPubkey, toPubkey, 1_000_000_000) // 1 SOL

    // 4. Use a fake blockhash (32 zero bytes)
    var blockhash transaction.Pubkey

    // 5. Build message
    msg := transaction.NewMessage(fromPubkey, []transaction.Instruction{ix}, blockhash)

    // 6. Verify message structure
    if msg.Header.NumRequiredSignatures != 1 {
        t.Errorf("expected 1 required signature, got %d", msg.Header.NumRequiredSignatures)
    }
    if len(msg.AccountKeys) != 3 { // from, to, system program
        t.Errorf("expected 3 account keys, got %d", len(msg.AccountKeys))
    }

    // 7. Sign transaction
    tx, err := transaction.NewTransaction(msg, []ed25519.PrivateKey{privKey})
    if err != nil {
        t.Fatalf("NewTransaction failed: %v", err)
    }

    // 8. Verify signature
    serializedMsg := msg.Serialize()
    if !ed25519.Verify(pubKey, serializedMsg, tx.Signatures[0]) {
        t.Error("signature verification failed")
    }

    // 9. Verify serialization round-trip
    serialized := tx.Serialize()
    if len(serialized) == 0 {
        t.Error("serialized transaction is empty")
    }

    // 10. Verify base64
    b64 := tx.ToBase64()
    if b64 == "" {
        t.Error("base64 transaction is empty")
    }

    // Verify base64 decodes back
    decoded, err := base64.StdEncoding.DecodeString(b64)
    if err != nil {
        t.Fatalf("base64 decode failed: %v", err)
    }
    if !bytes.Equal(decoded, serialized) {
        t.Error("base64 decode doesn't match serialized bytes")
    }
}
```

Also add an SPL transfer integration test:
```go
func TestFullSPLTransferTransaction(t *testing.T) {
    // Similar to above but with TokenTransferChecked instruction
    // Verify: 2 instructions if ATA creation needed, correct account ordering
}
```

**Verify:** `go test ./internal/transaction/ -run TestFull -v`
**Commit:** `test(transaction): add end-to-end transaction building integration tests`

---

## Full Verification

After all phases are complete, run the full test suite:

```bash
go test ./... -count=1
```

This should pass all existing 20 packages plus the new ones:
- `internal/transaction/` (new)
- `internal/services/` (modified — transfer + activity tests added)
- `internal/blockchain/` (modified — new RPC method tests)
- `internal/models/` (modified — new error tests)
- `internal/keystore/` (modified — generation tests)
- `internal/database/` (modified — UpdateWalletLabel test)
- `internal/tui/views/send/` (new)
- `internal/tui/views/receive/` (new)
- `internal/tui/views/history/` (modified)
- `internal/tui/views/walletimport/` (modified)
- `internal/tui/views/walletstatus/` (modified)

Also verify the binary builds:
```bash
go build ./cmd/hound/
```

---

## Summary of All Files

### New Files (13)
| File | Phase | Description |
|------|-------|-------------|
| `internal/transaction/encoding.go` | 1.1 | Compact-u16 encoding |
| `internal/transaction/encoding_test.go` | 1.1 | Encoding tests |
| `internal/transaction/types.go` | 1.2 | Core Solana types |
| `internal/transaction/types_test.go` | 1.2 | Type tests |
| `internal/transaction/programs.go` | 1.3 | Program ID constants |
| `internal/transaction/programs_test.go` | 1.3 | Program ID tests |
| `internal/transaction/message.go` | 1.4 | Message builder |
| `internal/transaction/message_test.go` | 1.4 | Message tests |
| `internal/transaction/transaction.go` | 1.5 | Transaction signing |
| `internal/transaction/transaction_test.go` | 1.5 | Transaction tests |
| `internal/transaction/system.go` | 1.6 | System program instructions |
| `internal/transaction/system_test.go` | 1.6 | System instruction tests |
| `internal/transaction/token.go` | 1.7 | Token program instructions |
| `internal/transaction/token_test.go` | 1.7 | Token instruction tests |
| `internal/transaction/ata.go` | 1.8 | ATA derivation + creation |
| `internal/transaction/ata_test.go` | 1.8 | ATA tests |
| `internal/transaction/integration_test.go` | 10.2 | E2E transaction tests |
| `internal/services/transfer.go` | 3.1 | Transfer service |
| `internal/services/transfer_test.go` | 3.1 | Transfer service tests |
| `internal/services/activity.go` | 6.1 | Activity service |
| `internal/services/activity_test.go` | 6.1 | Activity service tests |
| `internal/tui/views/send/send.go` | 4.2 | Send view |
| `internal/tui/views/send/send_test.go` | 4.2 | Send view tests |
| `internal/tui/views/receive/receive.go` | 5.1 | Receive view |
| `internal/tui/views/receive/receive_test.go` | 5.1 | Receive view tests |
| `internal/keystore/bip39_test.go` | 7.1 | BIP39 generation tests |

### Modified Files (10)
| File | Phase | Changes |
|------|-------|---------|
| `internal/blockchain/solana.go` | 2.1 | +5 RPC methods, +3 types |
| `internal/blockchain/solana_test.go` | 2.1 | +7 test functions |
| `internal/models/errors.go` | 2.2 | +5 sentinel errors, ExitCode/UserMessage updates |
| `internal/models/errors_test.go` | 2.2 | +5 error test cases |
| `internal/tui/messages.go` | 4.1 | +TransferSentMsg |
| `internal/keystore/bip39.go` | 7.1 | +GenerateMnemonic function |
| `internal/tui/views/walletimport/walletimport.go` | 7.2 | +StepChoice, +StepShowMnemonic, generate flow |
| `internal/database/wallets.go` | 8.1 | +UpdateWalletLabel method |
| `internal/database/wallets_test.go` | 8.1 | +3 test functions |
| `internal/tui/views/walletstatus/walletstatus.go` | 8.2, 9.1 | +rename mode, +auto-refresh |
| `internal/tui/views/walletstatus/walletstatus_test.go` | 8.2, 9.1 | +rename tests, +auto-refresh tests |
| `internal/tui/views/history/history.go` | 6.2 | Rewrite to use ActivityService |
| `internal/tui/views/history/history_test.go` | 6.2 | Update tests for ActivityItem |
| `cmd/hound/main.go` | 10.1 | +TransferService, +ActivityService, +send/receive views |

### New Dependency (1)
| Package | Reason |
|---------|--------|
| `filippo.io/edwards25519` | Ed25519 curve point check for PDA derivation (already transitive dep of `golang.org/x/crypto`) |

Verify it's already available:
```bash
go list -m filippo.io/edwards25519
```
If not in go.mod, add explicitly:
```bash
go get filippo.io/edwards25519
```
