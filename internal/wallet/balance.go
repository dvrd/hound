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
}

// BalanceFetcher fetches on-chain balances and assembles portfolios.
type BalanceFetcher struct {
	rpcClient    *blockchain.RPCClient
	priceFetcher PriceFetcher
	db           *database.Database
}

// NewBalanceFetcher creates a new BalanceFetcher.
func NewBalanceFetcher(rpcClient *blockchain.RPCClient, priceFetcher PriceFetcher, db *database.Database) *BalanceFetcher {
	return &BalanceFetcher{
		rpcClient:    rpcClient,
		priceFetcher: priceFetcher,
		db:           db,
	}
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

	// 6. Process each token account
	var tokenBalances []models.TokenBalance
	for _, ta := range tokenAccounts {
		if ta.Amount == 0 {
			continue
		}

		var symbol string
		var usdPrice float64
		var change24h float64
		decimals := ta.Decimals

		// Try to look up token in DB
		token, err := f.db.GetTokenByContractAddress(ta.Mint)
		if err == nil {
			symbol = token.Symbol
			decimals = models.GetTokenDecimals(token)

			// Fetch price via price fetcher
			if f.priceFetcher != nil {
				priceData, priceErr := f.priceFetcher.FetchPrice(token)
				if priceErr == nil {
					usdPrice = priceData.PriceUSD
					change24h = priceData.Change24h
				}
			}
		} else if errors.Is(err, models.ErrTokenNotFound) {
			// Unknown token: use truncated mint as symbol
			if len(ta.Mint) >= 8 {
				symbol = ta.Mint[:4] + "..." + ta.Mint[len(ta.Mint)-4:]
			} else {
				symbol = ta.Mint
			}
		} else {
			// Unexpected DB error, skip this token
			continue
		}

		amount := ta.UIAmount
		usdValue := amount * usdPrice
		totalUSD += usdValue

		tokenBalances = append(tokenBalances, models.TokenBalance{
			Mint:      ta.Mint,
			Symbol:    symbol,
			Amount:    amount,
			Decimals:  decimals,
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
