// History command implementation
// Display swap transaction history with formatted output
package commands

import "core:fmt"
import "core:log"
import "core:os"
import "core:strconv"
import "core:time"
import models "../../lib/models"
import db "../../lib/database"
import token_cfg "../../lib/config"
import output "../output"
import memory "../../lib/memory"

// ============================================================================
// History Command
// ============================================================================

// handle_history implements "hound history [--limit N] [--wallet ADDRESS]" workflow
//
// Workflow:
// 1. Parse optional flags (--limit, --wallet)
// 2. Open database
// 3. Query swap history with filters
// 4. Format and display results in table format
//
// Returns: ErrorType for consistent error handling
handle_history :: proc(args: []string) -> models.ErrorType {
	context.allocator = memory.command_allocator()
	defer memory.reset_command_arena()
	defer free_all(context.temp_allocator)

	log.debug("History command invoked")

	// Parse flags
	limit := 10 // Default limit
	wallet_address := ""

	// Parse command line args
	for i := 0; i < len(args); i += 1 {
		arg := args[i]

		if arg == "--limit" {
			if i + 1 >= len(args) {
				log.error("Missing value for --limit flag")
				fmt.eprintln("")
				fmt.eprintln("Usage: hound history [--limit N] [--wallet ADDRESS]")
				return .MissingArgument
			}

			// Parse limit value
			limit_val, ok := strconv.parse_int(args[i + 1], 10)
			if !ok || limit_val <= 0 {
				log.error("Invalid limit value (must be positive integer)")
				return .MissingArgument
			}
			limit = int(limit_val)
			i += 1 // Skip next arg since we consumed it
		} else if arg == "--wallet" {
			if i + 1 >= len(args) {
				log.error("Missing value for --wallet flag")
				fmt.eprintln("")
				fmt.eprintln("Usage: hound history [--limit N] [--wallet ADDRESS]")
				return .MissingArgument
			}

			wallet_address = args[i + 1]
			i += 1 // Skip next arg since we consumed it
		}
	}

	log.debugf("Parsed flags: limit=%d, wallet=%s", limit, wallet_address)

	// Open database
	db_path := token_cfg.get_database_path()
	if !os.exists(db_path) {
		log.error("Database not found")
		fmt.eprintln("")
		fmt.eprintln("No swap history available yet.")
		fmt.eprintln("Swaps will be recorded after you perform your first transaction.")
		return .DatabaseError
	}

	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Query swap history
	entries, query_err := db.get_swap_history(database, wallet_address, limit)
	if query_err != .None {
		log.errorf("Failed to query swap history: %v", query_err)
		return .DatabaseError
	}

	// Display results
	if len(entries) == 0 {
		fmt.eprintln("No swap history found.")
		fmt.eprintln("")
		if len(wallet_address) > 0 {
			fmt.eprintfln("Wallet: %s", wallet_address)
		}
		return .None
	}

	// Print table header
	fmt.eprintln("")
	fmt.eprintfln("Swap History (%d transactions)", len(entries))
	if len(wallet_address) > 0 {
		fmt.eprintfln("Wallet: %s", wallet_address)
	}
	fmt.eprintln("")

	// Table header
	fmt.eprintln("┌────────────────────┬─────────────────────────┬───────────┬──────────────┬──────────┐")
	fmt.eprintln("│ Date               │ Trade                   │ Rate      │ Status       │ DEX      │")
	fmt.eprintln("├────────────────────┼─────────────────────────┼───────────┼──────────────┼──────────┤")

	// Table rows
	for entry in entries {
		// Format timestamp - Odin's time module uses simple string formatting
		// Calculate days/hours/mins from Unix timestamp for display
		days := entry.timestamp / 86400
		hours := (entry.timestamp % 86400) / 3600
		mins := (entry.timestamp % 3600) / 60

		// For now, show relative time (this will be properly formatted when time module is better understood)
		date_str := fmt.tprintf("%.2dd %.2dh:%.2dm", days, hours, mins)

		// Format trade (e.g., "SOL → USDC")
		trade_str := fmt.tprintf("%s → %s", entry.input_symbol, entry.output_symbol)

		// Calculate rate (output / input)
		rate := entry.output_amount / entry.input_amount
		rate_str := fmt.tprintf("%.4f", rate)

		// Format status
		status_str := entry.status
		if len(entry.error_message) > 0 {
			status_str = "Failed"
		}

		// Format DEX name (truncate if too long)
		dex_str := entry.dex
		if len(dex_str) > 8 {
			dex_str = dex_str[:8]
		}

		// Print row
		fmt.eprintfln("│ %-18s │ %-23s │ %-9s │ %-12s │ %-8s │",
			date_str, trade_str, rate_str, status_str, dex_str)
	}

	// Table footer
	fmt.eprintln("└────────────────────┴─────────────────────────┴───────────┴──────────────┴──────────┘")
	fmt.eprintln("")

	// Show detailed info section
	fmt.eprintln("Recent Transactions:")
	fmt.eprintln("")

	for i := 0; i < min(3, len(entries)); i += 1 {
		entry := entries[i]

		fmt.eprintfln("%d. %s → %s",
			i + 1,
			entry.input_symbol,
			entry.output_symbol)

		// Format timestamp
		fmt.eprintfln("   Timestamp: %d (Unix)", entry.timestamp)

		fmt.eprintfln("   Amount: %.6f %s → %.6f %s",
			entry.input_amount,
			entry.input_symbol,
			entry.output_amount,
			entry.output_symbol)

		fmt.eprintfln("   Price Impact: %.2f%%", entry.price_impact)
		fmt.eprintfln("   Slippage: %.2f%% (%d bps)", f64(entry.slippage_bps) / 100.0, entry.slippage_bps)

		// Format fees
		total_fee_sol := f64(entry.network_fee + entry.priority_fee) / 1_000_000_000.0
		fmt.eprintfln("   Total Fees: %.6f SOL", total_fee_sol)

		fmt.eprintfln("   DEX: %s", entry.dex)
		fmt.eprintfln("   Status: %s", entry.status)

		if len(entry.error_message) > 0 {
			fmt.eprintfln("   Error: %s", entry.error_message)
		}

		fmt.eprintfln("   Signature: %s", entry.signature)
		fmt.eprintln("")
	}

	// Get total count
	total_count, count_err := db.get_swap_count(database, wallet_address)
	if count_err == .None && total_count > i64(limit) {
		fmt.eprintfln("Showing %d of %d total transactions", limit, total_count)
		fmt.eprintfln("Use --limit flag to show more: hound history --limit %d", total_count)
		fmt.eprintln("")
	}

	return .None
}
