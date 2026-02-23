# Responsive TUI Layout Implementation Plan

**Goal:** Make all TUI views use their stored `width`/`height` to render responsive layouts instead of hardcoded ~80-column widths.

**Architecture:** App.go computes inner dimensions (subtracting border+padding chrome) and forwards adjusted `WindowSizeMsg` to child views. Each view then uses `m.width`/`m.height` in its `View()` method to size tables, inputs, and status bars proportionally. No new files — only modifications to existing files.

**Design:** `thoughts/shared/designs/2026-02-23-responsive-tui-design.md`

---

## Dependency Graph

```
Batch 0 (single):   0.1                          [app.go — all views depend on this]
Batch 1 (parallel): 1.1, 1.2, 1.3, 1.4, 1.5     [simple input-width views — independent]
Batch 2 (parallel): 2.1, 2.2, 2.3, 2.4           [table/list views — independent]
Batch 3 (single):   3.1                           [full build verification]
```

---

## Batch 0: App Inner Dimensions (single implementer)

This must complete before any view changes — views will receive adjusted dimensions.

### Task 0.1: Forward inner dimensions from App
**File:** `internal/tui/app.go`
**Test:** `internal/tui/app_test.go`
**Depends:** none

**What to change:** The `App` currently forwards the raw `tea.WindowSizeMsg` to child views. We need it to subtract the StyleApp chrome (Padding(1,2) + RoundedBorder = 6 cols, 4 rows) before forwarding.

**Test code — add to `internal/tui/app_test.go`:**

```go
func TestApp_ForwardsInnerDimensions(t *testing.T) {
	// Create a view that captures the WindowSizeMsg it receives
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	_ = model

	// App chrome: Padding(1,2) + RoundedBorder = 6 cols, 4 rows
	// innerWidth = max(20, 100 - 6) = 94
	// innerHeight = max(5, 40 - 4) = 36
	if capturedWidth != 94 {
		t.Errorf("inner width = %d, want 94", capturedWidth)
	}
	if capturedHeight != 36 {
		t.Errorf("inner height = %d, want 36", capturedHeight)
	}
}

func TestApp_ForwardsInnerDimensions_Small(t *testing.T) {
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 20, Height: 6})
	_ = model

	// innerWidth = max(20, 20 - 6) = max(20, 14) = 20
	// innerHeight = max(5, 6 - 4) = max(5, 2) = 5
	if capturedWidth != 20 {
		t.Errorf("inner width = %d, want 20", capturedWidth)
	}
	if capturedHeight != 5 {
		t.Errorf("inner height = %d, want 5", capturedHeight)
	}
}

func TestApp_NavigateForwardsInnerDimensions(t *testing.T) {
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	// Set size first
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Navigate — the new view should get inner dimensions
	capturedWidth = 0
	capturedHeight = 0
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-import"})
	_ = model

	// innerWidth = max(20, 80 - 6) = 74
	// innerHeight = max(5, 24 - 4) = 20
	if capturedWidth != 74 {
		t.Errorf("navigate inner width = %d, want 74", capturedWidth)
	}
	if capturedHeight != 20 {
		t.Errorf("navigate inner height = %d, want 20", capturedHeight)
	}
}

// capturingView is a test helper that captures WindowSizeMsg dimensions.
type capturingView struct {
	onSize func(w, h int)
}

func (v *capturingView) Init() tea.Cmd { return nil }
func (v *capturingView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok && v.onSize != nil {
		v.onSize(wsm.Width, wsm.Height)
	}
	return v, nil
}
func (v *capturingView) View() string { return "capturing" }
```

**Implementation — modify `internal/tui/app.go`:**

Three changes needed:

**Change 1:** In the `Update` method, modify the `tea.WindowSizeMsg` handler (lines 83-92):

Find this block:
```go
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		if a.currentView != nil {
			var cmd tea.Cmd
			a.currentView, cmd = a.currentView.Update(msg)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
```

Replace with:
```go
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		if a.currentView != nil {
			var cmd tea.Cmd
			a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
				Width:  a.innerWidth(),
				Height: a.innerHeight(),
			})
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
```

**Change 2:** In the `navigate` method (lines 163-164), change:
```go
	a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.width, Height: a.height,
	})
```
to:
```go
	a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.innerWidth(), Height: a.innerHeight(),
	})
```

**Change 3:** In the `navigateBack` method (lines 183-185), change:
```go
	a.currentView, sizeCmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.width, Height: a.height,
	})
```
to:
```go
	a.currentView, sizeCmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.innerWidth(), Height: a.innerHeight(),
	})
```

