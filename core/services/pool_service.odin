// Pool service - business logic for pool discovery and ranking
// Stateless service functions for DEX pool discovery, ranking, and persistence
package services

import "core:log"
import "../models"
import "../database"
import "../dex"

// ============================================================================
// Service Context
// ============================================================================

// PoolServiceContext holds dependencies for pool operations
//
// This context is passed to all service functions, enabling:
// - Dependency injection
// - Stateless service functions
// - Easy testing with mock contexts
PoolServiceContext :: struct {
	db: ^database.Database,  // Database for pool persistence
}

// ============================================================================
// Pool Discovery
// ============================================================================

// PoolDiscoveryResult represents discovered pools with metadata
PoolDiscoveryResult :: struct {
	pools:         []dex.DexScreenerPair,  // All discovered pools
	filtered:      []dex.DexScreenerPair,  // Pools passing quality filters
	best_pool:     dex.DexScreenerPair,    // Highest-scoring pool
	best_found:    bool,                    // Whether a best pool was found
	total_count:   int,                     // Total pools discovered
	filtered_count: int,                    // Pools after filtering
}

// discover_pools fetches all pools for a token from DexScreener
//
// This function queries DexScreener API for all trading pairs:
// 1. Fetches from cache (1-hour TTL) if available
// 2. Queries DexScreener API on cache miss
// 3. Retries with exponential backoff on rate limit
// 4. Returns all pools (unfiltered, unranked)
//
// ASSERTION 1: Contract address must not be empty
//
// Returns: Array of pools and error status
discover_pools :: proc(
	contract_address: string,
	force_refresh: bool = false,
) -> (pools: []dex.DexScreenerPair, err: models.ErrorType) {
	assert(len(contract_address) > 0, "Contract address cannot be empty")

	log.infof("Discovering pools for token: %s (force_refresh=%v)", contract_address, force_refresh)

	// Fetch pools from DexScreener (with caching and retry)
	fetched_pools, fetch_err := dex.get_pools_cached(contract_address, force_refresh)
	if fetch_err != .None {
		log.errorf("Pool discovery failed: %v", fetch_err)
		return nil, fetch_err
	}

	log.infof("Discovered %d pool(s)", len(fetched_pools))
	return fetched_pools, .None
}

// ============================================================================
// Pool Ranking and Selection
// ============================================================================

// rank_and_select_best_pool filters, ranks, and selects the best pool
//
// This is the primary pool selection algorithm:
// 1. Filter pools by quality criteria (min liquidity, max fee)
// 2. Rank remaining pools by weighted score
// 3. Return highest-scoring pool
//
// Filtering criteria:
// - Minimum liquidity: $1,000 USD
// - Maximum fee: 1.0%
//
// Ranking algorithm:
// - Score = (0.8 × liquidity_usd) - (0.2 × fee_percent × 10000)
// - Liquidity dominates (80% weight)
// - Fees penalize (20% weight)
//
// ASSERTION 1: Pools array must not be nil
//
// Returns: Best pool and found flag
rank_and_select_best_pool :: proc(
	pools: []dex.DexScreenerPair,
) -> (best_pool: dex.DexScreenerPair, found: bool) {
	assert(pools != nil, "Pools array cannot be nil")

	if len(pools) == 0 {
		log.warn("No pools to rank (empty input)")
		return {}, false
	}

	log.infof("Ranking and selecting best pool from %d candidates", len(pools))

	// Filter pools by quality criteria
	filtered := dex.filter_pools(pools)
	// NO delete - using command arena allocator

	if len(filtered) == 0 {
		log.warn("No pools passed quality filters")
		return {}, false
	}

	log.infof("Quality filter: %d/%d pools passed", len(filtered), len(pools))

	// Rank filtered pools
	ranked := dex.rank_pools(filtered)
	// NO delete - using command arena allocator

	if len(ranked) == 0 {
		log.error("Ranking returned empty array (should not happen)")
		return {}, false
	}

	// Select best pool (highest score)
	best := ranked[0]
	log.infof("Best pool selected: %s on %s (score: %.2f, liquidity: $%.2f)",
		best.pair.pairAddress, best.pair.dexId, best.score, best.pair.liquidity.usd)

	return best.pair, true
}

