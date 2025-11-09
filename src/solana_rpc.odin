#+feature global-context
package main

import "core:encoding/base64"
import "core:encoding/json"
import "core:fmt"
import "core:strconv"
import client "../vendor/odin-http/client"

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
connect_rpc :: proc(endpoint: string) -> (RPCConnection, ErrorType) {
	if len(endpoint) == 0 {
		return {}, .RPCConnectionFailed
	}

	return RPCConnection{endpoint = endpoint, timeout = 10000}, .None
}

// Fetch account data from Solana RPC
get_account_info :: proc(conn: RPCConnection, address: string) -> ([]u8, ErrorType) {
	// Build RPC request body
	RPC_Options :: struct {
		encoding:   string,
		commitment: string,
	}

	RPC_Request :: struct {
		jsonrpc: string,
		id:      int,
		method:  string,
		params:  []any,
	}

	options := RPC_Options{encoding = "base64", commitment = "confirmed"}
	params := []any{address, options}

	rpc_req := RPC_Request{jsonrpc = "2.0", id = 1, method = "getAccountInfo", params = params}

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	// Add JSON body
	if marshal_err := client.with_json(&req, rpc_req); marshal_err != nil {
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
	if unmarshal_err := json.unmarshal_string(body_str, &response_json, spec); unmarshal_err != nil {
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
	decoded := base64_decode(string(encoded_data))
	if len(decoded) == 0 {
		return nil, .RPCInvalidResponse
	}

	return decoded, .None
}

// Fetch token account balance from Solana RPC
get_token_balance :: proc(conn: RPCConnection, vault: string) -> (TokenBalance, ErrorType) {
	// Build RPC request body
	RPC_Options :: struct {
		commitment: string,
	}

	RPC_Request :: struct {
		jsonrpc: string,
		id:      int,
		method:  string,
		params:  []any,
	}

	options := RPC_Options{commitment = "confirmed"}
	params := []any{vault, options}

	rpc_req := RPC_Request{jsonrpc = "2.0", id = 2, method = "getTokenAccountBalance", params = params}

	// Create HTTP request
	req: client.Request
	client.request_init(&req, .Post)
	defer client.request_destroy(&req)

	// Add JSON body
	if marshal_err := client.with_json(&req, rpc_req); marshal_err != nil {
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
base64_decode :: proc(encoded: string) -> []u8 {
	decoded, decode_err := base64.decode(encoded)
	if decode_err != nil {
		return nil
	}
	return decoded
}
