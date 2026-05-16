package services

import (
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/swap"
)

// SwapService orchestrates swap execution: quote validation, signing, submission, and history.
type SwapService struct {
	swapClient   *swap.SwapClient
	keyCustodian KeyCustodian
	db           *database.Database
}

// NewSwapService creates a new SwapService.
func NewSwapService(swapClient *swap.SwapClient, keyCustodian KeyCustodian, db *database.Database) *SwapService {
	return &SwapService{
		swapClient:   swapClient,
		keyCustodian: keyCustodian,
		db:           db,
	}
}

// ExecuteSwap validates a quote, signs the transaction, submits it, and records history.
func (s *SwapService) ExecuteSwap(quote models.SwapQuote, walletAddr string, password string) (models.SwapTransactionResult, error) {
	// Check if quote is expired
	if quote.IsExpired() {
		return models.SwapTransactionResult{}, fmt.Errorf("execute swap: %w", models.ErrQuoteExpired)
	}

	// Unlock keypair
	privKey, err := s.keyCustodian.UnlockWallet(walletAddr, password)
	if err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("execute swap: unlock keypair: %w", err)
	}
	// Zero private key when done
	defer func() {
		for i := range privKey {
			privKey[i] = 0
		}
	}()

	// Validate the quote has a transaction payload.
	if quote.TransactionPayload == "" {
		return models.SwapTransactionResult{}, fmt.Errorf("execute swap: no transaction in quote: %w", models.ErrInvalidTransaction)
	}

	// Sign transaction
	signedTx, err := swap.SignTransaction(quote.TransactionPayload, privKey)
	if err != nil {
		return models.SwapTransactionResult{}, fmt.Errorf("execute swap: %w", err)
	}

	// Submit transaction — reuse the SwapClient's HTTP client (already configured with timeouts).
	result, err := swap.SubmitTransaction(s.swapClient.HTTPClient(), "https://lite-api.jup.ag", signedTx, quote.RequestID)
	if err != nil {
		// Save failed swap to history
		s.saveHistory(quote, walletAddr, result)
		return result, fmt.Errorf("execute swap: %w", err)
	}

	// Save successful swap to history
	s.saveHistory(quote, walletAddr, result)

	return result, nil
}

// saveHistory records a swap attempt in the database (best-effort).
func (s *SwapService) saveHistory(quote models.SwapQuote, walletAddr string, result models.SwapTransactionResult) {
	entry := models.SwapHistoryEntry{
		WalletAddress: walletAddr,
		InputMint:     quote.InputMint,
		OutputMint:    quote.OutputMint,
		InputSymbol:   quote.InputSymbol,
		OutputSymbol:  quote.OutputSymbol,
		InputAmount:   quote.InputAmount,
		OutputAmount:  quote.OutputAmount,
		PriceImpact:   quote.PriceImpactPct,
		SlippageBps:   quote.SlippageBps,
		Signature:     result.Signature,
		Status:        result.Status,
		Dex:           result.Dex,
		NetworkFee:    result.Fees.NetworkFee,
		PriorityFee:   result.Fees.PriorityFee,
		ErrorMessage:  result.ErrorMessage,
		CreatedAt:     time.Now().Unix(),
	}

	// Best-effort: ignore errors
	_ = s.db.InsertSwapHistory(entry)
}
