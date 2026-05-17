package database

import (
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func BenchmarkUpdateBalancesBatch(b *testing.B) {
	db, _ := OpenInMemory()
	defer db.Close()
	db.CreateSchema()

	// Need a wallet for FK
	db.InsertWallet(models.Wallet{Address: "bench_wallet", Label: "bench", IsPrimary: true, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy"})

	balances := make([]models.TokenBalance, 20)
	for i := range balances {
		balances[i] = models.TokenBalance{
			Mint:     "mint" + string(rune('A'+i)),
			Symbol:   "TKN" + string(rune('A'+i)),
			Name:     "Token " + string(rune('A'+i)),
			Amount:   float64(i) * 1.5,
			USDPrice: 1.23,
			USDValue: float64(i) * 1.845,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTx()
		_ = db.UpdateBalancesBatch(tx, "bench_wallet", balances)
		tx.Commit()
	}
}

func BenchmarkUpdateBalanceTx_Loop(b *testing.B) {
	db, _ := OpenInMemory()
	defer db.Close()
	db.CreateSchema()

	db.InsertWallet(models.Wallet{Address: "bench_wallet", Label: "bench", IsPrimary: true, WalletType: models.WalletTypeLegacy, DerivationPath: "legacy"})

	balances := make([]models.TokenBalance, 20)
	for i := range balances {
		balances[i] = models.TokenBalance{
			Mint:     "mint" + string(rune('A'+i)),
			Symbol:   "TKN" + string(rune('A'+i)),
			Name:     "Token " + string(rune('A'+i)),
			Amount:   float64(i) * 1.5,
			USDPrice: 1.23,
			USDValue: float64(i) * 1.845,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := db.BeginTx()
		for _, bal := range balances {
			_ = db.UpdateBalanceTx(tx, "bench_wallet", bal.Mint, bal.Symbol, bal.Name, bal.Amount, bal.USDPrice, bal.USDValue)
		}
		tx.Commit()
	}
}
