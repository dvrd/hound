package main

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

	// Parse errors (data issues)
	InvalidResponse,      // Malformed JSON or unexpected structure

	// Config errors (configuration issues)
	TokenNotConfigured,   // Symbol not found in config
	ConfigNotFound,       // Config file doesn't exist
	ConfigParseError,     // Failed to parse config JSON
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
