package database

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestInsertAndGetTokenBySymbol(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	token := models.Token{
		Symbol:          "SOL",
		Name:            "Solana",
		ContractAddress: "So11111111111111111111111111111111",
		Chain:           "solana",
		IsQuoteToken:    false,
		USDPrice:        150.0,
	}

	if err := db.InsertToken(token); err != nil {
		t.Fatalf("InsertToken: %v", err)
	}

	got, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}

	if got.Symbol != "SOL" {
		t.Errorf("Symbol = %q, want %q", got.Symbol, "SOL")
	}
	if got.Name != "Solana" {
		t.Errorf("Name = %q, want %q", got.Name, "Solana")
	}
	if got.ContractAddress != "So11111111111111111111111111111111" {
		t.Errorf("ContractAddress = %q, want %q", got.ContractAddress, "So11111111111111111111111111111111")
	}
	if got.Chain != "solana" {
		t.Errorf("Chain = %q, want %q", got.Chain, "solana")
	}
	if got.USDPrice != 150.0 {
		t.Errorf("USDPrice = %f, want %f", got.USDPrice, 150.0)
	}
	if got.IsQuoteToken {
		t.Error("IsQuoteToken = true, want false")
	}
}

func TestGetTokenBySymbolCaseInsensitive(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	token := models.Token{
		Symbol:          "BONK",
		Name:            "Bonk",
		ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
		Chain:           "solana",
		USDPrice:        0.000028,
	}
	if err := db.InsertToken(token); err != nil {
		t.Fatalf("InsertToken: %v", err)
	}

	cases := []string{"BONK", "bonk", "Bonk", "bOnK"}
	for _, sym := range cases {
		t.Run(sym, func(t *testing.T) {
			got, err := db.GetTokenBySymbol(sym)
			if err != nil {
				t.Fatalf("GetTokenBySymbol(%q): %v", sym, err)
			}
			if got.Symbol != "BONK" {
				t.Errorf("Symbol = %q, want %q", got.Symbol, "BONK")
			}
		})
	}
}

func TestGetTokenByContractAddress(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	token := models.Token{
		Symbol:          "USDC",
		Name:            "USD Coin",
		ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		Chain:           "solana",
		IsQuoteToken:    true,
		USDPrice:        1.0,
	}
	if err := db.InsertToken(token); err != nil {
		t.Fatalf("InsertToken: %v", err)
	}

	got, err := db.GetTokenByContractAddress("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	if err != nil {
		t.Fatalf("GetTokenByContractAddress: %v", err)
	}

	if got.Symbol != "USDC" {
		t.Errorf("Symbol = %q, want %q", got.Symbol, "USDC")
	}
	if !got.IsQuoteToken {
		t.Error("IsQuoteToken = false, want true")
	}
}

func TestGetAllTokens(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	tokens := []models.Token{
		{Symbol: "SOL", Name: "Solana", ContractAddress: "addr1", Chain: "solana", USDPrice: 150.0},
		{Symbol: "BONK", Name: "Bonk", ContractAddress: "addr2", Chain: "solana", USDPrice: 0.000028},
		{Symbol: "USDC", Name: "USD Coin", ContractAddress: "addr3", Chain: "solana", USDPrice: 1.0},
	}
	for _, tok := range tokens {
		if err := db.InsertToken(tok); err != nil {
			t.Fatalf("InsertToken(%s): %v", tok.Symbol, err)
		}
	}

	got, err := db.GetAllTokens()
	if err != nil {
		t.Fatalf("GetAllTokens: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	// Should be ordered by symbol: BONK, SOL, USDC
	expected := []string{"BONK", "SOL", "USDC"}
	for i, want := range expected {
		if got[i].Symbol != want {
			t.Errorf("got[%d].Symbol = %q, want %q", i, got[i].Symbol, want)
		}
	}
}

func TestUpdateTokenPrice(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

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

	if err := db.UpdateTokenPrice("SOL", 175.50); err != nil {
		t.Fatalf("UpdateTokenPrice: %v", err)
	}

	got, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}
	if got.USDPrice != 175.50 {
		t.Errorf("USDPrice = %f, want %f", got.USDPrice, 175.50)
	}
}

func TestUpdateTokenPriceCaseInsensitive(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

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

	if err := db.UpdateTokenPrice("sol", 200.0); err != nil {
		t.Fatalf("UpdateTokenPrice(sol): %v", err)
	}

	got, err := db.GetTokenBySymbol("SOL")
	if err != nil {
		t.Fatalf("GetTokenBySymbol: %v", err)
	}
	if got.USDPrice != 200.0 {
		t.Errorf("USDPrice = %f, want %f", got.USDPrice, 200.0)
	}
}

func TestGetTokenBySymbolNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetTokenBySymbol("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("error = %v, want ErrTokenNotFound", err)
	}
}

func TestUpdateTokenPriceNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.UpdateTokenPrice("NONEXISTENT", 100.0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrTokenNotFound) {
		t.Errorf("error = %v, want ErrTokenNotFound", err)
	}
}
