package tokenlist_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/tokenlist"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestModel() tokenlist.Model {
	return tokenlist.NewWithJupiter(nil, dex.NewJupiterClient())
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

// searchModel returns a loaded model with SOL, WSOL, BONK for search testing.
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

// jupiterServer returns a test HTTP server that serves a fixed Jupiter-style
// JSON response for any search query.
func jupiterServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

// modelWithJupiter creates a model wired to a test Jupiter server.
func modelWithJupiter(t *testing.T, serverBody string) (tokenlist.Model, *httptest.Server) {
	t.Helper()
	srv := jupiterServer(t, serverBody)
	httpClient := &http.Client{}
	j := dex.NewJupiterClientWithHTTP(httpClient, "https://unused/", srv.URL+"/search?query=")
	m := tokenlist.NewWithJupiter(nil, j)
	updated, _ := m.Update(tokenlist.TokensLoadedMsg{Tokens: nil})
	return updated.(tokenlist.Model), srv
}

// ─── Basic view tests ────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Tokens") {
		t.Errorf("View should contain 'Tokens', got %q", view)
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
	if !strings.Contains(footer, "navigate") {
		t.Error("Footer should contain navigate hint")
	}
	if !strings.Contains(footer, "back") {
		t.Error("Footer should contain back hint")
	}
}

func TestFooterHasNoAddAction(t *testing.T) {
	m := loadedModel()
	footer := m.Footer()
	if strings.Contains(footer, "[a]dd") {
		t.Error("Footer must not contain [a]dd — add is done via search")
	}
}

func TestEscNavigatesBackWhenSearchEmpty(t *testing.T) {
	// esc with empty search → navigate back to main menu
	m := loadedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc with empty search should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc with empty search should return NavigateBackMsg, got %T", msg)
	}
}

func TestCursorNavigation(t *testing.T) {
	m := loadedModel()

	if m.GetCursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.GetCursor())
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(tokenlist.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after j = %d, want 1", model.GetCursor())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(tokenlist.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after second j = %d, want 1 (boundary)", model.GetCursor())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(tokenlist.Model)
	if model.GetCursor() != 0 {
		t.Errorf("cursor after k = %d, want 0", model.GetCursor())
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
		t.Error("View with no tokens should show 'No tokens saved yet'")
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
}

func TestLoadingView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Tokens") {
		t.Error("loading view should contain title")
	}
}

func TestTokenList_ResponsiveView_Narrow(t *testing.T) {
	m := tokenlist.NewWithJupiter(nil, dex.NewJupiterClient())

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

// ─── Search: esc behaviour ───────────────────────────────────────────────────

func TestEscClearsSearchWhenNonEmpty(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "sol")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tokenlist.Model)

	// Must not navigate back
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tui.NavigateBackMsg); ok {
			t.Error("esc with non-empty search should NOT emit NavigateBackMsg")
		}
	}

	// Must be back to idle/saved mode
	if m.IsInSearchMode() {
		t.Error("after esc clear, should no longer be in search mode")
	}
}

func TestEscNavigatesBackAfterClearingSearch(t *testing.T) {
	// First esc clears search, second esc navigates back.
	m := searchModel()
	m = sendChars(m, "sol")

	// First esc: clear search
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(tokenlist.Model)
	if m.IsInSearchMode() {
		t.Error("after first esc, should no longer be in search mode")
	}

	// Second esc: navigate back
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("second esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("second esc should return NavigateBackMsg, got %T", msg)
	}
}

// ─── Search: cursor reset ─────────────────────────────────────────────────────

func TestSearchResetsCursor(t *testing.T) {
	m := searchModel()

	// Move cursor to index 2
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tokenlist.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tokenlist.Model)
	if m.GetCursor() != 2 {
		t.Fatalf("precondition: cursor should be 2, got %d", m.GetCursor())
	}

	// Typing resets cursor to 0
	m = sendChars(m, "s")
	if m.GetCursor() != 0 {
		t.Errorf("cursor after typing = %d, want 0", m.GetCursor())
	}
}

// ─── Search: footer ───────────────────────────────────────────────────────────

func TestFooterDynamicWithSearch(t *testing.T) {
	m := searchModel()

	footer := m.Footer()
	if strings.Contains(footer, "clear") {
		t.Errorf("empty search footer should NOT contain 'clear', got: %s", footer)
	}

	m = sendChars(m, "sol")
	footer = m.Footer()
	if !strings.Contains(footer, "clear") {
		t.Errorf("active search footer should contain 'clear', got: %s", footer)
	}
}

// ─── Search: GetTokens stays unfiltered ──────────────────────────────────────

