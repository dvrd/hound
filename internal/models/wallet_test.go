package models_test

import (
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestWalletTypeString(t *testing.T) {
	tests := []struct {
		wt   models.WalletType
		want string
	}{
		{models.WalletTypeLegacy, "Legacy"},
		{models.WalletTypeBIP44Standard, "BIP44_Standard"},
		{models.WalletTypeBIP44Change, "BIP44_Change"},
		{models.WalletTypeSolanaCLI, "Solana_CLI"},
		{models.WalletType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.wt.String()
			if got != tt.want {
				t.Errorf("WalletType(%d).String() = %q, want %q", tt.wt, got, tt.want)
			}
		})
	}
}

func TestParseWalletType(t *testing.T) {
	tests := []struct {
		input string
		want  models.WalletType
	}{
		{"Legacy", models.WalletTypeLegacy},
		{"BIP44_Standard", models.WalletTypeBIP44Standard},
		{"BIP44_Change", models.WalletTypeBIP44Change},
		{"Solana_CLI", models.WalletTypeSolanaCLI},
		{"unknown", models.WalletTypeLegacy},
		{"", models.WalletTypeLegacy},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := models.ParseWalletType(tt.input)
			if got != tt.want {
				t.Errorf("ParseWalletType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseWalletTypeRoundTrip(t *testing.T) {
	types := []models.WalletType{
		models.WalletTypeLegacy,
		models.WalletTypeBIP44Standard,
		models.WalletTypeBIP44Change,
		models.WalletTypeSolanaCLI,
	}

	for _, wt := range types {
		t.Run(wt.String(), func(t *testing.T) {
			parsed := models.ParseWalletType(wt.String())
			if parsed != wt {
				t.Errorf("round-trip failed: %v → %q → %v", wt, wt.String(), parsed)
			}
		})
	}
}

func TestGetDerivationPath(t *testing.T) {
	tests := []struct {
		wt    models.WalletType
		index int
		want  string
	}{
		{models.WalletTypeBIP44Standard, 0, "m/44'/501'/0'/0'"},
		{models.WalletTypeBIP44Standard, 1, "m/44'/501'/1'/0'"},
		{models.WalletTypeBIP44Standard, 5, "m/44'/501'/5'/0'"},
		{models.WalletTypeBIP44Change, 0, "m/44'/501'/0'"},
		{models.WalletTypeBIP44Change, 3, "m/44'/501'/3'"},
		{models.WalletTypeSolanaCLI, 0, "m/44'/501'"},
		{models.WalletTypeSolanaCLI, 99, "m/44'/501'"}, // index ignored
		{models.WalletTypeLegacy, 0, "legacy-sha256"},
		{models.WalletTypeLegacy, 5, "legacy-sha256"}, // index ignored
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := models.GetDerivationPath(tt.wt, tt.index)
			if got != tt.want {
				t.Errorf("GetDerivationPath(%v, %d) = %q, want %q", tt.wt, tt.index, got, tt.want)
			}
		})
	}
}

func TestWalletStruct(t *testing.T) {
	w := models.Wallet{
		Address:        "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU",
		Label:          "Main Wallet",
		IsPrimary:      true,
		WalletType:     models.WalletTypeBIP44Standard,
		DerivationPath: "m/44'/501'/0'/0'",
		AccountIndex:   0,
	}

	if w.Address == "" {
		t.Error("wallet address should not be empty")
	}
	if !w.IsPrimary {
		t.Error("wallet should be primary")
	}
}

func TestPortfolioBalance(t *testing.T) {
	p := models.PortfolioBalance{
		WalletAddress: "7xKXtg...",
		SOLBalance: models.TokenBalance{
			Mint:     "So11111111111111111111111111111111111111112",
			Symbol:   "SOL",
			Amount:   12.3456,
			USDPrice: 145.32,
			USDValue: 1793.17,
		},
		TokenBalances: []models.TokenBalance{
			{
				Mint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				Symbol:   "USDC",
				Amount:   500.0,
				USDPrice: 1.0,
				USDValue: 500.0,
			},
		},
		TotalUSD: 2293.17,
	}

	if p.TotalUSD != 2293.17 {
		t.Errorf("total USD = %f, want 2293.17", p.TotalUSD)
	}
	if len(p.TokenBalances) != 1 {
		t.Errorf("token balances count = %d, want 1", len(p.TokenBalances))
	}
}
