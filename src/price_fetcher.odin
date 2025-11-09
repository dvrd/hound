package main

import "core:bufio"
import "core:encoding/json"
import "core:fmt"
import "core:math"
import "core:net"
import "core:strconv"
import client "../vendor/odin-http/client"

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

	// Get SOL price (hardcoded for MVP)
	sol_usd_price := 162.50

	// Convert to USD
	price_in_usd := price_in_quote * sol_usd_price

	// Return price data (24h change set to 0 for MVP)
	return PriceData{price_usd = price_in_usd, change_24h = 0.0}, .None
}
