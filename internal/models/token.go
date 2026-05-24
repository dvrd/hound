package models

// Token represents a tracked token in the database.
type Token struct {
	Symbol          string
	Name            string
	ContractAddress string
	Chain           string
	Decimals        int
	Pools           []PoolInfo
	IsQuoteToken    bool
	USDPrice        float64
}

// PoolInfo represents a DEX liquidity pool for a token.
type PoolInfo struct {
	Dex          string // "raydium", "orca", etc.
	PoolAddress  string
	QuoteToken   string // "sol", "usdc"
	PoolType     string // "amm_v4", "whirlpool"
	LiquidityUSD float64
	Volume24h    float64
	FeePercent   float64
	DiscoveredAt int64 // 0 for manually added
}

// PoolStats holds aggregate pool statistics for a token.
type PoolStats struct {
	PoolCount      int
	TotalLiquidity float64
}

// TopHolder represents a top token holder.
type TopHolder struct {
	Address      string
	Balance      float64
	OwnershipPct float64
}

// TokenExtendedInfo contains comprehensive token data from multiple sources.
type TokenExtendedInfo struct {
	Symbols      []string
	Name         string
	Network      string
	MintAddress  string
	PriceUSD     float64
	MarketCap    float64
	FDV          float64
	LiquidityUSD float64
	TotalSupply  float64
	Decimals     int
	Volume24h    float64
	Txns24h      int
	Buys24h      int
	Sells24h     int
	PriceChange  PriceChanges
	TopHolders   []TopHolder
	PriceHistory []PriceCandle
	CreatedAt    int64
	IsActive     bool
}

// PriceChanges holds price change percentages over different time periods.
type PriceChanges struct {
	M5  float64 // 5 minutes
	H1  float64 // 1 hour
	H6  float64 // 6 hours
	H24 float64 // 24 hours
}

// PriceCandle represents a single OHLCV candlestick data point.
type PriceCandle struct {
	Timestamp int64   // Unix timestamp in seconds
	Open      float64 // Opening price
	High      float64 // Highest price
	Low       float64 // Lowest price
	Close     float64 // Closing price
	Volume    float64 // Trading volume
}

// DexScreenerResponse is the top-level response from the DexScreener API.
type DexScreenerResponse struct {
	Pairs []PairData `json:"pairs"`
}

// PairData represents a single trading pair from DexScreener.
type PairData struct {
	ChainID       string          `json:"chainId"`
	DexID         string          `json:"dexId"`
	PairAddress   string          `json:"pairAddress"`
	Labels        []string        `json:"labels"`
	BaseToken     PairToken       `json:"baseToken"`
	QuoteToken    PairToken       `json:"quoteToken"`
	PriceNative   string          `json:"priceNative"`
	PriceUSD      string          `json:"priceUsd"`
	Txns          PairTxns        `json:"txns"`
	Volume        PairVolume      `json:"volume"`
	PriceChange   PairPriceChange `json:"priceChange"`
	Liquidity     PairLiquidity   `json:"liquidity"`
	FDV           float64         `json:"fdv"`
	MarketCap     float64         `json:"marketCap"`
	PairCreatedAt int64           `json:"pairCreatedAt"`
}

// PairToken represents a token in a DexScreener pair.
type PairToken struct {
	Address  string `json:"address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals,omitempty"`
}

// PairTxns holds transaction counts.
type PairTxns struct {
	H24 PairTxnCounts `json:"h24"`
}

// PairTxnCounts holds buy/sell counts.
type PairTxnCounts struct {
	Buys  int `json:"buys"`
	Sells int `json:"sells"`
}

// PairVolume holds volume data.
type PairVolume struct {
	H24 float64 `json:"h24"`
}

// PairPriceChange holds price change percentages.
type PairPriceChange struct {
	M5  float64 `json:"m5"`
	H1  float64 `json:"h1"`
	H6  float64 `json:"h6"`
	H24 float64 `json:"h24"`
}

// PairLiquidity holds liquidity data.
type PairLiquidity struct {
	USD   float64 `json:"usd"`
	Base  float64 `json:"base"`
	Quote float64 `json:"quote"`
}

// PriceData holds a resolved price with 24h change.
type PriceData struct {
	PriceUSD  float64
	Change24h float64
}

// GetTokenDecimals returns the decimals for a token, with hardcoded fallbacks.
func GetTokenDecimals(token Token) int {
	if token.Decimals > 0 {
		return token.Decimals
	}

	switch token.Symbol {
	case "USDC", "usdc":
		return 6
	case "USDT", "usdt":
		return 6
	case "SOL", "sol":
		return 9
	default:
		return 9
	}
}


