// Swap view - pure UI layer for token swap dialogs
// No business logic, no service calls, only AppKit dialog creation
package views

import "core:fmt"
import "core:log"
import "core:strconv"
import "core:strings"
import wallet "../../lib/wallet"
import swap "../../lib/swap"
import state "../state"
import appkit "../../appkit"

// ============================================================================
// Token Selection Dialog
// ============================================================================

// show_token_selection_dialog shows token picker from portfolio
//
// ASSERTION 1: State must not be nil
//
// Returns: Selected token mint, symbol, balance, and success status
show_token_selection_dialog :: proc(
	st: ^state.MenuBarState,
	portfolio: wallet.PortfolioBalance,
) -> (mint: string, symbol: string, balance: f64, ok: bool) {
	assert(st != nil, "MenuBarState cannot be nil")

	log.debug("[swap_view] Showing token selection dialog")

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(alert, appkit.NSString_fromString("Select Token"))

	// Build token list with balances
	info_builder := strings.builder_make()
	defer strings.builder_destroy(&info_builder)

	strings.write_string(&info_builder, "Choose source token to swap:\n\n")
	strings.write_string(&info_builder, "Available tokens:\n")

	// Add SOL
	fmt.sbprintf(&info_builder, "• SOL: %.6f ($%.2f)\n",
		portfolio.sol_balance.amount,
		portfolio.sol_balance.usd_value)

	// Add SPL tokens with non-zero balance
	for token in portfolio.token_balances {
		if token.amount > 0 {
			fmt.sbprintf(&info_builder, "• %s: %.6f ($%.2f)\n",
				token.symbol,
				token.amount,
				token.usd_value)
		}
	}

	info_text := strings.to_string(info_builder)
	appkit.NSAlert_setInformativeText(alert, appkit.NSString_fromString(info_text))

	// Text field for symbol input
	token_field := appkit.NSTextField_newWithFrame(
		appkit.NSRect{x = 0, y = 0, width = 300, height = 24},
	)
	appkit.NSTextField_setPlaceholderString(
		token_field,
		appkit.NSString_fromString("Enter token symbol (SOL, USDC, etc.)"),
	)

	appkit.NSAlert_setAccessoryView(alert, cast(^appkit.NSView)token_field)

	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Continue"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Cancel"))

	response := appkit.NSAlert_runModal(alert)
	if response != 1000 {
		log.debug("[swap_view] Token selection cancelled")
		return "", "", 0, false
	}

	// Get user input
	symbol_input_ns := appkit.NSTextField_stringValue(token_field)
	symbol_input := strings.to_upper(appkit.NSString_toString(symbol_input_ns))
	defer delete(symbol_input)

	log.debugf("[swap_view] User selected token: %s", symbol_input)

	// Find matching token in portfolio
	if symbol_input == "SOL" {
		return portfolio.sol_balance.mint, "SOL", portfolio.sol_balance.amount, true
	}

	// Search SPL tokens
	for token in portfolio.token_balances {
		if strings.to_upper(token.symbol) == symbol_input {
			return token.mint, token.symbol, token.amount, true
		}
	}

	// Token not found
	error_msg := fmt.tprintf("Token '%s' not found in portfolio", symbol_input)
	show_error_alert(error_msg)
	log.warnf("[swap_view] Token not found: %s", symbol_input)
	return "", "", 0, false
}

// ============================================================================
// Amount Input Dialog
// ============================================================================

