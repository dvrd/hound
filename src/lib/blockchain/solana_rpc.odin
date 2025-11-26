#+feature global-context
package blockchain

import "core:encoding/base64"
import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:math"
import "core:strconv"
import client "../../http/client"
import "../models"
import "../memory"

// RPC connection configuration
RPCConnection :: struct {
	endpoint: string,
	timeout:  int, // milliseconds
}

// Solana RPC request structure
RPCRequest :: struct {
	jsonrpc: string,
	id:      int,
	method:  string,
	params:  json.Value,
}

// Solana RPC response structure
RPCResponse :: struct {
	jsonrpc: string,
	id:      int,
	result:  json.Value,
	error:   Maybe(json.Value),
}

// Account information from getAccountInfo
AccountInfo :: struct {
	data:       []u8,
	executable: bool,
	lamports:   u64,
	owner:      string,
	rent_epoch: u64,
}

// Token balance from getTokenAccountBalance
TokenBalance :: struct {
	amount:   string, // Raw amount as string
	decimals: u8,
	ui_amount: f64,
}

// Initialize RPC connection
connect_rpc :: proc(endpoint: string) -> (RPCConnection, models.ErrorType) {
	if len(endpoint) == 0 {
		return {}, .RPCConnectionFailed
	}

	return RPCConnection{endpoint = endpoint, timeout = 10000}, .None
}

// Fetch account data from Solana RPC
get_account_info :: proc(conn: RPCConnection, address: string) -> ([]u8, models.ErrorType) {
	arena_alloc := memory.request_allocator()

	// Build RPC request body using json.Value
	options := json.Object{}
	options["encoding"] = json.String("base64")
	options["commitment"] = json.String("confirmed")

	params := make(json.Array, 2, arena_alloc)
	params[0] = json.String(address)
	params[1] = options

	request_obj := json.Object{}
	request_obj["jsonrpc"] = json.String("2.0")
	request_obj["id"] = json.Integer(1)
	request_obj["method"] = json.String("getAccountInfo")
	request_obj["params"] = params

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	// Add JSON body
	if marshal_err := client.with_json(&req, request_obj); marshal_err != nil {
		return nil, .RPCInvalidResponse
	}

	// Make request
	res, err := client.request(&req, conn.endpoint)
	if err != nil {
		return nil, .RPCConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check status code
	if res.status != .OK {
		return nil, .RPCConnectionFailed
	}

	// Parse response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return nil, .RPCInvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str := body.(string)

	// Parse JSON response
	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec, arena_alloc); unmarshal_err != nil {
		return nil, .RPCInvalidResponse
	}

	// Extract result
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		return nil, .RPCInvalidResponse
	}

	// Check for RPC error
	if "error" in response_obj {
		return nil, .RPCInvalidResponse
	}

	result, has_result := response_obj["result"]
	if !has_result {
		return nil, .RPCInvalidResponse
	}

	result_obj, is_result_obj := result.(json.Object)
	if !is_result_obj {
		return nil, .RPCInvalidResponse
	}

	// Handle null value (account not found)
	value, has_value := result_obj["value"]
	if !has_value {
		return nil, .TokenNotFound
	}

	// Check if value is null
	if value == nil {
		return nil, .TokenNotFound
	}

	value_obj, is_value_obj := value.(json.Object)
	if !is_value_obj {
		return nil, .RPCInvalidResponse
	}

	// Extract data field
	data_field, has_data := value_obj["data"]
	if !has_data {
		return nil, .RPCInvalidResponse
	}

	data_array, is_data_array := data_field.(json.Array)
	if !is_data_array || len(data_array) == 0 {
		return nil, .RPCInvalidResponse
	}

	// Get base64 encoded data (first element)
	encoded_data, is_string := data_array[0].(json.String)
	if !is_string {
		return nil, .RPCInvalidResponse
	}

	// Decode base64
	decoded := base64_decode(string(encoded_data), arena_alloc)
	if len(decoded) == 0 {
		return nil, .RPCInvalidResponse
	}

	return decoded, .None
}

