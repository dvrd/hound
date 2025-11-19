package jupiter_swap

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "core:strconv"
import "core:time"
import client "../../vendor/odin-http/client"
import src "../"

// Jupiter Swap API v6 endpoints
// Reference: PRPs/ai_docs/jupiter-api-v6.md
JUPITER_QUOTE_API :: "https://quote-api.jup.ag/v6/quote"
JUPITER_SWAP_API :: "https://quote-api.jup.ag/v6/swap"

// Rate limit: 600 requests per 60 seconds (same as Price API)
// Cache quotes for 90 seconds (quote validity period)

// Fetch swap quote from Jupiter v6 Quote API
//
// Parameters:
//   - input_mint: Token to swap from (Solana mint address)
//   - output_mint: Token to swap to (Solana mint address)
//   - amount: Amount in base units (multiply by 10^decimals)
//   - slippage_bps: Slippage tolerance in basis points (50 = 0.5%)
//
// Returns: JupiterQuote with route and pricing info, or error
//
// CRITICAL: Quotes expire after ~90 seconds. Cache accordingly.
// Reference: PRPs/ai_docs/jupiter-api-v6.md (Quote expiry section)
get_quote :: proc(
	input_mint: string,
	output_mint: string,
	amount: u64,
	slippage_bps: u16 = 50,
) -> (
	JupiterQuote,
	src.ErrorType,
) {
	assert(len(input_mint) > 0, "Input mint cannot be empty")
	assert(len(output_mint) > 0, "Output mint cannot be empty")
	assert(amount > 0, "Amount must be greater than 0")

	log.debugf(
		"Fetching Jupiter quote: %s → %s, amount: %d, slippage: %d BPS",
		input_mint,
		output_mint,
		amount,
		slippage_bps,
	)

	// Check cache first (90-second TTL)
	if cached_quote, found := get_cached_quote(input_mint, output_mint, amount); found {
		log.debug("Returning cached quote")
		return cached_quote, .None
	}

	// Build URL with query parameters
	// NOTE: Jupiter v6 uses camelCase in URL parameters
	// fmt.tprintf uses temp allocator - no manual cleanup needed
	url := fmt.tprintf(
		"%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		JUPITER_QUOTE_API,
		input_mint,
		output_mint,
		amount,
		slippage_bps,
	)

	log.debugf("API URL: %s", url)

	// HTTP GET with error handling (pattern from src/jupiter_client.odin:51-114)
	res, http_err := client.get(url)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		#partial switch e in http_err {
		case net.Network_Error:
			log.debug("Network timeout detected")
			return {}, .NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			log.debug("Connection error detected")
			return {}, .ConnectionFailed
		case:
			log.debug("Unknown network error")
			return {}, .NetworkError
		}
	}
	defer client.response_destroy(&res) // CRITICAL: Always defer

	// Check HTTP status code
	log.debugf("HTTP response status: %v", res.status)
	#partial switch res.status {
	case .OK:
		log.debug("HTTP 200 OK - processing response")
	// Continue
	case .Bad_Request:
		log.error("Bad request (400) - invalid parameters")
		return {}, .InvalidToken
	case .Not_Found:
		log.warn("No route found (404) for this swap pair")
		return {}, .TokenNotFound
	case .Too_Many_Requests:
		log.warn("Rate limited (429) - 600 req/min limit exceeded")
		return {}, .RateLimited
	case .Internal_Server_Error, .Service_Unavailable:
		log.error("Jupiter server error (500/503)")
		return {}, .ServerError
	case:
		log.warnf("Unexpected status code: %v", res.status)
		return {}, .ServerError
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract response body: %v", body_err)
		return {}, .InvalidResponse
	}
	defer client.body_destroy(body, allocation) // CRITICAL: Always defer

	log.debug("Parsing JSON response")

	// Parse JSON response
	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body.(string), &response_json, spec);
	   unmarshal_err != nil {
		log.errorf("JSON unmarshal failed: %v", unmarshal_err)
		return {}, .InvalidResponse
	}

	// Extract quote data
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		log.error("Response is not a JSON object")
		return {}, .InvalidResponse
	}

	// Build JupiterQuote struct from response
	// Reference: PRPs/ai_docs/jupiter-api-v6.md (Response structure)
	quote := JupiterQuote {
		input_mint             = get_json_string(response_obj, "inputMint"),
		output_mint            = get_json_string(response_obj, "outputMint"),
		in_amount              = get_json_string(response_obj, "inAmount"),
		out_amount             = get_json_string(response_obj, "outAmount"),
		other_amount_threshold = get_json_string(response_obj, "otherAmountThreshold"),
		swap_mode              = get_json_string(response_obj, "swapMode"),
		slippage_bps           = u16(get_json_int(response_obj, "slippageBps")),
		price_impact_pct       = get_json_string(response_obj, "priceImpactPct"),
		route_plan             = parse_route_plan(response_obj),
		cached_at              = time.now(),
		is_valid               = true,
	}

	// CRITICAL: Cache the quote (90-second TTL)
	cache_quote(quote)

	log.debugf("Quote fetched: %s %s → %s %s", quote.in_amount, input_mint, quote.out_amount, output_mint)

	return quote, .None
}

