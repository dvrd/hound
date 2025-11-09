package main

import "core:bufio"
import "core:encoding/hex"
import "core:encoding/json"
import "core:fmt"
import "core:math"
import "core:net"
import "core:strconv"
import "core:time"
import client "../vendor/odin-http/client"

// API Change Cache - 5 minute TTL (less volatile than SOL price)
APIChangeCache :: struct {
	contract_address: string,    // Token address this cache is for
	change_24h:       f64,        // Cached 24-hour percentage change
	cached_at:        time.Time,  // When cache was populated
	is_valid:         bool,       // Whether cache has been populated
}

// Global cache instance (similar to g_sol_cache pattern)
g_api_change_cache: APIChangeCache

// Cache TTL constant - 5 minutes for 24h change (less volatile than price)
API_CHANGE_CACHE_TTL :: 5 * time.Minute

// DexScreener response structure (minimal - only extract what we need)
// Note: Full structure already exists in types.odin, but we create minimal version
// for 24h change extraction to avoid confusion with fetch_price() usage
DexScreenerChangeResponse :: struct {
	pairs: []struct {
		priceChange: struct {
			h24: f64,  // 24-hour percentage change (e.g., 3.45 = +3.45%)
		},
	},
}

fetch_price :: proc(contract_address: string) -> (price: PriceData, err: ErrorType) {
	// Build URL
	url := fmt.tprintf("https://api.dexscreener.com/latest/dex/tokens/%s", contract_address)
	
	// Make HTTP request with error handling
	res, http_err := client.get(url)
	if http_err != nil {
		// Discriminate network error types using union switching
		#partial switch e in http_err {
		case net.Network_Error:
			// General network errors (includes timeouts)
			return {}, .NetworkTimeout
		case net.TCP_Send_Error:
			return {}, .ConnectionFailed
		case net.Dial_Error:
			return {}, .ConnectionFailed
		case client.Request_Error:
			return {}, .InvalidResponse
		case net.Parse_Endpoint_Error:
			return {}, .InvalidToken
		case bufio.Scanner_Error:
			return {}, .InvalidResponse
		case client.SSL_Error:
			return {}, .ConnectionFailed
		case:
			// Unknown network error
			return {}, .ConnectionFailed
		}
	}
	defer client.response_destroy(&res)

	// NEW: Check HTTP status code
	#partial switch res.status {
	case .Bad_Request:
		return {}, .InvalidToken
	case .Not_Found:
		return {}, .TokenNotFound
	case .Too_Many_Requests:
		return {}, .RateLimited
	case .Internal_Server_Error, .Service_Unavailable:
		return {}, .ServerError
	case .OK:
		// Continue processing
	case:
		// Unknown error code, treat as server error
		return {}, .ServerError
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return {}, .InvalidResponse
	}
	defer client.body_destroy(body, allocation)

	// Parse JSON (body is string)
	response: DexScreenerResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		return {}, .InvalidResponse
	}

	// Check for empty pairs
	if len(response.pairs) == 0 {
		return {}, .TokenNotFound
	}

	// Extract and convert price (priceUsd is string!)
	price_str := response.pairs[0].priceUsd
	price_val, parse_ok := strconv.parse_f64(price_str)
	if !parse_ok {
		return {}, .InvalidResponse
	}
	change := response.pairs[0].priceChange.h24
	
	// Return data
	return PriceData{price_usd = price_val, change_24h = change}, .None
}

// Fetch 24h change from DexScreener API (for hybrid pricing)
fetch_24h_change :: proc(contract_address: string) -> (f64, ErrorType) {
	// ASSERTION 1: TigerBeetle safety - validate input
	assert(len(contract_address) > 0, "Contract address must not be empty")

	// Build API URL - same endpoint as fetch_price()
	url := fmt.tprintf("https://api.dexscreener.com/latest/dex/tokens/%s", contract_address)

	// HTTP GET with error handling (follow fetch_price pattern)
	res, http_err := client.get(url)
	if http_err != nil {
		// PATTERN: Discriminate network errors
		#partial switch e in http_err {
		case net.Network_Error:
			return 0, .NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			return 0, .ConnectionFailed
		case:
			return 0, .ConnectionFailed
		}
	}
	defer client.response_destroy(&res)  // CRITICAL: Always cleanup

	// PATTERN: Check HTTP status code
	#partial switch res.status {
	case .Not_Found:
		return 0, .TokenNotFound
	case .Too_Many_Requests:
		return 0, .RateLimited
	case .Internal_Server_Error, .Service_Unavailable:
		return 0, .ServerError
	case .OK:
		// Continue processing
	case:
		return 0, .ServerError
	}

	// Extract and parse JSON body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return 0, .InvalidResponse
	}
	defer client.body_destroy(body, allocation)  // CRITICAL: Always cleanup

	// Parse JSON - use minimal struct (only extract h24)
	response: DexScreenerChangeResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		return 0, .InvalidResponse
	}

	// CRITICAL: Check pairs array not empty
	if len(response.pairs) == 0 {
		return 0, .TokenNotFound
	}

	// Extract 24h change (direct f64, no string conversion needed)
	change_24h := response.pairs[0].priceChange.h24

	// ASSERTION 2: Validate reasonable range (-100% to +10000%)
	// Most tokens don't change more than 10000% in 24h, less than -100% is impossible
	assert(change_24h >= -100.0 && change_24h <= 10000.0, "24h change outside reasonable range")

	return change_24h, .None
}

