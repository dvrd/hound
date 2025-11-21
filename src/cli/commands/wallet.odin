// Wallet command implementation
// Displays cryptocurrency holdings with current prices
package commands

import "core:fmt"
import "core:log"
import models "../../lib/models"
import db "../../lib/database"
import wallet "../../lib/wallet"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import output "../output"

// ============================================================================
// Wallet Command Handler
// ============================================================================

// handle_wallet displays all assets in a wallet
//
// Shows:
// - Token symbols and balances
// - Current USD prices
// - Total USD value per token
// - 24-hour price changes
// - Portfolio total
//
// Parameters:
// - address_flag: Optional wallet address (empty string for primary wallet)
//
// Returns: ErrorType for error handling in main
handle_wallet :: proc(address_flag: string = "") -> models.ErrorType {
	log.debugf("Handling wallet command: address_flag='%s'", address_flag)

	// Get database path
	db_path := token_cfg.get_database_path()
	log.debugf("Database path: %s", db_path)

	// Open database
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return .DatabaseError
	}
	defer db.database_close(database)

	// Determine target wallet
	target_wallet: models.Wallet
	wallet_err: models.ErrorType

	if len(address_flag) > 0 {
		// User specified address
		log.debugf("Using specified address: %s", address_flag)

		found: bool
		target_wallet, found, wallet_err = db.get_wallet_by_address(database, address_flag)
		if wallet_err != .None {
			log.errorf("Database query failed: %v", wallet_err)
			return .DatabaseError
		}
		if !found {
			log.warnf("Wallet not found: %s", address_flag)
			output.print_error(fmt.tprintf("Wallet address not found: %s", address_flag))
			fmt.println("")
			fmt.println("Run 'hound wallet list' to see configured wallets.")
			return .ConfigNotFound
		}
	} else {
		// Use primary wallet
		log.debug("Using primary wallet")

		target_wallet, wallet_err = db.get_primary_wallet(database)
		if wallet_err == .ConfigNotFound {
			log.warn("No primary wallet configured")
			output.print_error("No primary wallet configured")
			fmt.println("")
			fmt.println("Please configure a wallet in the database first.")
			fmt.println("Database location: ~/.config/hound/hound.db")
			return .ConfigNotFound
		} else if wallet_err != .None {
			log.errorf("Failed to get primary wallet: %v", wallet_err)
			return .DatabaseError
		}
	}

	log.infof("Using wallet: %s (%s)", target_wallet.label, target_wallet.address)

	// Initialize RPC client
	rpc_endpoint := "https://api.mainnet-beta.solana.com"
	backup_endpoints: []string = nil
	rpc_client := wallet.init_rpc_client(rpc_endpoint, backup_endpoints)

	// Initialize price fetcher (placeholder struct)
	price_fetcher := wallet.PriceFetcher{}

	// Initialize balance fetcher
	balance_fetcher := wallet.init_balance_fetcher(&rpc_client, &price_fetcher)

	// Load token config
	log.debug("Loading token configuration")
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}

	// Show progress
	output.print_progress("Fetching wallet balances...")

	// Fetch portfolio balance
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
			output.format_wallet_table(target_wallet, cached_portfolio)
			fmt.println("")
			output.print_warning("Displayed cached balances (prices may be stale)")
		} else {
			output.print_error("Could not fetch wallet balances")
			return portfolio_err
		}
	} else {
		// Check if wallet is empty
		if portfolio.total_usd == 0 && len(portfolio.token_balances) == 0 {
			output.print_info("Wallet is empty")
			fmt.println("")
			fmt.printfln("Wallet: %s (%s)", target_wallet.label, target_wallet.address)
			fmt.println("No assets found.")
			fmt.println("")
			fmt.println("This wallet has no token balances.")
		} else {
			// Display portfolio table
			output.format_wallet_table(target_wallet, portfolio)
		}
	}

	// Cleanup memory
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
