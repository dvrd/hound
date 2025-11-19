// Wallet manager - coordinates wallet operations
// Manages wallet list, balance fetching, and portfolio aggregation
package wallet

import "core:fmt"
import "core:log"
import src "../"

// ============================================================================
// Types
// ============================================================================

// WalletManager coordinates all wallet operations
WalletManager :: struct {
	config:          ^src.TokenConfig,
	rpc_client:      RPCClient,
	balance_fetcher: BalanceFetcher,
	portfolios:      map[string]PortfolioBalance,  // address -> portfolio
	db:              ^src.Database,                 // Database handle
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
	config: ^src.TokenConfig,
	db: ^src.Database,
	rpc_endpoint: string,
	backup_endpoints: []string,
) -> (WalletManager, src.ErrorType) {
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
) -> src.ErrorType {
	assert(manager != nil, "Wallet manager cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")
	assert(len(label) > 0, "Wallet label cannot be empty")

	log.infof("Adding wallet: %s (%s)", label, address)

	// Validate address format
	if !validate_solana_address(address) {
		log.error("Invalid Solana address format")
		return .InvalidToken
	}

	// Add to config
	wallet := src.Wallet{
		address    = address,
		label      = label,
		is_primary = is_primary,
	}

	// Insert into database
	db_err := src.insert_wallet(manager.db, wallet)
	if db_err != .None {
		log.errorf("Failed to insert wallet into database: %v", db_err)
		return db_err
	}

	// Add to in-memory config (append to wallets slice)
	// Note: This modifies the config struct, which should be reloaded from DB
	// For now, we just log success - the next config reload will pick it up
	log.infof("Wallet added successfully: %s", label)

	return .None
}

// get_all_wallets retrieves all configured wallet addresses
//
// ASSERTION 1: Manager must not be nil
//
// Returns: Array of wallets and error status
get_wallets :: proc(manager: ^WalletManager) -> (wallets: []src.Wallet, err: src.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")

	log.debug("Fetching all wallets from database")

	wallet_list, db_err := src.get_all_wallets(manager.db)
	if db_err != .None {
		log.errorf("Failed to fetch wallets: %v", db_err)
		return nil, db_err
	}

	log.infof("Fetched %d wallet(s)", len(wallet_list))
	return wallet_list, .None
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
) -> (portfolio: PortfolioBalance, err: src.ErrorType) {
	assert(manager != nil, "Wallet manager cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")

	log.infof("Refreshing portfolio for address: %s", address)

	// Fetch portfolio balance
	fetched_portfolio, fetch_err := fetch_portfolio_balance(&manager.balance_fetcher, address, manager.config)
	if fetch_err != .None {
		log.errorf("Failed to fetch portfolio: %v", fetch_err)
		return {}, fetch_err
	}

	// Store in cache
	manager.portfolios[address] = fetched_portfolio
	log.debugf("Portfolio cached for address: %s", address)

	// Update database with balances
	log.debug("Updating database with new balances")

	// Update SOL balance
	sol_err := src.update_balance(
		manager.db,
		address,
		fetched_portfolio.sol_balance.mint,
		fetched_portfolio.sol_balance.symbol,
		fetched_portfolio.sol_balance.amount,
		fetched_portfolio.sol_balance.usd_price,
		fetched_portfolio.sol_balance.usd_value,
	)
	if sol_err != .None {
		log.warnf("Failed to update SOL balance in database: %v", sol_err)
	}

	// Update token balances
	for token_balance in fetched_portfolio.token_balances {
		token_err := src.update_balance(
			manager.db,
			address,
			token_balance.mint,
			token_balance.symbol,
			token_balance.amount,
			token_balance.usd_price,
			token_balance.usd_value,
		)
		if token_err != .None {
			log.warnf("Failed to update token balance in database: %v (token: %s)", token_err, token_balance.symbol)
		}
	}

	log.infof("Portfolio refresh complete: %d token(s), $%.2f total",
		len(fetched_portfolio.token_balances), fetched_portfolio.total_usd)

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
refresh_all_portfolios :: proc(manager: ^WalletManager) -> src.ErrorType {
	assert(manager != nil, "Wallet manager cannot be nil")

	log.info("Refreshing all portfolios")

	// Get all wallets
	wallets, get_err := get_wallets(manager)
	if get_err != .None {
		log.errorf("Failed to get wallets: %v", get_err)
		return get_err
	}
	defer delete(wallets)

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

	log.debug("Aggregating portfolios")

	// Create aggregated portfolio
	aggregated := PortfolioBalance{
		wallet_address = "AGGREGATED",
		sol_balance    = TokenBalance{
			mint      = "So11111111111111111111111111111111111111112",
			symbol    = "SOL",
			decimals  = 9,
		},
	}

	// Aggregate balances from all cached portfolios
	token_totals := make(map[string]TokenBalance)  // mint -> aggregated balance

	for address, portfolio in manager.portfolios {
		// Aggregate SOL
		aggregated.sol_balance.amount += portfolio.sol_balance.amount
		aggregated.sol_balance.usd_value += portfolio.sol_balance.usd_value
		aggregated.total_usd += portfolio.sol_balance.usd_value

		// Use latest SOL price
		if portfolio.sol_balance.usd_price > 0 {
			aggregated.sol_balance.usd_price = portfolio.sol_balance.usd_price
		}

		// Aggregate tokens
		for token_balance in portfolio.token_balances {
			existing, exists := token_totals[token_balance.mint]
			if exists {
				// Add to existing
				existing.amount += token_balance.amount
				existing.usd_value += token_balance.usd_value
				// Use latest price
				if token_balance.usd_price > 0 {
					existing.usd_price = token_balance.usd_price
				}
				token_totals[token_balance.mint] = existing
			} else {
				// New token
				token_totals[token_balance.mint] = token_balance
			}

			aggregated.total_usd += token_balance.usd_value
		}
	}

	// Convert map to slice
	token_list := make([dynamic]TokenBalance, 0, len(token_totals))
	for _, token_balance in token_totals {
		append(&token_list, token_balance)
	}
	aggregated.token_balances = token_list[:]

	log.debugf("Aggregated portfolio: %d token(s), $%.2f total",
		len(aggregated.token_balances), aggregated.total_usd)

	return aggregated
}
