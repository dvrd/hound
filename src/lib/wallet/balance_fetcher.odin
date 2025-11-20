// Balance fetcher - integrates RPC client with price fetching
// Fetches wallet balances and converts to USD values
#+feature dynamic-literals
package wallet

import "core:fmt"
import "core:log"
import "../models"
import "../blockchain"
import "../database"
import "../dex"

// ============================================================================
// Types
// ============================================================================

// BalanceFetcher coordinates balance and price fetching
BalanceFetcher :: struct {
	rpc_client:   ^RPCClient,
	price_fetcher: ^PriceFetcher,  // Reuses existing price infrastructure
}

// PriceFetcher interface (from existing codebase)
// We'll use the existing fetch_price and fetch_onchain_price functions
PriceFetcher :: struct {
	// Empty for now - we'll use global functions
	// This is a placeholder to match the architecture
}

// TokenBalance represents a single token's balance
TokenBalance :: struct {
	mint:       string,  // Token mint address
	symbol:     string,  // Token symbol (e.g., "SOL", "USDC", "AURA")
	amount:     f64,     // Balance (adjusted for decimals)
	decimals:   int,     // Token decimals
	usd_price:  f64,     // Current USD price per token
	usd_value:  f64,     // Total USD value (amount * price)
}

// PortfolioBalance represents a wallet's complete portfolio
PortfolioBalance :: struct {
	wallet_address: string,
	sol_balance:    TokenBalance,
	token_balances: []TokenBalance,
	total_usd:      f64,  // Sum of all balances in USD
}

// ============================================================================
// Initialization
// ============================================================================

// init_balance_fetcher creates a new balance fetcher
//
// ASSERTION 1: RPC client must not be nil
//
// Returns: Initialized balance fetcher
init_balance_fetcher :: proc(
	rpc_client: ^RPCClient,
	price_fetcher: ^PriceFetcher,
) -> BalanceFetcher {
	assert(rpc_client != nil, "RPC client cannot be nil")

	log.debug("Initializing balance fetcher")

	return BalanceFetcher{
		rpc_client = rpc_client,
		price_fetcher = price_fetcher,
	}
}

// ============================================================================
// Balance Fetching
// ============================================================================