**Change 4:** Add two helper methods anywhere in app.go (e.g., after the `navigateBack` method):
```go
// innerWidth returns the usable content width after subtracting App chrome.
// StyleApp has Padding(1,2) = 4 cols + RoundedBorder = 2 cols = 6 total.
func (a App) innerWidth() int {
	w := a.width - 6
	if w < 20 {
		return 20
	}
	return w
}

// innerHeight returns the usable content height after subtracting App chrome.
// StyleApp has Padding(1,2) = 2 rows + RoundedBorder = 2 rows = 4 total.
func (a App) innerHeight() int {
	h := a.height - 4
	if h < 5 {
		return 5
	}
	return h
}
```

**Verify:** `go test ./internal/tui/ -run TestApp`
**Commit:** `fix(tui): forward inner dimensions to child views accounting for app chrome`

---

## Batch 1: Simple Input-Width Views (parallel — 5 implementers)

All tasks in this batch depend on Batch 0 completing. They are independent of each other.

### Task 1.1: send — Responsive text input widths
**File:** `internal/tui/views/send/send.go`
**Test:** `internal/tui/views/send/send_test.go`
**Depends:** 0.1

**What to change:** In the `WindowSizeMsg` handler, resize all text inputs to `min(original, m.width - 4)`. In `View()`, truncate the recipient address on the review step if narrow.

**Implementation — modify `internal/tui/views/send/send.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler in `Update` (currently lines ~111-114):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
```

**Change 2:** Add the `resizeInputs` method after the `SetSize` method:
```go
// resizeInputs adjusts text input widths to fit the available width.
func (m *Model) resizeInputs() {
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	m.recipientInput.Width = min(50, maxW)
	m.amountInput.Width = min(30, maxW)
	m.passwordInput.Width = min(40, maxW)
}
```

**Change 3:** In the `View()` method, in the `StepReview` case (around line ~340), change the "To:" line to truncate the address when narrow:

Find:
```go
		b.WriteString(fmt.Sprintf("  To:        %s\n", tui.StyleBold.Render(truncateAddr(m.recipient))))
```

Replace with:
```go
		toAddr := m.recipient
		if m.width > 0 && m.width < 60 {
			toAddr = truncateAddr(m.recipient)
		}
		b.WriteString(fmt.Sprintf("  To:        %s\n", tui.StyleBold.Render(toAddr)))
```

Wait — looking at the code again, `truncateAddr` is already used. The design says "truncate address display if narrow". Currently the review step already uses `truncateAddr`. The issue is that at wide widths we might want to show the full address. Let me adjust: we'll show the full address when wide, truncated when narrow.

Actually, re-reading the code, the review step already truncates. The real value is in the `StepAmount` line which also uses `truncateAddr`. The design intent is clear: make inputs responsive. The address truncation is already handled. Let me keep the change simple — just resize inputs.

**Test code — add to `internal/tui/views/send/send_test.go`:**

```go
func TestSend_ResponsiveInputWidths(t *testing.T) {
	portfolio := models.PortfolioBalance{
		SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0, Decimals: 9},
	}
	m := send.New("addr123", nil, nil, portfolio)

	// Simulate narrow terminal
	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	_ = model

	// Verify the view renders without panic at narrow width
	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}

func TestSend_ResponsiveInputWidths_Wide(t *testing.T) {
	portfolio := models.PortfolioBalance{
		SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0, Decimals: 9},
	}
	m := send.New("addr123", nil, nil, portfolio)

	// Simulate wide terminal
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = model

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at wide width")
	}
}
```

**Verify:** `go test ./internal/tui/views/send/`
**Commit:** `fix(tui/send): make text input widths responsive to terminal size`

---

### Task 1.2: swapview — Responsive text input widths
**File:** `internal/tui/views/swapview/swapview.go`
**Test:** `internal/tui/views/swapview/swapview_test.go`
**Depends:** 0.1

**What to change:** In the `WindowSizeMsg` handler, resize all text inputs to `min(original, m.width - 4)`.

**Implementation — modify `internal/tui/views/swapview/swapview.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler (lines 148-151):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
```

**Change 2:** Add the `resizeInputs` method after the `SetSize` method:
```go
// resizeInputs adjusts text input widths to fit the available width.
func (m *Model) resizeInputs() {
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	m.inputMint.Width = min(50, maxW)
	m.outputMint.Width = min(50, maxW)
	m.amountInput.Width = min(20, maxW)
	m.passwordInput.Width = min(40, maxW)
}
```

