package token_config

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:os"
import "core:path/filepath"
import "core:strings"
import models "../../core/models"
import db "../../core/database"
import dex "../../core/dex"

// load_token_config loads the token configuration from database
// Returns the configuration and an error type
//
// Strategy:
// 1. Try loading from ~/.config/hound/hound.db
// 2. If DB doesn't exist, return ConfigNotFound
load_token_config :: proc() -> (models.TokenConfig, models.ErrorType) {
	log.debug("Starting token config load")

	// Get home directory
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		log.error("Could not determine home directory")
		fmt.eprintln("ERROR: Could not determine home directory")
		return {}, .ConfigNotFound
	}
	log.debugf("Home directory: %s", home)

	db_path := filepath.join({home, ".config", "hound", "hound.db"})

	if !os.exists(db_path) {
		log.errorf("Database not found at %s", db_path)
		fmt.eprintfln("Database not found. Use 'hound add' to add your first token.")
		return {}, .ConfigNotFound
	}

	log.debugf("Loading tokens from database: %s", db_path)
	return load_token_config_from_db(db_path)
}

// load_token_config_from_db loads tokens from the database
//
// Internal helper for load_token_config
load_token_config_from_db :: proc(db_path: string) -> (models.TokenConfig, models.ErrorType) {
	log.debugf("Opening database: %s", db_path)

	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return {}, .DatabaseError
	}
	defer db.database_close(database)

	// Apply any pending schema migrations
	migrate_err := db.migrate_schema_5_3(database)
	if migrate_err != .None {
		log.warnf("Schema migration failed (non-fatal): %v", migrate_err)
		// Continue anyway - migration failures are non-fatal
	}

	// Integrity check
	if !db.database_integrity_check(database) {
		log.error("Database integrity check failed")
		return {}, .DatabaseCorrupted
	}

	// Load all tokens
	tokens, get_err := db.get_all_tokens(database)
	if get_err != .None {
		log.errorf("Failed to load tokens from database: %v", get_err)
		return {}, .DatabaseError
	}

	log.debugf("Loaded %d tokens from database", len(tokens))

	config := models.TokenConfig{
		version = "2.0.0",
		tokens  = tokens,
	}

	return config, .None
}


// find_token_by_symbol searches for a token by its symbol (case-insensitive)
// Returns the token and true if found, or an empty token and false if not found
find_token_by_symbol :: proc(config: models.TokenConfig, symbol: string) -> (models.Token, bool) {
	log.debugf("Searching for token symbol: %s", symbol)
	lower_symbol := strings.to_lower(symbol)

	for token in config.tokens {
		if strings.to_lower(token.symbol) == lower_symbol {
			log.debugf("Found token: %s (address: %s)", token.name, token.contract_address)
			return token, true
		}
	}

	log.debugf("Token not found: %s", symbol)
	return {}, false
}

// list_tokens prints all available tokens from the configuration
list_tokens :: proc(config: models.TokenConfig) {
	fmt.println("Available tokens:")
	fmt.println("")

	for token in config.tokens {
		fmt.printfln("  %s - %s", token.symbol, token.name)
	}
}

// list_tokens_with_stats prints tokens with pool metadata
//
// Enhanced list output showing:
// - Pool count for each token
// - Total liquidity across pools
// - ✨ indicator for auto-discovered tokens (discovered_at > 0)
//
// Fallback: If database unavailable, uses basic list_tokens()
list_tokens_with_stats :: proc(config: models.TokenConfig) {
	log.debug("Listing tokens with pool statistics")

	// Try to open database for stats
	db_path := get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.warnf("Database unavailable, falling back to basic list")
		list_tokens(config)
		return
	}
	defer db.database_close(database)

	fmt.println("Available tokens:")
	fmt.println("")

	for token in config.tokens {
		// Get pool stats for this token
		stats, stats_err := db.get_pool_stats(database, token.symbol)

		if stats_err != .None || stats.pool_count == 0 {
			// No pools configured
			fmt.printfln("  %s - %s (no pools)", token.symbol, token.name)
			continue
		}

		// Check if any pool was auto-discovered
		has_discovered_pools := false
		for pool in token.pools {
			if pool.discovered_at > 0 {
				has_discovered_pools = true
				break
			}
		}

		// Format output with pool stats
		discovery_indicator := has_discovered_pools ? " ✨" : ""
		if stats.total_liquidity > 0 {
			fmt.printfln("  %s - %s (%d pool%s, $%.0f liquidity)%s",
				token.symbol, token.name,
				stats.pool_count, stats.pool_count == 1 ? "" : "s",
				stats.total_liquidity, discovery_indicator)
		} else {
			// Pools exist but no liquidity data
			fmt.printfln("  %s - %s (%d pool%s)%s",
				token.symbol, token.name,
				stats.pool_count, stats.pool_count == 1 ? "" : "s",
				discovery_indicator)
		}
	}
}

