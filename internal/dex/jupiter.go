package dex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dvrd/hound/internal/models"
)

const (
	jupiterPriceBaseURL   = "https://lite-api.jup.ag/price/v3?ids="
	jupiterTokenSearchURL = "https://lite-api.jup.ag/tokens/v2/search?query="
	jupiterPriceCacheTTL  = 60 * time.Second
)

// TokenMetadata holds metadata for a token from Jupiter.
type TokenMetadata struct {
	Address  string
	Symbol   string
	Name     string
	Decimals int
}

// JupiterClient fetches token prices from Jupiter.
type JupiterClient struct {
	httpClient    *http.Client
	priceBaseURL  string
	searchBaseURL string
	priceCache    *priceCache
}

// NewJupiterClient creates a new Jupiter API client.
func NewJupiterClient() *JupiterClient {
	return &JupiterClient{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		priceBaseURL:  jupiterPriceBaseURL,
		searchBaseURL: jupiterTokenSearchURL,
		priceCache:    newPriceCache(jupiterPriceCacheTTL),
	}
}

// NewJupiterClientWithHTTP creates a client with a custom HTTP client and base URLs (for testing).
func NewJupiterClientWithHTTP(httpClient *http.Client, priceBaseURL, searchBaseURL string) *JupiterClient {
	return &JupiterClient{
		httpClient:    httpClient,
		priceBaseURL:  priceBaseURL,
		searchBaseURL: searchBaseURL,
		priceCache:    newPriceCache(jupiterPriceCacheTTL),
	}
}

// FetchPrice fetches the price for a token mint.
func (c *JupiterClient) FetchPrice(mint string) (models.PriceData, error) {
	// Check cache
	if cached, ok := c.priceCache.get(mint); ok {
		return cached, nil
	}

	resp, err := c.httpClient.Get(c.priceBaseURL + mint)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("jupiter price request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.PriceData{}, fmt.Errorf("jupiter price HTTP %d: %w", resp.StatusCode, models.ErrConnectionFailed)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("jupiter price read body: %w", err)
	}

	var parsed struct {
		Data map[string]struct {
			Price          string  `json:"price"`
			PriceChange24h float64 `json:"priceChange24h"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return models.PriceData{}, fmt.Errorf("jupiter price parse: %w", models.ErrInvalidResponse)
	}

	entry, ok := parsed.Data[mint]
	if !ok {
		return models.PriceData{}, fmt.Errorf("jupiter: mint %s not in response: %w", mint, models.ErrTokenNotFound)
	}

	priceUSD, err := strconv.ParseFloat(entry.Price, 64)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("jupiter: parse price %q: %w", entry.Price, models.ErrParseError)
	}

	data := models.PriceData{
		PriceUSD:  priceUSD,
		Change24h: entry.PriceChange24h,
	}

	c.priceCache.set(mint, data)
	return data, nil
}

// LookupTokenMetadata looks up token metadata by mint address or symbol,
// returning only the first result.
func (c *JupiterClient) LookupTokenMetadata(mintAddr string) (TokenMetadata, error) {
	results, err := c.LookupTokenList(mintAddr)
	if err != nil {
		return TokenMetadata{}, err
	}
	return results[0], nil
}

// LookupTokenList searches Jupiter for tokens matching the query (address,
// symbol, or name) and returns all results up to the API limit.
func (c *JupiterClient) LookupTokenList(query string) ([]TokenMetadata, error) {
	resp, err := c.httpClient.Get(c.searchBaseURL + query)
	if err != nil {
		return nil, fmt.Errorf("jupiter token search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jupiter token search HTTP %d: %w", resp.StatusCode, models.ErrConnectionFailed)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jupiter token search read body: %w", err)
	}

	var raw []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Decimals int    `json:"decimals"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("jupiter token search parse: %w", models.ErrInvalidResponse)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("jupiter: token %q not found: %w", query, models.ErrTokenNotFound)
	}

	out := make([]TokenMetadata, len(raw))
	for i, r := range raw {
		out[i] = TokenMetadata{
			Address:  r.ID,
			Symbol:   r.Symbol,
			Name:     r.Name,
			Decimals: r.Decimals,
		}
	}
	return out, nil
}
