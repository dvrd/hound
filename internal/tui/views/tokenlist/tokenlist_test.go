package tokenlist_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/tokenlist"
)

func newTestModel() tokenlist.Model {
	return tokenlist.New(nil)
}

func loadedModel() tokenlist.Model {
	m := newTestModel()
	rows := []tokenlist.TokenRow{
		{
			Token: models.Token{
				Symbol:          "BONK",
				Name:            "Bonk",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Chain:           "solana",
				Pools: []models.PoolInfo{
					{Dex: "raydium", PoolAddress: "pool1", DiscoveredAt: 1700000000},
				},
			},
			PoolStats: models.PoolStats{PoolCount: 2, TotalLiquidity: 1500000},
		},
		{
			Token: models.Token{
				Symbol:          "WIF",
				Name:            "dogwifhat",
				ContractAddress: "EKpQGSJtjMFqKZ9KQanSqYXRcF8fBopzLHYxdM65zcjm",
				Chain:           "solana",
			},
			PoolStats: models.PoolStats{PoolCount: 1, TotalLiquidity: 500000},
		},
	}
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: rows})
	return updated.(tokenlist.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Token List") {
		t.Errorf("View should contain 'Token List', got %q", view)
	}
}

func TestViewContainsTokens(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "BONK") {
		t.Error("View should contain 'BONK'")
	}
	if !strings.Contains(view, "WIF") {
		t.Error("View should contain 'WIF'")
	}
}

func TestViewContainsTokenName(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Bonk") {
		t.Error("View should contain token name 'Bonk'")
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "[enter]details") {
		t.Error("View should contain [enter]details in status bar")
	}
	if !strings.Contains(view, "[a]dd") {
		t.Error("View should contain [a]dd in status bar")
	}
	if !strings.Contains(view, "[esc]back") {
		t.Error("View should contain [esc]back in status bar")
	}
}

func TestViewAutoDiscovered(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "auto") {
		t.Error("View should show 'auto' for auto-discovered tokens")
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

func TestCursorNavigation(t *testing.T) {
	m := loadedModel()

	// Initial cursor at 0
	if m.GetCursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.GetCursor())
	}

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(tokenlist.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after j = %d, want 1", model.GetCursor())
	}

	// Move down again (should stay at 1, only 2 items)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(tokenlist.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after second j = %d, want 1 (boundary)", model.GetCursor())
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(tokenlist.Model)
	if model.GetCursor() != 0 {
		t.Errorf("cursor after k = %d, want 0", model.GetCursor())
	}

	// Move up again (should stay at 0)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(tokenlist.Model)
	if model.GetCursor() != 0 {
		t.Errorf("cursor after second k = %d, want 0 (boundary)", model.GetCursor())
	}
}

func TestEnterNavigatesToTokenFetch(t *testing.T) {
	m := loadedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return a command")
	}
	msg := cmd()
	navMsg, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("enter should return NavigateMsg, got %T", msg)
	}
	if navMsg.View != "token-fetch" {
		t.Errorf("NavigateMsg.View = %q, want %q", navMsg.View, "token-fetch")
	}
	if navMsg.Data != "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263" {
		t.Errorf("NavigateMsg.Data = %v, want BONK contract address", navMsg.Data)
	}
}

func TestANavigatesToTokenAdd(t *testing.T) {
	m := loadedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("'a' should return a command")
	}
	msg := cmd()
	navMsg, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("'a' should return NavigateMsg, got %T", msg)
	}
	if navMsg.View != "token-add" {
		t.Errorf("NavigateMsg.View = %q, want %q", navMsg.View, "token-add")
	}
}

func TestTokensLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Err: fmt.Errorf("db error")})
	model := updated.(tokenlist.Model)
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error when TokensLoadedMsg has error")
	}
}

func TestEmptyTokens(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: nil})
	model := updated.(tokenlist.Model)
	view := model.View()
	if !strings.Contains(view, "No tokens") {
		t.Error("View with no tokens should show 'No tokens found'")
	}
}

func TestEnterWithNoTokens(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: nil})
	model := updated.(tokenlist.Model)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter with no tokens should not return a command")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(tokenlist.Model)
	// Should not panic
}

func TestLoadingView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Token List") {
		t.Error("loading view should contain title")
	}
}
