package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// SOLMint is the mint address for native SOL (wrapped).
const SOLMint = "So11111111111111111111111111111111111111112"

const (
	solPriceTTL    = 30 * time.Second
	solPriceMinUSD = 1.0
	solPriceMaxUSD = 10000.0

	jupiterPriceURL   = "https://lite-api.jup.ag/price/v3?ids=" + SOLMint
	coingeckoPriceURL = "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd"
)

// solPriceCache holds the cached SOL/USD price.
var solPriceCache = struct {
	mu        sync.Mutex
	price     float64
	fetchedAt time.Time
}{}

// httpClientForOracle is the HTTP client used by the oracle.
// Exported via setter for testing.
var oracleHTTPClient = &http.Client{Timeout: 10 * time.Second}

// SetOracleHTTPClient replaces the HTTP client used by the oracle (for testing).
func SetOracleHTTPClient(c *http.Client) {
	oracleHTTPClient = c
}

// GetSOLPriceCached returns the cached SOL/USD price, refreshing if stale (30s TTL).
// Fallback chain: cache -> Jupiter Price API -> CoinGecko API.
// Validates price is in $1-$10000 range.
func GetSOLPriceCached(ctx context.Context) (float64, error) {
	solPriceCache.mu.Lock()
	defer solPriceCache.mu.Unlock()

	// Return cached value if fresh
	if solPriceCache.price > 0 && time.Since(solPriceCache.fetchedAt) < solPriceTTL {
		return solPriceCache.price, nil
	}

	// Try Jupiter first
	price, err := fetchSOLPriceJupiter(ctx)
	if err == nil {
		solPriceCache.price = price
		solPriceCache.fetchedAt = time.Now()
		return price, nil
	}

	// Fallback to CoinGecko
	price, err = fetchSOLPriceCoinGecko(ctx)
	if err == nil {
		solPriceCache.price = price
		solPriceCache.fetchedAt = time.Now()
		return price, nil
	}

	return 0, fmt.Errorf("all SOL price sources failed: %w", models.ErrOracleConnectionFailed)
}

// ResetSOLPriceCache clears the cached SOL price (for testing).
func ResetSOLPriceCache() {
	solPriceCache.mu.Lock()
	defer solPriceCache.mu.Unlock()
	solPriceCache.price = 0
	solPriceCache.fetchedAt = time.Time{}
}

// fetchSOLPriceJupiter fetches SOL/USD from Jupiter Price API v3.
func fetchSOLPriceJupiter(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jupiterPriceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("jupiter price create request: %w", err)
	}

	resp, err := oracleHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("jupiter price request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("jupiter price HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("jupiter price read body: %w", err)
	}

	var parsed struct {
		Data map[string]struct {
			Price string `json:"price"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("jupiter price parse: %w", err)
	}

	entry, ok := parsed.Data[SOLMint]
	if !ok {
		return 0, fmt.Errorf("jupiter price: SOL mint not in response")
	}

	price, err := strconv.ParseFloat(entry.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("jupiter price parse float: %w", err)
	}

	if err := validateSOLPrice(price); err != nil {
		return 0, err
	}

	return price, nil
}

// fetchSOLPriceCoinGecko fetches SOL/USD from CoinGecko API.
func fetchSOLPriceCoinGecko(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", coingeckoPriceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("coingecko price create request: %w", err)
	}

	resp, err := oracleHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("coingecko price request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("coingecko price HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("coingecko price read body: %w", err)
	}

	var parsed struct {
		Solana struct {
			USD float64 `json:"usd"`
		} `json:"solana"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("coingecko price parse: %w", err)
	}

	price := parsed.Solana.USD
	if err := validateSOLPrice(price); err != nil {
		return 0, err
	}

	return price, nil
}

// validateSOLPrice checks that the price is within a reasonable range.
func validateSOLPrice(price float64) error {
	if price < solPriceMinUSD || price > solPriceMaxUSD {
		return fmt.Errorf("SOL price $%.2f outside valid range [$%.0f-$%.0f]: %w",
			price, solPriceMinUSD, solPriceMaxUSD, models.ErrOraclePriceInvalid)
	}
	return nil
}