// fetch_portfolio_balance fetches complete portfolio for a wallet address
//
// ASSERTION 1: Fetcher must not be nil
// ASSERTION 2: Address must not be empty
//
// Returns: Portfolio balance and error status
fetch_portfolio_balance :: proc(
	fetcher: ^BalanceFetcher,
	address: string,
	config: ^models.TokenConfig,
	db: ^database.Database,
) -> (portfolio: PortfolioBalance, err: models.ErrorType) {
	assert(fetcher != nil, "Balance fetcher cannot be nil")
	assert(len(address) > 0, "Wallet address cannot be empty")
	assert(config != nil, "Token config cannot be nil")
	assert(db != nil, "Database cannot be nil")

	log.infof("Fetching portfolio balance for address: %s", address)

	portfolio.wallet_address = address

	// Step 1: Fetch SOL balance
	log.debug("Step 1: Fetching SOL balance")
	sol_lamports, sol_err := get_balance(fetcher.rpc_client, address)
	if sol_err != .None {
		log.errorf("Failed to fetch SOL balance: %v", sol_err)
		return {}, sol_err
	}

	// Convert lamports to SOL (1 SOL = 10^9 lamports)
	sol_amount := f64(sol_lamports) / 1_000_000_000.0
	log.debugf("SOL balance: %.9f SOL (%d lamports)", sol_amount, sol_lamports)

	// Step 2: Get SOL price in USD
	log.debug("Step 2: Fetching SOL price")
	sol_price, price_err := blockchain.get_sol_price_cached()
	if price_err != .None {
		log.warnf("Failed to fetch SOL price: %v (defaulting to $0)", price_err)
		sol_price = 0.0
	}
	log.debugf("SOL price: $%.2f", sol_price)

	// Calculate SOL value in USD
	sol_usd_value := sol_amount * sol_price

	portfolio.sol_balance = TokenBalance{
		mint      = "So11111111111111111111111111111111111111112",  // Wrapped SOL mint
		symbol    = "SOL",
		amount    = sol_amount,
		decimals  = 9,
		usd_price = sol_price,
		usd_value = sol_usd_value,
	}
	portfolio.total_usd = sol_usd_value

	// Step 3: Fetch SPL token accounts
	log.debug("Step 3: Fetching SPL token accounts")
	token_accounts, token_err := get_token_accounts_by_owner(fetcher.rpc_client, address)
	if token_err != .None {
		log.errorf("Failed to fetch token accounts: %v", token_err)
		return {}, token_err
	}
	defer delete(token_accounts)

	log.infof("Found %d token account(s)", len(token_accounts))

	// Step 4: Fetch prices and build token balances
	log.debug("Step 4: Fetching token prices")
	token_balance_list := make([dynamic]TokenBalance, 0, len(token_accounts))

	for account in token_accounts {
		// Skip zero balances
		if account.account.ui_amount <= 0 {
			log.debugf("Skipping token %s (zero balance)", account.account.mint)
			continue
		}

		log.debugf("Processing token: mint=%s, amount=%.6f",
			account.account.mint, account.account.ui_amount)

		// Try to find token in config for symbol and pricing info
		// First check in-memory config, then check database for auto-discovered tokens
		token, found := find_token_by_mint(config, account.account.mint)
		if !found {
			// Not in config - check database for previously discovered tokens
			db_token, db_found, db_err := database.get_token_by_contract_address(db, account.account.mint)
			if db_err == .None && db_found {
				token = db_token
				found = true
				log.debugf("Token found in database: %s", token.symbol)
			}
		}

		symbol := ""
		usd_price := 0.0
		usd_value := 0.0

		if found {
			symbol = token.symbol
			log.debugf("Token found in config: %s", symbol)

			// Get price (try on-chain first if pools configured, fallback to API)
			if len(token.pools) > 0 {
				log.debugf("Fetching on-chain price for %s", symbol)
				price_data, price_err := dex.fetch_onchain_price(token)
				if price_err == .None {
					usd_price = price_data.price_usd
					log.debugf("On-chain price for %s: $%.6f", symbol, usd_price)
				} else {
					log.warnf("On-chain price failed for %s, trying API", symbol)
					// Fallback to API
					price_data, api_err := dex.fetch_price(token.contract_address)
					if api_err == .None {
						usd_price = price_data.price_usd
						log.debugf("API price for %s: $%.6f", symbol, usd_price)
					} else {
						log.warnf("Failed to fetch price for %s: %v", symbol, api_err)
					}
				}
			} else {
				// No pools - try API directly
				log.debugf("Fetching API price for %s", symbol)
				price_data, api_err := dex.fetch_price(token.contract_address)
				if api_err == .None {
					usd_price = price_data.price_usd
					log.debugf("API price for %s: $%.6f", symbol, usd_price)
				} else {
					log.warnf("Failed to fetch price for %s: %v", symbol, api_err)
				}
			}

			usd_value = account.account.ui_amount * usd_price
		} else {
			// Token not in config - auto-discovery is disabled for now
			// TODO: Implement token auto-discovery
			// The lookup_token_metadata and save_discovered_token functions are in src/wallet/token_metadata.odin
			// which is a different package. Need to either:
			// 1. Move those functions to core/wallet, or
			// 2. Create a proper import path for src/wallet package
			log.infof("Token %s not found in config (auto-discovery disabled)", account.account.mint)

			// Fall back to truncated address as symbol
			symbol = fmt.tprintf("%s..%s",
				account.account.mint[:4], account.account.mint[len(account.account.mint)-4:])
			log.debugf("Using shortened mint as symbol: %s", symbol)

			// Try to fetch price by contract address
			price_data, api_err := dex.fetch_price(account.account.mint)
			if api_err == .None {
				usd_price = price_data.price_usd
				usd_value = account.account.ui_amount * usd_price
				log.debugf("API price for unknown token: $%.6f", usd_price)
			} else {
				log.warnf("Failed to fetch price for unknown token: %v", api_err)
			}
		}

		// Add to balance list
		token_balance := TokenBalance{
			mint      = account.account.mint,
			symbol    = symbol,
			amount    = account.account.ui_amount,
			decimals  = account.account.decimals,
			usd_price = usd_price,
			usd_value = usd_value,
		}
		append(&token_balance_list, token_balance)

		// Update total USD value
		portfolio.total_usd += usd_value
	}

	portfolio.token_balances = token_balance_list[:]

	log.infof("Portfolio fetch complete: %d token(s), total value: $%.2f",
		len(portfolio.token_balances), portfolio.total_usd)

	return portfolio, .None
}

// ============================================================================
// Helper Functions
// ============================================================================

// find_token_by_mint searches for a token by its mint address
//
// Returns: Token and found flag
find_token_by_mint :: proc(config: ^models.TokenConfig, mint: string) -> (token: models.Token, found: bool) {
	for t in config.tokens {
		if t.contract_address == mint {
			return t, true
		}
	}
	return {}, false
}
