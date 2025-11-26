// Token Information Service
// Aggregates token data from multiple sources:
// - DexScreener API: Market data, volume, transactions, price changes
// - Solana RPC: Token supply, top holders
package services

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:strconv"
import "core:strings"
import client "../../http/client"
import "../models"
import "../blockchain"
import "../memory"

// ============================================================================
// DexScreener Extended API Structures
// ============================================================================

// DexScreener pair response with full market data
DexScreenerPair :: struct {
	chainId:      string,
	dexId:        string,
	url:          string,
	pairAddress:  string,
	baseToken:    struct {
		address: string,
		name:    string,
		symbol:  string,
	},
	quoteToken:   struct {
		address: string,
		name:    string,
		symbol:  string,
	},
	priceNative:  string,
	priceUsd:     string,
	txns:         struct {
		m5:  struct { buys: int, sells: int },
		h1:  struct { buys: int, sells: int },
		h6:  struct { buys: int, sells: int },
		h24: struct { buys: int, sells: int },
	},
	volume:       struct {
		h24: f64,
		h6:  f64,
		h1:  f64,
		m5:  f64,
	},
	priceChange:  struct {
		m5:  f64,
		h1:  f64,
		h6:  f64,
		h24: f64,
	},
	liquidity:    struct {
		usd:   f64,
		base:  f64,
		quote: f64,
	},
	fdv:          f64,    // Fully Diluted Valuation
	marketCap:    f64,    // Market capitalization
	pairCreatedAt: i64,   // Unix timestamp (milliseconds)
	info:         struct {
		imageUrl:  string,
		websites:  []struct { url: string },
		socials:   []struct { type: string, url: string },
	},
}

DexScreenerTokenResponse :: struct {
	schemaVersion: string,
	pairs:         []DexScreenerPair,
}

// ============================================================================
// Token Info Aggregation
// ============================================================================

// fetch_extended_token_info aggregates comprehensive token information
//
// Data sources (in priority order):
// 1. DexScreener API - Market cap, FDV, liquidity, volume, transactions, price changes
// 2. Solana RPC - Token supply, top 20 holders
//
// Returns TokenExtendedInfo with all available data, or error if critical data unavailable
fetch_extended_token_info :: proc(
	mint_address: string,
	rpc_endpoint: string,
) -> (models.TokenExtendedInfo, models.ErrorType) {
	log.infof("Fetching extended token info for: %s", mint_address)

	arena_alloc := memory.request_allocator()

	// Initialize result structure
	info := models.TokenExtendedInfo{
		mint_address = mint_address,
		network      = "solana",
	}

	// Step 1: Fetch market data from DexScreener
	log.debug("Fetching market data from DexScreener...")
	dex_data, dex_err := fetch_dexscreener_data(mint_address, arena_alloc)
	if dex_err == .None {
		populate_market_data(&info, dex_data, arena_alloc)
	} else {
		log.warnf("DexScreener API failed: %v", dex_err)
		// Continue - we can still get on-chain data
	}

	// Step 2: Fetch supply and holders from Solana RPC
	log.debug("Fetching on-chain data from Solana RPC...")
	rpc_conn, rpc_err := blockchain.connect_rpc(rpc_endpoint)
	if rpc_err != .None {
		log.errorf("Failed to connect to Solana RPC: %v", rpc_err)
		return {}, .RPCConnectionFailed
	}

	// Get token supply
	supply_info, supply_err := blockchain.get_token_supply(rpc_conn, mint_address)
	if supply_err == .None {
		info.total_supply = supply_info.ui_amount
		info.decimals = int(supply_info.decimals)
		log.debugf("Token supply: %f (decimals: %d)", info.total_supply, info.decimals)
	} else {
		log.warnf("Failed to fetch token supply: %v", supply_err)
	}

	// Get largest token holders
	holders, holders_err := blockchain.get_token_largest_accounts(rpc_conn, mint_address)
	if holders_err == .None {
		info.top_holders = convert_holders_to_top_holders(holders, info.total_supply, arena_alloc)
		log.debugf("Found %d top holders", len(info.top_holders))
	} else {
		log.warnf("Failed to fetch top holders: %v", holders_err)
	}

	// Step 3: Validate we have minimum required data
	if info.price_usd == 0.0 && len(info.symbols) == 0 {
		log.error("No valid token data retrieved from any source")
		return {}, .TokenNotFound
	}

	info.is_active = info.volume_24h > 0.0 || info.txns_24h > 0

	return info, .None
}

// ============================================================================
// DexScreener API Client
// ============================================================================

