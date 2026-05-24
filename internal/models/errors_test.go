package models_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors are distinct
	sentinels := []error{
		models.ErrWalletNotFound,
		models.ErrWeakPassword,
		models.ErrCryptoFailed,
		models.ErrKeyNotFound,
		models.ErrQuoteExpired,
		models.ErrInsufficientBalance,
		models.ErrUntrustedTransaction,
		models.ErrInvalidTransaction,
		models.ErrRPCConnectionFailed,
		models.ErrRPCInvalidResponse,
		models.ErrConnectionFailed,
		models.ErrRateLimited,
		models.ErrTokenNotFound,
		models.ErrNoPoolsFound,
		models.ErrInvalidResponse,
		models.ErrParseError,
		models.ErrOracleConnectionFailed,
		models.ErrOraclePriceInvalid,
		models.ErrInvalidRecipient,
		models.ErrSendToSelf,
		models.ErrTransactionFailed,
		models.ErrConfirmationTimeout,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel errors %d and %d should not match: %v == %v", i, j, a, b)
			}
		}
	}
}

func TestWalletNotFoundError(t *testing.T) {
	err := &models.WalletNotFoundError{Identifier: "7xKXtg..."}

	if !errors.Is(err, models.ErrWalletNotFound) {
		t.Error("WalletNotFoundError should unwrap to ErrWalletNotFound")
	}

	expected := "wallet not found: 7xKXtg..."
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("fetching balance for wallet: %w", models.ErrRPCConnectionFailed)

	if !errors.Is(wrapped, models.ErrRPCConnectionFailed) {
		t.Error("wrapped error should match sentinel via errors.Is")
	}
}

func TestUserMessage(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{nil, ""},
		{models.ErrWeakPassword, "12 characters"},
		{models.ErrKeyNotFound, "hound wallet import"},
		{models.ErrQuoteExpired, "90 seconds"},
		{models.ErrRPCConnectionFailed, "Solana RPC"},
		{models.ErrRateLimited, "60 seconds"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			msg := models.UserMessage(tt.err)
			if tt.err == nil {
				if msg != "" {
					t.Errorf("UserMessage(nil) = %q, want empty", msg)
				}
				return
			}
			if tt.contains != "" && !containsSubstring(msg, tt.contains) {
				t.Errorf("UserMessage(%v) = %q, want to contain %q", tt.err, msg, tt.contains)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTransferErrorMessages(t *testing.T) {
	errs := []error{
		models.ErrInvalidRecipient,
		models.ErrSendToSelf,
		models.ErrTransactionFailed,
		models.ErrConfirmationTimeout,
	}
	for _, err := range errs {
		msg := models.UserMessage(err)
		if msg == "" {
			t.Errorf("UserMessage(%v) returned empty string", err)
		}
		if msg == err.Error() {
			t.Errorf("UserMessage(%v) should return user-friendly message, not raw error", err)
		}
	}
}
