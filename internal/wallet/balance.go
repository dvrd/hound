package wallet

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
)

// PriceFetcher is an interface for fetching token prices.
// This decouples the balance fetcher from the specific DEX/pricing implementation.
type PriceFetcher interface {
	FetchPrice(token models.Token) (models.PriceData, error)
	// FetchMultiplePrices fetches prices for multiple tokens concurrently (best-effort).
	FetchMultiplePrices(tokens []models.Token) map[string]models.PriceData
}

// TokenMetadata holds minimal metadata for an unknown token resolved from an external source.
type TokenMetadata struct {
	Symbol   string
	Name     string
	Decimals int
}

// MetadataFetcher resolves token metadata (symbol, name, decimals) by mint address.
// Used as a fallback when the token is not in the local DB.
type MetadataFetcher interface {
	LookupTokenMetadata(mintAddr string) (TokenMetadata, error)
}

// BalanceFetcher fetches on-chain balances and assembles portfolios.
type BalanceFetcher struct {
	rpcClient       *blockchain.RPCClient
	priceFetcher    PriceFetcher
	metadataFetcher MetadataFetcher
	db              *database.Database
}

// NewBalanceFetcher creates a new BalanceFetcher.
func NewBalanceFetcher(rpcClient *blockchain.RPCClient, priceFetcher PriceFetcher, db *database.Database) *BalanceFetcher {
	return &BalanceFetcher{
		rpcClient:    rpcClient,
		priceFetcher: priceFetcher,
		db:           db,
	}
}

// WithMetadataFetcher attaches a metadata fetcher used to resolve unknown tokens.
func (f *BalanceFetcher) WithMetadataFetcher(mf MetadataFetcher) *BalanceFetcher {
	f.metadataFetcher = mf
	return f
}

// FetchPortfolioBalance fetches the complete portfolio for a wallet address.
func (f *BalanceFetcher) FetchPortfolioBalance(address string) (models.PortfolioBalance, error) {
	// 1. Get SOL balance in lamports
	lamports, err := blockchain.GetBalance(context.Background(), f.rpcClient, address)
	if err != nil {
		return models.PortfolioBalance{}, fmt.Errorf("fetching SOL balance: %w", err)
	}

	// 2. Convert lamports to SOL
	solAmount := float64(lamports) / 1_000_000_000.0

	// 3. Get SOL price (best-effort, default to 0)
	solPrice, _ := blockchain.GetSOLPriceCached(context.Background())

	solUSDValue := solAmount * solPrice
	totalUSD := solUSDValue

	// 4. Build SOL TokenBalance
	solBalance := models.TokenBalance{
		Mint:     blockchain.SOLMint,
		Symbol:   "SOL",
		Name:     "Solana",
		Amount:   solAmount,
		Decimals: 9,
		USDPrice: solPrice,
		USDValue: solUSDValue,
	}

	// 5. Get token accounts
	tokenAccounts, err := blockchain.GetTokenAccountsByOwner(context.Background(), f.rpcClient, address)
	if err != nil {
		return models.PortfolioBalance{}, fmt.Errorf("fetching token accounts: %w", err)
	}

	// 6. Process each token account — resolve metadata, then batch-fetch prices.
	type pendingToken struct {
		ta       blockchain.TokenAccount
		token    models.Token
		symbol   string
		name     string
		decimals int
		hasToken bool // true if found in DB (eligible for price fetch)
	}

	var pending []pendingToken
	for _, ta := range tokenAccounts {
		if ta.Amount == 0 {
			continue
		}

		pt := pendingToken{ta: ta, decimals: ta.Decimals}

		token, err := f.db.GetTokenByContractAddress(ta.Mint)
		if err == nil {
			pt.symbol = token.Symbol
			pt.name = token.Name
			pt.decimals = models.GetTokenDecimals(token)
			pt.token = token
			pt.hasToken = true
		} else if errors.Is(err, models.ErrTokenNotFound) {
			// Unknown token: try to resolve metadata from external source (e.g. Jupiter).
			if f.metadataFetcher != nil {
				if meta, metaErr := f.metadataFetcher.LookupTokenMetadata(ta.Mint); metaErr == nil {
					pt.symbol = meta.Symbol
					pt.name = meta.Name
					if meta.Decimals > 0 {
						pt.decimals = meta.Decimals
					}
					// Cache in DB so subsequent fetches skip the network call.
					_ = f.db.InsertToken(models.Token{
						Symbol:          pt.symbol,
						Name:            pt.name,
						ContractAddress: ta.Mint,
						Chain:           "solana",
						Decimals:        pt.decimals,
					})
				}
			}
			// Fall back to truncated mint if metadata is still unknown.
			if pt.symbol == "" {
				if len(ta.Mint) >= 8 {
					pt.symbol = ta.Mint[:4] + "..." + ta.Mint[len(ta.Mint)-4:]
				} else {
					pt.symbol = ta.Mint
				}
			}
		} else {
			// Unexpected DB error, skip this token
			continue
		}

		pending = append(pending, pt)
	}

	// H2: Batch-fetch prices for all known tokens in one concurrent call.
	var priceMap map[string]models.PriceData
	if f.priceFetcher != nil && len(pending) > 0 {
		var tokensToPrice []models.Token
		for _, pt := range pending {
			if pt.hasToken {
				tokensToPrice = append(tokensToPrice, pt.token)
			}
		}
		if len(tokensToPrice) > 0 {
			priceMap = f.priceFetcher.FetchMultiplePrices(tokensToPrice)
		}
	}

	var tokenBalances []models.TokenBalance
	for _, pt := range pending {
		var usdPrice, change24h float64
		if priceMap != nil {
			if pd, ok := priceMap[pt.symbol]; ok {
				usdPrice = pd.PriceUSD
				change24h = pd.Change24h
			}
		}

		amount := pt.ta.UIAmount
		usdValue := amount * usdPrice
		totalUSD += usdValue

		tokenBalances = append(tokenBalances, models.TokenBalance{
			Mint:      pt.ta.Mint,
			Symbol:    pt.symbol,
			Name:      pt.name,
			Amount:    amount,
			Decimals:  pt.decimals,
			USDPrice:  usdPrice,
			USDValue:  usdValue,
			Change24h: change24h,
		})
	}

	return models.PortfolioBalance{
		WalletAddress: address,
		SOLBalance:    solBalance,
		TokenBalances: tokenBalances,
		TotalUSD:      totalUSD,
	}, nil
}

