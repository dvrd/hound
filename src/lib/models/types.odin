package models

import "core:strings"
import "core:fmt"

// Error types for the application
ErrorType :: enum {
	None,

	// Usage errors (user's fault)
	MissingArgument,      // No token address provided
	InvalidToken,         // Malformed token address

	// API errors (token/service issues)
	TokenNotFound,        // 404 or empty pairs array
	RateLimited,          // 429 Too Many Requests
	ServerError,          // 500/503 API down

	// Network errors (connection issues)
	NetworkTimeout,       // Timeout waiting for response
	ConnectionFailed,     // Cannot establish connection
	NetworkError,         // Generic network error

	// Parse errors (data issues)
	InvalidResponse,      // Malformed JSON or unexpected structure
	ParseError,           // JSON parsing failed

	// Config errors (configuration issues)
	TokenNotConfigured,   // Symbol not found in config
	ConfigNotFound,       // Config file doesn't exist
	ConfigParseError,     // Failed to parse config JSON

	// RPC errors (on-chain fetching issues)
	RPCConnectionFailed,  // Cannot connect to Solana RPC
	RPCInvalidResponse,   // Malformed RPC response
	PoolDataInvalid,      // Pool data validation failed
	VaultFetchFailed,     // Failed to fetch vault balances

	// Oracle errors (SOL price fetching)
	OracleConnectionFailed, // Cannot reach Jupiter/CoinGecko APIs
	OracleParseFailed,      // Invalid API response format
	OraclePriceInvalid,     // Price validation failed

	// Database errors
	DatabaseError,          // Generic database operation failure
	DatabaseCorrupted,      // Integrity check failed (PRAGMA integrity_check)
	MigrationFailed,        // JSON to database migration error

	// Pool Discovery errors
	PoolSearchFailed,       // DexScreener API call succeeded but returned invalid/unexpected data
	NoPoolsFound,           // No pools meet filtering criteria (min liquidity, max fees, etc.)

	// Keystore errors
	WeakPassword,           // Password doesn't meet strength requirements
	InvalidSeedPhrase,      // BIP39 validation failed
	CryptoOperationFailed,  // Argon2/AES operation error
	KeypairNotFound,        // No encrypted keypair in database
	WalletAlreadyExists,    // Duplicate wallet import attempt

	// Swap-specific errors (Phase 2)
	QuoteExpired,           // Jupiter quote older than 90 seconds
	HighPriceImpact,        // Price impact > 5%
	InsufficientBalance,    // Wallet balance too low
	SlippageExceeded,       // Actual output < minimum (Phase 3)
	InvalidTransaction,     // Transaction failed validation (bad signature, expired, etc.)
}

// API response structures matching DexScreener API
DexScreenerResponse :: struct {
	pairs: []PairData,
}

PairData :: struct {
	priceUsd:    string `json:"priceUsd"`,
	priceChange: PriceChange,
}

PriceChange :: struct {
	h24: f64,
}

// Internal price data structure
PriceData :: struct {
	price_usd:   f64,
	change_24h:  f64,
}

// PoolInfo represents a liquidity pool for a token
PoolInfo :: struct {
	dex:           string, // "raydium"
	pool_address:  string, // Pool account address
	quote_token:   string, // "sol", "usdc", etc.
	pool_type:     string, // "amm_v4"
	// Pool metadata
	liquidity_usd: f64,    // Current pool liquidity in USD (0.0 if unknown)
	volume_24h:    f64,    // 24-hour trading volume (0.0 if unknown)
	fee_percent:   f64,    // Trading fee percentage (0.0 if unknown)
	discovered_at: i64,    // Unix timestamp when auto-discovered (0 for manual)
}

// Token represents a single cryptocurrency token configuration
Token :: struct {
	symbol:           string,
	name:             string,
	contract_address: string,
	chain:            string,
	decimals:         int,         // Token decimals (6 for USDC, 9 for SOL)
	pools:            []PoolInfo, // Liquidity pools for on-chain pricing
	is_quote_token:   bool, // True if this is a quote token (SOL, USDC)
	usd_price:        f64, // USD price for quote tokens
}

// Wallet_Type represents the derivation standard used for a wallet
//
// ASSERTION: Must match database string values exactly
Wallet_Type :: enum {
	Legacy,          // Original Hound SHA-256 method (incompatible with other wallets)
	BIP44_Standard,  // m/44'/501'/{account}'/0' (Phantom, Solflare, Ledger, Backpack)
	BIP44_Change,    // m/44'/501'/{account}' (Trust Wallet)
	Solana_CLI,      // m/44'/501' (solana-keygen default)
}

// Wallet represents a Solana wallet address to watch
Wallet :: struct {
	address:         string,      // Base58-encoded Solana address
	label:           string,      // User-friendly name
	is_primary:      bool,        // Primary wallet for display
	wallet_type:     Wallet_Type, // Derivation standard used
	derivation_path: string,      // Full path (e.g., "m/44'/501'/0'/0'")
	account_index:   int,         // Account index for multi-account (default 0)
}

// TokenConfig represents the complete token configuration file
TokenConfig :: struct {
	version: string,
	tokens:  []Token,
	wallets: []Wallet, // Watch-only wallet addresses
}

// ============================================================================
// Wallet Type Helper Functions
// ============================================================================

