// Swap UI components for menubar app
// Multi-step dialog flow for token swaps
package menubar

import "core:fmt"
import "core:log"
import "core:strconv"
import "core:strings"
import models "../lib/models"
import wallet "../lib/wallet"
import jupiter "../swap"
import tx "../transaction"

// ============================================================================
// Main Swap Dialog Entry Point
// ============================================================================

// show_swap_dialog orchestrates the multi-step swap flow
//
// Flow:
//   1. Select source token from portfolio
//   2. Enter amount
//   3. Select destination token
//   4. Fetch and display quote
//   5. Build transaction
//   6. Export transaction (clipboard or Phantom)
//
// Reference: PRPs/hound-phase2-transaction-building.md (Swap Dialog section)
show_swap_dialog :: proc(manager: ^wallet.WalletManager) -> bool {
	log.debug("Opening swap dialog")

	// Step 1: Get portfolio to populate token list
	wallets, wallet_err := wallet.get_wallets(manager)
	if wallet_err != .None || len(wallets) == 0 {
		show_error_alert("No wallets configured. Please add a wallet first.")
		return false
	}

	// Get aggregated portfolio across all wallets
	portfolio := wallet.get_aggregated_portfolio(manager)

	if len(portfolio.token_balances) == 0 && portfolio.sol_balance.amount == 0 {
		show_error_alert("Portfolio is empty. Please add tokens to your wallet.")
		return false
	}

	log.debugf("Loaded portfolio with %d tokens", len(portfolio.token_balances))

	// Step 2: Select source token
	source_mint, source_symbol, source_balance, ok1 := show_token_selection_dialog(
		"Select Source Token",
		"Choose token to swap from:",
		portfolio,
	)
	if !ok1 {
		log.debug("User cancelled source token selection")
		return false
	}

	log.debugf("Source selected: %s (%s), balance: %.6f", source_symbol, source_mint, source_balance)

	// Step 3: Input amount
	amount, ok2 := show_amount_input_dialog(source_symbol, source_balance)
	if !ok2 {
		log.debug("User cancelled amount input")
		return false
	}

	log.debugf("Amount entered: %.6f %s", amount, source_symbol)

	// Step 4: Select destination token (can be any token, not just user's portfolio)
	dest_mint, dest_symbol, ok3 := show_destination_token_dialog()
	if !ok3 {
		log.debug("User cancelled destination token selection")
		return false
	}

	log.debugf("Destination selected: %s (%s)", dest_symbol, dest_mint)

	// Step 5: Fetch quote
	show_info_alert("Fetching Quote", "Please wait while we find the best route...")

	// Look up actual decimals from token metadata
	source_token, found_source := models.get_token_by_symbol(&g_token_config, source_symbol)
	decimals := u64(9) // Default to 9 if not found
	if found_source {
		decimals = u64(models.get_token_decimals(source_token))
	}

	amount_base_units := u64(amount * f64(pow_u64(10, decimals)))

	log.debugf("Amount in base units: %d (decimals: %d)", amount_base_units, decimals)

	quote, quote_err := jupiter.get_quote(source_mint, dest_mint, amount_base_units, 50)
	if quote_err != .None {
		error_msg := fmt.tprintf("Failed to get quote: %v\n\nPossible issues:\n• No liquidity route found\n• Invalid token pair\n• Network error\n\nPlease try again.", quote_err)
		show_error_alert(error_msg)
		return false
	}

	log.debug("Quote fetched successfully")

	// Step 6: Show quote preview
	confirmed := show_quote_preview_dialog(quote, source_symbol, dest_symbol, amount, decimals)
	if !confirmed {
		log.debug("User cancelled quote preview")
		return false
	}

	// Step 7: Build transaction
	show_info_alert("Building Transaction", "Creating unsigned transaction...")

	// Get first wallet address for transaction
	first_wallet_address := wallets[0].address

	tx_response, tx_err := jupiter.build_swap_transaction(quote, first_wallet_address)
	if tx_err != .None {
		error_msg := fmt.tprintf("Failed to build transaction: %v\n\nPlease try again.", tx_err)
		show_error_alert(error_msg)
		return false
	}

	log.debugf("Transaction built, expires at block: %d", tx_response.last_valid_block_height)

	// Validate transaction
	if !tx.validate_transaction_base64(tx_response.swap_transaction) {
		show_error_alert("Transaction validation failed. Please try again.")
		return false
	}

	// Step 8: Export transaction
	show_transaction_export_dialog(tx_response.swap_transaction, source_symbol, dest_symbol)

	log.debug("Swap dialog completed successfully")
	return true
}

// ============================================================================
// Step 2: Token Selection Dialog
// ============================================================================

