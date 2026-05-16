package swap

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
	defaultBaseURL = "https://lite-api.jup.ag"
)

// cachedQuote holds a quote with its fetch timestamp.
type cachedQuote struct {
	quote     models.SwapQuote
	fetchedAt time.Time
}

// SwapClient interacts with the Jupiter Ultra API for swap quotes.
type SwapClient struct {
	httpClient *http.Client
	baseURL    string
	quoteCache map[string]cachedQuote
	cacheMu    sync.RWMutex
}

// HTTPClient returns the underlying HTTP client used by this SwapClient.
// Callers may reuse it to avoid creating redundant clients for the same host.
func (c *SwapClient) HTTPClient() *http.Client {
	return c.httpClient
}

// NewSwapClient creates a new SwapClient with default settings.
func NewSwapClient() *SwapClient {
	return &SwapClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    defaultBaseURL,
		quoteCache: make(map[string]cachedQuote),
	}
}

// NewSwapClientWithHTTP creates a SwapClient with a custom HTTP client and base URL (for testing).
func NewSwapClientWithHTTP(httpClient *http.Client, baseURL string) *SwapClient {
	return &SwapClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		quoteCache: make(map[string]cachedQuote),
	}
}

// cacheKey builds the cache key for a quote.
func cacheKey(inputMint, outputMint, amount, taker string) string {
	return inputMint + ":" + outputMint + ":" + amount + ":" + taker
}

// GetQuote fetches a swap quote from Jupiter Ultra API.
// Results are cached for 90 seconds (QuoteTTL).
// slippageBps overrides Jupiter's automatic slippage when > 0 (manual mode).
// Pass 0 to use Jupiter's default adaptive slippage.
func (c *SwapClient) GetQuote(inputMint, outputMint, amount string, taker string, slippageBps int) (models.SwapQuote, error) {
	key := cacheKey(inputMint, outputMint, amount, taker)

	// Check cache
	c.cacheMu.RLock()
	if cached, ok := c.quoteCache[key]; ok {
		if time.Since(cached.fetchedAt) < models.QuoteTTL {
			c.cacheMu.RUnlock()
			return cached.quote, nil
		}
	}
	c.cacheMu.RUnlock()

	// Build request URL — append slippageBps only when user explicitly sets it (manual mode).
	url := fmt.Sprintf("%s/ultra/v1/order?inputMint=%s&outputMint=%s&amount=%s&taker=%s",
		c.baseURL, inputMint, outputMint, amount, taker)
	if slippageBps > 0 {
		url += fmt.Sprintf("&slippageBps=%d", slippageBps)
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.SwapQuote{}, fmt.Errorf("swap quote HTTP %d: %s: %w",
			resp.StatusCode, string(body), models.ErrConnectionFailed)
	}

	// Parse the Jupiter Ultra response
	var raw struct {
		RequestID            string `json:"requestId"`
		InputMint            string `json:"inputMint"`
		OutputMint           string `json:"outputMint"`
		InAmount             string `json:"inAmount"`
		OutAmount            string `json:"outAmount"`
		OtherAmountThreshold string `json:"otherAmountThreshold"`
		SwapMode             string `json:"swapMode"`
		SlippageBps          int    `json:"slippageBps"`
		PriceImpact          string `json:"priceImpactPct"`
		RoutePlan            []struct {
			SwapInfo struct {
				AmmKey     string `json:"ammKey"`
				Label      string `json:"label"`
				InputMint  string `json:"inputMint"`
				OutputMint string `json:"outputMint"`
				InAmount   string `json:"inAmount"`
				OutAmount  string `json:"outAmount"`
				FeeAmount  string `json:"feeAmount"`
				FeeMint    string `json:"feeMint"`
			} `json:"swapInfo"`
			Percent int `json:"percent"`
		} `json:"routePlan"`
		Transaction               string `json:"transaction"`
		PrioritizationFeeLamports int64  `json:"prioritizationFeeLamports"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote parse response: %w", models.ErrInvalidResponse)
	}

	// Build route plan
	var routePlan []models.RouteStep
	for _, rp := range raw.RoutePlan {
		routePlan = append(routePlan, models.RouteStep{
			DexLabel:   rp.SwapInfo.Label,
			InputMint:  rp.SwapInfo.InputMint,
			OutputMint: rp.SwapInfo.OutputMint,
			InAmount:   rp.SwapInfo.InAmount,
			OutAmount:  rp.SwapInfo.OutAmount,
			FeeAmount:  rp.SwapInfo.FeeAmount,
			FeeMint:    rp.SwapInfo.FeeMint,
			Percent:    rp.Percent,
		})
	}

	// Calculate rate
	var rate float64
	inAmt, err := strconv.ParseFloat(raw.InAmount, 64)
	if err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote: parse inAmount %q: %w", raw.InAmount, models.ErrInvalidResponse)
	}
	outAmt, err := strconv.ParseFloat(raw.OutAmount, 64)
	if err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote: parse outAmount %q: %w", raw.OutAmount, models.ErrInvalidResponse)
	}
	if inAmt > 0 {
		rate = outAmt / inAmt
	}

	priceImpact, err := strconv.ParseFloat(raw.PriceImpact, 64)
	if err != nil {
		return models.SwapQuote{}, fmt.Errorf("swap quote: parse priceImpact %q: %w", raw.PriceImpact, models.ErrInvalidResponse)
	}
	networkFee := float64(raw.PrioritizationFeeLamports) / 1_000_000_000.0

	// otherAmountThreshold is Jupiter's guaranteed minimum output (ExactIn mode).
	// It already accounts for slippage — this is what gets enforced on-chain.
	var minReceived float64
	if raw.OtherAmountThreshold != "" {
		if v, err := strconv.ParseFloat(raw.OtherAmountThreshold, 64); err == nil {
			minReceived = v
		}
	}

	quote := models.SwapQuote{
		InputMint:           raw.InputMint,
		OutputMint:          raw.OutputMint,
		InAmount:            raw.InAmount,
		OutAmount:           raw.OutAmount,
		Rate:                rate,
		SlippageBps:         raw.SlippageBps,
		PriceImpactPct:      priceImpact,
		RoutePlan:           routePlan,
		NetworkFee:          networkFee,
		MinReceived:         minReceived,
		FetchedAt:           time.Now(),
		TransactionPayload:  raw.Transaction,
		RequestID:           raw.RequestID,
	}

	// Cache result and evict stale entries (H6: prevent unbounded cache growth).
	c.cacheMu.Lock()
	c.quoteCache[key] = cachedQuote{quote: quote, fetchedAt: time.Now()}
	evictThreshold := models.QuoteTTL * 2
	for k, v := range c.quoteCache {
		if time.Since(v.fetchedAt) > evictThreshold {
			delete(c.quoteCache, k)
		}
	}
	c.cacheMu.Unlock()

	return quote, nil
}
