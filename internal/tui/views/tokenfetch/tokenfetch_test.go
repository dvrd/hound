package tokenfetch_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/tokenfetch"
)

func newTestModel() tokenfetch.Model {
	return tokenfetch.New("BONK", nil, nil)
}

func loadedModel() tokenfetch.Model {
	m := newTestModel()
	info := models.TokenExtendedInfo{
		Symbols:      []string{"BONK"},
		Name:         "Bonk",
		Network:      "solana",
		MintAddress:  "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
		PriceUSD:     0.000028,
		MarketCap:    1800000000,
		FDV:          2000000000,
		LiquidityUSD: 15000000,
		Volume24h:    5000000,
		Txns24h:      12000,
		Buys24h:      7000,
		Sells24h:     5000,
		PriceChange: models.PriceChanges{
			M5:  1.5,
			H1:  -0.3,
			H6:  2.1,
			H24: 5.2,
		},
		TopHolders: []models.TopHolder{
			{Address: "7xKXabc1234567890abcdef9mPqRSTUVWXYZ12345678", Balance: 1000000000, OwnershipPct: 5.5},
			{Address: "9yLYdef4567890abcdef1234nOpQRSTUVWXYZ87654321", Balance: 500000000, OwnershipPct: 2.75},
		},
		IsActive: true,
	}
	updated, _ := m.Update(tokenfetch.TokenInfoLoadedMsg{Info: info})
	return updated.(tokenfetch.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	if !m.IsLoading() {
		t.Error("new model should be in loading state")
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Token Info") {
		t.Errorf("loading view should contain 'Token Info', got %q", view)
	}
}

func TestLoadedViewContainsTokenName(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Bonk") {
		t.Error("View should contain token name 'Bonk'")
	}
	if !strings.Contains(view, "BONK") {
		t.Error("View should contain token symbol 'BONK'")
	}
}

func TestLoadedViewContainsMarketData(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Market Data") {
		t.Error("View should contain 'Market Data' section")
	}
	if !strings.Contains(view, "Price") {
		t.Error("View should contain 'Price'")
	}
	if !strings.Contains(view, "Market Cap") {
		t.Error("View should contain 'Market Cap'")
	}
	if !strings.Contains(view, "FDV") {
		t.Error("View should contain 'FDV'")
	}
	if !strings.Contains(view, "Liquidity") {
		t.Error("View should contain 'Liquidity'")
	}
}

func TestLoadedViewContainsTradingData(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Trading") {
		t.Error("View should contain 'Trading' section")
	}
	if !strings.Contains(view, "Volume") {
		t.Error("View should contain 'Volume'")
	}
	if !strings.Contains(view, "12000") {
		t.Error("View should contain txn count '12000'")
	}
	if !strings.Contains(view, "7000") {
		t.Error("View should contain buys count '7000'")
	}
}

func TestLoadedViewContainsPriceChanges(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Price Changes") {
		t.Error("View should contain 'Price Changes' section")
	}
	if !strings.Contains(view, "5m") {
		t.Error("View should contain '5m' price change")
	}
	if !strings.Contains(view, "24h") {
		t.Error("View should contain '24h' price change")
	}
}

func TestLoadedViewContainsTopHolders(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Top Holders") {
		t.Error("View should contain 'Top Holders' section")
	}
	if !strings.Contains(view, "5.50%") {
		t.Error("View should contain holder ownership percentage")
	}
}

func TestLoadedViewContainsMintAddress(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263") {
		t.Error("View should contain mint address")
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

func TestTokenInfoLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tokenfetch.TokenInfoLoadedMsg{Err: models.ErrTokenNotFound})
	model := updated.(tokenfetch.Model)
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error when TokenInfoLoadedMsg has error")
	}
	footer := model.Footer()
	if !strings.Contains(footer, "[esc]back") {
		t.Error("Error view should still show [esc]back in footer")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(tokenfetch.Model)
	// Should not panic
}

func TestViewStatusBar(t *testing.T) {
	m := loadedModel()
	footer := m.Footer()
	if !strings.Contains(footer, "[esc]back") {
		t.Error("Footer should contain [esc]back in status bar")
	}
}

func TestNoTopHolders(t *testing.T) {
	m := newTestModel()
	info := models.TokenExtendedInfo{
		Symbols:    []string{"TEST"},
		Name:       "Test Token",
		TopHolders: nil,
	}
	updated, _ := m.Update(tokenfetch.TokenInfoLoadedMsg{Info: info})
	model := updated.(tokenfetch.Model)
	view := model.View()
	if strings.Contains(view, "Top Holders") {
		t.Error("View should not contain 'Top Holders' when there are none")
	}
}
