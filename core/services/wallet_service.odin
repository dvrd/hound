// Wallet service - business logic for wallet operations
// Stateless service functions for wallet management, portfolio fetching, and aggregation
package services

import "core:log"
import "../models"
import "../database"
import "../blockchain"
import "../wallet"

// ============================================================================
// Service Context
// ============================================================================

// WalletServiceContext holds dependencies for wallet operations
//
// This context is passed to all service functions, enabling:
// - Dependency injection
// - Stateless service functions
// - Easy testing with mock contexts
WalletServiceContext :: struct {
	db:              ^database.Database,
	rpc_client:      ^wallet.RPCClient,
	balance_fetcher: ^wallet.BalanceFetcher,
	config:          ^models.TokenConfig,
}

// ============================================================================
// Portfolio Operations
// ============================================================================

// fetch_and_persist_portfolio fetches portfolio and persists to database
//
// This is the core wallet refresh operation that:
// 1. Fetches on-chain balances via RPC
// 2. Enriches with USD prices
// 3. Persists balances to database for history
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Address must not be empty
//
// Returns: Portfolio balance and error status
fetch_and_persist_portfolio :: proc(
	ctx: ^WalletServiceContext,
	address: string,
) -> (portfolio: wallet.PortfolioBalance, err: models.ErrorType) {
	assert(ctx != nil, "Wallet service context cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")

	log.infof("Fetching and persisting portfolio for: %s", address)

	// Step 1: Fetch portfolio from blockchain
	fetched_portfolio, fetch_err := wallet.fetch_portfolio_balance(
		ctx.balance_fetcher,
		address,
		ctx.config,
		ctx.db,
	)
	if fetch_err != .None {
		log.errorf("Failed to fetch portfolio: %v", fetch_err)
		return {}, fetch_err
	}
	portfolio = fetched_portfolio

	// Step 2: Persist SOL balance to database
	sol_err := database.update_balance(
		ctx.db,
		address,
		portfolio.sol_balance.mint,
		portfolio.sol_balance.symbol,
		portfolio.sol_balance.amount,
		portfolio.sol_balance.usd_price,
		portfolio.sol_balance.usd_value,
	)
	if sol_err != .None {
		log.warnf("Failed to persist SOL balance (non-fatal): %v", sol_err)
	}

	// Step 3: Persist token balances to database
	for token_balance in portfolio.token_balances {
		token_err := database.update_balance(
			ctx.db,
			address,
			token_balance.mint,
			token_balance.symbol,
			token_balance.amount,
			token_balance.usd_price,
			token_balance.usd_value,
		)
		if token_err != .None {
			log.warnf("Failed to persist token balance (non-fatal): %v (token: %s)",
				token_err, token_balance.symbol)
		}
	}

	log.infof("Portfolio fetched and persisted: $%.2f total", portfolio.total_usd)
	return portfolio, .None
}

// ============================================================================
// Batch Operations
// ============================================================================

// RefreshPolicy defines how to handle errors during batch refresh
RefreshPolicy :: enum {
	FailFast,      // Stop on first error
	BestEffort,    // Continue on errors, return partial results
}

// RefreshResult represents the result of refreshing multiple wallets
RefreshResult :: struct {
	portfolios:    map[string]wallet.PortfolioBalance,  // address -> portfolio
	success_count: int,
	failure_count: int,
	errors:        map[string]models.ErrorType,         // address -> error
}

// refresh_multiple_with_policy refreshes multiple wallets with error policy
//
// This enables efficient batch refresh operations with configurable error handling:
// - FailFast: Stop immediately on first error
// - BestEffort: Continue refreshing remaining wallets, collect all errors
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Addresses slice must not be empty
//
// Returns: Refresh result with portfolios, counts, and errors
refresh_multiple_with_policy :: proc(
	ctx: ^WalletServiceContext,
	addresses: []string,
	policy: RefreshPolicy = .BestEffort,
) -> (result: RefreshResult, err: models.ErrorType) {
	assert(ctx != nil, "Wallet service context cannot be nil")
	assert(len(addresses) > 0, "Addresses slice cannot be empty")

	log.infof("Refreshing %d wallet(s) with policy: %v", len(addresses), policy)

	// Initialize result
	result.portfolios = make(map[string]wallet.PortfolioBalance)
	result.errors = make(map[string]models.ErrorType)

	// Refresh each wallet
	for address in addresses {
		portfolio, refresh_err := fetch_and_persist_portfolio(ctx, address)

		if refresh_err == .None {
			// Success
			result.portfolios[address] = portfolio
			result.success_count += 1
			log.debugf("Wallet %s refreshed successfully", address)
		} else {
			// Failure
			result.errors[address] = refresh_err
			result.failure_count += 1
			log.warnf("Wallet %s refresh failed: %v", address, refresh_err)

			// Check policy
			if policy == .FailFast {
				log.error("FailFast policy: stopping on first error")
				return result, refresh_err
			}
		}
	}

	log.infof("Batch refresh complete: %d/%d successful",
		result.success_count, len(addresses))

	// Determine overall status
	if result.failure_count == len(addresses) {
		// All failed
		log.error("All wallet refreshes failed")
		return result, .RPCConnectionFailed
	} else if result.failure_count > 0 {
		// Partial success
		log.warnf("Partial success: %d failures", result.failure_count)
		return result, .None  // BestEffort succeeded
	}

	// All succeeded
	return result, .None
}

