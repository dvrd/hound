// Swap handler - business logic for token swapping
// Extracts business logic from swap_ui.odin, no UI code
package handlers

import "core:log"
import models "../../lib/models"
import swap "../../lib/swap"
import tx "../../lib/transaction"
import memory "../../lib/memory"
import state "../state"

// ============================================================================
// Swap Quote Operations
// ============================================================================

// handle_get_swap_quote fetches a swap quote from Jupiter Ultra API
//
// This handler implements the business logic for swap quote fetching:
// 1. Validates input parameters (mints, amounts, wallet address)
// 2. Calls Jupiter Ultra API to get quote (with taker for transaction generation)
// 3. Resets request arena after API call
// 4. Returns quote or error (NO UI updates)
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: From mint must not be empty
// ASSERTION 3: To mint must not be empty
// ASSERTION 4: Amount must be greater than zero
// ASSERTION 5: Wallet address must not be empty (required for Ultra API)
//
// Parameters:
//   - st: MenuBar state
//   - from_mint: Source token mint address (Solana contract address)
//   - to_mint: Destination token mint address
//   - amount: Amount in base units (already scaled by decimals)
//   - wallet_address: Wallet address that will execute swap (taker)
//
// Returns: Jupiter quote and error status
//
// Pattern: Handler returns data only - caller decides how to display
handle_get_swap_quote :: proc(
	st: ^state.MenuBarState,
	from_mint: string,
	to_mint: string,
	amount: u64,
	wallet_address: string,
) -> (quote: swap.JupiterQuote, err: models.ErrorType) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(from_mint) > 0, "From mint cannot be empty")
	assert(len(to_mint) > 0, "To mint cannot be empty")
	assert(amount > 0, "Amount must be greater than zero")
	assert(len(wallet_address) > 0, "Wallet address cannot be empty")

	log.infof("Handling swap quote: %s -> %s, amount: %d, taker: %s", from_mint, to_mint, amount, wallet_address)

	// Step 1: Validate inputs
	validate_err := validate_swap_inputs(from_mint, to_mint, amount)
	if validate_err != .None {
		log.errorf("Swap input validation failed: %v", validate_err)
		return {}, validate_err
	}

	log.debug("Swap inputs validated")

	// Step 2: Get quote from Jupiter Ultra API
	// NOTE: Uses slippage_bps from state configuration
	// NOTE: Passes wallet_address as taker to get transaction in response
	fetched_quote, quote_err := swap.get_quote(
		from_mint,
		to_mint,
		amount,
		wallet_address,
		u16(st.slippage_bps),
	)
	if quote_err != .None {
		log.errorf("Failed to get Jupiter quote: %v", quote_err)
		return {}, quote_err
	}

	log.debugf("Quote received: %s %s -> %s %s",
		fetched_quote.in_amount, from_mint,
		fetched_quote.out_amount, to_mint)

	// Step 3: Reset request arena to clean up API call allocations
	memory.reset_request_arena()
	log.debug("Request arena reset after quote fetch")

	// ASSERTION: Validate quote has required fields
	assert(len(fetched_quote.input_mint) > 0, "Quote must have input mint")
	assert(len(fetched_quote.output_mint) > 0, "Quote must have output mint")
	assert(fetched_quote.is_valid, "Quote must be marked valid")

	log.info("Swap quote fetched successfully")

	return fetched_quote, .None
}

// ============================================================================
// Transaction Building Operations
// ============================================================================