func TestGetTokensReturnsUnfiltered(t *testing.T) {
	m := searchModel()
	total := len(m.GetTokens())

	m = sendChars(m, "sol")

	if len(m.GetTokens()) != total {
		t.Errorf("GetTokens() after typing = %d, want %d (unfiltered)", len(m.GetTokens()), total)
	}
}

// ─── Search: live Jupiter results ────────────────────────────────────────────

const jupiterTwoResults = `[
  {"id":"So11111111111111111111111111111111111111112","symbol":"SOL","name":"Solana","decimals":9},
  {"id":"EKpQGSJtjMFqKZ9KQanSqYXRcF8fBopzLHYxdM65zcjm","symbol":"WSOL","name":"Wrapped SOL","decimals":9}
]`

func TestSearchResultsDisplayed(t *testing.T) {
	m, srv := modelWithJupiter(t, jupiterTwoResults)
	defer srv.Close()

	// Type the query so searchInput.Value() == "sol"
	m = sendChars(m, "sol")

	// Inject search results directly (bypasses debounce timer)
	updated, _ := m.Update(searchResultsMsgFor("sol", []tokenlist.SearchResult{
		{Symbol: "SOL", Name: "Solana", Address: "So11111111111111111111111111111111111111112"},
		{Symbol: "WSOL", Name: "Wrapped SOL", Address: "EKpQGSJtjMFqKZ9KQanSqYXRcF8fBopzLHYxdM65zcjm"},
	}))
	m = updated.(tokenlist.Model)

	view := m.View()
	if !strings.Contains(view, "SOL") {
		t.Error("View should contain SOL from search results")
	}
	if !strings.Contains(view, "WSOL") {
		t.Error("View should contain WSOL from search results")
	}
}

func TestSearchResultsSavedBadge(t *testing.T) {
	// Load a model with SOL saved
	base := tokenlist.NewWithJupiter(nil, dex.NewJupiterClient())
	savedRows := []tokenlist.TokenRow{
		{Token: models.Token{
			Symbol:          "SOL",
			Name:            "Solana",
			ContractAddress: "So11111111111111111111111111111111111111112",
		}},
	}
	updated, _ := base.Update(tokenlist.TokensLoadedMsg{Tokens: savedRows})
	m := updated.(tokenlist.Model)

	// Type query so searchInput.Value() matches
	m = sendChars(m, "sol")

	// Inject search results where SOL is saved and WSOL is not
	updated, _ = m.Update(searchResultsMsgFor("sol", []tokenlist.SearchResult{
		{Symbol: "SOL", Name: "Solana", Address: "So11111111111111111111111111111111111111112", Saved: true},
		{Symbol: "WSOL", Name: "Wrapped SOL", Address: "EKpQGSJtjMFqKZ9KQanSqYXRcF8fBopzLHYxdM65zcjm", Saved: false},
	}))
	m = updated.(tokenlist.Model)

	results := m.GetResults()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Saved {
		t.Error("SOL result should be marked Saved=true")
	}
	if results[1].Saved {
		t.Error("WSOL result should be marked Saved=false")
	}
}

func TestNoSearchResultsShowsMessage(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "zzz")

	// Inject empty results — query must match searchInput.Value()
	updated, _ := m.Update(searchResultsMsgFor("zzz", []tokenlist.SearchResult{}))
	m = updated.(tokenlist.Model)

	view := m.View()
	if !strings.Contains(view, "No results") {
		t.Errorf("view should contain 'No results' for empty search, got:\n%s", view)
	}
}

func TestEnterOpensSearchResult(t *testing.T) {
	m := searchModel()
	m = sendChars(m, "sol")

	updated, _ := m.Update(searchResultsMsgFor("sol", []tokenlist.SearchResult{
		{Symbol: "SOL", Name: "Solana", Address: "So11111111111111111111111111111111111111112"},
	}))
	m = updated.(tokenlist.Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on search result should return a command")
	}
	msg := cmd()
	navMsg, ok := msg.(tui.NavigateMsg)
	if !ok {
		t.Fatalf("enter should return NavigateMsg, got %T", msg)
	}
	if navMsg.View != "token-fetch" {
		t.Errorf("NavigateMsg.View = %q, want token-fetch", navMsg.View)
	}
	if navMsg.Data != "So11111111111111111111111111111111111111112" {
		t.Errorf("NavigateMsg.Data = %v, want SOL address", navMsg.Data)
	}
}

// ─── Internal test helper ─────────────────────────────────────────────────────

// searchResultsMsgFor constructs the internal searchResultsMsg. Because the
// type is unexported we rely on the exported Update path — but we need to
// inject the message directly to avoid network calls. We expose it via a
// package-level helper in the tokenlist package (see export_test.go).
func searchResultsMsgFor(query string, results []tokenlist.SearchResult) tea.Msg {
	return tokenlist.NewSearchResultsMsg(query, results, nil)
}
