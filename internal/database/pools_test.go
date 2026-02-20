package database

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func insertTestToken(t *testing.T, db *Database) {
	t.Helper()
	token := models.Token{
		Symbol:          "SOL",
		Name:            "Solana",
		ContractAddress: "So11111111111111111111111111111111",
		Chain:           "solana",
		USDPrice:        150.0,
	}
	if err := db.InsertToken(token); err != nil {
		t.Fatalf("InsertToken: %v", err)
	}
}

func TestInsertAndGetPool(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestToken(t, db)

	pool := models.PoolInfo{
		Dex:          "orca",
		PoolAddress:  "pool_addr_1",
		QuoteToken:   "USDC",
		PoolType:     "whirlpool",
		LiquidityUSD: 50000.0,
		Volume24h:    10000.0,
		FeePercent:   0.3,
		DiscoveredAt: 1700000000,
	}

	if err := db.InsertPool("SOL", pool); err != nil {
		t.Fatalf("InsertPool: %v", err)
	}

	// Get token to verify pools are loaded
	token, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}

	if len(token.Pools) != 1 {
		t.Fatalf("len(Pools) = %d, want 1", len(token.Pools))
	}

	got := token.Pools[0]
	if got.Dex != "orca" {
		t.Errorf("Dex = %q, want %q", got.Dex, "orca")
	}
	if got.PoolAddress != "pool_addr_1" {
		t.Errorf("PoolAddress = %q, want %q", got.PoolAddress, "pool_addr_1")
	}
	if got.QuoteToken != "USDC" {
		t.Errorf("QuoteToken = %q, want %q", got.QuoteToken, "USDC")
	}
	if got.LiquidityUSD != 50000.0 {
		t.Errorf("LiquidityUSD = %f, want %f", got.LiquidityUSD, 50000.0)
	}
}

func TestInsertPoolTokenNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	pool := models.PoolInfo{
		Dex:         "orca",
		PoolAddress: "pool_addr_1",
		QuoteToken:  "USDC",
		PoolType:    "whirlpool",
	}

	err := db.InsertPool("NONEXISTENT", pool)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("error = %v, want ErrTokenNotFound", err)
	}
}

func TestDeletePoolsForToken(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestToken(t, db)

	pools := []models.PoolInfo{
		{Dex: "orca", PoolAddress: "pool1", QuoteToken: "USDC", PoolType: "whirlpool", LiquidityUSD: 10000},
		{Dex: "raydium", PoolAddress: "pool2", QuoteToken: "SOL", PoolType: "amm_v4", LiquidityUSD: 20000},
	}
	for _, p := range pools {
		if err := db.InsertPool("SOL", p); err != nil {
			t.Fatalf("InsertPool: %v", err)
		}
	}

	if err := db.DeletePoolsForToken("SOL"); err != nil {
		t.Fatalf("DeletePoolsForToken: %v", err)
	}

	token, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}
	if len(token.Pools) != 0 {
		t.Errorf("len(Pools) = %d, want 0", len(token.Pools))
	}
}

func TestGetPoolStats(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestToken(t, db)

	pools := []models.PoolInfo{
		{Dex: "orca", PoolAddress: "pool1", QuoteToken: "USDC", PoolType: "whirlpool", LiquidityUSD: 10000},
		{Dex: "raydium", PoolAddress: "pool2", QuoteToken: "SOL", PoolType: "amm_v4", LiquidityUSD: 25000},
		{Dex: "orca", PoolAddress: "pool3", QuoteToken: "USDT", PoolType: "whirlpool", LiquidityUSD: 15000},
	}
	for _, p := range pools {
		if err := db.InsertPool("SOL", p); err != nil {
			t.Fatalf("InsertPool: %v", err)
		}
	}

	stats, err := db.GetPoolStats("SOL")
	if err != nil {
		t.Fatalf("GetPoolStats: %v", err)
	}

	if stats.PoolCount != 3 {
		t.Errorf("PoolCount = %d, want 3", stats.PoolCount)
	}
	if stats.TotalLiquidity != 50000.0 {
		t.Errorf("TotalLiquidity = %f, want %f", stats.TotalLiquidity, 50000.0)
	}
}

func TestGetPoolStatsTokenNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetPoolStats("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("error = %v, want ErrTokenNotFound", err)
	}
}

func TestInsertPoolCaseInsensitive(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}
	insertTestToken(t, db)

	pool := models.PoolInfo{
		Dex:         "orca",
		PoolAddress: "pool_addr_1",
		QuoteToken:  "USDC",
		PoolType:    "whirlpool",
	}

	// Insert using lowercase symbol
	if err := db.InsertPool("sol", pool); err != nil {
		t.Fatalf("InsertPool(sol): %v", err)
	}

	token, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}
	if len(token.Pools) != 1 {
		t.Errorf("len(Pools) = %d, want 1", len(token.Pools))
	}
}
