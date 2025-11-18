package main

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:os"
import "core:path/filepath"
import "core:strings"

// PoolInfo represents a liquidity pool for a token
PoolInfo :: struct {
	dex:           string, // "raydium"
	pool_address:  string, // Pool account address
	quote_token:   string, // "sol", "usdc", etc.
	pool_type:     string, // "amm_v4"
	// Phase 5.3: Pool metadata
	liquidity_usd: f64,    // Current pool liquidity in USD (0.0 if unknown)
	volume_24h:    f64,    // 24-hour trading volume (0.0 if unknown)
	fee_percent:   f64,    // Trading fee percentage (0.0 if unknown)
	discovered_at: i64,    // Unix timestamp when auto-discovered (0 for manual)
}

// Token represents a single cryptocurrency token configuration
Token :: struct {
	symbol:           string,
	name:             string,
	contract_address: string,
	chain:            string,
	pools:            []PoolInfo, // Liquidity pools for on-chain pricing
	is_quote_token:   bool, // True if this is a quote token (SOL, USDC)
	usd_price:        f64, // USD price for quote tokens
}

// Wallet represents a Solana wallet address to watch
Wallet :: struct {
	address:    string, // Base58-encoded Solana address
	label:      string, // User-friendly name
	is_primary: bool,   // Primary wallet for display
}

// TokenConfig represents the complete token configuration file
TokenConfig :: struct {
	version: string,
	tokens:  []Token,
	wallets: []Wallet, // Watch-only wallet addresses
}

// load_token_config loads the token configuration from database or JSON fallback
// Returns the configuration and an error type
//
// Strategy (Phase 5.1):
// 1. Try loading from ~/.config/hound/tokens.db
// 2. If DB doesn't exist but tokens.json exists, migrate JSON -> DB
// 3. If neither exists, return ConfigNotFound
//
// This provides automatic migration on first use after Phase 5.1 deployment
load_token_config :: proc() -> (TokenConfig, ErrorType) {
	log.debug("Starting token config load")

	// Get home directory
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		log.error("Could not determine home directory")
		fmt.eprintln("ERROR: Could not determine home directory")
		return {}, .ConfigNotFound
	}
	log.debugf("Home directory: %s", home)

	db_path := filepath.join({home, ".config", "hound", "tokens.db"})
	json_path := filepath.join({home, ".config", "hound", "tokens.json"})

	if os.exists(db_path) {
		log.debugf("Loading tokens from database: %s", db_path)
		return load_token_config_from_db(db_path)
	}

	if os.exists(json_path) {
		log.debugf("Database not found, migrating from JSON: %s", json_path)

		json_config, json_err := load_token_config_from_json(json_path)
		if json_err != .None {
			return {}, json_err
		}

		db, db_err := database_open(db_path)
		if db_err != .None {
			log.errorf("Failed to create database for migration")
			return {}, db_err
		}
		defer database_close(db)

		schema_err := create_schema(db)
		if schema_err != .None {
			log.errorf("Failed to create database schema")
			return {}, schema_err
		}

		// Apply schema migrations after initial schema creation
		migrate_schema_err := migrate_schema_5_3(db)
		if migrate_schema_err != .None {
			log.warnf("Schema migration failed (non-fatal): %v", migrate_schema_err)
		}

		migrate_err := migrate_from_json(db, json_config)
		if migrate_err != .None {
			log.errorf("Failed to migrate JSON to database")
			return {}, migrate_err
		}

		log.info("Migration completed successfully")
		return json_config, .None
	}

	// Neither DB nor JSON exists
	log.errorf("No config found at %s or %s", db_path, json_path)
	fmt.eprintfln("Config not found. Please create a config file.")
	return {}, .ConfigNotFound
}

// load_token_config_from_db loads tokens from the database
//
// Internal helper for load_token_config
load_token_config_from_db :: proc(db_path: string) -> (TokenConfig, ErrorType) {
	log.debugf("Opening database: %s", db_path)

	db, db_err := database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		return {}, .DatabaseError
	}
	defer database_close(db)

	// Apply any pending schema migrations
	migrate_err := migrate_schema_5_3(db)
	if migrate_err != .None {
		log.warnf("Schema migration failed (non-fatal): %v", migrate_err)
		// Continue anyway - migration failures are non-fatal
	}

	// Integrity check
	if !database_integrity_check(db) {
		log.error("Database integrity check failed")
		return {}, .DatabaseCorrupted
	}

	// Load all tokens
	tokens, get_err := get_all_tokens(db)
	if get_err != .None {
		log.errorf("Failed to load tokens from database: %v", get_err)
		return {}, .DatabaseError
	}

	log.debugf("Loaded %d tokens from database", len(tokens))

	config := TokenConfig{
		version = "2.0.0",
		tokens  = tokens,
	}

	return config, .None
}