fetch_dexscreener_data :: proc(
	mint_address: string,
	allocator := context.allocator,
) -> (DexScreenerTokenResponse, models.ErrorType) {
	url := fmt.tprintf("https://api.dexscreener.com/latest/dex/tokens/%s", mint_address)
	log.debugf("Fetching DexScreener data: %s", url)

	// Make HTTP request
	res, http_err := client.get(url)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		return {}, .ConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check status
	if res.status != .OK {
		log.errorf("HTTP status: %v", res.status)
		if res.status == .Too_Many_Requests {
			return {}, .RateLimited
		}
		return {}, .ServerError
	}

	// Parse response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to read response body: %v", body_err)
		return {}, .InvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str := body.(string)

	// Parse JSON
	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec, allocator); unmarshal_err != nil {
		log.errorf("JSON unmarshal failed: %v", unmarshal_err)
		return {}, .InvalidResponse
	}

	// Convert to structured response
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		return {}, .InvalidResponse
	}

	pairs_val, has_pairs := response_obj["pairs"]
	if !has_pairs {
		return {}, .TokenNotFound
	}

	pairs_array, is_array := pairs_val.(json.Array)
	if !is_array || len(pairs_array) == 0 {
		return {}, .TokenNotFound
	}

	// Parse pairs
	pairs := make([]DexScreenerPair, len(pairs_array), allocator)
	for pair_val, i in pairs_array {
		pair_obj, is_pair_obj := pair_val.(json.Object)
		if !is_pair_obj {
			continue
		}

		pair := parse_dexscreener_pair(pair_obj, allocator)
		pairs[i] = pair
	}

	return DexScreenerTokenResponse{
		pairs = pairs,
	}, .None
}

parse_dexscreener_pair :: proc(pair_obj: json.Object, allocator := context.allocator) -> DexScreenerPair {
	context.allocator = allocator

	pair: DexScreenerPair

	// Parse simple string fields
	if chain_id, ok := pair_obj["chainId"].(json.String); ok {
		pair.chainId = string(chain_id)
	}
	if dex_id, ok := pair_obj["dexId"].(json.String); ok {
		pair.dexId = string(dex_id)
	}
	if price_usd, ok := pair_obj["priceUsd"].(json.String); ok {
		pair.priceUsd = string(price_usd)
	}
	if price_native, ok := pair_obj["priceNative"].(json.String); ok {
		pair.priceNative = string(price_native)
	}
	if pair_address, ok := pair_obj["pairAddress"].(json.String); ok {
		pair.pairAddress = string(pair_address)
	}

	// Parse baseToken
	if base_token_val, has_base := pair_obj["baseToken"]; has_base {
		if base_token, ok := base_token_val.(json.Object); ok {
			if address, ok2 := base_token["address"].(json.String); ok2 {
				pair.baseToken.address = string(address)
			}
			if name, ok2 := base_token["name"].(json.String); ok2 {
				pair.baseToken.name = string(name)
			}
			if symbol, ok2 := base_token["symbol"].(json.String); ok2 {
				pair.baseToken.symbol = string(symbol)
			}
		}
	}

	// Parse quoteToken
	if quote_token_val, has_quote := pair_obj["quoteToken"]; has_quote {
		if quote_token, ok := quote_token_val.(json.Object); ok {
			if symbol, ok2 := quote_token["symbol"].(json.String); ok2 {
				pair.quoteToken.symbol = string(symbol)
			}
		}
	}

	// Parse volume
	if volume_val, has_volume := pair_obj["volume"]; has_volume {
		if volume, ok := volume_val.(json.Object); ok {
			if h24, ok2 := volume["h24"].(json.Float); ok2 {
				pair.volume.h24 = h24
			}
		}
	}

	// Parse transactions
	if txns_val, has_txns := pair_obj["txns"]; has_txns {
		if txns, ok := txns_val.(json.Object); ok {
			if h24_val, has_h24 := txns["h24"]; has_h24 {
				if h24, ok2 := h24_val.(json.Object); ok2 {
					if buys, ok3 := h24["buys"].(json.Integer); ok3 {
						pair.txns.h24.buys = int(buys)
					}
					if sells, ok3 := h24["sells"].(json.Integer); ok3 {
						pair.txns.h24.sells = int(sells)
					}
				}
			}
		}
	}

	// Parse priceChange
	if price_change_val, has_change := pair_obj["priceChange"]; has_change {
		if price_change, ok := price_change_val.(json.Object); ok {
			if m5, ok2 := price_change["m5"].(json.Float); ok2 {
				pair.priceChange.m5 = m5
			}
			if h1, ok2 := price_change["h1"].(json.Float); ok2 {
				pair.priceChange.h1 = h1
			}
			if h6, ok2 := price_change["h6"].(json.Float); ok2 {
				pair.priceChange.h6 = h6
			}
			if h24, ok2 := price_change["h24"].(json.Float); ok2 {
				pair.priceChange.h24 = h24
			}
		}
	}

	// Parse liquidity
	if liquidity_val, has_liquidity := pair_obj["liquidity"]; has_liquidity {
		if liquidity, ok := liquidity_val.(json.Object); ok {
			if usd, ok2 := liquidity["usd"].(json.Float); ok2 {
				pair.liquidity.usd = usd
			}
		}
	}

	// Parse FDV and market cap
	if fdv, ok := pair_obj["fdv"].(json.Float); ok {
		pair.fdv = fdv
	}
	if market_cap, ok := pair_obj["marketCap"].(json.Float); ok {
		pair.marketCap = market_cap
	}

	// Parse pairCreatedAt
	if created_at, ok := pair_obj["pairCreatedAt"].(json.Integer); ok {
		pair.pairCreatedAt = i64(created_at)
	}

	return pair
}

