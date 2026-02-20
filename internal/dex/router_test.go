package dex_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

func TestParseDexType(t *testing.T) {
	tests := []struct {
		dex      string
		poolType string
		want     dex.DexType
	}{
		{"orca", "whirlpool", dex.DexOrcaWhirlpool},
		{"Orca", "Whirlpool", dex.DexOrcaWhirlpool},
		{"raydium", "clmm", dex.DexRaydiumCLMM},
		{"Raydium", "CLMM", dex.DexRaydiumCLMM},
		{"raydium", "amm_v4", dex.DexRaydiumAMMV4},
		{"meteora", "dlmm", dex.DexMeteoraDLMM},
		{"unknown", "unknown", dex.DexUnknown},
		{"", "", dex.DexUnknown},
		{"raydium", "whirlpool", dex.DexUnknown},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.dex, tt.poolType), func(t *testing.T) {
			got := dex.ParseDexType(tt.dex, tt.poolType)
			if got != tt.want {
				t.Errorf("ParseDexType(%q, %q) = %v, want %v", tt.dex, tt.poolType, got, tt.want)
			}
		})
	}
}

func TestDexTypeString(t *testing.T) {
	tests := []struct {
		dt   dex.DexType
		want string
	}{
		{dex.DexOrcaWhirlpool, "Orca Whirlpool"},
		{dex.DexRaydiumCLMM, "Raydium CLMM"},
		{dex.DexRaydiumAMMV4, "Raydium AMM V4"},
		{dex.DexMeteoraDLMM, "Meteora DLMM"},
		{dex.DexJupiterAPI, "Jupiter API"},
		{dex.DexUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.dt.String()
			if got != tt.want {
				t.Errorf("DexType(%d).String() = %q, want %q", tt.dt, got, tt.want)
			}
		})
	}
}

func TestRouterFetchPriceJupiterFallback(t *testing.T) {
	mint := "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":-2.5}}}`, mint)
	}))
	defer server.Close()

	jupClient := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	mockSOLPrice := func() (float64, error) { return 150.0, nil }
	router := dex.NewRouterWithSOLPrice(nil, jupClient, mockSOLPrice)

	token := models.Token{
		Symbol:          "BONK",
		ContractAddress: mint,
		Pools: []models.PoolInfo{
			{Dex: "raydium", PoolType: "amm_v4", PoolAddress: "pool1"},
		},
	}

	price, err := router.FetchPrice(token)
	if err != nil {
		t.Fatalf("FetchPrice failed: %v", err)
	}

	if price.PriceUSD != 0.000028 {
		t.Errorf("expected price 0.000028, got %f", price.PriceUSD)
	}
	if price.Change24h != -2.5 {
		t.Errorf("expected change24h -2.5, got %f", price.Change24h)
	}
}

func TestRouterFetchPriceNoPools(t *testing.T) {
	mint := "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"%s":{"price":"0.000028","priceChange24h":1.0}}}`, mint)
	}))
	defer server.Close()

	jupClient := dex.NewJupiterClientWithHTTP(server.Client(), server.URL+"?ids=", server.URL+"?query=")
	mockSOLPrice := func() (float64, error) { return 150.0, nil }
	router := dex.NewRouterWithSOLPrice(nil, jupClient, mockSOLPrice)

	token := models.Token{
		Symbol:          "BONK",
		ContractAddress: mint,
		Pools:           nil, // No pools
	}

	price, err := router.FetchPrice(token)
	if err != nil {
		t.Fatalf("FetchPrice with no pools failed: %v", err)
	}
	if price.PriceUSD != 0.000028 {
		t.Errorf("expected price 0.000028, got %f", price.PriceUSD)
	}
}

func TestRouterQuoteToUSD(t *testing.T) {
	mockSOLPrice := func() (float64, error) { return 150.0, nil }
	router := dex.NewRouterWithSOLPrice(nil, nil, mockSOLPrice)

	tests := []struct {
		quoteToken   string
		priceInQuote float64
		wantUSD      float64
	}{
		{"sol", 0.001, 0.15},
		{"SOL", 1.0, 150.0},
		{"wsol", 0.5, 75.0},
		{"usdc", 1.5, 1.5},
		{"USDT", 100.0, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.quoteToken, func(t *testing.T) {
			got, err := router.QuoteToUSD(tt.quoteToken, tt.priceInQuote)
			if err != nil {
				t.Fatalf("QuoteToUSD(%q, %f) error: %v", tt.quoteToken, tt.priceInQuote, err)
			}
			if got != tt.wantUSD {
				t.Errorf("QuoteToUSD(%q, %f) = %f, want %f", tt.quoteToken, tt.priceInQuote, got, tt.wantUSD)
			}
		})
	}
}

func TestRouterQuoteToUSDUnknownToken(t *testing.T) {
	mockSOLPrice := func() (float64, error) { return 150.0, nil }
	router := dex.NewRouterWithSOLPrice(nil, nil, mockSOLPrice)

	_, err := router.QuoteToUSD("bonk", 1.0)
	if err == nil {
		t.Fatal("expected error for unknown quote token")
	}
}
