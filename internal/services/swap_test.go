package services_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/swap"
	"github.com/dvrd/hound/internal/transaction"
)

// makeTestTransaction builds a minimal valid Solana transaction for testing.
// The fee payer must match the signer's public key for ValidateSwapTransaction.
func makeTestTransaction(address string) string {
	// Decode base58 address to get 32-byte pubkey
	pubkey, _ := transaction.PubkeyFromBase58(address)

	var tx []byte
	tx = append(tx, 1)                   // 1 signature
	tx = append(tx, make([]byte, 64)...) // empty sig slot
	tx = append(tx, 1, 0, 1)             // header: 1 required sig, 0 readonly signed, 1 readonly unsigned
	tx = append(tx, 2)                   // 2 accounts
	tx = append(tx, pubkey[:]...)        // account 0: signer/fee payer
	tx = append(tx, make([]byte, 32)...) // account 1: system program (all zeros)
	tx = append(tx, make([]byte, 32)...) // blockhash
	tx = append(tx, 1)                   // 1 instruction
	tx = append(tx, 1)                   // programIdIndex = 1 (system program)
	tx = append(tx, 1)                   // 1 account index
	tx = append(tx, 0)                   // account index 0
	tx = append(tx, 4)                   // data length 4
	tx = append(tx, 0, 0, 0, 0)          // data
	return base64.StdEncoding.EncodeToString(tx)
}

func TestSwapService_ExecuteSwap_ExpiredQuote(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "swap-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, svc, db)

	// Create an expired quote
	quote := models.SwapQuote{
		InputMint:  "So11111111111111111111111111111111111111112",
		OutputMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:   "1000000000",
		OutAmount:  "150000000",
		FetchedAt:  time.Now().Add(-2 * models.QuoteTTL), // expired
	}

	_, err = swapSvc.ExecuteSwap(quote, address, testPassword)
	if err == nil {
		t.Fatal("expected error for expired quote")
	}
	if !errors.Is(err, models.ErrQuoteExpired) {
		t.Errorf("expected ErrQuoteExpired, got: %v", err)
	}
}

func TestSwapService_ExecuteSwap_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "swap-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, svc, db)

	rawResp, _ := json.Marshal(map[string]interface{}{
		"transaction": makeTestTransaction(address),
		"requestId":   "req-123",
	})

	quote := models.SwapQuote{
		InputMint:   "So11111111111111111111111111111111111111112",
		OutputMint:  "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:    "1000000000",
		OutAmount:   "150000000",
		FetchedAt:   time.Now(),
		RawResponse: rawResp,
	}

	_, err = swapSvc.ExecuteSwap(quote, address, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !errors.Is(err, models.ErrCryptoFailed) {
		t.Errorf("expected ErrCryptoFailed, got: %v", err)
	}
}

func TestSwapService_ExecuteSwap_NoTransaction(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "swap-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, svc, db)

	// Quote with no transaction field
	rawResp, _ := json.Marshal(map[string]interface{}{
		"requestId": "req-123",
	})

	quote := models.SwapQuote{
		InputMint:   "So11111111111111111111111111111111111111112",
		OutputMint:  "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:    "1000000000",
		OutAmount:   "150000000",
		FetchedAt:   time.Now(),
		RawResponse: rawResp,
	}

	_, err = swapSvc.ExecuteSwap(quote, address, testPassword)
	if err == nil {
		t.Fatal("expected error for missing transaction")
	}
	if !errors.Is(err, models.ErrInvalidTransaction) {
		t.Errorf("expected ErrInvalidTransaction, got: %v", err)
	}
}

func TestSwapService_ExecuteSwap_SignsAndSubmits(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "swap-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	rawResp, _ := json.Marshal(map[string]interface{}{
		"transaction": makeTestTransaction(address),
		"requestId":   "req-123",
	})

	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, svc, db)

	quote := models.SwapQuote{
		InputMint:    "So11111111111111111111111111111111111111112",
		OutputMint:   "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:     "1000000000",
		OutAmount:    "150000000",
		InputSymbol:  "SOL",
		OutputSymbol: "USDC",
		InputAmount:  1.0,
		OutputAmount: 150.0,
		FetchedAt:    time.Now(),
		RawResponse:  rawResp,
	}

	// Execute - will fail on submit (hits real Jupiter API), but signing should succeed
	_, err = swapSvc.ExecuteSwap(quote, address, testPassword)
	// We expect a submit error (network), NOT a signing or unlock error
	if err != nil {
		if errors.Is(err, models.ErrQuoteExpired) || errors.Is(err, models.ErrCryptoFailed) || errors.Is(err, models.ErrInvalidTransaction) {
			t.Fatalf("unexpected error type (should only fail on submit): %v", err)
		}
		// Submit failure is expected - we're not mocking the Jupiter execute endpoint
	}

	// Verify history was saved (even on failure)
	count, err := db.GetSwapHistoryCount(address)
	if err != nil {
		t.Fatalf("GetSwapHistoryCount failed: %v", err)
	}
	if count == 0 {
		t.Error("expected swap history to be saved")
	}
}

func TestSwapService_ExecuteSwap_InvalidRawResponse(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "swap-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, svc, db)

	quote := models.SwapQuote{
		InputMint:   "So11111111111111111111111111111111111111112",
		OutputMint:  "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		InAmount:    "1000000000",
		OutAmount:   "150000000",
		FetchedAt:   time.Now(),
		RawResponse: json.RawMessage(`{invalid json`),
	}

	_, err = swapSvc.ExecuteSwap(quote, address, testPassword)
	if err == nil {
		t.Fatal("expected error for invalid raw response")
	}
	if !errors.Is(err, models.ErrInvalidTransaction) {
		t.Errorf("expected ErrInvalidTransaction, got: %v", err)
	}
}
