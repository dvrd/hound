package services

import (
	"fmt"
	"math"
	"strconv"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

// TokenInfoService fetches extended token information from multiple sources.
type TokenInfoService struct {
	dexscreener *dex.DexScreenerClient
	rpcClient   *blockchain.RPCClient
}

// NewTokenInfoService creates a new TokenInfoService.
func NewTokenInfoService(dexscreener *dex.DexScreenerClient, rpcClient *blockchain.RPCClient) *TokenInfoService {
	return &TokenInfoService{
		dexscreener: dexscreener,
		rpcClient:   rpcClient,
	}
}

// FetchExtendedTokenInfo fetches comprehensive token data from DexScreener and on-chain sources.
// mintOrSymbol can be a mint address or a token symbol (resolved via DB).
func (s *TokenInfoService) FetchExtendedTokenInfo(mintOrSymbol string, db *database.Database) (models.TokenExtendedInfo, error) {
	// Resolve mint address
	mint := mintOrSymbol
	var dbToken *models.Token

	// Try looking up as symbol first
	if db != nil {
		token, err := db.GetTokenBySymbol(mintOrSymbol)
		if err == nil {
			mint = token.ContractAddress
			dbToken = &token
		} else {
			// Try as contract address
			token, err = db.GetTokenByContractAddress(mintOrSymbol)
			if err == nil {
				dbToken = &token
			}
			// If both fail, use mintOrSymbol as-is (assume it's a mint address)
		}
	}

	// Fetch DexScreener data
	pairs, err := s.dexscreener.FetchPoolsForToken(mint)
	if err != nil {
		return models.TokenExtendedInfo{}, fmt.Errorf("fetch extended info for %s: %w", mintOrSymbol, err)
	}

	if len(pairs) == 0 {
		return models.TokenExtendedInfo{}, fmt.Errorf("fetch extended info for %s: no pairs found: %w", mintOrSymbol, models.ErrTokenNotFound)
	}

	// Aggregate across pairs: best price, total volume, total liquidity
	var info models.TokenExtendedInfo
	var bestPrice float64
	var totalVolume float64
	var totalLiquidity float64
	var totalBuys, totalSells int

	symbolSet := make(map[string]bool)

	for _, pair := range pairs {
		priceUSD, _ := strconv.ParseFloat(pair.PriceUSD, 64)
		if priceUSD > bestPrice {
			bestPrice = priceUSD
		}
		totalVolume += pair.Volume.H24
		totalLiquidity += pair.Liquidity.USD
		totalBuys += pair.Txns.H24.Buys
		totalSells += pair.Txns.H24.Sells

		if pair.BaseToken.Symbol != "" {
			symbolSet[pair.BaseToken.Symbol] = true
		}
	}

	// Use first pair for metadata
	first := pairs[0]
	info.Name = first.BaseToken.Name
	info.Network = first.ChainID
	info.MintAddress = first.BaseToken.Address
	info.PriceUSD = bestPrice
	info.FDV = first.FDV
	info.MarketCap = first.MarketCap
	info.LiquidityUSD = totalLiquidity
	info.Volume24h = totalVolume
	info.Txns24h = totalBuys + totalSells
	info.Buys24h = totalBuys
	info.Sells24h = totalSells
	info.Decimals = first.BaseToken.Decimals
	info.PriceChange = models.PriceChanges{
		M5:  first.PriceChange.M5,
		H1:  first.PriceChange.H1,
		H6:  first.PriceChange.H6,
		H24: first.PriceChange.H24,
	}
	info.CreatedAt = first.PairCreatedAt
	info.IsActive = totalLiquidity > 0

	// Collect symbols
	for sym := range symbolSet {
		info.Symbols = append(info.Symbols, sym)
	}

	// Override with DB data if available
	if dbToken != nil {
		info.Decimals = dbToken.Decimals
	}

	// Fetch on-chain supply
	if s.rpcClient != nil {
		totalSupply, decimals, err := blockchain.GetTokenSupply(s.rpcClient, mint)
		if err == nil {
			if decimals > 0 {
				info.TotalSupply = float64(totalSupply) / math.Pow(10, float64(decimals))
			} else {
				info.TotalSupply = float64(totalSupply)
			}
			if info.Decimals == 0 {
				info.Decimals = decimals
			}
		}

		// Fetch top holders
		holders, err := blockchain.GetTokenLargestAccounts(s.rpcClient, mint)
		if err == nil && info.TotalSupply > 0 {
			for _, h := range holders {
				if h.UIAmount <= 0 {
					continue
				}
				pct := (h.UIAmount / info.TotalSupply) * 100
				info.TopHolders = append(info.TopHolders, models.TopHolder{
					Address:      h.Address,
					Balance:      h.UIAmount,
					OwnershipPct: pct,
				})
			}
		}
	}

	return info, nil
}
