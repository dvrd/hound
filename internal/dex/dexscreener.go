package dex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dvrd/hound/internal/models"
)

const (
	dexScreenerBaseURL = "https://api.dexscreener.com/latest/dex/tokens/"
	changeCacheTTL     = 5 * time.Minute
	poolCacheTTL       = 1 * time.Hour
	maxRetries         = 3
)

// priceCache is a thread-safe cache for price data with TTL.
type priceCache struct {
	mu      sync.Mutex
	entries map[string]priceCacheEntry
	ttl     time.Duration
}

type priceCacheEntry struct {
	data      models.PriceData
	fetchedAt time.Time
}

func newPriceCache(ttl time.Duration) *priceCache {
	return &priceCache{
		entries: make(map[string]priceCacheEntry),
		ttl:     ttl,
	}
}

func (c *priceCache) get(key string) (models.PriceData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.fetchedAt) > c.ttl {
		return models.PriceData{}, false
	}
	return entry.data, true
}

func (c *priceCache) set(key string, data models.PriceData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = priceCacheEntry{data: data, fetchedAt: time.Now()}
}

// poolCache is a thread-safe cache for pool data with TTL.
type poolCache struct {
	mu      sync.Mutex
	entries map[string]poolCacheEntry
	ttl     time.Duration
}

type poolCacheEntry struct {
	data      []models.PairData
	fetchedAt time.Time
}

func newPoolCache(ttl time.Duration) *poolCache {
	return &poolCache{
		entries: make(map[string]poolCacheEntry),
		ttl:     ttl,
	}
}

func (c *poolCache) get(key string) ([]models.PairData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.fetchedAt) > c.ttl {
		return nil, false
	}
	return entry.data, true
}

func (c *poolCache) set(key string, data []models.PairData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = poolCacheEntry{data: data, fetchedAt: time.Now()}
}

// DexScreenerClient fetches token prices and pool data from DexScreener.
type DexScreenerClient struct {
	httpClient  *http.Client
	baseURL     string
	changeCache *priceCache
	poolCache   *poolCache
}

// NewDexScreenerClient creates a new DexScreener API client.
func NewDexScreenerClient() *DexScreenerClient {
	return &DexScreenerClient{
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		baseURL:     dexScreenerBaseURL,
		changeCache: newPriceCache(changeCacheTTL),
		poolCache:   newPoolCache(poolCacheTTL),
	}
}

// NewDexScreenerClientWithHTTP creates a client with a custom HTTP client and base URL (for testing).
func NewDexScreenerClientWithHTTP(httpClient *http.Client, baseURL string) *DexScreenerClient {
	return &DexScreenerClient{
		httpClient:  httpClient,
		baseURL:     baseURL,
		changeCache: newPriceCache(changeCacheTTL),
		poolCache:   newPoolCache(poolCacheTTL),
	}
}

// FetchPrice fetches the current price and 24h change for a token.
func (c *DexScreenerClient) FetchPrice(contractAddr string) (models.PriceData, error) {
	// Check cache
	if cached, ok := c.changeCache.get(contractAddr); ok {
		return cached, nil
	}

	resp, err := c.FetchWithRetry(contractAddr)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("dexscreener fetch price: %w", err)
	}

	if len(resp.Pairs) == 0 {
		return models.PriceData{}, fmt.Errorf("dexscreener: no pairs found for %s: %w", contractAddr, models.ErrTokenNotFound)
	}

	pair := resp.Pairs[0]
	priceUSD, err := strconv.ParseFloat(pair.PriceUSD, 64)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("dexscreener: parse priceUsd %q: %w", pair.PriceUSD, models.ErrParseError)
	}

	data := models.PriceData{
		PriceUSD:  priceUSD,
		Change24h: pair.PriceChange.H24,
	}

	c.changeCache.set(contractAddr, data)
	return data, nil
}

// FetchPoolsForToken fetches all trading pairs for a token, filtered to Solana chain.
func (c *DexScreenerClient) FetchPoolsForToken(contractAddr string) ([]models.PairData, error) {
	// Check cache
	if cached, ok := c.poolCache.get(contractAddr); ok {
		return cached, nil
	}

	resp, err := c.FetchWithRetry(contractAddr)
	if err != nil {
		return nil, fmt.Errorf("dexscreener fetch pools: %w", err)
	}

	// Filter to Solana chain
	var solanaPairs []models.PairData
	for _, pair := range resp.Pairs {
		if pair.ChainID == "solana" {
			solanaPairs = append(solanaPairs, pair)
		}
	}

	c.poolCache.set(contractAddr, solanaPairs)
	return solanaPairs, nil
}

// FetchWithRetry retries on rate limit (max 3 attempts, exponential backoff 1s→2s→4s).
func (c *DexScreenerClient) FetchWithRetry(contractAddr string) (models.DexScreenerResponse, error) {
	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}

		resp, err := c.httpClient.Get(c.baseURL + contractAddr)
		if err != nil {
			lastErr = fmt.Errorf("dexscreener HTTP request: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("dexscreener read body: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("dexscreener rate limited (attempt %d/%d): %w",
				attempt+1, maxRetries, models.ErrRateLimited)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("dexscreener HTTP %d: %w", resp.StatusCode, models.ErrConnectionFailed)
			continue
		}

		var result models.DexScreenerResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return models.DexScreenerResponse{}, fmt.Errorf("dexscreener parse response: %w", models.ErrInvalidResponse)
		}

		return result, nil
	}

	return models.DexScreenerResponse{}, fmt.Errorf("dexscreener: all retries exhausted: %w", lastErr)
}
