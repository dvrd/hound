package database

import (
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func insertTestWallet(t *testing.T, db *Database, addr, label string) {
	t.Helper()
	w := models.Wallet{
		Address:        addr,
		Label:          label,
		WalletType:     models.WalletTypeLegacy,
		DerivationPath: "legacy-sha256",
	}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("InsertWallet(%s): %v", addr, err)
	}
}

func TestUpdateAndGetBalances(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestWallet(t, db, "wallet1", "Test Wallet")

	// Insert multiple balances
	balances := []struct {
		mint, symbol         string
		amount, price, value float64
	}{
		{"mint_sol", "SOL", 10.0, 150.0, 1500.0},
		{"mint_usdc", "USDC", 500.0, 1.0, 500.0},
		{"mint_bonk", "BONK", 1000000.0, 0.000028, 28.0},
	}
	for _, b := range balances {
		if err := db.UpdateBalance("wallet1", b.mint, b.symbol, b.amount, b.price, b.value); err != nil {
			t.Fatalf("UpdateBalance(%s): %v", b.symbol, err)
		}
	}

	got, err := db.GetBalancesForWallet("wallet1")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	// Should be ordered by usd_value DESC: SOL (1500), USDC (500), BONK (28)
	if got[0].Symbol != "SOL" {
		t.Errorf("got[0].Symbol = %q, want %q", got[0].Symbol, "SOL")
	}
	if got[0].USDValue != 1500.0 {
		t.Errorf("got[0].USDValue = %f, want %f", got[0].USDValue, 1500.0)
	}
	if got[1].Symbol != "USDC" {
		t.Errorf("got[1].Symbol = %q, want %q", got[1].Symbol, "USDC")
	}
	if got[2].Symbol != "BONK" {
		t.Errorf("got[2].Symbol = %q, want %q", got[2].Symbol, "BONK")
	}
}

func TestUpdateBalanceUpsert(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestWallet(t, db, "wallet1", "Test Wallet")

	// Insert initial balance
	if err := db.UpdateBalance("wallet1", "mint_sol", "SOL", 10.0, 150.0, 1500.0); err != nil {
		t.Fatalf("UpdateBalance (insert): %v", err)
	}

	// Update same balance
	if err := db.UpdateBalance("wallet1", "mint_sol", "SOL", 20.0, 175.0, 3500.0); err != nil {
		t.Fatalf("UpdateBalance (update): %v", err)
	}

	got, err := db.GetBalancesForWallet("wallet1")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (upsert should not duplicate)", len(got))
	}
	if got[0].Amount != 20.0 {
		t.Errorf("Amount = %f, want %f", got[0].Amount, 20.0)
	}
	if got[0].USDPrice != 175.0 {
		t.Errorf("USDPrice = %f, want %f", got[0].USDPrice, 175.0)
	}
	if got[0].USDValue != 3500.0 {
		t.Errorf("USDValue = %f, want %f", got[0].USDValue, 3500.0)
	}
}

func TestGetBalancesForWalletEmpty(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestWallet(t, db, "wallet1", "Test Wallet")

	got, err := db.GetBalancesForWallet("wallet1")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestUpdateBalanceTx(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert a wallet first (foreign key)
	insertTestWallet(t, db, "txTestAddr", "tx-test")

	// Test commit path
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := db.UpdateBalanceTx(tx, "txTestAddr", "SOLmint", "SOL", 1.5, 150.0, 225.0); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateBalanceTx: %v", err)
	}
	if err := db.UpdateBalanceTx(tx, "txTestAddr", "USDCmint", "USDC", 100.0, 1.0, 100.0); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateBalanceTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify both balances persisted
	balances, err := db.GetBalancesForWallet("txTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}

	// Test rollback path
	tx2, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx (2): %v", err)
	}
	if err := db.UpdateBalanceTx(tx2, "txTestAddr", "BONKmint", "BONK", 999.0, 0.001, 0.999); err != nil {
		tx2.Rollback()
		t.Fatalf("UpdateBalanceTx (rollback): %v", err)
	}
	tx2.Rollback()

	// Verify BONK was NOT persisted
	balances2, err := db.GetBalancesForWallet("txTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet (2): %v", err)
	}
	if len(balances2) != 2 {
		t.Fatalf("expected 2 balances after rollback, got %d", len(balances2))
	}
}
