// =============================================================================
// POOL RANKING MODULE
// =============================================================================
// This module implements liquidity-based pool ranking for selecting optimal
// DEX pools for price fetching.
//
// Ranking Algorithm:
// - Simple weighted scoring: (0.8 × liquidity_usd) - (0.2 × fee_percent × 10000)
// - Liquidity dominates (80% weight) - deeper pools = better prices
// - Fees penalize (20% weight) - lower fees = less impact
//
// Filtering Criteria:
// - Minimum liquidity: $1,000 USD (prevents scam/illiquid pools)
// - Maximum fee: 1.0% (prevents exploitative pools)
//
// Design Decisions:
// - Simple scoring for MVP, multi-factor scoring can be added later
// - Liquidity is king: research shows 30-40% weight in major aggregators
// - Conservative thresholds: $1K min liquidity is low barrier, 1% max fee is generous
//
// References:
// - Pool ranking research: PRPs/ai_docs/pool_ranking.md
// - Jupiter Metis algorithm (liquidity-weighted)
// - 1inch Pathfinder (graph-based routing)
// - Uniswap Auto Router (multi-pool splitting)
// =============================================================================

package dex

import "core:log"
import "core:slice"
import "core:strconv"
import "core:time"
import "../models"
import memory "../memory"

// =============================================================================
// DATA STRUCTURES
// =============================================================================

// Pool with calculated score
PoolScore :: struct {
	pair:  DexScreenerPair, // Original pool data
	score: f64,              // Calculated ranking score
}

// =============================================================================
// CONFIGURATION CONSTANTS
// =============================================================================

// Scoring weights (simple formula for MVP)
LIQUIDITY_WEIGHT :: 0.8 // 80% weight on liquidity
FEE_WEIGHT :: 0.2       // 20% weight on fees

// Filtering thresholds
MIN_LIQUIDITY_USD :: 1000.0 // $1,000 minimum (prevents scam pools)
MAX_FEE_PERCENT :: 1.0      // 1.0% maximum fee (generous upper bound)

// Default fee assumption (when fee data unavailable)
// Raydium AMM uses 0.25%, CLMM uses 0.01-1.0%, Orca uses 0.01-1.0%
DEFAULT_FEE_PERCENT :: 0.3 // Assume 0.3% if not specified

// =============================================================================
// CORE RANKING FUNCTIONS
// =============================================================================

// Calculate score for a single pool using simple weighted formula
//
// Formula: score = (0.8 × liquidity_usd) - (0.2 × fee_percent × 10000)
//
// Rationale:
// - Liquidity dominates (80%) because deeper pools = lower slippage
// - Fees penalize (20%) but are secondary concern for price fetching
// - Fee multiplier (× 10000) normalizes percentage to comparable scale
//
// Examples:
// - Pool A: $500K liquidity, 0.25% fee → score = 400,000 - 500 = 399,500
// - Pool B: $100K liquidity, 0.01% fee → score = 80,000 - 20 = 79,980
// - Pool A wins (liquidity dominates)
calculate_pool_score :: proc(pair: DexScreenerPair) -> f64 {
	// ASSERTION 1: Liquidity must be non-negative
	assert(pair.liquidity.usd >= 0, "Liquidity cannot be negative")

	// Extract liquidity (guaranteed field in API response)
	liquidity_usd := pair.liquidity.usd

	// Extract or estimate fee percentage
	// DexScreener API doesn't always include fee data, so we use heuristics:
	// - Raydium AMM: 0.25% (fixed)
	// - Raydium CLMM: 0.01-1.0% (variable, often 0.3%)
	// - Orca Whirlpool: 0.01-1.0% (variable)
	// - Meteora DLMM: Dynamic fees
	//
	// Since API doesn't provide fee field, we use DEX-based heuristics
	fee_percent := estimate_pool_fee(pair)

	// ASSERTION 2: Fee must be reasonable (0% to 10%)
	assert(fee_percent >= 0 && fee_percent <= 10.0, "Fee outside reasonable range")

	// Calculate weighted score
	liquidity_score := LIQUIDITY_WEIGHT * liquidity_usd
	fee_penalty := FEE_WEIGHT * fee_percent * 10_000

	score := liquidity_score - fee_penalty

	// ASSERTION 3: Score should be reasonable (can be negative for very high fees)
	// No upper bound assertion since liquidity can be very large ($100M+ pools exist)

	return score
}

