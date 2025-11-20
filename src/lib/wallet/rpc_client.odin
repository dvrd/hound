// Solana RPC client for blockchain interaction
// Implements JSON-RPC calls with automatic failover to backup endpoints
#+feature dynamic-literals
package wallet

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:net"
import "core:bufio"
import "core:strings"
import http_client "../../../odin-http/client"
import "../models"
import "../blockchain"

// ============================================================================
// Types
// ============================================================================

// RPCClient handles Solana RPC communication with failover support
RPCClient :: struct {
	endpoint:               string,            // Primary RPC endpoint
	backup_endpoints:       []string,          // Failover endpoints
	current_endpoint_index: int,               // Currently active endpoint (0 = primary)
	request_id:             int,               // JSON-RPC request counter
}

// JSON-RPC request structure
RPCRequest :: struct {
	jsonrpc: string,         // Always "2.0"
	id:      int,            // Request ID (for matching responses)
	method:  string,         // RPC method name
	params:  json.Array,     // Method parameters
}

// JSON-RPC response structure
RPCResponse :: struct {
	jsonrpc: string,
	id:      int,
	result:  json.Value,     // Successful result
	error:   Maybe(RPCError), // Error if request failed
}

// JSON-RPC error structure
RPCError :: struct {
	code:    int,
	message: string,
}

// Token account returned by getTokenAccountsByOwner
TokenAccount :: struct {
	pubkey:  string,         // Token account address
	account: TokenAccountData,
}

// Token account data
TokenAccountData :: struct {
	mint:       string,      // Token mint address
	owner:      string,      // Owner address
	amount:     u64,         // Raw amount (with decimals)
	decimals:   int,         // Number of decimals
	ui_amount:  f64,         // Human-readable amount
}

// ============================================================================
// Initialization
// ============================================================================

// init_rpc_client creates a new RPC client with failover support
//
// ASSERTION 1: Primary endpoint must not be empty
//
// Returns: Initialized RPC client
init_rpc_client :: proc(endpoint: string, backups: []string) -> RPCClient {
	assert(len(endpoint) > 0, "Primary endpoint cannot be empty")

	log.debugf("Initializing RPC client with endpoint: %s", endpoint)
	log.debugf("Backup endpoints: %d configured", len(backups))

	return RPCClient{
		endpoint = endpoint,
		backup_endpoints = backups,
		current_endpoint_index = 0,
		request_id = 1,
	}
}

// ============================================================================
// RPC Methods - Balance Queries
// ============================================================================

// get_balance fetches SOL balance for an address (in lamports: 1 SOL = 10^9 lamports)
//
// ASSERTION 1: Client must not be nil
// ASSERTION 2: Address must not be empty
//
// Returns: Balance in lamports and error status
get_balance :: proc(client: ^RPCClient, address: string) -> (balance: u64, err: models.ErrorType) {
	assert(client != nil, "RPC client cannot be nil")
	assert(len(address) > 0, "Address cannot be empty")

	log.debugf("Fetching balance for address: %s", address)

	// Build params array
	params := json.Array{address}

	// Make RPC request
	response_value, rpc_err := rpc_request(client, "getBalance", params)
	if rpc_err != .None {
		return 0, rpc_err
	}

	// Parse response: {"value": 1234567890}
	result_obj, is_obj := response_value.(json.Object)
	if !is_obj {
		log.error("Balance response is not an object")
		return 0, .RPCInvalidResponse
	}

	value_field, value_exists := result_obj["value"]
	if !value_exists {
		log.error("Balance response missing 'value' field")
		return 0, .RPCInvalidResponse
	}

	// Convert to u64
	#partial switch v in value_field {
	case json.Integer:
		balance = u64(v)
	case json.Float:
		balance = u64(v)
	case:
		log.errorf("Balance value has unexpected type: %T", value_field)
		return 0, .RPCInvalidResponse
	}

	log.debugf("Balance fetched: %d lamports", balance)
	return balance, .None
}

