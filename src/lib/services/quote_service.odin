#+feature global-context
package services

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "core:strconv"
import "core:strings"
import "core:time"
import "../models"
import client "../../http/client"
import memory "../memory"

// Jupiter Ultra API (free tier)
JUPITER_QUOTE_URL :: "https://lite-api.jup.ag/ultra/v1/order"

// Quote cache (90-second TTL)
QuoteCache :: struct {
	quote:     models.SwapQuote,
	cached_at: time.Time,
	is_valid:  bool,
}

g_quote_cache: QuoteCache
QUOTE_CACHE_TTL :: 90 * time.Second

// fetch_swap_quote fetches a swap quote from Jupiter Ultra API
//
// ASSERTION 1: Validate input/output mints are not empty
// ASSERTION 2: Validate amount is positive
// ASSERTION 3: Validate slippage_bps is reasonable (1-1000)
// ASSERTION 4: Validate taker is not empty
//
// Parameters:
//   - input_mint: Token to swap from (e.g., SOL mint)
//   - output_mint: Token to swap to (e.g., USDC mint)
//   - amount: Amount in lamports (smallest unit)
//   - taker: Wallet address executing the swap (REQUIRED for transaction)
//   - slippage_bps: Slippage tolerance in basis points (default: 50 = 0.5%)
//                   Note: Used for minimum_out calculation, not sent to API
//
// Returns: SwapQuote with routing info + transaction, ErrorType
fetch_swap_quote :: proc(
	input_mint: string,
	output_mint: string,
	amount: u64,
	taker: string,
	slippage_bps: int = 50,
) -> (models.SwapQuote, models.ErrorType) {
	// ASSERTION 1: Validate mints
	assert(len(input_mint) > 0, "Input mint cannot be empty")
	assert(len(output_mint) > 0, "Output mint cannot be empty")

	// ASSERTION 2: Validate amount
	assert(amount > 0, "Amount must be positive")

	// ASSERTION 3: Validate slippage
	assert(
		slippage_bps >= 1 && slippage_bps <= 1000,
		fmt.tprintf("Slippage must be 1-1000 bps, got %d", slippage_bps),
	)

	// ASSERTION 4: Validate taker
	assert(len(taker) > 0, "Taker wallet address cannot be empty")

	log.debugf("Fetching quote: %s → %s, amount: %d, taker: %s, slippage: %dbps",
		input_mint, output_mint, amount, taker, slippage_bps)

	// Build URL with query parameters (include taker to get transaction)
	url := fmt.tprintf(
		"%s?inputMint=%s&outputMint=%s&amount=%d&taker=%s",
		JUPITER_QUOTE_URL,
		input_mint,
		output_mint,
		amount,
		taker,
	)

	log.debugf("Quote URL: %s", url)

	// Make HTTP request
	res, http_err := client.get(url)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		#partial switch e in http_err {
		case net.Network_Error:
			log.debug("Network timeout detected")
			return {}, models.ErrorType.NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			log.debug("Connection error detected")
			return {}, models.ErrorType.ConnectionFailed
		case client.Request_Error:
			log.debug("Request error detected")
			return {}, models.ErrorType.InvalidResponse
		case:
			log.debug("Unknown network error detected")
			return {}, models.ErrorType.NetworkError
		}
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	#partial switch res.status {
	case .Bad_Request:
		log.debug("Bad request (400) - invalid parameters")
		return {}, models.ErrorType.InvalidToken
	case .Not_Found:
		log.debug("Not found (404) - no route available")
		return {}, models.ErrorType.TokenNotFound
	case .Too_Many_Requests:
		log.warn("Rate limited (429)")
		return {}, models.ErrorType.RateLimited
	case .OK:
		log.debug("HTTP 200 OK - processing quote")
	case:
		log.warnf("Unknown status code: %v", res.status)
		return {}, models.ErrorType.ServerError
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract body: %v", body_err)
		return {}, models.ErrorType.InvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str, is_str := body.(string)
	if !is_str {
		log.error("Response body is not a string")
		return {}, models.ErrorType.InvalidResponse
	}

	log.debug("Parsing quote JSON")

	// Parse JSON response with request arena
	arena_alloc := memory.request_allocator()
	response_json: json.Value
	spec := json.DEFAULT_SPECIFICATION
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec, arena_alloc); unmarshal_err != nil {
		log.errorf("JSON unmarshal failed: %v", unmarshal_err)
		return {}, models.ErrorType.InvalidResponse
	}

	// Extract quote data
	quote, parse_err := parse_quote_response(response_json, input_mint, output_mint, body_str)
	if parse_err != .None {
		return {}, parse_err
	}

	// Override slippage with user-provided value (Ultra API doesn't return it)
	quote.slippage_bps = slippage_bps

	// Set timestamps
	now := time.now()
	quote.fetched_at = time.time_to_unix(now)
	quote.expires_at = time.time_to_unix(time.time_add(now, QUOTE_CACHE_TTL))

	log.debug("Quote fetch successful")
	return quote, .None
}

