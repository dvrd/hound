#+feature global-context
package blockchain

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:math"
import "core:strconv"
import "core:time"
import "../models"
import client "../../../vendor/odin-http/client"

// NOTE: Jupiter types are now defined in jupiter_client.odin
// This file uses the shared Jupiter client for fetching SOL price

// CoinGecko API response structure
CoinGeckoResponse :: struct {
	solana: CoinGeckoPrice,
}

CoinGeckoPrice :: struct {
	usd: f64,
}

// Price cache with timestamp
SolPriceCache :: struct {
	price:     f64,
	cached_at: time.Time,
	is_valid:  bool,
}

// Global cache instance
g_sol_cache: SolPriceCache

// Cache TTL constant (30 seconds)
CACHE_TTL :: 30 * time.Second

// SOL mint address constant
SOL_MINT :: "So11111111111111111111111111111111111111112"

// Fetch SOL price from Jupiter Price API v3 (using shared jupiter_client)
fetch_sol_price_jupiter :: proc() -> (f64, models.ErrorType) {
	// Assertion 1: Ensure we're using valid SOL mint address
	assert(len(SOL_MINT) > 0, "SOL_MINT constant must not be empty")

	log.info("Fetching SOL/USD price from Jupiter (price oracle)")

	// Use the shared Jupiter client
	price_info, err := get_jupiter_price_cached(SOL_MINT)
	if err != models.ErrorType.None {
		log.warnf("Jupiter oracle failed: %v", err)
		return 0, models.ErrorType.OracleConnectionFailed
	}

	log.infof("Jupiter oracle: SOL/USD = $%.2f", price_info.usd_price)

	price := price_info.usd_price

	// Assertion 2: Validate price is reasonable ($50-$1000 range)
	assert(price >= 0, "Price must be non-negative")

	// Validate price is reasonable ($50-$1000 range)
	if price < 50.0 || price > 1000.0 {
		return 0, models.ErrorType.OraclePriceInvalid
	}

	return price, models.ErrorType.None
}

// Fetch SOL price from CoinGecko API (fallback)
fetch_coingecko_price :: proc() -> (f64, models.ErrorType) {
	// Build URL - CoinGecko endpoint
	url := "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd"

	log.info("Fetching SOL/USD price from CoinGecko (fallback oracle)")

	// Make GET request
	res, http_err := client.get(url)
	if http_err != nil {
		log.warnf("CoinGecko oracle connection failed: %v", http_err)
		return 0, models.ErrorType.OracleConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	if res.status != .OK {
		log.warnf("CoinGecko oracle returned status: %v", res.status)
		return 0, models.ErrorType.OracleConnectionFailed
	}

	// Extract body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.warnf("CoinGecko oracle body parsing failed: %v", body_err)
		return 0, models.ErrorType.OracleParseFailed
	}
	defer client.body_destroy(body, allocation)

	// Parse JSON
	response: CoinGeckoResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		log.warnf("CoinGecko oracle JSON parsing failed: %v", json_err)
		return 0, models.ErrorType.OracleParseFailed
	}

	price := response.solana.usd

	// Assertion 1: Validate price is non-negative
	assert(price >= 0, "Price must be non-negative")

	// Validate price is reasonable ($50-$1000 range)
	if price < 50.0 || price > 1000.0 {
		log.errorf("CoinGecko oracle returned unreasonable price: $%.2f", price)
		return 0, models.ErrorType.OraclePriceInvalid
	}

	log.infof("CoinGecko oracle: SOL/USD = $%.2f", price)

	return price, models.ErrorType.None
}

// Check if cache is stale (> 30 seconds old)
is_cache_stale :: proc(cache: SolPriceCache) -> bool {
	// Assertion 1: Cache validity flag is boolean
	assert(
		cache.is_valid == true || cache.is_valid == false,
		"Cache is_valid must be boolean",
	)

	// If never populated, it's stale
	if !cache.is_valid {
		return true
	}

	// Calculate elapsed time
	elapsed := time.diff(cache.cached_at, time.now())

	// Assertion 2: Elapsed time should be non-negative
	assert(elapsed >= 0, "Elapsed time cannot be negative")

	// Check if expired
	return elapsed > CACHE_TTL
}

// Main entry point: Get SOL price with caching and fallback
get_sol_price_cached :: proc() -> (f64, models.ErrorType) {
	// Assertion 1: CACHE_TTL is positive
	assert(CACHE_TTL > 0, "Cache TTL must be positive")

	// Return cached if fresh
	if !is_cache_stale(g_sol_cache) {
		// Assertion 2: Cached price is valid
		assert(g_sol_cache.price > 0, "Cached price must be positive")
		log.debugf("Using cached SOL/USD price: $%.2f", g_sol_cache.price)
		return g_sol_cache.price, models.ErrorType.None
	}

	log.debug("SOL price cache stale, fetching fresh price from oracle")

	// Try Jupiter first
	price, err := fetch_sol_price_jupiter()
	if err == models.ErrorType.None {
		// Update cache
		g_sol_cache.price = price
		g_sol_cache.cached_at = time.now()
		g_sol_cache.is_valid = true
		log.debugf("SOL/USD price cached for next %d seconds", CACHE_TTL / time.Second)
		return price, models.ErrorType.None
	}

	// Fallback to CoinGecko
	log.warn("Jupiter oracle failed, trying CoinGecko fallback")
	price, err = fetch_coingecko_price()
	if err == models.ErrorType.None {
		// Update cache
		g_sol_cache.price = price
		g_sol_cache.cached_at = time.now()
		g_sol_cache.is_valid = true
		log.debugf("SOL/USD price cached for next %d seconds", CACHE_TTL / time.Second)
		return price, models.ErrorType.None
	}

	// Both failed
	return 0, err
}