// load_token_config_from_json loads tokens from the JSON file
//
// Internal helper for migration path
load_token_config_from_json :: proc(json_path: string) -> (TokenConfig, ErrorType) {
	log.debugf("Loading JSON config: %s", json_path)

	// Read file
	data, read_ok := os.read_entire_file_from_filename(json_path)
	if !read_ok {
		log.error("Failed to read config file")
		fmt.eprintln("Failed to read config file")
		return {}, .ConfigNotFound
	}
	defer delete(data)
	log.debugf("Read %d bytes from config file", len(data))

	// Parse JSON
	config: TokenConfig
	err := json.unmarshal(data, &config)
	if err != nil {
		log.errorf("Failed to parse config JSON: %v", err)
		fmt.eprintfln("Failed to parse config: %v", err)
		return {}, .ConfigParseError
	}
	log.debugf("Parsed config version: %s", config.version)

	// Validate config has tokens
	if len(config.tokens) == 0 {
		log.error("Config file contains no tokens")
		fmt.eprintln("Config file contains no tokens")
		return {}, .ConfigParseError
	}
	log.infof("Loaded %d tokens from JSON", len(config.tokens))

	return config, .None
}

// find_token_by_symbol searches for a token by its symbol (case-insensitive)
// Returns the token and true if found, or an empty token and false if not found
find_token_by_symbol :: proc(config: TokenConfig, symbol: string) -> (Token, bool) {
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
list_tokens :: proc(config: TokenConfig) {
	fmt.println("Available tokens:")
	fmt.println("")

	for token in config.tokens {
		fmt.printfln("  %s - %s", token.symbol, token.name)
	}
}

// list_tokens_with_stats prints tokens with pool metadata (Phase 5.3)
//
// Enhanced list output showing:
// - Pool count for each token
// - Total liquidity across pools
// - ✨ indicator for auto-discovered tokens (discovered_at > 0)
//
// Fallback: If database unavailable, uses basic list_tokens()
list_tokens_with_stats :: proc(config: TokenConfig) {
	log.debug("Listing tokens with pool statistics")

	// Try to open database for stats
	db_path := get_database_path()
	db, db_err := database_open(db_path)
	if db_err != .None {
		log.warnf("Database unavailable, falling back to basic list")
		list_tokens(config)
		return
	}
	defer database_close(db)

	fmt.println("Available tokens:")
	fmt.println("")

	for token in config.tokens {
		// Get pool stats for this token
		stats, stats_err := get_pool_stats(db, token.symbol)

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
// POOL DISCOVERY INTEGRATION - Phase 5.2/5.3
// =============================================================================

// Phase 5.3: Store top N pools (not just best)
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
// Phase 5.3: Added force_refresh parameter to bypass cache
//
// Returns: Best pool and .None on success, or empty pool and error on failure
discover_and_store_pools :: proc(token: Token, force_refresh: bool = false) -> (PoolInfo, ErrorType) {
	log.infof("Starting automatic pool discovery for token: %s (force_refresh=%v)", token.symbol, force_refresh)

	// ASSERTION 1: Token must have contract address
	assert(len(token.contract_address) > 0, "Token contract address must not be empty")

	// Step 1: Query DexScreener API for pools (with retry and caching)
	log.debug("Step 1: Fetching pools from DexScreener API")
	pairs, api_err := get_pools_cached(token.contract_address, force_refresh)
	if api_err != .None {
		log.errorf("Failed to fetch pools from DexScreener: %v", api_err)
		return PoolInfo{}, api_err
	}
	defer delete(pairs)

	log.infof("DexScreener returned %d pool(s)", len(pairs))

	// Step 2: Filter + Rank pools (Phase 5.3: store top N, not just best)
	log.debug("Step 2: Filtering and ranking pools")
	filtered := filter_pools(pairs)
	defer delete(filtered)

	if len(filtered) == 0 {
		log.warn("No pools passed filtering criteria (min $1K liquidity, max 1% fee)")
		return PoolInfo{}, .NoPoolsFound
	}

	ranked := rank_pools(filtered)
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
	db, db_err := database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		// Non-fatal: return best pool even if storage fails
		return pair_to_pool_info(best_pair), .None
	}
	defer database_close(db)

	// Step 4: Store top N pools with metadata (Phase 5.3)
	stored_count := 0
	for i := 0; i < pools_to_store; i += 1 {
		pool_pair := ranked[i].pair
		pool_info := pair_to_pool_info(pool_pair)

		insert_err := insert_pool(
			db,
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
	return pair_to_pool_info(best_pair), .None
}

// get_database_path returns the standard database path
//
// Helper for database operations (avoids duplicating path logic)
get_database_path :: proc() -> string {
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		// Fallback to /tmp if HOME not set (should never happen)
		return "/tmp/hound_tokens.db"
	}
	return filepath.join({home, ".config", "hound", "tokens.db"})
}
