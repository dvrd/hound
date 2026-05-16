package dex

import (
	"context"
	"fmt"
	"strings"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/models"
)

// DexType identifies a DEX protocol.
type DexType int

const (
	DexUnknown DexType = iota
	DexOrcaWhirlpool
	DexRaydiumCLMM
	DexRaydiumAMMV4
	DexMeteoraDLMM
	DexJupiterAPI
)

// String returns the human-readable name of a DexType.
func (d DexType) String() string {
	switch d {
	case DexOrcaWhirlpool:
		return "Orca Whirlpool"
	case DexRaydiumCLMM:
		return "Raydium CLMM"
	case DexRaydiumAMMV4:
		return "Raydium AMM V4"
	case DexMeteoraDLMM:
		return "Meteora DLMM"
	case DexJupiterAPI:
		return "Jupiter API"
	default:
		return "Unknown"
	}
}

// ParseDexType converts a dex/pool_type string pair to DexType.
func ParseDexType(dex, poolType string) DexType {
	dexLower := strings.ToLower(dex)
	poolLower := strings.ToLower(poolType)

	switch {
	case dexLower == "orca" && poolLower == "whirlpool":
		return DexOrcaWhirlpool
	case dexLower == "raydium" && poolLower == "clmm":
		return DexRaydiumCLMM
	case dexLower == "raydium" && poolLower == "amm_v4":
		return DexRaydiumAMMV4
	case dexLower == "meteora" && poolLower == "dlmm":
		return DexMeteoraDLMM
	default:
		return DexUnknown
	}
}

// Router routes price queries through multiple DEX sources.
type Router struct {
	rpcClient     *blockchain.RPCClient
	jupiterClient *JupiterClient
	getSOLPrice   func(context.Context) (float64, error)
}

// NewRouter creates a new price router.
func NewRouter(rpcClient *blockchain.RPCClient, jupiterClient *JupiterClient) *Router {
	return &Router{
		rpcClient:     rpcClient,
		jupiterClient: jupiterClient,
		getSOLPrice:   blockchain.GetSOLPriceCached,
	}
}

// NewRouterWithSOLPrice creates a router with a custom SOL price function (for testing).
func NewRouterWithSOLPrice(rpcClient *blockchain.RPCClient, jupiterClient *JupiterClient, getSOLPrice func(context.Context) (float64, error)) *Router {
	return &Router{
		rpcClient:     rpcClient,
		jupiterClient: jupiterClient,
		getSOLPrice:   getSOLPrice,
	}
}

// FetchPrice fetches the price for a token, trying pools in priority order.
// Falls back to Jupiter API if all pool-based methods fail.
func (r *Router) FetchPrice(token models.Token) (models.PriceData, error) {
	// TODO: implement pool-based price decoders (Orca Whirlpool, Raydium CLMM/AMM, Meteora DLMM).
	// For now, all pool-based methods are unimplemented — fall through to Jupiter API.

	// Fallback: Jupiter API
	priceData, err := r.jupiterClient.FetchPrice(token.ContractAddress)
	if err != nil {
		return models.PriceData{}, fmt.Errorf("router: all price sources failed for %s: %w", token.Symbol, err)
	}

	return priceData, nil
}

// QuoteToUSD converts a quote token price to USD.
// "sol"/"wsol" → multiply by SOL/USD price
// "usdc"/"usdt" → treat as $1.00
func (r *Router) QuoteToUSD(quoteToken string, priceInQuote float64) (float64, error) {
	switch strings.ToLower(quoteToken) {
	case "sol", "wsol":
		solPrice, err := r.getSOLPrice(context.Background())
		if err != nil {
			return 0, fmt.Errorf("quote to USD: %w", err)
		}
		return priceInQuote * solPrice, nil
	case "usdc", "usdt":
		return priceInQuote, nil
	default:
		return 0, fmt.Errorf("unknown quote token %q: %w", quoteToken, models.ErrTokenNotFound)
	}
}
