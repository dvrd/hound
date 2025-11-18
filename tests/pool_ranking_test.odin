#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "../src"

// =============================================================================
// POOL RANKING TESTS - Phase 5.2
// =============================================================================
// These tests validate the pool ranking algorithm including:
// - Liquidity-weighted scoring formula
// - Pool filtering (min liquidity, max fees)
// - Ranking by score (descending order)
// - Best pool selection
// - Fee estimation logic
// - Pool type inference
//
// Test Philosophy:
// - Pure algorithmic testing (no network calls)
// - Test edge cases (empty pools, identical scores, extreme values)
// - Validate filtering thresholds
// - Follow Odin test patterns
//
// Coverage:
// 1. Scoring formula: (0.8 × liquidity) - (0.2 × fee × 10000)
// 2. Filtering: min $1K liquidity, max 1% fee
// 3. Ranking: highest score first
// 4. Fee estimation: DEX-specific defaults
// 5. Pool type inference: labels → pool_type mapping
// =============================================================================

@(test)
test_calculate_pool_score_high_liquidity :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test scoring formula for high liquidity pool
	// Formula: score = (0.8 × 500,000) - (0.2 × 0.25 × 10,000)
	//                = 400,000 - 500
	//                = 399,500

	pair := src.DexScreenerPair{
		liquidity = src.DexScreenerLiquidity{usd = 500_000},
		dexId     = "raydium",
		labels    = []string{},
	}

	score := src.calculate_pool_score(pair)

	// Expected: 399,500 (approximately, due to floating point)
	testing.expect(t, score >= 399_000 && score <= 400_000,
		fmt.tprintf("High liquidity pool score should be ~399,500, got %.2f", score))
}

@(test)
test_calculate_pool_score_low_liquidity_low_fee :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that low fee doesn't overcome low liquidity
	// Pool A: $100K liquidity, 0.01% fee → score = 80,000 - 20 = 79,980
	// Pool B: $500K liquidity, 0.25% fee → score = 400,000 - 500 = 399,500
	// Result: Pool B (high liquidity) wins despite higher fees

	pool_a := src.DexScreenerPair{
		liquidity = src.DexScreenerLiquidity{usd = 100_000},
		dexId     = "raydium",
		labels    = []string{"CLMM"}, // 0.3% fee estimate
	}

	pool_b := src.DexScreenerPair{
		liquidity = src.DexScreenerLiquidity{usd = 500_000},
		dexId     = "raydium",
		labels    = []string{}, // 0.25% fee estimate
	}

	score_a := src.calculate_pool_score(pool_a)
	score_b := src.calculate_pool_score(pool_b)

	testing.expect(t, score_b > score_a,
		fmt.tprintf("High liquidity pool ($500K) should beat low liquidity pool ($100K), scores: %.2f vs %.2f",
			score_b, score_a))
}

@(test)
test_filter_pools_minimum_liquidity :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that pools below $1,000 liquidity are filtered out
	// MIN_LIQUIDITY_USD = 1000.0

	pools := []src.DexScreenerPair{
		{
			pairAddress = "PoolA",
			liquidity   = src.DexScreenerLiquidity{usd = 500}, // Below threshold
			dexId       = "raydium",
		},
		{
			pairAddress = "PoolB",
			liquidity   = src.DexScreenerLiquidity{usd = 5000}, // Above threshold
			dexId       = "raydium",
		},
	}

	filtered := src.filter_pools(pools)
	defer delete(filtered)

	testing.expect(t, len(filtered) == 1,
		fmt.tprintf("Should filter to 1 pool (above $1K), got %d", len(filtered)))
	testing.expect(t, filtered[0].pairAddress == "PoolB",
		"Remaining pool should be PoolB (above $1K liquidity)")
}

@(test)
test_filter_pools_maximum_fee :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that pools with fees > 1% are filtered out
	// MAX_FEE_PERCENT = 1.0
	// Note: Since API doesn't provide fees, we test with DEX types that have different estimates

	pools := []src.DexScreenerPair{
		{
			pairAddress = "PoolA",
			liquidity   = src.DexScreenerLiquidity{usd = 10_000},
			dexId       = "raydium",
			labels      = []string{}, // 0.25% estimated (should pass)
		},
		{
			pairAddress = "PoolB",
			liquidity   = src.DexScreenerLiquidity{usd = 10_000},
			dexId       = "unknown_dex", // DEFAULT_FEE_PERCENT = 0.3% (should pass)
			labels      = []string{},
		},
	}

	filtered := src.filter_pools(pools)
	defer delete(filtered)

	// Both pools should pass (no DEX has >1% default fee)
	testing.expect(t, len(filtered) == 2,
		fmt.tprintf("Both pools should pass fee filter, got %d", len(filtered)))
}

@(test)
test_rank_pools_descending_order :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that pools are ranked by score descending (highest first)

	pools := []src.DexScreenerPair{
		{
			pairAddress = "PoolLow",
			liquidity   = src.DexScreenerLiquidity{usd = 50_000},
			dexId       = "raydium",
		},
		{
			pairAddress = "PoolHigh",
			liquidity   = src.DexScreenerLiquidity{usd = 500_000},
			dexId       = "raydium",
		},
		{
			pairAddress = "PoolMedium",
			liquidity   = src.DexScreenerLiquidity{usd = 200_000},
			dexId       = "raydium",
		},
	}

	ranked := src.rank_pools(pools)
	defer delete(ranked)

	testing.expect(t, len(ranked) == 3, "Should rank all 3 pools")
	testing.expect(t, ranked[0].pair.pairAddress == "PoolHigh",
		"First pool should be PoolHigh (highest liquidity)")
	testing.expect(t, ranked[1].pair.pairAddress == "PoolMedium",
		"Second pool should be PoolMedium")
	testing.expect(t, ranked[2].pair.pairAddress == "PoolLow",
		"Third pool should be PoolLow (lowest liquidity)")
}

