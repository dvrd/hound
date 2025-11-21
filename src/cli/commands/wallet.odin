// Wallet command implementation
// Displays cryptocurrency holdings with current prices
package commands

import "core:fmt"
import "core:log"
import "core:strings"
import "core:slice"
import "core:encoding/json"
import "core:time"
import "core:os"
import models "../../lib/models"
import db "../../lib/database"
import wallet "../../lib/wallet"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import output "../output"

// ============================================================================
// Phase 3: Flag Parsing and Configuration
// ============================================================================

// WalletFlags contains all command-line flags for wallet command
WalletFlags :: struct {
	show_all:      bool,   // --all: Show zero-balance tokens
	json_output:   bool,   // --json: Output JSON format
	wallet_addr:   string, // --wallet: Specific wallet address/label
	sort_by:       string, // --sort: Sort field (value, symbol, balance)
	force_refresh: bool,   // --refresh: Force price refresh
}

// parse_wallet_flags extracts flags from os.args
//
// Parses: --all, --json, --wallet <addr>, --sort <field>, --refresh
//
// Returns: WalletFlags struct with parsed values
parse_wallet_flags :: proc(args: []string) -> (flags: WalletFlags, err: models.ErrorType) {
	log.debugf("Parsing wallet flags from %d args", len(args))

	// Initialize defaults
	flags = WalletFlags{
		show_all      = false,
		json_output   = false,
		wallet_addr   = "",
		sort_by       = "value",  // Default sort
		force_refresh = false,
	}

	// Skip program name and "wallet" command (os.args[0] and os.args[1])
	for i := 2; i < len(args); i += 1 {
		arg := args[i]

		if arg == "--all" {
			flags.show_all = true
			log.debug("Flag: --all (show zero balances)")
		}
		else if arg == "--json" {
			flags.json_output = true
			log.debug("Flag: --json (JSON output)")
		}
		else if arg == "--refresh" {
			flags.force_refresh = true
			log.debug("Flag: --refresh (force refresh)")
		}
		else if arg == "--wallet" {
			// Next arg is the wallet identifier
			if i + 1 >= len(args) {
				log.error("--wallet flag requires an address/label argument")
				output.print_error("--wallet flag requires an address or label")
				fmt.println("")
				fmt.println("Usage: hound wallet --wallet <address|label>")
				return {}, .MissingArgument
			}
			i += 1  // Consume next arg
			flags.wallet_addr = args[i]
			log.debugf("Flag: --wallet %s", flags.wallet_addr)
		}
		else if arg == "--sort" {
			// Next arg is the sort field
			if i + 1 >= len(args) {
				log.error("--sort flag requires a field argument")
				output.print_error("--sort flag requires a field (value, symbol, balance)")
				fmt.println("")
				fmt.println("Usage: hound wallet --sort <value|symbol|balance>")
				return {}, .MissingArgument
			}
			i += 1  // Consume next arg
			sort_field := strings.to_lower(args[i])

			// Validate sort field
			if sort_field != "value" && sort_field != "symbol" && sort_field != "balance" {
				log.errorf("Invalid sort field: %s", sort_field)
				output.print_error(fmt.tprintf("Invalid sort field '%s'", sort_field))
				fmt.println("Valid fields: value, symbol, balance")
				return {}, .InvalidToken
			}

			flags.sort_by = sort_field
			log.debugf("Flag: --sort %s", sort_field)
		}
		else if strings.has_prefix(arg, "--") {
			// Unknown flag
			log.warnf("Unknown flag: %s", arg)
			fmt.printfln("Warning: Unknown flag '%s' (ignored)", arg)
		}
		// Ignore other args (not flags)
	}

	log.infof("Parsed flags: all=%v json=%v wallet=%s sort=%s refresh=%v",
		flags.show_all, flags.json_output, flags.wallet_addr, flags.sort_by, flags.force_refresh)

	return flags, .None
}