// ============================================================================
// Pool Discovery with Selection
// ============================================================================

// discover_and_select_best_pool combines discovery and selection
//
// This is the recommended one-step function for pool discovery:
// 1. Fetch all pools from DexScreener
// 2. Filter and rank pools
// 3. Return best pool
//
// ASSERTION 1: Contract address must not be empty
//
// Returns: Full discovery result with best pool selected
discover_and_select_best_pool :: proc(
	contract_address: string,
	force_refresh: bool = false,
) -> (result: PoolDiscoveryResult, err: models.ErrorType) {
	assert(len(contract_address) > 0, "Contract address cannot be empty")

	log.infof("Starting pool discovery and selection for token: %s", contract_address)

	// Step 1: Discover all pools
	pools, discover_err := discover_pools(contract_address, force_refresh)
	if discover_err != .None {
		log.errorf("Pool discovery failed: %v", discover_err)
		return {}, discover_err
	}
	// NO delete - using command arena allocator

	result.pools = pools
	result.total_count = len(pools)

	if len(pools) == 0 {
		log.info("No pools found for token")
		return result, .NoPoolsFound
	}

	// Step 2: Filter pools
	filtered := dex.filter_pools(pools)
	result.filtered = filtered
	result.filtered_count = len(filtered)

	if len(filtered) == 0 {
		log.warn("No pools passed quality filters")
		return result, .NoPoolsFound
	}

	// Step 3: Select best pool
	best_pool, found := rank_and_select_best_pool(pools)
	result.best_pool = best_pool
	result.best_found = found

	if !found {
		log.warn("Could not select best pool")
		return result, .NoPoolsFound
	}

	log.infof("Pool discovery completed: best pool = %s on %s",
		best_pool.pairAddress, best_pool.dexId)

	return result, .None
}

// ============================================================================
// Pool Persistence
// ============================================================================

// store_discovered_pool saves a pool to database with metadata
//
// This enables:
// - Persistent pool configuration
// - Historical pool tracking
// - Pool metadata for future price fetching
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Token symbol must not be empty
// ASSERTION 3: Pool pair must have valid address
//
// Returns: Error status
store_discovered_pool :: proc(
	ctx: ^PoolServiceContext,
	token_symbol: string,
	pool: dex.DexScreenerPair,
) -> models.ErrorType {
	assert(ctx != nil, "Pool service context cannot be nil")
	assert(len(token_symbol) > 0, "Token symbol cannot be empty")
	assert(len(pool.pairAddress) > 0, "Pool address cannot be empty")

	log.infof("Storing pool for token %s: %s on %s", token_symbol, pool.pairAddress, pool.dexId)

	// Convert DexScreener pair to PoolInfo
	pool_info := dex.pair_to_pool_info(pool)

	// Store in database
	db_err := database.insert_pool(
		ctx.db,
		token_symbol,
		pool_info,
		pool_info.liquidity_usd,
		pool_info.volume_24h,
		pool_info.fee_percent,
		pool_info.discovered_at,
	)
	if db_err != .None {
		log.errorf("Failed to store pool: %v", db_err)
		return db_err
	}

	log.infof("Pool stored successfully: %s", pool.pairAddress)
	return .None
}

// ============================================================================
// Batch Pool Storage
// ============================================================================

// BatchStorageResult represents results of batch pool storage
BatchStorageResult :: struct {
	stored_count:  int,
	skipped_count: int,
	errors:        map[string]models.ErrorType,  // pool_address -> error
}

