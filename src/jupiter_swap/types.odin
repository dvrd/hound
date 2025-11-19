package jupiter_swap

import "core:time"

// Jupiter quote response from GET /v6/quote
// Reference: PRPs/ai_docs/jupiter-api-v6.md
JupiterQuote :: struct {
	input_mint:             string,
	output_mint:            string,
	in_amount:              string, // Base units as string
	out_amount:             string, // Base units as string
	other_amount_threshold: string, // Minimum output with slippage
	swap_mode:              string, // "ExactIn" or "ExactOut"
	slippage_bps:           u16, // Basis points (50 = 0.5%)
	price_impact_pct:       string, // Percentage as string
	route_plan:             []RoutePlanStep,

	// Caching metadata
	cached_at:              time.Time,
	is_valid:               bool,
}

// Step in the routing plan showing which DEXes are used
RoutePlanStep :: struct {
	swap_info: SwapInfo,
	percent:   u8, // Percentage of amount routed through this path
}

// Information about a single swap in the route
SwapInfo :: struct {
	amm_key:     string,
	label:       string, // "Raydium", "Orca", "Whirlpool", etc.
	input_mint:  string,
	output_mint: string,
	in_amount:   string,
	out_amount:  string,
	fee_amount:  string,
	fee_mint:    string,
}

// Jupiter swap request for POST /v6/swap
// Reference: PRPs/ai_docs/jupiter-api-v6.md
JupiterSwapRequest :: struct {
	quote_response:              JupiterQuote,
	user_public_key:             string,
	wrap_and_unwrap_sol:         bool,
	dynamic_compute_unit_limit:  bool,
	prioritization_fee_lamports: string, // "auto" or specific value
}

// Jupiter swap response containing unsigned transaction
JupiterSwapResponse :: struct {
	swap_transaction:        string, // Base64-encoded transaction
	last_valid_block_height: u64,
}

// UI state for swap dialog flow
SwapState :: struct {
	source_mint:    string,
	source_symbol:  string,
	source_amount:  f64,
	source_balance: f64,

	dest_mint:      string,
	dest_symbol:    string,
	dest_amount:    f64,

	quote:          Maybe(JupiterQuote),
	transaction:    Maybe(string), // Base64-encoded transaction

	loading:        bool,
	error:          Maybe(string),
}
