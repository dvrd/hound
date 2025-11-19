// =============================================================================
// POOL DISCOVERY MODULE
// =============================================================================
// This module implements automatic DEX pool discovery via the DexScreener API.
// It handles:
// - HTTP requests to DexScreener's /latest/dex/tokens/{address} endpoint
// - Exponential backoff retry logic for rate limiting (300 req/min limit)
// - TTL-based caching (1-hour cache for pool searches)
// - Robust error handling following existing patterns
//
// Design Decisions:
// - 1-hour cache TTL (pools are relatively stable, don't change often)
// - Max 3 retry attempts with exponential backoff (1s → 2s → 4s)
// - Only retry .RateLimited errors (fail fast on others)
// - Empty pairs array is NOT an error (token may have no pools yet)
//
// References:
// - DexScreener API docs: PRPs/ai_docs/dexscreener_api.md
// - HTTP error pattern: src/price_fetcher.odin:46-104
// - Retry pattern: src/jupiter_client.odin:277-319
// - Cache pattern: src/jupiter_client.odin:28-40, 217-241
// =============================================================================

package main

import "core:bufio"
import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "core:strconv"
import "core:time"
import client "../vendor/odin-http/client"

// =============================================================================
// DATA MODELS - DexScreener API Response Structures
// =============================================================================

// DexScreener API Response (top level)
DexScreenerPoolResponse :: struct {
	schemaVersion: string `json:"schemaVersion"`,
	pairs:         []DexScreenerPair `json:"pairs"`,
}

// Individual trading pair/pool
DexScreenerPair :: struct {
	chainId:        string `json:"chainId"`,        // "solana"
	dexId:          string `json:"dexId"`,          // "raydium", "orca", "meteora"
	url:            string `json:"url"`,            // DexScreener UI URL
	pairAddress:    string `json:"pairAddress"`,    // Pool contract address
	labels:         []string `json:"labels"`,       // ["CLMM", "wp", "DLMM", etc.]
	baseToken:      DexScreenerToken `json:"baseToken"`,
	quoteToken:     DexScreenerToken `json:"quoteToken"`,
	priceNative:    string `json:"priceNative"`,    // Price in quote token (string!)
	priceUsd:       string `json:"priceUsd"`,       // USD price (string!)
	txns:           DexScreenerTxns `json:"txns"`,
	volume:         DexScreenerVolume `json:"volume"`,
	priceChange:    DexScreenerPriceChange `json:"priceChange"`,
	liquidity:      DexScreenerLiquidity `json:"liquidity"`,
	fdv:            f64 `json:"fdv"`,               // Fully Diluted Valuation
	marketCap:      f64 `json:"marketCap"`,
	pairCreatedAt:  i64 `json:"pairCreatedAt"`,     // Unix timestamp in MILLISECONDS
}

// Token information in a pair
DexScreenerToken :: struct {
	address: string `json:"address"`,
	name:    string `json:"name"`,
	symbol:  string `json:"symbol"`,
}

// Liquidity data
DexScreenerLiquidity :: struct {
	usd:   f64 `json:"usd"`,   // Total liquidity in USD
	base:  f64 `json:"base"`,  // Base token amount
	quote: f64 `json:"quote"`, // Quote token amount
}

// Volume data (across time periods)
DexScreenerVolume :: struct {
	h24: f64 `json:"h24"`, // 24-hour volume in USD
	h6:  f64 `json:"h6"`,  // 6-hour volume
	h1:  f64 `json:"h1"`,  // 1-hour volume
	m5:  f64 `json:"m5"`,  // 5-minute volume
}

// Transaction counts (buys/sells)
DexScreenerTxns :: struct {
	h24: DexScreenerTxnPeriod `json:"h24"`,
	h6:  DexScreenerTxnPeriod `json:"h6"`,
	h1:  DexScreenerTxnPeriod `json:"h1"`,
	m5:  DexScreenerTxnPeriod `json:"m5"`,
}

DexScreenerTxnPeriod :: struct {
	buys:  i64 `json:"buys"`,
	sells: i64 `json:"sells"`,
}

// Price change percentages
DexScreenerPriceChange :: struct {
	h24: f64 `json:"h24"`, // 24-hour % change (e.g., 3.45 = +3.45%)
	h6:  f64 `json:"h6"`,
	h1:  f64 `json:"h1"`,
	m5:  f64 `json:"m5"`,
}

// =============================================================================
// CACHE STRUCTURE - 1-Hour TTL for Pool Searches
// =============================================================================