@(test)
test_rank_pools_empty_input :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test ranking with empty input returns empty array

	pools := []src.DexScreenerPair{}

	ranked := src.rank_pools(pools)
	defer delete(ranked)

	testing.expect(t, len(ranked) == 0, "Ranking empty array should return empty array")
}

@(test)
test_select_best_pool_success :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test selecting best pool from valid candidates

	pools := []src.DexScreenerPair{
		{
			pairAddress = "PoolBest",
			liquidity   = src.DexScreenerLiquidity{usd = 500_000},
			dexId       = "raydium",
		},
		{
			pairAddress = "PoolOkay",
			liquidity   = src.DexScreenerLiquidity{usd = 100_000},
			dexId       = "orca",
		},
	}

	best_pool, found := src.select_best_pool(pools)

	testing.expect(t, found == true, "Should find best pool")
	testing.expect(t, best_pool.pairAddress == "PoolBest",
		"Best pool should be the one with highest liquidity")
}

@(test)
test_select_best_pool_empty_input :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that empty input returns false

	pools := []src.DexScreenerPair{}

	_, found := src.select_best_pool(pools)

	testing.expect(t, found == false, "Empty input should return false")
}

@(test)
test_select_best_pool_all_filtered_out :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that pools below threshold return false

	pools := []src.DexScreenerPair{
		{
			pairAddress = "PoolTooSmall",
			liquidity   = src.DexScreenerLiquidity{usd = 100}, // Below $1K threshold
			dexId       = "raydium",
		},
	}

	_, found := src.select_best_pool(pools)

	testing.expect(t, found == false,
		"Should return false when all pools filtered out (below $1K)")
}

@(test)
test_estimate_pool_fee_raydium_amm :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test fee estimation for Raydium AMM (no CLMM label)
	// Expected: 0.25%

	pair := src.DexScreenerPair{
		dexId  = "raydium",
		labels = []string{}, // No CLMM label
	}

	fee := src.estimate_pool_fee(pair)

	testing.expect(t, fee == 0.25,
		fmt.tprintf("Raydium AMM fee should be 0.25%%, got %.2f%%", fee))
}

@(test)
test_estimate_pool_fee_raydium_clmm :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test fee estimation for Raydium CLMM
	// Expected: 0.30% (common tier)

	pair := src.DexScreenerPair{
		dexId  = "raydium",
		labels = []string{"CLMM"},
	}

	fee := src.estimate_pool_fee(pair)

	testing.expect(t, fee == 0.3,
		fmt.tprintf("Raydium CLMM fee should be 0.30%%, got %.2f%%", fee))
}

@(test)
test_estimate_pool_fee_orca_whirlpool :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test fee estimation for Orca Whirlpool
	// Expected: 0.30%

	pair := src.DexScreenerPair{
		dexId  = "orca",
		labels = []string{"wp"},
	}

	fee := src.estimate_pool_fee(pair)

	testing.expect(t, fee == 0.3,
		fmt.tprintf("Orca Whirlpool fee should be 0.30%%, got %.2f%%", fee))
}

@(test)
test_infer_pool_type_clmm :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test pool type inference for CLMM pools

	pair := src.DexScreenerPair{
		labels = []string{"CLMM"},
	}

	pool_type := src.infer_pool_type(pair)

	testing.expect(t, pool_type == "clmm",
		fmt.tprintf("CLMM label should map to 'clmm', got '%s'", pool_type))
}

@(test)
test_infer_pool_type_whirlpool :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test pool type inference for Whirlpool

	pair := src.DexScreenerPair{
		labels = []string{"wp"},
	}

	pool_type := src.infer_pool_type(pair)

	testing.expect(t, pool_type == "whirlpool",
		fmt.tprintf("wp label should map to 'whirlpool', got '%s'", pool_type))
}

@(test)
test_infer_pool_type_default_amm :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test pool type inference defaults to AMM

	pair := src.DexScreenerPair{
		labels = []string{}, // No special labels
	}

	pool_type := src.infer_pool_type(pair)

	testing.expect(t, pool_type == "amm",
		fmt.tprintf("No labels should default to 'amm', got '%s'", pool_type))
}

@(test)
test_pair_to_pool_info_conversion :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test conversion from DexScreenerPair to PoolInfo

	pair := src.DexScreenerPair{
		dexId       = "raydium",
		pairAddress = "PoolAddr123",
		quoteToken  = src.DexScreenerToken{symbol = "SOL"},
		labels      = []string{"CLMM"},
	}

	pool_info := src.pair_to_pool_info(pair)

	testing.expect(t, pool_info.dex == "raydium", "DEX should be raydium")
	testing.expect(t, pool_info.pool_address == "PoolAddr123", "Pool address should match")
	testing.expect(t, pool_info.quote_token == "SOL", "Quote token should be SOL")
	testing.expect(t, pool_info.pool_type == "clmm", "Pool type should be clmm")
}