// Fetch token account balance from Solana RPC
get_token_balance :: proc(conn: RPCConnection, vault: string) -> (TokenBalance, models.ErrorType) {
	// Build RPC request body using json.Value
	options := json.Object{}
	options["commitment"] = json.String("confirmed")

	params := make(json.Array, 2, context.temp_allocator)
	params[0] = json.String(vault)
	params[1] = options

	request_obj := json.Object{}
	request_obj["jsonrpc"] = json.String("2.0")
	request_obj["id"] = json.Integer(2)
	request_obj["method"] = json.String("getTokenAccountBalance")
	request_obj["params"] = params

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	// Add JSON body
	if marshal_err := client.with_json(&req, request_obj); marshal_err != nil {
		return {}, .RPCInvalidResponse
	}

	// Make request
	res, err := client.request(&req, conn.endpoint)
	if err != nil {
		return {}, .RPCConnectionFailed
	}
	defer client.response_destroy(&res)

	// Check status code
	if res.status != .OK {
		return {}, .RPCConnectionFailed
	}

	// Parse response body
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return {}, .RPCInvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str := body.(string)

	// Parse JSON response
	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec); unmarshal_err != nil {
		return {}, .RPCInvalidResponse
	}

	// Extract result
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		return {}, .RPCInvalidResponse
	}

	// Check for RPC error
	if "error" in response_obj {
		return {}, .VaultFetchFailed
	}

	result, has_result := response_obj["result"]
	if !has_result {
		return {}, .RPCInvalidResponse
	}

	result_obj, is_result_obj := result.(json.Object)
	if !is_result_obj {
		return {}, .RPCInvalidResponse
	}

	// Extract value
	value, has_value := result_obj["value"]
	if !has_value {
		return {}, .VaultFetchFailed
	}

	value_obj, is_value_obj := value.(json.Object)
	if !is_value_obj {
		return {}, .VaultFetchFailed
	}

	// Extract amount
	amount_field, has_amount := value_obj["amount"]
	if !has_amount {
		return {}, .VaultFetchFailed
	}

	amount, is_amount_string := amount_field.(json.String)
	if !is_amount_string {
		return {}, .VaultFetchFailed
	}

	// Extract decimals
	decimals_field, has_decimals := value_obj["decimals"]
	if !has_decimals {
		return {}, .VaultFetchFailed
	}

	decimals_int, is_decimals_int := decimals_field.(json.Integer)
	if !is_decimals_int {
		return {}, .VaultFetchFailed
	}

	// Extract uiAmount (optional)
	ui_amount: f64 = 0.0
	if ui_amount_field, has_ui := value_obj["uiAmount"]; has_ui {
		if ui_float, is_float := ui_amount_field.(json.Float); is_float {
			ui_amount = f64(ui_float)
		}
	}

	return TokenBalance{
			amount = string(amount),
			decimals = u8(decimals_int),
			ui_amount = ui_amount,
		},
		.None
}

// Decode base64 string to bytes
base64_decode :: proc(encoded: string, allocator := context.allocator) -> []u8 {
	context.allocator = allocator
	decoded, decode_err := base64.decode(encoded)
	if decode_err != nil {
		return nil
	}
	return decoded
}

// Fetch decimals from SPL Token mint account
//
// SPL Token mint structure (82 bytes):
// - Bytes 0-35: Mint authority (COption<Pubkey>)
// - Bytes 36-43: Supply (u64)
// - Byte 44: Decimals (u8) ← TARGET
// - Byte 45: Is initialized (bool)
// - Bytes 46-81: Freeze authority (COption<Pubkey>)
//
// Error handling:
// - RPC connection failures → `.RPCConnectionFailed`
// - Invalid account data → `.RPCInvalidResponse`
// - Account not found → `.TokenNotFound`
// - Decimals out of range → `.RPCInvalidResponse`
get_token_decimals :: proc(conn: RPCConnection, mint_pubkey: [32]u8) -> (u8, models.ErrorType) {
	// Convert pubkey to base58 address
	mint_address := pubkey_to_base58(mint_pubkey)

	log.debugf("Fetching decimals for mint: %s", mint_address)

	// Fetch mint account data
	mint_data, err := get_account_info(conn, mint_address)
	if err != .None {
		log.errorf("Failed to fetch mint account: %v", err)
		return 0, err
	}
	// NO defer delete - arena resets externally

	// Validate account size (SPL Token mint is 82 bytes)
	if len(mint_data) != 82 {
		log.errorf("Invalid mint account size: %d (expected 82)", len(mint_data))
		return 0, .RPCInvalidResponse
	}

	// Extract decimals at byte 44
	decimals := mint_data[44]

	// Validate decimals (typical range: 0-18)
	if decimals > 18 {
		log.errorf("Unreasonable decimals value: %d", decimals)
		return 0, .RPCInvalidResponse
	}

	log.debugf("Mint decimals: %d", decimals)
	return decimals, .None
}

