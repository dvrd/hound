package wallet

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
)

// testDB creates an in-memory database with schema for testing.
func testDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedWallets inserts test wallets into the database.
func seedWallets(t *testing.T, db *database.Database) []models.Wallet {
	t.Helper()
	wallets := []models.Wallet{
		{Address: "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF", Label: "Main", IsPrimary: true, WalletType: models.WalletTypeBIP44Standard, DerivationPath: "m/44'/501'/0'/0'"},
		{Address: "XXXX9999YYYY8888ZZZZ7777WWWW6666VVVV5555UUUU", Label: "Trading", IsPrimary: false, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy-sha256"},
	}
	for _, w := range wallets {
		if err := db.InsertWallet(w); err != nil {
			t.Fatalf("InsertWallet(%s): %v", w.Label, err)
		}
	}
	return wallets
}

func TestGetWallets(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	wallets, err := mgr.GetWallets()
	if err != nil {
		t.Fatalf("GetWallets: %v", err)
	}
	if len(wallets) != 2 {
		t.Fatalf("len = %d, want 2", len(wallets))
	}
	// Primary should be first
	if !wallets[0].IsPrimary {
		t.Error("first wallet should be primary")
	}
}

func TestGetPrimaryWallet(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	w, err := mgr.GetPrimaryWallet()
	if err != nil {
		t.Fatalf("GetPrimaryWallet: %v", err)
	}
	if w.Label != "Main" {
		t.Errorf("Label = %q, want %q", w.Label, "Main")
	}
	if !w.IsPrimary {
		t.Error("IsPrimary = false, want true")
	}
}

func TestGetPrimaryWalletNotFound(t *testing.T) {
	db := testDB(t)
	mgr := NewWalletManager(db, nil)

	_, err := mgr.GetPrimaryWallet()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestResolveWalletByAddress(t *testing.T) {
	db := testDB(t)
	wallets := seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	got, err := mgr.ResolveWallet(wallets[1].Address)
	if err != nil {
		t.Fatalf("ResolveWallet(address): %v", err)
	}
	if got.Address != wallets[1].Address {
		t.Errorf("Address = %q, want %q", got.Address, wallets[1].Address)
	}
}

func TestResolveWalletByPartialAddress(t *testing.T) {
	db := testDB(t)
	wallets := seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	// Use first 8 chars of second wallet
	partial := wallets[1].Address[:8]
	got, err := mgr.ResolveWallet(partial)
	if err != nil {
		t.Fatalf("ResolveWallet(partial): %v", err)
	}
	if got.Address != wallets[1].Address {
		t.Errorf("Address = %q, want %q", got.Address, wallets[1].Address)
	}
}

func TestResolveWalletByLabel(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	got, err := mgr.ResolveWallet("trading")
	if err != nil {
		t.Fatalf("ResolveWallet(label): %v", err)
	}
	if got.Label != "Trading" {
		t.Errorf("Label = %q, want %q", got.Label, "Trading")
	}
}

func TestResolveWalletEmpty(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	got, err := mgr.ResolveWallet("")
	if err != nil {
		t.Fatalf("ResolveWallet(empty): %v", err)
	}
	if !got.IsPrimary {
		t.Error("expected primary wallet for empty identifier")
	}
}

func TestResolveWalletNotFound(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	mgr := NewWalletManager(db, nil)
	_, err := mgr.ResolveWallet("nonexistent_wallet_xyz")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Errorf("error = %v, want ErrWalletNotFound", err)
	}
}

func TestAggregatePortfolios(t *testing.T) {
	db := testDB(t)
	mgr := NewWalletManager(db, nil)

	portfolios := map[string]models.PortfolioBalance{
		"wallet1": {
			WalletAddress: "wallet1",
			SOLBalance: models.TokenBalance{
				Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
				Amount: 10.0, USDPrice: 150.0, USDValue: 1500.0,
			},
			TokenBalances: []models.TokenBalance{
				{Mint: "USDC_MINT", Symbol: "USDC", Amount: 100.0, USDPrice: 1.0, USDValue: 100.0},
				{Mint: "BONK_MINT", Symbol: "BONK", Amount: 1000000.0, USDPrice: 0.00003, USDValue: 30.0},
			},
			TotalUSD: 1630.0,
		},
		"wallet2": {
			WalletAddress: "wallet2",
			SOLBalance: models.TokenBalance{
				Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
				Amount: 5.0, USDPrice: 150.0, USDValue: 750.0,
			},
			TokenBalances: []models.TokenBalance{
				{Mint: "USDC_MINT", Symbol: "USDC", Amount: 200.0, USDPrice: 1.0, USDValue: 200.0},
				{Mint: "RAY_MINT", Symbol: "RAY", Amount: 50.0, USDPrice: 2.0, USDValue: 100.0},
			},
			TotalUSD: 1050.0,
		},
	}

	agg := mgr.AggregatePortfolios(portfolios)

	if agg.WalletAddress != "AGGREGATED" {
		t.Errorf("WalletAddress = %q, want %q", agg.WalletAddress, "AGGREGATED")
	}

	// SOL should be summed
	if agg.SOLBalance.Amount != 15.0 {
		t.Errorf("SOL Amount = %v, want 15.0", agg.SOLBalance.Amount)
	}
	if agg.SOLBalance.USDValue != 2250.0 {
		t.Errorf("SOL USDValue = %v, want 2250.0", agg.SOLBalance.USDValue)
	}

	// Should have 3 unique tokens: USDC, BONK, RAY
	if len(agg.TokenBalances) != 3 {
		t.Fatalf("TokenBalances len = %d, want 3", len(agg.TokenBalances))
	}

	// Find USDC and verify it's merged
	var usdc *models.TokenBalance
	for i := range agg.TokenBalances {
		if agg.TokenBalances[i].Symbol == "USDC" {
			usdc = &agg.TokenBalances[i]
			break
		}
	}
	if usdc == nil {
		t.Fatal("USDC not found in aggregated balances")
	}
	if usdc.Amount != 300.0 {
		t.Errorf("USDC Amount = %v, want 300.0", usdc.Amount)
	}
	if usdc.USDValue != 300.0 {
		t.Errorf("USDC USDValue = %v, want 300.0", usdc.USDValue)
	}

	// Total USD should be sum of all
	expectedTotal := 2250.0 + 300.0 + 30.0 + 100.0
	if agg.TotalUSD != expectedTotal {
		t.Errorf("TotalUSD = %v, want %v", agg.TotalUSD, expectedTotal)
	}
}

func TestGetCachedPortfolioFromDB(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	// Insert some balances into the DB
	addr := "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF"
	if err := db.UpdateBalance(addr, "So11111111111111111111111111111111111111112", "SOL", 5.0, 150.0, 750.0); err != nil {
		t.Fatalf("UpdateBalance SOL: %v", err)
	}
	if err := db.UpdateBalance(addr, "USDC_MINT", "USDC", 100.0, 1.0, 100.0); err != nil {
		t.Fatalf("UpdateBalance USDC: %v", err)
	}

	mgr := NewWalletManager(db, nil)
	portfolio, err := mgr.GetCachedPortfolio(addr)
	if err != nil {
		t.Fatalf("GetCachedPortfolio: %v", err)
	}

	if portfolio.WalletAddress != addr {
		t.Errorf("WalletAddress = %q, want %q", portfolio.WalletAddress, addr)
	}
	if portfolio.SOLBalance.Amount != 5.0 {
		t.Errorf("SOL Amount = %v, want 5.0", portfolio.SOLBalance.Amount)
	}
	// USDC should be in token balances
	if len(portfolio.TokenBalances) != 1 {
		t.Fatalf("TokenBalances len = %d, want 1", len(portfolio.TokenBalances))
	}
	if portfolio.TokenBalances[0].Symbol != "USDC" {
		t.Errorf("token symbol = %q, want %q", portfolio.TokenBalances[0].Symbol, "USDC")
	}
}

func TestPersistPortfolio(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	addr := "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF"
	portfolio := models.PortfolioBalance{
		WalletAddress: addr,
		SOLBalance: models.TokenBalance{
			Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
			Amount: 3.5, USDPrice: 150.0, USDValue: 525.0,
		},
		TokenBalances: []models.TokenBalance{
			{Mint: "BONK_MINT", Symbol: "BONK", Amount: 500000.0, USDPrice: 0.00003, USDValue: 15.0},
		},
		TotalUSD: 540.0,
	}

	mgr := NewWalletManager(db, nil)
	if err := mgr.PersistPortfolio(portfolio); err != nil {
		t.Fatalf("PersistPortfolio: %v", err)
	}

	// Verify balances were saved
	balances, err := db.GetBalancesForWallet(addr)
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(balances) != 2 {
		t.Fatalf("balances len = %d, want 2", len(balances))
	}

	// Balances are ordered by usd_value DESC
	if balances[0].Symbol != "SOL" {
		t.Errorf("first balance symbol = %q, want %q", balances[0].Symbol, "SOL")
	}
	if balances[0].Amount != 3.5 {
		t.Errorf("SOL amount = %v, want 3.5", balances[0].Amount)
	}
	if balances[1].Symbol != "BONK" {
		t.Errorf("second balance symbol = %q, want %q", balances[1].Symbol, "BONK")
	}
}

func TestPersistPortfolioAtomic(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert wallet
	w := models.Wallet{Address: "persistTestAddr", Label: "persist-test", IsPrimary: true}
	if err := db.InsertWallet(w); err != nil {
		t.Fatalf("insert wallet: %v", err)
	}

	mgr := NewWalletManager(db, nil)

	portfolio := models.PortfolioBalance{
		WalletAddress: "persistTestAddr",
		SOLBalance: models.TokenBalance{
			Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
			Amount: 5.0, USDPrice: 150.0, USDValue: 750.0,
		},
		TokenBalances: []models.TokenBalance{
			{Mint: "USDCmint", Symbol: "USDC", Amount: 100.0, USDPrice: 1.0, USDValue: 100.0},
			{Mint: "BONKmint", Symbol: "BONK", Amount: 1000000.0, USDPrice: 0.00001, USDValue: 10.0},
		},
		TotalUSD: 860.0,
	}

	if err := mgr.PersistPortfolio(portfolio); err != nil {
		t.Fatalf("PersistPortfolio failed: %v", err)
	}

	// Verify all 3 balances were written
	balances, err := db.GetBalancesForWallet("persistTestAddr")
	if err != nil {
		t.Fatalf("GetBalancesForWallet: %v", err)
	}
	if len(balances) != 3 {
		t.Fatalf("expected 3 balances, got %d", len(balances))
	}
}

func TestCachedPortfolioReturnsCached(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	addr := "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF"
	mgr := NewWalletManager(db, nil)

	// Manually set cache
	cached := models.PortfolioBalance{
		WalletAddress: addr,
		SOLBalance: models.TokenBalance{
			Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
			Amount: 99.0, USDPrice: 200.0, USDValue: 19800.0,
		},
		TotalUSD: 19800.0,
	}
	mgr.mu.Lock()
	mgr.portfolioCache[addr] = cached
	mgr.mu.Unlock()

	got, err := mgr.GetCachedPortfolio(addr)
	if err != nil {
		t.Fatalf("GetCachedPortfolio: %v", err)
	}
	if got.SOLBalance.Amount != 99.0 {
		t.Errorf("SOL Amount = %v, want 99.0 (from cache)", got.SOLBalance.Amount)
	}
}