// resolve_target_wallet determines which wallet to use
//
// Priority:
// 1. If wallet_identifier provided: lookup by full address, partial address, or label
// 2. Otherwise: use primary wallet
//
// Returns: Wallet and error status
resolve_target_wallet :: proc(
	database: ^db.Database,
	wallet_identifier: string,
) -> (wallet: models.Wallet, err: models.ErrorType) {
	assert(database != nil, "Database cannot be nil")

	// Case 1: No identifier - use primary wallet
	if len(wallet_identifier) == 0 {
		log.debug("Using primary wallet (no identifier provided)")
		return db.get_primary_wallet(database)
	}

	log.debugf("Resolving wallet identifier: %s", wallet_identifier)

	// Get all wallets for searching
	all_wallets, wallets_err := db.get_all_wallets(database)
	if wallets_err != .None {
		log.errorf("Failed to get wallets: %v", wallets_err)
		return {}, wallets_err
	}

	if len(all_wallets) == 0 {
		log.warn("No wallets configured")
		return {}, .ConfigNotFound
	}

	// Case 2: Try full address match (exact)
	for w in all_wallets {
		if w.address == wallet_identifier {
			log.infof("Matched by full address: %s", w.label)
			return w, .None
		}
	}

	// Case 3: Try partial address match (first 8 characters)
	if len(wallet_identifier) >= 8 {
		partial := wallet_identifier[:8]
		matches := make([dynamic]models.Wallet, memory.command_allocator())

		for w in all_wallets {
			if strings.has_prefix(w.address, partial) {
				append(&matches, w)
			}
		}

		if len(matches) == 1 {
			// Unique match
			log.infof("Matched by partial address: %s", matches[0].label)
			return matches[0], .None
		} else if len(matches) > 1 {
			// Ambiguous - multiple matches
			log.warnf("Partial address '%s' matches %d wallets (ambiguous)", partial, len(matches))
			output.print_error(fmt.tprintf("Partial address '%s' matches multiple wallets:", partial))
			for m in matches {
				fmt.printfln("  - %s (%s)", m.label, m.address[:12])
			}
			fmt.println("Please use a more specific address or full address.")
			return {}, .InvalidToken
		}
	}

	// Case 4: Try label match (case-insensitive)
	identifier_lower := strings.to_lower(wallet_identifier, context.temp_allocator)
	for w in all_wallets {
		label_lower := strings.to_lower(w.label, context.temp_allocator)
		if label_lower == identifier_lower {
			log.infof("Matched by label: %s", w.label)
			return w, .None
		}
	}

	// No match found
	log.warnf("No wallet found for identifier: %s", wallet_identifier)
	output.print_error(fmt.tprintf("No wallet found for '%s'", wallet_identifier))
	fmt.println("")
	fmt.println("Available wallets:")
	for w in all_wallets {
		primary_marker := w.is_primary ? " (primary)" : ""
		fmt.printfln("  - %s: %s%s", w.label, w.address[:16], primary_marker)
	}

	return {}, .ConfigNotFound
}

// sort_token_balances sorts balances by specified field
//
// Modifies: balances slice (sorted in-place)
sort_token_balances :: proc(balances: []wallet.TokenBalance, sort_field: string) {
	assert(balances != nil, "Balances slice cannot be nil")

	log.debugf("Sorting %d balances by %s", len(balances), sort_field)

	switch sort_field {
	case "value":
		// Sort by USD value (descending - highest first)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.usd_value > b.usd_value
		})

	case "symbol":
		// Sort by symbol (alphabetical)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.symbol < b.symbol
		})

	case "balance":
		// Sort by balance amount (descending - highest first)
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.amount > b.amount
		})

	case:
		log.warnf("Unknown sort field '%s', defaulting to value", sort_field)
		// Default: sort by value
		slice.sort_by(balances, proc(a, b: wallet.TokenBalance) -> bool {
			return a.usd_value > b.usd_value
		})
	}

	log.debug("Sorting complete")
}

// ============================================================================
// Wallet Command Handler
// ============================================================================

