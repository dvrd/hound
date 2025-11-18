#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:strings"
import "core:time"
import "core:os"
import "core:path/filepath"
import sqlite3 "../vendor/odin-sqlite3"

// =============================================================================
// DATABASE MODULE - SQLite Token and Pool Storage (Phase 5.1)
// =============================================================================
// This module implements persistent storage for token configuration and pool
// data using SQLite3. It replaces the JSON-based storage with a proper database
// that supports:
// - Fast lookups (case-insensitive token symbol search)
// - TTL-based cache management (24h default)
// - Foreign key constraints (pool -> token relationship)
// - WAL mode for crash safety
// - Automatic migration from tokens.json
//
// Architecture:
// - Database handle wraps SQLite3 connection
// - All functions return (result, ErrorType) tuple
// - Prepared statements with defer finalize
// - Transaction-wrapped writes for atomicity
//
// Tables:
// - tokens: Token configuration with metadata
// - pools: Pool configuration with foreign key to tokens
//
// Critical Gotchas (from odin-sqlite3-guide.md):
// 1. Always defer sqlite3.finalize(stmt) after prepare
// 2. Parameter binding is 1-indexed (not 0-indexed!)
// 3. Text strings must stay in scope until step() completes
// 4. WAL mode and foreign keys must be enabled explicitly via PRAGMA
// =============================================================================

// Database handle wrapping SQLite3 connection
Database :: struct {
	handle: ^sqlite3.Connection,
	path:   string,
}

// database_open opens or creates a SQLite database at the given path
//
// ASSERTION 1: Validate db_path is not empty
//
// Steps:
// 1. Open SQLite connection
// 2. Enable critical PRAGMAs (WAL mode, foreign keys, busy timeout)
// 3. Wrap in Database struct
//
// Returns: Database handle and error status
database_open :: proc(db_path: string) -> (^Database, ErrorType) {
	assert(len(db_path) > 0, "Database path cannot be empty")
	log.debugf("Opening database: %s", db_path)

	db_handle: ^sqlite3.Connection
	result := sqlite3.open(cstring(raw_data(db_path)), &db_handle)
	if result != .Ok {
		log.errorf("Failed to open database: %v", result)
		return nil, .DatabaseError
	}

	db := new(Database)
	db.handle = db_handle
	db.path = db_path

	// CRITICAL: Enable essential PRAGMAs
	// - foreign_keys: Enforce FK constraints (SQLite default is OFF!)
	// - journal_mode=WAL: Write-Ahead Logging for crash safety + concurrency
	// - synchronous=NORMAL: Safe compromise (FULL is overkill for local app)
	// - busy_timeout: Wait up to 5 seconds if database is locked
	pragmas := "PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;"
	errmsg: cstring
	pragma_result := sqlite3.exec(db.handle, cstring(raw_data(pragmas)), nil, nil, &errmsg)
	if pragma_result != .Ok {
		log.warnf("Failed to enable database pragmas: %s", errmsg)
		sqlite3.free(cast(rawptr)errmsg)
	}

	log.infof("Database opened: %s", db_path)
	return db, .None
}

// database_close closes the database connection and frees resources
//
// ASSERTION 1: Validate db is not nil
database_close :: proc(db: ^Database) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	log.debugf("Closing database: %s", db.path)
	sqlite3.close(db.handle)
	free(db)
	log.debug("Database closed")
}