// store_multiple_pools stores top N pools for a token
//
// This enables storing multiple high-quality pools for redundancy:
// - Primary pool for immediate use
// - Backup pools for automatic failover
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Token symbol must not be empty
// ASSERTION 3: Pools array must not be empty
// ASSERTION 4: Count must be positive
//
// Returns: Batch storage result and error status
store_multiple_pools :: proc(
	ctx: ^PoolServiceContext,
	token_symbol: string,
	pools: []dex.DexScreenerPair,
	count: int,
) -> (result: BatchStorageResult, err: models.ErrorType) {
	assert(ctx != nil, "Pool service context cannot be nil")
	assert(len(token_symbol) > 0, "Token symbol cannot be empty")
	assert(len(pools) > 0, "Pools array cannot be empty")
	assert(count > 0, "Count must be positive")

	log.infof("Storing top %d pool(s) for token: %s", count, token_symbol)

	// Initialize result
	result.errors = make(map[string]models.ErrorType)

	// Store up to count pools
	pools_to_store := min(count, len(pools))

	for i := 0; i < pools_to_store; i += 1 {
		pool := pools[i]

		store_err := store_discovered_pool(ctx, token_symbol, pool)
		if store_err != .None {
			result.errors[pool.pairAddress] = store_err
			result.skipped_count += 1
			log.warnf("Failed to store pool #%d (%s): %v", i+1, pool.pairAddress, store_err)
		} else {
			result.stored_count += 1
			log.debugf("Stored pool #%d: %s", i+1, pool.pairAddress)
		}
	}

	log.infof("Batch storage complete: %d stored, %d skipped",
		result.stored_count, result.skipped_count)

	if result.stored_count == 0 {
		// All storage attempts failed
		log.error("Failed to store any pools")
		return result, .DatabaseError
	}

	return result, .None
}

// ============================================================================
// Pool Discovery with Auto-Storage
// ============================================================================

// discover_and_store_best_pools discovers pools and stores top N
//
// This is the complete auto-discovery workflow:
// 1. Fetch all pools from DexScreener
// 2. Filter and rank by quality
// 3. Store top N pools in database
// 4. Return best pool for immediate use
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Token must have valid contract address
// ASSERTION 3: Count must be positive
//
// Returns: Best pool and error status
discover_and_store_best_pools :: proc(
	ctx: ^PoolServiceContext,
	token: models.Token,
	count: int = 3,
	force_refresh: bool = false,
) -> (best_pool: models.PoolInfo, err: models.ErrorType) {
	assert(ctx != nil, "Pool service context cannot be nil")
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")
	assert(count > 0, "Count must be positive")

	log.infof("Auto-discovery workflow for token %s: discovering and storing top %d pools",
		token.symbol, count)

	// Step 1: Discover and select best pool
	discovery_result, discover_err := discover_and_select_best_pool(token.contract_address, force_refresh)
	if discover_err != .None {
		log.errorf("Pool discovery failed: %v", discover_err)
		return {}, discover_err
	}

	if !discovery_result.best_found {
		log.error("No suitable pools found")
		return {}, .NoPoolsFound
	}

	// Step 2: Rank all pools for storage
	ranked := dex.rank_pools(discovery_result.filtered)
	// NO delete - using command arena allocator

	if len(ranked) == 0 {
		log.error("Ranking returned empty array")
		return {}, .NoPoolsFound
	}

	// Step 3: Delete old pools if force refresh
	if force_refresh {
		log.debugf("Force refresh: deleting old pools for %s", token.symbol)
		delete_err := database.delete_pools_for_token(ctx.db, token.symbol)
		if delete_err != .None {
			log.warnf("Failed to delete old pools (non-fatal): %v", delete_err)
		}
	}

	// Step 4: Store top N pools
	pools_to_store := make([dynamic]dex.DexScreenerPair, 0, min(count, len(ranked)))
	for i := 0; i < min(count, len(ranked)); i += 1 {
		append(&pools_to_store, ranked[i].pair)
	}
	// NO delete - using command arena allocator

	storage_result, storage_err := store_multiple_pools(ctx, token.symbol, pools_to_store[:], count)
	if storage_err != .None {
		log.warnf("Pool storage failed (non-fatal): %v", storage_err)
		// Continue - return best pool even if storage fails
	}

	log.infof("Auto-discovery completed: stored %d pool(s) for %s",
		storage_result.stored_count, token.symbol)

	// Return best pool as PoolInfo
	best_pool_info := dex.pair_to_pool_info(discovery_result.best_pool)
	return best_pool_info, .None
}
