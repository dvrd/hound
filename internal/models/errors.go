package models

import (
	"errors"
	"fmt"
)

// Sentinel errors for known conditions.
var (
	// Wallet errors
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrWalletAlreadyExists = errors.New("wallet already exists")

	// Keystore errors
	ErrWeakPassword      = errors.New("password does not meet strength requirements")
	ErrInvalidSeedPhrase = errors.New("invalid seed phrase")
	ErrCryptoFailed      = errors.New("cryptographic operation failed")
	ErrKeyNotFound       = errors.New("encrypted keypair not found")

	// Swap errors
	ErrQuoteExpired         = errors.New("swap quote expired")
	ErrHighPriceImpact      = errors.New("price impact too high")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrSlippageExceeded     = errors.New("slippage exceeded")
	ErrUntrustedTransaction = errors.New("transaction contains untrusted programs or wrong fee payer")
	ErrInvalidTransaction   = errors.New("transaction validation failed")

	// Network errors
	ErrRPCConnectionFailed = errors.New("cannot connect to Solana RPC")
	ErrRPCInvalidResponse  = errors.New("invalid response from Solana RPC")
	ErrNetworkTimeout      = errors.New("request timed out")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrServerError         = errors.New("server error")

	// Token errors
	ErrTokenNotFound      = errors.New("token not found")
	ErrTokenNotConfigured = errors.New("token not configured")
	ErrNoPoolsFound       = errors.New("no liquidity pools found")
	ErrPoolDataInvalid    = errors.New("pool data invalid")

	// Database errors
	ErrDatabaseError     = errors.New("database operation failed")
	ErrDatabaseCorrupted = errors.New("database integrity check failed")
	ErrMigrationFailed   = errors.New("database migration failed")

	// Config errors
	ErrConfigNotFound = errors.New("database not found")

	// Parse errors
	ErrInvalidResponse = errors.New("invalid response")
	ErrParseError      = errors.New("parse error")

	// Oracle errors
	ErrOracleConnectionFailed = errors.New("cannot fetch SOL price")
	ErrOraclePriceInvalid     = errors.New("SOL price validation failed")

	// Transfer errors
	ErrInvalidRecipient           = errors.New("invalid recipient address")
	ErrSendToSelf                 = errors.New("cannot send to own address")
	ErrInsufficientBalanceForRent = errors.New("insufficient balance for rent exemption")
	ErrTransactionFailed          = errors.New("transaction failed on-chain")
	ErrBlockhashExpired           = errors.New("blockhash expired — please retry")
	ErrConfirmationTimeout        = errors.New("transaction confirmation timed out")
)

// WalletNotFoundError provides context about which wallet was not found.
type WalletNotFoundError struct {
	Identifier string
}

func (e *WalletNotFoundError) Error() string {
	return fmt.Sprintf("wallet not found: %s", e.Identifier)
}

func (e *WalletNotFoundError) Unwrap() error {
	return ErrWalletNotFound
}

// TokenNotConfiguredError provides context about which token is missing.
type TokenNotConfiguredError struct {
	Symbol string
}

func (e *TokenNotConfiguredError) Error() string {
	return fmt.Sprintf("token %q not found in database", e.Symbol)
}

func (e *TokenNotConfiguredError) Unwrap() error {
	return ErrTokenNotConfigured
}

// ExitCode maps an error to a Unix exit code.
// Matches the Odin version's exit code mapping.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	switch {
	// Usage errors
	case errors.Is(err, ErrInvalidSeedPhrase),
		errors.Is(err, ErrWeakPassword),
		errors.Is(err, ErrWalletAlreadyExists),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrKeyNotFound),
		errors.Is(err, ErrTokenNotFound),
		errors.Is(err, ErrTokenNotConfigured),
		errors.Is(err, ErrNoPoolsFound),
		errors.Is(err, ErrQuoteExpired),
		errors.Is(err, ErrHighPriceImpact),
		errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrSlippageExceeded),
		errors.Is(err, ErrUntrustedTransaction),
		errors.Is(err, ErrInvalidRecipient),
		errors.Is(err, ErrSendToSelf),
		errors.Is(err, ErrInsufficientBalanceForRent):
		return 1

	// Migration/data format errors
	case errors.Is(err, ErrMigrationFailed):
		return 65

	// Service unavailable
	case errors.Is(err, ErrNetworkTimeout),
		errors.Is(err, ErrConnectionFailed),
		errors.Is(err, ErrRateLimited),
		errors.Is(err, ErrServerError),
		errors.Is(err, ErrRPCConnectionFailed),
		errors.Is(err, ErrOracleConnectionFailed),
		errors.Is(err, ErrBlockhashExpired),
		errors.Is(err, ErrConfirmationTimeout):
		return 69

	// Internal software error
	case errors.Is(err, ErrRPCInvalidResponse),
		errors.Is(err, ErrInvalidResponse),
		errors.Is(err, ErrParseError),
		errors.Is(err, ErrPoolDataInvalid),
		errors.Is(err, ErrOraclePriceInvalid),
		errors.Is(err, ErrCryptoFailed),
		errors.Is(err, ErrInvalidTransaction),
		errors.Is(err, ErrTransactionFailed):
		return 70

	// I/O error
	case errors.Is(err, ErrDatabaseError),
		errors.Is(err, ErrDatabaseCorrupted):
		return 74

	// Configuration error
	case errors.Is(err, ErrConfigNotFound):
		return 78

	default:
		return 1
	}
}

