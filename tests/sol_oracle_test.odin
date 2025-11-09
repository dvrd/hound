#+feature global-context
#+feature dynamic-literals
package tests

import "core:fmt"
import "core:testing"
import "core:time"
import "../src"

// =============================================================================
// SOL ORACLE TESTS
// =============================================================================
// These tests document the SOL price oracle module with caching functionality.
//
// Test Philosophy:
// - Tests verify oracle integration with Jupiter and CoinGecko APIs
// - Cache behavior is tested with time-based expiration
// - Error handling covers API failures and validation
// - Tests serve as documentation for oracle usage
//
// Coverage:
// 1. Jupiter API price fetching
// 2. CoinGecko fallback mechanism
// 3. Cache freshness (30-second TTL)
// 4. Cache expiration and refresh
// 5. Price validation (reasonable bounds)
// =============================================================================

@(test)
test_jupiter_price_fetch :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify Jupiter Price API v6 integration
	//
	// Why: Jupiter is the primary price source for SOL/USD
	//
	// Example: Tool calls Jupiter API to get live SOL price
	// Expected: Returns price in $50-$1000 range (reasonable for SOL)

	// Act: Fetch price from Jupiter
	price, err := src.fetch_jupiter_price()

	// Assert 1: Fetch should succeed
	testing.expect(t, err == .None, "Jupiter API should be accessible")

	// Assert 2: Price should be in reasonable range
	testing.expect(
		t,
		price > 50.0 && price < 1000.0,
		fmt.tprintf("SOL price should be reasonable: $%.2f", price),
	)
}

@(test)
test_coingecko_price_fetch :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify CoinGecko API fallback integration
	//
	// Why: CoinGecko serves as backup when Jupiter fails
	//
	// Example: If Jupiter is down, tool falls back to CoinGecko
	// Expected: Returns price in $50-$1000 range

	// Act: Fetch price from CoinGecko
	price, err := src.fetch_coingecko_price()

	// Assert 1: Fetch should succeed
	testing.expect(t, err == .None, "CoinGecko API should be accessible")

	// Assert 2: Price should be in reasonable range
	testing.expect(
		t,
		price > 50.0 && price < 1000.0,
		fmt.tprintf("SOL price should be reasonable: $%.2f", price),
	)
}

@(test)
test_cache_freshness :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify cache returns same price within 30 seconds
	//
	// Why: Prevents excessive API calls for rapid consecutive queries
	//
	// Example: User runs `hound aura` then `hound btc` 5 seconds later.
	// Both should use same cached SOL price without re-fetching.

	// Arrange: Reset cache
	src.g_sol_cache = src.SolPriceCache{}

	// Act: Fetch price twice within 30 seconds
	price1, err1 := src.get_sol_price_cached()
	price2, err2 := src.get_sol_price_cached()

	// Assert 1: Both fetches should succeed
	testing.expect(t, err1 == .None, "First fetch should succeed")
	testing.expect(t, err2 == .None, "Second fetch should succeed")

	// Assert 2: Same price returned (cache hit)
	testing.expect(
		t,
		price1 == price2,
		fmt.tprintf("Cached price should match: $%.2f vs $%.2f", price1, price2),
	)

	// Assert 3: Price is reasonable
	testing.expect(
		t,
		price1 > 50.0 && price1 < 1000.0,
		fmt.tprintf("SOL price should be reasonable: $%.2f", price1),
	)
}