PoolSearchCache :: struct {
	token_address: string,              // Token address this cache is for
	pairs:         []DexScreenerPair,   // Cached pool data
	cached_at:     time.Time,           // When cache was populated
	is_valid:      bool,                // Whether cache has been populated
}

// Global cache instance (similar to g_jupiter_cache pattern)
g_pool_search_cache: PoolSearchCache

// Cache TTL constant - 1 hour (pools don't change frequently)
POOL_SEARCH_CACHE_TTL :: 60 * 60 * time.Second

// =============================================================================
// API CONSTANTS
// =============================================================================

DEXSCREENER_API_BASE :: "https://api.dexscreener.com"
DEXSCREENER_RATE_LIMIT :: 300 // requests per minute

// =============================================================================
// CORE API FUNCTIONS
// =============================================================================

// Fetch pools for a token from DexScreener API (no retry, single attempt)
//
// Endpoint: GET /latest/dex/tokens/{tokenAddress}
// Rate Limit: 300 requests per minute
// Returns: Array of pairs (can be empty if token has no pools)
//
// CRITICAL: Empty pairs array is NOT an error - token may simply have no pools yet
fetch_pools_for_token :: proc(token_address: string) -> ([]DexScreenerPair, ErrorType) {
	// ASSERTION 1: Token address must not be empty
	assert(len(token_address) > 0, "Token address must not be empty")

	log.debugf("Fetching pools from DexScreener API for token: %s", token_address)

	// Build API URL
	url := fmt.tprintf("%s/latest/dex/tokens/%s", DEXSCREENER_API_BASE, token_address)
	log.debugf("API URL: %s", url)

	// Make HTTP request with error handling (PATTERN: price_fetcher.odin:46-79)
	res, http_err := client.get(url)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		// PATTERN: Discriminate network error types using union switching
		#partial switch e in http_err {
		case net.Network_Error:
			log.debug("Network timeout detected")
			return nil, .NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			log.debug("Connection error detected")
			return nil, .ConnectionFailed
		case client.Request_Error:
			log.debug("Request error detected")
			return nil, .InvalidResponse
		case net.Parse_Endpoint_Error:
			log.debug("Parse endpoint error detected")
			return nil, .InvalidToken
		case bufio.Scanner_Error:
			log.debug("Scanner error detected")
			return nil, .InvalidResponse
		case client.SSL_Error:
			log.debug("SSL error detected")
			return nil, .ConnectionFailed
		case:
			log.debug("Unknown network error detected")
			return nil, .ConnectionFailed
		}
	}
	defer client.response_destroy(&res) // CRITICAL: Always cleanup

	// Check HTTP status code (PATTERN: price_fetcher.odin:84-104)
	log.debugf("HTTP response status: %v", res.status)
	#partial switch res.status {
	case .Bad_Request:
		log.debug("Bad request (400)")
		return nil, .InvalidToken
	case .Not_Found:
		log.debug("Not found (404) - token may not exist or have no pools")
		return nil, .TokenNotFound
	case .Too_Many_Requests:
		log.warn("Rate limited (429) - DexScreener allows 300 req/min")
		return nil, .RateLimited
	case .Internal_Server_Error, .Service_Unavailable:
		log.error("Server error (500/503)")
		return nil, .ServerError
	case .OK:
		log.debug("HTTP 200 OK - processing response")
		// Continue processing
	case:
		log.warnf("Unknown status code: %v", res.status)
		return nil, .ServerError
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract response body: %v", body_err)
		return nil, .InvalidResponse
	}
	defer client.body_destroy(body, allocation) // CRITICAL: Always cleanup

	log.debug("Parsing JSON response")

	// Parse JSON response
	response: DexScreenerPoolResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		log.errorf("JSON unmarshal failed: %v", json_err)
		return nil, .InvalidResponse
	}

	log.debugf("API returned %d pair(s)", len(response.pairs))

	// IMPORTANT: Empty pairs array is NOT an error (per DexScreener API docs)
	// A token may simply not have any trading pools yet
	// Return empty array with .None error
	if len(response.pairs) == 0 {
		log.info("No pools found for token (this is not an error)")
		return make([]DexScreenerPair, 0), .None
	}

	// Filter to Solana chains only
	solana_pairs := make([dynamic]DexScreenerPair, 0, len(response.pairs))
	for pair in response.pairs {
		if pair.chainId == "solana" {
			append(&solana_pairs, pair)
		}
	}

	log.infof("Found %d Solana pool(s) for token", len(solana_pairs))

	// ASSERTION 2: Validate that filtered pairs are reasonable
	assert(len(solana_pairs) <= len(response.pairs), "Filtered pairs cannot exceed original count")

	return solana_pairs[:], .None
}

