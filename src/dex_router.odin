#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:strconv"
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
// - Raydium CLMM (planned for future)
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
	Raydium_CLMM,     // Raydium CLMM
	Raydium_AMM_V4,   // Raydium AMM V4 (standard AMM)
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
parse_dex_type :: proc(dex: string, pool_type: string = "") -> DexType {
	assert(len(dex) > 0, "DEX type string cannot be empty")

	lower_dex := strings.to_lower(dex)

	switch lower_dex {
	case "orca", "orca_whirlpool", "whirlpool":
		return .Orca_Whirlpool
	case "jupiter", "jupiter_api", "jupiter_aggregator":
		return .Jupiter_API
	case "raydium", "raydium_clmm", "raydium_amm":
		// For Raydium, distinguish between CLMM and AMM V4 based on pool_type
		if pool_type == "amm_v4" || pool_type == "amm" {
			return .Raydium_AMM_V4
		}
		return .Raydium_CLMM  // Default to CLMM if not specified
	case:
		log.warnf("Unknown DEX type: %s", dex)
		return .Unknown
	}
}

// Convert PoolInfo (from config) to DexPoolConfig (for routing)
//
// ASSERTION 1: Validate pool address for on-chain DEXs (not Jupiter API)
// ASSERTION 2: Validate quote token is not empty (except for Jupiter API)
pool_info_to_dex_config :: proc(pool: PoolInfo, priority: int = 1) -> DexPoolConfig {
	// Parse DEX type, passing pool_type for Raydium disambiguation
	dex_type := parse_dex_type(pool.dex, pool.pool_type)

	// Jupiter API doesn't need pool address or quote token
	if dex_type != .Jupiter_API {
		assert(len(pool.pool_address) > 0, "Pool address cannot be empty for on-chain DEXs")
	}

	return DexPoolConfig{
		dex_type     = dex_type,
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
			log.debugf("Successfully fetched price from %v: $%.6f", config.dex_type, price_result.price_usd)
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
		assert(len(config.pool_address) > 0, "Raydium CLMM pool address cannot be empty")
		return fetch_raydium_clmm_price(config, token)

	case .Raydium_AMM_V4:
		assert(len(config.pool_address) > 0, "Raydium AMM V4 pool address cannot be empty")
		return fetch_raydium_amm_v4_price(config, token)

	case .Unknown:
		log.error("Attempted to fetch from unknown DEX type")
		return {}, .PoolDataInvalid
	}

	return {}, .PoolDataInvalid
}

// Fetch price from Orca Whirlpool CLMM pool
//
// Implementation details:
// 1. Fetches pool account data via Solana RPC (653 bytes)
// 2. Decodes Orca Whirlpool state structure
// 3. Fetches token decimals from SPL Token mint accounts (key difference from Raydium)
// 4. Calculates price using Q64.64 sqrt_price format
// 5. Converts to USD using SOL oracle or stablecoin price
//
// Error handling:
// - .RPCConnectionFailed: Cannot connect to Solana RPC
// - .RPCInvalidResponse: Malformed RPC response or account data
// - .PoolDataInvalid: Pool decoding failed or zero liquidity
// - .TokenNotFound: Pool account or mint accounts don't exist
// - .OracleConnectionFailed: Cannot fetch SOL price for USD conversion
//
// Returns: DexPriceResult with price_usd, source (.Orca_Whirlpool), and pool_address
fetch_orca_whirlpool_price :: proc(config: DexPoolConfig, token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(config.pool_address) > 0, "Orca pool address cannot be empty")
	assert(len(config.quote_token) > 0, "Quote token cannot be empty")

	log.infof("Fetching from Orca Whirlpool: %s (quote: %s)", config.pool_address, config.quote_token)

	// 1. Setup RPC Connection
	conn := RPCConnection{endpoint = "https://api.mainnet-beta.solana.com", timeout = 10000}
	log.debugf("RPC endpoint: %s, timeout: %dms", conn.endpoint, conn.timeout)

	// 2. Fetch Pool Account Data
	pool_data, err := get_account_info(conn, config.pool_address)
	if err != .None {
		log.errorf("Failed to fetch Orca pool data: %v", err)
		return {}, err
	}
	defer delete(pool_data)
	log.debugf("Received %d bytes of pool data", len(pool_data))

	// 3. Decode Orca Whirlpool State
	pool_state, ok := decode_orca_whirlpool(pool_data)
	if !ok {
		log.error("Pool data decoding failed")
		return {}, .PoolDataInvalid
	}
	log.debugf("Pool decoded - sqrt_price: %v, liquidity: %v", pool_state.sqrt_price, pool_state.liquidity)

	// 4. Validate Liquidity
	if pool_state.liquidity == 0 {
		log.error("Pool has zero liquidity")
		return {}, .PoolDataInvalid
	}

	// 5. Fetch Token Decimals (KEY DIFFERENCE - Orca doesn't embed decimals)
	decimals_a, err_a := get_token_decimals(conn, pool_state.token_mint_a)
	if err_a != .None {
		log.errorf("Failed to fetch decimals for token A: %v", err_a)
		return {}, err_a
	}

	decimals_b, err_b := get_token_decimals(conn, pool_state.token_mint_b)
	if err_b != .None {
		log.errorf("Failed to fetch decimals for token B: %v", err_b)
		return {}, err_b
	}

	log.debugf("Token decimals: A=%d, B=%d", decimals_a, decimals_b)

	// 6. Calculate Price in Quote Token
	price_in_quote := sqrt_price_to_price(pool_state.sqrt_price, decimals_a, decimals_b)
	log.debugf("Price in quote token: %.18f", price_in_quote)

	// 7. Fetch Quote Token USD Price
	quote_usd_price: f64
	lower_quote := strings.to_lower(config.quote_token)

	switch lower_quote {
	case "sol", "wsol":
		sol_price, sol_err := get_sol_price_cached()
		if sol_err != .None {
			log.errorf("Failed to get SOL price: %v", sol_err)
			return {}, sol_err
		}
		quote_usd_price = sol_price
		log.debugf("SOL/USD price: $%.2f", quote_usd_price)

	case "usdc", "usdt":
		quote_usd_price = 1.0
		log.debug("Using 1.0 for stablecoin quote")

	case:
		log.errorf("Unsupported quote token: %s", config.quote_token)
		return {}, .PoolDataInvalid
	}

	// 8. Convert to USD
	price_usd := price_in_quote * quote_usd_price
	log.debugf("Calculated USD price: $%.6f (%.9f %s × $%.2f)",
		price_usd, price_in_quote, config.quote_token, quote_usd_price)

	// 9. Return Result
	log.info("Orca Whirlpool price fetch completed successfully")
	return DexPriceResult{
		price_usd    = price_usd,
		source       = .Orca_Whirlpool,
		pool_address = config.pool_address,
	}, .None
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