// handle_wallet implements "hound wallet [flags]" workflow
//
// Phase 3 enhancements:
// - Flag parsing (--all, --json, --wallet, --sort, --refresh)
// - JSON output support
// - Flexible sorting
// - Advanced wallet selection
//
// Returns: ErrorType for consistent error handling
handle_wallet :: proc(flags: WalletFlags) -> models.ErrorType {
	log.debugf("Wallet command: flags=%#v", flags)

	// Step 1: Load token configuration
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}

	// Step 2: Open database
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		output.print_error("Could not open database")
		return .DatabaseError
	}
	defer db.database_close(database)

	// Step 3: Resolve target wallet (primary or specified)
	target_wallet, wallet_err := resolve_target_wallet(database, flags.wallet_addr)
	if wallet_err != .None {
		return wallet_err
	}

	log.infof("Target wallet: %s (%s)", target_wallet.label, target_wallet.address)

	// Step 4: Initialize RPC client and balance fetcher
	rpc_endpoint := "https://api.mainnet-beta.solana.com"
	backup_endpoints: []string = nil
	rpc_client := wallet.init_rpc_client(rpc_endpoint, backup_endpoints)

	price_fetcher := wallet.PriceFetcher{}
	balance_fetcher := wallet.init_balance_fetcher(&rpc_client, &price_fetcher)

	// Step 5: Fetch portfolio
	if flags.force_refresh {
		output.print_progress("Refreshing wallet balances...")
	} else {
		output.print_progress("Fetching wallet balances...")
	}

	portfolio, portfolio_err := wallet.fetch_portfolio_balance(
		&balance_fetcher,
		target_wallet.address,
		&config,
		database,
	)

	if portfolio_err != .None {
		log.errorf("Failed to fetch portfolio: %v", portfolio_err)

		// Try to show cached balances from database
		output.print_warning("Failed to fetch fresh balances, trying cached data...")
		cached_portfolio, cached_err := get_cached_portfolio(database, target_wallet.address)

		if cached_err == .None && len(cached_portfolio.token_balances) > 0 {
			portfolio = cached_portfolio
			output.print_warning("Using cached balances (prices may be stale)")
		} else {
			output.print_error("Could not fetch wallet balances")
			return portfolio_err
		}
	}

	log.infof("Portfolio fetched: $%.2f total", portfolio.total_usd)

	// Step 6: Build combined balances list (SOL + tokens)
	all_balances := make([dynamic]wallet.TokenBalance, memory.command_allocator())

	// Add SOL balance (always show if non-zero, or if --all flag)
	if portfolio.sol_balance.amount > 0 || flags.show_all {
		append(&all_balances, portfolio.sol_balance)
	}

	// Add token balances
	for token_balance in portfolio.token_balances {
		// Filter zero balances unless --all flag
		if token_balance.amount > 0 || flags.show_all {
			append(&all_balances, token_balance)
		}
	}

	log.debugf("Total balances to display: %d (show_all=%v)", len(all_balances), flags.show_all)

	// Step 7: Sort balances by specified field
	sort_token_balances(all_balances[:], flags.sort_by)

	// Step 8: Display output in requested format
	if flags.json_output {
		// JSON output
		output.format_wallet_json(target_wallet, portfolio, all_balances[:])
	} else {
		// Table output (default)
		output.format_wallet_table(target_wallet, portfolio, all_balances[:])
	}

	// Step 9: Reset arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// ============================================================================
// Swap Subcommand (Phase 3 Foundation)
// ============================================================================

// handle_wallet_swap implements basic swap command structure
//
// Phase 3 Scope: Command routing + usage help only
// Phase 4 Will Add: Actual quote fetching + transaction building
//
// Parameters:
// - args: Command arguments (from_symbol, to_symbol, amount)
//
// Returns: ErrorType for error handling
handle_wallet_swap :: proc(args: []string) -> models.ErrorType {
	log.debugf("Wallet swap subcommand: %d args", len(args))

	// Phase 3: Basic validation + usage help
	if len(args) < 3 {
		output.print_error("Swap functionality requires additional arguments")
		fmt.println("")
		fmt.println("Usage: hound wallet swap <from_symbol> <to_symbol> <amount>")
		fmt.println("")
		fmt.println("Examples:")
		fmt.println("  hound wallet swap sol usdc 1.0     # Swap 1 SOL for USDC")
		fmt.println("  hound wallet swap usdc aura 100    # Swap 100 USDC for AURA")
		fmt.println("")
		fmt.println("Note: Swap execution will be available in a future release.")
		return .MissingArgument
	}

	// TODO Phase 4: Implement actual swap logic
	// - Parse from_symbol, to_symbol, amount
	// - Look up token mints from config
	// - Get quote from Jupiter
	// - Build transaction
	// - Display confirmation prompt
	// - Sign and submit transaction

	log.warn("Swap execution not yet implemented (Phase 4)")
	output.print_error("Swap execution not yet implemented")
	fmt.println("This feature is planned for Phase 4.")

	return .None
}

// ============================================================================
// Helper Functions
// ============================================================================

// get_cached_portfolio retrieves portfolio from database cache
//
// Falls back to database when network fetch fails
get_cached_portfolio :: proc(
	database: ^db.Database,
	wallet_address: string,
) -> (portfolio: wallet.PortfolioBalance, err: models.ErrorType) {
	// Get cached balances from database
	balances_map, balances_err := db.get_balances_for_wallet(database, wallet_address)
	if balances_err != .None {
		return {}, balances_err
	}

	// Build portfolio from cached data
	portfolio.wallet_address = wallet_address
	portfolio.total_usd = 0

	// Convert map to token balances slice
	token_balances := make([dynamic]wallet.TokenBalance, memory.command_allocator())

	for mint, data in balances_map {
		balance := wallet.TokenBalance{
			mint      = mint,
			symbol    = "",  // Will be filled if we can match
			amount    = data[0],
			decimals  = 9,  // Default, actual value may vary
			usd_price = data[1],
			usd_value = data[2],
		}

		append(&token_balances, balance)
		portfolio.total_usd += data[2]  // Add usd_value
	}

	portfolio.token_balances = token_balances[:]

	return portfolio, .None
}
