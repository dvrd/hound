// Wallet manager - coordinates wallet operations
// Manages wallet list, balance fetching, and portfolio aggregation
package wallet

import "core:log"
import models "../models"
import db "../database"

// ============================================================================
// Types
// ============================================================================

// WalletManager coordinates all wallet operations
WalletManager :: struct {
	config:          ^models.TokenConfig,
	rpc_client:      RPCClient,
	balance_fetcher: BalanceFetcher,
	portfolios:      map[string]PortfolioBalance,  // address -> portfolio (cache)
	db:              ^db.Database,                 // Database handle
}

// ============================================================================
// Initialization
// ============================================================================

// init_wallet_manager creates a new wallet manager
//
// ASSERTION 1: Config must not be nil
// ASSERTION 2: Database must not be nil
//
// Returns: Initialized wallet manager and error status
init_wallet_manager :: proc(
	config: ^models.TokenConfig,
	db: ^db.Database,
	rpc_endpoint: string,
	backup_endpoints: []string,
) -> (WalletManager, models.ErrorType) {
	assert(config != nil, "Token config cannot be nil")
	assert(db != nil, "Database cannot be nil")

	log.info("Initializing wallet manager")

	// Initialize RPC client
	rpc_client := init_rpc_client(rpc_endpoint, backup_endpoints)
	log.debugf("RPC client initialized with endpoint: %s", rpc_endpoint)

	// Create portfolio map
	portfolios := make(map[string]PortfolioBalance)

	// Create manager first
	manager := WalletManager{
		config          = config,
		rpc_client      = rpc_client,
		balance_fetcher = {},  // Initialize empty, set below
		portfolios      = portfolios,
		db              = db,
	}

	// Now initialize balance fetcher with pointer to manager's rpc_client
	price_fetcher := PriceFetcher{}  // Empty placeholder
	manager.balance_fetcher = init_balance_fetcher(&manager.rpc_client, &price_fetcher)
	log.debug("Balance fetcher initialized")

	log.info("Wallet manager initialized successfully")
	return manager, .None
}

// cleanup_wallet_manager frees resources
cleanup_wallet_manager :: proc(manager: ^WalletManager) {
	log.debug("Cleaning up wallet manager")

	// Clean up portfolios
	for address, portfolio in manager.portfolios {
		delete(portfolio.token_balances)
	}
	delete(manager.portfolios)

	log.debug("Wallet manager cleanup complete")
}

// ============================================================================
// Wallet Management
// ============================================================================

// add_wallet adds a new wallet address to watch
//
// ASSERTION 1: Manager must not be nil
// ASSERTION 2: Address must not be empty
// ASSERTION 3: Label must not be empty
//
// Returns: Error status
add_wallet :: proc(
	manager: ^WalletManager,
	address: string,
	label: string,
	is_primary: bool = false,
) -> models.ErrorType {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Validate and add wallet
	return validate_and_add_wallet(manager.db, address, label, is_primary)
}

// get_all_wallets retrieves all configured wallet addresses
//
// ASSERTION 1: Manager must not be nil
//
// Returns: Array of wallets and error status
get_wallets :: proc(manager: ^WalletManager) -> (wallets: []models.Wallet, err: models.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Get all wallets from database
	return get_all_wallets(manager.db)
}

// ============================================================================
// Portfolio Management
// ============================================================================

// refresh_portfolio fetches fresh balance data for a wallet
//
// ASSERTION 1: Manager must not be nil
// ASSERTION 2: Address must not be empty
//
// Returns: Portfolio balance and error status
refresh_portfolio :: proc(
	manager: ^WalletManager,
	address: string,
) -> (portfolio: PortfolioBalance, err: models.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Fetch and persist portfolio using local operations
	fetched_portfolio, fetch_err := fetch_and_persist_portfolio(
		&manager.balance_fetcher,
		manager.db,
		manager.config,
		address,
	)
	if fetch_err != .None {
		return {}, fetch_err
	}

	// Store in manager's cache for quick access
	manager.portfolios[address] = fetched_portfolio
	log.debugf("Portfolio cached in manager for address: %s", address)

	return fetched_portfolio, .None
}

// get_cached_portfolio retrieves portfolio from cache without refreshing
//
// ASSERTION 1: Manager must not be nil
// ASSERTION 2: Address must not be empty
//
// Returns: Portfolio balance, found flag, and error status
get_cached_portfolio :: proc(
	manager: ^WalletManager,
	address: string,
) -> (portfolio: PortfolioBalance, found: bool) {
	assert(manager != nil, "Wallet manager cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")

	portfolio, found = manager.portfolios[address]
	if found {
		log.debugf("Retrieved cached portfolio for address: %s", address)
	} else {
		log.debugf("No cached portfolio for address: %s", address)
	}

	return portfolio, found
}

// refresh_all_portfolios refreshes balances for all configured wallets
//
// ASSERTION 1: Manager must not be nil
//
// Returns: Error status
refresh_all_portfolios :: proc(manager: ^WalletManager) -> models.ErrorType {
	assert(manager != nil, "Wallet manager cannot be nil")

	log.info("Refreshing all portfolios")

	// Get all wallets
	wallets, get_err := get_wallets(manager)
	if get_err != .None {
		log.errorf("Failed to get wallets: %v", get_err)
		return get_err
	}
	// NO delete needed - command arena cleanup

	// Refresh each wallet
	success_count := 0
	for wallet in wallets {
		_, refresh_err := refresh_portfolio(manager, wallet.address)
		if refresh_err == .None {
			success_count += 1
		} else {
			log.warnf("Failed to refresh portfolio for %s: %v", wallet.label, refresh_err)
		}
	}

	log.infof("Portfolio refresh complete: %d/%d successful", success_count, len(wallets))

	if success_count == 0 && len(wallets) > 0 {
		// All refreshes failed
		return .RPCConnectionFailed
	}

	return .None
}

// get_aggregated_portfolio aggregates all wallets into a single portfolio view
//
// ASSERTION 1: Manager must not be nil
//
// Returns: Aggregated portfolio balance
get_aggregated_portfolio :: proc(manager: ^WalletManager) -> PortfolioBalance {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Aggregate all cached portfolios using local operation
	return aggregate_portfolios(manager.portfolios)
}
