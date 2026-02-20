package tui

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestWalletToJSON(t *testing.T) {
	w := models.Wallet{
		Address:        "ABC123DEF456",
		Label:          "main",
		IsPrimary:      true,
		WalletType:     models.WalletTypeBIP44Standard,
		DerivationPath: "m/44'/501'/0'/0'",
		AccountIndex:   0,
	}

	got := WalletToJSON(w, 123.45)

	if got.Address != "ABC123DEF456" {
		t.Errorf("Address = %q, want %q", got.Address, "ABC123DEF456")
	}
	if got.Label != "main" {
		t.Errorf("Label = %q, want %q", got.Label, "main")
	}
	if !got.IsPrimary {
		t.Error("IsPrimary = false, want true")
	}
	if got.WalletType != "BIP44_Standard" {
		t.Errorf("WalletType = %q, want %q", got.WalletType, "BIP44_Standard")
	}
	if got.DerivationPath != "m/44'/501'/0'/0'" {
		t.Errorf("DerivationPath = %q, want %q", got.DerivationPath, "m/44'/501'/0'/0'")
	}
	if got.TotalUSD != 123.45 {
		t.Errorf("TotalUSD = %f, want %f", got.TotalUSD, 123.45)
	}
}

func TestPortfolioToJSON(t *testing.T) {
	p := models.PortfolioBalance{
		WalletAddress: "WALLET1",
		SOLBalance: models.TokenBalance{
			Mint:     "So11111111111111111111111111111111111111112",
			Symbol:   "SOL",
			Amount:   10.5,
			USDPrice: 150.0,
			USDValue: 1575.0,
		},
		TokenBalances: []models.TokenBalance{
			{
				Mint:      "USDC_MINT",
				Symbol:    "USDC",
				Amount:    500.0,
				USDPrice:  1.0,
				USDValue:  500.0,
				Change24h: 0.01,
			},
		},
		TotalUSD: 2075.0,
	}

	got := PortfolioToJSON(p)

	if got.WalletAddress != "WALLET1" {
		t.Errorf("WalletAddress = %q, want %q", got.WalletAddress, "WALLET1")
	}
	if got.TotalUSD != 2075.0 {
		t.Errorf("TotalUSD = %f, want %f", got.TotalUSD, 2075.0)
	}
	if got.SOL.Symbol != "SOL" {
		t.Errorf("SOL.Symbol = %q, want %q", got.SOL.Symbol, "SOL")
	}
	if got.SOL.Amount != 10.5 {
		t.Errorf("SOL.Amount = %f, want %f", got.SOL.Amount, 10.5)
	}
	if len(got.Tokens) != 1 {
		t.Fatalf("len(Tokens) = %d, want 1", len(got.Tokens))
	}
	if got.Tokens[0].Symbol != "USDC" {
		t.Errorf("Tokens[0].Symbol = %q, want %q", got.Tokens[0].Symbol, "USDC")
	}
}

func TestPortfolioToJSON_EmptyTokens(t *testing.T) {
	p := models.PortfolioBalance{
		WalletAddress: "WALLET2",
		SOLBalance: models.TokenBalance{
			Mint:   "So11111111111111111111111111111111111111112",
			Symbol: "SOL",
		},
		TotalUSD: 0,
	}

	got := PortfolioToJSON(p)

	if got.Tokens == nil {
		t.Error("Tokens should be non-nil empty slice, got nil")
	}
	if len(got.Tokens) != 0 {
		t.Errorf("len(Tokens) = %d, want 0", len(got.Tokens))
	}
}

func TestTokenBalanceToJSON(t *testing.T) {
	tb := models.TokenBalance{
		Mint:      "BONK_MINT",
		Symbol:    "BONK",
		Amount:    1000000.0,
		USDPrice:  0.000028,
		USDValue:  28.0,
		Change24h: 5.2,
	}

	got := TokenBalanceToJSON(tb)

	if got.Mint != "BONK_MINT" {
		t.Errorf("Mint = %q, want %q", got.Mint, "BONK_MINT")
	}
	if got.Symbol != "BONK" {
		t.Errorf("Symbol = %q, want %q", got.Symbol, "BONK")
	}
	if got.Amount != 1000000.0 {
		t.Errorf("Amount = %f, want %f", got.Amount, 1000000.0)
	}
	if got.USDPrice != 0.000028 {
		t.Errorf("USDPrice = %f, want %f", got.USDPrice, 0.000028)
	}
	if got.USDValue != 28.0 {
		t.Errorf("USDValue = %f, want %f", got.USDValue, 28.0)
	}
	if got.Change24h != 5.2 {
		t.Errorf("Change24h = %f, want %f", got.Change24h, 5.2)
	}
}

