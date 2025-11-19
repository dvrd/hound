// Token metadata discovery via Jupiter Token List API
// Automatically discovers token symbol and name for unknown tokens
#+feature dynamic-literals
package wallet

import "core:encoding/json"
import "core:fmt"
import "core:log"
import http_client "../../vendor/odin-http/client"
import src "../"

// ============================================================================
// Constants
// ============================================================================

// Jupiter Token API V2
JUPITER_TOKEN_API_BASE :: "https://lite-api.jup.ag/tokens/v2"

// ============================================================================
// Types
// ============================================================================

// TokenMetadata represents discovered token information
TokenMetadata :: struct {
	address:  string,
	symbol:   string,
	name:     string,
	decimals: int,
}

// JupiterToken represents a token from Jupiter's API V2 response
JupiterToken :: struct {
	id:       string, // mint address
	name:     string,
	symbol:   string,
	icon:     string,
	decimals: int,
}

// ============================================================================
// Token Metadata Discovery
// ============================================================================

// lookup_token_metadata fetches token symbol and name from Jupiter Token API V2
//
// This function queries Jupiter's Token API V2 search endpoint to discover metadata
// for unknown tokens found in user wallets.
//
// ASSERTION 1: Mint address must not be empty
//
// Returns: Token metadata and error status
lookup_token_metadata :: proc(mint_address: string) -> (TokenMetadata, src.ErrorType) {
	assert(len(mint_address) > 0, "Mint address cannot be empty")

	log.debugf("Looking up token metadata for mint: %s", mint_address)

	// Build search URL with mint address as query parameter
	// Using aprintf to allocate string that we can properly free
	search_url := fmt.aprintf("%s/search?query=%s", JUPITER_TOKEN_API_BASE, mint_address)
	defer delete(search_url)

	// Make HTTP GET request to Jupiter Token API V2
	request: http_client.Request
	http_client.request_init(&request, .Get)
	defer http_client.request_destroy(&request)

	res, http_err := http_client.request(&request, search_url)
	if http_err != nil {
		log.errorf("HTTP request failed for token lookup: %v", http_err)
		return {}, .NetworkError
	}
	defer http_client.response_destroy(&res)

	// Check HTTP status
	#partial switch res.status {
	case .OK:
		// Success
		log.debug("Jupiter Token API V2 response received")
	case:
		log.warnf("HTTP error status from Jupiter: %v", res.status)
		return {}, .NetworkError
	}

	// Extract response body
	body, allocation, body_err := http_client.response_body(&res)
	if body_err != nil {
		log.errorf("Failed to extract response body: %v", body_err)
		return {}, .NetworkError
	}
	defer http_client.body_destroy(body, allocation)

	// Parse JSON array of token results
	tokens: []JupiterToken
	parse_err := json.unmarshal_string(body.(string), &tokens)
	if parse_err != nil {
		log.errorf("Failed to parse Jupiter token response: %v", parse_err)
		return {}, .ParseError
	}
	defer delete(tokens)

	// Check if we got any results
	if len(tokens) == 0 {
		log.warnf("Token %s not found in Jupiter Token API", mint_address)
		return {}, .TokenNotFound
	}

	// Use first result (exact match)
	token := tokens[0]
	log.infof("Found token: %s (%s)", token.symbol, token.name)

	metadata := TokenMetadata{
		address  = token.id,
		symbol   = token.symbol,
		name     = token.name,
		decimals = token.decimals,
	}

	return metadata, .None
}

// ============================================================================
// Database Integration
// ============================================================================

// save_discovered_token stores discovered token metadata in database
//
// This allows the app to remember token metadata for future wallet refreshes
// without needing to query Jupiter again.
//
// ASSERTION 1: Database must not be nil
// ASSERTION 2: Metadata must have valid symbol
//
// Returns: Error status
save_discovered_token :: proc(
	db: ^src.Database,
	metadata: TokenMetadata,
) -> src.ErrorType {
	assert(db != nil, "Database cannot be nil")
	assert(len(metadata.symbol) > 0, "Token symbol cannot be empty")

	log.debugf("Saving discovered token to database: %s (%s)", metadata.symbol, metadata.address)

	// Insert token into database
	// We'll add it as a token with no pools (wallet-only token)
	token := src.Token{
		symbol           = metadata.symbol,
		name             = metadata.name,
		contract_address = metadata.address,
		chain            = "solana",
		pools            = {},  // No pools - just for display
		is_quote_token   = false,
		usd_price        = 0.0,
	}

	// Use existing insert_token function
	insert_err := src.insert_token(db, token)
	if insert_err != .None {
		log.errorf("Failed to save token to database: %v", insert_err)
		return insert_err
	}

	log.infof("Token %s saved to database", metadata.symbol)
	return .None
}
