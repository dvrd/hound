---
date: 2026-02-22
topic: "Batch 4 — Make It Complete (Remaining Audit Fixes)"
status: validated
---

# Batch 4 — "Make It Complete" — All Remaining Audit Fixes

## Problem Statement

After Batches 1-3 resolved all critical, high, and most medium issues, 13 items remain from the original audit. These range from security hardening (swap transaction validation) to UX polish (cursor clamping, clipboard support). Fixing all of them brings Hound to production-ready quality.

## Constraints

- No CGO — pure Go only
- All 24 test packages must continue passing
- M9 changes `PubkeyFromBytes` return type — all callers must be updated
- M11 changes `ParseWalletType` return type — all callers must be updated
- M5 adds a new RPC method and a new TUI step — moderate complexity
- SECURITY-6 reuses existing transaction parser from `internal/transaction/`

## Fixes by Priority

### SECURITY — Cluster A (3 fixes)

#### Fix 1 — SECURITY-6: Validate Swap Transaction Before Signing

**Problem:** `swap/transaction.go:SignTransaction` blindly signs whatever bytes Jupiter returns. A malicious/compromised API response could drain the wallet.

**Solution:** After base64 decode and structural validation, parse the transaction message to extract:
1. **Fee payer** (first account key) — must match signer's public key
2. **Program IDs** — must be limited to an allowlist: Jupiter v6, Jupiter DCA, System Program, SPL Token, SPL Token-2022, Associated Token Account, Compute Budget
3. If either check fails, return a new `ErrUntrustedTransaction` error without signing

The transaction message format: `[num_signatures(1)][signatures(64*N)][message_bytes]`. The message starts with a header (3 bytes: numRequiredSignatures, numReadonlySignedAccounts, numReadonlyUnsignedAccounts), then compact-u16 encoded account keys, then recent blockhash, then instructions. We can use `internal/transaction/encoding.go` CompactU16 decoder to parse this.

Add a `ValidateSwapTransaction(txBytes []byte, signerPubkey []byte) error` function that does the parsing and validation. Call it from `SignTransaction` before signing.

**Files:** `internal/swap/transaction.go`, `internal/swap/transaction_test.go`

#### Fix 2 — SECURITY-7: Add Taker to Swap Quote Cache Key

**Problem:** Cache key is `inputMint:outputMint:amount` — missing `taker`. Two different wallets get the same cached quote, but the embedded transaction is taker-specific.

**Solution:**
1. Change `cacheKey` to include `taker`: `inputMint + ":" + outputMint + ":" + amount + ":" + taker`
2. Fix silent `strconv.ParseFloat` errors — return `ErrInvalidResponse` if parsing fails

**Files:** `internal/swap/client.go`, `internal/swap/client_test.go`

#### Fix 3 — SECURITY-3: Deprecate Legacy Derivation

**Problem:** Legacy derivation (SHA-256 of mnemonic) is Hound-only — no other wallet can recover these funds. Users can still create new legacy wallets.

**Solution:**
1. Add `// Deprecated: Use DeriveKeypairBIP44 instead. Legacy derivation is not compatible with other wallets.` doc comment to `DeriveKeypairLegacy`
2. In `walletimport.go`, when user selects legacy type (cursor==3), show a warning: "Legacy wallets cannot be recovered in other wallets (Phantom, Solflare, etc.). Use BIP44 Standard instead." Require confirmation before proceeding.
3. Do NOT remove the function — existing legacy wallets still need it for unlock

**Files:** `internal/keystore/keypair.go`, `internal/tui/views/walletimport/walletimport.go`

### HIGH — Transaction Reliability

#### Fix 4 — M5: Transaction Confirmation Polling

**Problem:** After `SendTransaction`, the app immediately shows "Transaction Sent!" without knowing if the tx was confirmed. It could expire or fail.

**Solution:**
1. Add `GetSignatureStatuses(ctx, client, signatures []string)` to `solana.go` — calls `getSignatureStatuses` RPC method
2. Add `AwaitConfirmation(ctx, client, signature string, timeout time.Duration)` to `services/transfer.go` — polls every 2s for up to 30s, returns confirmation status or timeout error
3. In `send.go`, change flow: `StepSending` → call `SendSOL/SendSPL` → on success, enter `StepConfirming` → poll confirmation → `StepResult` with confirmed/failed status
4. Add `StepConfirming` step constant and view rendering (spinner + "Confirming transaction...")
5. `TransferSentMsg` now includes `Confirmed bool` and `ConfirmationErr error`

