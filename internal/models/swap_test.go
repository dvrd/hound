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

// TestRouteLabelSingleHop verifies that a single-hop route shows "INPUT → DEX → OUTPUT".
func TestRouteLabelSingleHop(t *testing.T) {
	q := models.SwapQuote{
		RoutePlan: []models.RouteStep{
			{DexLabel: "Raydium"},
		},
	}
	got := q.RouteLabel("SOL", "USDC")
	want := "SOL → Raydium → USDC"
	if got != want {
		t.Errorf("RouteLabel (single hop) = %q, want %q", got, want)
	}
}

// TestRouteLabelMultiHop verifies that 2+ hops show all DEX labels and "(N hops)" suffix.
func TestRouteLabelMultiHop(t *testing.T) {
	tests := []struct {
		name      string
		steps     []models.RouteStep
		input     string
		output    string
		wantLabel string
	}{
		{
			name: "two hops",
			steps: []models.RouteStep{
				{DexLabel: "Raydium"},
				{DexLabel: "Orca"},
			},
			input:     "SOL",
			output:    "USDC",
			wantLabel: "SOL → Raydium → Orca → USDC (2 hops)",
		},
		{
			name: "three hops",
			steps: []models.RouteStep{
				{DexLabel: "Raydium"},
				{DexLabel: "Orca"},
				{DexLabel: "Jupiter"},
			},
			input:     "SOL",
			output:    "BONK",
			wantLabel: "SOL → Raydium → Orca → Jupiter → BONK (3 hops)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := models.SwapQuote{RoutePlan: tt.steps}
			got := q.RouteLabel(tt.input, tt.output)
			if got != tt.wantLabel {
				t.Errorf("RouteLabel = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}

// TestRouteLabelEmptyRoutePlan verifies graceful fallback when RoutePlan is empty.
func TestRouteLabelEmptyRoutePlan(t *testing.T) {
	q := models.SwapQuote{
		RoutePlan: []models.RouteStep{},
	}
	got := q.RouteLabel("SOL", "USDC")
	// Empty route plan should produce a non-empty label (direct route fallback)
	if got == "" {
		t.Error("RouteLabel with empty RoutePlan returned empty string")
	}
	// Should contain both symbols
	if !containsStr(got, "SOL") {
		t.Errorf("RouteLabel = %q, want to contain 'SOL'", got)
	}
	if !containsStr(got, "USDC") {
		t.Errorf("RouteLabel = %q, want to contain 'USDC'", got)
	}
}

// TestRouteLabelNilRoutePlan verifies graceful fallback when RoutePlan is nil.
func TestRouteLabelNilRoutePlan(t *testing.T) {
	q := models.SwapQuote{
		RoutePlan: nil,
	}
	got := q.RouteLabel("SOL", "BONK")
	if got == "" {
		t.Error("RouteLabel with nil RoutePlan returned empty string")
	}
	if !containsStr(got, "SOL") {
		t.Errorf("RouteLabel = %q, want to contain 'SOL'", got)
	}
	if !containsStr(got, "BONK") {
		t.Errorf("RouteLabel = %q, want to contain 'BONK'", got)
	}
}

// TestSwapQuoteErrQuoteExpired verifies that ErrQuoteExpired is the right sentinel for expired quotes.
// S1: quote older than 90s should be considered expired.
func TestSwapQuoteErrQuoteExpired(t *testing.T) {
	// A quote fetched 91 seconds ago is expired
	q := &models.SwapQuote{FetchedAt: time.Now().Add(-91 * time.Second)}
	if !q.IsExpired() {
		t.Error("quote fetched 91s ago should be expired")
	}

	// Verify ErrQuoteExpired is the correct sentinel error for this condition
	if models.ErrQuoteExpired == nil {
		t.Error("ErrQuoteExpired sentinel should not be nil")
	}
	if models.ErrQuoteExpired.Error() == "" {
		t.Error("ErrQuoteExpired should have a non-empty message")
	}
}

// containsStr is a helper to check if s contains substr.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