show_token_selection_dialog :: proc(
	title: string,
	info: string,
	portfolio: wallet.PortfolioBalance,
) -> (
	mint: string,
	symbol: string,
	balance: f64,
	ok: bool,
) {
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString(title))

	// Build token list with balances
	tokens_builder := strings.builder_make()
	defer strings.builder_destroy(&tokens_builder)

	fmt.sbprintf(&tokens_builder, "%s\n\nAvailable tokens:\n", info)

	// Add SOL
	fmt.sbprintf(&tokens_builder, "• SOL: %.6f ($%.2f)\n", portfolio.sol_balance.amount, portfolio.sol_balance.usd_value)

	// Add SPL tokens
	for token in portfolio.token_balances {
		if token.amount > 0 {
			fmt.sbprintf(&tokens_builder, "• %s: %.6f ($%.2f)\n", token.symbol, token.amount, token.usd_value)
		}
	}

	info_text := strings.to_string(tokens_builder)
	NSAlert_setInformativeText(alert, NSString_fromString(info_text))

	// Text field for symbol input
	token_field := NSTextField_newWithFrame(NSRect{x = 0, y = 0, width = 300, height = 24})
	NSTextField_setPlaceholderString(token_field, NSString_fromString("Enter token symbol (SOL, USDC, etc.)"))

	NSAlert_setAccessoryView(alert, cast(^NSView)token_field)

	NSAlert_addButtonWithTitle(alert, NSString_fromString("Continue"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Cancel"))

	response := NSAlert_runModal(alert)
	if response != 1000 do return "", "", 0, false

	// Get user input
	symbol_input_ns := NSTextField_stringValue(token_field)
	symbol_input := strings.to_upper(NSString_toString(symbol_input_ns))
	defer delete(symbol_input)

	log.debugf("User entered symbol: %s", symbol_input)

	// Find matching token
	if symbol_input == "SOL" {
		// SOL is always available - get mint from config
		sol_mint := get_token_mint_from_config("SOL")
		if sol_mint == "" {
			// Fallback to well-known SOL mint if not in config
			sol_mint = "So11111111111111111111111111111111111111112"
		}
		return sol_mint, "SOL", portfolio.sol_balance.amount, true
	}

	// Search SPL tokens
	for token in portfolio.token_balances {
		if strings.to_upper(token.symbol) == symbol_input {
			return token.mint, token.symbol, token.amount, true
		}
	}

	// Token not found
	show_error_alert(fmt.tprintf("Token '%s' not found in portfolio", symbol_input))
	return "", "", 0, false
}

// ============================================================================
// Step 3: Amount Input Dialog
// ============================================================================

show_amount_input_dialog :: proc(symbol: string, balance: f64) -> (f64, bool) {
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString(fmt.tprintf("Enter %s Amount", symbol)))
	NSAlert_setInformativeText(alert, NSString_fromString(fmt.tprintf("Available: %.6f %s", balance, symbol)))

	// Amount input field
	amount_field := NSTextField_newWithFrame(NSRect{x = 0, y = 0, width = 200, height = 24})
	NSTextField_setPlaceholderString(amount_field, NSString_fromString("0.0"))

	NSAlert_setAccessoryView(alert, cast(^NSView)amount_field)

	NSAlert_addButtonWithTitle(alert, NSString_fromString("Continue"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Max"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Cancel"))

	response := NSAlert_runModal(alert)

	if response == 1002 {
		// Cancel button
		return 0, false
	}

	if response == 1001 {
		// Max button
		log.debugf("User selected MAX: %.6f", balance)
		return balance, true
	}

	// Continue button - parse input
	amount_str := NSString_toString(NSTextField_stringValue(amount_field))
	amount, parse_ok := strconv.parse_f64(amount_str)
	if !parse_ok {
		show_error_alert("Invalid amount. Please enter a valid number.")
		return 0, false
	}

	// Validate amount
	if amount <= 0 {
		show_error_alert("Amount must be greater than 0")
		return 0, false
	}

	if amount > balance {
		show_error_alert(fmt.tprintf("Insufficient balance. Available: %.6f %s", balance, symbol))
		return 0, false
	}

	return amount, true
}

// ============================================================================
// Step 4: Destination Token Dialog
// ============================================================================

