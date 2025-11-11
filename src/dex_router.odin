#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:strings"
import client "../vendor/odin-http/client"

// =============================================================================
// DEX ROUTER - Multi-DEX Price Routing with Priority-Based Fallback
// =============================================================================
// This module implements intelligent routing of price queries across multiple
// DEX sources with priority-based fallback and comprehensive error handling.
//
// Supported DEX Types:
// - Orca Whirlpool (CLMM pools)
// - Jupiter Aggregator API (v3)
// - Raydium CLMM (deferred to Phase 4.5)
//
// Routing Strategy:
// 1. Sort pools by priority (lowest number = highest priority)
// 2. Try each pool in order until success
// 3. Fall back to Jupiter API if all pools fail
// 4. Return comprehensive error if all sources fail
//
// Architecture:
// - Decoupled from specific DEX implementations
// - Extensible for new DEX types
// - Comprehensive error handling per TigerBeetle philosophy
// =============================================================================

// DEX type enumeration for routing decisions
DexType :: enum {
	Orca_Whirlpool,   // Orca CLMM pools
	Jupiter_API,      // Jupiter Aggregator API
	Raydium_CLMM,     // Raydium CLMM (Phase 4.5)
	Unknown,          // Unsupported DEX
}

// Extended pool information with priority and DEX type
DexPoolConfig :: struct {
	dex_type:      DexType,  // Which DEX this pool belongs to
	pool_address:  string,   // On-chain pool address (for CLMM pools)
	quote_token:   string,   // Quote token (sol, usdc, etc.)
	priority:      int,      // Lower = higher priority (1 = highest)
	pool_type:     string,   // Pool type identifier (e.g., "whirlpool", "clmm")
}

// Price result from DEX query
DexPriceResult :: struct {
	price_usd:    f64,      // Token price in USD
	source:       DexType,  // Which DEX provided the price
	pool_address: string,   // Pool address (if on-chain)
}

// Parse DEX type from string (from config)
//
// ASSERTION 1: Validate dex string is not empty
parse_dex_type :: proc(dex: string) -> DexType {
	assert(len(dex) > 0, "DEX type string cannot be empty")

	lower_dex := strings.to_lower(dex)

	switch lower_dex {
	case "orca", "orca_whirlpool", "whirlpool":
		return .Orca_Whirlpool
	case "jupiter", "jupiter_api", "jupiter_aggregator":
		return .Jupiter_API
	case "raydium", "raydium_clmm":
		return .Raydium_CLMM
	case:
		log.warnf("Unknown DEX type: %s", dex)
		return .Unknown
	}
}

// Convert PoolInfo (from config) to DexPoolConfig (for routing)
//
// ASSERTION 1: Validate pool address is not empty
// ASSERTION 2: Validate quote token is not empty
pool_info_to_dex_config :: proc(pool: PoolInfo, priority: int = 1) -> DexPoolConfig {
	assert(len(pool.pool_address) > 0, "Pool address cannot be empty")
	assert(len(pool.quote_token) > 0, "Quote token cannot be empty")

	return DexPoolConfig{
		dex_type     = parse_dex_type(pool.dex),
		pool_address = pool.pool_address,
		quote_token  = pool.quote_token,
		priority     = priority,
		pool_type    = pool.pool_type,
	}
}

