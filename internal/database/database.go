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

	// Fix H4: Force single connection so PRAGMAs apply to all queries.
	// SQLite is single-writer anyway; pooling only causes PRAGMA drift.
	db.SetMaxOpenConns(1)

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

// Migrate runs ALTER TABLE migrations for schema evolution.
// Safe to call multiple times — uses IF NOT EXISTS / ignores "duplicate column" errors.
func (d *Database) Migrate() error {
	migrations := []string{
		// C1: Add verifier_salt for dual-salt derivation
		`ALTER TABLE encrypted_keypairs ADD COLUMN verifier_salt BLOB`,
		`ALTER TABLE hyperliquid_wallets ADD COLUMN verifier_salt BLOB`,
		// H7: Track Argon2 parameter version
		`ALTER TABLE encrypted_keypairs ADD COLUMN argon2_version INTEGER DEFAULT 1`,
		`ALTER TABLE hyperliquid_wallets ADD COLUMN argon2_version INTEGER DEFAULT 1`,
		// Token name display: add name column to balances
		`ALTER TABLE balances ADD COLUMN name TEXT`,
	}

	for _, m := range migrations {
		_, err := d.db.Exec(m)
		if err != nil {
			// SQLite returns "duplicate column name" if column already exists.
			// This is expected on subsequent runs — ignore it.
			if isDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("migration %q: %w", m, err)
		}
	}
	return nil
}

// isDuplicateColumnError checks if the error is a "duplicate column" from ALTER TABLE.
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column")
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

// BeginTx starts a new database transaction.
// The caller must call tx.Commit() or tx.Rollback() when done.
func (d *Database) BeginTx() (*sql.Tx, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return tx, nil
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
    name TEXT,
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
    verifier_salt BLOB,
    argon2_version INTEGER DEFAULT 1,
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
    verifier_salt BLOB,
    argon2_version INTEGER DEFAULT 1,
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

CREATE TABLE IF NOT EXISTS hidden_tokens (
    wallet_address TEXT NOT NULL,
    mint           TEXT NOT NULL,
    hidden_at      INTEGER NOT NULL,
    PRIMARY KEY (wallet_address, mint),
    FOREIGN KEY (wallet_address) REFERENCES wallets(address) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_hidden_tokens_wallet ON hidden_tokens(wallet_address);

CREATE TABLE IF NOT EXISTS app_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`