// UserMessage returns a user-friendly error message with actionable guidance.
// Matches the Odin version's error display catalog.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, ErrWalletNotFound):
		return "Wallet not found.\nRun 'hound wallet list' to see available wallets."

	case errors.Is(err, ErrWalletAlreadyExists):
		return "This seed phrase has already been imported.\nEach wallet can only be imported once."

	case errors.Is(err, ErrWeakPassword):
		return "Password does not meet strength requirements.\nYour password must:\n  - Be at least 12 characters long\n  - Contain uppercase and lowercase letters\n  - Contain at least one digit\n  - Contain at least one special character"

	case errors.Is(err, ErrInvalidSeedPhrase):
		return "Invalid seed phrase.\nSeed phrase must be exactly 12 or 24 words.\nEnsure you've copied the phrase correctly with proper spacing."

	case errors.Is(err, ErrCryptoFailed):
		return "Cryptographic operation failed.\nThis may be due to:\n  - Incorrect password\n  - Corrupted encrypted data\n  - Invalid seed phrase"

	case errors.Is(err, ErrKeyNotFound):
		return "Wallet keypair not found.\nNo encrypted wallet found in database.\nImport a wallet first with: hound wallet import"

	case errors.Is(err, ErrQuoteExpired):
		return "Quote expired.\nThe swap quote is older than 90 seconds.\nPlease fetch a new quote and try again."

	case errors.Is(err, ErrHighPriceImpact):
		return "Price impact too high.\nThe swap would significantly move the market price (>5% impact).\nConsider splitting into smaller trades."

	case errors.Is(err, ErrInsufficientBalance):
		return "Insufficient balance.\nYour wallet doesn't have enough tokens to complete this swap."

	case errors.Is(err, ErrSlippageExceeded):
		return "Slippage exceeded.\nThe price moved beyond your slippage tolerance.\nTry increasing slippage with --slippage <bps>."

	case errors.Is(err, ErrInvalidTransaction):
		return "Transaction validation failed.\nCommon causes:\n  - Quote expired (>90 seconds old)\n  - Insufficient SOL for network fees\n  - Route no longer available"

	case errors.Is(err, ErrUntrustedTransaction):
		return "Swap transaction rejected: contains untrusted programs or wrong fee payer.\nThis may indicate a compromised API response. Do NOT retry."

	case errors.Is(err, ErrRPCConnectionFailed):
		return "Cannot connect to Solana RPC.\nThe Solana network may be temporarily unavailable.\nCheck your internet connection or try a different RPC endpoint."

	case errors.Is(err, ErrRPCInvalidResponse):
		return "Invalid response from Solana RPC.\nReceived malformed data from blockchain node."

	case errors.Is(err, ErrNetworkTimeout):
		return "Request timed out.\nCheck your internet connection and try again."

	case errors.Is(err, ErrRateLimited):
		return "Rate limit exceeded.\nWait 60 seconds before trying again."

	case errors.Is(err, ErrTokenNotFound):
		return "Token not found.\nThis token may not be listed yet or the address is incorrect.\nVisit https://dexscreener.com to verify."

	case errors.Is(err, ErrTokenNotConfigured):
		return "Token not found in database.\nRun 'hound tokens list' to see available tokens.\nAdd new tokens with: hound tokens add <symbol> <name> <address>"

	case errors.Is(err, ErrNoPoolsFound):
		return "No liquidity pools found.\nThis token may not have active trading pools yet.\nPools must have at least $1,000 liquidity and max 1% fees."

	case errors.Is(err, ErrDatabaseError):
		return "Database operation failed.\nCheck file permissions at ~/.config/hound/hound.db"

	case errors.Is(err, ErrDatabaseCorrupted):
		return "Database integrity check failed.\nThe database file may be corrupted.\nDelete the file and re-add your tokens and wallets."

	case errors.Is(err, ErrMigrationFailed):
		return "Database initialization failed.\nCould not create or update database schema.\nCheck file permissions at ~/.config/hound/"

	case errors.Is(err, ErrConfigNotFound):
		return "Database not found.\nExpected location: ~/.config/hound/hound.db\n\nAdd your first token to create the database:\n  hound tokens add <symbol> <name> <address>"

	case errors.Is(err, ErrInvalidRecipient):
		return "Invalid recipient address. Check the address and try again."

	case errors.Is(err, ErrSendToSelf):
		return "Cannot send to your own address."

	case errors.Is(err, ErrInsufficientBalanceForRent):
		return "Insufficient SOL to cover rent exemption for new account."

	case errors.Is(err, ErrTransactionFailed):
		return "Transaction failed on-chain. Check explorer for details."

	case errors.Is(err, ErrBlockhashExpired):
		return "Transaction expired. Please try again."

	case errors.Is(err, ErrConfirmationTimeout):
		return "Transaction may have been sent but confirmation timed out.\nCheck https://solscan.io for the transaction status."

	default:
		return err.Error()
	}
}