// parse_quote_response extracts SwapQuote from Jupiter JSON response
//
// Internal helper - parses nested JSON structure
parse_quote_response :: proc(
	json_val: json.Value,
	input_mint: string,
	output_mint: string,
	raw_json: string,
) -> (quote: models.SwapQuote, err: models.ErrorType) {
	obj, is_obj := json_val.(json.Object)
	if !is_obj {
		log.error("Response is not a JSON object")
		return {}, .InvalidResponse
	}

	// Extract amounts (Ultra API uses different field names)
	in_amount_val, has_in := obj["inAmount"]
	out_amount_val, has_out := obj["outAmount"]
	if !has_in || !has_out {
		log.error("Missing inAmount or outAmount")
		return {}, .InvalidResponse
	}

	// Parse amounts (Ultra API returns strings for u64 precision)
	in_amount_str, in_is_str := in_amount_val.(json.String)
	out_amount_str, out_is_str := out_amount_val.(json.String)

	if !in_is_str || !out_is_str {
		log.error("Amount values are not strings")
		return {}, .InvalidResponse
	}

	in_lamports, in_ok := strconv.parse_u64(in_amount_str)
	if !in_ok {
		log.errorf("Failed to parse inAmount: %s", in_amount_str)
		return {}, .InvalidResponse
	}

	out_lamports, out_ok := strconv.parse_u64(out_amount_str)
	if !out_ok {
		log.errorf("Failed to parse outAmount: %s", out_amount_str)
		return {}, .InvalidResponse
	}

	// Extract slippage and threshold
	slippage_val, has_slippage := obj["slippageBps"]
	threshold_val, has_threshold := obj["otherAmountThreshold"]

	slippage := 50 // Default
	if has_slippage {
		if slippage_int, is_int := slippage_val.(json.Integer); is_int {
			slippage = int(slippage_int)
		}
	}

	minimum_out_lamports := u64(0)
	if has_threshold {
		if threshold_str, is_str := threshold_val.(json.String); is_str {
			minimum_out_lamports, _ = strconv.parse_u64(threshold_str)
		}
	}

	// Extract price impact (Ultra API uses "priceImpact" not "priceImpactPct")
	price_impact := f64(0.0)
	if impact_val, has_impact := obj["priceImpact"]; has_impact {
		if impact_float, is_float := impact_val.(json.Float); is_float {
			price_impact = f64(impact_float)
		} else if impact_int, is_int := impact_val.(json.Integer); is_int {
			price_impact = f64(impact_int)
		}
	}

	// Extract router type (Ultra API uses "router" field directly)
	primary_dex := "Unknown"
	if router_val, has_router := obj["router"]; has_router {
		if router_str, is_str := router_val.(json.String); is_str {
			primary_dex = router_str
		}
	}

	// Extract route plan (if available)
	route_steps: []models.RouteStep
	if route_val, has_route := obj["routePlan"]; has_route {
		route_steps, _ = parse_route_plan(route_val)
	}

	// Build SwapQuote
	quote.input_mint = input_mint
	quote.input_lamports = in_lamports
	quote.output_mint = output_mint
	quote.output_lamports = out_lamports
	quote.slippage_bps = slippage
	quote.minimum_out = f64(minimum_out_lamports) / 1_000_000_000.0
	quote.price_impact_pct = price_impact
	quote.route_plan = route_steps
	quote.primary_dex = primary_dex

	// Calculate rate
	if in_lamports > 0 {
		quote.rate = f64(out_lamports) / f64(in_lamports)
	}

	// Estimate network fee (typical Solana transaction)
	quote.network_fee_sol = 0.00001  // 10,000 lamports = 0.00001 SOL

	// Store raw JSON for Phase 3
	// CRITICAL: Copy string to command arena because request arena will be reset
	// after this function returns, invalidating the pointer
	command_alloc := memory.command_allocator()
	quote.jupiter_response = strings.clone(raw_json, command_alloc)

	return quote, .None
}