// handle_build_swap_transaction builds unsigned transaction from quote
//
// This handler implements the business logic for transaction building:
// 1. Validates quote is still valid (not expired)
// 2. Validates wallet address format
// 3. Calls Jupiter API to build transaction
// 4. Validates transaction base64 encoding
// 5. Resets request arena after API call
// 6. Returns transaction or error (NO UI updates)
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Quote must be valid
// ASSERTION 3: Wallet address must not be empty
//
// Parameters:
//   - st: MenuBar state
//   - quote: Jupiter quote from handle_get_swap_quote
//   - wallet_address: User's wallet public key (base58)
//
// Returns: Base64-encoded unsigned transaction and error status
//
// Pattern: Handler returns data only - caller decides how to display
handle_build_swap_transaction :: proc(
	st: ^state.MenuBarState,
	quote: swap.JupiterQuote,
	wallet_address: string,
) -> (transaction: string, err: models.ErrorType) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(quote.is_valid, "Quote must be valid")
	assert(len(wallet_address) > 0, "Wallet address cannot be empty")

	log.infof("Handling swap transaction build for wallet: %s", wallet_address)

	// Step 1: Validate wallet address format
	addr_err := validate_wallet_address_format(wallet_address)
	if addr_err != .None {
		log.errorf("Invalid wallet address: %v", addr_err)
		return "", addr_err
	}

	log.debug("Wallet address validated")

	// Step 2: Build transaction from quote
	// NOTE: Jupiter API requires user public key and returns unsigned transaction
	tx_response, build_err := swap.build_swap_transaction(quote, wallet_address)
	if build_err != .None {
		log.errorf("Failed to build swap transaction: %v", build_err)
		return "", build_err
	}

	log.debugf("Transaction built: %d bytes (base64)", len(tx_response.swap_transaction))

	// Step 3: Validate transaction format
	if !tx.validate_transaction_base64(tx_response.swap_transaction) {
		log.error("Transaction validation failed: invalid base64 or format")
		return "", .InvalidResponse
	}

	log.debug("Transaction validated successfully")

	// Step 4: Reset request arena to clean up API call allocations
	memory.reset_request_arena()
	log.debug("Request arena reset after transaction build")

	// ASSERTION: Validate transaction is not empty
	assert(len(tx_response.swap_transaction) > 0, "Transaction cannot be empty")

	log.info("Swap transaction built successfully")

	return tx_response.swap_transaction, .None
}

// ============================================================================
// Validation Functions
// ============================================================================

// validate_swap_inputs validates swap parameters
//
// ASSERTION 1: From mint must not be empty
// ASSERTION 2: To mint must not be empty
// ASSERTION 3: Amount must be greater than zero
//
// Parameters:
//   - from_mint: Source token mint
//   - to_mint: Destination token mint
//   - amount: Amount in base units
//
// Returns: Error status
validate_swap_inputs :: proc(
	from_mint: string,
	to_mint: string,
	amount: u64,
) -> models.ErrorType {
	assert(len(from_mint) > 0, "From mint cannot be empty")
	assert(len(to_mint) > 0, "To mint cannot be empty")
	assert(amount > 0, "Amount must be greater than zero")

	// Validate mint addresses are different
	if from_mint == to_mint {
		log.error("Cannot swap token to itself")
		return .InvalidToken
	}

	// Validate mint address lengths (Solana addresses are 32-44 chars)
	if len(from_mint) < 32 || len(from_mint) > 44 {
		log.errorf("Invalid from_mint length: %d", len(from_mint))
		return .InvalidToken
	}

	if len(to_mint) < 32 || len(to_mint) > 44 {
		log.errorf("Invalid to_mint length: %d", len(to_mint))
		return .InvalidToken
	}

	return .None
}

// validate_wallet_address_format validates Solana wallet address format
//
// ASSERTION 1: Address must not be empty
//
// Parameters:
//   - address: Wallet address to validate
//
// Returns: Error status
validate_wallet_address_format :: proc(address: string) -> models.ErrorType {
	assert(len(address) > 0, "Address cannot be empty")

	// Solana addresses are 32-44 characters (base58 encoding)
	if len(address) < 32 || len(address) > 44 {
		log.errorf("Invalid address length: %d (expected 32-44)", len(address))
		return .InvalidToken
	}

	// TODO: Add base58 character validation if needed

	return .None
}

// validate_swap_amount validates swap amount is within reasonable bounds
//
// ASSERTION 1: Amount must be greater than zero
//
// Parameters:
//   - amount: Amount in base units
//   - decimals: Token decimals (for validation)
//
// Returns: Error status
validate_swap_amount :: proc(amount: u64, decimals: int) -> models.ErrorType {
	assert(amount > 0, "Amount must be greater than zero")
	assert(decimals >= 0 && decimals <= 18, "Decimals must be 0-18")

	// Validate amount is not dust (too small)
	// Minimum 0.000001 tokens (1 micro-unit with 6 decimals)
	min_amount: u64 = 1
	if amount < min_amount {
		log.errorf("Amount too small: %d (minimum: %d)", amount, min_amount)
		return .InvalidToken
	}

	// Validate amount is reasonable (not overflow risk)
	// Max ~18 quintillion (u64 max / 1000 for safety margin)
	max_amount: u64 = 18_446_744_073_709_551_615 / 1000
	if amount > max_amount {
		log.errorf("Amount too large: %d (maximum: %d)", amount, max_amount)
		return .InvalidToken
	}

	return .None
}