// create_schema creates the tokens and pools tables if they don't exist
//
// ASSERTION 1: Validate db is not nil
//
// Schema:
// - tokens table: id, symbol, name, contract_address, chain, is_quote_token,
//                 usd_price, discovered_at, last_updated, cache_ttl
// - pools table: id, token_id (FK), dex, pool_address, quote_token, pool_type
//
// Returns: Error status
create_schema :: proc(db: ^Database) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	log.debug("Creating database schema")

	// Schema SQL from PRP (Phase 5.1 + Phase 1 Wallet Extension)
	schema_sql := `
		CREATE TABLE IF NOT EXISTS tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE COLLATE NOCASE,
			name TEXT NOT NULL,
			contract_address TEXT NOT NULL UNIQUE,
			chain TEXT NOT NULL DEFAULT 'solana',
			is_quote_token INTEGER DEFAULT 0,
			usd_price REAL DEFAULT 0.0,
			discovered_at INTEGER NOT NULL,
			last_updated INTEGER NOT NULL,
			cache_ttl INTEGER DEFAULT 86400
		);

		CREATE INDEX IF NOT EXISTS idx_tokens_symbol ON tokens(symbol COLLATE NOCASE);
		CREATE INDEX IF NOT EXISTS idx_tokens_contract ON tokens(contract_address);

		CREATE TABLE IF NOT EXISTS pools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER NOT NULL,
			dex TEXT NOT NULL,
			pool_address TEXT NOT NULL,
			quote_token TEXT NOT NULL,
			pool_type TEXT NOT NULL,
			FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_pools_token ON pools(token_id);

		-- Wallet Tables (Phase 1: Watch-Only Wallet Foundation)
		CREATE TABLE IF NOT EXISTS wallets (
			address TEXT PRIMARY KEY,
			label TEXT NOT NULL,
			is_primary INTEGER DEFAULT 0,
			added_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_wallets_primary ON wallets(is_primary);

		CREATE TABLE IF NOT EXISTS balances (
			wallet_address TEXT NOT NULL,
			mint TEXT NOT NULL,
			symbol TEXT,
			amount REAL NOT NULL,
			usd_price REAL NOT NULL,
			usd_value REAL NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (wallet_address, mint),
			FOREIGN KEY (wallet_address) REFERENCES wallets(address) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_balances_wallet ON balances(wallet_address);
		CREATE INDEX IF NOT EXISTS idx_balances_updated ON balances(updated_at);
	`

	errmsg: cstring
	result := sqlite3.exec(db.handle, cstring(raw_data(schema_sql)), nil, nil, &errmsg)
	if result != .Ok {
		log.errorf("Failed to create schema: %s", errmsg)
		sqlite3.free(cast(rawptr)errmsg)
		return .DatabaseError
	}

	log.info("Database schema created/verified")
	return .None
}

