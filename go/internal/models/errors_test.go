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
		models.ErrWalletAlreadyExists,
		models.ErrWeakPassword,
		models.ErrInvalidSeedPhrase,
		models.ErrCryptoFailed,
		models.ErrKeyNotFound,
		models.ErrQuoteExpired,
		models.ErrHighPriceImpact,
		models.ErrInsufficientBalance,
		models.ErrRPCConnectionFailed,
		models.ErrNetworkTimeout,
		models.ErrRateLimited,
		models.ErrTokenNotFound,
		models.ErrTokenNotConfigured,
		models.ErrNoPoolsFound,
		models.ErrDatabaseError,
		models.ErrDatabaseCorrupted,
		models.ErrMigrationFailed,
		models.ErrConfigNotFound,
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

func TestTokenNotConfiguredError(t *testing.T) {
	err := &models.TokenNotConfiguredError{Symbol: "BONK"}

	if !errors.Is(err, models.ErrTokenNotConfigured) {
		t.Error("TokenNotConfiguredError should unwrap to ErrTokenNotConfigured")
	}

	expected := `token "BONK" not found in database`
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

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"nil", nil, 0},
		{"wallet not found", models.ErrWalletNotFound, 1},
		{"weak password", models.ErrWeakPassword, 1},
		{"invalid seed", models.ErrInvalidSeedPhrase, 1},
		{"wallet exists", models.ErrWalletAlreadyExists, 1},
		{"quote expired", models.ErrQuoteExpired, 1},
		{"insufficient balance", models.ErrInsufficientBalance, 1},
		{"token not found", models.ErrTokenNotFound, 1},
		{"no pools", models.ErrNoPoolsFound, 1},
		{"migration failed", models.ErrMigrationFailed, 65},
		{"network timeout", models.ErrNetworkTimeout, 69},
		{"rate limited", models.ErrRateLimited, 69},
		{"rpc connection", models.ErrRPCConnectionFailed, 69},
		{"oracle connection", models.ErrOracleConnectionFailed, 69},
		{"rpc invalid response", models.ErrRPCInvalidResponse, 70},
		{"crypto failed", models.ErrCryptoFailed, 70},
		{"invalid transaction", models.ErrInvalidTransaction, 70},
		{"database error", models.ErrDatabaseError, 74},
		{"database corrupted", models.ErrDatabaseCorrupted, 74},
		{"config not found", models.ErrConfigNotFound, 78},
		{"unknown error", errors.New("something else"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.ExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

func TestExitCodeWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", models.ErrRPCConnectionFailed)
	got := models.ExitCode(wrapped)
	if got != 69 {
		t.Errorf("ExitCode(wrapped RPC error) = %d, want 69", got)
	}
}

func TestUserMessage(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{nil, ""},
		{models.ErrWeakPassword, "12 characters"},
		{models.ErrInvalidSeedPhrase, "12 or 24 words"},
		{models.ErrWalletAlreadyExists, "already been imported"},
		{models.ErrKeyNotFound, "hound wallet import"},
		{models.ErrQuoteExpired, "90 seconds"},
		{models.ErrRPCConnectionFailed, "Solana RPC"},
		{models.ErrRateLimited, "60 seconds"},
		{models.ErrConfigNotFound, "~/.config/hound/hound.db"},
		{models.ErrDatabaseCorrupted, "corrupted"},
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
