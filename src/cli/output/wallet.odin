// Wallet output formatting
// Table display for portfolio balances
package output

import "core:fmt"
import "core:log"
import "core:strings"
import "core:time"
import models "../../lib/models"
import wallet "../../lib/wallet"

// ============================================================================
// Table Formatting
// ============================================================================

// Column widths for dynamic sizing
ColumnWidths :: struct {
	symbol:  int,
	balance: int,
	price:   int,
	value:   int,
	change:  int,
}

// format_wallet_table displays portfolio in formatted table
//
// Layout:
// - Header with wallet info
// - Column headers
// - Separator line
// - Token rows (one per token)
// - Separator line
// - Total row
// - Timestamp
format_wallet_table :: proc(
	wallet_info: models.Wallet,
	portfolio: wallet.PortfolioBalance,
) {
	// Header
	fmt.printfln("Wallet: %s (%s)", wallet_info.label, wallet_info.address)
	fmt.println("")

	// Build combined balances (SOL + tokens)
	all_balances := make([dynamic]wallet.TokenBalance)
	defer delete(all_balances)

	// Add SOL if non-zero
	if portfolio.sol_balance.amount > 0 {
		append(&all_balances, portfolio.sol_balance)
	}

	// Add all token balances
	for token_balance in portfolio.token_balances {
		if token_balance.amount > 0 {  // Skip zero balances
			append(&all_balances, token_balance)
		}
	}

	if len(all_balances) == 0 {
		fmt.println("No assets in wallet")
		return
	}

	// Calculate column widths
	widths := calculate_column_widths(all_balances[:])

	// Print table header
	format_table_header(widths)

	// Print separator line
	format_separator_line(widths)

	// Print each token row
	for balance in all_balances {
		format_table_row(balance, widths)
	}

	// Print footer separator
	format_separator_line(widths)

	// Print total row
	format_total_row(portfolio.total_usd, widths)

	// Timestamp
	fmt.println("")
	current_time := time.now()
	// Format timestamp as UTC string
	year, month, day := time.date(current_time)
	hour, min, sec := time.clock(current_time)
	fmt.printfln("Last updated: %04d-%02d-%02d %02d:%02d:%02d UTC",
		year, int(month), day, hour, min, sec)
}

// calculate_column_widths determines minimum widths for each column
//
// Ensures content fits without truncation
calculate_column_widths :: proc(balances: []wallet.TokenBalance) -> ColumnWidths {
	widths := ColumnWidths{
		symbol  = len("Token"),
		balance = len("Balance"),
		price   = len("Price"),
		value   = len("Value (USD)"),
		change  = len("24h Change"),
	}

	for balance in balances {
		// Symbol width
		symbol_width := len(balance.symbol)
		if symbol_width > widths.symbol {
			widths.symbol = symbol_width
		}

		// Balance width (formatted)
		balance_str := format_balance(balance.amount, balance.decimals)
		balance_width := len(balance_str)
		delete(balance_str)
		if balance_width > widths.balance {
			widths.balance = balance_width
		}

		// Price width
		price_str := format_price_value(balance.usd_price)
		price_width := len(price_str)
		delete(price_str)
		if price_width > widths.price {
			widths.price = price_width
		}

		// Value width (using tprintf for temp string - no need to delete)
		value_str := fmt.tprintf("$%.2f", balance.usd_value)
		value_width := len(value_str)
		if value_width > widths.value {
			widths.value = value_width
		}

		// Change width (using tprintf for temp string - no need to delete)
		change_str := fmt.tprintf("+0.00%%")
		change_width := len(change_str)
		if change_width > widths.change {
			widths.change = change_width
		}
	}

	return widths
}

// format_table_header prints column headers
format_table_header :: proc(widths: ColumnWidths) {
	fmt.printfln("%-*s  %*s  %*s  %*s  %*s",
		widths.symbol, "Token",
		widths.balance, "Balance",
		widths.price, "Price",
		widths.value, "Value (USD)",
		widths.change, "24h Change")
}

// format_separator_line prints horizontal separator
format_separator_line :: proc(widths: ColumnWidths) {
	// Calculate total width: columns + spacing (2 spaces between each column)
	total_width := widths.symbol + widths.balance + widths.price +
	               widths.value + widths.change + 8  // 8 = 4 gaps × 2 spaces

	for i in 0..<total_width {
		fmt.print("─")
	}
	fmt.println("")
}

// format_table_row prints a single token row
format_table_row :: proc(balance: wallet.TokenBalance, widths: ColumnWidths) {
	// Format values
	balance_str := format_balance(balance.amount, balance.decimals)
	price_str := format_price_value(balance.usd_price)

	// Print row with proper alignment (using tprintf for temp values)
	value_str := fmt.tprintf("$%.2f", balance.usd_value)

	// Note: 24h change not available in TokenBalance struct
	// For Phase 1, we show "+0.00%" as placeholder
	// Phase 2 will fetch actual price changes
	change_str := "+0.00%"

	// Print row with proper alignment
	fmt.printfln("%-*s  %*s  %*s  %*s  %*s",
		widths.symbol, balance.symbol,
		widths.balance, balance_str,
		widths.price, price_str,
		widths.value, value_str,
		widths.change, change_str)

	// Clean up allocated strings
	delete(balance_str)
	delete(price_str)
}

// format_total_row prints the total portfolio value
format_total_row :: proc(total_usd: f64, widths: ColumnWidths) {
	total_str := fmt.tprintf("$%.2f", total_usd)

	// Calculate padding to align total with value column
	padding_width := widths.symbol + widths.balance + widths.price + 6  // 6 = 3 gaps × 2

	fmt.printfln("%-*s  %*s",
		padding_width, "Total:",
		widths.value, total_str)
}

// ============================================================================
// Formatting Helpers
// ============================================================================

// format_balance formats balance with smart precision
//
// Rules:
// - Large (≥1000): 2 decimals (1,234.56)
// - Medium (1-1000): 4 decimals (123.4567)
// - Small (0.01-1): 6 decimals (0.123456)
// - Micro (<0.01): 8 decimals (0.00012345)
format_balance :: proc(amount: f64, decimals: int) -> string {
	if amount >= 1000 {
		return fmt.aprintf("%.2f", amount)
	} else if amount >= 1 {
		return fmt.aprintf("%.4f", amount)
	} else if amount >= 0.01 {
		return fmt.aprintf("%.6f", amount)
	} else if amount > 0 {
		return fmt.aprintf("%.8f", amount)
	} else {
		return fmt.aprintf("0.00")
	}
}

// format_price_value formats USD price with appropriate precision
//
// Rules:
// - Standard (≥$1): 2 decimals ($145.32)
// - Small ($0.01-$1): 4 decimals ($0.4567)
// - Micro (<$0.01): 6 decimals ($0.000145)
format_price_value :: proc(price: f64) -> string {
	if price >= 1 {
		return fmt.aprintf("$%.2f", price)
	} else if price >= 0.01 {
		return fmt.aprintf("$%.4f", price)
	} else {
		return fmt.aprintf("$%.6f", price)
	}
}

// format_change formats 24h price change with sign
//
// Note: Placeholder for Phase 1
// Phase 2 will integrate actual price change data
format_change :: proc(price: f64) -> string {
	// Placeholder - always returns neutral
	return fmt.aprintf("+0.00%%")
}