// insert_token inserts a token into the database
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate token has required fields
//
// Returns: Error status
insert_token :: proc(db: ^Database, token: Token) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(token.symbol) > 0, "Token symbol cannot be empty")
	assert(len(token.contract_address) > 0, "Token contract address cannot be empty")

	log.debugf("Inserting token: %s (%s)", token.symbol, token.contract_address)

	sql := `INSERT INTO tokens (symbol, name, contract_address, chain, is_quote_token,
	        usd_price, discovered_at, last_updated, cache_ttl)
	        VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare insert statement: %v", prep_result)
		return .DatabaseError
	}
	defer sqlite3.finalize(stmt)  // CRITICAL: Always finalize

	now := time.now()
	unix_timestamp := i64(now._nsec / 1_000_000_000)

	// CRITICAL: Parameters are 1-indexed!
	// CRITICAL: Strings must stay in scope until step() completes
	sqlite3.bind_text(stmt, 1, cstring(raw_data(token.symbol)), i32(len(token.symbol)), nil)
	sqlite3.bind_text(stmt, 2, cstring(raw_data(token.name)), i32(len(token.name)), nil)
	sqlite3.bind_text(stmt, 3, cstring(raw_data(token.contract_address)), i32(len(token.contract_address)), nil)
	sqlite3.bind_text(stmt, 4, cstring(raw_data(token.chain)), i32(len(token.chain)), nil)
	sqlite3.bind_int(stmt, 5, token.is_quote_token ? 1 : 0)
	sqlite3.bind_double(stmt, 6, token.usd_price)
	sqlite3.bind_int64(stmt, 7, unix_timestamp)
	sqlite3.bind_int64(stmt, 8, unix_timestamp)
	sqlite3.bind_int64(stmt, 9, 86400)  // cache_ttl = 24 hours

	step_result := sqlite3.step(stmt)
	if step_result != .Done {
		log.errorf("Failed to insert token: %v", step_result)
		errmsg := sqlite3.errmsg(db.handle)
		log.errorf("Error message: %s", errmsg)
		return .DatabaseError
	}

	log.infof("Inserted token %s", token.symbol)
	return .None
}

// insert_pool inserts a pool for a given token (by symbol lookup)
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate token_symbol is not empty
// ASSERTION 3: Validate pool has required fields
//
// Returns: Error status
insert_pool :: proc(db: ^Database, token_symbol: string, pool: PoolInfo) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(token_symbol) > 0, "Token symbol cannot be empty")
	assert(len(pool.dex) > 0, "Pool DEX cannot be empty")

	log.debugf("Inserting pool for token %s: %s/%s", token_symbol, pool.dex, pool.pool_address)

	// First get token_id
	get_id_sql := `SELECT id FROM tokens WHERE symbol = ?1 COLLATE NOCASE`
	id_stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(get_id_sql)), i32(len(get_id_sql)), &id_stmt, nil)
	if prep_result != .Ok {
		return .DatabaseError
	}
	defer sqlite3.finalize(id_stmt)

	sqlite3.bind_text(id_stmt, 1, cstring(raw_data(token_symbol)), i32(len(token_symbol)), nil)
	if sqlite3.step(id_stmt) != .Row {
		log.errorf("Token %s not found", token_symbol)
		return .DatabaseError
	}
	token_id := sqlite3.column_int64(id_stmt, 0)

	// Now insert pool
	sql := `INSERT INTO pools (token_id, dex, pool_address, quote_token, pool_type)
	        VALUES (?1, ?2, ?3, ?4, ?5)`

	stmt: ^sqlite3.Statement
	prep_result2 := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result2 != .Ok {
		log.errorf("Failed to prepare pool insert: %v", prep_result2)
		return .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	// CRITICAL: Parameters are 1-indexed
	sqlite3.bind_int64(stmt, 1, token_id)
	sqlite3.bind_text(stmt, 2, cstring(raw_data(pool.dex)), i32(len(pool.dex)), nil)
	sqlite3.bind_text(stmt, 3, cstring(raw_data(pool.pool_address)), i32(len(pool.pool_address)), nil)
	sqlite3.bind_text(stmt, 4, cstring(raw_data(pool.quote_token)), i32(len(pool.quote_token)), nil)
	sqlite3.bind_text(stmt, 5, cstring(raw_data(pool.pool_type)), i32(len(pool.pool_type)), nil)

	step_result := sqlite3.step(stmt)
	if step_result != .Done {
		log.errorf("Failed to insert pool: %v", step_result)
		return .DatabaseError
	}

	log.debugf("Inserted pool for token %s", token_symbol)
	return .None
}

// get_token_by_symbol retrieves a token by symbol (case-insensitive)
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate symbol is not empty
//
// Returns: Token, found flag, and error status
get_token_by_symbol :: proc(db: ^Database, symbol: string) -> (token: Token, found: bool, err: ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")

	log.debugf("Looking up token by symbol: %s", symbol)

	// COLLATE NOCASE makes the comparison case-insensitive
	sql := `SELECT id, symbol, name, contract_address, chain, is_quote_token, usd_price, cache_ttl
	        FROM tokens WHERE symbol = ?1 COLLATE NOCASE`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare select: %v", prep_result)
		return {}, false, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	sqlite3.bind_text(stmt, 1, cstring(raw_data(symbol)), i32(len(symbol)), nil)

	step_result := sqlite3.step(stmt)
	if step_result == .Row {
		// Extract token fields
		token_id := sqlite3.column_int64(stmt, 0)
		token.symbol = strings.clone(string(sqlite3.column_text(stmt, 1)))
		token.name = strings.clone(string(sqlite3.column_text(stmt, 2)))
		token.contract_address = strings.clone(string(sqlite3.column_text(stmt, 3)))
		token.chain = strings.clone(string(sqlite3.column_text(stmt, 4)))
		token.is_quote_token = sqlite3.column_int(stmt, 5) == 1
		token.usd_price = sqlite3.column_double(stmt, 6)

		// Fetch associated pools
		pools, pool_err := get_pools_for_token(db, token_id)
		if pool_err != .None {
			log.warnf("Failed to fetch pools for token %s", symbol)
			return {}, false, pool_err
		}
		token.pools = pools

		log.infof("Found token: %s (%s)", token.symbol, token.contract_address)
		return token, true, .None
	} else if step_result == .Done {
		log.debugf("Token not found: %s", symbol)
		return {}, false, .None
	} else {
		log.errorf("Query failed: %v", step_result)
		return {}, false, .DatabaseError
	}
}

// get_pools_for_token retrieves all pools for a given token ID
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate token_id is positive
//
// Returns: Array of pools and error status
get_pools_for_token :: proc(db: ^Database, token_id: i64) -> (pools: []PoolInfo, err: ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(token_id > 0, "Token ID must be positive")

	log.debugf("Fetching pools for token_id=%d", token_id)

	sql := `SELECT dex, pool_address, quote_token, pool_type FROM pools WHERE token_id = ?1`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare pools query: %v", prep_result)
		return nil, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	sqlite3.bind_int64(stmt, 1, token_id)

	// Collect all pools
	pool_list := make([dynamic]PoolInfo, 0, 4)

	for {
		step_result := sqlite3.step(stmt)
		if step_result == .Row {
			pool := PoolInfo{
				dex          = strings.clone(string(sqlite3.column_text(stmt, 0))),
				pool_address = strings.clone(string(sqlite3.column_text(stmt, 1))),
				quote_token  = strings.clone(string(sqlite3.column_text(stmt, 2))),
				pool_type    = strings.clone(string(sqlite3.column_text(stmt, 3))),
			}
			append(&pool_list, pool)
		} else if step_result == .Done {
			break
		} else {
			log.errorf("Failed to fetch pools: %v", step_result)
			delete(pool_list)
			return nil, .DatabaseError
		}
	}

	log.debugf("Found %d pool(s) for token_id=%d", len(pool_list), token_id)
	return pool_list[:], .None
}

// get_all_tokens retrieves all tokens from the database
//
// ASSERTION 1: Validate db is not nil
//
// Returns: Array of tokens and error status
get_all_tokens :: proc(db: ^Database) -> (tokens: []Token, err: ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	log.debug("Fetching all tokens")

	sql := `SELECT id, symbol, name, contract_address, chain, is_quote_token, usd_price
	        FROM tokens ORDER BY symbol`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare all tokens query: %v", prep_result)
		return nil, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	token_list := make([dynamic]Token, 0, 16)

	for {
		step_result := sqlite3.step(stmt)
		if step_result == .Row {
			token_id := sqlite3.column_int64(stmt, 0)
			token := Token{
				symbol           = strings.clone(string(sqlite3.column_text(stmt, 1))),
				name             = strings.clone(string(sqlite3.column_text(stmt, 2))),
				contract_address = strings.clone(string(sqlite3.column_text(stmt, 3))),
				chain            = strings.clone(string(sqlite3.column_text(stmt, 4))),
				is_quote_token   = sqlite3.column_int(stmt, 5) == 1,
				usd_price        = sqlite3.column_double(stmt, 6),
			}

			// Fetch pools
			pools, pool_err := get_pools_for_token(db, token_id)
			if pool_err != .None {
				log.warnf("Failed to fetch pools for token %s", token.symbol)
				continue
			}
			token.pools = pools

			append(&token_list, token)
		} else if step_result == .Done {
			break
		} else {
			log.errorf("Failed to fetch tokens: %v", step_result)
			delete(token_list)
			return nil, .DatabaseError
		}
	}

	log.infof("Fetched %d token(s)", len(token_list))
	return token_list[:], .None
}

// database_integrity_check performs SQLite integrity check
//
// ASSERTION 1: Validate db is not nil
//
// Returns: true if database is intact, false otherwise
database_integrity_check :: proc(db: ^Database) -> (ok: bool) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	log.debug("Running database integrity check")

	sql := "PRAGMA integrity_check"
	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.error("Failed to prepare integrity check")
		return false
	}
	defer sqlite3.finalize(stmt)

	step_result := sqlite3.step(stmt)
	if step_result == .Row {
		result_text := string(sqlite3.column_text(stmt, 0))
		if result_text == "ok" {
			log.info("Database integrity check: OK")
			return true
		} else {
			log.errorf("Database integrity check failed: %s", result_text)
			return false
		}
	}

	log.error("Integrity check returned no results")
	return false
}

// migrate_from_json migrates tokens from JSON config to database
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate config has tokens
//
// Steps:
// 1. Begin transaction
// 2. Insert each token
// 3. Insert pools for each token
// 4. Commit transaction
// 5. Backup original JSON file
//
// Returns: Error status
migrate_from_json :: proc(db: ^Database, config: TokenConfig) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(config.tokens) > 0, "Config must have tokens")

	log.infof("Starting migration of %d tokens from JSON to database", len(config.tokens))

	// Begin transaction
	errmsg: cstring
	begin_result := sqlite3.exec(db.handle, "BEGIN TRANSACTION", nil, nil, &errmsg)
	if begin_result != .Ok {
		log.errorf("Failed to begin transaction: %s", errmsg)
		sqlite3.free(cast(rawptr)errmsg)
		return .MigrationFailed
	}

	// Insert all tokens and their pools
	for token in config.tokens {
		insert_err := insert_token(db, token)
		if insert_err != .None {
			log.errorf("Failed to insert token %s, rolling back", token.symbol)
			sqlite3.exec(db.handle, "ROLLBACK", nil, nil, nil)
			return .MigrationFailed
		}

		// Insert pools (skip pools with empty addresses - not usable for on-chain pricing)
		for pool in token.pools {
			// Skip pools with empty pool_address or quote_token (e.g., Jupiter aggregator placeholder)
			if len(pool.pool_address) == 0 || len(pool.quote_token) == 0 {
				log.debugf("Skipping pool with empty address/quote_token for token %s (dex: %s)",
					token.symbol, pool.dex)
				continue
			}

			pool_err := insert_pool(db, token.symbol, pool)
			if pool_err != .None {
				log.errorf("Failed to insert pool for token %s, rolling back", token.symbol)
				sqlite3.exec(db.handle, "ROLLBACK", nil, nil, nil)
				return .MigrationFailed
			}
		}
	}

	// Commit transaction
	commit_result := sqlite3.exec(db.handle, "COMMIT", nil, nil, &errmsg)
	if commit_result != .Ok {
		log.errorf("Failed to commit transaction: %s", errmsg)
		sqlite3.free(cast(rawptr)errmsg)
		sqlite3.exec(db.handle, "ROLLBACK", nil, nil, nil)
		return .MigrationFailed
	}

	log.info("Migration completed successfully")

	// Backup original JSON file
	home, found := os.lookup_env("HOME")
	if found && len(home) > 0 {
		config_path := filepath.join({home, ".config", "hound", "tokens.json"})
		if os.exists(config_path) {
			now := time.now()
			unix_timestamp := now._nsec / 1_000_000_000
			backup_path := fmt.tprintf("%s.bak.%d", config_path, unix_timestamp)
			copy_ok := os.rename(config_path, backup_path)
			if copy_ok {
				log.infof("Backed up original config to: %s", backup_path)
			} else {
				log.warnf("Failed to backup original config file")
			}
		}
	}

	return .None
}

// get_default_db_path returns the default database path (~/.config/hound/tokens.db)
//
// Returns: Database path or empty string if home directory not found
get_default_db_path :: proc() -> string {
	home, found := os.lookup_env("HOME")
	if !found || len(home) == 0 {
		log.error("Could not determine home directory")
		return ""
	}

	db_path := filepath.join({home, ".config", "hound", "tokens.db"})
	return db_path
}

// =============================================================================
// WALLET DATABASE OPERATIONS - Phase 1
// =============================================================================

// insert_wallet adds a new wallet to the database
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate wallet address is not empty
// ASSERTION 3: Validate wallet label is not empty
//
// Returns: Error status
insert_wallet :: proc(db: ^Database, wallet: Wallet) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(wallet.address) > 0, "Wallet address cannot be empty")
	assert(len(wallet.label) > 0, "Wallet label cannot be empty")

	log.debugf("Inserting wallet: %s (%s)", wallet.label, wallet.address)

	sql := `INSERT INTO wallets (address, label, is_primary, added_at)
	        VALUES (?1, ?2, ?3, ?4)`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare wallet insert: %v", prep_result)
		return .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	now := time.now()
	unix_timestamp := i64(now._nsec / 1_000_000_000)

	sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet.address)), i32(len(wallet.address)), nil)
	sqlite3.bind_text(stmt, 2, cstring(raw_data(wallet.label)), i32(len(wallet.label)), nil)
	sqlite3.bind_int(stmt, 3, wallet.is_primary ? 1 : 0)
	sqlite3.bind_int64(stmt, 4, unix_timestamp)

	step_result := sqlite3.step(stmt)
	if step_result != .Done {
		log.errorf("Failed to insert wallet: %v", step_result)
		errmsg := sqlite3.errmsg(db.handle)
		log.errorf("Error message: %s", errmsg)
		return .DatabaseError
	}

	log.infof("Inserted wallet: %s", wallet.label)
	return .None
}

// get_all_wallets retrieves all wallets from the database
//
// ASSERTION 1: Validate db is not nil
//
// Returns: Array of wallets and error status
get_all_wallets :: proc(db: ^Database) -> (wallets: []Wallet, err: ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	log.debug("Fetching all wallets")

	sql := `SELECT address, label, is_primary FROM wallets ORDER BY is_primary DESC, added_at ASC`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare wallets query: %v", prep_result)
		return nil, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	wallet_list := make([dynamic]Wallet, 0, 4)

	for {
		step_result := sqlite3.step(stmt)
		if step_result == .Row {
			wallet := Wallet{
				address    = strings.clone(string(sqlite3.column_text(stmt, 0))),
				label      = strings.clone(string(sqlite3.column_text(stmt, 1))),
				is_primary = sqlite3.column_int(stmt, 2) == 1,
			}
			append(&wallet_list, wallet)
		} else if step_result == .Done {
			break
		} else {
			log.errorf("Failed to fetch wallets: %v", step_result)
			delete(wallet_list)
			return nil, .DatabaseError
		}
	}

	log.infof("Fetched %d wallet(s)", len(wallet_list))
	return wallet_list[:], .None
}

// update_balances updates or inserts balance records for a wallet
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate wallet_address is not empty
//
// Returns: Error status
update_balance :: proc(
	db: ^Database,
	wallet_address: string,
	mint: string,
	symbol: string,
	amount: f64,
	usd_price: f64,
	usd_value: f64,
) -> ErrorType {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(wallet_address) > 0, "Wallet address cannot be empty")
	assert(len(mint) > 0, "Mint address cannot be empty")

	log.debugf("Updating balance: %s for wallet %s", symbol, wallet_address)

	sql := `INSERT OR REPLACE INTO balances
	        (wallet_address, mint, symbol, amount, usd_price, usd_value, updated_at)
	        VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare balance update: %v", prep_result)
		return .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	now := time.now()
	unix_timestamp := i64(now._nsec / 1_000_000_000)

	sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet_address)), i32(len(wallet_address)), nil)
	sqlite3.bind_text(stmt, 2, cstring(raw_data(mint)), i32(len(mint)), nil)
	sqlite3.bind_text(stmt, 3, cstring(raw_data(symbol)), i32(len(symbol)), nil)
	sqlite3.bind_double(stmt, 4, amount)
	sqlite3.bind_double(stmt, 5, usd_price)
	sqlite3.bind_double(stmt, 6, usd_value)
	sqlite3.bind_int64(stmt, 7, unix_timestamp)

	step_result := sqlite3.step(stmt)
	if step_result != .Done {
		log.errorf("Failed to update balance: %v", step_result)
		return .DatabaseError
	}

	log.debugf("Updated balance for %s", symbol)
	return .None
}

