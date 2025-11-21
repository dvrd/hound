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

// handle_wallet displays all assets in the primary wallet
//
// Shows:
// - Token symbols and balances
// - Current USD prices
// - Total USD value per token
// - 24-hour price changes
// - Portfolio total
//
// Returns: ErrorType for error handling in main
handle_wallet :: proc() -> models.ErrorType {
	log.debug("Handling wallet command")

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

	// Get all wallets (need at least one primary wallet)
	wallets, wallets_err := db.get_all_wallets(database)
	if wallets_err != .None {
		log.errorf("Failed to get wallets: %v", wallets_err)
		return .DatabaseError
	}

	if len(wallets) == 0 {
		output.print_error("No wallet configured")
		fmt.eprintln("")
		fmt.eprintln("Please configure a wallet in the database first.")
		fmt.eprintln("Database location: ~/.config/hound/hound.db")
		return .MissingArgument
	}

	// Find primary wallet
	primary_wallet: models.Wallet
	found_primary := false
	for wallet_item in wallets {
		if wallet_item.is_primary {
			primary_wallet = wallet_item
			found_primary = true
			break
		}
	}

	// If no primary wallet, use first wallet
	if !found_primary {
		primary_wallet = wallets[0]
		log.warn("No primary wallet found, using first wallet")
	}

	log.infof("Using wallet: %s (%s)", primary_wallet.label, primary_wallet.address)

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
		primary_wallet.address,
		&config,
		database,
	)

	if portfolio_err != .None {
		log.errorf("Failed to fetch portfolio: %v", portfolio_err)

		// Try to show cached balances from database
		output.print_warning("Failed to fetch fresh balances, trying cached data...")
		cached_portfolio, cached_err := get_cached_portfolio(database, primary_wallet.address)

		if cached_err == .None && len(cached_portfolio.token_balances) > 0 {
			output.format_wallet_table(primary_wallet, cached_portfolio)
			fmt.eprintln("")
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
			fmt.printfln("Wallet: %s (%s)", primary_wallet.label, primary_wallet.address)
			fmt.println("No assets found.")
			fmt.println("")
			fmt.println("This wallet has no token balances.")
		} else {
			// Display portfolio table
			output.format_wallet_table(primary_wallet, portfolio)
		}
	}

	// Cleanup memory
	memory.reset_command_arena()
	memory.log_memory_stats()

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
