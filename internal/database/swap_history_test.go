package database

import (
	"testing"
	"time"

	"github.com/dvrd/hound/internal/models"
)

func insertSwapTestWallet(t *testing.T, db *Database, addr string) {
	t.Helper()
	w := models.Wallet{
		Address:        addr,
		Label:          "Swap Test",
		WalletType:     models.WalletTypeLegacy,
		DerivationPath: "legacy-sha256",
	}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("InsertWallet(%s): %v", addr, err)
	}
}

func TestInsertAndGetSwapHistory(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertSwapTestWallet(t, db, "wallet1")

	entry := models.SwapHistoryEntry{
		WalletAddress: "wallet1",
		InputMint:     "So11111111111111111111111111111111",
		OutputMint:    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InputSymbol:   "SOL",
		OutputSymbol:  "USDC",
		InputAmount:   1.0,
		OutputAmount:  150.0,
		PriceImpact:   0.01,
		SlippageBps:   50,
		Signature:     "sig123",
		Status:        "confirmed",
		Dex:           "jupiter",
		NetworkFee:    0.000005,
		PriorityFee:   0.0001,
		CreatedAt:     time.Now().Unix(),
	}

	if err := db.InsertSwapHistory(entry); err != nil {
		t.Fatalf("InsertSwapHistory: %v", err)
	}

	got, err := db.GetSwapHistory("wallet1", 10)
	if err != nil {
		t.Fatalf("GetSwapHistory: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}

	g := got[0]
	if g.WalletAddress != "wallet1" {
		t.Errorf("WalletAddress = %q, want %q", g.WalletAddress, "wallet1")
	}
	if g.InputSymbol != "SOL" {
		t.Errorf("InputSymbol = %q, want %q", g.InputSymbol, "SOL")
	}
	if g.OutputSymbol != "USDC" {
		t.Errorf("OutputSymbol = %q, want %q", g.OutputSymbol, "USDC")
	}
	if g.InputAmount != 1.0 {
		t.Errorf("InputAmount = %f, want %f", g.InputAmount, 1.0)
	}
	if g.OutputAmount != 150.0 {
		t.Errorf("OutputAmount = %f, want %f", g.OutputAmount, 150.0)
	}
	if g.Signature != "sig123" {
		t.Errorf("Signature = %q, want %q", g.Signature, "sig123")
	}
	if g.Dex != "jupiter" {
		t.Errorf("Dex = %q, want %q", g.Dex, "jupiter")
	}
}

func TestGetSwapHistoryWithLimit(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertSwapTestWallet(t, db, "wallet1")

	now := time.Now().Unix()
	for i := range 5 {
		entry := models.SwapHistoryEntry{
			WalletAddress: "wallet1",
			InputMint:     "mint_in",
			OutputMint:    "mint_out",
			InputSymbol:   "SOL",
			OutputSymbol:  "USDC",
			InputAmount:   float64(i + 1),
			OutputAmount:  float64((i + 1) * 150),
			Status:        "confirmed",
			CreatedAt:     now + int64(i), // Increasing timestamps
		}
		if err := db.InsertSwapHistory(entry); err != nil {
			t.Fatalf("InsertSwapHistory[%d]: %v", i, err)
		}
	}

	got, err := db.GetSwapHistory("wallet1", 3)
	if err != nil {
		t.Fatalf("GetSwapHistory: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	// Should be ordered by created_at DESC (newest first)
	if got[0].InputAmount != 5.0 {
		t.Errorf("got[0].InputAmount = %f, want %f (newest)", got[0].InputAmount, 5.0)
	}
}

func TestGetSwapHistoryCount(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertSwapTestWallet(t, db, "wallet1")
	insertSwapTestWallet(t, db, "wallet2")

	entries := []models.SwapHistoryEntry{
		{WalletAddress: "wallet1", InputMint: "m1", OutputMint: "m2", InputAmount: 1, OutputAmount: 1, Status: "confirmed"},
		{WalletAddress: "wallet1", InputMint: "m1", OutputMint: "m2", InputAmount: 2, OutputAmount: 2, Status: "confirmed"},
		{WalletAddress: "wallet2", InputMint: "m1", OutputMint: "m2", InputAmount: 3, OutputAmount: 3, Status: "confirmed"},
	}
	for i, e := range entries {
		if err := db.InsertSwapHistory(e); err != nil {
			t.Fatalf("InsertSwapHistory[%d]: %v", i, err)
		}
	}

	// Count for wallet1
	count, err := db.GetSwapHistoryCount("wallet1")
	if err != nil {
		t.Fatalf("GetSwapHistoryCount(wallet1): %v", err)
	}
	if count != 2 {
		t.Errorf("count(wallet1) = %d, want 2", count)
	}

	// Count for wallet2
	count, err = db.GetSwapHistoryCount("wallet2")
	if err != nil {
		t.Fatalf("GetSwapHistoryCount(wallet2): %v", err)
	}
	if count != 1 {
		t.Errorf("count(wallet2) = %d, want 1", count)
	}

	// Total count
	count, err = db.GetSwapHistoryCount("")
	if err != nil {
		t.Fatalf("GetSwapHistoryCount(): %v", err)
	}
	if count != 3 {
		t.Errorf("count(all) = %d, want 3", count)
	}
}

func TestGetSwapHistoryEmptyWalletReturnsAll(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertSwapTestWallet(t, db, "wallet1")
	insertSwapTestWallet(t, db, "wallet2")

	entries := []models.SwapHistoryEntry{
		{WalletAddress: "wallet1", InputMint: "m1", OutputMint: "m2", InputAmount: 1, OutputAmount: 1, Status: "confirmed"},
		{WalletAddress: "wallet2", InputMint: "m1", OutputMint: "m2", InputAmount: 2, OutputAmount: 2, Status: "confirmed"},
	}
	for i, e := range entries {
		if err := db.InsertSwapHistory(e); err != nil {
			t.Fatalf("InsertSwapHistory[%d]: %v", i, err)
		}
	}

	// Empty wallet address should return all
	got, err := db.GetSwapHistory("", 100)
	if err != nil {
		t.Fatalf("GetSwapHistory: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestGetSwapHistoryFilterByWallet(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertSwapTestWallet(t, db, "wallet1")
	insertSwapTestWallet(t, db, "wallet2")

	entries := []models.SwapHistoryEntry{
		{WalletAddress: "wallet1", InputMint: "m1", OutputMint: "m2", InputAmount: 1, OutputAmount: 1, Status: "confirmed"},
		{WalletAddress: "wallet1", InputMint: "m1", OutputMint: "m2", InputAmount: 2, OutputAmount: 2, Status: "confirmed"},
		{WalletAddress: "wallet2", InputMint: "m1", OutputMint: "m2", InputAmount: 3, OutputAmount: 3, Status: "confirmed"},
	}
	for i, e := range entries {
		if err := db.InsertSwapHistory(e); err != nil {
			t.Fatalf("InsertSwapHistory[%d]: %v", i, err)
		}
	}

	got, err := db.GetSwapHistory("wallet1", 100)
	if err != nil {
		t.Fatalf("GetSwapHistory(wallet1): %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	for _, g := range got {
		if g.WalletAddress != "wallet1" {
			t.Errorf("WalletAddress = %q, want %q", g.WalletAddress, "wallet1")
		}
	}
}
