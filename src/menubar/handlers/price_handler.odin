// Price handler - business logic for token price fetching
// Extracts business logic from app.odin, no UI code
package handlers

import "core:log"
import models "../../lib/models"
import dex "../../lib/dex"
import memory "../../lib/memory"
import state "../state"

// ============================================================================
// Price Fetching
// ============================================================================

// handle_fetch_price fetches price for a token symbol using DEX routing
//
// This handler implements the business logic for token price fetching:
// 1. Finds token by symbol in config
// 2. Routes price query through DEX pools with fallback
// 3. Resets request arena after RPC operations
// 4. Returns price data or error (NO UI updates)
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Symbol must not be empty
//
// Parameters:
//   - st: MenuBar state containing token config
//   - symbol: Token symbol to fetch (e.g., "sol", "usdc")
//
// Returns: Price in USD, 24h change percentage, and error status
//
// Pattern: Handler returns data only - caller decides how to display
handle_fetch_price :: proc(
	st: ^state.MenuBarState,
	symbol: string,
) -> (price_usd: f64, change_24h: f64, err: models.ErrorType) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")

	log.infof("Handling price fetch for symbol: %s", symbol)

	// Step 1: Find token by symbol in config
	token, found := models.get_token_by_symbol(&st.token_config, symbol)
	if !found {
		log.errorf("Token not found in config: %s", symbol)
		return 0.0, 0.0, .TokenNotConfigured
	}

	log.debugf("Found token: %s (%s)", token.name, token.contract_address)

	// Step 2: Route price query through DEX with fallback
	// Uses multi-DEX routing: tries pools in priority order, falls back to Jupiter API
	price_result, fetch_err := dex.route_price_query(token)
	if fetch_err != .None {
		log.errorf("Failed to fetch price from DEX router: %v", fetch_err)
		return 0.0, 0.0, fetch_err
	}

	log.debugf("Price fetched from %v: $%.6f", price_result.source, price_result.price_usd)

	// Step 3: Fetch 24h change (uses separate cached API call)
	// NOTE: 24h change is less volatile, has 5-minute cache
	change_data, change_err := dex.fetch_24h_change(token.contract_address)
	if change_err != .None {
		log.warnf("Failed to fetch 24h change (non-fatal): %v", change_err)
		// Continue with 0.0 change if fetch fails
		change_data = 0.0
	}

	log.debugf("24h change: %.2f%%", change_data)

	// Step 4: Reset request arena to clean up RPC allocations
	// CRITICAL: Request arena must be reset after each RPC operation
	memory.reset_request_arena()
	log.debug("Request arena reset after RPC operations")

	// ASSERTION: Validate price is non-negative
	assert(price_result.price_usd >= 0, "Price must be non-negative")

	log.infof("Price fetch successful: $%.6f (%.2f%% 24h)", price_result.price_usd, change_data)

	return price_result.price_usd, change_data, .None
}

// ============================================================================
// Helper Functions
// ============================================================================

// validate_price_inputs validates inputs for price fetching
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Symbol must not be empty
//
// Returns: Error status
validate_price_inputs :: proc(st: ^state.MenuBarState, symbol: string) -> models.ErrorType {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")

	// Check if symbol exists in config
	_, found := models.get_token_by_symbol(&st.token_config, symbol)
	if !found {
		log.errorf("Symbol not configured: %s", symbol)
		return .TokenNotConfigured
	}

	return .None
}

// get_token_from_state retrieves token from state by symbol
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Symbol must not be empty
//
// Returns: Token and found flag
get_token_from_state :: proc(
	st: ^state.MenuBarState,
	symbol: string,
) -> (token: models.Token, found: bool) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")

	return models.get_token_by_symbol(&st.token_config, symbol)
}