**Test code — add to `internal/tui/views/swapview/swapview_test.go`:**

```go
func TestSwap_ResponsiveInputWidths(t *testing.T) {
	m := swapview.New("addr123", nil, nil, false)

	// Simulate narrow terminal
	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
```

**Verify:** `go test ./internal/tui/views/swapview/`
**Commit:** `fix(tui/swap): make text input widths responsive to terminal size`

---

### Task 1.3: tokenadd — Responsive text input widths
**File:** `internal/tui/views/tokenadd/tokenadd.go`
**Test:** `internal/tui/views/tokenadd/tokenadd_test.go`
**Depends:** 0.1

**What to change:** In the `WindowSizeMsg` handler, resize all text inputs to `min(original, m.width - 4)`.

**Implementation — modify `internal/tui/views/tokenadd/tokenadd.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler (lines 109-112):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
```

**Change 2:** Add the `resizeInputs` method after the `SetSize` method:
```go
// resizeInputs adjusts text input widths to fit the available width.
func (m *Model) resizeInputs() {
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	m.symbolInput.Width = min(30, maxW)
	m.nameInput.Width = min(40, maxW)
	m.addressInput.Width = min(50, maxW)
}
```

**Test code — add to `internal/tui/views/tokenadd/tokenadd_test.go`:**

```go
func TestTokenAdd_ResponsiveInputWidths(t *testing.T) {
	m := tokenadd.New(nil)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
```

**Verify:** `go test ./internal/tui/views/tokenadd/`
**Commit:** `fix(tui/tokenadd): make text input widths responsive to terminal size`

---

### Task 1.4: walletdelete — Responsive text input width
**File:** `internal/tui/views/walletdelete/walletdelete.go`
**Test:** `internal/tui/views/walletdelete/walletdelete_test.go`
**Depends:** 0.1

**What to change:** In the `WindowSizeMsg` handler, resize the confirm input to `min(50, m.width - 4)`.

**Implementation — modify `internal/tui/views/walletdelete/walletdelete.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler (lines 64-67):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		maxW := m.width - 4
		if maxW < 10 {
			maxW = 10
		}
		m.confirmInput.Width = min(50, maxW)
		return m, nil
```

**Test code — add to `internal/tui/views/walletdelete/walletdelete_test.go`:**

```go
func TestWalletDelete_ResponsiveInputWidth(t *testing.T) {
	w := models.Wallet{Address: "abc123", Label: "test"}
	m := walletdelete.New(w, nil, 2)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
```

**Verify:** `go test ./internal/tui/views/walletdelete/`
**Commit:** `fix(tui/walletdelete): make confirm input width responsive to terminal size`

---

### Task 1.5: walletimport — Responsive textarea and inputs, adaptive mnemonic grid
**File:** `internal/tui/views/walletimport/walletimport.go`
**Test:** `internal/tui/views/walletimport/walletimport_test.go`
**Depends:** 0.1

**What to change:**
1. In the `WindowSizeMsg` handler, resize textarea and all text inputs.
2. In `View()`, render the mnemonic grid with 2 columns if `m.width < 50`, 3 columns otherwise.

**Implementation — modify `internal/tui/views/walletimport/walletimport.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler (lines ~125-128):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
```

**Change 2:** Add the `resizeInputs` method after the `SetSize` method:
```go
// resizeInputs adjusts input widths to fit the available width.
func (m *Model) resizeInputs() {
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	m.seedInput.SetWidth(min(60, maxW))
	m.accountInput.Width = min(10, maxW)
	m.passwordInput.Width = min(40, maxW)
	m.confirmPwInput.Width = min(40, maxW)
	m.labelInput.Width = min(30, maxW)
}
```

**Change 3:** In the `View()` method, in the `StepShowMnemonic` case, replace the hardcoded 3-column grid with an adaptive grid. Find this block:

```go
		// Display words in a 3-column grid
		for i, word := range m.words {
			col := i % 3
			num := fmt.Sprintf("%2d. %-12s", i+1, word)
			b.WriteString(tui.StyleBold.Render(num))
			if col == 2 || i == len(m.words)-1 {
				b.WriteString("\n")
			} else {
				b.WriteString("  ")
			}
		}
```

Replace with:
```go
		// Display words in an adaptive grid (2 cols if narrow, 3 cols otherwise)
		cols := 3
		if m.width > 0 && m.width < 50 {
			cols = 2
		}
		for i, word := range m.words {
			col := i % cols
			num := fmt.Sprintf("%2d. %-12s", i+1, word)
			b.WriteString(tui.StyleBold.Render(num))
			if col == cols-1 || i == len(m.words)-1 {
				b.WriteString("\n")
			} else {
				b.WriteString("  ")
			}
		}
```

