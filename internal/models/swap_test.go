package models_test

import (
	"testing"
	"time"

	"github.com/dvrd/hound/internal/models"
)

func TestSwapQuoteIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		fetchedAt time.Time
		want      bool
	}{
		{
			name:      "fresh quote",
			fetchedAt: time.Now(),
			want:      false,
		},
		{
			name:      "30 seconds old",
			fetchedAt: time.Now().Add(-30 * time.Second),
			want:      false,
		},
		{
			name:      "89 seconds old",
			fetchedAt: time.Now().Add(-89 * time.Second),
			want:      false,
		},
		{
			name:      "91 seconds old",
			fetchedAt: time.Now().Add(-91 * time.Second),
			want:      true,
		},
		{
			name:      "5 minutes old",
			fetchedAt: time.Now().Add(-5 * time.Minute),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &models.SwapQuote{FetchedAt: tt.fetchedAt}
			got := q.IsExpired()
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v (age: %v)", got, tt.want, time.Since(tt.fetchedAt))
			}
		})
	}
}

func TestSwapQuoteTTL(t *testing.T) {
	if models.QuoteTTL != 90*time.Second {
		t.Errorf("QuoteTTL = %v, want 90s", models.QuoteTTL)
	}
}

func TestSwapHistoryEntry(t *testing.T) {
	entry := models.SwapHistoryEntry{
		WalletAddress: "7xKXtg...",
		InputMint:     "So11111111111111111111111111111111111111112",
		OutputMint:    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InputSymbol:   "SOL",
		OutputSymbol:  "USDC",
		InputAmount:   1.0,
		OutputAmount:  145.32,
		PriceImpact:   0.05,
		SlippageBps:   50,
		Signature:     "5KtR...abc123",
		Status:        "finalized",
		Dex:           "Jupiter",
		CreatedAt:     1771567845,
	}

	if entry.InputSymbol != "SOL" {
		t.Errorf("input symbol = %q, want %q", entry.InputSymbol, "SOL")
	}
	if entry.Status != "finalized" {
		t.Errorf("status = %q, want %q", entry.Status, "finalized")
	}
}

func TestRouteStep(t *testing.T) {
	step := models.RouteStep{
		DexLabel:   "Orca",
		InputMint:  "So111...",
		OutputMint: "EPjFW...",
		InAmount:   "1000000000",
		OutAmount:  "145320000",
		Percent:    60,
	}

	if step.Percent != 60 {
		t.Errorf("percent = %d, want 60", step.Percent)
	}
}
