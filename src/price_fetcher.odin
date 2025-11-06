package main

import "core:fmt"
import "core:encoding/json"
import "core:strconv"
import client "../vendor/odin-http/client"

AURA_CONTRACT_ADDRESS :: "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2"

fetch_price_for_aura :: proc() -> (price: PriceData, err: ErrorType) {
	// Build URL
	url := fmt.tprintf("https://api.dexscreener.com/latest/dex/tokens/%s", AURA_CONTRACT_ADDRESS)
	
	// Make HTTP request with error handling
	res, http_err := client.get(url)
	if http_err != nil {
		fmt.eprintfln("HTTP error: %v", http_err)
		return {}, .NetworkError
	}
	defer client.response_destroy(&res)
	
	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		fmt.eprintfln("Body error: %v", body_err)
		return {}, .NetworkError
	}
	defer client.body_destroy(body, allocation)
	
	// Parse JSON (body is string)
	response: DexScreenerResponse
	json_err := json.unmarshal_string(body.(string), &response)
	if json_err != nil {
		fmt.eprintfln("JSON error: %v", json_err)
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
		fmt.eprintfln("Failed to parse price: %s", price_str)
		return {}, .InvalidResponse
	}
	change := response.pairs[0].priceChange.h24
	
	// Return data
	return PriceData{price_usd = price_val, change_24h = change}, .None
}