// get_balances_for_wallet retrieves all balances for a given wallet
//
// ASSERTION 1: Validate db is not nil
// ASSERTION 2: Validate wallet_address is not empty
//
// Returns: Map of mint -> (symbol, amount, usd_value) and error status
get_balances_for_wallet :: proc(
	db: ^Database,
	wallet_address: string,
) -> (balances: map[string][3]f64, err: ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(len(wallet_address) > 0, "Wallet address cannot be empty")

	log.debugf("Fetching balances for wallet: %s", wallet_address)

	sql := `SELECT mint, symbol, amount, usd_price, usd_value
	        FROM balances WHERE wallet_address = ?1 ORDER BY usd_value DESC`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare balances query: %v", prep_result)
		return nil, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet_address)), i32(len(wallet_address)), nil)

	balance_map := make(map[string][3]f64)

	for {
		step_result := sqlite3.step(stmt)
		if step_result == .Row {
			mint := strings.clone(string(sqlite3.column_text(stmt, 0)))
			amount := sqlite3.column_double(stmt, 2)
			usd_price := sqlite3.column_double(stmt, 3)
			usd_value := sqlite3.column_double(stmt, 4)

			balance_map[mint] = {amount, usd_price, usd_value}
		} else if step_result == .Done {
			break
		} else {
			log.errorf("Failed to fetch balances: %v", step_result)
			delete(balance_map)
			return nil, .DatabaseError
		}
	}

	log.infof("Fetched %d balance(s) for wallet", len(balance_map))
	return balance_map, .None
}
