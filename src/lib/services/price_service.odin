// Price service - business logic for price fetching operations
// Stateless service functions for token price discovery from multiple sources
package services

import "core:log"
import "../models"
import "../dex"
import "../blockchain"

// ============================================================================
// Service Context
// ============================================================================

// PriceServiceContext holds dependencies for price operations
//
// This context is passed to all service functions, enabling:
// - Dependency injection
// - Stateless service functions
// - Easy testing with mock contexts
PriceServiceContext :: struct {
	config: ^models.TokenConfig,  // Token configuration for pool info
}

// ============================================================================
// Price Discovery Methods
// ============================================================================

// PriceSource indicates where the price was fetched from
PriceSource :: enum {
	OnChain,  // Fetched from on-chain DEX pools
	API,      // Fetched from DexScreener API
}

// PriceResult represents a complete price fetch result
PriceResult :: struct {
	price_usd:  f64,
	change_24h: f64,
	source:     PriceSource,
}

// fetch_api_price fetches token price from DexScreener API
//
// This is the fallback method for tokens without configured pools.
// Uses DexScreener aggregator to find best available price across DEXs.
//
// ASSERTION 1: Contract address must not be empty
//
// Returns: Price result and error status
fetch_api_price :: proc(
	contract_address: string,
) -> (result: PriceResult, err: models.ErrorType) {
	assert(len(contract_address) > 0, "Contract address cannot be empty")

	log.infof("Fetching API price for token: %s", contract_address)

	// Fetch from DexScreener
	price_data, fetch_err := dex.fetch_price(contract_address)
	if fetch_err != .None {
		log.errorf("API price fetch failed: %v", fetch_err)
		return {}, fetch_err
	}

	log.infof("API price fetched: $%.6f (%.2f%% 24h)", price_data.price_usd, price_data.change_24h)

	return PriceResult{
		price_usd  = price_data.price_usd,
		change_24h = price_data.change_24h,
		source     = .API,
	}, .None
}

// fetch_onchain_price_for_token fetches token price from on-chain DEX pools
//
// This method queries configured pools in priority order:
// 1. Orca Whirlpool CLMM pools
// 2. Raydium AMM v4 pools
// 3. Jupiter Aggregator API (fallback)
//
// Provides best price discovery with automatic failover.
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Token must have valid contract address
//
// Returns: Price result and error status
fetch_onchain_price_for_token :: proc(
	ctx: ^PriceServiceContext,
	token: models.Token,
) -> (result: PriceResult, err: models.ErrorType) {
	assert(ctx != nil, "Price service context cannot be nil")
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")

	log.infof("Fetching on-chain price for token: %s", token.symbol)

	// Check if token has configured pools
	if len(token.pools) == 0 {
		log.warn("No pools configured, falling back to API")
		return fetch_api_price(token.contract_address)
	}

	// Fetch from on-chain DEX pools (with automatic failover)
	price_data, fetch_err := dex.fetch_onchain_price(token)
	if fetch_err != .None {
		log.errorf("On-chain price fetch failed: %v", fetch_err)
		return {}, fetch_err
	}

	log.infof("On-chain price fetched: $%.6f (%.2f%% 24h)", price_data.price_usd, price_data.change_24h)

	return PriceResult{
		price_usd  = price_data.price_usd,
		change_24h = price_data.change_24h,
		source     = .OnChain,
	}, .None
}

// fetch_price_with_fallback attempts on-chain first, falls back to API
//
// This is the recommended method for price fetching as it provides:
// - Best accuracy (on-chain when available)
// - High availability (API fallback)
// - Optimal performance (prioritizes fastest source)
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Token must have valid contract address
//
// Returns: Price result and error status
fetch_price_with_fallback :: proc(
	ctx: ^PriceServiceContext,
	token: models.Token,
) -> (result: PriceResult, err: models.ErrorType) {
	assert(ctx != nil, "Price service context cannot be nil")
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")

	log.infof("Fetching price with fallback for token: %s", token.symbol)

	// Try on-chain first if pools configured
	if len(token.pools) > 0 {
		log.debug("Attempting on-chain price fetch")
		onchain_result, onchain_err := fetch_onchain_price_for_token(ctx, token)
		if onchain_err == .None {
			log.info("On-chain price fetch succeeded")
			return onchain_result, .None
		}
		log.warnf("On-chain price fetch failed (%v), falling back to API", onchain_err)
	} else {
		log.debug("No pools configured, using API directly")
	}

	// Fallback to API
	log.debug("Fetching price from API")
	return fetch_api_price(token.contract_address)
}