// Fetch price from Raydium CLMM pool
//
// ASSERTION 1: Validate pool address
// ASSERTION 2: Validate quote token
//
// Steps:
// 1. Fetch pool account data from Solana RPC
// 2. Decode Raydium CLMM PoolState
// 3. Convert sqrt_price_x64 to price (reuse Q64.64 conversion)
// 4. Fetch quote token price (SOL/USDC) and convert to USD
fetch_raydium_clmm_price :: proc(config: DexPoolConfig, token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(config.pool_address) > 0, "Raydium CLMM pool address cannot be empty")
	assert(len(config.quote_token) > 0, "Quote token cannot be empty")

	log.infof("Fetching from Raydium CLMM: %s (quote: %s)", config.pool_address, config.quote_token)

	// 1. Fetch pool account from RPC
	conn := RPCConnection{endpoint = "https://api.mainnet-beta.solana.com", timeout = 10000}
	pool_data, err := get_account_info(conn, config.pool_address)
	if err != .None {
		log.errorf("Failed to fetch Raydium CLMM pool data: %v", err)
		return {}, err
	}
	defer delete(pool_data)

	log.debugf("Received %d bytes of Raydium CLMM pool data", len(pool_data))

	// 2. Decode pool state
	pool_state, ok := decode_raydium_clmm_pool(pool_data)
	if !ok {
		log.error("Raydium CLMM pool decoding failed")
		return {}, .PoolDataInvalid
	}

	log.debugf("Pool decoded - decimals: (%d, %d), tick_spacing: %d",
		pool_state.mint_decimals_0, pool_state.mint_decimals_1, pool_state.tick_spacing)

	// 3. Calculate price in quote token
	// Raydium has embedded decimals - no need to fetch mint accounts
	price_in_quote := calculate_raydium_clmm_price(pool_state)

	log.debugf("Price in quote token: %.18f", price_in_quote)

	// 4. Convert to USD (get quote token price)
	quote_usd_price: f64
	lower_quote := strings.to_lower(config.quote_token)
	switch lower_quote {
	case "sol":
		sol_price, sol_err := get_sol_price_cached()
		if sol_err != .None {
			log.errorf("Failed to get SOL price: %v", sol_err)
			return {}, sol_err
		}
		quote_usd_price = sol_price
		log.debugf("SOL/USD price: $%.2f", sol_price)
	case "usdc", "usdt":
		quote_usd_price = 1.0  // USDC/USDT = $1.00
		log.debugf("Using %s = $1.00", config.quote_token)
	case:
		log.errorf("Unsupported quote token: %s", config.quote_token)
		return {}, .PoolDataInvalid
	}

	// 5. Calculate final USD price
	price_usd := price_in_quote * quote_usd_price

	log.infof("Raydium CLMM price: $%.6f %s",
		price_usd)
	log.infof("Raydium CLMM price: %.9f %s",
		price_in_quote, config.quote_token)

	return DexPriceResult{
		price_usd    = price_usd,
		source       = .Raydium_CLMM,
		pool_address = config.pool_address,
	}, .None
}

