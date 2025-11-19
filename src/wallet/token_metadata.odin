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

JUPITER_TOKEN_LIST_URL :: "https://token.jup.ag/strict"

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

// JupiterToken represents a token from Jupiter's API response
JupiterToken :: struct {
	address:  string,
	chainId:  int,
	decimals: int,
	name:     string,
	symbol:   string,
	logoURI:  string,
}

// ============================================================================
// Token Metadata Discovery
// ============================================================================

// lookup_token_metadata fetches token symbol and name from Jupiter Token List
//
// This function queries Jupiter's curated token list to discover metadata
// for unknown tokens found in user wallets.
//
// ASSERTION 1: Mint address must not be empty
//
// Returns: Token metadata and error status
lookup_token_metadata :: proc(mint_address: string) -> (TokenMetadata, src.ErrorType) {
	assert(len(mint_address) > 0, "Mint address cannot be empty")

	log.debugf("Looking up token metadata for mint: %s", mint_address)

	// Make HTTP GET request to Jupiter Token List
	request: http_client.Request
	http_client.request_init(&request, .Get)
	defer http_client.request_destroy(&request)

	res, http_err := http_client.request(&request, JUPITER_TOKEN_LIST_URL)
	if http_err != nil {
		log.errorf("HTTP request failed for token lookup: %v", http_err)
		return {}, .NetworkError
	}
	defer http_client.response_destroy(&res)

	// Check HTTP status
	#partial switch res.status {
	case .OK:
		// Success
		log.debug("Jupiter Token List fetched successfully")
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

	// Parse JSON array of tokens
	tokens: []JupiterToken
	parse_err := json.unmarshal_string(body.(string), &tokens)
	if parse_err != nil {
		log.errorf("Failed to parse Jupiter token list: %v", parse_err)
		return {}, .ParseError
	}
	defer delete(tokens)

	// Search for token by mint address
	for token in tokens {
		if token.address == mint_address {
			log.infof("Found token: %s (%s)", token.symbol, token.name)

			metadata := TokenMetadata{
				address  = token.address,
				symbol   = token.symbol,
				name     = token.name,
				decimals = token.decimals,
			}

			return metadata, .None
		}
	}

	// Token not found in Jupiter list
	log.warnf("Token %s not found in Jupiter Token List", mint_address)
	return {}, .TokenNotFound
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
