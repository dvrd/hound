#+feature global-context
package services

import "core:bytes"
import "core:encoding/base64"
import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "../models"
import client "../../http/client"
import http "../../http"
import keystore "../keystore"
import "core:crypto/ed25519"

// Jupiter Ultra Execute API endpoint
JUPITER_EXECUTE_URL :: "https://lite-api.jup.ag/ultra/v1/execute"

// execute_swap_transaction signs and submits a swap transaction via Jupiter Ultra API
//
// ASSERTION 1: Quote must not be expired (< 90 seconds old)
// ASSERTION 2: Keypair must be valid
// ASSERTION 3: Transaction must be present in quote
// ASSERTION 4: RequestId must be present in quote
//
// Parameters:
//   - quote: SwapQuote from fetch_swap_quote() with transaction and requestId
//   - keypair: Decrypted Ed25519 keypair from keystore
//   - input_symbol: Symbol for input token (e.g., "SOL")
//   - output_symbol: Symbol for output token (e.g., "USDC")
//
// Returns: SwapTransactionResult with signature and details, ErrorType
execute_swap_transaction :: proc(
	quote: models.SwapQuote,
	keypair: keystore.Keypair,
	input_symbol: string,
	output_symbol: string,
) -> (models.SwapTransactionResult, models.ErrorType) {
	// ASSERTION 1: Check quote expiry
	if is_quote_expired(quote) {
		log.error("Quote has expired (> 90 seconds old)")
		return {}, models.ErrorType.QuoteExpired
	}

	// Debug: Log the Jupiter response to see what fields are present
	log.debugf("Jupiter response JSON (first 500 chars): %s", quote.jupiter_response[:min(len(quote.jupiter_response), 500)])

	// Extract transaction and requestId from quote JSON
	transaction_b64, request_id, extract_err := extract_transaction_and_request_id(quote.jupiter_response)
	if extract_err != .None {
		log.error("Failed to extract transaction and requestId from Jupiter response")
		log.errorf("Full response: %s", quote.jupiter_response)
		return {}, extract_err
	}

	// ASSERTION 3: Transaction must be present
	if len(transaction_b64) == 0 {
		// Check if Jupiter returned an error message
		error_msg := extract_error_message(quote.jupiter_response)
		if len(error_msg) > 0 {
			log.errorf("Jupiter API error: %s", error_msg)

			// Map common Jupiter errors to our error types
			if error_msg == "Insufficient funds" {
				return {}, models.ErrorType.InsufficientBalance
			}

			return {}, models.ErrorType.InvalidResponse
		}

		log.error("Transaction field is empty or missing in quote")
		return {}, models.ErrorType.InvalidResponse
	}

	// ASSERTION 4: RequestId must be present
	if len(request_id) == 0 {
		log.error("RequestId field is missing in quote")
		return {}, models.ErrorType.InvalidResponse
	}

	log.debugf("Extracted transaction (length: %d bytes) and requestId: %s", len(transaction_b64), request_id)

	// Sign transaction
	signed_tx, sign_err := sign_transaction(transaction_b64, keypair.private_key_struct)
	if sign_err != .None {
		return {}, sign_err
	}
	defer delete(signed_tx)

	log.debug("Transaction signed successfully")

	// Submit to Ultra execute endpoint
	result, submit_err := submit_to_ultra_execute(signed_tx, request_id, quote, input_symbol, output_symbol)
	if submit_err != .None {
		return {}, submit_err
	}

	log.infof("Transaction submitted successfully: %s", result.signature)
	return result, .None
}

// extract_transaction_and_request_id parses Jupiter Ultra response for transaction and requestId
//
// Ultra API response format:
// {
//   "inAmount": "1000000000",
//   "outAmount": "185432100",
//   "transaction": "base64-encoded-transaction",
//   "requestId": "uuid-v4-string",
//   ...
// }
//
// Returns: transaction (base64 string), requestId, ErrorType
extract_transaction_and_request_id :: proc(raw_json: string) -> (transaction: string, request_id: string, err: models.ErrorType) {
	// Parse JSON
	json_val: json.Value
	if unmarshal_err := json.unmarshal_string(raw_json, &json_val); unmarshal_err != nil {
		log.errorf("Failed to parse quote JSON: %v", unmarshal_err)
		return "", "", .InvalidResponse
	}

	obj, is_obj := json_val.(json.Object)
	if !is_obj {
		log.error("Quote JSON is not an object")
		return "", "", .InvalidResponse
	}

	// Extract transaction field
	tx_val, has_tx := obj["transaction"]
	if !has_tx {
		log.error("Missing 'transaction' field in quote response")
		return "", "", .InvalidResponse
	}

	tx_str, is_str := tx_val.(json.String)
	if !is_str {
		log.error("'transaction' field is not a string")
		return "", "", .InvalidResponse
	}

	// Extract requestId field
	req_id_val, has_req_id := obj["requestId"]
	if !has_req_id {
		log.error("Missing 'requestId' field in quote response")
		return "", "", .InvalidResponse
	}

	req_id_str, is_req_str := req_id_val.(json.String)
	if !is_req_str {
		log.error("'requestId' field is not a string")
		return "", "", .InvalidResponse
	}

	return tx_str, req_id_str, .None
}

