// Wallet handler - business logic for portfolio management
// Extracts business logic from app.odin, no UI code
package handlers

import "core:log"
import models "../../lib/models"
import wallet "../../lib/wallet"
import memory "../../lib/memory"
import db "../../lib/database"
import state "../state"

// ============================================================================
// Portfolio Fetching
// ============================================================================

// handle_fetch_portfolio fetches aggregated portfolio for all configured wallets
//
// This handler implements the business logic for portfolio management:
// 1. Gets all wallet addresses from database
// 2. Creates WalletManager with RPC client and balance fetcher
// 3. Refreshes portfolios for all wallets (best-effort)
// 4. Aggregates balances into single portfolio view
// 5. Resets command arena after operations
// 6. Returns aggregated portfolio or error (NO UI updates)
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Database must not be nil
//
// Parameters:
//   - st: MenuBar state containing database and RPC client
//
// Returns: Aggregated portfolio balance and error status
//
// Pattern: Handler returns data only - caller decides how to display
handle_fetch_portfolio :: proc(
	st: ^state.MenuBarState,
) -> (portfolio: wallet.PortfolioBalance, err: models.ErrorType) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(st.db != nil, "Database cannot be nil")

	log.info("Handling portfolio fetch for all wallets")

	// Step 1: Get all wallets from database
	wallets, get_err := db.get_all_wallets(st.db)
	if get_err != .None {
		log.errorf("Failed to get wallets from database: %v", get_err)
		return {}, get_err
	}

	if len(wallets) == 0 {
		log.warn("No wallets configured in database")
		// Return empty portfolio (not an error)
		return wallet.PortfolioBalance{}, .None
	}

	log.infof("Found %d wallet(s) to refresh", len(wallets))

	// Step 2: Initialize WalletManager
	// NOTE: WalletManager coordinates balance fetching and portfolio aggregation
	manager, init_err := wallet.init_wallet_manager(
		&st.token_config,
		st.db,
		"https://api.mainnet-beta.solana.com", // TODO: Make configurable
		[]string{}, // TODO: Add backup endpoints
	)
	if init_err != .None {
		log.errorf("Failed to initialize wallet manager: %v", init_err)
		return {}, init_err
	}
	defer wallet.cleanup_wallet_manager(&manager)

	log.debug("WalletManager initialized successfully")

	// Step 3: Refresh all portfolios (best-effort)
	// NOTE: refresh_all_portfolios uses best-effort strategy - continues on errors
	refresh_err := wallet.refresh_all_portfolios(&manager)
	if refresh_err != .None {
		log.warnf("Portfolio refresh completed with errors: %v", refresh_err)
		// Continue - partial results may still be available
	}

	log.debug("Portfolio refresh completed")

	// Step 4: Get aggregated portfolio from manager
	// NOTE: Aggregates all cached portfolios into single view
	aggregated := wallet.get_aggregated_portfolio(&manager)

	log.infof("Aggregated portfolio: $%.2f total (SOL: $%.2f)",
		aggregated.total_usd,
		aggregated.sol_balance.usd_value)

	// Step 5: Reset command arena to clean up allocations
	// CRITICAL: Command arena must be reset after batch operations
	memory.reset_command_arena()
	log.debug("Command arena reset after portfolio operations")

	// ASSERTION: Validate portfolio totals are non-negative
	assert(aggregated.total_usd >= 0, "Total USD must be non-negative")
	assert(aggregated.sol_balance.usd_value >= 0, "SOL value must be non-negative")

	log.info("Portfolio fetch successful")

	return aggregated, .None
}

// ============================================================================
// Single Wallet Operations
// ============================================================================

// handle_fetch_single_wallet fetches portfolio for a specific wallet address
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Address must not be empty
//
// Parameters:
//   - st: MenuBar state
//   - address: Wallet address to fetch
//
// Returns: Portfolio balance and error status
handle_fetch_single_wallet :: proc(
	st: ^state.MenuBarState,
	address: string,
) -> (portfolio: wallet.PortfolioBalance, err: models.ErrorType) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")

	log.infof("Handling portfolio fetch for wallet: %s", address)

	// Initialize WalletManager
	manager, init_err := wallet.init_wallet_manager(
		&st.token_config,
		st.db,
		"https://api.mainnet-beta.solana.com",
		[]string{},
	)
	if init_err != .None {
		log.errorf("Failed to initialize wallet manager: %v", init_err)
		return {}, init_err
	}
	defer wallet.cleanup_wallet_manager(&manager)

	// Refresh single wallet portfolio
	fetched_portfolio, refresh_err := wallet.refresh_portfolio(&manager, address)
	if refresh_err != .None {
		log.errorf("Failed to refresh wallet portfolio: %v", refresh_err)
		return {}, refresh_err
	}

	log.infof("Wallet portfolio fetched: $%.2f total", fetched_portfolio.total_usd)

	// Reset command arena
	memory.reset_command_arena()
	log.debug("Command arena reset")

	return fetched_portfolio, .None
}

// ============================================================================
// Validation Functions
// ============================================================================

// validate_wallet_address validates a Solana wallet address format
//
// ASSERTION 1: Address must not be empty
//
// Parameters:
//   - address: Wallet address to validate
//
// Returns: Error status
validate_wallet_address :: proc(address: string) -> models.ErrorType {
	assert(len(address) > 0, "Address cannot be empty")

	// Basic validation: Solana addresses are 32-44 characters (base58)
	if len(address) < 32 || len(address) > 44 {
		log.errorf("Invalid address length: %d (expected 32-44)", len(address))
		return .InvalidToken
	}

	// TODO: Add base58 validation if needed

	return .None
}
