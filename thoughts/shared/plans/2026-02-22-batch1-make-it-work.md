# Batch 1 — Make It Work: Implementation Plan

**Goal:** Fix 8 critical/high bugs that prevent real usage of send, receive, and activity features.

**Architecture:** All fixes are surgical edits to existing files. No new files, no schema changes, no API changes. Each fix is independent — they touch different files and can be applied in any order.

**Design:** `thoughts/shared/designs/2026-02-22-batch1-make-it-work-design.md`

---

## Dependency Graph

```
Batch A (parallel): 1, 2, 3, 4, 7, 8  [all touch different files — 6 implementers]
Batch B (after A):  5                   [touches app.go — 1 implementer]
Batch C (after A):  6                   [touches transfer.go + test — 1 implementer]
```

Batches B and C are independent of each other and can run in parallel after A completes.

---

## Batch A: Independent Fixes (parallel — 6 implementers)

### Task 1: Wire send/receive keybindings in walletlist + walletstatus

**Files:**
- `internal/tui/views/walletlist/walletlist.go`
- `internal/tui/views/walletlist/walletlist_test.go`
- `internal/tui/views/walletstatus/walletstatus.go`

**Depends:** none

#### Changes to `internal/tui/views/walletlist/walletlist.go`

**Change 1a — Add "S" keybinding for send (after the "w" swap case, before "q"):**

Find:
```go
		case "q":
			return m, tea.Quit
```

Replace with:
```go
		case "S":
			if len(m.wallets) > 0 {
				addr := m.wallets[m.cursor].Address
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "send", Data: addr}
				}
			}
		case "R":
			if len(m.wallets) > 0 {
				addr := m.wallets[m.cursor].Address
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "receive", Data: addr}
				}
			}
		case "q":
			return m, tea.Quit
```

**Change 1b — Update status bar to show new keybindings:**

Find:
```go
	b.WriteString(tui.StyleStatusBar.Render("[i]mport [s]tatus [d]elete [t]okens [w]swap [h]istory [r]efresh [q]uit"))
```

Replace with:
```go
	b.WriteString(tui.StyleStatusBar.Render("[i]mport [s]tatus [d]elete [t]okens [S]end [R]eceive [w]swap [h]istory [r]efresh [q]uit"))
```

#### Changes to `internal/tui/views/walletlist/walletlist_test.go`

**Append these two tests at the end of the file (before the closing of the file):**

