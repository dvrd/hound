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
	footer := m.Footer()
	if !strings.Contains(footer, "details") {
		t.Error("Footer should contain details in status bar")
	}
	if !strings.Contains(footer, "[a]dd") {
		t.Error("Footer should contain [a]dd in status bar")
	}
	if !strings.Contains(footer, "back") {
		t.Error("Footer should contain back in status bar")
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

func TestTokenList_ResponsiveView_Narrow(t *testing.T) {
	m := tokenlist.New(nil)

	tokens := make([]tokenlist.TokenRow, 20)
	for i := range tokens {
		tokens[i] = tokenlist.TokenRow{
			Token: models.Token{Symbol: fmt.Sprintf("TK%d", i), Name: fmt.Sprintf("Token %d", i)},
		}
	}
	model, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: tokens})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 12})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
	if !strings.Contains(view, "more") {
		t.Error("should show scroll indicator when tokens exceed visible rows")
	}
}

// searchModel returns a loaded model with three tokens for search testing:
//   - SOL  / Solana
//   - WSOL / Wrapped Solana
//   - BONK / Bonk
func searchModel() tokenlist.Model {
	m := newTestModel()
	rows := []tokenlist.TokenRow{
		{
			Token: models.Token{
				Symbol:          "SOL",
				Name:            "Solana",
				ContractAddress: "So11111111111111111111111111111111111111112",
				Chain:           "solana",
			},
		},
		{
			Token: models.Token{
				Symbol:          "WSOL",
				Name:            "Wrapped Solana",
				ContractAddress: "So11111111111111111111111111111111111111113",
				Chain:           "solana",
			},
		},
		{
			Token: models.Token{
				Symbol:          "BONK",
				Name:            "Bonk",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Chain:           "solana",
			},
		},
	}
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: rows})
	return updated.(tokenlist.Model)
}

// sendChars simulates typing a string into the model one rune at a time.
func sendChars(m tokenlist.Model, s string) tokenlist.Model {
	for _, r := range s {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tokenlist.Model)
	}
	return m
}

func TestSearchFiltersBySymbol(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "sol")

	filtered := m.GetFilteredTokens()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered tokens (SOL, WSOL), got %d", len(filtered))
	}
	syms := map[string]bool{}
	for _, tr := range filtered {
		syms[tr.Token.Symbol] = true
	}
	if !syms["SOL"] || !syms["WSOL"] {
		t.Errorf("expected SOL and WSOL in filtered results, got %v", syms)
	}
}

func TestSearchFiltersByName(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "wrapped")

	filtered := m.GetFilteredTokens()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered token (WSOL), got %d", len(filtered))
	}
	if filtered[0].Token.Symbol != "WSOL" {
		t.Errorf("expected WSOL, got %s", filtered[0].Token.Symbol)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	m := searchModel()

	mLower := sendChars(m, "sol")
	mUpper := sendChars(m, "SOL")

	if len(mLower.GetFilteredTokens()) != len(mUpper.GetFilteredTokens()) {
		t.Errorf("case-insensitive: lower=%d upper=%d, should be equal",
			len(mLower.GetFilteredTokens()), len(mUpper.GetFilteredTokens()))
	}
}

func TestSearchResetsCursor(t *testing.T) {
	m := searchModel()

	// Move cursor to index 2 (BONK)
	m = sendChars(m, "") // no-op to get a clean model
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tokenlist.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tokenlist.Model)
	if m.GetCursor() != 2 {
		t.Fatalf("precondition: cursor should be 2, got %d", m.GetCursor())
	}

	// Now type a search query — cursor must reset to 0
	m = sendChars(m, "sol")
	if m.GetCursor() != 0 {
		t.Errorf("cursor after filter = %d, want 0", m.GetCursor())
	}
}

func TestSearchEmptyShowsAll(t *testing.T) {
	m := searchModel()
	total := len(m.GetTokens())

	// Type something, then clear it
	m = sendChars(m, "sol")
	if len(m.GetFilteredTokens()) == total {
		t.Fatal("precondition: filter should have reduced the list")
	}

	// Backspace to clear (simulate clearing via esc)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tokenlist.Model)

	if len(m.GetFilteredTokens()) != total {
		t.Errorf("after clearing search, filteredTokens=%d, want %d (all)", len(m.GetFilteredTokens()), total)
	}
}

func TestEscClearsSearchWhenNonEmpty(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "sol")

	// esc with non-empty input must NOT navigate back
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tokenlist.Model)

	if cmd != nil {
		// cmd must not be a NavigateBackMsg
		msg := cmd()
		if _, ok := msg.(tui.NavigateBackMsg); ok {
			t.Error("esc with non-empty search should NOT emit NavigateBackMsg")
		}
	}

	// Search input must be cleared
	if len(m.GetFilteredTokens()) != len(m.GetTokens()) {
		t.Errorf("after esc clear, filteredTokens=%d, want %d (all)", len(m.GetFilteredTokens()), len(m.GetTokens()))
	}
}

func TestEscNavigatesBackWhenSearchEmpty(t *testing.T) {
	m := searchModel()
	// No typing — search is empty

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc with empty search should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc with empty search should return NavigateBackMsg, got %T", msg)
	}
}

func TestNoMatchShowsMessage(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "zzznomatch")

	view := m.View()
	if !strings.Contains(view, "No tokens match") {
		t.Errorf("view should contain 'No tokens match' when filter has no results, got:\n%s", view)
	}
}

func TestEnterOpensFilteredToken(t *testing.T) {
	m := searchModel()
	// Filter to WSOL only
	m = sendChars(m, "wrapped")

	filtered := m.GetFilteredTokens()
	if len(filtered) != 1 || filtered[0].Token.Symbol != "WSOL" {
		t.Fatalf("precondition: expected only WSOL in filtered list")
	}

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
		t.Errorf("NavigateMsg.View = %q, want token-fetch", navMsg.View)
	}
	if navMsg.Data != "So11111111111111111111111111111111111111113" {
		t.Errorf("NavigateMsg.Data = %v, want WSOL contract address", navMsg.Data)
	}
}

func TestFooterDynamicWithSearch(t *testing.T) {
	m := searchModel()

	// Empty search — standard footer
	footer := m.Footer()
	if !strings.Contains(footer, "[a]dd") {
		t.Errorf("empty search footer should contain [a]dd, got: %s", footer)
	}
	if strings.Contains(footer, "clear search") {
		t.Errorf("empty search footer should NOT contain 'clear search', got: %s", footer)
	}

	// Non-empty search — modified footer
	m = sendChars(m, "sol")
	footer = m.Footer()
	if !strings.Contains(footer, "clear search") {
		t.Errorf("active search footer should contain 'clear search', got: %s", footer)
	}
	if strings.Contains(footer, "[a]dd") {
		t.Errorf("active search footer should NOT contain [a]dd, got: %s", footer)
	}
}

func TestGetTokensReturnsUnfiltered(t *testing.T) {
	m := searchModel()
	total := len(m.GetTokens())

	m = sendChars(m, "sol")
	if len(m.GetFilteredTokens()) >= total {
		t.Fatal("precondition: filter should have reduced the list")
	}

	// GetTokens must still return the full unfiltered list
	if len(m.GetTokens()) != total {
		t.Errorf("GetTokens() after filter = %d, want %d (unfiltered)", len(m.GetTokens()), total)
	}
}
