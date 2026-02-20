package models_test

import (
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestGetTokenDecimals(t *testing.T) {
	tests := []struct {
		name  string
		token models.Token
		want  int
	}{
		{
			name:  "explicit decimals",
			token: models.Token{Symbol: "BONK", Decimals: 5},
			want:  5,
		},
		{
			name:  "USDC fallback",
			token: models.Token{Symbol: "USDC", Decimals: 0},
			want:  6,
		},
		{
			name:  "usdc lowercase",
			token: models.Token{Symbol: "usdc", Decimals: 0},
			want:  6,
		},
		{
			name:  "USDT fallback",
			token: models.Token{Symbol: "USDT", Decimals: 0},
			want:  6,
		},
		{
			name:  "SOL fallback",
			token: models.Token{Symbol: "SOL", Decimals: 0},
			want:  9,
		},
		{
			name:  "sol lowercase",
			token: models.Token{Symbol: "sol", Decimals: 0},
			want:  9,
		},
		{
			name:  "unknown defaults to 9",
			token: models.Token{Symbol: "UNKNOWN", Decimals: 0},
			want:  9,
		},
		{
			name:  "explicit overrides fallback",
			token: models.Token{Symbol: "USDC", Decimals: 8},
			want:  8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.GetTokenDecimals(tt.token)
			if got != tt.want {
				t.Errorf("GetTokenDecimals(%+v) = %d, want %d", tt.token, got, tt.want)
			}
		})
	}
}

func TestTokenStruct(t *testing.T) {
	token := models.Token{
		Symbol:          "SOL",
		Name:            "Solana",
		ContractAddress: "So11111111111111111111111111111111111111112",
		Chain:           "solana",
		Decimals:        9,
		IsQuoteToken:    false,
		USDPrice:        145.32,
		Pools: []models.PoolInfo{
			{
				Dex:          "raydium",
				PoolAddress:  "58oQChx4yWmvKdwLLZzBi4ChoCc2fqCUWBkwMihLYQo2",
				QuoteToken:   "sol",
				PoolType:     "amm_v4",
				LiquidityUSD: 1234567.89,
				Volume24h:    456789.12,
				FeePercent:   0.25,
				DiscoveredAt: 1640000000,
			},
		},
	}

	if len(token.Pools) != 1 {
		t.Errorf("pools count = %d, want 1", len(token.Pools))
	}
	if token.Pools[0].Dex != "raydium" {
		t.Errorf("pool dex = %q, want %q", token.Pools[0].Dex, "raydium")
	}
}

func TestPoolStats(t *testing.T) {
	stats := models.PoolStats{
		PoolCount:      3,
		TotalLiquidity: 1234567.89,
	}

	if stats.PoolCount != 3 {
		t.Errorf("pool count = %d, want 3", stats.PoolCount)
	}
}

func TestTokenExtendedInfo(t *testing.T) {
	info := models.TokenExtendedInfo{
		Symbols:     []string{"BONK", "bonk"},
		Name:        "Bonk",
		Network:     "solana",
		MintAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
		PriceUSD:    0.000028,
		MarketCap:   1230000000,
		FDV:         2450000000,
		TopHolders: []models.TopHolder{
			{Address: "7xKXtg...", Balance: 1234567, OwnershipPct: 12.34},
		},
		IsActive: true,
	}

	if len(info.Symbols) != 2 {
		t.Errorf("symbols count = %d, want 2", len(info.Symbols))
	}
	if len(info.TopHolders) != 1 {
		t.Errorf("top holders count = %d, want 1", len(info.TopHolders))
	}
}

func TestPriceData(t *testing.T) {
	pd := models.PriceData{
		PriceUSD:  145.32,
		Change24h: 2.3,
	}

	if pd.PriceUSD != 145.32 {
		t.Errorf("price = %f, want 145.32", pd.PriceUSD)
	}
}
