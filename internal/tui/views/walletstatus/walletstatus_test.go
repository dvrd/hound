package walletstatus_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/walletstatus"
)

func newTestModel() walletstatus.Model {
	return walletstatus.New(nil, "7xKXabc1234567890abcdef9mPq", nil)
}

func loadedModel() walletstatus.Model {
	m := newTestModel()
	portfolio := models.PortfolioBalance{
		WalletAddress: "7xKXabc1234567890abcdef9mPq",
		SOLBalance: models.TokenBalance{
			Mint:      "So11111111111111111111111111111111111111112",
			Symbol:    "SOL",
			Amount:    10.5,
			USDPrice:  150.0,
			USDValue:  1575.0,
			Change24h: 2.5,
		},
		TokenBalances: []models.TokenBalance{
			{
				Mint:      "USDC_MINT",
				Symbol:    "USDC",
				Amount:    500.0,
				USDPrice:  1.0,
				USDValue:  500.0,
				Change24h: 0.01,
			},
			{
				Mint:      "BONK_MINT",
				Symbol:    "BONK",
				Amount:    1000000.0,
				USDPrice:  0.000028,
				USDValue:  28.0,
				Change24h: -5.2,
			},
		},
		TotalUSD: 2103.0,
	}
	updated, _ := m.Update(tui.PortfolioRefreshedMsg{Portfolio: portfolio})
	return updated.(walletstatus.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Wallet Status") {
		t.Errorf("View should contain 'Wallet Status', got %q", view)
	}
}

func TestViewContainsTruncatedAddress(t *testing.T) {
	m := loadedModel()
	view := m.View()
	// Address should be truncated: 7xKX...9mPq
	if !strings.Contains(view, "7xKX...9mPq") {
		t.Error("View should contain truncated address")
	}
}

func TestViewContainsTotalUSD(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Total") {
		t.Error("View should contain 'Total'")
	}
}

func TestViewContainsSOLBalance(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "SOL") {
		t.Error("View should contain 'SOL'")
	}
}

func TestViewContainsTokens(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "USDC") {
		t.Error("View should contain 'USDC'")
	}
	if !strings.Contains(view, "BONK") {
		t.Error("View should contain 'BONK'")
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	m := loadedModel()
	footer := m.Footer()
	if !strings.Contains(footer, "refresh") {
		t.Error("Footer should contain refresh in status bar")
	}
	if !strings.Contains(footer, "quit") {
		t.Error("Footer should contain quit in status bar")
	}
}

func TestEscNavigatesBack(t *testing.T) {
	m := loadedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc should return NavigateBackMsg, got %T", msg)
	}
}

func TestSortModes(t *testing.T) {
	tests := []struct {
		key      string
		wantSort walletstatus.SortMode
	}{
		{"1", walletstatus.SortByValue},
		{"2", walletstatus.SortBySymbol},
		{"3", walletstatus.SortByBalance},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := loadedModel()
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			model := updated.(walletstatus.Model)
			if model.GetSortMode() != tt.wantSort {
				t.Errorf("after key %q, sort mode = %d, want %d", tt.key, model.GetSortMode(), tt.wantSort)
			}
		})
	}
}