func TestTokenToJSON(t *testing.T) {
	token := models.Token{
		Symbol:          "BONK",
		Name:            "Bonk",
		ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
		USDPrice:        0.000028,
	}

	got := TokenToJSON(token, 3)

	if got.Symbol != "BONK" {
		t.Errorf("Symbol = %q, want %q", got.Symbol, "BONK")
	}
	if got.Name != "Bonk" {
		t.Errorf("Name = %q, want %q", got.Name, "Bonk")
	}
	if got.ContractAddress != "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263" {
		t.Errorf("ContractAddress = %q, want %q", got.ContractAddress, "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263")
	}
	if got.PoolCount != 3 {
		t.Errorf("PoolCount = %d, want %d", got.PoolCount, 3)
	}
	if got.USDPrice != 0.000028 {
		t.Errorf("USDPrice = %f, want %f", got.USDPrice, 0.000028)
	}
}

func TestSwapHistoryToJSON(t *testing.T) {
	entry := models.SwapHistoryEntry{
		ID:            1,
		WalletAddress: "WALLET1",
		InputSymbol:   "SOL",
		OutputSymbol:  "USDC",
		InputAmount:   1.0,
		OutputAmount:  150.0,
		PriceImpact:   0.05,
		Status:        "confirmed",
		Dex:           "Jupiter",
		Signature:     "sig123",
		CreatedAt:     1700000000,
	}

	got := SwapHistoryToJSON(entry)

	if got.ID != 1 {
		t.Errorf("ID = %d, want %d", got.ID, 1)
	}
	if got.InputSymbol != "SOL" {
		t.Errorf("InputSymbol = %q, want %q", got.InputSymbol, "SOL")
	}
	if got.OutputSymbol != "USDC" {
		t.Errorf("OutputSymbol = %q, want %q", got.OutputSymbol, "USDC")
	}
	if got.InputAmount != 1.0 {
		t.Errorf("InputAmount = %f, want %f", got.InputAmount, 1.0)
	}
	if got.OutputAmount != 150.0 {
		t.Errorf("OutputAmount = %f, want %f", got.OutputAmount, 150.0)
	}
	if got.Status != "confirmed" {
		t.Errorf("Status = %q, want %q", got.Status, "confirmed")
	}
	if got.Dex != "Jupiter" {
		t.Errorf("Dex = %q, want %q", got.Dex, "Jupiter")
	}
	if got.Signature != "sig123" {
		t.Errorf("Signature = %q, want %q", got.Signature, "sig123")
	}
}

func TestWalletJSON_Serialization(t *testing.T) {
	wj := WalletJSON{
		Address:        "ABC123",
		Label:          "main",
		IsPrimary:      true,
		WalletType:     "BIP44_Standard",
		DerivationPath: "m/44'/501'/0'/0'",
		TotalUSD:       100.50,
	}

	data, err := json.Marshal(wj)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got WalletJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got != wj {
		t.Errorf("Round-trip mismatch: got %+v, want %+v", got, wj)
	}
}

func TestWalletJSON_OmitEmptyTotalUSD(t *testing.T) {
	wj := WalletJSON{
		Address:    "ABC123",
		Label:      "test",
		WalletType: "Legacy",
		TotalUSD:   0,
	}

	data, err := json.Marshal(wj)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	if bytes.Contains(data, []byte(`"total_usd"`)) {
		t.Error("Expected total_usd to be omitted when zero")
	}
}

func TestPortfolioJSON_Serialization(t *testing.T) {
	pj := PortfolioJSON{
		WalletAddress: "WALLET1",
		TotalUSD:      1000.0,
		SOL: TokenJSON{
			Mint:     "SOL_MINT",
			Symbol:   "SOL",
			Amount:   5.0,
			USDPrice: 150.0,
			USDValue: 750.0,
		},
		Tokens: []TokenJSON{
			{
				Mint:      "USDC_MINT",
				Symbol:    "USDC",
				Amount:    250.0,
				USDPrice:  1.0,
				USDValue:  250.0,
				Change24h: 0.01,
			},
		},
	}

	data, err := json.Marshal(pj)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got PortfolioJSON
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got.WalletAddress != pj.WalletAddress {
		t.Errorf("WalletAddress = %q, want %q", got.WalletAddress, pj.WalletAddress)
	}
	if got.TotalUSD != pj.TotalUSD {
		t.Errorf("TotalUSD = %f, want %f", got.TotalUSD, pj.TotalUSD)
	}
	if len(got.Tokens) != 1 {
		t.Fatalf("len(Tokens) = %d, want 1", len(got.Tokens))
	}
}
