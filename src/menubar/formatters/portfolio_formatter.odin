// Portfolio formatting utilities - pure string formatting for portfolio display
// No business logic, no service calls, no UI calls
package formatters

import "core:fmt"
import "core:strings"
import wallet "../../lib/wallet"

// ============================================================================
// Portfolio Formatting
// ============================================================================

// format_portfolio_total formats the total portfolio value
//
// ASSERTION 1: Total must be non-negative
//
// Returns: Formatted total string (e.g., "Total: $1,234.56")
format_portfolio_total :: proc(total_usd: f64) -> string {
	assert(total_usd >= 0, "Total portfolio value must be non-negative")

	return fmt.tprintf("Total: %s", format_usd_value(total_usd))
}

// format_sol_balance formats the SOL balance line
//
// Returns: Formatted SOL balance (e.g., "SOL: 5.123 ($500.00)")
format_sol_balance :: proc(balance: wallet.TokenBalance) -> string {
	assert(balance.amount >= 0, "SOL balance must be non-negative")

	return fmt.tprintf(
		"SOL: %.6f (%s)",
		balance.amount,
		format_usd_value(balance.usd_value),
	)
}

// format_token_balance formats a single token balance line
//
// ASSERTION 1: Symbol must not be empty
// ASSERTION 2: Amount must be non-negative
//
// Returns: Formatted token balance (e.g., "USDC: 100.00 ($100.00)")
format_token_balance :: proc(balance: wallet.TokenBalance) -> string {
	assert(len(balance.symbol) > 0, "Token symbol cannot be empty")
	assert(balance.amount >= 0, "Token balance must be non-negative")

	return fmt.tprintf(
		"%s: %.6f (%s)",
		strings.to_upper(balance.symbol),
		balance.amount,
		format_usd_value(balance.usd_value),
	)
}

// format_balance_list formats an array of token balances
//
// Returns: Multi-line string with all token balances
format_balance_list :: proc(balances: []wallet.TokenBalance) -> string {
	if len(balances) == 0 {
		return "No tokens"
	}

	builder := strings.builder_make()
	defer strings.builder_destroy(&builder)

	for balance, i in balances {
		if i > 0 {
			strings.write_string(&builder, "\n")
		}
		strings.write_string(&builder, format_token_balance(balance))
	}

	return strings.to_string(builder)
}

// Note: format_usd_value is defined in price_formatter.odin (shared within formatters package)