// ============================================================================
// Data Population Helpers
// ============================================================================

populate_market_data :: proc(info: ^models.TokenExtendedInfo, dex_data: DexScreenerTokenResponse, allocator := context.allocator) {
	if len(dex_data.pairs) == 0 {
		return
	}

	// Collect all unique symbols
	symbols_set := make(map[string]bool, allocator)
	defer delete(symbols_set)

	// Aggregate data from all pairs
	total_liquidity := 0.0
	total_volume := 0.0
	total_txns := 0
	total_buys := 0
	total_sells := 0
	oldest_created_at := i64(9999999999999) // Very large number

	highest_fdv := 0.0
	highest_market_cap := 0.0

	// Use first pair for price and price changes (they should be similar across pairs)
	first_pair := dex_data.pairs[0]

	if price_str := first_pair.priceUsd; len(price_str) > 0 {
		info.price_usd = parse_float(price_str)
	}

	info.price_change_5m = first_pair.priceChange.m5
	info.price_change_1h = first_pair.priceChange.h1
	info.price_change_6h = first_pair.priceChange.h6
	info.price_change_24h = first_pair.priceChange.h24

	for pair in dex_data.pairs {
		// Collect symbols
		if len(pair.baseToken.symbol) > 0 {
			symbols_set[pair.baseToken.symbol] = true
		}

		// Aggregate liquidity and volume
		total_liquidity += pair.liquidity.usd
		total_volume += pair.volume.h24

		// Aggregate transactions
		total_txns += pair.txns.h24.buys + pair.txns.h24.sells
		total_buys += pair.txns.h24.buys
		total_sells += pair.txns.h24.sells

		// Track oldest pair creation date
		if pair.pairCreatedAt > 0 && pair.pairCreatedAt < oldest_created_at {
			oldest_created_at = pair.pairCreatedAt
		}

		// Use highest FDV and market cap
		if pair.fdv > highest_fdv {
			highest_fdv = pair.fdv
		}
		if pair.marketCap > highest_market_cap {
			highest_market_cap = pair.marketCap
		}

		// Use first pair's name if available
		if len(info.name) == 0 && len(pair.baseToken.name) > 0 {
			info.name = pair.baseToken.name
		}
	}

	// Convert symbols set to array
	info.symbols = make([]string, len(symbols_set), allocator)
	i := 0
	for symbol in symbols_set {
		info.symbols[i] = symbol
		i += 1
	}

	// Set aggregated values
	info.liquidity = total_liquidity
	info.volume_24h = total_volume
	info.txns_24h = total_txns
	info.buys_24h = total_buys
	info.sells_24h = total_sells
	info.fdv = highest_fdv
	info.market_cap = highest_market_cap

	// Set creation date (convert from milliseconds timestamp to ISO 8601 string)
	if oldest_created_at < 9999999999999 {
		info.created_at = fmt.tprintf("%d", oldest_created_at)
	}
}

convert_holders_to_top_holders :: proc(
	holders: []blockchain.TokenAccountInfo,
	total_supply: f64,
	allocator := context.allocator,
) -> []models.TopHolder {
	top_holders := make([]models.TopHolder, len(holders), allocator)

	for holder, i in holders {
		ownership_pct := 0.0
		if total_supply > 0.0 {
			ownership_pct = (holder.ui_amount / total_supply) * 100.0
		}

		top_holders[i] = models.TopHolder{
			address = holder.address,
			balance = holder.ui_amount,
			ownership_pct = ownership_pct,
		}
	}

	return top_holders
}

// Helper to parse float from string (DexScreener returns numbers as strings)
parse_float :: proc(s: string) -> f64 {
	if len(s) == 0 {
		return 0.0
	}

	// Try parsing as float
	if val, ok := strconv.parse_f64(s); ok {
		return val
	}

	return 0.0
}
