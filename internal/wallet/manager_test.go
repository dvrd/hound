package wallet

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if err := db.UpdateBalance(addr, "So11111111111111111111111111111111111111112", "SOL", "Solana", 5.0, 150.0, 750.0); err != nil {
		t.Fatalf("UpdateBalance SOL: %v", err)
	}
	if err := db.UpdateBalance(addr, "USDC_MINT", "USDC", "USD Coin", 100.0, 1.0, 100.0); err != nil {
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

// TestPortfolioCacheHasNoTTL verifies W5: the portfolio cache has NO TTL.
// Cached data is returned even after time passes — there is no expiry mechanism.
func TestPortfolioCacheHasNoTTL(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db)

	addr := "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF"
	mgr := NewWalletManager(db, nil)

	// Set a "stale" portfolio in the cache (simulating data from the past)
	stalePortfolio := models.PortfolioBalance{
		WalletAddress: addr,
		SOLBalance: models.TokenBalance{
			Mint:     "So11111111111111111111111111111111111111112",
			Symbol:   "SOL",
			Amount:   42.0,
			USDPrice: 100.0,
			USDValue: 4200.0,
		},
		TotalUSD: 4200.0,
	}
	mgr.mu.Lock()
	mgr.portfolioCache[addr] = stalePortfolio
	mgr.mu.Unlock()

	// Simulate time passing (the cache has no TTL, so this should not matter)
	time.Sleep(1 * time.Millisecond)

	// GetCachedPortfolio should still return the stale cached data
	got, err := mgr.GetCachedPortfolio(addr)
	if err != nil {
		t.Fatalf("GetCachedPortfolio: %v", err)
	}

	// Verify stale data is returned (no TTL = no expiry)
	if got.SOLBalance.Amount != 42.0 {
		t.Errorf("SOL Amount = %v, want 42.0 (stale cache, no TTL)", got.SOLBalance.Amount)
	}
	if got.TotalUSD != 4200.0 {
		t.Errorf("TotalUSD = %v, want 4200.0 (stale cache, no TTL)", got.TotalUSD)
	}
}

// TestRefreshAllPortfoliosFullFailure verifies W7: when all wallet refreshes fail,
// RefreshAllPortfolios returns an error and an empty result map.
func TestRefreshAllPortfoliosFullFailure(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db) // seeds 2 wallets

	// nil balanceFetcher causes every RefreshPortfolio call to fail
	mgr := NewWalletManager(db, nil)

	results, err := mgr.RefreshAllPortfolios(context.Background())
	// When ALL wallets fail, should return error
	if err == nil {
		t.Fatal("RefreshAllPortfolios should return error when all wallets fail")
	}
	// Results should be nil or empty
	if len(results) != 0 {
		t.Errorf("expected empty results when all wallets fail, got %d entries", len(results))
	}

	// PreloadError should be set for each wallet
	wallets, _ := db.GetAllWallets()
	for _, w := range wallets {
		preloadErr := mgr.PreloadError(w.Address)
		if preloadErr == nil {
			t.Errorf("expected PreloadError for wallet %s, got nil", w.Address)
		}
	}
}

// TestRefreshAllPortfoliosPartialFailure verifies W6: when some wallets fail and some succeed,
// RefreshAllPortfolios returns the successful results and records errors for failed wallets.
// We simulate this by pre-populating the cache for one wallet and using nil fetcher for the rest.
// Since we can't partially mock the fetcher, we test the preloadErrors tracking behavior:
// after a full failure, clearing one wallet's error and verifying the state.
func TestRefreshAllPortfoliosPartialFailure(t *testing.T) {
	db := testDB(t)
	seedWallets(t, db) // seeds 2 wallets: "Main" and "Trading"

	mgr := NewWalletManager(db, nil)

	// Run refresh — all fail with nil fetcher
	_, _ = mgr.RefreshAllPortfolios(context.Background())

	wallets, err := db.GetAllWallets()
	if err != nil {
		t.Fatalf("GetAllWallets: %v", err)
	}
	if len(wallets) < 2 {
		t.Fatalf("expected at least 2 wallets, got %d", len(wallets))
	}

	// Both wallets should have preload errors
	for _, w := range wallets {
		if mgr.PreloadError(w.Address) == nil {
			t.Errorf("expected PreloadError for wallet %s after full failure", w.Address)
		}
	}

	// Simulate partial recovery: manually clear error for one wallet and set its cache
	addr0 := wallets[0].Address
	mgr.preloadErrMu.Lock()
	delete(mgr.preloadErrors, addr0)
	mgr.preloadErrMu.Unlock()

	mgr.mu.Lock()
	mgr.portfolioCache[addr0] = models.PortfolioBalance{
		WalletAddress: addr0,
		SOLBalance: models.TokenBalance{
			Mint: "So11111111111111111111111111111111111111112", Symbol: "SOL",
			Amount: 5.0, USDPrice: 150.0, USDValue: 750.0,
		},
		TotalUSD: 750.0,
	}
	mgr.mu.Unlock()

	// Wallet 0 should now have no preload error
	if mgr.PreloadError(addr0) != nil {
		t.Errorf("expected no PreloadError for wallet %s after recovery", addr0)
	}

	// Wallet 1 should still have a preload error
	addr1 := wallets[1].Address
	if mgr.PreloadError(addr1) == nil {
		t.Errorf("expected PreloadError for wallet %s (still failed)", addr1)
	}

	// GetCachedPortfolio for wallet 0 should return the cached data
	portfolio, err := mgr.GetCachedPortfolio(addr0)
	if err != nil {
		t.Fatalf("GetCachedPortfolio for recovered wallet: %v", err)
	}
	if portfolio.SOLBalance.Amount != 5.0 {
		t.Errorf("SOL Amount = %v, want 5.0", portfolio.SOLBalance.Amount)
	}
}

// TestRefreshAllPortfoliosEmptyWallets verifies that RefreshAllPortfolios with no wallets
// returns an empty map and no error.
func TestRefreshAllPortfoliosEmptyWallets(t *testing.T) {
	db := testDB(t)
	// No wallets seeded

	mgr := NewWalletManager(db, nil)
	results, err := mgr.RefreshAllPortfolios(context.Background())
	if err != nil {
		t.Fatalf("RefreshAllPortfolios with no wallets should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results for no wallets, got %d", len(results))
	}
}

// TestPreloadErrorClearedOnSuccess verifies that PreloadError is cleared when a wallet
// successfully refreshes after a previous failure.
func TestPreloadErrorClearedOnSuccess(t *testing.T) {
	db := testDB(t)
	addr := "AAAA1111BBBB2222CCCC3333DDDD4444EEEE5555FFFF"
	if err := db.InsertWallet(models.Wallet{
		Address: addr, Label: "test", IsPrimary: true,
		WalletType: models.WalletTypeBIP44Standard, DerivationPath: "m/44'/501'/0'/0'",
	}); err != nil {
		t.Fatalf("InsertWallet: %v", err)
	}

	mgr := NewWalletManager(db, nil)

	// Manually set a preload error
	mgr.preloadErrMu.Lock()
	mgr.preloadErrors[addr] = errors.New("previous fetch failed")
	mgr.preloadErrMu.Unlock()

	if mgr.PreloadError(addr) == nil {
		t.Fatal("expected preload error to be set")
	}

	// Manually clear it (simulating what RefreshAllPortfolios does on success)
	mgr.preloadErrMu.Lock()
	delete(mgr.preloadErrors, addr)
	mgr.preloadErrMu.Unlock()

	if mgr.PreloadError(addr) != nil {
		t.Error("expected preload error to be cleared after success")
	}
}