// extract_error_message parses Jupiter error message from response
//
// Returns: error message string (empty if no error)
extract_error_message :: proc(raw_json: string) -> string {
	// Parse JSON
	json_val: json.Value
	if unmarshal_err := json.unmarshal_string(raw_json, &json_val); unmarshal_err != nil {
		return ""
	}

	obj, is_obj := json_val.(json.Object)
	if !is_obj {
		return ""
	}

	// Try "errorMessage" field first
	if error_val, has_error := obj["errorMessage"]; has_error {
		if error_str, is_str := error_val.(json.String); is_str {
			return error_str
		}
	}

	// Try "error" field as fallback
	if error_val, has_error := obj["error"]; has_error {
		if error_str, is_str := error_val.(json.String); is_str {
			return error_str
		}
	}

	return ""
}

// sign_transaction signs a base64-encoded Solana transaction with Ed25519 private key
//
// Process:
// 1. Decode base64 transaction bytes
// 2. Extract message to sign (skip signature bytes at start)
// 3. Sign with Ed25519
// 4. Replace signature bytes in transaction
// 5. Re-encode to base64
//
// Returns: Signed transaction (base64), ErrorType
sign_transaction :: proc(transaction_b64: string, private_key: ed25519.Private_Key) -> (string, models.ErrorType) {
	// Decode base64 transaction
	tx_bytes, decode_err := base64.decode(transaction_b64)
	if decode_err != nil {
		log.errorf("Failed to decode base64 transaction: %v", decode_err)
		return "", models.ErrorType.InvalidResponse
	}
	defer delete(tx_bytes)

	// Solana transaction structure (simplified):
	// [0]: Number of signatures (1 byte)
	// [1-64]: Signature placeholder (64 bytes) - we'll replace this
	// [65+]: Message to sign

	if len(tx_bytes) < 65 {
		log.errorf("Transaction too short: %d bytes (expected at least 65)", len(tx_bytes))
		return "", models.ErrorType.InvalidResponse
	}

	num_signatures := int(tx_bytes[0])
	if num_signatures != 1 {
		log.errorf("Expected 1 signature, got %d", num_signatures)
		return "", models.ErrorType.InvalidResponse
	}

	// Extract message (everything after signature placeholder)
	message := tx_bytes[65:]

	log.debugf("Signing message of %d bytes", len(message))

	// Sign with Ed25519 (pass by pointer)
	priv_key_copy := private_key
	signature: [ed25519.SIGNATURE_SIZE]byte
	ed25519.sign(&priv_key_copy, message, signature[:])

	log.debug("Ed25519 signature generated")

	// Replace signature bytes in transaction
	copy(tx_bytes[1:65], signature[:])

	// Re-encode to base64
	signed_b64, encode_err := base64.encode(tx_bytes)
	if encode_err != nil {
		log.errorf("Failed to encode signed transaction: %v", encode_err)
		return "", models.ErrorType.InvalidResponse
	}

	return signed_b64, .None
}