// Parse wallet type from string (for database retrieval)
//
// ASSERTION: Returns Legacy as fallback for unknown types (backward compatibility)
parse_wallet_type :: proc(type_str: string) -> (Wallet_Type, bool) {
	switch type_str {
	case "Legacy":
		return .Legacy, true
	case "BIP44_Standard":
		return .BIP44_Standard, true
	case "BIP44_Change":
		return .BIP44_Change, true
	case "Solana_CLI":
		return .Solana_CLI, true
	}
	return .Legacy, false // Default to Legacy for unknown (backward compatibility)
}

// Get derivation path for wallet type
//
// ASSERTION: account_index must be >= 0
get_derivation_path :: proc(wallet_type: Wallet_Type, account_index: int) -> string {
	assert(account_index >= 0, "Account index must be non-negative")

	switch wallet_type {
	case .BIP44_Standard:
		// Phantom, Solflare, Ledger, Backpack
		return fmt.tprintf("m/44'/501'/%d'/0'", account_index)
	case .BIP44_Change:
		// Trust Wallet
		return fmt.tprintf("m/44'/501'/%d'", account_index)
	case .Solana_CLI:
		// solana-keygen default (no account support)
		return "m/44'/501'"
	case .Legacy:
		// Original Hound SHA-256 method
		return "legacy-sha256"
	}
	return "" // Should never reach here
}

// ============================================================================
// Token Metadata Helper Functions
// ============================================================================

// get_token_by_symbol searches for a token by symbol (case-insensitive)
get_token_by_symbol :: proc(config: ^TokenConfig, symbol: string) -> (Token, bool) {
	lower_symbol := strings.to_lower(symbol)
	defer delete(lower_symbol)

	for token in config.tokens {
		token_symbol_lower := strings.to_lower(token.symbol)
		defer delete(token_symbol_lower)

		if token_symbol_lower == lower_symbol {
			return token, true
		}
	}
	return Token{}, false
}

// get_token_mint returns the token's mint address (contract_address)
get_token_mint :: proc(token: Token) -> string {
	return token.contract_address
}

// get_token_decimals returns the token's decimals, defaulting to 9 (SOL default)
// TODO: Add decimals column to database schema (Phase 3)
get_token_decimals :: proc(token: Token) -> int {
	if token.decimals > 0 {
		return token.decimals
	}

	// Hardcoded decimals for known tokens (temporary fix until database migration)
	switch token.contract_address {
	case "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": // USDC
		return 6
	case "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": // USDT
		return 6
	case "So11111111111111111111111111111111111111112": // SOL (native)
		return 9
	}

	return 9 // Default to 9 for SOL-like tokens
}

// ============================================================================
// Swap Types (Phase 2: Quote & Execution)
// ============================================================================

// Swap command flags (parallel to WalletFlags)
SwapFlags :: struct {
	dry_run:      bool,   // --dry-run: Simulation mode
	slippage_bps: int,    // --slippage: Custom slippage (default: 50)
	wallet_addr:  string, // --wallet: Specific wallet to use
}

// Single step in Jupiter's route plan
RouteStep :: struct {
	dex_label:     string,    // "Orca Whirlpool", "Raydium CLMM"
	input_mint:    string,
	output_mint:   string,
	input_amount:  u64,
	output_amount: u64,
	fee_amount:    u64,
	percent:       int,       // Percentage of total route (for splits)
}

// Swap quote from Jupiter API v6
SwapQuote :: struct {
	// Input/Output
	input_mint:    string,
	input_symbol:  string,
	input_amount:  f64,       // Human-readable (e.g., 1.0 SOL)
	input_lamports: u64,      // Raw amount (e.g., 1000000000)

	output_mint:   string,
	output_symbol: string,
	output_amount: f64,       // Estimated output
	output_lamports: u64,

	// Rate & Slippage
	rate:          f64,       // Exchange rate (output/input)
	slippage_bps:  int,       // Slippage tolerance (basis points)
	minimum_out:   f64,       // Minimum output after slippage

	// Routing
	route_plan:    []RouteStep,
	primary_dex:   string,    // Main DEX label (e.g., "Orca Whirlpool")

	// Impact & Fees
	price_impact_pct: f64,    // Price impact percentage
	network_fee_sol:  f64,    // Estimated network fee in SOL

	// Metadata
	fetched_at:    i64,       // Unix timestamp when quote was fetched
	expires_at:    i64,       // Unix timestamp when quote expires (fetched_at + 90s)

	// Raw Response (for Phase 3 transaction building)
	jupiter_response: string, // Store full JSON string for /swap endpoint
}

// Swap transaction execution result (Phase 3)
SwapTransactionResult :: struct {
	// Transaction identifiers
	signature:       string,      // Base58-encoded transaction signature
	slot:            u64,          // Confirmed slot number
	block_time:      i64,          // Unix timestamp
	status:          string,       // "confirmed", "finalized", or "failed"
	error_message:   string,       // Empty if successful

	// Swap details (actual amounts from blockchain)
	input_amount:    f64,          // Actual input amount
	output_amount:   f64,          // Actual output amount
	price_impact:    f64,          // Actual price impact
	slippage_actual: f64,          // Actual slippage percentage

	// Fees (in lamports)
	network_fee:     u64,          // Transaction fee
	priority_fee:    u64,          // Priority fee

	// Metadata
	dex:             string,       // DEX used (from quote)
	input_mint:      string,
	input_symbol:    string,
	output_mint:     string,
	output_symbol:   string,
}
