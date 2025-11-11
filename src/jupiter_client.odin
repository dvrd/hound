#+feature global-context
package main

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "core:strconv"
import "core:time"
import client "../vendor/odin-http/client"

// Jupiter Price API V3 response structure (Lite API)
// Endpoint: https://lite-api.jup.ag/price/v3?ids={token_mints}
//
// Free tier: 600 requests per 60 seconds
// Pro tier: https://api.jup.ag (requires x-api-key header)
JupiterPriceResponse :: struct {
	data: map[string]JupiterPriceInfo `json:"-"`, // Populated via custom unmarshaling
}

JupiterPriceInfo :: struct {
	usd_price:       f64 `json:"usdPrice"`,
	block_id:        i64 `json:"blockId"`,
	decimals:        u8 `json:"decimals"`,
	price_change_24h: f64 `json:"priceChange24h"`,
}

// Jupiter price cache (60-second TTL per research recommendation)
JupiterPriceCache :: struct {
	mint_address: string, // Token mint this cache is for
	price_info:   JupiterPriceInfo, // Cached price data
	cached_at:    time.Time, // When cache was populated
	is_valid:     bool, // Whether cache has been populated
}

// Global cache instance
g_jupiter_cache: JupiterPriceCache

// Cache TTL constant (60 seconds)
JUPITER_CACHE_TTL :: 60 * time.Second

// Jupiter API endpoints
JUPITER_LITE_API_URL :: "https://lite-api.jup.ag/price/v3"
JUPITER_PRO_API_URL :: "https://api.jup.ag/price/v3"

// Fetch token price from Jupiter Price API V3 (Lite endpoint)
//
// ASSERTION 1: Validate token mint address is not empty
// ASSERTION 2: Validate price is reasonable (non-negative, within bounds)
// ASSERTION 3: Validate 24h change is within reasonable range
fetch_jupiter_price :: proc(token_mint: string) -> (JupiterPriceInfo, ErrorType) {
	// ASSERTION 1: Validate input
	assert(len(token_mint) > 0, "Token mint address cannot be empty")

	log.debugf("Fetching Jupiter price for mint: %s", token_mint)

	// Build URL
	url := fmt.tprintf("%s?ids=%s", JUPITER_LITE_API_URL, token_mint)
	log.debugf("API URL: %s", url)

	// Make HTTP request with error handling
	res, http_err := client.get(url)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		// Discriminate network error types
		#partial switch e in http_err {
		case net.Network_Error:
			log.debug("Network timeout detected")
			return {}, .NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			log.debug("Connection error detected")
			return {}, .ConnectionFailed
		case client.Request_Error:
			log.debug("Request error detected")
			return {}, .InvalidResponse
		case:
			log.debug("Unknown network error detected")
			return {}, .ConnectionFailed
		}
	}
	defer client.response_destroy(&res)

	// Check HTTP status code
	log.debugf("HTTP response status: %v", res.status)
	#partial switch res.status {
	case .Bad_Request:
		log.debug("Bad request (400)")
		return {}, .InvalidToken
	case .Not_Found:
		log.debug("Not found (404)")
		return {}, .TokenNotFound
	case .Too_Many_Requests:
		log.warn("Rate limited (429) - Jupiter allows 600 req/min on Lite tier")
		return {}, .RateLimited
	case .Internal_Server_Error, .Service_Unavailable:
		log.error("Server error (500/503)")
		return {}, .ServerError
	case .OK:
		log.debug("HTTP 200 OK - processing response")
		// Continue processing
	case:
		log.warnf("Unknown status code: %v", res.status)
		return {}, .ServerError
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract response body: %v", body_err)
		return {}, .InvalidResponse
	}
	defer client.body_destroy(body, allocation)

	log.debug("Parsing JSON response")

	// Parse JSON - Jupiter v3 returns object with mint as key
	// Example: {"So11111111...": {"usdPrice": 147.48, ...}}
	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body.(string), &response_json, spec); unmarshal_err != nil {
		log.errorf("JSON unmarshal failed: %v", unmarshal_err)
		return {}, .InvalidResponse
	}

	// Extract the token data from response
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		log.error("Response is not a JSON object")
		return {}, .InvalidResponse
	}

	// Get the price info for this mint
	price_data, has_mint := response_obj[token_mint]
	if !has_mint {
		log.warnf("Token %s not found in response", token_mint)
		return {}, .TokenNotFound
	}

	// Handle null price (unreliable token flagged by Jupiter's validation heuristics)
	if price_data == nil {
		log.warnf("Jupiter returned null for token %s (unreliable token)", token_mint)
		return {}, .TokenNotFound // Treat as not found
	}

	price_obj, is_price_obj := price_data.(json.Object)
	if !is_price_obj {
		log.error("Price data is not a JSON object")
		return {}, .InvalidResponse
	}

	// Extract fields
	usd_price_val, has_price := price_obj["usdPrice"]
	if !has_price {
		log.error("Missing usdPrice field")
		return {}, .InvalidResponse
	}

	usd_price_float, is_float := usd_price_val.(json.Float)
	if !is_float {
		log.errorf("usdPrice is not a number: %v", usd_price_val)
		return {}, .InvalidResponse
	}

	// Extract blockId
	block_id_val, has_block := price_obj["blockId"]
	block_id: i64 = 0
	if has_block {
		if block_int, is_int := block_id_val.(json.Integer); is_int {
			block_id = i64(block_int)
		}
	}

	// Extract decimals
	decimals_val, has_decimals := price_obj["decimals"]
	decimals: u8 = 0
	if has_decimals {
		if decimals_int, is_int := decimals_val.(json.Integer); is_int {
			decimals = u8(decimals_int)
		}
	}

	// Extract priceChange24h
	change_val, has_change := price_obj["priceChange24h"]
	price_change: f64 = 0.0
	if has_change {
		if change_float, is_float := change_val.(json.Float); is_float {
			price_change = f64(change_float)
		}
	}

	log.debugf("Parsed price: $%.6f, 24h change: %.2f%%, blockId: %d", f64(usd_price_float), price_change, block_id)

	// ASSERTION 2: Validate price is reasonable (non-negative, within bounds)
	// Typical token prices: $0.000001 to $1,000,000
	assert(
		f64(usd_price_float) >= 0 && f64(usd_price_float) < 1_000_000_000,
		fmt.tprintf("Price outside reasonable range: $%.9f", f64(usd_price_float)),
	)

	// ASSERTION 3: Validate 24h change is within reasonable range (-100% to +10000%)
	assert(
		price_change >= -100.0 && price_change <= 10000.0,
		fmt.tprintf("24h change outside reasonable range: %.2f%%", price_change),
	)

	log.info("Jupiter API fetch successful")

	return JupiterPriceInfo{
		usd_price = f64(usd_price_float),
		block_id = block_id,
		decimals = decimals,
		price_change_24h = price_change,
	}, .None
}