// Fetch price from Raydium AMM V4 pool
//
// ASSERTION 1: Validate pool address
// ASSERTION 2: Validate quote token
//
// Steps:
// 1. Fetch pool account data from Solana RPC (752 bytes)
// 2. Decode Raydium AMM V4 PoolState
// 3. Fetch vault balances for both tokens
// 4. Calculate price from reserves (x*y=k formula)
// 5. Fetch quote token price (SOL/USDC) and convert to USD
fetch_raydium_amm_v4_price :: proc(config: DexPoolConfig, token: Token) -> (DexPriceResult, ErrorType) {
	assert(len(config.pool_address) > 0, "Raydium AMM V4 pool address cannot be empty")
	assert(len(config.quote_token) > 0, "Quote token cannot be empty")

	log.infof("Fetching from Raydium AMM V4: %s (quote: %s)", config.pool_address, config.quote_token)

	// 1. Fetch pool account from RPC
	conn := RPCConnection{endpoint = "https://api.mainnet-beta.solana.com", timeout = 10000}
	pool_data, err := get_account_info(conn, config.pool_address)
	if err != .None {
		log.errorf("Failed to fetch Raydium AMM V4 pool data: %v", err)
		return {}, err
	}
	defer delete(pool_data)

	log.debugf("Received %d bytes of Raydium AMM V4 pool data", len(pool_data))

	// 2. Decode pool state
	pool_state, ok := decode_raydium_pool_v4(pool_data)
	if !ok {
		log.error("Raydium AMM V4 pool decoding failed")
		return {}, .PoolDataInvalid
	}

	log.debugf("Pool decoded - base_decimal: %d, quote_decimal: %d",
		pool_state.base_decimal, pool_state.quote_decimal)

	// 3. Fetch vault balances
	base_vault_addr := pubkey_to_base58(pool_state.base_vault)
	quote_vault_addr := pubkey_to_base58(pool_state.quote_vault)

	log.debugf("Base vault: %s, Quote vault: %s", base_vault_addr, quote_vault_addr)

	base_balance, base_err := get_token_balance(conn, base_vault_addr)
	if base_err != .None {
		log.errorf("Failed to fetch base vault balance: %v", base_err)
		return {}, .VaultFetchFailed
	}

	quote_balance, quote_err := get_token_balance(conn, quote_vault_addr)
	if quote_err != .None {
		log.errorf("Failed to fetch quote vault balance: %v", quote_err)
		return {}, .VaultFetchFailed
	}

	// Parse amounts
	base_reserve, base_parse_ok := strconv.parse_u64(base_balance.amount)
	if !base_parse_ok {
		log.error("Failed to parse base vault balance")
		return {}, .VaultFetchFailed
	}

	quote_reserve, quote_parse_ok := strconv.parse_u64(quote_balance.amount)
	if !quote_parse_ok {
		log.error("Failed to parse quote vault balance")
		return {}, .VaultFetchFailed
	}

	log.debugf("Base reserve: %d, Quote reserve: %d", base_reserve, quote_reserve)

	// 4. Calculate price in quote token
	price_in_quote := calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		pool_state.base_decimal,
		pool_state.quote_decimal,
	)

	log.debugf("Price in quote token: %.18f", price_in_quote)

	// 5. Convert to USD (get quote token price)
	quote_usd_price: f64
	lower_quote := strings.to_lower(config.quote_token)
	switch lower_quote {
	case "sol":
		sol_price, sol_err := get_sol_price_cached()
		if sol_err != .None {
			log.errorf("Failed to get SOL price: %v", sol_err)
			return {}, sol_err
		}
		quote_usd_price = sol_price
		log.debugf("SOL/USD price: $%.2f", sol_price)
	case "usdc", "usdt":
		quote_usd_price = 1.0  // USDC/USDT = $1.00
		log.debugf("Using %s = $1.00", config.quote_token)
	case:
		log.errorf("Unsupported quote token: %s", config.quote_token)
		return {}, .PoolDataInvalid
	}

	// 6. Calculate final USD price
	price_usd := price_in_quote * quote_usd_price

	log.infof("Raydium AMM V4 price: $%.6f",
		price_usd)
	log.infof("Raydium AMM V4 price: %.9f %s",
		price_in_quote, config.quote_token)

	return DexPriceResult{
		price_usd    = price_usd,
		source       = .Raydium_AMM_V4,
		pool_address = config.pool_address,
	}, .None
}