**Test code — add to `internal/tui/views/walletimport/walletimport_test.go`:**

```go
func TestWalletImport_ResponsiveInputWidths(t *testing.T) {
	m := walletimport.New(nil, nil)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
```

**Verify:** `go test ./internal/tui/views/walletimport/`
**Commit:** `fix(tui/walletimport): make inputs and mnemonic grid responsive to terminal size`

---

## Batch 2: Table/List Views (parallel — 4 implementers)

All tasks in this batch depend on Batch 0 completing. They are independent of each other.

### Task 2.1: walletlist — Proportional table columns, capped rows, abbreviated status bar
**File:** `internal/tui/views/walletlist/walletlist.go`
**Test:** `internal/tui/views/walletlist/walletlist_test.go`
**Depends:** 0.1

**What to change:**
1. In `View()`, compute table column widths proportionally from `m.width`.
2. Cap visible rows to `m.height - 8`.
3. Show scroll indicator when list exceeds visible rows.
4. Abbreviate status bar when `m.width < 80`.

**Implementation — modify `internal/tui/views/walletlist/walletlist.go`:**

Replace the entire `View()` method with:

```go
// View renders the wallet list.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Hound - Wallet Manager")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.wallets) == 0 {
		b.WriteString(tui.StyleMuted.Render("No wallets found. Press [i] to import one.") + "\n")
	} else {
		// Compute proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colLabel := max(6, w*15/100)
		colAddr := max(8, w*18/100)
		colType := max(8, w*20/100)
		colBal := max(8, w*12/100)

		// Table header
		headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%%ds", colLabel, colAddr, colType, colBal)
		header := fmt.Sprintf(headerFmt, "Label", "Address", "Type", "Balance")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Cap visible rows
		maxRows := len(m.wallets)
		if m.height > 0 {
			visible := m.height - 8
			if visible < 1 {
				visible = 1
			}
			if visible < maxRows {
				maxRows = visible
			}
		}

		// Determine visible window around cursor
		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(m.wallets) {
			endIdx = len(m.wallets)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		// Table rows
		rowFmt := fmt.Sprintf("%%s%%-%ds %%-%ds %%-%ds %%%ds", colLabel, colAddr, colType, colBal)
		for i := startIdx; i < endIdx; i++ {
			w := m.wallets[i]
			primary := "  "
			if w.IsPrimary {
				primary = tui.StylePrimaryBadge.Render("* ")
			}

			addr := TruncateAddress(w.Address)
			typeBadge := tui.StyleTypeBadge.Render(w.WalletType.String())

			balance := "$0.00"
			if p, ok := m.portfolios[w.Address]; ok {
				balance = wallet.FormatPrice(p.TotalUSD)
			}

			row := fmt.Sprintf(rowFmt,
				primary, truncate(w.Label, colLabel), addr, typeBadge, balance)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render("> "+row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render("  "+row) + "\n")
			}
		}

		// Scroll indicator
		if len(m.wallets) > maxRows {
			hidden := len(m.wallets) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}

		// Footer: total USD
		var totalUSD float64
		for _, p := range m.portfolios {
			totalUSD += p.TotalUSD
		}
		b.WriteString("\n")
		b.WriteString(tui.StyleBold.Render(fmt.Sprintf("  Total: %s", wallet.FormatPrice(totalUSD))) + "\n")
	}

	// Status bar — abbreviated if narrow
	b.WriteString("\n")
	if m.width > 0 && m.width < 80 {
		b.WriteString(tui.StyleStatusBar.Render("[i]mp [s]tat [d]el [t]ok [S]end [R]ecv [w]swap [h]ist [r]ef [q]uit"))
	} else {
		b.WriteString(tui.StyleStatusBar.Render("[i]mport [s]tatus [d]elete [t]okens [S]end [R]eceive [w]swap [h]istory [r]efresh [q]uit"))
	}

	return b.String()
}
```

**Test code — add to `internal/tui/views/walletlist/walletlist_test.go`:**

