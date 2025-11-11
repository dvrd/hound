#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "core:time"
import "../src"

// =============================================================================
// JUPITER PRICE API V3 CLIENT TESTS
// =============================================================================
// These tests validate the Jupiter Price API v3 client implementation
// including caching logic, error handling, and retry mechanisms.
//
// Test categories:
// 1. Cache staleness logic (deterministic)
// 2. Cache state management
// 3. Integration tests for API calls (require network)
//
// Cache TTL: 60 seconds (JUPITER_CACHE_TTL)
// Rate limit: 600 requests/60 seconds (Lite tier)
// Retry strategy: 1s → 2s → 4s with exponential backoff
// =============================================================================

@(test)
test_is_jupiter_cache_stale_never_populated :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that unpopulated cache is always stale
	// A cache that has never been populated (is_valid = false) should
	// always be considered stale, regardless of timestamp.

	cache := src.JupiterPriceCache{
		mint_address = "So11111111111111111111111111111111111111112",
		price_info   = src.JupiterPriceInfo{},
		cached_at    = time.now(),
		is_valid     = false,
	}

	is_stale := src.is_jupiter_cache_stale(cache, "So11111111111111111111111111111111111111112")

	testing.expect(t, is_stale,
		"Cache with is_valid=false should be stale")
}

@(test)
test_is_jupiter_cache_stale_different_mint :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that cache for different mint is stale
	// Even if cache is valid and fresh, it's stale if the mint address differs.

	cache := src.JupiterPriceCache{
		mint_address = "So11111111111111111111111111111111111111112",  // SOL
		price_info   = src.JupiterPriceInfo{usd_price = 150.0},
		cached_at    = time.now(),
		is_valid     = true,
	}

	different_mint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"  // USDC
	is_stale := src.is_jupiter_cache_stale(cache, different_mint)

	testing.expect(t, is_stale,
		"Cache for different mint address should be stale")
}

@(test)
test_is_jupiter_cache_stale_fresh :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that fresh cache (< 60 seconds old) is not stale
	// A valid cache with the same mint and timestamp within TTL should be fresh.

	cache := src.JupiterPriceCache{
		mint_address = "So11111111111111111111111111111111111111112",
		price_info   = src.JupiterPriceInfo{usd_price = 150.0},
		cached_at    = time.now(),  // Just cached
		is_valid     = true,
	}

	is_stale := src.is_jupiter_cache_stale(cache, "So11111111111111111111111111111111111111112")

	testing.expect(t, !is_stale,
		"Freshly cached data should not be stale")
}

@(test)
test_is_jupiter_cache_stale_expired :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test that expired cache (> 60 seconds old) is stale
	// A cache older than JUPITER_CACHE_TTL (60 seconds) should be stale.

	// Create cache with timestamp 61 seconds in the past
	past_time := time.now()
	past_time._nsec -= 61_000_000_000  // 61 seconds in nanoseconds

	cache := src.JupiterPriceCache{
		mint_address = "So11111111111111111111111111111111111111112",
		price_info   = src.JupiterPriceInfo{usd_price = 150.0},
		cached_at    = past_time,
		is_valid     = true,
	}

	is_stale := src.is_jupiter_cache_stale(cache, "So11111111111111111111111111111111111111112")

	testing.expect(t, is_stale,
		"Cache older than 60 seconds should be stale")
}

@(test)
test_jupiter_price_info_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test JupiterPriceInfo structure fields
	// Verify that the structure can hold expected data types and values.

	price_info := src.JupiterPriceInfo{
		usd_price        = 147.48,
		block_id         = 348004026,
		decimals         = 9,
		price_change_24h = 1.29,
	}

	testing.expect(t, price_info.usd_price == 147.48,
		fmt.tprintf("Expected usd_price=147.48, got %.2f", price_info.usd_price))

	testing.expect(t, price_info.block_id == 348004026,
		fmt.tprintf("Expected block_id=348004026, got %d", price_info.block_id))

	testing.expect(t, price_info.decimals == 9,
		fmt.tprintf("Expected decimals=9, got %d", price_info.decimals))

	testing.expect(t, price_info.price_change_24h == 1.29,
		fmt.tprintf("Expected price_change_24h=1.29, got %.2f", price_info.price_change_24h))
}

