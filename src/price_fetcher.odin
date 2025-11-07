package main

import "core:bufio"
import "core:encoding/json"
import "core:fmt"
import "core:net"
import "core:strconv"
import client "../vendor/odin-http/client"

AURA_CONTRACT_ADDRESS :: "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2"

fetch_price_for_aura :: proc() -> (price: PriceData, err: ErrorType) {
	// Build URL
	url := fmt.tprintf("https://api.dexscreener.com/latest/dex/tokens/%s", AURA_CONTRACT_ADDRESS)
	
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