// Main router: Query token price across multiple DEX sources with fallback
//
// This is the primary entry point for multi-DEX price fetching.
//
// ASSERTION 1: Validate token has contract address
// ASSERTION 2: Validate pools array is valid (can be empty)
//
// Algorithm:
// 1. Convert PoolInfo array to DexPoolConfig array with priorities
// 2. Sort by priority (lowest first)
// 3. Try each on-chain pool in order
// 4. Fall back to Jupiter API if all pools fail
// 5. Return comprehensive error if all sources fail
route_price_query :: proc(token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")

	log.infof("Routing price query for token: %s (%s)", token.symbol, token.contract_address)

	// Convert pools to DEX configs with priorities
	dex_configs := make([dynamic]DexPoolConfig, 0, len(token.pools))
	defer delete(dex_configs)

	for pool, idx in token.pools {
		// Priority based on order in config (first = highest priority)
		priority := idx + 1
		config := pool_info_to_dex_config(pool, priority)

		// Skip unsupported DEX types
		if config.dex_type == .Unknown {
			log.warnf("Skipping unsupported DEX: %s", pool.dex)
			continue
		}

		append(&dex_configs, config)
		log.debugf("Added pool: %s (priority: %d, type: %v)", config.pool_address, config.priority, config.dex_type)
	}

	log.infof("Attempting price fetch from %d configured pool(s)", len(dex_configs))

	// Try each pool in priority order
	for config in dex_configs {
		log.infof("Trying pool: %s (DEX: %v, priority: %d)", config.pool_address, config.dex_type, config.priority)

		price_result, err := fetch_from_dex(config, token)

		if err == .None {
			log.infof("Successfully fetched price from %v: $%.6f", config.dex_type, price_result.price_usd)
			return price_result, .None
		}

		log.warnf("Pool %s failed with error: %v, trying next source", config.pool_address, err)
	}

	// All pools failed - fall back to Jupiter API
	log.info("All configured pools failed, falling back to Jupiter Aggregator API")

	price_info, err := get_jupiter_price_cached(token.contract_address)
	if err == .None {
		log.infof("Jupiter API fetch successful: $%.6f", price_info.usd_price)
		return DexPriceResult{
			price_usd    = price_info.usd_price,
			source       = .Jupiter_API,
			pool_address = "",
		}, .None
	}

	log.errorf("Jupiter API fallback failed: %v", err)

	// All sources failed
	return {}, err
}

// Fetch price from specific DEX pool
//
// ASSERTION 1: Validate pool address for on-chain DEXs
//
// Dispatches to appropriate DEX-specific fetcher based on dex_type
fetch_from_dex :: proc(config: DexPoolConfig, token: Token) -> (DexPriceResult, ErrorType) {
	log.debugf("Fetching from DEX: %v (pool: %s)", config.dex_type, config.pool_address)

	switch config.dex_type {
	case .Orca_Whirlpool:
		assert(len(config.pool_address) > 0, "Orca pool address cannot be empty")
		return fetch_orca_whirlpool_price(config, token)

	case .Jupiter_API:
		// Jupiter API doesn't need pool address
		return fetch_jupiter_api_price(token)

	case .Raydium_CLMM:
		log.warn("Raydium CLMM support deferred to Phase 4.5")
		return {}, .PoolDataInvalid

	case .Unknown:
		log.error("Attempted to fetch from unknown DEX type")
		return {}, .PoolDataInvalid
	}

	return {}, .PoolDataInvalid
}

// Fetch price from Orca Whirlpool CLMM pool
//
// ASSERTION 1: Validate pool address
// ASSERTION 2: Validate quote token
//
// Steps:
// 1. Fetch pool account data from Solana RPC
// 2. Decode Whirlpool state (using orca_decoder)
// 3. Convert sqrt_price to real price
// 4. Fetch quote token price (SOL/USDC) and convert to USD
fetch_orca_whirlpool_price :: proc(config: DexPoolConfig, token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(config.pool_address) > 0, "Orca pool address cannot be empty")
	assert(len(config.quote_token) > 0, "Quote token cannot be empty")

	log.infof("Fetching from Orca Whirlpool: %s (quote: %s)", config.pool_address, config.quote_token)

	// TODO Phase 4.4: Implement Orca Whirlpool price fetching
	// 1. Fetch pool account data via RPC
	// 2. Decode Whirlpool state
	// 3. Get token decimals for both tokens
	// 4. Convert sqrt_price to price using sqrt_price_to_price()
	// 5. Fetch quote token price (SOL or USDC)
	// 6. Calculate token USD price
	//
	// This requires:
	// - RPC client integration (from existing rpc_client.odin)
	// - Token mint account fetching for decimals
	// - Quote token price oracle (SOL from sol_oracle, USDC = $1)

	log.warn("Orca Whirlpool fetching not yet implemented - returning error")
	return {}, .PoolDataInvalid
}

// Fetch price from Jupiter Aggregator API
//
// ASSERTION 1: Validate token contract address
//
// Uses the shared Jupiter client (jupiter_client.odin)
fetch_jupiter_api_price :: proc(token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")

	log.infof("Fetching from Jupiter API: %s", token.contract_address)

	price_info, err := get_jupiter_price_cached(token.contract_address)
	if err != .None {
		log.errorf("Jupiter API fetch failed: %v", err)
		return {}, err
	}

	log.infof("Jupiter API fetch successful: $%.6f", price_info.usd_price)

	return DexPriceResult{
		price_usd    = price_info.usd_price,
		source       = .Jupiter_API,
		pool_address = "",
	}, .None
}
