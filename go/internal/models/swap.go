package models

import (
	"encoding/json"
	"time"
)

const (
	// QuoteTTL is the maximum age of a swap quote before it expires.
	QuoteTTL = 90 * time.Second
)

// SwapFlags holds CLI flags for swap commands.
type SwapFlags struct {
	DryRun      bool
	SlippageBps int
	WalletAddr  string
}

// RouteStep represents a single hop in a swap route.
type RouteStep struct {
	DexLabel   string
	InputMint  string
	OutputMint string
	InAmount   string
	OutAmount  string
	FeeAmount  string
	FeeMint    string
	Percent    int
}

// SwapQuote represents a complete swap quote from Jupiter.
type SwapQuote struct {
	InputMint      string
	OutputMint     string
	InAmount       string
	OutAmount      string
	Rate           float64
	SlippageBps    int
	PriceImpactPct float64
	RoutePlan      []RouteStep
	NetworkFee     float64
	FetchedAt      time.Time
	RawResponse    json.RawMessage // Raw Jupiter response for signing

	// Human-readable fields populated after quote
	InputSymbol  string
	OutputSymbol string
	InputAmount  float64
	OutputAmount float64
	MinReceived  float64
}

// IsExpired returns true if the quote is older than QuoteTTL (90 seconds).
func (q *SwapQuote) IsExpired() bool {
	return time.Since(q.FetchedAt) > QuoteTTL
}

// SwapTransactionResult represents the result of executing a swap.
type SwapTransactionResult struct {
	Signature       string
	Slot            int64
	BlockTime       int64
	Status          string // "finalized", "confirmed", "failed"
	ActualInAmount  float64
	ActualOutAmount float64
	Fees            SwapFees
	Dex             string
	ErrorMessage    string
}

// SwapFees holds fee information for a swap transaction.
type SwapFees struct {
	NetworkFee  float64
	PriorityFee float64
}

// SwapHistoryEntry represents a stored swap transaction in the database.
type SwapHistoryEntry struct {
	ID            int64
	WalletAddress string
	InputMint     string
	OutputMint    string
	InputSymbol   string
	OutputSymbol  string
	InputAmount   float64
	OutputAmount  float64
	PriceImpact   float64
	SlippageBps   int
	Signature     string
	Status        string
	Dex           string
	NetworkFee    float64
	PriorityFee   float64
	ErrorMessage  string
	CreatedAt     int64
}