// show_amount_input_dialog shows amount input with validation
//
// ASSERTION 1: Symbol must not be empty
// ASSERTION 2: Balance must be non-negative
//
// Returns: Selected amount and success status
show_amount_input_dialog :: proc(
	symbol: string,
	balance: f64,
) -> (amount: f64, ok: bool) {
	assert(len(symbol) > 0, "Symbol cannot be empty")
	assert(balance >= 0, "Balance must be non-negative")

	log.debugf("[swap_view] Showing amount input dialog: %s (balance: %.6f)", symbol, balance)

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(
		alert,
		appkit.NSString_fromString(fmt.tprintf("Enter %s Amount", symbol)),
	)
	appkit.NSAlert_setInformativeText(
		alert,
		appkit.NSString_fromString(fmt.tprintf("Available: %.6f %s", balance, symbol)),
	)

	// Amount input field
	amount_field := appkit.NSTextField_newWithFrame(
		appkit.NSRect{x = 0, y = 0, width = 200, height = 24},
	)
	appkit.NSTextField_setPlaceholderString(
		amount_field,
		appkit.NSString_fromString("0.0"),
	)

	appkit.NSAlert_setAccessoryView(alert, cast(^appkit.NSView)amount_field)

	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Continue"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Max"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Cancel"))

	response := appkit.NSAlert_runModal(alert)

	switch response {
	case 1002: // Cancel
		log.debug("[swap_view] Amount input cancelled")
		return 0, false

	case 1001: // Max
		log.debugf("[swap_view] User selected MAX: %.6f", balance)
		return balance, true

	case 1000: // Continue
		// Parse amount input
		amount_str := appkit.NSString_toString(
			appkit.NSTextField_stringValue(amount_field),
		)
		parsed_amount, parse_ok := strconv.parse_f64(amount_str)
		if !parse_ok {
			show_error_alert("Invalid amount. Please enter a valid number.")
			log.warnf("[swap_view] Invalid amount input: %s", amount_str)
			return 0, false
		}

		// Validate amount
		if parsed_amount <= 0 {
			show_error_alert("Amount must be greater than 0")
			log.warn("[swap_view] Amount must be positive")
			return 0, false
		}

		if parsed_amount > balance {
			show_error_alert(fmt.tprintf("Insufficient balance. Available: %.6f %s", balance, symbol))
			log.warnf("[swap_view] Insufficient balance: %.6f > %.6f", parsed_amount, balance)
			return 0, false
		}

		log.debugf("[swap_view] Amount validated: %.6f", parsed_amount)
		return parsed_amount, true
	}

	return 0, false
}

// ============================================================================
// Destination Token Dialog
// ============================================================================

// show_destination_token_dialog shows destination token picker
//
// Returns: Destination token mint, symbol, and success status
show_destination_token_dialog :: proc() -> (mint: string, symbol: string, ok: bool) {
	log.debug("[swap_view] Showing destination token dialog")

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(
		alert,
		appkit.NSString_fromString("Select Destination Token"),
	)

	info_text := "Enter destination token symbol.\n\n" +
		"Common tokens:\n" +
		"• SOL\n" +
		"• USDC\n" +
		"• USDT\n" +
		"• BONK\n" +
		"• JUP\n" +
		"• RAY\n\n" +
		"You can enter any SPL token symbol."

	appkit.NSAlert_setInformativeText(
		alert,
		appkit.NSString_fromString(info_text),
	)

	// Token symbol field
	token_field := appkit.NSTextField_newWithFrame(
		appkit.NSRect{x = 0, y = 0, width = 300, height = 24},
	)
	appkit.NSTextField_setPlaceholderString(
		token_field,
		appkit.NSString_fromString("Enter destination token symbol"),
	)

	appkit.NSAlert_setAccessoryView(alert, cast(^appkit.NSView)token_field)

	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Continue"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Cancel"))

	response := appkit.NSAlert_runModal(alert)
	if response != 1000 {
		log.debug("[swap_view] Destination token selection cancelled")
		return "", "", false
	}

	// Get symbol
	symbol_ns := appkit.NSTextField_stringValue(token_field)
	symbol_input := strings.to_upper(appkit.NSString_toString(symbol_ns))
	defer delete(symbol_input)

	if len(symbol_input) == 0 {
		show_error_alert("Please enter a token symbol")
		log.warn("[swap_view] Empty destination token symbol")
		return "", "", false
	}

	log.debugf("[swap_view] User selected destination: %s", symbol_input)

	// Note: Mint lookup should be done by handler/service layer
	// This view only returns the symbol - handler will resolve to mint
	return "", symbol_input, true
}

// ============================================================================
// Quote Preview Dialog
// ============================================================================

// show_quote_preview_dialog shows quote confirmation
//
// Returns: True if user confirms, false if cancelled
show_quote_preview_dialog :: proc(
	quote: swap.JupiterQuote,
	source_symbol: string,
	dest_symbol: string,
) -> bool {
	log.debugf("[swap_view] Showing quote preview: %s -> %s", source_symbol, dest_symbol)

	// Parse amounts from quote
	in_amount_parsed, _ := strconv.parse_u64(quote.in_amount)
	out_amount_parsed, _ := strconv.parse_u64(quote.out_amount)

	// Convert to display units (assuming 9 decimals for simplicity)
	// Note: Real implementation should use actual token decimals
	decimals := u64(1_000_000_000) // 10^9 for typical SOL/SPL tokens
	in_amount_display := f64(in_amount_parsed) / f64(decimals)
	out_amount_display := f64(out_amount_parsed) / f64(decimals)

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

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(alert, appkit.NSString_fromString("Swap Quote"))
	appkit.NSAlert_setInformativeText(alert, appkit.NSString_fromString(message))

	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Build Transaction"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Cancel"))

	response := appkit.NSAlert_runModal(alert)
	confirmed := response == 1000

	log.debugf("[swap_view] Quote preview result: %v", confirmed)
	return confirmed
}