// ============================================================================
// Portfolio Aggregation
// ============================================================================

// aggregate_portfolios combines multiple portfolios into a single view
//
// This enables:
// - Total portfolio view across all wallets
// - Consolidated token balances
// - Aggregate USD value
//
// ASSERTION 1: Portfolios map must not be empty
//
// Returns: Aggregated portfolio balance
aggregate_portfolios :: proc(
	portfolios: map[string]wallet.PortfolioBalance,
) -> wallet.PortfolioBalance {
	assert(len(portfolios) > 0, "Portfolios map cannot be empty")

	log.debugf("Aggregating %d portfolio(s)", len(portfolios))

	// Initialize aggregated portfolio
	aggregated := wallet.PortfolioBalance{
		wallet_address = "AGGREGATED",
		sol_balance    = wallet.TokenBalance{
			mint     = "So11111111111111111111111111111111111111112",
			symbol   = "SOL",
			decimals = 9,
		},
	}

	// Token aggregation map: mint -> token balance
	token_totals := make(map[string]wallet.TokenBalance)

	// Aggregate all portfolios
	for _, portfolio in portfolios {
		// Aggregate SOL balance
		aggregated.sol_balance.amount += portfolio.sol_balance.amount
		aggregated.sol_balance.usd_value += portfolio.sol_balance.usd_value
		aggregated.total_usd += portfolio.sol_balance.usd_value

		// Use latest SOL price
		if portfolio.sol_balance.usd_price > 0 {
			aggregated.sol_balance.usd_price = portfolio.sol_balance.usd_price
		}

		// Aggregate token balances
		for token_balance in portfolio.token_balances {
			existing, exists := token_totals[token_balance.mint]
			if exists {
				// Accumulate existing token
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

	// Convert token map to slice
	token_list := make([dynamic]wallet.TokenBalance, 0, len(token_totals))
	for _, token_balance in token_totals {
		append(&token_list, token_balance)
	}
	aggregated.token_balances = token_list[:]

	log.infof("Aggregated portfolio: %d token(s), $%.2f total",
		len(aggregated.token_balances), aggregated.total_usd)

	return aggregated
}

// ============================================================================
// Wallet Management
// ============================================================================

// validate_and_add_wallet validates address and adds wallet to database
//
// This encapsulates:
// - Address format validation
// - Duplicate checking
// - Database insertion
//
// ASSERTION 1: Context must not be nil
// ASSERTION 2: Address must not be empty
// ASSERTION 3: Label must not be empty
//
// Returns: Error status
validate_and_add_wallet :: proc(
	ctx: ^WalletServiceContext,
	address: string,
	label: string,
	is_primary: bool = false,
) -> models.ErrorType {
	assert(ctx != nil, "Wallet service context cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")
	assert(len(label) > 0, "Wallet label cannot be empty")

	log.infof("Validating and adding wallet: %s (%s)", label, address)

	// Step 1: Validate Solana address format
	if !blockchain.validate_solana_address(address) {
		log.error("Invalid Solana address format")
		return .InvalidToken
	}

	// Step 2: Create wallet struct
	wallet_obj := models.Wallet{
		address    = address,
		label      = label,
		is_primary = is_primary,
	}

	// Step 3: Insert into database (handles duplicates)
	db_err := database.insert_wallet(ctx.db, wallet_obj)
	if db_err != .None {
		log.errorf("Failed to insert wallet into database: %v", db_err)
		return db_err
	}

	log.infof("Wallet added successfully: %s", label)
	return .None
}

// ============================================================================
// Wallet Retrieval
// ============================================================================

// get_all_wallets retrieves all configured wallets from database
//
// ASSERTION 1: Context must not be nil
//
// Returns: Array of wallets and error status
get_all_wallets :: proc(ctx: ^WalletServiceContext) -> (wallets: []models.Wallet, err: models.ErrorType) {
	assert(ctx != nil, "Wallet service context cannot be nil")

	log.debug("Fetching all wallets from database")

	wallet_list, db_err := database.get_all_wallets(ctx.db)
	if db_err != .None {
		log.errorf("Failed to fetch wallets: %v", db_err)
		return nil, db_err
	}

	log.infof("Fetched %d wallet(s)", len(wallet_list))
	return wallet_list, .None
}