// ============================================================================
// Token Supply and Holder Methods
// ============================================================================

// TokenSupplyInfo represents the result from getTokenSupply
TokenSupplyInfo :: struct {
	amount:         string, // Raw amount as string
	decimals:       u8,
	ui_amount:      f64,    // Amount with decimals applied
	ui_amount_str:  string, // UI amount as string (for precision)
}

// TokenAccountInfo represents a token holder account
TokenAccountInfo :: struct {
	address:        string, // Account address (base58)
	amount:         string, // Raw amount as string
	decimals:       u8,
	ui_amount:      f64,    // Amount with decimals applied
}

// Fetch total token supply using getTokenSupply RPC method
//
// This method returns the total supply of an SPL Token mint.
//
// Error handling:
// - RPC connection failures → `.RPCConnectionFailed`
// - Invalid response format → `.RPCInvalidResponse`
get_token_supply :: proc(conn: RPCConnection, mint_address: string) -> (TokenSupplyInfo, models.ErrorType) {
	log.debugf("Fetching token supply for: %s", mint_address)

	// Build RPC request
	options := json.Object{}
	options["commitment"] = json.String("confirmed")

	params := make(json.Array, 2, context.temp_allocator)
	params[0] = json.String(mint_address)
	params[1] = options

	request_obj := json.Object{}
	request_obj["jsonrpc"] = json.String("2.0")
	request_obj["id"] = json.Integer(1)
	request_obj["method"] = json.String("getTokenSupply")
	request_obj["params"] = params

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	if marshal_err := client.with_json(&req, request_obj); marshal_err != nil {
		return {}, .RPCInvalidResponse
	}

	// Make request
	res, err := client.request(&req, conn.endpoint)
	if err != nil {
		return {}, .RPCConnectionFailed
	}
	defer client.response_destroy(&res)

	if res.status != .OK {
		return {}, .RPCConnectionFailed
	}

	// Parse response
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return {}, .RPCInvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str := body.(string)

	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec); unmarshal_err != nil {
		return {}, .RPCInvalidResponse
	}

	// Extract result
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		return {}, .RPCInvalidResponse
	}

	if "error" in response_obj {
		return {}, .RPCInvalidResponse
	}

	result, has_result := response_obj["result"]
	if !has_result {
		return {}, .RPCInvalidResponse
	}

	result_obj, is_result_obj := result.(json.Object)
	if !is_result_obj {
		return {}, .RPCInvalidResponse
	}

	// Extract value object
	value, has_value := result_obj["value"]
	if !has_value {
		return {}, .RPCInvalidResponse
	}

	value_obj, is_value_obj := value.(json.Object)
	if !is_value_obj {
		return {}, .RPCInvalidResponse
	}

	// Parse fields
	amount_val, has_amount := value_obj["amount"]
	if !has_amount {
		return {}, .RPCInvalidResponse
	}
	amount := string(amount_val.(json.String))

	decimals_val, has_decimals := value_obj["decimals"]
	if !has_decimals {
		return {}, .RPCInvalidResponse
	}
	decimals := u8(decimals_val.(json.Integer))

	ui_amount := 0.0
	if ui_amount_val, has_ui_amount := value_obj["uiAmount"]; has_ui_amount {
		ui_amount = ui_amount_val.(json.Float)
	}

	ui_amount_str := ""
	if ui_amount_str_val, has_ui_amount_str := value_obj["uiAmountString"]; has_ui_amount_str {
		ui_amount_str = string(ui_amount_str_val.(json.String))
	}

	return TokenSupplyInfo{
		amount = amount,
		decimals = decimals,
		ui_amount = ui_amount,
		ui_amount_str = ui_amount_str,
	}, .None
}

