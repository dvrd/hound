package database

import (
	"database/sql"
	"testing"
	"time"
)

func mustOpenInMemory(t *testing.T) *Database {
	t.Helper()
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenInMemory(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer db.Close()

	if db.DB() == nil {
		t.Fatal("expected non-nil *sql.DB")
	}

	// Verify we can ping
	if err := db.DB().Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestPragmasAreSet(t *testing.T) {
	db := mustOpenInMemory(t)

	tests := []struct {
		pragma   string
		expected string
	}{
		{"foreign_keys", "1"},
		{"journal_mode", "memory"}, // in-memory DBs use "memory" not "wal"
		{"synchronous", "1"},       // NORMAL = 1
		{"busy_timeout", "5000"},
	}

	for _, tc := range tests {
		t.Run(tc.pragma, func(t *testing.T) {
			var val string
			err := db.DB().QueryRow("PRAGMA " + tc.pragma).Scan(&val)
			if err != nil {
				t.Fatalf("PRAGMA %s query failed: %v", tc.pragma, err)
			}
			if val != tc.expected {
				t.Errorf("PRAGMA %s = %q, want %q", tc.pragma, val, tc.expected)
			}
		})
	}
}

func TestCreateSchemaIdempotent(t *testing.T) {
	db := mustOpenInMemory(t)

	// First call should succeed
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("first CreateSchema failed: %v", err)
	}

	// Second call should also succeed (IF NOT EXISTS)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("second CreateSchema failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{
		"tokens", "pools", "wallets", "balances",
		"encrypted_keypairs", "hyperliquid_wallets", "swap_history",
	}
	for _, table := range tables {
		var name string
		err := db.DB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestMigrateSchemaIdempotent(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// First migration
	if err := db.MigrateSchema(); err != nil {
		t.Fatalf("first MigrateSchema failed: %v", err)
	}

	// Second migration should be a no-op
	if err := db.MigrateSchema(); err != nil {
		t.Fatalf("second MigrateSchema failed: %v", err)
	}
}

func TestIntegrityCheck(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	if err := db.IntegrityCheck(); err != nil {
		t.Fatalf("IntegrityCheck failed: %v", err)
	}
}

func TestForeignKeyEnforcement(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// Insert a balance without a corresponding wallet - should fail
	now := time.Now().Unix()
	_, err := db.DB().Exec(
		`INSERT INTO balances (wallet_address, mint, symbol, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"nonexistent_wallet", "So11111111111111111111111111111111", "SOL",
		1.5, 150.0, 225.0, now,
	)
	if err == nil {
		t.Fatal("expected foreign key violation, got nil error")
	}
}

func TestForeignKeyCascadeDelete(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	now := time.Now().Unix()

	// Insert a wallet
	_, err := db.DB().Exec(
		`INSERT INTO wallets (address, label, is_primary, added_at) VALUES (?, ?, ?, ?)`,
		"wallet1", "Test Wallet", 1, now,
	)
	if err != nil {
		t.Fatalf("insert wallet failed: %v", err)
	}

	// Insert a balance for that wallet
	_, err = db.DB().Exec(
		`INSERT INTO balances (wallet_address, mint, symbol, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wallet1", "So11111111111111111111111111111111", "SOL",
		1.5, 150.0, 225.0, now,
	)
	if err != nil {
		t.Fatalf("insert balance failed: %v", err)
	}

	// Delete the wallet - balance should cascade
	_, err = db.DB().Exec(`DELETE FROM wallets WHERE address = ?`, "wallet1")
	if err != nil {
		t.Fatalf("delete wallet failed: %v", err)
	}

	// Verify balance is gone
	var count int
	err = db.DB().QueryRow(`SELECT COUNT(*) FROM balances WHERE wallet_address = ?`, "wallet1").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 balances after cascade delete, got %d", count)
	}
}

func TestClose(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// After close, operations should fail
	err = db.DB().Ping()
	if err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

func TestDBAccessor(t *testing.T) {
	db := mustOpenInMemory(t)

	sqlDB := db.DB()
	if sqlDB == nil {
		t.Fatal("DB() returned nil")
	}

	// Verify it's a working *sql.DB
	var result int
	err := sqlDB.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("query via DB() failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

func TestTokenInsertAndQuery(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	now := time.Now().Unix()

	_, err := db.DB().Exec(
		`INSERT INTO tokens (symbol, name, contract_address, chain, is_quote_token, usd_price, discovered_at, last_updated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"SOL", "Solana", "So11111111111111111111111111111111", "solana", 0, 150.0, now, now,
	)
	if err != nil {
		t.Fatalf("insert token failed: %v", err)
	}

	// Case-insensitive lookup
	var symbol string
	err = db.DB().QueryRow(
		`SELECT symbol FROM tokens WHERE symbol = ? COLLATE NOCASE`, "sol",
	).Scan(&symbol)
	if err != nil {
		t.Fatalf("case-insensitive query failed: %v", err)
	}
	if symbol != "SOL" {
		t.Errorf("expected SOL, got %s", symbol)
	}
}

func TestPoolForeignKey(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// Insert pool without token - should fail
	_, err := db.DB().Exec(
		`INSERT INTO pools (token_id, dex, pool_address, quote_token, pool_type)
		 VALUES (?, ?, ?, ?, ?)`,
		999, "orca", "pool_addr_1", "USDC", "whirlpool",
	)
	if err == nil {
		t.Fatal("expected foreign key violation for pool insert, got nil")
	}
}

func TestSwapHistoryInsert(t *testing.T) {
	db := mustOpenInMemory(t)

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	now := time.Now().Unix()

	// Insert wallet first
	_, err := db.DB().Exec(
		`INSERT INTO wallets (address, label, is_primary, added_at) VALUES (?, ?, ?, ?)`,
		"wallet1", "Test", 1, now,
	)
	if err != nil {
		t.Fatalf("insert wallet failed: %v", err)
	}

	// Insert swap history
	res, err := db.DB().Exec(
		`INSERT INTO swap_history (wallet_address, input_mint, output_mint, input_symbol, output_symbol,
		 input_amount, output_amount, price_impact, slippage_bps, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"wallet1", "So11111111111111111111111111111111", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"SOL", "USDC", 1.0, 150.0, 0.01, 50, "confirmed", now,
	)
	if err != nil {
		t.Fatalf("insert swap_history failed: %v", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId failed: %v", err)
	}
	if id < 1 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestMigrateAddsColumns(t *testing.T) {
	db := mustOpenInMemory(t)

	// Create a minimal schema WITHOUT the migration columns to simulate old DB
	_, err := db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS wallets (
			address TEXT PRIMARY KEY,
			label TEXT NOT NULL,
			is_primary INTEGER DEFAULT 0,
			added_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS pools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id INTEGER NOT NULL,
			dex TEXT NOT NULL,
			pool_address TEXT NOT NULL,
			quote_token TEXT NOT NULL,
			pool_type TEXT NOT NULL,
			UNIQUE(token_id, pool_address)
		);
	`)
	if err != nil {
		t.Fatalf("create minimal schema failed: %v", err)
	}

	// Run migration
	if err := db.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema failed: %v", err)
	}

	// Verify wallet columns were added
	walletCols := getColumnNames(t, db.DB(), "wallets")
	for _, col := range []string{"wallet_type", "derivation_path", "account_index"} {
		if !contains(walletCols, col) {
			t.Errorf("wallets table missing column %q after migration", col)
		}
	}

	// Verify pool columns were added
	poolCols := getColumnNames(t, db.DB(), "pools")
	for _, col := range []string{"liquidity_usd", "volume_24h", "fee_percent", "discovered_at"} {
		if !contains(poolCols, col) {
			t.Errorf("pools table missing column %q after migration", col)
		}
	}
}

// helpers

func getColumnNames(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) failed: %v", table, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan table_info row failed: %v", err)
		}
		cols = append(cols, name)
	}
	return cols
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