// =============================================================================
// POOL DISCOVERY INTEGRATION
// =============================================================================

// Store top N pools (not just best)
TOP_POOLS_TO_STORE :: 3

// discover_and_store_pools performs automatic pool discovery for a token
//
// Workflow:
// 1. Query DexScreener API for all pools trading this token
// 2. Filter pools by minimum liquidity ($1K) and maximum fee (1%)
// 3. Rank pools by liquidity-weighted score
// 4. Select best pool
// 5. Store pool in database for future use
//
// This is called during "hound fetch <symbol>" when no pools are configured
// for the token yet. It enables zero-configuration price fetching.
//
// The force_refresh parameter bypasses the cache to get fresh pool data
//
// Returns: Best pool and .None on success, or empty pool and error on failure
discover_and_store_pools :: proc(token: models.Token, force_refresh: bool = false) -> (models.PoolInfo, models.ErrorType) {
	log.infof("Starting automatic pool discovery for token: %s (force_refresh=%v)", token.symbol, force_refresh)

	// ASSERTION 1: Token must have contract address
	assert(len(token.contract_address) > 0, "Token contract address must not be empty")

	// Step 1: Query DexScreener API for pools (with retry and caching)
	log.debug("Step 1: Fetching pools from DexScreener API")
	pairs, api_err := dex.get_pools_cached(token.contract_address, force_refresh)
	if api_err != .None {
		log.errorf("Failed to fetch pools from DexScreener: %v", api_err)
		return models.PoolInfo{}, api_err
	}
	defer delete(pairs)

	log.infof("DexScreener returned %d pool(s)", len(pairs))

	// Step 2: Filter + Rank pools (store top N, not just best)
	log.debug("Step 2: Filtering and ranking pools")
	filtered := dex.filter_pools(pairs)
	defer delete(filtered)

	if len(filtered) == 0 {
		log.warn("No pools passed filtering criteria (min $1K liquidity, max 1% fee)")
		return models.PoolInfo{}, .NoPoolsFound
	}

	ranked := dex.rank_pools(filtered)
	defer delete(ranked)

	// Determine how many pools to store (top N or all if fewer)
	pools_to_store := min(TOP_POOLS_TO_STORE, len(ranked))
	log.infof("Found %d valid pool(s), storing top %d", len(ranked), pools_to_store)

	// Best pool for immediate use
	best_pair := ranked[0].pair
	log.infof("Best pool: %s on %s (score: %.2f, liquidity: $%.2f)",
		best_pair.pairAddress, best_pair.dexId, ranked[0].score, best_pair.liquidity.usd)

	// Step 3: Open database for storage
	log.debug("Step 3: Storing pools in database")
	db_path := get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		// Non-fatal: return best pool even if storage fails
		return dex.pair_to_pool_info(best_pair), .None
	}
	defer db.database_close(database)

	// Step 3.5: If force_refresh, delete old pools first to avoid duplicates
	if force_refresh {
		log.debugf("Force refresh: deleting old pools for %s", token.symbol)
		delete_err := db.delete_pools_for_token(database, token.symbol)
		if delete_err != .None {
			log.warnf("Failed to delete old pools (non-fatal): %v", delete_err)
		}
	}

	// Step 4: Store top N pools with metadata
	stored_count := 0
	for i := 0; i < pools_to_store; i += 1 {
		pool_pair := ranked[i].pair
		pool_info := dex.pair_to_pool_info(pool_pair)

		insert_err := db.insert_pool(
			database,
			token.symbol,
			pool_info,
			pool_info.liquidity_usd,
			pool_info.volume_24h,
			pool_info.fee_percent,
			pool_info.discovered_at,
		)

		if insert_err != .None {
			log.warnf("Failed to store pool #%d (%s): %v", i+1, pool_pair.pairAddress, insert_err)
			continue
		}

		stored_count += 1
		log.debugf("Stored pool #%d: %s on %s ($%.2f liquidity)",
			i+1, pool_pair.pairAddress, pool_pair.dexId, pool_pair.liquidity.usd)
	}

	log.infof("Pool discovery completed: stored %d pool(s) for %s",
		stored_count, token.symbol)

	// Return best pool for immediate use
	return dex.pair_to_pool_info(best_pair), .None
}

// get_database_path returns the standard database path
//
// Helper for database operations (avoids duplicating path logic)
get_database_path :: proc() -> string {
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		// Fallback to /tmp if HOME not set (should never happen)
		return "/tmp/hound.db"
	}
	return filepath.join({home, ".config", "hound", "hound.db"})
}
