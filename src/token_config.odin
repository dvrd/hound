package main

import "core:encoding/json"
import "core:fmt"
import "core:log"
import "core:os"
import "core:path/filepath"
import "core:strings"

// PoolInfo represents a liquidity pool for a token
PoolInfo :: struct {
	dex:          string, // "raydium"
	pool_address: string, // Pool account address
	quote_token:  string, // "sol", "usdc", etc.
	pool_type:    string, // "amm_v4"
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

// =============================================================================
// POOL DISCOVERY INTEGRATION - Phase 5.2
// =============================================================================

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
// Returns: Best pool and .None on success, or empty pool and error on failure
discover_and_store_pools :: proc(token: Token) -> (PoolInfo, ErrorType) {
	log.infof("Starting automatic pool discovery for token: %s", token.symbol)

	// ASSERTION 1: Token must have contract address
	assert(len(token.contract_address) > 0, "Token contract address must not be empty")

	// Step 1: Query DexScreener API for pools (with retry and caching)
	log.debug("Step 1: Fetching pools from DexScreener API")
	pairs, api_err := get_pools_cached(token.contract_address)
	if api_err != .None {
		log.errorf("Failed to fetch pools from DexScreener: %v", api_err)
		return PoolInfo{}, api_err
	}
	defer delete(pairs)

	log.infof("DexScreener returned %d pool(s)", len(pairs))

	// Step 2: Filter + Rank + Select best pool
	log.debug("Step 2: Selecting best pool via ranking algorithm")
	best_pair, found := select_best_pool(pairs)
	if !found {
		log.warn("No pools passed filtering criteria (min $1K liquidity, max 1% fee)")
		return PoolInfo{}, .NoPoolsFound
	}

	log.infof("Selected best pool: %s on %s (liquidity: $%.2f)",
		best_pair.pairAddress, best_pair.dexId, best_pair.liquidity.usd)

	// Step 3: Convert DexScreenerPair to PoolInfo
	pool_info := pair_to_pool_info(best_pair)

	// Step 4: Store pool in database (for future fast lookups)
	log.debug("Step 3: Storing pool in database")
	db_path := get_database_path()
	db, db_err := database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		// Non-fatal: we have pool info, just can't cache it
		return pool_info, .None
	}
	defer database_close(db)

	// Insert pool into database (linked to token)
	insert_err := insert_pool(db, token.symbol, pool_info)
	if insert_err != .None {
		log.warnf("Failed to store pool in database: %v (non-fatal)", insert_err)
		// Non-fatal: pool discovery succeeded, storage failed
		return pool_info, .None
	}

	log.infof("Pool discovery completed: %s pool at %s stored for future use",
		pool_info.dex, pool_info.pool_address)

	return pool_info, .None
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