// ============================================================================
// 24-Hour Change Operations
// ============================================================================

// get_24h_change fetches cached 24-hour price change percentage
//
// This function uses a 5-minute TTL cache to minimize API calls.
// Returns cached value when available and fresh, fetches new value when stale.
//
// ASSERTION 1: Contract address must not be empty
//
// Returns: Percentage change (e.g., 3.45 = +3.45%) and error status
get_24h_change :: proc(
	contract_address: string,
) -> (change_percent: f64, err: models.ErrorType) {
	assert(len(contract_address) > 0, "Contract address cannot be empty")

	log.debugf("Fetching 24h change for token: %s", contract_address)

	// Use cached API function
	change, fetch_err := dex.get_24h_change_cached(contract_address)
	if fetch_err != .None {
		log.warnf("24h change fetch failed: %v", fetch_err)
		return 0.0, fetch_err
	}

	log.debugf("24h change: %.2f%%", change)
	return change, .None
}

// ============================================================================
// SOL Price Operations
// ============================================================================

// get_sol_price fetches cached SOL/USD price
//
// SOL price is fetched from Pyth Network oracle and cached for 30 seconds.
// This is used for converting SOL-denominated prices to USD.
//
// Returns: SOL price in USD and error status
get_sol_price :: proc() -> (price_usd: f64, err: models.ErrorType) {
	log.debug("Fetching SOL/USD price from oracle")

	// Fetch from blockchain oracle (30s cache)
	sol_price, oracle_err := blockchain.get_sol_price_cached()
	if oracle_err != .None {
		log.errorf("SOL price fetch failed: %v", oracle_err)
		return 0.0, oracle_err
	}

	log.debugf("SOL/USD price: $%.2f", sol_price)
	return sol_price, .None
}

// ============================================================================
// Batch Price Operations
// ============================================================================

// BatchPriceResult represents results of batch price fetching
BatchPriceResult :: struct {
	prices:        map[string]PriceResult,   // contract_address -> price
	success_count: int,
	failure_count: int,
	errors:        map[string]models.ErrorType,  // contract_address -> error
}

// fetch_multiple_prices fetches prices for multiple tokens in batch
//
// This enables efficient batch price fetching with best-effort semantics:
// - Continues fetching even if some tokens fail
// - Collects all errors for debugging
// - Returns partial results when available
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Tokens slice must not be empty
//
// Returns: Batch price result and error status
fetch_multiple_prices :: proc(
	ctx: ^PriceServiceContext,
	tokens: []models.Token,
) -> (result: BatchPriceResult, err: models.ErrorType) {
	assert(ctx != nil, "Price service context cannot be nil")
	assert(len(tokens) > 0, "Tokens slice cannot be empty")

	log.infof("Fetching prices for %d token(s) in batch", len(tokens))

	// Initialize result
	result.prices = make(map[string]PriceResult)
	result.errors = make(map[string]models.ErrorType)

	// Fetch price for each token
	for token in tokens {
		price_result, fetch_err := fetch_price_with_fallback(ctx, token)

		if fetch_err == .None {
			// Success
			result.prices[token.contract_address] = price_result
			result.success_count += 1
			log.debugf("Token %s: $%.6f", token.symbol, price_result.price_usd)
		} else {
			// Failure
			result.errors[token.contract_address] = fetch_err
			result.failure_count += 1
			log.warnf("Token %s failed: %v", token.symbol, fetch_err)
		}
	}

	log.infof("Batch price fetch complete: %d/%d successful",
		result.success_count, len(tokens))

	// Determine overall status
	if result.failure_count == len(tokens) {
		// All failed
		log.error("All price fetches failed")
		return result, .NetworkError
	}

	// At least partial success
	return result, .None
}
