package dex_test

import (
	"testing"

	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

func makePair(chainID, dexID string, liquidityUSD float64, labels []string) models.PairData {
	return models.PairData{
		ChainID:     chainID,
		DexID:       dexID,
		PairAddress: "pool_" + dexID,
		Labels:      labels,
		BaseToken:   models.PairToken{Address: "base", Symbol: "TOKEN", Name: "Token"},
		QuoteToken:  models.PairToken{Address: "quote", Symbol: "SOL", Name: "SOL"},
		PriceUSD:    "1.00",
		Liquidity:   models.PairLiquidity{USD: liquidityUSD},
		Volume:      models.PairVolume{H24: 50000},
	}
}

func TestFilterPoolsSolanaOnly(t *testing.T) {
	pairs := []models.PairData{
		makePair("solana", "raydium", 5000, nil),
		makePair("ethereum", "uniswap", 100000, nil),
		makePair("solana", "orca", 2000, nil),
	}

	filtered := dex.FilterPools(pairs)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 Solana pools, got %d", len(filtered))
	}
	for _, p := range filtered {
		if p.ChainID != "solana" {
			t.Errorf("expected chainId 'solana', got %q", p.ChainID)
		}
	}
}

func TestFilterPoolsMinLiquidity(t *testing.T) {
	pairs := []models.PairData{
		makePair("solana", "raydium", 5000, nil), // Above threshold
		makePair("solana", "orca", 500, nil),     // Below threshold
		makePair("solana", "meteora", 1000, nil), // Exactly at threshold
	}

	filtered := dex.FilterPools(pairs)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 pools above $1K liquidity, got %d", len(filtered))
	}
}

func TestFilterPoolsMaxFee(t *testing.T) {
	pairs := []models.PairData{
		makePair("solana", "raydium", 5000, nil),                // 0.25% default
		makePair("solana", "orca", 5000, []string{"1%"}),        // Exactly 1% - should pass
		makePair("solana", "unknown", 5000, []string{"5% fee"}), // Over 1% - default 0.3% (no match on "5%")
	}

	filtered := dex.FilterPools(pairs)
	// raydium (0.25%), orca (1.0%), unknown (0.3% default) - all pass
	if len(filtered) != 3 {
		t.Fatalf("expected 3 pools, got %d", len(filtered))
	}
}

func TestRankPoolsByScore(t *testing.T) {
	pools := []models.PairData{
		makePair("solana", "orca", 10000, nil),    // Score: 0.8*10000 - 0.2*0.3*10000 = 7400
		makePair("solana", "raydium", 50000, nil), // Score: 0.8*50000 - 0.2*0.25*10000 = 39500
		makePair("solana", "meteora", 30000, nil), // Score: 0.8*30000 - 0.2*0.3*10000 = 23400
	}

	ranked := dex.RankPools(pools)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked pools, got %d", len(ranked))
	}

	// Raydium should be first (highest liquidity)
	if ranked[0].DexID != "raydium" {
		t.Errorf("expected raydium first, got %s", ranked[0].DexID)
	}
	// Meteora second
	if ranked[1].DexID != "meteora" {
		t.Errorf("expected meteora second, got %s", ranked[1].DexID)
	}
	// Orca last
	if ranked[2].DexID != "orca" {
		t.Errorf("expected orca third, got %s", ranked[2].DexID)
	}
}

func TestRankPoolsEmpty(t *testing.T) {
	ranked := dex.RankPools(nil)
	if len(ranked) != 0 {
		t.Errorf("expected empty result, got %d", len(ranked))
	}
}

func TestPairToPoolInfo(t *testing.T) {
	pair := models.PairData{
		ChainID:     "solana",
		DexID:       "raydium",
		PairAddress: "pool123",
		Labels:      nil,
		QuoteToken:  models.PairToken{Symbol: "SOL"},
		Liquidity:   models.PairLiquidity{USD: 50000},
		Volume:      models.PairVolume{H24: 10000},
	}

	info := dex.PairToPoolInfo(pair)

	if info.Dex != "raydium" {
		t.Errorf("expected dex 'raydium', got %q", info.Dex)
	}
	if info.PoolAddress != "pool123" {
		t.Errorf("expected pool address 'pool123', got %q", info.PoolAddress)
	}
	if info.QuoteToken != "sol" {
		t.Errorf("expected quote token 'sol', got %q", info.QuoteToken)
	}
	if info.PoolType != "amm_v4" {
		t.Errorf("expected pool type 'amm_v4', got %q", info.PoolType)
	}
	if info.LiquidityUSD != 50000 {
		t.Errorf("expected liquidity 50000, got %f", info.LiquidityUSD)
	}
	if info.Volume24h != 10000 {
		t.Errorf("expected volume 10000, got %f", info.Volume24h)
	}
	if info.FeePercent != 0.25 {
		t.Errorf("expected fee 0.25, got %f", info.FeePercent)
	}
}

func TestPairToPoolInfoOrcaWhirlpool(t *testing.T) {
	pair := models.PairData{
		DexID:      "orca",
		QuoteToken: models.PairToken{Symbol: "USDC"},
	}

	info := dex.PairToPoolInfo(pair)
	if info.PoolType != "whirlpool" {
		t.Errorf("expected pool type 'whirlpool', got %q", info.PoolType)
	}
	if info.QuoteToken != "usdc" {
		t.Errorf("expected quote token 'usdc', got %q", info.QuoteToken)
	}
}

func TestPairToPoolInfoMeteoraLabel(t *testing.T) {
	pair := models.PairData{
		DexID:      "meteora",
		Labels:     []string{"DLMM"},
		QuoteToken: models.PairToken{Symbol: "SOL"},
	}

	info := dex.PairToPoolInfo(pair)
	if info.PoolType != "dlmm" {
		t.Errorf("expected pool type 'dlmm', got %q", info.PoolType)
	}
}

func TestFilterPoolsAllFiltered(t *testing.T) {
	pairs := []models.PairData{
		makePair("ethereum", "uniswap", 100000, nil), // Wrong chain
		makePair("solana", "raydium", 100, nil),      // Too low liquidity
	}

	filtered := dex.FilterPools(pairs)
	if len(filtered) != 0 {
		t.Errorf("expected 0 pools after filtering, got %d", len(filtered))
	}
}