// submit_to_ultra_execute submits signed transaction to Jupiter Ultra execute endpoint
//
// POST /ultra/v1/execute
// Body: {
//   "transaction": "base64-signed-transaction",
//   "requestId": "uuid-from-order-response"
// }
//
// Response: {
//   "signature": "base58-tx-signature",
//   "status": "confirmed" | "finalized" | "failed",
//   "slot": 12345678,
//   ...
// }
//
// Returns: SwapTransactionResult, ErrorType
submit_to_ultra_execute :: proc(
	signed_transaction: string,
	request_id: string,
	quote: models.SwapQuote,
	input_symbol: string,
	output_symbol: string,
) -> (models.SwapTransactionResult, models.ErrorType) {
	log.debug("Submitting transaction to Ultra execute endpoint")

	// Build request body
	request_body := fmt.tprintf(
		`{"transaction":"%s","requestId":"%s"}`,
		signed_transaction,
		request_id,
	)
	defer delete(request_body)

	// Create POST request
	req: client.Request
	client.request_init(&req, .Post, context.allocator)
	defer client.request_destroy(&req)

	// Set JSON content type
	http.headers_set_content_type(&req.headers, "application/json")

	// Write body
	bytes.buffer_write_string(&req.body, request_body)

	// Make HTTP POST request
	res, http_err := client.request(&req, JUPITER_EXECUTE_URL, context.allocator)
	if http_err != nil {
		log.errorf("HTTP request failed: %v", http_err)
		#partial switch _ in http_err {
		case net.Network_Error:
			return {}, models.ErrorType.NetworkTimeout
		case net.TCP_Send_Error, net.Dial_Error:
			return {}, models.ErrorType.ConnectionFailed
		case client.Request_Error:
			return {}, models.ErrorType.InvalidResponse
		case:
			return {}, models.ErrorType.NetworkError
		}
	}
	defer client.response_destroy(&res)

	// Check HTTP status
	#partial switch res.status {
	case .OK:
		log.debug("Execute request successful (200 OK)")
	case .Bad_Request:
		log.error("Bad request (400) - invalid transaction or requestId")
		return {}, models.ErrorType.InvalidResponse
	case .Too_Many_Requests:
		log.warn("Rate limited (429)")
		return {}, models.ErrorType.RateLimited
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

	log.debug("Parsing execute response")

	// Parse JSON response
	result, parse_err := parse_execute_response(body_str, quote, input_symbol, output_symbol)
	if parse_err != .None {
		return {}, parse_err
	}

	return result, .None
}

// parse_execute_response extracts SwapTransactionResult from Ultra execute response
//
// Expected fields:
// - signature: Base58-encoded transaction signature
// - status: "confirmed" | "finalized" | "failed"
// - slot: Block slot number
// - blockTime: Unix timestamp (optional)
// - error: Error message if failed (optional)
//
// Returns: SwapTransactionResult, ErrorType
parse_execute_response :: proc(
	body_str: string,
	quote: models.SwapQuote,
	input_symbol: string,
	output_symbol: string,
) -> (models.SwapTransactionResult, models.ErrorType) {
	// Parse JSON
	json_val: json.Value
	if unmarshal_err := json.unmarshal_string(body_str, &json_val); unmarshal_err != nil {
		log.errorf("JSON unmarshal failed: %v", unmarshal_err)
		return {}, models.ErrorType.InvalidResponse
	}

	obj, is_obj := json_val.(json.Object)
	if !is_obj {
		log.error("Response is not a JSON object")
		return {}, models.ErrorType.InvalidResponse
	}

	// Extract signature (required)
	sig_val, has_sig := obj["signature"]
	if !has_sig {
		log.error("Missing 'signature' field")
		return {}, models.ErrorType.InvalidResponse
	}

	signature, is_str := sig_val.(json.String)
	if !is_str {
		log.error("'signature' field is not a string")
		return {}, models.ErrorType.InvalidResponse
	}

	// Extract status (default to "confirmed")
	status := "confirmed"
	if status_val, has_status := obj["status"]; has_status {
		if status_str, is_str := status_val.(json.String); is_str {
			status = status_str
		}
	}

	// Extract slot (optional)
	slot := u64(0)
	if slot_val, has_slot := obj["slot"]; has_slot {
		if slot_int, is_int := slot_val.(json.Integer); is_int {
			slot = u64(slot_int)
		}
	}

	// Extract blockTime (optional)
	block_time := i64(0)
	if time_val, has_time := obj["blockTime"]; has_time {
		if time_int, is_int := time_val.(json.Integer); is_int {
			block_time = i64(time_int)
		}
	}

	// Extract error message (if failed)
	error_message := ""
	if error_val, has_error := obj["error"]; has_error {
		if error_str, is_str := error_val.(json.String); is_str {
			error_message = error_str
		}
	}

	// Build result from quote data
	result := models.SwapTransactionResult{
		signature      = signature,
		slot           = slot,
		block_time     = block_time,
		status         = status,
		error_message  = error_message,
		input_amount   = quote.input_amount,
		output_amount  = quote.output_amount,
		price_impact   = quote.price_impact_pct,
		slippage_actual = 0.0, // Cannot calculate without on-chain data
		network_fee    = u64(quote.network_fee_sol * 1_000_000_000), // Convert SOL to lamports
		priority_fee   = 0, // Not provided by Ultra API
		dex            = quote.primary_dex,
		input_mint     = quote.input_mint,
		input_symbol   = input_symbol,
		output_mint    = quote.output_mint,
		output_symbol  = output_symbol,
	}

	log.debugf("Parsed execute result: signature=%s, status=%s, slot=%d", signature, status, slot)
	return result, .None
}