// Build unsigned swap transaction from quote
//
// Parameters:
//   - quote: Jupiter quote from get_quote()
//   - user_public_key: User's wallet public key (Solana address)
//
// Returns: JupiterSwapResponse with base64-encoded unsigned transaction
//
// CRITICAL: Transaction includes recent blockhash and expires after ~150 blocks (~60s)
// Reference: PRPs/ai_docs/solana-transactions.md (Transaction lifetime)
build_swap_transaction :: proc(
	quote: JupiterQuote,
	user_public_key: string,
) -> (
	JupiterSwapResponse,
	src.ErrorType,
) {
	assert(len(user_public_key) > 0, "User public key cannot be empty")
	assert(quote.is_valid, "Quote must be valid")

	log.debugf("Building swap transaction for user: %s", user_public_key)

	// Check if quote is still valid (~90 seconds)
	elapsed := time.since(quote.cached_at)
	if elapsed > 90 * time.Second {
		log.warn("Quote expired (> 90 seconds old)")
		return {}, .InvalidToken // Reuse error enum, could add .QuoteExpired
	}

	// Build JSON request body (pattern from src/solana_rpc.odin:60-91)
	request_obj := json.Object{}

	// Reconstruct full quote object (Jupiter requires complete quote in swap request)
	// Reference: PRPs/ai_docs/jupiter-api-v6.md (POST /v6/swap)
	quote_obj := json.Object{}
	quote_obj["inputMint"] = json.String(quote.input_mint)
	quote_obj["outputMint"] = json.String(quote.output_mint)
	quote_obj["inAmount"] = json.String(quote.in_amount)
	quote_obj["outAmount"] = json.String(quote.out_amount)
	quote_obj["otherAmountThreshold"] = json.String(quote.other_amount_threshold)
	quote_obj["swapMode"] = json.String(quote.swap_mode)
	quote_obj["slippageBps"] = json.Integer(quote.slippage_bps)
	quote_obj["priceImpactPct"] = json.String(quote.price_impact_pct)

	// NOTE: routePlan is optional in Jupiter API request
	// Omitting for simplicity - Jupiter will reconstruct from the quote parameters

	// Build full swap request
	request_obj["quoteResponse"] = quote_obj
	request_obj["userPublicKey"] = json.String(user_public_key)
	request_obj["wrapAndUnwrapSol"] = json.Boolean(true) // Auto-wrap/unwrap SOL
	request_obj["dynamicComputeUnitLimit"] = json.Boolean(true) // CRITICAL: Use dynamic CU limits
	request_obj["prioritizationFeeLamports"] = json.String("auto") // CRITICAL: Auto priority fees

	log.debug("Sending swap request to Jupiter")

	// HTTP POST with JSON (pattern from src/solana_rpc.odin)
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req) // CRITICAL

	if marshal_err := client.with_json(&req, request_obj); marshal_err != nil {
		log.errorf("Failed to marshal swap request: %v", marshal_err)
		return {}, .InvalidResponse
	}

	res, err := client.request(&req, JUPITER_SWAP_API)
	if err != nil {
		log.errorf("Jupiter swap request failed: %v", err)
		#partial switch e in err {
		case net.Network_Error:
			return {}, .NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			return {}, .ConnectionFailed
		case:
			return {}, .NetworkError
		}
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	log.debugf("Swap response status: %v", res.status)
	if res.status != .OK {
		log.errorf("Jupiter swap failed: %v", res.status)
		#partial switch res.status {
		case .Bad_Request:
			return {}, .InvalidToken
		case .Too_Many_Requests:
			return {}, .RateLimited
		case:
			return {}, .ServerError
		}
	}

	// Extract response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract swap response body: %v", body_err)
		return {}, .InvalidResponse
	}
	defer client.body_destroy(body, allocation)

	log.debug("Parsing swap response JSON")

	// Parse swap response
	response_json: json.Value
	spec := json.Specification{}
	if json_err := json.unmarshal_string(body.(string), &response_json, spec); json_err != nil {
		log.errorf("Failed to parse swap response: %v", json_err)
		return {}, .InvalidResponse
	}

	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		log.error("Swap response is not a JSON object")
		return {}, .InvalidResponse
	}

	// Extract transaction data
	swap_response := JupiterSwapResponse {
		swap_transaction        = get_json_string(response_obj, "swapTransaction"),
		last_valid_block_height = u64(get_json_int(response_obj, "lastValidBlockHeight")),
	}

	log.debugf("Transaction built successfully, expires at block: %d", swap_response.last_valid_block_height)

	return swap_response, .None
}