// FormatBalance formats a balance with smart precision.
// >=1000: 2 decimals, >=1: 4 decimals, >=0.01: 6 decimals, <0.01: 8 decimals
func FormatBalance(amount float64) string {
	switch {
	case amount >= 1000:
		return fmt.Sprintf("%.2f", amount)
	case amount >= 1:
		return fmt.Sprintf("%.4f", amount)
	case amount >= 0.01:
		return fmt.Sprintf("%.6f", amount)
	case amount == 0:
		return "0.00"
	default:
		return fmt.Sprintf("%.8f", amount)
	}
}

// FormatPrice formats a USD price with smart precision.
// >=$1: 2 decimals, $0.01-$1: 4 decimals, <$0.01: 6 decimals
func FormatPrice(price float64) string {
	switch {
	case price >= 1:
		return fmt.Sprintf("$%.2f", price)
	case price >= 0.01:
		return fmt.Sprintf("$%.4f", price)
	case price == 0:
		return "$0.00"
	default:
		return fmt.Sprintf("$%.6f", price)
	}
}

// FormatLargeNumber formats large numbers with suffixes.
// >=1B: "$1.23B", >=1M: "$12.34M", >=1K: "$45.67K", <1K: "$123.45"
func FormatLargeNumber(n float64) string {
	abs := math.Abs(n)
	switch {
	case abs >= 1e9:
		return fmt.Sprintf("$%.2fB", n/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("$%.2fM", n/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("$%.2fK", n/1e3)
	default:
		return fmt.Sprintf("$%.2f", n)
	}
}

// FormatChange24h formats a 24h change percentage with sign.
func FormatChange24h(change float64) string {
	if change > 0 {
		return fmt.Sprintf("+%.2f%%", change)
	}
	if change < 0 {
		return fmt.Sprintf("%.2f%%", change)
	}
	return "0.00%"
}