// Fetch top 20 token holders using getTokenLargestAccounts RPC method
//
// This method returns the 20 largest token accounts for a given mint.
// Results are ordered by balance (descending).
//
// Error handling:
// - RPC connection failures → `.RPCConnectionFailed`
// - Invalid response format → `.RPCInvalidResponse`
get_token_largest_accounts :: proc(conn: RPCConnection, mint_address: string) -> ([]TokenAccountInfo, models.ErrorType) {
	log.debugf("Fetching largest accounts for: %s", mint_address)

	// Build RPC request
	options := json.Object{}
	options["commitment"] = json.String("confirmed")

	params := make(json.Array, 2, context.temp_allocator)
	params[0] = json.String(mint_address)
	params[1] = options

	request_obj := json.Object{}
	request_obj["jsonrpc"] = json.String("2.0")
	request_obj["id"] = json.Integer(1)
	request_obj["method"] = json.String("getTokenLargestAccounts")
	request_obj["params"] = params

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	if marshal_err := client.with_json(&req, request_obj); marshal_err != nil {
		return nil, .RPCInvalidResponse
	}

	// Make request
	res, err := client.request(&req, conn.endpoint)
	if err != nil {
		return nil, .RPCConnectionFailed
	}
	defer client.response_destroy(&res)

	if res.status != .OK {
		return nil, .RPCConnectionFailed
	}

	// Parse response
	body, allocation, body_err := client.response_body(&res)
	if body_err != nil {
		return nil, .RPCInvalidResponse
	}
	defer client.body_destroy(body, allocation)

	body_str := body.(string)

	response_json: json.Value
	spec := json.Specification{}
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec); unmarshal_err != nil {
		return nil, .RPCInvalidResponse
	}

	// Extract result
	response_obj, is_obj := response_json.(json.Object)
	if !is_obj {
		return nil, .RPCInvalidResponse
	}

	if "error" in response_obj {
		return nil, .RPCInvalidResponse
	}

	result, has_result := response_obj["result"]
	if !has_result {
		return nil, .RPCInvalidResponse
	}

	result_obj, is_result_obj := result.(json.Object)
	if !is_result_obj {
		return nil, .RPCInvalidResponse
	}

	// Extract value array
	value, has_value := result_obj["value"]
	if !has_value {
		return nil, .RPCInvalidResponse
	}

	value_array, is_value_array := value.(json.Array)
	if !is_value_array {
		return nil, .RPCInvalidResponse
	}

	// Parse each account
	arena_alloc := memory.request_allocator()
	accounts := make([]TokenAccountInfo, len(value_array), arena_alloc)

	for account_val, i in value_array {
		account_obj, is_account_obj := account_val.(json.Object)
		if !is_account_obj {
			continue
		}

		// Parse address
		address_val, has_address := account_obj["address"]
		if !has_address {
			continue
		}
		address := string(address_val.(json.String))

		// Parse amount object
		amount_obj_val, has_amount_obj := account_obj["amount"]
		if !has_amount_obj {
			continue
		}

		amount_obj, is_amount_obj := amount_obj_val.(json.Object)
		if !is_amount_obj {
			continue
		}

		// Extract fields
		amount_val, has_amount := amount_obj["amount"]
		if !has_amount {
			continue
		}
		amount := string(amount_val.(json.String))

		decimals_val, has_decimals := amount_obj["decimals"]
		if !has_decimals {
			continue
		}
		decimals := u8(decimals_val.(json.Integer))

		ui_amount := 0.0
		if ui_amount_val, has_ui_amount := amount_obj["uiAmount"]; has_ui_amount {
			if ui_amount_float, ok := ui_amount_val.(json.Float); ok {
				ui_amount = ui_amount_float
			}
		}

		accounts[i] = TokenAccountInfo{
			address = address,
			amount = amount,
			decimals = decimals,
			ui_amount = ui_amount,
		}
	}

	return accounts, .None
}