@(test)
test_jupiter_cache_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test JupiterPriceCache structure initialization
	// Verify that cache structure can be properly initialized and accessed.

	cache := src.JupiterPriceCache{
		mint_address = "So11111111111111111111111111111111111111112",
		price_info   = src.JupiterPriceInfo{usd_price = 150.0, decimals = 9},
		cached_at    = time.now(),
		is_valid     = true,
	}

	testing.expect(t, cache.mint_address == "So11111111111111111111111111111111111111112",
		"Mint address should match")

	testing.expect(t, cache.price_info.usd_price == 150.0,
		fmt.tprintf("Expected price=150.0, got %.2f", cache.price_info.usd_price))

	testing.expect(t, cache.is_valid == true,
		"Cache should be marked as valid")
}

// =============================================================================
// INTEGRATION TESTS - Require Network Access
// =============================================================================
// The following tests make actual HTTP requests to Jupiter API.
// They may fail if:
// - Network is unavailable
// - Jupiter API is down
// - Rate limits are exceeded
// - Token addresses become invalid
//
// These tests are intended to validate real-world behavior and should be
// run in integration test suites, not necessarily in CI/CD pipelines.
// =============================================================================

@(test)
test_fetch_jupiter_price_valid_token_sol :: proc(t: ^testing.T) {
	// INTEGRATION TEST: Fetch real SOL price from Jupiter API
	// This test requires network access and may be rate-limited.
	//
	// SOL mint: So11111111111111111111111111111111111111112
	// Expected: Valid price data with usd_price > 0

	sol_mint := "So11111111111111111111111111111111111111112"

	price_info, err := src.fetch_jupiter_price(sol_mint)

	// Check for success
	if err != .None {
		// Warning: API may be down or rate limited
		// This is expected if network is unavailable or rate limited
		// Skip test if API is unavailable
		return
	}

	// Validate price data
	testing.expect(t, price_info.usd_price > 0,
		fmt.tprintf("Expected positive price, got %.6f", price_info.usd_price))

	testing.expect(t, price_info.decimals == 9,
		fmt.tprintf("SOL should have 9 decimals, got %d", price_info.decimals))

	testing.expect(t, price_info.block_id > 0,
		fmt.tprintf("Expected positive block_id, got %d", price_info.block_id))
}

@(test)
test_get_jupiter_price_cached_sol :: proc(t: ^testing.T) {
	// INTEGRATION TEST: Test cached price fetch for SOL
	// This test verifies caching behavior with real API calls.
	//
	// Expected behavior:
	// 1. First call: Cache miss, fetches from API
	// 2. Second call: Cache hit, returns cached data

	sol_mint := "So11111111111111111111111111111111111111112"

	// Clear cache by setting invalid state
	src.g_jupiter_cache.is_valid = false

	// First call - should fetch from API
	price_info_1, err_1 := src.get_jupiter_price_cached(sol_mint)

	if err_1 != .None {
		// Warning: Jupiter API call failed
		// This is expected if network is unavailable or rate limited
		// Skip test if API is unavailable
		return
	}

	// Second call - should hit cache
	price_info_2, err_2 := src.get_jupiter_price_cached(sol_mint)

	testing.expect(t, err_2 == .None,
		fmt.tprintf("Cached call should succeed, got error: %v", err_2))

	// Prices should match (from cache)
	testing.expect(t, price_info_1.usd_price == price_info_2.usd_price,
		fmt.tprintf("Cached price should match: %.6f vs %.6f",
			price_info_1.usd_price, price_info_2.usd_price))
}

@(test)
test_fetch_jupiter_price_invalid_mint :: proc(t: ^testing.T) {
	// INTEGRATION TEST: Test error handling for invalid mint address
	// Expected: Should return .TokenNotFound or .InvalidToken error

	invalid_mint := "InvalidMintAddress123"

	_, err := src.fetch_jupiter_price(invalid_mint)

	// Should get an error (either TokenNotFound or InvalidToken)
	testing.expect(t, err != .None,
		"Invalid mint should return an error")
}
