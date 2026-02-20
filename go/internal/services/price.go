package services

import (
	"fmt"
	"sync"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

// PriceService fetches token prices with fallback across multiple sources.
// It implements the wallet.PriceFetcher interface.
type PriceService struct {
	router      *dex.Router
	dexscreener *dex.DexScreenerClient
	jupiter     *dex.JupiterClient
}

// NewPriceService creates a new PriceService with the given DEX clients.
func NewPriceService(router *dex.Router, dexscreener *dex.DexScreenerClient, jupiter *dex.JupiterClient) *PriceService {
	return &PriceService{
		router:      router,
		dexscreener: dexscreener,
		jupiter:     jupiter,
	}
}

// FetchPrice fetches the price for a token, trying multiple sources in order.
// If the token has pools, it tries the router first.
// Falls back to DexScreener, then Jupiter.
func (s *PriceService) FetchPrice(token models.Token) (models.PriceData, error) {
	// If token has pools, try router first
	if len(token.Pools) > 0 && s.router != nil {
		price, err := s.router.FetchPrice(token)
		if err == nil {
			return price, nil
		}
	}

	// Fallback: DexScreener
	if s.dexscreener != nil && token.ContractAddress != "" {
		price, err := s.dexscreener.FetchPrice(token.ContractAddress)
		if err == nil {
			return price, nil
		}
	}

	// Fallback: Jupiter
	if s.jupiter != nil && token.ContractAddress != "" {
		price, err := s.jupiter.FetchPrice(token.ContractAddress)
		if err == nil {
			return price, nil
		}
	}

	return models.PriceData{}, fmt.Errorf("all price sources failed for %s: %w", token.Symbol, models.ErrTokenNotFound)
}

// FetchMultiplePrices fetches prices for multiple tokens concurrently.
// Best-effort: continues on individual failures and returns partial results.
func (s *PriceService) FetchMultiplePrices(tokens []models.Token) map[string]models.PriceData {
	results := make(map[string]models.PriceData)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, token := range tokens {
		wg.Add(1)
		go func(t models.Token) {
			defer wg.Done()
			price, err := s.FetchPrice(t)
			if err == nil {
				mu.Lock()
				results[t.Symbol] = price
				mu.Unlock()
			}
		}(token)
	}

	wg.Wait()
	return results
}
