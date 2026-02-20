package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Database wraps a SQLite connection with schema management.
type Database struct {
	db   *sql.DB
	path string
}

// Open opens a SQLite database at the given path and configures PRAGMAs.
func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}

	if err := configurePragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure pragmas: %w", err)
	}

	return &Database{db: db, path: path}, nil
}

// OpenInMemory opens an in-memory SQLite database for testing.
func OpenInMemory() (*Database, error) {
	return Open(":memory:")
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB handle.
func (d *Database) DB() *sql.DB {
	return d.db
}

// CreateSchema creates all tables and indexes if they don't already exist.
func (d *Database) CreateSchema() error {
	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// MigrateSchema applies ALTER TABLE migrations for columns that may not exist
// in older database versions. It is safe to call multiple times.
func (d *Database) MigrateSchema() error {
	walletMigrations := []struct {
		column string
		ddl    string
	}{
		{"wallet_type", "ALTER TABLE wallets ADD COLUMN wallet_type TEXT DEFAULT 'Legacy'"},
		{"derivation_path", "ALTER TABLE wallets ADD COLUMN derivation_path TEXT DEFAULT 'legacy-sha256'"},
		{"account_index", "ALTER TABLE wallets ADD COLUMN account_index INTEGER DEFAULT 0"},
	}

	for _, m := range walletMigrations {
		if err := addColumnIfNotExists(d.db, "wallets", m.column, m.ddl); err != nil {
			return fmt.Errorf("migrate wallets.%s: %w", m.column, err)
		}
	}

	poolMigrations := []struct {
		column string
		ddl    string
	}{
		{"liquidity_usd", "ALTER TABLE pools ADD COLUMN liquidity_usd REAL DEFAULT 0.0"},
		{"volume_24h", "ALTER TABLE pools ADD COLUMN volume_24h REAL DEFAULT 0.0"},
		{"fee_percent", "ALTER TABLE pools ADD COLUMN fee_percent REAL DEFAULT 0.0"},
		{"discovered_at", "ALTER TABLE pools ADD COLUMN discovered_at INTEGER DEFAULT 0"},
	}

	for _, m := range poolMigrations {
		if err := addColumnIfNotExists(d.db, "pools", m.column, m.ddl); err != nil {
			return fmt.Errorf("migrate pools.%s: %w", m.column, err)
		}
	}

	// Migrate swap_history: Odin schema used "timestamp" column, Go uses "created_at".
	// Also drop the old unique index on signature and the timestamp-based index.
	if err := migrateSwapHistory(d.db); err != nil {
		return fmt.Errorf("migrate swap_history: %w", err)
	}

	return nil
}

// IntegrityCheck runs PRAGMA integrity_check and returns an error if the
// database is corrupt.
func (d *Database) IntegrityCheck() error {
	var result string
	if err := d.db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check query: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}

// configurePragmas sets performance and safety PRAGMAs on the connection.
func configurePragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("exec %q: %w", p, err)
		}
	}
	return nil
}

// migrateSwapHistory handles the Odin→Go schema transition for swap_history.
// The Odin schema used "timestamp" instead of "created_at" and had different
// nullability/uniqueness constraints. This migration renames the column if the
// old schema is detected. Safe to call on new databases (no-op).
func migrateSwapHistory(db *sql.DB) error {
	hasTimestamp, err := columnExists(db, "swap_history", "timestamp")
	if err != nil {
		// Table doesn't exist yet — nothing to migrate.
		return nil
	}
	if !hasTimestamp {
		return nil
	}

	// Odin schema has "timestamp" — rename to "created_at".
	if _, err := db.Exec(`ALTER TABLE swap_history RENAME COLUMN "timestamp" TO created_at`); err != nil {
		return fmt.Errorf("rename timestamp→created_at: %w", err)
	}

	// Drop the old timestamp-based index if it exists, the schema creation
	// will recreate it as idx_swap_history_created on created_at.
	db.Exec(`DROP INDEX IF EXISTS idx_swap_history_timestamp`)

	return nil
}

// addColumnIfNotExists checks whether a column exists in a table using
// PRAGMA table_info and adds it via ALTER TABLE if missing.
func addColumnIfNotExists(db *sql.DB, table, column, ddl string) error {
	exists, err := columnExists(db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("exec %q: %w", ddl, err)
	}
	return nil
}

// columnExists returns true if the given column exists in the table.
func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, fmt.Errorf("table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan table_info row: %w", err)
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// schema contains all CREATE TABLE and CREATE INDEX statements.
const schema = `
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
    liquidity_usd REAL DEFAULT 0.0,
    volume_24h REAL DEFAULT 0.0,
    fee_percent REAL DEFAULT 0.0,
    discovered_at INTEGER DEFAULT 0,
    UNIQUE(token_id, pool_address),
    FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pools_token ON pools(token_id);

CREATE TABLE IF NOT EXISTS wallets (
    address TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    is_primary INTEGER DEFAULT 0,
    added_at INTEGER NOT NULL,
    wallet_type TEXT DEFAULT 'Legacy',
    derivation_path TEXT DEFAULT 'legacy-sha256',
    account_index INTEGER DEFAULT 0
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

CREATE TABLE IF NOT EXISTS encrypted_keypairs (
    address TEXT PRIMARY KEY,
    encrypted_private_key BLOB NOT NULL,
    salt BLOB NOT NULL,
    nonce BLOB NOT NULL,
    tag BLOB NOT NULL,
    password_hash BLOB NOT NULL,
    label TEXT NOT NULL,
    is_primary INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used INTEGER
);

CREATE INDEX IF NOT EXISTS idx_encrypted_keypairs_primary ON encrypted_keypairs(is_primary);

CREATE TABLE IF NOT EXISTS hyperliquid_wallets (
    address TEXT PRIMARY KEY,
    label TEXT NOT NULL UNIQUE,
    api_wallet_name TEXT NOT NULL,
    encrypted_api_key BLOB NOT NULL,
    encrypted_api_secret BLOB NOT NULL,
    salt BLOB NOT NULL,
    nonce_key BLOB NOT NULL,
    nonce_secret BLOB NOT NULL,
    tag_key BLOB NOT NULL,
    tag_secret BLOB NOT NULL,
    password_hash BLOB NOT NULL,
    is_active INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used INTEGER
);

CREATE INDEX IF NOT EXISTS idx_hyperliquid_wallets_active ON hyperliquid_wallets(is_active);

CREATE TABLE IF NOT EXISTS swap_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    wallet_address TEXT NOT NULL,
    input_mint TEXT NOT NULL,
    output_mint TEXT NOT NULL,
    input_symbol TEXT,
    output_symbol TEXT,
    input_amount REAL NOT NULL,
    output_amount REAL NOT NULL,
    price_impact REAL DEFAULT 0.0,
    slippage_bps INTEGER DEFAULT 0,
    signature TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    dex TEXT,
    network_fee REAL DEFAULT 0.0,
    priority_fee REAL DEFAULT 0.0,
    error_message TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (wallet_address) REFERENCES wallets(address) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_swap_history_wallet ON swap_history(wallet_address);
CREATE INDEX IF NOT EXISTS idx_swap_history_created ON swap_history(created_at);
`
