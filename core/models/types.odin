package models

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
	pools:            []PoolInfo, // Liquidity pools for on-chain pricing
	is_quote_token:   bool, // True if this is a quote token (SOL, USDC)
	usd_price:        f64, // USD price for quote tokens
}

// Wallet represents a Solana wallet address to watch
Wallet :: struct {
	address:    string, // Base58-encoded Solana address
	label:      string, // User-friendly name
	is_primary: bool,   // Primary wallet for display
}

// TokenConfig represents the complete token configuration file
TokenConfig :: struct {
	version: string,
	tokens:  []Token,
	wallets: []Wallet, // Watch-only wallet addresses
}
