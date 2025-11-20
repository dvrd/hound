package jupiter_swap

import "core:fmt"
import "core:log"
import "core:time"

// Quote cache with 90-second TTL (Jupiter quote validity period)
// Reference: PRPs/ai_docs/jupiter-api-v6.md (Quote expiry section)
//
// CRITICAL: Jupiter quotes expire after ~90 seconds. Cache must respect this.
// Pattern: Similar to jupiter_client.odin:217-241 (60s price cache)

QuoteCacheEntry :: struct {
	quote:      JupiterQuote,
	cache_key:  string, // Composite key: "input_mint:output_mint:amount"
	cached_at:  time.Time,
	is_valid:   bool,
}

// Global quote cache (single entry for simplicity, extend to map for multi-quote support)
g_quote_cache: QuoteCacheEntry

// Cache TTL: 90 seconds (quote validity period)
// Reference: PRPs/ai_docs/jupiter-api-v6.md
QUOTE_CACHE_TTL :: 90 * time.Second

// Check if cached quote is still valid
//
// Returns: true if cache exists and is not stale (< 90 seconds old)
is_cache_valid :: proc(entry: QuoteCacheEntry, key: string) -> bool {
	if !entry.is_valid {
		return false
	}

	if entry.cache_key != key {
		return false
	}

	elapsed := time.since(entry.cached_at)
	if elapsed > QUOTE_CACHE_TTL {
		log.debugf("Quote cache stale (age: %.1f seconds)", time.duration_seconds(elapsed))
		return false
	}

	log.debugf("Quote cache hit (age: %.1f seconds)", time.duration_seconds(elapsed))
	return true
}

// Cache a quote for future use
//
// Parameters:
//   - quote: Jupiter quote to cache
//
// Side effect: Updates global cache entry
cache_quote :: proc(quote: JupiterQuote) {
	cache_key := build_cache_key(quote.input_mint, quote.output_mint, quote.in_amount)

	g_quote_cache = QuoteCacheEntry {
		quote     = quote,
		cache_key = cache_key,
		cached_at = time.now(),
		is_valid  = true,
	}

	log.debugf("Cached quote: %s", cache_key)
}

// Retrieve cached quote if available and valid
//
// Parameters:
//   - input_mint: Token mint to swap from
//   - output_mint: Token mint to swap to
//   - amount: Amount in base units
//
// Returns: (quote, true) if cache hit, (empty, false) if cache miss
get_cached_quote :: proc(
	input_mint: string,
	output_mint: string,
	amount: u64,
) -> (
	JupiterQuote,
	bool,
) {
	// fmt.tprintf uses temp allocator - no manual cleanup needed
	amount_str := fmt.tprintf("%d", amount)
	cache_key := build_cache_key(input_mint, output_mint, amount_str)

	if is_cache_valid(g_quote_cache, cache_key) {
		log.debug("Returning cached quote")
		return g_quote_cache.quote, true
	}

	log.debug("Cache miss or stale")
	return {}, false
}

// Build composite cache key from quote parameters
//
// Format: "input_mint:output_mint:amount"
// Example: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v:So11111111111111111111111111111111111111112:100000000"
build_cache_key :: proc(input_mint: string, output_mint: string, amount: string) -> string {
	return fmt.tprintf("%s:%s:%s", input_mint, output_mint, amount)
}

// Invalidate current cache (useful for testing or forcing refresh)
invalidate_cache :: proc() {
	g_quote_cache.is_valid = false
	log.debug("Quote cache invalidated")
}

// Get cache statistics (useful for debugging/monitoring)
CacheStats :: struct {
	is_valid:   bool,
	age:        time.Duration,
	cache_key:  string,
}

get_cache_stats :: proc() -> CacheStats {
	age := time.Duration(0)
	if g_quote_cache.is_valid {
		age = time.since(g_quote_cache.cached_at)
	}

	return CacheStats {
		is_valid  = g_quote_cache.is_valid,
		age       = age,
		cache_key = g_quote_cache.cache_key,
	}
}
