package dex

import (
	"fmt"

	"github.com/dvrd/hound/internal/models"
)

// Router routes price queries through Jupiter API.
type Router struct {
	jupiterClient *JupiterClient
}

// NewRouter creates a new price router.
func NewRouter(jupiterClient *JupiterClient) *Router {
	return &Router{
		jupiterClient: jupiterClient,
	}
}

// FetchPrice fetches the price for a token via Jupiter API.
func (r *Router) FetchPrice(token models.Token) (models.PriceData, error) {
	priceData, err := r.jupiterClient.FetchPrice(token.ContractAddress)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("router: all price sources failed for %s: %w", token.Symbol, err)
	}

	return priceData, nil
}