// Helper: Parse route plan from JSON response
parse_route_plan :: proc(response_obj: json.Object) -> []RoutePlanStep {
	route_plan_value, has_route := response_obj["routePlan"]
	if !has_route {
		return nil
	}

	route_array, is_array := route_plan_value.(json.Array)
	if !is_array {
		return nil
	}

	steps := make([]RoutePlanStep, len(route_array))

	for step_value, i in route_array {
		step_obj, is_obj := step_value.(json.Object)
		if !is_obj do continue

		swap_info_value, has_swap := step_obj["swapInfo"]
		if !has_swap do continue

		swap_info_obj, is_swap_obj := swap_info_value.(json.Object)
		if !is_swap_obj do continue

		steps[i] = RoutePlanStep {
			swap_info = SwapInfo {
				amm_key = get_json_string(swap_info_obj, "ammKey"),
				label = get_json_string(swap_info_obj, "label"),
				input_mint = get_json_string(swap_info_obj, "inputMint"),
				output_mint = get_json_string(swap_info_obj, "outputMint"),
				in_amount = get_json_string(swap_info_obj, "inAmount"),
				out_amount = get_json_string(swap_info_obj, "outAmount"),
				fee_amount = get_json_string(swap_info_obj, "feeAmount"),
				fee_mint = get_json_string(swap_info_obj, "feeMint"),
			},
			percent = u8(get_json_int(step_obj, "percent")),
		}
	}

	return steps
}

// Helper: Safely extract string from JSON object
get_json_string :: proc(obj: json.Object, key: string) -> string {
	value, found := obj[key]
	if !found do return ""

	str, is_string := value.(json.String)
	if !is_string do return ""

	return string(str)
}

// Helper: Safely extract integer from JSON object
get_json_int :: proc(obj: json.Object, key: string) -> i64 {
	value, found := obj[key]
	if !found do return 0

	num, is_int := value.(json.Integer)
	if !is_int do return 0

	return i64(num)
}