// ============================================================================
// Transaction Export Dialog
// ============================================================================

// show_transaction_export_dialog shows transaction export options
//
// ASSERTION 1: Transaction must not be empty
//
// Side effects: May copy to clipboard or generate deeplink
show_transaction_export_dialog :: proc(
	transaction_base64: string,
	source_symbol: string,
	dest_symbol: string,
) {
	assert(len(transaction_base64) > 0, "Transaction cannot be empty")

	log.debugf("[swap_view] Showing transaction export: %s -> %s", source_symbol, dest_symbol)

	message := fmt.tprintf(
		"Transaction ready for signing\n\n" +
		"Swap: %s → %s\n\n" +
		"Choose export option:\n" +
		"• Clipboard: Copy base64 transaction\n" +
		"• Phantom: Copy Phantom deeplink\n\n" +
		"Transaction expires in ~60 seconds",
		source_symbol,
		dest_symbol,
	)

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(alert, appkit.NSString_fromString("Export Transaction"))
	appkit.NSAlert_setInformativeText(alert, appkit.NSString_fromString(message))

	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Copy to Clipboard"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Copy Phantom Link"))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("Close"))

	response := appkit.NSAlert_runModal(alert)

	switch response {
	case 1000: // Copy to Clipboard
		log.debug("[swap_view] Copying transaction to clipboard")
		success := copy_to_clipboard(transaction_base64)
		if success {
			show_info_alert("Success", fmt.tprintf("Transaction copied to clipboard\n\n%d characters", len(transaction_base64)))
		} else {
			show_error_alert("Failed to copy transaction to clipboard")
		}

	case 1001: // Copy Phantom Link
		log.debug("[swap_view] Generating Phantom deeplink")
		// Build Phantom deeplink
		deeplink := fmt.tprintf("https://phantom.app/ul/v1/signAndSendTransaction?transaction=%s", transaction_base64)

		success := copy_to_clipboard(deeplink)
		if success {
			show_info_alert("Phantom Link Copied", "Paste the link in your browser to open Phantom wallet")
		} else {
			show_error_alert("Failed to copy Phantom deeplink")
		}

	case 1002: // Close
		log.debug("[swap_view] Transaction export closed")
	}
}

// ============================================================================
// Helper Dialogs
// ============================================================================

// show_error_alert shows error message
show_error_alert :: proc(message: string) {
	log.warnf("[swap_view] Error alert: %s", message)

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(alert, appkit.NSString_fromString("Error"))
	appkit.NSAlert_setInformativeText(alert, appkit.NSString_fromString(message))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("OK"))
	appkit.NSAlert_runModal(alert)
}

// show_info_alert shows informational message
show_info_alert :: proc(title: string, message: string) {
	log.debugf("[swap_view] Info alert: %s - %s", title, message)

	alert := appkit.NSAlert_new()
	appkit.NSAlert_setMessageText(alert, appkit.NSString_fromString(title))
	appkit.NSAlert_setInformativeText(alert, appkit.NSString_fromString(message))
	appkit.NSAlert_addButtonWithTitle(alert, appkit.NSString_fromString("OK"))
	appkit.NSAlert_runModal(alert)
}

// ============================================================================
// Clipboard Helper
// ============================================================================

// copy_to_clipboard copies text to system clipboard
//
// Returns: True if successful, false otherwise
copy_to_clipboard :: proc(text: string) -> bool {
	pasteboard := appkit.NSPasteboard_generalPasteboard()
	if pasteboard == nil {
		log.error("[swap_view] Failed to get general pasteboard")
		return false
	}

	// Clear existing contents
	appkit.NSPasteboard_clearContents(pasteboard)

	// Copy text
	ns_text := appkit.NSString_fromString(text)
	success := appkit.NSPasteboard_setString(
		pasteboard,
		ns_text,
		appkit.NSPasteboardTypeString,
	)

	if success {
		log.debug("[swap_view] Text copied to clipboard successfully")
	} else {
		log.error("[swap_view] Failed to copy text to clipboard")
	}

	return success
}
