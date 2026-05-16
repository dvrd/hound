package database

import (
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func BenchmarkGetTokenMapByContract(b *testing.B) {
	db, err := OpenInMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	db.CreateSchema()

	// Seed 50 tokens
	for i := 0; i < 50; i++ {
		db.InsertToken(models.Token{
			Symbol:          "TKN" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Name:            "Token " + string(rune('A'+i%26)),
			ContractAddress: "mint" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + "111111111111111111111111111",
			Chain:           "solana",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetTokenMapByContract()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetAllTokens(b *testing.B) {
	db, err := OpenInMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	db.CreateSchema()

	// Seed 50 tokens with pools
	for i := 0; i < 50; i++ {
		sym := "TKN" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		db.InsertToken(models.Token{
			Symbol:          sym,
			Name:            "Token " + string(rune('A'+i%26)),
			ContractAddress: "mint" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + "111111111111111111111111111",
			Chain:           "solana",
		})
		db.InsertPool(sym, models.PoolInfo{
			Dex:         "raydium",
			PoolAddress: "pool" + sym,
			QuoteToken:  "sol",
			PoolType:    "amm_v4",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetAllTokens()
		if err != nil {
			b.Fatal(err)
		}
	}
}
