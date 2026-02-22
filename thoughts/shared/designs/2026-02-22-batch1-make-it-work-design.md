---
date: 2026-02-22
topic: "Batch 1 — Make It Work (Critical + High Bug Fixes)"
status: validated
---

# Batch 1 — Make It Work

## Problem Statement

The Tier 1 features (send, receive, activity) are fully implemented but have 8 critical/high bugs that prevent real usage. Send/receive views are unreachable from the TUI. Amounts are silently truncated. Transaction serialization is non-deterministic. Decrypted seeds leak in memory.

## Constraints

- All fixes must be backward-compatible — no schema changes, no API changes
- Tests must continue passing after each fix
- Fixes are independent and can be applied in any order

## Fixes

### Fix 1: C2 — Wire send/receive keybindings

**Problem**: No view emits `NavigateMsg{View: "send"}` or `NavigateMsg{View: "receive"}`. The features are dead code.

**Fix**:
- `walletlist.go`: Add keybinding `"S"` (capital S, since lowercase `s` is "status") → navigate to `"send"` with selected wallet address
- `walletlist.go`: Add keybinding `"R"` (capital R, since lowercase `r` is "refresh") → navigate to `"receive"` with selected wallet address  
- `walletstatus.go`: Add keybinding `"s"` → navigate to `"send"` with current wallet address
- `walletstatus.go`: Add keybinding `"c"` → navigate to `"receive"` with current wallet address
- Update status bar text in both views to show the new keybindings

### Fix 2: C3 — Fix float→uint64 truncation

**Problem**: `uint64(0.1 * 1e9)` = `99999999` not `100000000`. Silent value loss on every transaction.

**Fix in `send.go`**:
- Replace `uint64(amountFloat * math.Pow10(decimals))` with `uint64(math.Round(amountFloat * math.Pow10(decimals)))`
- Same fix in `maxSendable()` for `baseBalance` calculation
- Add `math.Round` to all float→uint64 conversions in the file

### Fix 3: C4 — Sort account groups deterministically

**Problem**: Go map iteration is random → account ordering in message is non-deterministic → different signatures for same logical transaction.

**Fix in `message.go`**:
- After building each of the 4 account groups (writableSigners, readonlySigners, writableNonSigners, readonlyNonSigners), sort each group by pubkey bytes lexicographically using `sort.Slice` with `bytes.Compare`
- Import `bytes` and `sort`

### Fix 4: C5 — Zero plaintext seed in UnlockKeypair

**Problem**: The decrypted 32-byte seed at `keystore.go:140` is never zeroed. It persists in memory.

**Fix in `services/keystore.go`**:
- After `plaintext, err := keystore.Decrypt(...)`, add `defer keystore.ZeroBytes(plaintext)` before the error check

### Fix 5: H2 — Re-init views on back-navigation

**Problem**: `navigateBack()` doesn't call `Init()` on the popped view. Wallet list shows stale data after import.

**Fix in `app.go`**:
- In `navigateBack()`, after popping the view and sending WindowSizeMsg, also call `a.currentView.Init()` and batch the resulting command

### Fix 6: H3 — Add balance checks before sending

**Problem**: No balance verification before building the transaction. User gets cryptic RPC error instead of "insufficient balance".

**Fix in `services/transfer.go`**:
- `SendSOL`: After unlocking keypair, call `blockchain.GetBalance(rpcClient, fromAddr)` and compare against `lamports + 5000` (amount + fee). Return `models.ErrInsufficientBalance` if insufficient.
- `SendSPL`: After unlocking keypair, call `blockchain.GetTokenAccountsByOwner(rpcClient, fromAddr)`, find the sender's token account for the given mint, verify balance >= amount. Also verify SOL balance >= 5000 (fee) + potential ATA rent.

### Fix 7: H8 — Set createATA flag for accurate fee estimate

**Problem**: `send.go` `createATA` field is never set to `true`. Fee estimate always shows base fee even when ATA creation is needed.

**Fix in `send.go`**:
- In `doTransfer()`, before calling `SendSPL`, check if recipient ATA exists via `blockchain.GetAccountInfo(rpcClient, recipientATAAddr)`. But this is too late for the fee estimate shown at StepReview.
- Better approach: Add an async check in the transition from StepRecipient → StepAmount. After validating the recipient address, if the selected token is NOT SOL, derive the recipient's ATA and check existence via RPC. Set `m.createATA` based on the result. Show a spinner during this check.
- However, this adds complexity. Simpler approach: In `estimateFee()`, if the token is NOT SOL, always assume ATA creation is needed (worst case estimate). The actual fee may be lower if the ATA exists. This is safe — overestimating fees is better than underestimating.
- Go with the simpler approach: `estimateFee()` returns `5000 + 2_039_280` for any SPL token transfer, and `5000` for SOL.

### Fix 8: H9 — Append pages in history view

**Problem**: `m.items = msg.Items` overwrites the list on each page load.

**Fix in `history.go`**:
- Change `m.items = msg.Items` to `m.items = append(m.items, msg.Items...)`
- Don't reset cursor on subsequent loads (only reset on first load when `m.items` was empty)
- If `msg.Items` is empty, set a `m.noMorePages` flag and hide the `[n]ext page` hint

## Testing Strategy

- Run `go test ./...` after each fix
- For C4 (deterministic ordering): the existing tests should still pass, and we can add a test that builds the same message twice and verifies identical serialization
- For H3 (balance checks): existing transfer_test.go mock tests need updating to include balance responses

## Open Questions

None.