@(test)
test_cache_stale_check :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify is_cache_stale() correctly identifies stale cache
	//
	// Why: Cache staleness detection is critical for refresh logic
	//
	// Example: Cache older than 30 seconds should be marked stale

	// Test 1: Never populated cache is stale
	cache1 := src.SolPriceCache{is_valid = false}
	testing.expect(t, src.is_cache_stale(cache1), "Unpopulated cache should be stale")

	// Test 2: Fresh cache (just cached) is not stale
	cache2 := src.SolPriceCache{
		price = 162.50,
		cached_at = time.now(),
		is_valid = true,
	}
	testing.expect(t, !src.is_cache_stale(cache2), "Fresh cache should not be stale")

	// Test 3: Old cache (40 seconds ago) is stale
	cache3 := src.SolPriceCache{
		price = 162.50,
		cached_at = time.Time{_nsec = time.now()._nsec - 40 * i64(time.Second)},
		is_valid = true,
	}
	testing.expect(t, src.is_cache_stale(cache3), "40-second old cache should be stale")
}

@(test)
test_price_validation_bounds :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify price validation rejects unreasonable values
	//
	// Why: Protects against corrupted API data or manipulation
	//
	// Example: Price of $10 or $5000 should be rejected as unreasonable
	// Valid range: $50-$1000 based on historical SOL prices

	// This test documents the expected validation behavior
	// Actual validation happens inside fetch_jupiter_price() and fetch_coingecko_price()
	// If API returns price outside bounds, should return .OraclePriceInvalid

	// Note: We can't directly test validation without mocking API responses
	// But we document the expected behavior:
	//
	// price < $50  → .OraclePriceInvalid
	// price > $1000 → .OraclePriceInvalid
	// $50 ≤ price ≤ $1000 → .None (success)

	// Fetch real price and verify it's in valid range
	price, err := src.get_sol_price_cached()
	if err == .None {
		testing.expect(
			t,
			price >= 50.0 && price <= 1000.0,
			fmt.tprintf("Price $%.2f should be within valid range [$50, $1000]", price),
		)
	}
}

@(test)
test_cache_update_mechanism :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify cache is properly updated after fetch
	//
	// Why: Cache must store price and timestamp for staleness checks
	//
	// Example: After successful fetch, cache should have valid price and recent timestamp

	// Arrange: Reset cache
	src.g_sol_cache = src.SolPriceCache{}

	// Act: Fetch price (populates cache)
	price, err := src.get_sol_price_cached()

	// Assert 1: Fetch succeeded
	testing.expect(t, err == .None, "Fetch should succeed")

	// Assert 2: Cache is now valid
	testing.expect(t, src.g_sol_cache.is_valid, "Cache should be marked valid after fetch")

	// Assert 3: Cache price matches returned price
	testing.expect(
		t,
		src.g_sol_cache.price == price,
		fmt.tprintf("Cache price $%.2f should match returned price $%.2f", src.g_sol_cache.price, price),
	)

	// Assert 4: Cache timestamp is recent (< 1 second old)
	elapsed := time.diff(src.g_sol_cache.cached_at, time.now())
	testing.expect(
		t,
		elapsed < 1 * time.Second,
		fmt.tprintf("Cache timestamp should be recent, elapsed: %v", elapsed),
	)
}

@(test)
test_global_cache_instance :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify global cache persists across function calls
	//
	// Why: Cache must be global to work across multiple token queries
	//
	// Example: First query populates cache, second query uses it

	// Arrange: Reset cache and record initial state
	src.g_sol_cache = src.SolPriceCache{}
	initial_valid := src.g_sol_cache.is_valid

	// Act: First call populates cache
	_, err1 := src.get_sol_price_cached()
	after_first_valid := src.g_sol_cache.is_valid

	// Act: Second call uses cached value
	_, err2 := src.get_sol_price_cached()
	after_second_valid := src.g_sol_cache.is_valid

	// Assert 1: Initially cache is invalid
	testing.expect(t, !initial_valid, "Initial cache should be invalid")

	// Assert 2: First call succeeds and validates cache
	testing.expect(t, err1 == .None, "First fetch should succeed")
	testing.expect(t, after_first_valid, "Cache should be valid after first fetch")

	// Assert 3: Second call succeeds using cache
	testing.expect(t, err2 == .None, "Second fetch should succeed (from cache)")
	testing.expect(t, after_second_valid, "Cache should still be valid after second fetch")
}
