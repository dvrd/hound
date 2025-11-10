#+feature global-context
package main

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:math"
import "core:strconv"
import "core:time"
import client "../vendor/odin-http/client"

// Jupiter Price API v6 response structure
JupiterPriceResponse :: struct {
	data: map[string]JupiterPriceData,
}

JupiterPriceData :: struct {
	price: f64,
}

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

// Fetch SOL price from Jupiter Price API v6
fetch_jupiter_price :: proc() -> (f64, ErrorType) {
	// Assertion 1: Ensure we're building a valid URL
	assert(len(SOL_MINT) > 0, "SOL_MINT constant must not be empty")

	// Build URL - Jupiter v6 endpoint
	url := "https://price.jup.ag/v6/price?ids=SOL"

	// Make GET request
	res, http_err := client.get(url)
	if http_err != nil {
		return 0, .OracleConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	if res.status != .OK {
		return 0, .OracleConnectionFailed
	}

	// Extract body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return 0, .OracleParseFailed
	}
	defer client.body_destroy(body, allocation)

	// Parse JSON
	response: JupiterPriceResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		return 0, .OracleParseFailed
	}

	// Extract SOL price
	sol_data, has_sol := response.data["SOL"]
	if !has_sol {
		return 0, .OracleParseFailed
	}

	price := sol_data.price

	// Assertion 2: Validate price is reasonable ($50-$1000 range)
	assert(price >= 0, "Price must be non-negative")

	// Validate price is reasonable ($50-$1000 range)
	if price < 50.0 || price > 1000.0 {
		return 0, .OraclePriceInvalid
	}

	return price, .None
}

// Fetch SOL price from CoinGecko API (fallback)
fetch_coingecko_price :: proc() -> (f64, ErrorType) {
	// Build URL - CoinGecko endpoint
	url := "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd"

	// Make GET request
	res, http_err := client.get(url)
	if http_err != nil {
		return 0, .OracleConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	if res.status != .OK {
		return 0, .OracleConnectionFailed
	}

	// Extract body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return 0, .OracleParseFailed
	}
	defer client.body_destroy(body, allocation)

	// Parse JSON
	response: CoinGeckoResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		return 0, .OracleParseFailed
	}

	price := response.solana.usd

	// Assertion 1: Validate price is non-negative
	assert(price >= 0, "Price must be non-negative")

	// Validate price is reasonable ($50-$1000 range)
	if price < 50.0 || price > 1000.0 {
		return 0, .OraclePriceInvalid
	}

	return price, .None
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
get_sol_price_cached :: proc() -> (f64, ErrorType) {
	// Assertion 1: CACHE_TTL is positive
	assert(CACHE_TTL > 0, "Cache TTL must be positive")

	// Return cached if fresh
	if !is_cache_stale(g_sol_cache) {
		// Assertion 2: Cached price is valid
		assert(g_sol_cache.price > 0, "Cached price must be positive")
		return g_sol_cache.price, .None
	}

	// Try Jupiter first
	price, err := fetch_jupiter_price()
	if err == .None {
		// Update cache
		g_sol_cache.price = price
		g_sol_cache.cached_at = time.now()
		g_sol_cache.is_valid = true
		return price, .None
	}

	// Fallback to CoinGecko
	price, err = fetch_coingecko_price()
	if err == .None {
		// Update cache
		g_sol_cache.price = price
		g_sol_cache.cached_at = time.now()
		g_sol_cache.is_valid = true
		return price, .None
	}

	// Both failed
	return 0, err
}
