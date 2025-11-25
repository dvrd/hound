// Fetch command implementation
// Discovers pools and fetches token prices with fallback strategies
package commands

import "core:fmt"
import "core:log"
import models "../../lib/models"
import dex "../../lib/dex"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import output "../output"

// ============================================================================
// Fetch Command
// ============================================================================

// handle_fetch implements the "hound fetch <symbol>" workflow
//
// Workflow:
// 1. Load token configuration
// 2. Check if --refresh flag present
// 3. Perform pool discovery (with force_refresh if --refresh)
// 4. Fetch price using discovered/configured pools
// 5. Display result
//
// Progress indicators for slow operations (>1s)
// Returns: ErrorType for consistent error handling
handle_fetch :: proc(symbol: string, force_refresh: bool) -> models.ErrorType {
	log.debugf("Fetch command: symbol=%s, force_refresh=%v", symbol, force_refresh)

	// Load token configuration
	log.debug("Loading token configuration")
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Find token by symbol
	log.debugf("Looking up token: %s", symbol)
	token, found := token_cfg.find_token_by_symbol(config, symbol)
	if !found {
		log.warnf("Token not found in configuration: %s", symbol)
		return .TokenNotConfigured
	}
	log.infof("Found token: %s (%s)", token.symbol, token.name)

	// Try on-chain fetch with existing or discovered pools
	price_data: models.PriceData
	err: models.ErrorType

	if len(token.pools) > 0 && !force_refresh {
		// Use existing configured pools
		log.infof("Using %d configured pool(s)", len(token.pools))
		price_data, err = dex.fetch_onchain_price(token)
		if err != .None {
			log.warnf("On-chain fetch failed (%v), falling back to API", err)
			price_data, err = dex.fetch_price(token.contract_address)
		} else {
			log.info("On-chain price fetch successful")
		}
	} else {
		// Pool discovery needed (no pools or force refresh)
		if force_refresh {
			output.print_progress("Refreshing pool discovery...")
		} else {
			output.print_progress("Discovering liquidity pools...")
		}

		// Pass force_refresh to bypass cache
		pool_info, discovery_err := discover_and_store_pools_with_refresh(token, force_refresh)
		if discovery_err == .None {
			log.infof("Pool discovery succeeded: %s pool at %s", pool_info.dex, pool_info.pool_address)
			output.format_pool_summary(pool_info.dex, pool_info.pool_address, pool_info.liquidity_usd)

			// Create temporary token with discovered pool
			token_with_pool := token
			token_with_pool.pools = []models.PoolInfo{pool_info}

			// Fetch price from discovered pool
			price_data, err = dex.fetch_onchain_price(token_with_pool)
			if err != .None {
				log.warnf("On-chain fetch failed after discovery (%v), falling back to API", err)
				price_data, err = dex.fetch_price(token.contract_address)
			} else {
				log.info("On-chain price fetch successful from discovered pool")
			}
		} else {
			// Pool discovery failed - fallback to API
			log.warnf("Pool discovery failed (%v), falling back to API", discovery_err)
			price_data, err = dex.fetch_price(token.contract_address)
		}
	}

	// Reset request arena after all RPC operations complete
	memory.reset_request_arena()

	if err != .None {
		log.errorf("Price fetch failed with error: %v", err)
		return err
	}

	log.infof("Price fetched successfully: $%.6f", price_data.price_usd)

	// Display result using output formatter
	output.format_price(token.symbol, price_data)

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// ============================================================================
// Helper Functions
// ============================================================================

// discover_and_store_pools_with_refresh is a wrapper to pass force_refresh to pool discovery
//
// Used by: handle_fetch when pools need to be discovered or refreshed
// Pattern: Delegates to token_config layer
discover_and_store_pools_with_refresh :: proc(
	token: models.Token,
	force_refresh: bool,
) -> (models.PoolInfo, models.ErrorType) {
	return token_cfg.discover_and_store_pools(token, force_refresh)
}
