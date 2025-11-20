// Price formatting utilities - pure string formatting for price display
// No business logic, no service calls, no UI calls
package formatters

import "core:fmt"
import "core:strings"

// ============================================================================
// Price Formatting
// ============================================================================

// format_price_menu_title formats the menu bar title with symbol, price, and 24h change
//
// ASSERTION 1: Symbol must not be empty
// ASSERTION 2: Price must be non-negative
//
// Returns: Formatted title string (e.g., "SOL: $100.50 (+2.3%)")
format_price_menu_title :: proc(
	symbol: string,
	price_usd: f64,
	change_24h: f64,
) -> string {
	assert(len(symbol) > 0, "Symbol cannot be empty")
	assert(price_usd >= 0, "Price must be non-negative")

	// Format price with appropriate precision
	price_str := format_usd_value(price_usd)

	// Format 24h change with +/- indicator
	change_str := format_price_change(change_24h)

	return fmt.tprintf("%s: %s (%s)", strings.to_upper(symbol), price_str, change_str)
}

// format_price_change formats a percentage change with +/- indicator
//
// Returns: Formatted change string (e.g., "+2.3%", "-1.5%")
format_price_change :: proc(change_percent: f64) -> string {
	if change_percent > 0 {
		return fmt.tprintf("+%.2f%%", change_percent)
	} else if change_percent < 0 {
		return fmt.tprintf("%.2f%%", change_percent)  // Negative sign already present
	} else {
		return "0.00%"
	}
}

// format_usd_value formats a USD value with appropriate precision
//
// ASSERTION 1: Value must be non-negative
//
// Returns: Formatted USD string (e.g., "$100.50", "$0.000123")
format_usd_value :: proc(value: f64) -> string {
	assert(value >= 0, "USD value must be non-negative")

	if value >= 1.0 {
		// Standard currency format for values >= $1
		return fmt.tprintf("$%.2f", value)
	} else if value >= 0.01 {
		// 4 decimal places for cents
		return fmt.tprintf("$%.4f", value)
	} else {
		// Scientific notation or 6 decimals for very small values
		return fmt.tprintf("$%.6f", value)
	}
}