show_destination_token_dialog :: proc() -> (mint: string, symbol: string, ok: bool) {
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString("Select Destination Token"))

	info_text := "Enter destination token symbol.\n\nCommon tokens:\n• SOL\n• USDC\n• USDT\n• BONK\n• JUP\n• RAY\n\nYou can also enter any SPL token symbol."
	NSAlert_setInformativeText(alert, NSString_fromString(info_text))

	// Token symbol field
	token_field := NSTextField_newWithFrame(NSRect{x = 0, y = 0, width = 300, height = 24})
	NSTextField_setPlaceholderString(token_field, NSString_fromString("Enter destination token symbol"))

	NSAlert_setAccessoryView(alert, cast(^NSView)token_field)

	NSAlert_addButtonWithTitle(alert, NSString_fromString("Continue"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Cancel"))

	response := NSAlert_runModal(alert)
	if response != 1000 do return "", "", false

	// Get symbol
	symbol_ns := NSTextField_stringValue(token_field)
	symbol_input := strings.to_upper(NSString_toString(symbol_ns))
	defer delete(symbol_input)

	if len(symbol_input) == 0 {
		show_error_alert("Please enter a token symbol")
		return "", "", false
	}

	log.debugf("User entered destination symbol: %s", symbol_input)

	// Map symbols to mints using token configuration
	symbol_to_mint := get_token_mint_from_config(symbol_input)

	if len(symbol_to_mint) == 0 {
		show_error_alert(fmt.tprintf("Unknown token: %s\n\nPlease enter a common token symbol or the full mint address.", symbol_input))
		return "", "", false
	}

	return symbol_to_mint, symbol_input, true
}

// ============================================================================
// Step 6: Quote Preview Dialog
// ============================================================================

show_quote_preview_dialog :: proc(
	quote: jupiter.JupiterQuote,
	source_symbol: string,
	dest_symbol: string,
	amount: f64,
	decimals: u64,
) -> bool {
	// Parse amounts from quote
	in_amount_parsed, _ := strconv.parse_u64(quote.in_amount)
	out_amount_parsed, _ := strconv.parse_u64(quote.out_amount)

	// Convert to display units
	in_amount_display := f64(in_amount_parsed) / f64(pow_u64(10, decimals))
	out_amount_display := f64(out_amount_parsed) / f64(pow_u64(10, decimals)) // Assuming same decimals

	// Calculate rate
	rate := out_amount_display / in_amount_display

	// Build route description
	route_text := "Direct"
	if len(quote.route_plan) > 1 {
		route_text = fmt.tprintf("%d-step route", len(quote.route_plan))
	}

	// Build message
	message := fmt.tprintf(
		"Swap: %.6f %s → %.6f %s\n\n" +
		"Rate: 1 %s = %.6f %s\n" +
		"Route: %s\n" +
		"Slippage: %.1f%%\n" +
		"Price Impact: %s%%\n\n" +
		"Continue to build transaction?",
		in_amount_display,
		source_symbol,
		out_amount_display,
		dest_symbol,
		source_symbol,
		rate,
		dest_symbol,
		route_text,
		f64(quote.slippage_bps) / 100.0,
		quote.price_impact_pct,
	)

	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString("Swap Quote"))
	NSAlert_setInformativeText(alert, NSString_fromString(message))

	NSAlert_addButtonWithTitle(alert, NSString_fromString("Build Transaction"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Cancel"))

	response := NSAlert_runModal(alert)
	return response == 1000
}

// ============================================================================
// Step 8: Transaction Export Dialog
// ============================================================================

show_transaction_export_dialog :: proc(transaction_base64: string, source_symbol: string, dest_symbol: string) {
	message := fmt.tprintf(
		"Transaction ready for signing\n\n" +
		"Swap: %s → %s\n\n" +
		"Choose export option:\n" +
		"• Clipboard: Copy base64 transaction\n" +
		"• Phantom: Open in Phantom wallet\n\n" +
		"Transaction expires in ~60 seconds",
		source_symbol,
		dest_symbol,
	)

	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString("Export Transaction"))
	NSAlert_setInformativeText(alert, NSString_fromString(message))

	NSAlert_addButtonWithTitle(alert, NSString_fromString("📋 Copy to Clipboard"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("👻 Open in Phantom"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Close"))

	response := NSAlert_runModal(alert)

	switch response {
	case 1000: // Copy to Clipboard
		success := copy_to_clipboard(transaction_base64)
		if success {
			show_info_alert("Success", fmt.tprintf("Transaction copied to clipboard\n\n%d characters", len(transaction_base64)))
		} else {
			show_error_alert("Failed to copy transaction to clipboard")
		}

	case 1001: // Open in Phantom
		deeplink, phantom_err := tx.generate_phantom_deeplink(transaction_base64)
		if phantom_err != .None {
			show_error_alert("Failed to generate Phantom deeplink")
			return
		}

		// TODO: Open URL using NSWorkspace (requires additional bindings)
		// For now, copy deeplink to clipboard
		success := copy_to_clipboard(deeplink)
		if success {
			show_info_alert("Phantom Deeplink", fmt.tprintf("Deeplink copied to clipboard.\n\nPaste in browser to open Phantom:\n\n%s", deeplink))
		} else {
			show_error_alert("Failed to generate Phantom deeplink")
		}
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// show_info_alert displays an informational message
show_info_alert :: proc(title: string, message: string) {
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString(title))
	NSAlert_setInformativeText(alert, NSString_fromString(message))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("OK"))
	NSAlert_runModal(alert)
}

// get_token_mint_from_config looks up token mint address from global config
//
// Uses core/models token configuration instead of hardcoded addresses
get_token_mint_from_config :: proc(symbol: string) -> string {
	token, found := models.get_token_by_symbol(&g_token_config, symbol)
	if !found {
		return "" // Token not in configuration
	}
	return models.get_token_mint(token)
}

// pow_u64 calculates base^exp for u64 (simple integer power)
pow_u64 :: proc(base: u64, exp: u64) -> u64 {
	if exp == 0 do return 1
	result := base
	for i in 1 ..< exp {
		result *= base
	}
	return result
}