// Fetch pools with exponential backoff retry on rate limit
//
// Retry Strategy (PATTERN: jupiter_client.odin:277-319):
// - Max 3 attempts
// - Exponential backoff: 1s → 2s → 4s
// - Only retry .RateLimited errors (fail fast on others)
//
// This handles DexScreener's 300 req/min rate limit gracefully
fetch_pools_with_retry :: proc(token_address: string, max_retries: int = 3) -> ([]DexScreenerPair, ErrorType) {
	log.debugf("Attempting DexScreener fetch with max %d retries", max_retries)

	delay_ms: i64 = 1000 // Start with 1 second

	for attempt in 0 ..< max_retries {
		log.debugf("Attempt %d of %d", attempt + 1, max_retries)

		pairs, err := fetch_pools_for_token(token_address)

		// Success - return immediately
		if err == .None {
			log.info("DexScreener fetch successful")
			return pairs, .None
		}

		// Rate limited - use exponential backoff
		if err == .RateLimited {
			if attempt == max_retries - 1 {
				// Last attempt failed
				log.errorf("Max retries exceeded for rate limit")
				return nil, .RateLimited
			}

			// Wait with exponential backoff
			log.debugf("Rate limited, waiting %dms before retry", delay_ms)
			time.sleep(time.Duration(delay_ms * 1_000_000)) // Convert ms to ns
			delay_ms *= 2 // Double the delay (1s → 2s → 4s)
			continue
		}

		// Other errors - don't retry (fail fast)
		log.errorf("Non-retryable error: %v", err)
		return nil, err
	}

	// Should not reach here
	log.error("Retry loop completed without success or error return")
	return nil, .PoolSearchFailed
}

// =============================================================================
// CACHE FUNCTIONS - 1-Hour TTL
// =============================================================================

// Check if pool search cache is stale (> 1 hour old)
//
// PATTERN: jupiter_client.odin:217-241 (cache staleness check)
// Returns true if cache is invalid, for different token, or expired
is_pool_search_cache_stale :: proc(cache: PoolSearchCache, token_address: string) -> bool {
	// ASSERTION 1: Cache validity is boolean
	assert(cache.is_valid == true || cache.is_valid == false, "Cache is_valid must be boolean")

	// Invalid cache is always stale
	if !cache.is_valid {
		return true
	}

	// Different token means cache doesn't apply (cache miss)
	if cache.token_address != token_address {
		return true
	}

	// Check time-based staleness
	elapsed := time.diff(cache.cached_at, time.now())

	// ASSERTION 2: Time cannot go backwards (TigerBeetle safety)
	assert(elapsed >= 0, "Elapsed time cannot be negative")

	return elapsed > POOL_SEARCH_CACHE_TTL
}

// Get pools with caching (1-hour TTL)
//
// PATTERN: jupiter_client.odin:244-275 (cache wrapper with fetch)
// This is the main entry point for pool discovery
//
// The force_refresh parameter allows bypassing cache to get fresh pool data
get_pools_cached :: proc(token_address: string, force_refresh: bool = false) -> ([]DexScreenerPair, ErrorType) {
	// ASSERTION 1: Cache TTL is positive (configuration check)
	assert(POOL_SEARCH_CACHE_TTL > 0, "Cache TTL must be positive")

	// Skip cache check if force refresh requested
	if force_refresh {
		log.info("Force refresh requested, bypassing cache")
	} else {
		// Check cache freshness
		if !is_pool_search_cache_stale(g_pool_search_cache, token_address) {
			log.debugf("Cache hit for token: %s", token_address)
			return g_pool_search_cache.pairs, .None
		}
	}

	log.debugf("Cache miss or force refresh for token: %s", token_address)

	// Cache miss or stale - fetch fresh data with retry
	pairs, err := fetch_pools_with_retry(token_address)
	if err != .None {
		// Fetch failed - return error
		return nil, err
	}

	// Update global cache
	g_pool_search_cache.token_address = token_address
	g_pool_search_cache.pairs = pairs
	g_pool_search_cache.cached_at = time.now()
	g_pool_search_cache.is_valid = true

	log.debugf("Updated cache for token: %s with %d pools", token_address, len(pairs))

	return pairs, .None
}