func TestSortModeString(t *testing.T) {
	tests := []struct {
		mode walletstatus.SortMode
		want string
	}{
		{walletstatus.SortByValue, "Value"},
		{walletstatus.SortBySymbol, "Symbol"},
		{walletstatus.SortByBalance, "Balance"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.want {
				t.Errorf("SortMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestCycleFilterMode(t *testing.T) {
	m := loadedModel()
	if m.GetFilterMode() != walletstatus.FilterDefault {
		t.Error("filterMode should be FilterDefault initially")
	}

	// First press: FilterDefault → FilterAll
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(walletstatus.Model)
	if model.GetFilterMode() != walletstatus.FilterAll {
		t.Errorf("filterMode should be FilterAll after first 'a', got %v", model.GetFilterMode())
	}

	// Second press: FilterAll → FilterDefault
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(walletstatus.Model)
	if model.GetFilterMode() != walletstatus.FilterDefault {
		t.Errorf("filterMode should be FilterDefault after second 'a', got %v", model.GetFilterMode())
	}
}

func TestPortfolioRefreshedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tui.PortfolioRefreshedMsg{
		Err: models.ErrWalletNotFound,
	})
	model := updated.(walletstatus.Model)
	view := model.View()
	if !strings.Contains(view, "unavailable") && !strings.Contains(view, "last error") {
		t.Error("View should show error state when PortfolioRefreshedMsg has error")
	}
}

func TestViewSortIndicator(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Sort: Value") {
		t.Error("View should show sort indicator 'Sort: Value'")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model := updated.(walletstatus.Model)
	view = model.View()
	if !strings.Contains(view, "Sort: Symbol") {
		t.Error("View should show sort indicator 'Sort: Symbol'")
	}
}

func TestCursorNavigation(t *testing.T) {
	m := loadedModel()

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = updated.(walletstatus.Model)
	// Should not panic

	// Move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	_ = updated.(walletstatus.Model)
	// Should not panic
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletstatus.Model)
	// Should not panic
}

func TestLoadingView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	// Loading state should show spinner or loading message
	if !strings.Contains(view, "Loading") && !strings.Contains(view, "Wallet Status") {
		t.Error("loading view should contain loading indicator or title")
	}
}

func TestEmptyTokens(t *testing.T) {
	m := newTestModel()
	portfolio := models.PortfolioBalance{
		WalletAddress: "7xKXabc1234567890abcdef9mPq",
		SOLBalance: models.TokenBalance{
			Symbol: "SOL",
			Amount: 1.0,
		},
		TotalUSD: 150.0,
	}
	updated, _ := m.Update(tui.PortfolioRefreshedMsg{Portfolio: portfolio})
	model := updated.(walletstatus.Model)
	view := model.View()
	if !strings.Contains(view, "No tokens") {
		t.Error("View with no tokens should show 'No tokens found'")
	}
}

func TestFilterLabel(t *testing.T) {
	m := loadedModel()

	// Default: no asterisk
	if strings.Contains(m.Footer(), "filter*") {
		t.Error("Footer should not show asterisk when filter is default")
	}

	// After one press (FilterAll): asterisk appears
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(walletstatus.Model)
	if !strings.Contains(model.Footer(), "filter*") {
		t.Error("Footer should show filter* when filter is non-default")
	}
}

func TestRenameKeyBinding(t *testing.T) {
	m := loadedModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(walletstatus.Model)
	if !model.IsRenaming() {
		t.Error("pressing R should enter rename mode")
	}
}

func TestRenameEscCancels(t *testing.T) {
	m := loadedModel()
	// Enter rename mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(walletstatus.Model)

	// Press esc to cancel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = updated.(walletstatus.Model)
	if model.IsRenaming() {
		t.Error("esc should cancel rename mode")
	}
}

func TestRenameEmptyRejects(t *testing.T) {
	m := loadedModel()
	// Enter rename mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(walletstatus.Model)

	// Press enter with empty input
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(walletstatus.Model)
	// Should still be in rename mode or show error
	view := model.View()
	if !strings.Contains(view, "empty") && !strings.Contains(view, "Error") {
		t.Error("empty label should show error")
	}
}

func TestRenameViewContainsInput(t *testing.T) {
	m := loadedModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	model := updated.(walletstatus.Model)
	view := model.View()
	if !strings.Contains(view, "Rename") {
		t.Error("rename view should contain 'Rename'")
	}
}

func TestStatusBarContainsRename(t *testing.T) {
	m := loadedModel()
	footer := m.Footer()
	if !strings.Contains(footer, "rename") {
		t.Error("Footer should contain rename")
	}
}

func TestWalletStatus_ResponsiveView_Narrow(t *testing.T) {
	m := walletstatus.New(nil, "addr123", nil)

	model, _ := m.Update(tui.PortfolioRefreshedMsg{
		Portfolio: models.PortfolioBalance{
			TotalUSD:   100.0,
			SOLBalance: models.TokenBalance{Symbol: "SOL", Amount: 1.0},
		},
	})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 15})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	footer := model.(interface{ Footer() string }).Footer()
	if !strings.Contains(footer, "menu") {
		t.Error("narrow view should contain menu")
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

	footer := model.(interface{ Footer() string }).Footer()
	if !strings.Contains(footer, "menu") {
		t.Error("wide view should contain menu")
	}
}
