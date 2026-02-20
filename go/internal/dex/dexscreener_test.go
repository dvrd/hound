package dex_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

const testTokenAddr = "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263"

func dexScreenerResponse(chainID string) string {
	return `{
		"pairs": [
			{
				"chainId": "` + chainID + `",
				"dexId": "raydium",
				"pairAddress": "pool1",
				"baseToken": {"address": "` + testTokenAddr + `", "name": "Bonk", "symbol": "BONK"},
				"quoteToken": {"address": "So11111111111111111111111111111111111111112", "name": "SOL", "symbol": "SOL"},
				"priceNative": "0.0000002",
				"priceUsd": "0.000028",
				"txns": {"h24": {"buys": 100, "sells": 50}},
				"volume": {"h24": 500000},
				"priceChange": {"m5": 0.5, "h1": 1.2, "h6": -0.8, "h24": 5.2},
				"liquidity": {"usd": 250000, "base": 1000000000, "quote": 500},
				"fdv": 1500000,
				"marketCap": 1200000,
				"pairCreatedAt": 1700000000000
			},
			{
				"chainId": "ethereum",
				"dexId": "uniswap",
				"pairAddress": "pool2",
				"baseToken": {"address": "0xabc", "name": "Bonk", "symbol": "BONK"},
				"quoteToken": {"address": "0xdef", "name": "WETH", "symbol": "WETH"},
				"priceNative": "0.00000001",
				"priceUsd": "0.000027",
				"txns": {"h24": {"buys": 10, "sells": 5}},
				"volume": {"h24": 10000},
				"priceChange": {"m5": 0.1, "h1": 0.5, "h6": -0.2, "h24": 3.0},
				"liquidity": {"usd": 50000, "base": 100000000, "quote": 10},
				"fdv": 1400000,
				"marketCap": 1100000,
				"pairCreatedAt": 1700000000000
			}
		]
	}`
}

func TestDexScreenerFetchPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dexScreenerResponse("solana")))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	price, err := client.FetchPrice(testTokenAddr)
	if err != nil {
		t.Fatalf("FetchPrice failed: %v", err)
	}

	if price.PriceUSD != 0.000028 {
		t.Errorf("expected price 0.000028, got %f", price.PriceUSD)
	}
	if price.Change24h != 5.2 {
		t.Errorf("expected change24h 5.2, got %f", price.Change24h)
	}
}

func TestDexScreenerFetchPriceCacheHit(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Write([]byte(dexScreenerResponse("solana")))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")

	// First call
	_, err := client.FetchPrice(testTokenAddr)
	if err != nil {
		t.Fatalf("first FetchPrice failed: %v", err)
	}

	// Second call should use cache
	_, err = client.FetchPrice(testTokenAddr)
	if err != nil {
		t.Fatalf("second FetchPrice failed: %v", err)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount.Load())
	}
}

func TestDexScreenerFetchPoolsForTokenSolanaFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dexScreenerResponse("solana")))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	pools, err := client.FetchPoolsForToken(testTokenAddr)
	if err != nil {
		t.Fatalf("FetchPoolsForToken failed: %v", err)
	}

	// Should only include the Solana pair, not the Ethereum one
	if len(pools) != 1 {
		t.Fatalf("expected 1 Solana pool, got %d", len(pools))
	}
	if pools[0].ChainID != "solana" {
		t.Errorf("expected chainId 'solana', got %q", pools[0].ChainID)
	}
	if pools[0].DexID != "raydium" {
		t.Errorf("expected dexId 'raydium', got %q", pools[0].DexID)
	}
}

func TestDexScreenerFetchPoolsCacheHit(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Write([]byte(dexScreenerResponse("solana")))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")

	_, err := client.FetchPoolsForToken(testTokenAddr)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	_, err = client.FetchPoolsForToken(testTokenAddr)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call (cached), got %d", callCount.Load())
	}
}

func TestDexScreenerFetchWithRetryRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(dexScreenerResponse("solana")))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	resp, err := client.FetchWithRetry(testTokenAddr)
	if err != nil {
		t.Fatalf("FetchWithRetry failed: %v", err)
	}

	if len(resp.Pairs) == 0 {
		t.Fatal("expected pairs in response")
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestDexScreenerFetchWithRetryAllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	_, err := client.FetchWithRetry(testTokenAddr)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	if !errors.Is(err, models.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
}

func TestDexScreenerFetchPriceNoPairs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"pairs":[]}`))
	}))
	defer server.Close()

	client := dex.NewDexScreenerClientWithHTTP(server.Client(), server.URL+"/")
	_, err := client.FetchPrice(testTokenAddr)
	if err == nil {
		t.Fatal("expected error for empty pairs")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}
