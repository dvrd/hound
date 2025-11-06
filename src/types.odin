package main

// Error types for the application
ErrorType :: enum {
	None,
	TokenNotFound,
	NetworkError,
	APITimeout,
	InvalidResponse,
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
