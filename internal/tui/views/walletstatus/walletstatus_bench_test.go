package walletstatus

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
)

func BenchmarkView_20Tokens(b *testing.B) {
	// Build a portfolio with 20 tokens.
	tokens := make([]models.TokenBalance, 20)
	for i := range tokens {
		tokens[i] = models.TokenBalance{
			Mint:      "mint" + string(rune('A'+i)),
			Symbol:    "TKN" + string(rune('A'+i)),
			Name:      "Token " + string(rune('A'+i)) + " Name Here",
			Amount:    float64(i+1) * 1.234,
			Decimals:  9,
			USDPrice:  float64(i+1) * 0.567,
			USDValue:  float64(i+1) * 1.234 * 0.567,
			Change24h: float64(i%5) - 2.0,
		}
	}

	m := Model{
		wallet:      models.Wallet{Label: "Test Wallet", Address: "7xKXabc123def456ghi789jkl012mno345pqr678stu9"},
		portfolio:   models.PortfolioBalance{WalletAddress: "7xKXabc123", SOLBalance: models.TokenBalance{Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL", Amount: 5.5, USDPrice: 142.0, USDValue: 781.0, Change24h: 2.3}, TokenBalances: tokens, TotalUSD: 1234.56},
		hasData:     true,
		width:       120,
		height:      40,
		hiddenMints: make(map[string]bool),
		address:     "7xKXabc123",
	}

	// Trigger the cached row build.
	m.recomputeVisible()
	m.rebuildHeader()

	// Simulate a WindowSizeMsg so inner dimensions are set.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkView_Footer(b *testing.B) {
	m := Model{hasData: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Footer()
	}
}

// Ensure tui package is referenced to avoid unused import.
var _ = tui.RenderRow