```go
func TestWalletList_ResponsiveView_Narrow(t *testing.T) {
	m := walletlist.New(nil, nil)
	// Simulate loading complete with wallets
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets: []models.Wallet{
			{Address: "abc123def456ghi789", Label: "Main", IsPrimary: true},
			{Address: "xyz987wvu654tsr321", Label: "Secondary"},
		},
		Portfolios: map[string]models.PortfolioBalance{},
	})

	// Set narrow width
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 15})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	// Abbreviated status bar should be used
	if !strings.Contains(view, "[i]mp") {
		t.Error("narrow view should use abbreviated status bar")
	}
}

func TestWalletList_ResponsiveView_Wide(t *testing.T) {
	m := walletlist.New(nil, nil)
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets: []models.Wallet{
			{Address: "abc123def456ghi789", Label: "Main", IsPrimary: true},
		},
		Portfolios: map[string]models.PortfolioBalance{},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.(tea.Model).View()
	if !strings.Contains(view, "[i]mport") {
		t.Error("wide view should use full status bar")
	}
}

func TestWalletList_CappedVisibleRows(t *testing.T) {
	wallets := make([]models.Wallet, 20)
	for i := range wallets {
		wallets[i] = models.Wallet{
			Address: fmt.Sprintf("addr%d", i),
			Label:   fmt.Sprintf("Wallet %d", i),
		}
	}

	m := walletlist.New(nil, nil)
	model, _ := m.Update(walletlist.WalletsLoadedMsg{
		Wallets:    wallets,
		Portfolios: map[string]models.PortfolioBalance{},
	})

	// Very short terminal: height=12, maxRows = 12 - 8 = 4
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

	view := model.(tea.Model).View()
	// Should show scroll indicator
	if !strings.Contains(view, "more") {
		t.Error("should show scroll indicator when list exceeds visible rows")
	}
}
```

**Verify:** `go test ./internal/tui/views/walletlist/`
**Commit:** `fix(tui/walletlist): proportional columns, capped rows, abbreviated status bar`

---

### Task 2.2: walletstatus — Proportional table columns, capped rows, abbreviated status bar, responsive rename input
**File:** `internal/tui/views/walletstatus/walletstatus.go`
**Test:** `internal/tui/views/walletstatus/walletstatus_test.go`
**Depends:** 0.1

**What to change:**
1. In `WindowSizeMsg` handler, resize rename input to `min(30, m.width - 10)`.
2. In `View()`, compute table column widths proportionally from `m.width`.
3. Cap visible token rows to `m.height - 10`.
4. Abbreviate status bar when `m.width < 80`.

**Implementation — modify `internal/tui/views/walletstatus/walletstatus.go`:**

**Change 1:** Replace the `tea.WindowSizeMsg` handler (lines ~138-141):

Find:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
```

Replace with:
```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.renaming {
			maxW := m.width - 10
			if maxW < 10 {
				maxW = 10
			}
			m.renameInput.Width = min(30, maxW)
		}
		return m, nil
```

**Change 2:** In the `Update` method, in the `"R"` key handler where the rename input is created (around line ~155), add responsive width:

Find:
```go
		case "R":
			m.renaming = true
			m.renameErr = nil
			ri := textinput.New()
			ri.Placeholder = "New wallet name"
			ri.CharLimit = 32
			ri.Width = 30
			ri.Focus()
			m.renameInput = ri
			return m, ri.Focus()
```

Replace with:
```go
		case "R":
			m.renaming = true
			m.renameErr = nil
			ri := textinput.New()
			ri.Placeholder = "New wallet name"
			ri.CharLimit = 32
			maxW := m.width - 10
			if maxW < 10 {
				maxW = 10
			}
			ri.Width = min(30, maxW)
			ri.Focus()
			m.renameInput = ri
			return m, ri.Focus()
