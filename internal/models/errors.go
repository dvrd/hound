package models

import (
	"errors"
	"fmt"
)

// Sentinel errors for known conditions.
var (
	// Wallet errors
	ErrWalletNotFound = errors.New("wallet not found")

	// Keystore errors
	ErrWeakPassword = errors.New("password does not meet strength requirements")
	ErrCryptoFailed = errors.New("cryptographic operation failed")
	ErrKeyNotFound  = errors.New("encrypted keypair not found")

	// Swap errors
	ErrQuoteExpired         = errors.New("swap quote expired")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrUntrustedTransaction = errors.New("transaction contains untrusted programs or wrong fee payer")
	ErrInvalidTransaction   = errors.New("transaction validation failed")

	// Network errors
	ErrRPCConnectionFailed = errors.New("cannot connect to Solana RPC")
	ErrRPCInvalidResponse  = errors.New("invalid response from Solana RPC")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrRateLimited         = errors.New("rate limit exceeded")

	// Token errors
	ErrTokenNotFound = errors.New("token not found")
	ErrNoPoolsFound  = errors.New("no liquidity pools found")

	// Parse errors
	ErrInvalidResponse = errors.New("invalid response")
	ErrParseError      = errors.New("parse error")

	// Oracle errors
	ErrOracleConnectionFailed = errors.New("cannot fetch SOL price")
	ErrOraclePriceInvalid     = errors.New("SOL price validation failed")

	// Transfer errors
	ErrInvalidRecipient    = errors.New("invalid recipient address")
	ErrSendToSelf          = errors.New("cannot send to own address")
	ErrTransactionFailed   = errors.New("transaction failed on-chain")
	ErrConfirmationTimeout = errors.New("transaction confirmation timed out")
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

// UserMessage returns a user-friendly error message with actionable guidance.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, ErrWalletNotFound):
		return "Wallet not found.\nRun 'hound wallet list' to see available wallets."

	case errors.Is(err, ErrWeakPassword):
		return "Password does not meet strength requirements.\nYour password must:\n  - Be at least 12 characters long\n  - Contain uppercase and lowercase letters\n  - Contain at least one digit\n  - Contain at least one special character"

	case errors.Is(err, ErrCryptoFailed):
		return "Cryptographic operation failed.\nThis may be due to:\n  - Incorrect password\n  - Corrupted encrypted data\n  - Invalid seed phrase"

	case errors.Is(err, ErrKeyNotFound):
		return "Wallet keypair not found.\nNo encrypted wallet found in database.\nImport a wallet first with: hound wallet import"

	case errors.Is(err, ErrQuoteExpired):
		return "Quote expired.\nThe swap quote is older than 90 seconds.\nPlease fetch a new quote and try again."

	case errors.Is(err, ErrInsufficientBalance):
		return "Insufficient balance.\nYour wallet doesn't have enough tokens to complete this swap."

	case errors.Is(err, ErrInvalidTransaction):
		return "Transaction validation failed.\nCommon causes:\n  - Quote expired (>90 seconds old)\n  - Insufficient SOL for network fees\n  - Route no longer available"

	case errors.Is(err, ErrUntrustedTransaction):
		return "Swap transaction rejected: contains untrusted programs or wrong fee payer.\nThis may indicate a compromised API response. Do NOT retry."

	case errors.Is(err, ErrRPCConnectionFailed):
		return "Cannot connect to Solana RPC.\nThe Solana network may be temporarily unavailable.\nCheck your internet connection or try a different RPC endpoint."

	case errors.Is(err, ErrRPCInvalidResponse):
		return "Invalid response from Solana RPC.\nReceived malformed data from blockchain node."

	case errors.Is(err, ErrRateLimited):
		return "Rate limit exceeded.\nWait 60 seconds before trying again."

	case errors.Is(err, ErrTokenNotFound):
		return "Token not found.\nThis token may not be listed yet or the address is incorrect.\nVisit https://dexscreener.com to verify."

	case errors.Is(err, ErrNoPoolsFound):
		return "No liquidity pools found.\nThis token may not have active trading pools yet.\nPools must have at least $1,000 liquidity and max 1% fees."

	case errors.Is(err, ErrInvalidRecipient):
		return "Invalid recipient address. Check the address and try again."

	case errors.Is(err, ErrSendToSelf):
		return "Cannot send to your own address."

	case errors.Is(err, ErrTransactionFailed):
		return "Transaction failed on-chain. Check explorer for details."

	case errors.Is(err, ErrConfirmationTimeout):
		return "Transaction may have been sent but confirmation timed out.\nCheck https://solscan.io for the transaction status."

	default:
		return err.Error()
	}
}