**Files:** `internal/blockchain/solana.go`, `internal/services/transfer.go`, `internal/tui/views/send/send.go`, `internal/tui/messages.go`

### MEDIUM — Activity Classification (Cluster C)

#### Fix 5 — M3: Parse Inner Instructions for CPI Transfers

**Problem:** `GetTransaction` only extracts top-level instructions. CPI transfers (e.g., DEX programs calling spl-token internally) are invisible.

**Solution:**
1. In `solana.go:GetTransaction`, extract `meta.innerInstructions` from the response and include in `TransactionDetail`
2. Add `InnerInstructions []InnerInstruction` to the `TransactionDetail` struct in `models/`
3. In `activity.go:classifyTransaction`, iterate over both `detail.Instructions` AND `detail.InnerInstructions[*].Instructions` when looking for transfer instructions
4. Inner instructions have the same structure as top-level parsed instructions

**Files:** `internal/blockchain/solana.go`, `internal/models/wallet.go` (or wherever TransactionDetail is), `internal/services/activity.go`

#### Fix 6 — BUG: classifyDirectionFromBalances Uses Wrong Index

**Problem:** Always checks `PreBalances[0]` / `PostBalances[0]` (fee payer), not the queried wallet's index.

**Solution:**
1. Pass the transaction's account keys to `classifyDirectionFromBalances`
2. Find the index of `address` in the account keys array
3. Compare `PreBalances[idx]` vs `PostBalances[idx]`
4. If address not found in account keys, return "unknown" direction

**Files:** `internal/services/activity.go`

#### Fix 7 — BUG: SPL Transfer Direction Detection

**Problem:** In `classifySPLTransfer`, comparing `source == address` and `destination == address` fails for SPL transfers because source/destination are ATA addresses, not wallet addresses.

**Solution:**
1. Compare the `authority` field instead of source/destination for SPL transfers
2. If `authority == address`, direction is "sent" (the wallet authorized the transfer)
3. Otherwise check if the destination ATA's owner matches the address (would require additional data), or fall through to balance-based classification

Actually, the simpler fix: the authority field tells us who initiated the transfer. If `authority == address`, it's "sent". Otherwise, fall through to `classifyDirectionFromBalances` which will correctly identify receives.

**Files:** `internal/services/activity.go`

### MEDIUM — Input Validation (Cluster D)

#### Fix 8 — M1: Base58 Address Validation in Send View

**Problem:** Recipient validation only checks string length (32-44 chars), not base58 validity. Invalid addresses pass validation until the transfer step (after password entry).

**Solution:**
1. In `send.go:updateRecipient`, replace length check with `transaction.PubkeyFromBase58(addr)` call
2. If it returns error, show "Invalid Solana address" to user
3. Remove the old length-only check

**Files:** `internal/tui/views/send/send.go`

#### Fix 9 — M9: PubkeyFromBytes Should Validate Length

**Problem:** `PubkeyFromBytes(b []byte)` silently truncates or zero-pads if input isn't 32 bytes.

**Solution:**
1. Change signature to `PubkeyFromBytes(b []byte) (Pubkey, error)`
2. Return error if `len(b) != 32`
3. Update all callers to handle the error

**Files:** `internal/transaction/types.go`, all callers of `PubkeyFromBytes`

#### Fix 10 — M10: Handle ParseUint Errors in solana.go

**Problem:** `strconv.ParseUint` errors silently produce 0 at 4 locations in `solana.go`.

**Solution:**
1. At each location, check the error from `strconv.ParseUint`
2. If error, return `fmt.Errorf("...: %w", models.ErrRPCInvalidResponse)`
3. Locations: `solana.go` in `GetTokenAccountsByOwner`, `GetTokenAccountBalance`, `GetTokenSupply`, `GetTokenLargestAccounts`

**Files:** `internal/blockchain/solana.go`

#### Fix 11 — M11: ParseWalletType Should Return Error

**Problem:** `ParseWalletType` silently defaults to `WalletTypeLegacy` for unrecognized strings.

**Solution:**
1. Change signature to `ParseWalletType(s string) (WalletType, error)`
2. Return `fmt.Errorf("unknown wallet type: %q", s)` for unrecognized strings
3. Update all callers to handle the error

**Files:** `internal/models/wallet.go`, all callers of `ParseWalletType`

### LOW — UX Polish (Cluster E)