// parse_route_plan extracts routing steps from Jupiter response
//
// NOTE: Caller should set context.allocator = memory.request_allocator()
parse_route_plan :: proc(route_val: json.Value, arena_alloc := context.allocator) -> ([]models.RouteStep, models.ErrorType) {
	route_arr, is_arr := route_val.(json.Array)
	if !is_arr {
		return nil, .InvalidResponse
	}

	steps := make([dynamic]models.RouteStep, 0, 10, arena_alloc)

	for step_val in route_arr {
		step_obj, is_obj := step_val.(json.Object)
		if !is_obj {
			continue
		}

		swap_info_val, has_swap := step_obj["swapInfo"]
		if !has_swap {
			continue
		}

		swap_info, is_swap_obj := swap_info_val.(json.Object)
		if !is_swap_obj {
			continue
		}

		// Extract swap info fields
		step: models.RouteStep

		if label_val, has_label := swap_info["label"]; has_label {
			if label_str, is_str := label_val.(json.String); is_str {
				step.dex_label = label_str
			}
		}

		if input_mint_val, has_input := swap_info["inputMint"]; has_input {
			if input_str, is_str := input_mint_val.(json.String); is_str {
				step.input_mint = input_str
			}
		}

		if output_mint_val, has_output := swap_info["outputMint"]; has_output {
			if output_str, is_str := output_mint_val.(json.String); is_str {
				step.output_mint = output_str
			}
		}

		// Parse amounts
		if in_amount_val, has_in := swap_info["inAmount"]; has_in {
			if in_str, is_str := in_amount_val.(json.String); is_str {
				step.input_amount, _ = strconv.parse_u64(in_str)
			}
		}

		if out_amount_val, has_out := swap_info["outAmount"]; has_out {
			if out_str, is_str := out_amount_val.(json.String); is_str {
				step.output_amount, _ = strconv.parse_u64(out_str)
			}
		}

		if fee_amount_val, has_fee := swap_info["feeAmount"]; has_fee {
			if fee_str, is_str := fee_amount_val.(json.String); is_str {
				step.fee_amount, _ = strconv.parse_u64(fee_str)
			}
		}

		// Extract percent (route split percentage)
		if percent_val, has_percent := step_obj["percent"]; has_percent {
			if percent_int, is_int := percent_val.(json.Integer); is_int {
				step.percent = int(percent_int)
			}
		}

		append(&steps, step)
	}

	return steps[:], .None
}

// is_quote_expired checks if a quote is older than 90 seconds
is_quote_expired :: proc(quote: models.SwapQuote) -> bool {
	now := time.now()
	now_unix := time.time_to_unix(now)
	return (now_unix - quote.fetched_at) > 90
}