```

**Change 3:** Replace the `View()` method with a responsive version:

```go
// View renders the wallet status.
func (m Model) View() string {
	var b strings.Builder

	// Header
	title := tui.StyleTitle.Render("Wallet Status")
	b.WriteString(title + "\n")

	addrDisplay := m.address
	if len(addrDisplay) > 11 {
		addrDisplay = addrDisplay[:4] + "..." + addrDisplay[len(addrDisplay)-4:]
	}
	b.WriteString(tui.StyleSubtitle.Render(addrDisplay) + "\n\n")

	// Rename overlay
	if m.renaming {
		b.WriteString("Rename wallet:\n\n")
		b.WriteString(m.renameInput.View() + "\n\n")
		if m.renameErr != nil {
			b.WriteString(tui.StyleError.Render("Error: "+m.renameErr.Error()) + "\n\n")
		}
		b.WriteString(tui.StyleMuted.Render("Enter to save, Esc to cancel"))
		return b.String()
	}

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// Total USD
	totalStr := wallet.FormatPrice(m.portfolio.TotalUSD)
	b.WriteString(tui.StyleBold.Render("Total: "+totalStr) + "\n\n")

	// SOL balance
	sol := m.portfolio.SOLBalance
	solLine := fmt.Sprintf("SOL  %s  %s  %s",
		wallet.FormatBalance(sol.Amount),
		wallet.FormatPrice(sol.USDValue),
		tui.FormatChange(sol.Change24h))
	b.WriteString(solLine + "\n\n")

	// Token table with proportional columns
	tokens := m.visibleTokens()
	w := m.width
	if w <= 0 {
		w = 80
	}
	colSym := max(6, w*13/100)
	colBal := max(8, w*15/100)
	colPrice := max(8, w*13/100)
	colVal := max(8, w*15/100)
	colChg := max(6, w*10/100)

	if len(tokens) > 0 {
		headerFmt := fmt.Sprintf("%%-%ds %%%ds %%%ds %%%ds %%%ds", colSym, colBal, colPrice, colVal, colChg)
		header := fmt.Sprintf(headerFmt, "Symbol", "Balance", "Price", "Value", "24h")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Cap visible rows
		maxRows := len(tokens)
		if m.height > 0 {
			visible := m.height - 10
			if visible < 1 {
				visible = 1
			}
			if visible < maxRows {
				maxRows = visible
			}
		}

		// Determine visible window around cursor
		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(tokens) {
			endIdx = len(tokens)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		rowFmt := fmt.Sprintf("%%-%ds %%%ds %%%ds %%%ds %%%ds", colSym, colBal, colPrice, colVal, colChg)
		for i := startIdx; i < endIdx; i++ {
			t := tokens[i]
			row := fmt.Sprintf(rowFmt,
				truncate(t.Symbol, colSym),
				wallet.FormatBalance(t.Amount),
				wallet.FormatPrice(t.USDPrice),
				wallet.FormatPrice(t.USDValue),
				tui.FormatChange(t.Change24h))

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}

		// Scroll indicator
		if len(tokens) > maxRows {
			hidden := len(tokens) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	} else {
		b.WriteString(tui.StyleMuted.Render("No tokens found") + "\n")
	}

	// Sort indicator + last refresh
	b.WriteString("\n")
	sortLine := fmt.Sprintf("Sort: %s", m.sortMode.String())
	if !m.lastRefresh.IsZero() {
		sortLine += fmt.Sprintf("  |  Last refresh: %s", m.lastRefresh.Format("15:04:05"))
	}
	b.WriteString(tui.StyleMuted.Render(sortLine) + "\n")

	// Status bar — abbreviated if narrow
	showAllLabel := "[a]ll"
	if m.showAll {
		showAllLabel = "[a]ll*"
	}
	if m.width > 0 && m.width < 80 {
		b.WriteString(tui.StyleStatusBar.Render(
			fmt.Sprintf("[s]end [c]rcv [r]ef [R]en %s [1][2][3] [esc]", showAllLabel)))
	} else {
		b.WriteString(tui.StyleStatusBar.Render(
			fmt.Sprintf("[s]end re[c]eive [r]efresh [R]ename %s [1]value [2]symbol [3]balance [esc]back", showAllLabel)))
	}

	return b.String()
}
```

**Test code — add to `internal/tui/views/walletstatus/walletstatus_test.go`:**

```go
func TestWalletStatus_ResponsiveView_Narrow(t *testing.T) {
	m := walletstatus.New(nil, "addr123", nil)

	// Simulate portfolio loaded
	model, _ := m.Update(tui.PortfolioRefreshedMsg{
		Portfolio: models.PortfolioBalance{
			TotalUSD:   100.0,
			SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0},
		},
	})

	// Set narrow width
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 15})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	// Should use abbreviated status bar
	if !strings.Contains(view, "[s]end [c]rcv") {
		t.Error("narrow view should use abbreviated status bar")
	}
}