// Estimate pool fee based on DEX and pool type
//
// DexScreener doesn't provide fee in API response, so we use reasonable defaults:
// - Raydium standard: 0.25%
// - Raydium CLMM: 0.30% (common tier)
// - Orca Whirlpool: 0.30% (common tier)
// - Meteora DLMM: 0.25% (approximate)
// - Unknown: 0.30% (conservative default)
estimate_pool_fee :: proc(pair: DexScreenerPair) -> f64 {
	dex_id := pair.dexId

	// Check for CLMM/Whirlpool indicators in labels
	is_clmm := false
	is_whirlpool := false
	for label in pair.labels {
		if label == "CLMM" do is_clmm = true
		if label == "wp" do is_whirlpool = true
	}

	// DEX-specific fee estimation
	switch dex_id {
	case "raydium":
		if is_clmm {
			return 0.3 // Raydium CLMM common tier
		}
		return 0.25 // Raydium AMM V4 fixed fee

	case "orca":
		if is_whirlpool {
			return 0.3 // Orca Whirlpool common tier
		}
		return 0.3 // Orca standard

	case "meteora", "meteoradbc":
		return 0.25 // Meteora DLMM approximate

	case:
		log.debugf("Unknown DEX '%s', using default fee estimate", dex_id)
		return DEFAULT_FEE_PERCENT
	}
}

// Rank pools by score (descending order - highest score first)
//
// NOTE: Caller should set context.allocator = memory.request_allocator()
//
// Returns: Array of PoolScore sorted by score descending
rank_pools :: proc(pairs: []DexScreenerPair, arena_alloc := context.allocator) -> []PoolScore {
	// ASSERTION 1: Input array is not null
	assert(pairs != nil, "Pairs array cannot be nil")

	if len(pairs) == 0 {
		log.debug("No pools to rank (empty input)")
		return nil
	}

	log.debugf("Ranking %d pool(s)", len(pairs))

	// Calculate scores for all pools with arena allocator
	scores := make([dynamic]PoolScore, 0, len(pairs), arena_alloc)
	for pair in pairs {
		score := calculate_pool_score(pair)
		append(&scores, PoolScore{pair = pair, score = score})
		log.debugf("Pool %s (DEX: %s): score=%.2f (liquidity=$%.2f)",
			pair.pairAddress, pair.dexId, score, pair.liquidity.usd)
	}

	// Sort descending by score (highest first)
	slice.sort_by(scores[:], proc(a, b: PoolScore) -> bool {
		return a.score > b.score
	})

	log.debugf("Top pool: %s with score %.2f", scores[0].pair.dexId, scores[0].score)

	return scores[:]
}

// Filter pools by minimum liquidity and maximum fee thresholds
//
// Filtering Criteria:
// - liquidity.usd >= $1,000 (prevents scam/illiquid pools)
// - estimated fee <= 1.0% (prevents exploitative pools)
//
// NOTE: Caller should set context.allocator = memory.request_allocator()
//
// Returns: Filtered array of pools meeting criteria
filter_pools :: proc(pairs: []DexScreenerPair, arena_alloc := context.allocator) -> []DexScreenerPair {
	// ASSERTION 1: Input array is not null
	assert(pairs != nil, "Pairs array cannot be nil")

	if len(pairs) == 0 {
		log.debug("No pools to filter (empty input)")
		return nil
	}

	log.debugf("Filtering %d pool(s) with criteria: min_liquidity=$%.2f, max_fee=%.2f%%",
		len(pairs), MIN_LIQUIDITY_USD, MAX_FEE_PERCENT)

	filtered := make([dynamic]DexScreenerPair, 0, len(pairs), arena_alloc)
	filtered_count := 0

	for pair in pairs {
		// Check minimum liquidity
		if pair.liquidity.usd < MIN_LIQUIDITY_USD {
			log.debugf("Filtered out pool %s: liquidity $%.2f < $%.2f",
				pair.pairAddress, pair.liquidity.usd, MIN_LIQUIDITY_USD)
			continue
		}

		// Check maximum fee
		fee := estimate_pool_fee(pair)
		if fee > MAX_FEE_PERCENT {
			log.debugf("Filtered out pool %s: fee %.2f%% > %.2f%%",
				pair.pairAddress, fee, MAX_FEE_PERCENT)
			continue
		}

		// Pool passes all filters
		append(&filtered, pair)
		filtered_count += 1
	}

	log.infof("Filtered pools: %d passed, %d failed", filtered_count, len(pairs) - filtered_count)

	return filtered[:]
}

