// Wallet manager - coordinates wallet operations
// Manages wallet list, balance fetching, and portfolio aggregation
package wallet_manager

import "core:log"
import models "../lib/models"
import db "../lib/database"
import wallet_backend "../lib/wallet"
import blockchain "../lib/blockchain"
import services "../lib/services"

// ============================================================================
// Types
// ============================================================================

// WalletManager coordinates all wallet operations
// Acts as orchestrator - delegates business logic to wallet_service
WalletManager :: struct {
	config:          ^models.TokenConfig,
	rpc_client:      wallet_backend.RPCClient,
	balance_fetcher: wallet_backend.BalanceFetcher,
	portfolios:      map[string]wallet_backend.PortfolioBalance,  // address -> portfolio (cache)
	db:              ^db.Database,                 // Database handle
	service_ctx:     services.WalletServiceContext,  // Service context for delegation
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
	rpc_client := wallet_backend.init_rpc_client(rpc_endpoint, backup_endpoints)
	log.debugf("RPC client initialized with endpoint: %s", rpc_endpoint)

	// Create portfolio map
	portfolios := make(map[string]wallet_backend.PortfolioBalance)

	// Create manager first
	manager := WalletManager{
		config          = config,
		rpc_client      = rpc_client,
		balance_fetcher = {},  // Initialize empty, set below
		portfolios      = portfolios,
		db              = db,
	}

	// Now initialize balance fetcher with pointer to manager's rpc_client
	price_fetcher := wallet_backend.PriceFetcher{}  // Empty placeholder
	manager.balance_fetcher = wallet_backend.init_balance_fetcher(&manager.rpc_client, &price_fetcher)
	log.debug("Balance fetcher initialized")

	// Initialize service context for delegation
	manager.service_ctx = services.WalletServiceContext{
		db              = db,
		rpc_client      = &manager.rpc_client,
		balance_fetcher = &manager.balance_fetcher,
		config          = config,
	}
	log.debug("Wallet service context initialized")

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

	// Delegate to wallet service
	return services.validate_and_add_wallet(&manager.service_ctx, address, label, is_primary)
}

// get_all_wallets retrieves all configured wallet addresses
//
// ASSERTION 1: Manager must not be nil
//
// Returns: Array of wallets and error status
get_wallets :: proc(manager: ^WalletManager) -> (wallets: []models.Wallet, err: models.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Delegate to wallet service
	return services.get_all_wallets(&manager.service_ctx)
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
) -> (portfolio: wallet_backend.PortfolioBalance, err: models.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Delegate to wallet service (includes fetch + persist)
	fetched_portfolio, fetch_err := services.fetch_and_persist_portfolio(&manager.service_ctx, address)
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
) -> (portfolio: wallet_backend.PortfolioBalance, found: bool) {
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
get_aggregated_portfolio :: proc(manager: ^WalletManager) -> wallet_backend.PortfolioBalance {
	assert(manager != nil, "Wallet manager cannot be nil")

	// Delegate to wallet service - aggregate all cached portfolios
	return services.aggregate_portfolios(manager.portfolios)
}