func TestWalletStatus_ResponsiveView_Wide(t *testing.T) {
	m := walletstatus.New(nil, "addr123", nil)

	model, _ := m.Update(tui.PortfolioRefreshedMsg{
		Portfolio: models.PortfolioBalance{
			TotalUSD:   100.0,
			SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0},
		},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.(tea.Model).View()
	if !strings.Contains(view, "[s]end re[c]eive") {
		t.Error("wide view should use full status bar")
	}
}
```

**Verify:** `go test ./internal/tui/views/walletstatus/`
**Commit:** `fix(tui/walletstatus): proportional columns, capped rows, abbreviated status bar`

---

### Task 2.3: history — Proportional table columns, capped visible rows
**File:** `internal/tui/views/history/history.go`
**Test:** `internal/tui/views/history/history_test.go`
**Depends:** 0.1

**What to change:**
1. In `View()`, compute the activity row format proportionally from `m.width`.
2. Cap visible rows to `m.height - 6`.
3. Show scroll indicator when list exceeds visible rows.

**Implementation — modify `internal/tui/views/history/history.go`:**

Replace the `View()` method with:

```go
// View renders the activity history.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("History")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(tui.StyleMuted.Render("No transaction history found.") + "\n")
	} else {
		// Proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colDesc := max(15, w*38/100)
		colTime := max(8, w*19/100)
		colStatus := max(8, w*15/100)

		// Cap visible rows
		maxRows := len(m.items)
		if m.height > 0 {
			visible := m.height - 6
			if visible < 1 {
				visible = 1
			}
			if visible < maxRows {
				maxRows = visible
			}
		}

		// Determine visible window around cursor
		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(m.items) {
			endIdx = len(m.items)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		for i := startIdx; i < endIdx; i++ {
			item := m.items[i]
			icon := directionStyledIcon(item.Direction, item.Type)
			line := formatActivityLine(item)
			timeStr := FormatRelativeTime(item.Timestamp)

			var counterparty string
			if item.Counterparty != "" {
				if item.Direction == "sent" {
					counterparty = fmt.Sprintf(" → %s", item.Counterparty)
				} else {
					counterparty = fmt.Sprintf(" ← %s", item.Counterparty)
				}
			}

			statusStr := item.Status
			switch item.Status {
			case "confirmed":
				statusStr = tui.StyleSuccess.Render(item.Status)
			case "failed":
				statusStr = tui.StyleError.Render(item.Status)
			}

			rowFmt := fmt.Sprintf("%%s %%-%ds %%-%ds %%-%ds", colDesc, colTime, colStatus)
			row := fmt.Sprintf(rowFmt,
				icon,
				truncate(line+counterparty, colDesc),
				tui.StyleMuted.Render(timeStr),
				statusStr,
			)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}

		// Scroll indicator
		if len(m.items) > maxRows {
			hidden := len(m.items) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	}

	// Status bar
	b.WriteString("\n")
	if m.noMorePages {
		b.WriteString(tui.StyleStatusBar.Render("[j/k]navigate [esc]back"))
	} else {
		b.WriteString(tui.StyleStatusBar.Render("[n]ext page [j/k]navigate [esc]back"))
	}

	return b.String()
}
```

**Test code — add to `internal/tui/views/history/history_test.go`:**

```go
func TestHistory_ResponsiveView_Narrow(t *testing.T) {
	m := history.New("addr123", nil, nil)

	// Simulate loaded items
	items := make([]services.ActivityItem, 30)
	for i := range items {
		items[i] = services.ActivityItem{
			Type:      "sol_transfer",
			Direction: "sent",
			Amount:    "1 SOL",
			Status:    "confirmed",
			Signature: fmt.Sprintf("sig%d", i),
		}
	}
	model, _ := m.Update(history.ActivityLoadedMsg{Items: items})

	// Set narrow + short terminal
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 15})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	// Should show scroll indicator (15 - 6 = 9 visible, 30 items)
	if !strings.Contains(view, "more") {
		t.Error("should show scroll indicator when items exceed visible rows")
	}
}
```

**Verify:** `go test ./internal/tui/views/history/`
**Commit:** `fix(tui/history): proportional columns and capped visible rows`

---

### Task 2.4: tokenlist — Proportional table columns, capped visible rows
**File:** `internal/tui/views/tokenlist/tokenlist.go`
**Test:** `internal/tui/views/tokenlist/tokenlist_test.go`
**Depends:** 0.1

**What to change:**
1. In `View()`, compute table column widths proportionally from `m.width`.
2. Cap visible rows to `m.height - 6`.
3. Show scroll indicator when list exceeds visible rows.

**Implementation — modify `internal/tui/views/tokenlist/tokenlist.go`:**

Replace the `View()` method with:

```go
// View renders the token list.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Token List")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.tokens) == 0 {
		b.WriteString(tui.StyleMuted.Render("No tokens found. Press [a] to add one.") + "\n")
	} else {
		// Proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colSym := max(6, w*13/100)
		colName := max(10, w*25/100)
		colPools := max(5, w*8/100)
		colLiq := max(10, w*18/100)

		headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%%ds %%%ds %%s", colSym, colName, colPools, colLiq)
		header := fmt.Sprintf(headerFmt, "Symbol", "Name", "Pools", "Liquidity", "")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Cap visible rows
		maxRows := len(m.tokens)
		if m.height > 0 {
			visible := m.height - 6
			if visible < 1 {
				visible = 1
			}
			if visible < maxRows {
				maxRows = visible
			}
		}

		// Determine visible window around cursor
		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(m.tokens) {
			endIdx = len(m.tokens)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		rowFmt := fmt.Sprintf("%%-%ds %%-%ds %%%dd %%%ds %%s", colSym, colName, colPools, colLiq)
		for i := startIdx; i < endIdx; i++ {
			tr := m.tokens[i]
			autoDiscovered := ""
			for _, p := range tr.Token.Pools {
				if p.DiscoveredAt > 0 {
					autoDiscovered = "auto"
					break
				}
			}

			row := fmt.Sprintf(rowFmt,
				truncate(tr.Token.Symbol, colSym),
				truncate(tr.Token.Name, colName),
				tr.PoolStats.PoolCount,
				wallet.FormatLargeNumber(tr.PoolStats.TotalLiquidity),
				autoDiscovered,
			)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}

		// Scroll indicator
		if len(m.tokens) > maxRows {
			hidden := len(m.tokens) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(tui.StyleStatusBar.Render("[enter]details [a]dd [esc]back"))

	return b.String()
}
```

**Test code — add to `internal/tui/views/tokenlist/tokenlist_test.go`:**

```go
func TestTokenList_ResponsiveView_Narrow(t *testing.T) {
	m := tokenlist.New(nil)

	// Simulate loaded tokens
	tokens := make([]tokenlist.TokenRow, 20)
	for i := range tokens {
		tokens[i] = tokenlist.TokenRow{
			Token: models.Token{Symbol: fmt.Sprintf("TK%d", i), Name: fmt.Sprintf("Token %d", i)},
		}
	}
	model, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: tokens})

	// Set narrow + short terminal
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 12})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	// Should show scroll indicator (12 - 6 = 6 visible, 20 items)
	if !strings.Contains(view, "more") {
		t.Error("should show scroll indicator when tokens exceed visible rows")
	}
}
```

**Verify:** `go test ./internal/tui/views/tokenlist/`
**Commit:** `fix(tui/tokenlist): proportional columns and capped visible rows`

---

## Batch 3: Full Verification (single implementer)

### Task 3.1: Build and test verification
**File:** none (verification only)
**Test:** none
**Depends:** 0.1, 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4

Run the full build and test suite to verify nothing is broken:

```bash
go build ./...
go test ./...
```

**Expected:** All 24 test packages pass, zero build errors.

If any test fails, the failure will be in the specific view package that was modified. Fix in that package — the changes are isolated per-view.

**Commit:** none (verification only)

---

## Summary of Changes

| File | Change Type | Key Changes |
|------|------------|-------------|
| `internal/tui/app.go` | Modify | Add `innerWidth()`/`innerHeight()` helpers; forward adjusted `WindowSizeMsg` in 3 places |
| `internal/tui/views/send/send.go` | Modify | Add `resizeInputs()` called from `WindowSizeMsg` handler |
| `internal/tui/views/swapview/swapview.go` | Modify | Add `resizeInputs()` called from `WindowSizeMsg` handler |
| `internal/tui/views/tokenadd/tokenadd.go` | Modify | Add `resizeInputs()` called from `WindowSizeMsg` handler |
| `internal/tui/views/walletdelete/walletdelete.go` | Modify | Inline resize in `WindowSizeMsg` handler |
| `internal/tui/views/walletimport/walletimport.go` | Modify | Add `resizeInputs()` + adaptive mnemonic grid (2/3 cols) |
| `internal/tui/views/walletlist/walletlist.go` | Modify | Rewrite `View()` with proportional columns, capped rows, scroll indicator, abbreviated status bar |
| `internal/tui/views/walletstatus/walletstatus.go` | Modify | Rewrite `View()` with proportional columns, capped rows, scroll indicator, abbreviated status bar, responsive rename input |
| `internal/tui/views/history/history.go` | Modify | Rewrite `View()` with proportional columns, capped rows, scroll indicator |
| `internal/tui/views/tokenlist/tokenlist.go` | Modify | Rewrite `View()` with proportional columns, capped rows, scroll indicator |

**No changes needed:** `receive`, `tokenfetch` (content is minimal and already fits any width).

**Total: 10 files modified, 0 files created.**