// Select best pool from array (highest scoring pool after filtering)
//
// Returns: Best pool and true, or empty pool and false if no pools available
select_best_pool :: proc(pairs: []DexScreenerPair) -> (DexScreenerPair, bool) {
	// ASSERTION 1: Input array is not null
	assert(pairs != nil, "Pairs array cannot be nil")

	if len(pairs) == 0 {
		log.warn("Cannot select best pool: empty input")
		return DexScreenerPair{}, false
	}

	// Set up request arena for this function
	arena_alloc := memory.request_allocator()

	// Filter pools first with arena allocator
	filtered := filter_pools(pairs, arena_alloc)

	if len(filtered) == 0 {
		log.warn("Cannot select best pool: no pools passed filtering criteria")
		return DexScreenerPair{}, false
	}

	// Rank filtered pools with arena allocator
	ranked := rank_pools(filtered, arena_alloc)

	if len(ranked) == 0 {
		log.error("Ranking returned empty array (should not happen)")
		return DexScreenerPair{}, false
	}

	// Return top-ranked pool
	best := ranked[0]
	log.infof("Selected best pool: %s on %s (score: %.2f, liquidity: $%.2f)",
		best.pair.pairAddress, best.pair.dexId, best.score, best.pair.liquidity.usd)

	return best.pair, true
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// Convert DexScreenerPair to PoolInfo (for database storage)
//
// Extracts essential pool metadata for token configuration
pair_to_pool_info :: proc(pair: DexScreenerPair) -> models.PoolInfo {
	// Determine pool type from labels
	pool_type := infer_pool_type(pair)

	// Extract quote token symbol (e.g., "SOL", "USDC", "USDT")
	quote_symbol := pair.quoteToken.symbol

	// Extract pool metadata for database storage
	liquidity_usd := pair.liquidity.usd
	volume_24h := pair.volume.h24
	fee_percent := estimate_pool_fee(pair)
	discovered_at := time.now()._nsec / 1_000_000_000 // Unix timestamp

	return models.PoolInfo{
		dex           = pair.dexId,
		pool_address  = pair.pairAddress,
		quote_token   = quote_symbol,
		pool_type     = pool_type,
		liquidity_usd = liquidity_usd,
		volume_24h    = volume_24h,
		fee_percent   = fee_percent,
		discovered_at = i64(discovered_at),
	}
}

// Infer pool type from DexScreener pair data
//
// Maps DexScreener labels to Hound pool type strings:
// - "CLMM" → "clmm" (Raydium Concentrated Liquidity)
// - "wp" → "whirlpool" (Orca Whirlpool)
// - "DLMM", "DYN" → "dlmm" (Meteora Dynamic Liquidity)
// - Default → "amm" (Standard AMM V4)
infer_pool_type :: proc(pair: DexScreenerPair) -> string {
	// Check labels for pool type indicators
	for label in pair.labels {
		switch label {
		case "CLMM":
			return "clmm"
		case "wp":
			return "whirlpool"
		case "DLMM", "DYN", "DYN2":
			return "dlmm"
		}
	}

	// Default to standard AMM
	return "amm"
}