#### Fix 12 — M7: Clamp Cursor After Filter/Sort Change

**Problem:** Toggling `showAll` or changing `sortMode` doesn't clamp `m.cursor`, leading to out-of-bounds selection.

**Solution:**
1. After toggling `showAll` (line 139): add `tokens := m.visibleTokens(); if m.cursor >= len(tokens) { m.cursor = max(0, len(tokens)-1) }`
2. After changing `sortMode` (lines 140-145): same clamping logic
3. Extract to helper: `m.clampCursor()`

**Files:** `internal/tui/views/walletstatus/walletstatus.go`

#### Fix 13 — M8: Update In-Memory Label After Rename

**Problem:** `updateRename` saves to DB but doesn't update `m.wallet.Label`, so the UI doesn't reflect the change until restart.

**Solution:**
1. After successful `UpdateWalletLabel`, set the label in the model's display state
2. The view header should show the updated label

**Files:** `internal/tui/views/walletstatus/walletstatus.go`

#### Fix 14 — M13: Add Wayland Clipboard Support

**Problem:** Clipboard fallback chain is `pbcopy → xclip → xsel → clip.exe`. Missing `wl-copy` for Wayland Linux desktops.

**Solution:**
1. Add `wl-copy` to the fallback chain: `pbcopy → wl-copy → xclip → xsel → clip.exe`
2. `wl-copy` goes before `xclip` because on Wayland, `xclip` may be installed but non-functional

**Files:** `internal/tui/views/receive/receive.go`

## Data Flow (M5 Confirmation Polling)

```
Send Wizard Flow (updated):
  StepSelectToken → StepRecipient → StepAmount → StepReview → StepPassword
    → StepSending (calls SendSOL/SendSPL, gets signature)
    → StepConfirming (polls GetSignatureStatuses every 2s, max 30s)
    → StepResult (shows confirmed/failed/timeout)

GetSignatureStatuses RPC:
  Request: { "method": "getSignatureStatuses", "params": [["sig1"], {"searchTransactionHistory": true}] }
  Response: { "result": { "value": [{ "slot": N, "confirmationStatus": "confirmed"|"finalized"|null, "err": null|{...} }] } }

AwaitConfirmation loop:
  1. Call GetSignatureStatuses(ctx, client, [signature])
  2. If status.confirmationStatus == "confirmed" or "finalized" → return success
  3. If status.err != nil → return tx execution error
  4. If timeout exceeded → return timeout error
  5. Sleep 2s, goto 1
```

## Data Flow (SECURITY-6 Swap Validation)

```
ValidateSwapTransaction(txBytes, signerPubkey):
  1. Skip signature section: message = txBytes[1 + 64*numSignatures:]
  2. Parse header: numRequiredSigs, numReadonlySigned, numReadonlyUnsigned
  3. Parse account keys: read compact-u16 count, then N * 32-byte keys
  4. Verify accountKeys[0] == signerPubkey (fee payer check)
  5. Parse instructions: for each, check programId is in allowlist
  6. Return nil if all checks pass, ErrUntrustedTransaction if any fail
```

## Error Handling

- **SECURITY-6:** Returns `ErrUntrustedTransaction` — swap is aborted, user sees error
- **M5 timeout:** Shows "Transaction may have been sent but confirmation timed out. Check explorer." — not a hard error
- **M9/M11 callers:** Existing callers that ignore errors will now need to handle them. This is a breaking change within the codebase but improves correctness.
- **M10:** Returns `ErrRPCInvalidResponse` — callers already handle this sentinel

## Testing Strategy

- **SECURITY-6:** Unit test with crafted transaction bytes — valid tx passes, wrong fee payer fails, unknown program fails
- **SECURITY-7:** Unit test cache with different takers returns different quotes
- **M5:** Mock RPC server that returns signature statuses; test confirmed, failed, and timeout paths
- **M3:** Mock transaction with inner instructions; verify CPI transfers are classified
- **BUGs:** Test classifyDirectionFromBalances with wallet at various account indices
- **M1:** Test base58 validation catches invalid characters
- **M9:** Test PubkeyFromBytes rejects wrong-sized input
- **M10:** Test ParseUint error propagation with malformed amounts
- **M11:** Test ParseWalletType returns error for unknown strings
- **M7:** Test cursor clamping after filter toggle
- **M13:** Verify wl-copy is in the clipboard chain (unit test the command list)

## Open Questions

None — all decisions are made.