Find (the last test function's closing brace — the `TestWindowSizeMsg` test):
```go
func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletlist.Model)
	// Should not panic
}
```

Replace with:
```go
func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletlist.Model)
	// Should not panic
}

func TestSendKeyNavigatesToSend(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("S should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "send" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "send")
	}
	if nav.Data != "7xKXabc1234567890abcdef9mPq" {
		t.Errorf("NavigateMsg.Data = %v, want wallet address", nav.Data)
	}
}

func TestReceiveKeyNavigatesToReceive(t *testing.T) {
	m := loadedModel(sampleWallets(), samplePortfolios())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("R should return a command")
	}
	msg := cmd()
	nav, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.View != "receive" {
		t.Errorf("NavigateMsg.View = %q, want %q", nav.View, "receive")
	}
	if nav.Data != "7xKXabc1234567890abcdef9mPq" {
		t.Errorf("NavigateMsg.Data = %v, want wallet address", nav.Data)
	}
}
```

#### Changes to `internal/tui/views/walletstatus/walletstatus.go`

**Change 1c — Add "s" and "c" keybindings (after the "3" sort case, before "up"):**

Find:
```go
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			tokens := m.visibleTokens()
			if m.cursor < len(tokens)-1 {
				m.cursor++
			}
```

Replace with:
```go
		case "s":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "send", Data: m.address}
			}
		case "c":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "receive", Data: m.address}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			tokens := m.visibleTokens()
			if m.cursor < len(tokens)-1 {
				m.cursor++
			}
```

**Change 1d — Update status bar to show new keybindings:**

Find:
```go
	b.WriteString(tui.StyleStatusBar.Render(
		fmt.Sprintf("[r]efresh [R]ename %s [1]value [2]symbol [3]balance [esc]back", showAllLabel)))
```

Replace with:
```go
	b.WriteString(tui.StyleStatusBar.Render(
		fmt.Sprintf("[s]end re[c]eive [r]efresh [R]ename %s [1]value [2]symbol [3]balance [esc]back", showAllLabel)))
```

**Verify:** `go test ./internal/tui/views/walletlist/... ./internal/tui/views/walletstatus/...`
**Commit:** `fix(tui): wire send/receive keybindings in wallet list and status views`

---

### Task 2: Fix float→uint64 truncation in send.go

**File:** `internal/tui/views/send/send.go`
**Test:** No test changes needed — this is a math fix in UI code
**Depends:** none

#### Changes

**Change 2a — Add `math.Round` to amount conversion in `updateAmount`:**

Find:
```go
		baseUnits := uint64(amountFloat * math.Pow10(decimals))
```

Replace with:
```go
		baseUnits := uint64(math.Round(amountFloat * math.Pow10(decimals)))
```

**Change 2b — Add `math.Round` to `maxSendable` balance calculation:**

Find:
```go
	baseBalance := uint64(m.selectedToken.Amount * math.Pow10(decimals))
```

Replace with:
```go
	baseBalance := uint64(math.Round(m.selectedToken.Amount * math.Pow10(decimals)))
```

**Verify:** `go test ./internal/tui/views/send/...`
**Commit:** `fix(send): use math.Round to prevent float→uint64 truncation`

---

### Task 3: Sort account groups deterministically in message.go

**File:** `internal/transaction/message.go`
**Test:** `internal/transaction/message_test.go`
**Depends:** none

#### Changes to `internal/transaction/message.go`

**Change 3a — Add imports for `bytes` and `sort`:**

Find:
```go
package transaction
```

Replace with:
```go
package transaction

import (
	"bytes"
	"sort"
)
```

**Change 3b — Sort each account group after building them (before "Build ordered account keys"):**

Find:
```go
	// Build ordered account keys: fee payer first
	accountKeys := make([]Pubkey, 0, len(accountMap))
```

Replace with:
```go
	// Sort each group by pubkey bytes for deterministic ordering
	sortPubkeys := func(keys []Pubkey) {
		sort.Slice(keys, func(i, j int) bool {
			return bytes.Compare(keys[i][:], keys[j][:]) < 0
		})
	}
	sortPubkeys(writableSigners)
	sortPubkeys(readonlySigners)
	sortPubkeys(writableNonSigners)
	sortPubkeys(readonlyNonSigners)

	// Build ordered account keys: fee payer first
	accountKeys := make([]Pubkey, 0, len(accountMap))
```

#### Changes to `internal/transaction/message_test.go`

**Change 3c — Add import for `bytes`:**

Find:
```go
package transaction

import "testing"
```

Replace with:
```go
package transaction

import (
	"bytes"
	"testing"
)
```

**Change 3d — Add deterministic serialization test at end of file:**

Find (the last test — `TestMessage_MultipleInstructions`):
```go
func TestMessage_MultipleInstructions(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient1, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	recipient2, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	var blockhash Pubkey

	ix1 := SystemTransfer(sender, recipient1, 100)
	ix2 := SystemTransfer(sender, recipient2, 200)

	msg := NewMessage(sender, []Instruction{ix1, ix2}, blockhash)

	// sender + recipient1 + recipient2 + system program = 4
	if len(msg.AccountKeys) != 4 {
		t.Errorf("AccountKeys count = %d, want 4", len(msg.AccountKeys))
	}

	// 2 compiled instructions
	if len(msg.Instructions) != 2 {
		t.Errorf("Instructions count = %d, want 2", len(msg.Instructions))
	}
}
```

Replace with:
```go
func TestMessage_MultipleInstructions(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient1, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	recipient2, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	var blockhash Pubkey

	ix1 := SystemTransfer(sender, recipient1, 100)
	ix2 := SystemTransfer(sender, recipient2, 200)

	msg := NewMessage(sender, []Instruction{ix1, ix2}, blockhash)

	// sender + recipient1 + recipient2 + system program = 4
	if len(msg.AccountKeys) != 4 {
		t.Errorf("AccountKeys count = %d, want 4", len(msg.AccountKeys))
	}

	// 2 compiled instructions
	if len(msg.Instructions) != 2 {
		t.Errorf("Instructions count = %d, want 2", len(msg.Instructions))
	}
}

func TestMessage_DeterministicSerialization(t *testing.T) {
	sender, _ := PubkeyFromBase58("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU")
	recipient1, _ := PubkeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	recipient2, _ := PubkeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	var blockhash Pubkey

	ix1 := SystemTransfer(sender, recipient1, 100)
	ix2 := SystemTransfer(sender, recipient2, 200)

	// Build the same message 100 times and verify identical serialization
	var reference []byte
	for i := 0; i < 100; i++ {
		msg := NewMessage(sender, []Instruction{ix1, ix2}, blockhash)
		serialized := msg.Serialize()
		if i == 0 {
			reference = serialized
		} else if !bytes.Equal(serialized, reference) {
			t.Fatalf("iteration %d: serialization differs from reference (non-deterministic ordering)", i)
		}
	}
}
```

**Verify:** `go test ./internal/transaction/...`
**Commit:** `fix(transaction): sort account groups for deterministic message serialization`

---

### Task 4: Zero plaintext seed in UnlockKeypair

**File:** `internal/services/keystore.go`
**Test:** No test changes needed — this is a security hardening fix
**Depends:** none

#### Changes

**Change 4a — Add `defer keystore.ZeroBytes(plaintext)` after decrypt:**

Find:
```go
	plaintext, err := keystore.Decrypt(encData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}

	// 5. Reconstruct ed25519 key from 32-byte seed
	privKey := ed25519.NewKeyFromSeed(plaintext)
```

Replace with:
```go
	plaintext, err := keystore.Decrypt(encData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}
	defer keystore.ZeroBytes(plaintext)

	// 5. Reconstruct ed25519 key from 32-byte seed
	privKey := ed25519.NewKeyFromSeed(plaintext)
```

**Verify:** `go test ./internal/services/...`
**Commit:** `fix(keystore): zero decrypted seed after reconstructing private key`

---

### Task 7: Set createATA flag for accurate fee estimate in send.go

**File:** `internal/tui/views/send/send.go`
**Test:** No test changes needed — this is a fee display fix
**Depends:** none

**Design decision:** Per the design doc, we use the simpler approach: `estimateFee()` returns `5000 + 2_039_280` for any SPL token transfer (worst-case estimate), and `5000` for SOL. This avoids an async RPC call and is safe — overestimating fees is better than underestimating.

#### Changes

**Change 7a — Update `estimateFee()` to check `m.isSOL` instead of relying on `m.createATA`:**

Find:
```go
// estimateFee returns the estimated fee in lamports.
func (m Model) estimateFee() uint64 {
	if m.transferSvc != nil {
		return m.transferSvc.EstimateFee(m.createATA)
	}
	// Fallback: base fee
	fee := uint64(5000)
	if m.createATA {
		fee += 2_039_280
	}
	return fee
}
```

Replace with:
```go
// estimateFee returns the estimated fee in lamports.
// For SPL tokens, always assumes ATA creation (worst-case estimate).
func (m Model) estimateFee() uint64 {
	needsATA := !m.isSOL
	if m.transferSvc != nil {
		return m.transferSvc.EstimateFee(needsATA)
	}
	// Fallback: base fee
	fee := uint64(5000)
	if needsATA {
		fee += 2_039_280
	}
	return fee
}
```

**Verify:** `go test ./internal/tui/views/send/...`
**Commit:** `fix(send): assume ATA creation for SPL fee estimates`

---

### Task 8: Append pages in history view

**File:** `internal/tui/views/history/history.go`
**Test:** No test changes needed — existing tests don't test pagination
**Depends:** none

#### Changes

**Change 8a — Add `noMorePages` field to Model struct:**

Find:
```go
// Model is the activity history view.
type Model struct {
	items         []services.ActivityItem
	cursor        int
	walletAddr    string
	activitySvc   *services.ActivityService
	rpcClient     *blockchain.RPCClient
	lastSignature string // pagination cursor
	loading       bool
	spinner       components.SpinnerModel
	width         int
	height        int
	err           error
}
```

Replace with:
```go
// Model is the activity history view.
type Model struct {
	items         []services.ActivityItem
	cursor        int
	walletAddr    string
	activitySvc   *services.ActivityService
	rpcClient     *blockchain.RPCClient
	lastSignature string // pagination cursor
	noMorePages   bool
	loading       bool
	spinner       components.SpinnerModel
	width         int
	height        int
	err           error
}
```

**Change 8b — Append items instead of overwriting, and handle empty pages:**

Find:
```go
	case ActivityLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.items = msg.Items
		m.cursor = 0
		if len(m.items) > 0 {
			m.lastSignature = m.items[len(m.items)-1].Signature
		}
		return m, nil
```

Replace with:
```go
	case ActivityLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		if len(msg.Items) == 0 {
			m.noMorePages = true
			return m, nil
		}
		isFirstLoad := len(m.items) == 0
		m.items = append(m.items, msg.Items...)
		if isFirstLoad {
			m.cursor = 0
		}
		m.lastSignature = m.items[len(m.items)-1].Signature
		if len(msg.Items) < pageSize {
			m.noMorePages = true
		}
		return m, nil
```

**Change 8c — Guard "next page" with `noMorePages` flag:**

Find:
```go
		case "n":
			// Load more (next page)
			if len(m.items) >= pageSize && m.lastSignature != "" {
				m.loading = true
				m.spinner = components.NewSpinner("Loading more...")
				return m, tea.Batch(m.spinner.Init(), m.loadActivity())
			}
```

Replace with:
```go
		case "n":
			// Load more (next page)
			if !m.noMorePages && m.lastSignature != "" {
				m.loading = true
				m.spinner = components.NewSpinner("Loading more...")
				return m, tea.Batch(m.spinner.Init(), m.loadActivity())
			}
```

**Change 8d — Update status bar to hide "[n]ext page" when no more pages:**

Find:
```go
	// Status bar
	b.WriteString("\n")
	b.WriteString(tui.StyleStatusBar.Render("[n]ext page [j/k]navigate [esc]back"))
```

Replace with:
```go
	// Status bar
	b.WriteString("\n")
	if m.noMorePages {
		b.WriteString(tui.StyleStatusBar.Render("[j/k]navigate [esc]back"))
	} else {
		b.WriteString(tui.StyleStatusBar.Render("[n]ext page [j/k]navigate [esc]back"))
	}
```

**Verify:** `go test ./internal/tui/views/history/...`
**Commit:** `fix(history): append pages instead of overwriting on pagination`

---

## Batch B: Back-navigation re-init (after Batch A)

### Task 5: Re-init views on back-navigation

**File:** `internal/tui/app.go`
**Test:** No test changes needed — existing `navigateBack` tests verify the stack behavior; the Init() call is additive
**Depends:** Tasks 1-4, 7, 8 (Batch A must complete so no merge conflicts)

#### Changes

**Change 5a — Call Init() on the popped view in `navigateBack()`:**

Find:
```go
func (a App) navigateBack() (tea.Model, tea.Cmd) {
	if len(a.viewStack) == 0 {
		return a, tea.Quit
	}

	// Pop from stack
	last := len(a.viewStack) - 1
	a.currentView = a.viewStack[last]
	a.viewStack = a.viewStack[:last]

	// Pass current size
	var cmd tea.Cmd
	a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.width, Height: a.height,
	})

	return a, cmd
}
```

Replace with:
```go
func (a App) navigateBack() (tea.Model, tea.Cmd) {
	if len(a.viewStack) == 0 {
		return a, tea.Quit
	}

	// Pop from stack
	last := len(a.viewStack) - 1
	a.currentView = a.viewStack[last]
	a.viewStack = a.viewStack[:last]

	// Pass current size
	var sizeCmd tea.Cmd
	a.currentView, sizeCmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.width, Height: a.height,
	})

	// Re-init the view so it refreshes its data
	initCmd := a.currentView.Init()

	return a, tea.Batch(sizeCmd, initCmd)
}
```

**Verify:** `go test ./internal/tui/...`
**Commit:** `fix(tui): re-init views on back-navigation to refresh stale data`

---

## Batch C: Balance checks before sending (after Batch A)

### Task 6: Add balance checks before sending

**File:** `internal/services/transfer.go`
**Test:** `internal/services/transfer_test.go`
**Depends:** Tasks 1-4, 7, 8 (Batch A must complete so no merge conflicts)

#### Changes to `internal/services/transfer.go`

**Change 6a — Add balance check in `SendSOL` after unlocking keypair (step 3), before getting blockhash (step 4):**

Find:
```go
	// 4. Get latest blockhash
	blockhashStr, _, err := blockchain.GetLatestBlockhash(rpcClient)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}

	// 5. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
```

(This is in the `SendSOL` function.)

Replace with:
```go
	// 4. Check balance
	balance, err := blockchain.GetBalance(rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SOL: get balance: %w", err)
	}
	requiredBalance := lamports + 5000 // amount + base fee
	if balance < requiredBalance {
		return "", fmt.Errorf("send SOL: %w", models.ErrInsufficientBalance)
	}

	// 5. Get latest blockhash
	blockhashStr, _, err := blockchain.GetLatestBlockhash(rpcClient)
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}

	// 6. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
```

**Change 6b — Add balance check in `SendSPL` after unlocking keypair (step 3), before parsing pubkeys (step 4):**

Find (in `SendSPL`):
```go
	// 4. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	mintPubkey, err := transaction.PubkeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("send SPL: invalid mint: %w", err)
	}

	// 5. Derive ATAs
```

Replace with:
```go
	// 4. Check SOL balance for fees
	solBalance, err := blockchain.GetBalance(rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SPL: get SOL balance: %w", err)
	}
	// Need at least base fee (5000) + potential ATA rent (2_039_280)
	minSOL := uint64(5000 + 2_039_280)
	if solBalance < minSOL {
		return "", fmt.Errorf("send SPL: insufficient SOL for fees: %w", models.ErrInsufficientBalance)
	}

	// 5. Check token balance
	tokenAccounts, err := blockchain.GetTokenAccountsByOwner(rpcClient, fromAddr)
	if err != nil {
		return "", fmt.Errorf("send SPL: get token accounts: %w", err)
	}
	var tokenBalance uint64
	for _, ta := range tokenAccounts {
		if ta.Mint == mint {
			tokenBalance = ta.Amount
			break
		}
	}
	if tokenBalance < amount {
		return "", fmt.Errorf("send SPL: %w", models.ErrInsufficientBalance)
	}

	// 6. Parse pubkeys
	fromPubkey, _ := transaction.PubkeyFromBase58(fromAddr)
	toPubkey, _ := transaction.PubkeyFromBase58(toAddr)
	mintPubkey, err := transaction.PubkeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("send SPL: invalid mint: %w", err)
	}

	// 7. Derive ATAs
```

**Note:** The remaining step comments in `SendSPL` (6→8, 7→9, 8→10) are renumbered by the insertion. The code is correct — only the comment numbers shift. The implementer should renumber the subsequent comments: `// 6. Check if recipient ATA exists` → `// 8.`, `// 7. Build instructions` → `// 9.`, `// 8. Get blockhash...` → `// 10.`.

#### Changes to `internal/services/transfer_test.go`

**Change 6c — Update the mock server in `TestTransfer_SendSOL_Success` to handle `getBalance`:**

Find:
```go
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result interface{}
		switch req.Method {
		case "getLatestBlockhash":
			result = map[string]interface{}{
				"value": map[string]interface{}{
					"blockhash":            "GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi",
					"lastValidBlockHeight": 12345,
				},
			}
		case "sendTransaction":
			result = expectedSig
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
```

Replace with:
```go
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result interface{}
		switch req.Method {
		case "getBalance":
			result = map[string]interface{}{
				"value": 10_000_000_000, // 10 SOL — plenty for the transfer
			}
		case "getLatestBlockhash":
			result = map[string]interface{}{
				"value": map[string]interface{}{
					"blockhash":            "GHtXQBsoZHVnNFa9YevAzFr17DJjgHXk3ycTKD5xD3Zi",
					"lastValidBlockHeight": 12345,
				},
			}
		case "sendTransaction":
			result = expectedSig
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
```

**Change 6d — Add a test for insufficient SOL balance at end of file:**

Find:
```go
func TestTransfer_EstimateFee(t *testing.T) {
	svc := services.NewTransferService(nil, nil)

	fee := svc.EstimateFee(false)
	if fee != 5000 {
		t.Errorf("EstimateFee(false) = %d, want 5000", fee)
	}

	feeWithATA := svc.EstimateFee(true)
	if feeWithATA != 5000+2_039_280 {
		t.Errorf("EstimateFee(true) = %d, want %d", feeWithATA, 5000+2_039_280)
	}
}
```

Replace with:
```go
func TestTransfer_SendSOL_InsufficientBalance(t *testing.T) {
	_, transferSvc, addr := setupTransferTest(t)

	recipient := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req blockchain.RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result interface{}
		switch req.Method {
		case "getBalance":
			result = map[string]interface{}{
				"value": 1000, // Only 1000 lamports — not enough
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := blockchain.NewRPCClient(server.URL, nil)
	_, err := transferSvc.SendSOL(client, addr, recipient, 1_000_000_000, "MyStr0ng!Pass#1")
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
	if !errors.Is(err, models.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got: %v", err)
	}
}

func TestTransfer_EstimateFee(t *testing.T) {
	svc := services.NewTransferService(nil, nil)

	fee := svc.EstimateFee(false)
	if fee != 5000 {
		t.Errorf("EstimateFee(false) = %d, want 5000", fee)
	}

	feeWithATA := svc.EstimateFee(true)
	if feeWithATA != 5000+2_039_280 {
		t.Errorf("EstimateFee(true) = %d, want %d", feeWithATA, 5000+2_039_280)
	}
}
```

**Verify:** `go test ./internal/services/...`
**Commit:** `fix(transfer): add balance checks before sending SOL and SPL tokens`

---

## Full Verification

After all tasks are complete, run:

```bash
go test ./...
```

All 24+ packages should pass.
