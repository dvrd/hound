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
func cacheKey(inputMint, outputMint, amount string) string {
	return inputMint + ":" + outputMint + ":" + amount
}

// GetQuote fetches a swap quote from Jupiter Ultra API.
// Results are cached for 90 seconds (QuoteTTL).
func (c *SwapClient) GetQuote(inputMint, outputMint, amount string, taker string) (models.SwapQuote, error) {
	key := cacheKey(inputMint, outputMint, amount)

	// Check cache
	c.cacheMu.RLock()
	if cached, ok := c.quoteCache[key]; ok {
		if time.Since(cached.fetchedAt) < models.QuoteTTL {
			c.cacheMu.RUnlock()
			return cached.quote, nil
		}
	}
	c.cacheMu.RUnlock()

	// Build request URL
	url := fmt.Sprintf("%s/ultra/v1/order?inputMint=%s&outputMint=%s&amount=%s&taker=%s",
		c.baseURL, inputMint, outputMint, amount, taker)

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
		RequestID   string `json:"requestId"`
		InputMint   string `json:"inputMint"`
		OutputMint  string `json:"outputMint"`
		InAmount    string `json:"inAmount"`
		OutAmount   string `json:"outAmount"`
		SwapMode    string `json:"swapMode"`
		SlippageBps int    `json:"slippageBps"`
		PriceImpact string `json:"priceImpactPct"`
		RoutePlan   []struct {
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
	inAmt, _ := strconv.ParseFloat(raw.InAmount, 64)
	outAmt, _ := strconv.ParseFloat(raw.OutAmount, 64)
	if inAmt > 0 {
		rate = outAmt / inAmt
	}

	priceImpact, _ := strconv.ParseFloat(raw.PriceImpact, 64)
	networkFee := float64(raw.PrioritizationFeeLamports) / 1_000_000_000.0

	quote := models.SwapQuote{
		InputMint:      raw.InputMint,
		OutputMint:     raw.OutputMint,
		InAmount:       raw.InAmount,
		OutAmount:      raw.OutAmount,
		Rate:           rate,
		SlippageBps:    raw.SlippageBps,
		PriceImpactPct: priceImpact,
		RoutePlan:      routePlan,
		NetworkFee:     networkFee,
		FetchedAt:      time.Now(),
		RawResponse:    json.RawMessage(body),
	}

	// Cache result
	c.cacheMu.Lock()
	c.quoteCache[key] = cachedQuote{quote: quote, fetchedAt: time.Now()}
	c.cacheMu.Unlock()

	return quote, nil
}
