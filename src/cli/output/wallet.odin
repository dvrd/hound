// Wallet output formatting
// Table display for portfolio balances
package output

import "core:fmt"
import "core:log"
import "core:strings"
import "core:time"
import "core:encoding/json"
import "core:os"
import models "../../lib/models"
import wallet "../../lib/wallet"

// ============================================================================
// JSON Output Formatting (Phase 3)
// ============================================================================

// JSON output structures
JsonAsset :: struct {
	symbol:     string,
	balance:    f64,
	price_usd:  f64,
	value_usd:  f64,
	change_24h: f64,
}

JsonWalletOutput :: struct {
	wallet_address:  string,
	label:           string,
	assets:          []JsonAsset,
	total_value_usd: f64,
	timestamp:       i64,
}

// format_wallet_json outputs portfolio in JSON format
//
// Matches spec from PRD:
// {
//   "wallet_address": "...",
//   "label": "...",
//   "assets": [...],
//   "total_value_usd": 0.0,
//   "timestamp": 0
// }
format_wallet_json :: proc(
	wallet_info: models.Wallet,
	portfolio: wallet.PortfolioBalance,
	balances: []wallet.TokenBalance,
) {
	assert(len(wallet_info.address) > 0, "Wallet address cannot be empty")
	assert(balances != nil, "Balances slice cannot be nil")

	log.debug("Formatting wallet output as JSON")

	// Build assets array
	assets := make([dynamic]JsonAsset)
	defer delete(assets)

	for balance in balances {
		asset := JsonAsset{
			symbol     = balance.symbol,
			balance    = balance.amount,
			price_usd  = balance.usd_price,
			value_usd  = balance.usd_value,
			change_24h = balance.change_24h,
		}
		append(&assets, asset)
	}

	// Build output structure
	output := JsonWalletOutput{
		wallet_address  = wallet_info.address,
		label           = wallet_info.label,
		assets          = assets[:],
		total_value_usd = portfolio.total_usd,
		timestamp       = time.to_unix_seconds(time.now()),
	}

	// Marshal to stdout using json.marshal_to_writer
	marshal_opt := json.Marshal_Options{
		pretty     = true,
		spec       = .JSON,
		use_spaces = true,
		spaces     = 2,
	}

	writer := os.stream_from_handle(os.stdout)
	if err := json.marshal_to_writer(writer, output, &marshal_opt); err != nil {
		log.errorf("Failed to marshal JSON: %v", err)
		fmt.eprintln("Error: Failed to output JSON")
	}

	// Newline after JSON
	fmt.println("")

	log.debug("JSON output complete")
}

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
// Phase 3 enhancement: Now accepts balances array (pre-sorted, pre-filtered)
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
	balances: []wallet.TokenBalance,  // Phase 3: Pre-sorted and filtered
) {
	assert(len(wallet_info.address) > 0, "Wallet address cannot be empty")
	assert(balances != nil, "Balances slice cannot be nil")

	// Header
	fmt.printfln("Wallet: %s (%s)", wallet_info.label, wallet_info.address)
	fmt.printfln("Type: %v", wallet_info.wallet_type)
	if len(wallet_info.derivation_path) > 0 {
		fmt.printfln("Derivation Path: %s", wallet_info.derivation_path)
		fmt.printfln("Account Index: %d", wallet_info.account_index)
	}
	fmt.println("")

	if len(balances) == 0 {
		fmt.println("No assets in wallet")
		return
	}

	// Calculate column widths
	widths := calculate_column_widths(balances)

	// Print table header
	format_table_header(widths)

	// Print separator line
	format_separator_line(widths)

	// Print each token row (balances already sorted and filtered)
	for balance in balances {
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
		change_str := fmt.tprintf("%+.2f%%", balance.change_24h)
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

	// Format 24h change with sign
	change_str := fmt.tprintf("%+.2f%%", balance.change_24h)

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