// get_token_accounts_by_owner fetches all SPL token accounts owned by an address
//
// ASSERTION 1: Client must not be nil
// ASSERTION 2: Owner address must not be empty
//
// Returns: Array of token accounts and error status
get_token_accounts_by_owner :: proc(
	client: ^RPCClient,
	owner: string,
) -> (accounts: []TokenAccount, err: models.ErrorType) {
	assert(client != nil, "RPC client cannot be nil")
	assert(len(owner) > 0, "Owner address cannot be empty")

	log.debugf("Fetching token accounts for owner: %s", owner)

	// Build params:
	// [owner_address, {programId: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"},
	//  {encoding: "jsonParsed"}]
	filter := json.Object{
		"programId" = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
	}
	config := json.Object{
		"encoding" = "jsonParsed",
	}
	params := json.Array{owner, filter, config}

	// Make RPC request
	response_value, rpc_err := rpc_request(client, "getTokenAccountsByOwner", params)
	if rpc_err != .None {
		return nil, rpc_err
	}

	// Parse response: {"value": [{pubkey, account}, ...]}
	result_obj, is_obj := response_value.(json.Object)
	if !is_obj {
		log.error("Token accounts response is not an object")
		return nil, .RPCInvalidResponse
	}

	value_field, value_exists := result_obj["value"]
	if !value_exists {
		log.error("Token accounts response missing 'value' field")
		return nil, .RPCInvalidResponse
	}

	value_array, is_array := value_field.(json.Array)
	if !is_array {
		log.error("Token accounts 'value' is not an array")
		return nil, .RPCInvalidResponse
	}

	// Parse each token account
	account_list := make([dynamic]TokenAccount, 0, len(value_array))

	for item in value_array {
		item_obj, item_is_obj := item.(json.Object)
		if !item_is_obj {
			log.warn("Token account item is not an object, skipping")
			continue
		}

		// Extract pubkey
		pubkey_val, pubkey_exists := item_obj["pubkey"]
		if !pubkey_exists {
			log.warn("Token account missing 'pubkey', skipping")
			continue
		}
		pubkey, pubkey_is_str := pubkey_val.(json.String)
		if !pubkey_is_str {
			log.warn("Token account pubkey is not a string, skipping")
			continue
		}

		// Extract account data
		account_val, account_exists := item_obj["account"]
		if !account_exists {
			log.warn("Token account missing 'account', skipping")
			continue
		}
		account_obj, account_is_obj := account_val.(json.Object)
		if !account_is_obj {
			log.warn("Token account 'account' is not an object, skipping")
			continue
		}

		// Extract parsed data
		data_val, data_exists := account_obj["data"]
		if !data_exists {
			log.warn("Token account missing 'data', skipping")
			continue
		}
		data_obj, data_is_obj := data_val.(json.Object)
		if !data_is_obj {
			log.warn("Token account 'data' is not an object, skipping")
			continue
		}

		parsed_val, parsed_exists := data_obj["parsed"]
		if !parsed_exists {
			log.warn("Token account missing 'parsed', skipping")
			continue
		}
		parsed_obj, parsed_is_obj := parsed_val.(json.Object)
		if !parsed_is_obj {
			log.warn("Token account 'parsed' is not an object, skipping")
			continue
		}

		info_val, info_exists := parsed_obj["info"]
		if !info_exists {
			log.warn("Token account missing 'info', skipping")
			continue
		}
		info_obj, info_is_obj := info_val.(json.Object)
		if !info_is_obj {
			log.warn("Token account 'info' is not an object, skipping")
			continue
		}

		// Extract fields: mint, owner, tokenAmount
		mint_val, mint_exists := info_obj["mint"]
		if !mint_exists {
			log.warn("Token account missing 'mint', skipping")
			continue
		}
		mint, mint_is_str := mint_val.(json.String)
		if !mint_is_str {
			log.warn("Token account mint is not a string, skipping")
			continue
		}

		owner_val, owner_exists := info_obj["owner"]
		if !owner_exists {
			log.warn("Token account missing 'owner', skipping")
			continue
		}
		owner_str, owner_is_str := owner_val.(json.String)
		if !owner_is_str {
			log.warn("Token account owner is not a string, skipping")
			continue
		}

		token_amount_val, token_amount_exists := info_obj["tokenAmount"]
		if !token_amount_exists {
			log.warn("Token account missing 'tokenAmount', skipping")
			continue
		}
		token_amount_obj, token_amount_is_obj := token_amount_val.(json.Object)
		if !token_amount_is_obj {
			log.warn("Token account 'tokenAmount' is not an object, skipping")
			continue
		}

		// Extract amount, decimals, uiAmount
		amount_val, amount_exists := token_amount_obj["amount"]
		if !amount_exists {
			log.warn("Token amount missing 'amount', skipping")
			continue
		}
		amount_str, amount_is_str := amount_val.(json.String)
		if !amount_is_str {
			log.warn("Token amount 'amount' is not a string, skipping")
			continue
		}
		amount: u64 = 0
		// Parse amount string to u64
		for char in amount_str {
			amount = amount * 10 + u64(char - '0')
		}

		decimals_val, decimals_exists := token_amount_obj["decimals"]
		if !decimals_exists {
			log.warn("Token amount missing 'decimals', skipping")
			continue
		}
		decimals_int, decimals_is_int := decimals_val.(json.Integer)
		if !decimals_is_int {
			log.warn("Token amount 'decimals' is not an integer, skipping")
			continue
		}
		decimals := int(decimals_int)

		ui_amount_val, ui_amount_exists := token_amount_obj["uiAmount"]
		if !ui_amount_exists {
			log.warn("Token amount missing 'uiAmount', skipping")
			continue
		}
		ui_amount: f64
		#partial switch v in ui_amount_val {
		case json.Integer:
			ui_amount = f64(v)
		case json.Float:
			ui_amount = f64(v)
		case json.Null:
			ui_amount = 0.0
		case:
			log.warn("Token amount 'uiAmount' has unexpected type, using 0")
			ui_amount = 0.0
		}

		// Create token account
		token_account := TokenAccount{
			pubkey = string(pubkey),
			account = TokenAccountData{
				mint      = string(mint),
				owner     = string(owner_str),
				amount    = amount,
				decimals  = decimals,
				ui_amount = ui_amount,
			},
		}

		append(&account_list, token_account)
	}

	log.infof("Fetched %d token account(s)", len(account_list))
	return account_list[:], .None
}

