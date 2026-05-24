package dex

import (
	"sort"
	"strings"

	"github.com/dvrd/hound/internal/models"
)

const (
	// minLiquidityUSD is the minimum liquidity threshold for pool filtering.
	minLiquidityUSD = 1000.0

	// maxFeePercent is the maximum fee percentage for pool filtering.
	maxFeePercent = 1.0
)

// FilterPools filters pairs to Solana chain with min $1K liquidity and max 1% fee.
func FilterPools(pairs []models.PairData) []models.PairData {
	var filtered []models.PairData
	for _, pair := range pairs {
		if pair.ChainID != "solana" {
			continue
		}
		if pair.Liquidity.USD < minLiquidityUSD {
			continue
		}
		// Estimate fee from labels or use a default check
		fee := estimateFee(pair)
		if fee > maxFeePercent {
			continue
		}
		filtered = append(filtered, pair)
	}
	return filtered
}

// RankPools sorts pools by score. Score = 0.8 * liquidity - 0.2 * fee * 10000.
// Returns a new sorted slice (highest score first).
func RankPools(pools []models.PairData) []models.PairData {
	ranked := make([]models.PairData, len(pools))
	copy(ranked, pools)

	sort.Slice(ranked, func(i, j int) bool {
		scoreI := poolScore(ranked[i])
		scoreJ := poolScore(ranked[j])
		return scoreI > scoreJ
	})

	return ranked
}

// poolScore calculates the ranking score for a pool.
func poolScore(pair models.PairData) float64 {
	fee := estimateFee(pair)
	return 0.8*pair.Liquidity.USD - 0.2*fee*10000
}

// estimateFee estimates the fee percentage for a pair based on its labels.
func estimateFee(pair models.PairData) float64 {
	for _, label := range pair.Labels {
		lower := strings.ToLower(label)
		if strings.Contains(lower, "0.01%") {
			return 0.01
		}
		if strings.Contains(lower, "0.05%") {
			return 0.05
		}
		if strings.Contains(lower, "0.3%") || strings.Contains(lower, "0.30%") {
			return 0.3
		}
		if strings.Contains(lower, "1%") {
			return 1.0
		}
	}
	// Default fee estimate based on DEX
	switch strings.ToLower(pair.DexID) {
	case "raydium":
		return 0.25
	case "orca":
		return 0.3
	default:
		return 0.3
	}
}

// PairToPoolInfo converts a DexScreener PairData to a models.PoolInfo.
func PairToPoolInfo(pair models.PairData) models.PoolInfo {
	quoteSymbol := strings.ToLower(pair.QuoteToken.Symbol)
	return models.PoolInfo{
		Dex:          pair.DexID,
		PoolAddress:  pair.PairAddress,
		QuoteToken:   quoteSymbol,
		PoolType:     inferPoolType(pair),
		LiquidityUSD: pair.Liquidity.USD,
		Volume24h:    pair.Volume.H24,
		FeePercent:   estimateFee(pair),
	}
}

// inferPoolType infers the pool type from the pair's DEX and labels.
func inferPoolType(pair models.PairData) string {
	dex := strings.ToLower(pair.DexID)
	for _, label := range pair.Labels {
		lower := strings.ToLower(label)
		if strings.Contains(lower, "whirlpool") {
			return "whirlpool"
		}
		if strings.Contains(lower, "clmm") {
			return "clmm"
		}
		if strings.Contains(lower, "dlmm") {
			return "dlmm"
		}
	}
	switch dex {
	case "orca":
		return "whirlpool"
	case "raydium":
		return "amm_v4"
	case "meteora":
		return "dlmm"
	default:
		return ""
	}
}
