package services_test

import (
	"testing"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

func TestTokenCatalog_LookupMetadata_FromDB(t *testing.T) {
	db := setupTestDB(t)

	// Insert a token into the database.
	err := db.InsertToken(models.Token{
		Symbol:          "BONK",
		Name:            "Bonk",
		ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
		Chain:           "solana",
		Decimals:        5,
	})
	if err != nil {
		t.Fatalf("InsertToken: %v", err)
	}

	catalog := services.NewTokenCatalog(nil, nil, db)

	result, err := catalog.LookupMetadata("DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263")
	if err != nil {
		t.Fatalf("LookupMetadata: %v", err)
	}

	if result.Symbol != "BONK" {
		t.Errorf("Symbol = %q, want BONK", result.Symbol)
	}
	if result.Name != "Bonk" {
		t.Errorf("Name = %q, want Bonk", result.Name)
	}
	if !result.Saved {
		t.Error("expected Saved = true for DB token")
	}
	// Note: the tokens table does not store decimals, so GetTokenDecimals falls back
	// to the hardcoded default (9 for unknown symbols). This is expected.
	if result.Decimals != 9 {
		t.Errorf("Decimals = %d, want 9 (default fallback)", result.Decimals)
	}
}

func TestTokenCatalog_LookupMetadata_NotFound(t *testing.T) {
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	db.CreateSchema()
	defer db.Close()

	// No Jupiter client, no DB entry → should fail
	catalog := services.NewTokenCatalog(nil, nil, db)

	_, err = catalog.LookupMetadata("nonexistent_mint")
	if err == nil {
		t.Fatal("expected error for unknown token with no fallback")
	}
}

func TestTokenCatalog_Search_NilJupiter(t *testing.T) {
	catalog := services.NewTokenCatalog(nil, nil, nil)

	_, err := catalog.Search("SOL")
	if err == nil {
		t.Fatal("expected error when Jupiter client is nil")
	}
}