// ============================================================================
// Low-Level RPC Communication
// ============================================================================

// rpc_request makes a JSON-RPC call to Solana with automatic failover
//
// ASSERTION 1: Client must not be nil
// ASSERTION 2: Method must not be empty
//
// Returns: JSON response value and error status
rpc_request :: proc(
	client: ^RPCClient,
	method: string,
	params: json.Array,
) -> (result: json.Value, err: models.ErrorType) {
	assert(client != nil, "RPC client cannot be nil")
	assert(len(method) > 0, "RPC method cannot be empty")

	log.debugf("Making RPC request: %s", method)

	// Try primary endpoint first, then failover to backups
	max_attempts := 1 + len(client.backup_endpoints)

	for attempt := 0; attempt < max_attempts; attempt += 1 {
		// Select endpoint
		endpoint: string
		if client.current_endpoint_index == 0 {
			endpoint = client.endpoint
		} else if client.current_endpoint_index <= len(client.backup_endpoints) {
			endpoint = client.backup_endpoints[client.current_endpoint_index - 1]
		} else {
			// Exhausted all endpoints - reset and fail
			client.current_endpoint_index = 0
			log.error("All RPC endpoints failed")
			return nil, .RPCConnectionFailed
		}

		log.debugf("Attempt %d/%d - using endpoint: %s", attempt + 1, max_attempts, endpoint)

		// Build request
		request_obj := RPCRequest{
			jsonrpc = "2.0",
			id      = client.request_id,
			method  = method,
			params  = params,
		}
		client.request_id += 1

		// Make HTTP POST request
		request: http_client.Request
		http_client.request_init(&request, .Post)
		defer http_client.request_destroy(&request)

		// Set JSON body
		json_err := http_client.with_json(&request, request_obj)
		if json_err != nil {
			log.errorf("Failed to marshal JSON request: %v", json_err)
			// Try next endpoint
			client.current_endpoint_index += 1
			continue
		}

		// Send request
		res, http_err := http_client.request(&request, endpoint)
		if http_err != nil {
			log.warnf("HTTP request failed (endpoint %d): %v", client.current_endpoint_index, http_err)
			// Try next endpoint
			client.current_endpoint_index += 1
			continue
		}
		defer http_client.response_destroy(&res)

		// Check HTTP status
		#partial switch res.status {
		case .OK:
			// Success - parse response
			log.debug("HTTP 200 OK")
		case:
			log.warnf("HTTP error status: %v", res.status)
			// Try next endpoint
			client.current_endpoint_index += 1
			continue
		}

		// Extract response body using helper function
		body, allocation, body_err := http_client.response_body(&res)
		if body_err != nil {
			log.errorf("Failed to extract response body: %v", body_err)
			// Try next endpoint
			client.current_endpoint_index += 1
			continue
		}
		defer http_client.body_destroy(body, allocation)

		// Parse JSON (body is string)
		response: RPCResponse
		parse_err := json.unmarshal_string(body.(string), &response)
		if parse_err != nil {
			log.errorf("Failed to parse RPC response: %v", parse_err)
			// Try next endpoint
			client.current_endpoint_index += 1
			continue
		}

		// Check for RPC error
		if error, has_error := response.error.?; has_error {
			log.errorf("RPC error: [%d] %s", error.code, error.message)
			return nil, .RPCInvalidResponse
		}

		// Success - return result
		log.debugf("RPC request succeeded: %s", method)
		return response.result, .None
	}

	// All attempts failed
	log.error("RPC request failed after all attempts")
	return nil, .RPCConnectionFailed
}