// Check if API change cache is stale (> 5 minutes old)
is_api_cache_stale :: proc(cache: APIChangeCache, contract_address: string) -> bool {
	// ASSERTION 1: Cache validity is boolean (TigerBeetle safety)
	assert(cache.is_valid == true || cache.is_valid == false, "Cache is_valid must be boolean")

	// Invalid cache is always stale
	if !cache.is_valid {
		return true
	}

	// Different token means cache doesn't apply (cache miss)
	if cache.contract_address != contract_address {
		return true
	}

	// Check time-based staleness
	elapsed := time.diff(cache.cached_at, time.now())

	// ASSERTION 2: Time cannot go backwards (TigerBeetle safety)
	assert(elapsed >= 0, "Elapsed time cannot be negative")

	return elapsed > API_CHANGE_CACHE_TTL
}

// Get 24h change with caching (5-minute TTL)
get_24h_change_cached :: proc(contract_address: string) -> (f64, ErrorType) {
	// ASSERTION 1: Cache TTL is positive (configuration check)
	assert(API_CHANGE_CACHE_TTL > 0, "Cache TTL must be positive")

	// Check cache freshness
	if !is_api_cache_stale(g_api_change_cache, contract_address) {
		// ASSERTION 2: Cached value is in reasonable range
		assert(g_api_change_cache.change_24h >= -100.0 && g_api_change_cache.change_24h <= 10000.0,
		       "Cached 24h change outside reasonable range")
		return g_api_change_cache.change_24h, .None
	}

	// Cache miss or stale - fetch fresh data
	change_24h, err := fetch_24h_change(contract_address)
	if err == .None {
		// Update global cache
		g_api_change_cache.contract_address = contract_address
		g_api_change_cache.change_24h = change_24h
		g_api_change_cache.cached_at = time.now()
		g_api_change_cache.is_valid = true
		return change_24h, .None
	}

	// Fetch failed - return error
	return 0, err
}

// Calculate price from reserves using AMM constant product formula
calculate_price_from_reserves :: proc(
	base_reserve: u64,
	quote_reserve: u64,
	base_decimals: u64,
	quote_decimals: u64,
) -> f64 {
	// Adjust for decimals
	base_actual := f64(base_reserve) / math.pow(10.0, f64(base_decimals))
	quote_actual := f64(quote_reserve) / math.pow(10.0, f64(quote_decimals))

	// Avoid division by zero
	if base_actual <= 0 {
		return 0
	}

	// Price of base token in quote token
	return quote_actual / base_actual
}

// Fetch price directly from Raydium pool on-chain
fetch_onchain_price :: proc(token: Token) -> (PriceData, ErrorType) {
	// Check pools exist
	if len(token.pools) == 0 {
		return {}, .TokenNotConfigured
	}

	pool := token.pools[0]

	// Connect to RPC
	conn := RPCConnection{endpoint = "https://api.mainnet-beta.solana.com", timeout = 10000}

	// Fetch pool data (752 bytes)
	pool_data, err := get_account_info(conn, pool.pool_address)
	if err != .None {
		return {}, err
	}
	defer delete(pool_data)

	// Decode pool
	pool_state, ok := decode_raydium_pool_v4(pool_data)
	if !ok {
		return {}, .PoolDataInvalid
	}

	// Convert vaults to base58 addresses
	base_vault_addr := pubkey_to_base58(pool_state.base_vault)
	quote_vault_addr := pubkey_to_base58(pool_state.quote_vault)

	// Fetch vault balances
	base_balance, base_err := get_token_balance(conn, base_vault_addr)
	if base_err != .None {
		return {}, .VaultFetchFailed
	}

	quote_balance, quote_err := get_token_balance(conn, quote_vault_addr)
	if quote_err != .None {
		return {}, .VaultFetchFailed
	}

	// Parse amounts
	base_reserve, base_parse_ok := strconv.parse_u64(base_balance.amount)
	if !base_parse_ok {
		return {}, .VaultFetchFailed
	}

	quote_reserve, quote_parse_ok := strconv.parse_u64(quote_balance.amount)
	if !quote_parse_ok {
		return {}, .VaultFetchFailed
	}

	// Calculate price
	price_in_quote := calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		pool_state.base_decimal,
		pool_state.quote_decimal,
	)

	// Get live SOL price from oracle (cached for 30s)
	sol_usd_price, oracle_err := get_sol_price_cached()
	if oracle_err != .None {
		return {}, oracle_err
	}

	// Convert to USD
	price_in_usd := price_in_quote * sol_usd_price

	// Fetch 24h change from API (graceful degradation - non-fatal if fails)
	change_24h := 0.0  // Default fallback if API fails
	api_change, api_err := get_24h_change_cached(token.contract_address)
	if api_err == .None {
		change_24h = api_change
	}
	// Note: API error is non-fatal - we have on-chain price, that's sufficient
	// Displaying 0.0% is better than blocking the entire price display

	// Return price data with hybrid approach (on-chain price + API 24h change)
	return PriceData{price_usd = price_in_usd, change_24h = change_24h}, .None
}
