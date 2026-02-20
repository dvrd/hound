package models

import "fmt"

// WalletType represents the key derivation standard used for a wallet.
type WalletType int

const (
	WalletTypeLegacy        WalletType = iota // SHA-256 seed → Ed25519 (Hound-only)
	WalletTypeBIP44Standard                   // m/44'/501'/{account}'/0' (Phantom, Solflare, Ledger, Backpack)
	WalletTypeBIP44Change                     // m/44'/501'/{account}' (Trust Wallet)
	WalletTypeSolanaCLI                       // m/44'/501' (solana-keygen)
)

// String returns the display name of the wallet type.
func (wt WalletType) String() string {
	switch wt {
	case WalletTypeLegacy:
		return "Legacy"
	case WalletTypeBIP44Standard:
		return "BIP44_Standard"
	case WalletTypeBIP44Change:
		return "BIP44_Change"
	case WalletTypeSolanaCLI:
		return "Solana_CLI"
	default:
		return "Unknown"
	}
}

// ParseWalletType converts a string to a WalletType.
// Returns WalletTypeLegacy for unrecognized strings.
func ParseWalletType(s string) WalletType {
	switch s {
	case "Legacy":
		return WalletTypeLegacy
	case "BIP44_Standard":
		return WalletTypeBIP44Standard
	case "BIP44_Change":
		return WalletTypeBIP44Change
	case "Solana_CLI":
		return WalletTypeSolanaCLI
	default:
		return WalletTypeLegacy
	}
}

// GetDerivationPath returns the BIP44 derivation path for the given wallet type and account index.
func GetDerivationPath(wt WalletType, accountIndex int) string {
	switch wt {
	case WalletTypeBIP44Standard:
		return fmt.Sprintf("m/44'/501'/%d'/0'", accountIndex)
	case WalletTypeBIP44Change:
		return fmt.Sprintf("m/44'/501'/%d'", accountIndex)
	case WalletTypeSolanaCLI:
		return "m/44'/501'"
	case WalletTypeLegacy:
		return "legacy-sha256"
	default:
		return "legacy-sha256"
	}
}

// Wallet represents a stored wallet in the database.
type Wallet struct {
	Address        string
	Label          string
	IsPrimary      bool
	WalletType     WalletType
	DerivationPath string
	AccountIndex   int
}

// TokenBalance represents a single token holding with USD valuation.
type TokenBalance struct {
	Mint      string  // Token mint address
	Symbol    string  // "SOL", "USDC", etc.
	Amount    float64 // Decimal-adjusted balance
	Decimals  int
	USDPrice  float64 // Price per token
	USDValue  float64 // Amount * Price
	Change24h float64 // 24h change percentage
}

// PortfolioBalance represents the complete portfolio for a wallet.
type PortfolioBalance struct {
	WalletAddress string
	SOLBalance    TokenBalance
	TokenBalances []TokenBalance
	TotalUSD      float64
}