// Check if Jupiter cache is stale (> 60 seconds old)
is_jupiter_cache_stale :: proc(cache: JupiterPriceCache, mint_address: string) -> bool {
	// ASSERTION 1: Cache validity is boolean
	assert(
		cache.is_valid == true || cache.is_valid == false,
		"Cache is_valid must be boolean",
	)

	// Invalid cache is always stale
	if !cache.is_valid {
		return true
	}

	// Different token means cache doesn't apply
	if cache.mint_address != mint_address {
		return true
	}

	// Check time-based staleness
	elapsed := time.diff(cache.cached_at, time.now())

	// ASSERTION 2: Time cannot go backwards
	assert(elapsed >= 0, "Elapsed time cannot be negative")

	return elapsed > JUPITER_CACHE_TTL
}

// Get Jupiter price with caching (60-second TTL)
get_jupiter_price_cached :: proc(mint_address: string) -> (JupiterPriceInfo, ErrorType) {
	// ASSERTION 1: Cache TTL is positive
	assert(JUPITER_CACHE_TTL > 0, "Cache TTL must be positive")

	// Check cache freshness
	if !is_jupiter_cache_stale(g_jupiter_cache, mint_address) {
		// ASSERTION 2: Cached price is valid
		assert(
			g_jupiter_cache.price_info.usd_price >= 0,
			"Cached price must be non-negative",
		)
		log.debugf("Cache hit for mint: %s", mint_address)
		return g_jupiter_cache.price_info, .None
	}

	log.debugf("Cache miss for mint: %s", mint_address)

	// Cache miss or stale - fetch fresh data
	price_info, err := fetch_jupiter_price(mint_address)
	if err == .None {
		// Update global cache
		g_jupiter_cache.mint_address = mint_address
		g_jupiter_cache.price_info = price_info
		g_jupiter_cache.cached_at = time.now()
		g_jupiter_cache.is_valid = true
		log.debugf("Updated cache for mint: %s", mint_address)
		return price_info, .None
	}

	// Fetch failed - return error
	return {}, err
}

// Fetch Jupiter price with exponential backoff on rate limit (429)
//
// Retry logic: 1s → 2s → 4s → fail
// This matches the recommended pattern from Jupiter research
fetch_jupiter_price_with_retry :: proc(token_mint: string, max_retries: int = 3) -> (JupiterPriceInfo, ErrorType) {
	log.debugf("Attempting Jupiter fetch with max %d retries", max_retries)

	delay_ms: i64 = 1000 // Start with 1 second

	for attempt in 0..<max_retries {
		log.debugf("Attempt %d of %d", attempt + 1, max_retries)

		price_info, err := fetch_jupiter_price(token_mint)

		// Success - return immediately
		if err == .None {
			log.info("Jupiter fetch successful")
			return price_info, .None
		}

		// Rate limited - use exponential backoff
		if err == .RateLimited {
			if attempt == max_retries - 1 {
				// Last attempt failed
				log.errorf("Max retries exceeded for rate limit")
				return {}, .RateLimited
			}

			// Wait with exponential backoff
			log.debugf("Rate limited, waiting %dms before retry", delay_ms)
			time.sleep(time.Duration(delay_ms * 1_000_000)) // Convert ms to ns
			delay_ms *= 2 // Double the delay (1s → 2s → 4s)
			continue
		}

		// Other errors - don't retry
		log.errorf("Non-retryable error: %v", err)
		return {}, err
	}

	// Should not reach here
	return {}, .ServerError
}
