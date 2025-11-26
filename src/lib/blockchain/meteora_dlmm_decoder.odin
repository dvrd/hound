#+feature global-context
package blockchain

import "core:log"
import "core:math"
import "core:mem"

// =============================================================================
// METEORA DLMM (Dynamic Liquidity Market Maker) DECODER
// =============================================================================
// Decodes Meteora DLMM LbPair (Liquidity Book Pair) pool state from on-chain data
//
// Meteora DLMM uses a bin-based pricing model similar to Uniswap V3's
// concentrated liquidity, where liquidity is distributed across discrete
// price bins. Each bin has a unique price determined by:
//
//   price = (1 + bin_step / 10_000) ^ (active_id - 8388608)
//
// where:
//   - bin_step: The granularity of price movement (e.g., 1 = 0.01% steps)
//   - active_id: Current active bin ID (24-bit signed integer)
//   - 8388608: Center bin where price = 1.0 (2^23)
//
// References:
// - Meteora DLMM Docs: https://docs.meteora.ag/dlmm-pool/dynamic-liquidity-market-maker
// - LbPair Account: https://github.com/MeteoraAg/dlmm-sdk
// =============================================================================

// LbPair (Liquidity Book Pair) State
//
// Account size: ~1024 bytes (varies based on padding)
// Discriminator: First 8 bytes identify account type
//
// Key fields:
// - active_id: Current active bin (24-bit signed, offset by 8388608)
// - bin_step: Price granularity in basis points (e.g., 1 = 0.01%)
// - token_x_mint: Token X (base token) mint address (32 bytes)
// - token_y_mint: Token Y (quote token) mint address (32 bytes)
// - token_x_decimals: Token X decimal precision (u8)
// - token_y_decimals: Token Y decimal precision (u8)
MeteoraDLMMPoolState :: struct {
	// Core pricing fields
	active_id:        i32,     // Current active bin ID (signed, centered at 8388608)
	bin_step:         u16,     // Price step in basis points (1 = 0.01%)

	// Token metadata
	token_x_mint:     [32]u8,  // Base token mint
	token_y_mint:     [32]u8,  // Quote token mint
	token_x_decimals: u8,      // Base token decimals
	token_y_decimals: u8,      // Quote token decimals

	// Pool metadata
	protocol_fee:     u16,     // Protocol fee in basis points
	base_fee:         u16,     // Base trading fee in basis points
}

// Decode Meteora DLMM LbPair account data
//
// ASSERTION 1: Validate data length >= minimum expected size
// ASSERTION 2: Validate bin_step > 0
// ASSERTION 3: Validate active_id is within reasonable bounds
//
// Returns: (MeteoraDLMMPoolState, success: bool)
decode_meteora_dlmm_pool :: proc(data: []u8) -> (MeteoraDLMMPoolState, bool) {
	// ASSERTION 1: Minimum size check
	// Meteora DLMM accounts are ~1024 bytes, but we only need the first ~256 bytes
	MIN_SIZE :: 256
	if len(data) < MIN_SIZE {
		log.errorf("Meteora DLMM pool data too small: %d bytes (expected >= %d)", len(data), MIN_SIZE)
		return {}, false
	}

	log.debugf("Decoding Meteora DLMM pool: %d bytes", len(data))

	state: MeteoraDLMMPoolState

	// Account layout (empirically determined from on-chain data):
	// Bytes 0-7:   Discriminator (account type identifier)
	// Bytes 8-15:  Unknown/padding
	// Bytes 16-17: Bin Step (u16)
	// Bytes 18-27: Unknown fields
	// Bytes 28-31: Active ID (i32)
	// Bytes 32-35: Unknown
	// Note: Token mints and decimals are at different offsets than initially assumed
	// TODO: Determine exact locations of token_x_mint, token_y_mint, decimals

	// Read bin_step at offset 16
	if 16 + 2 <= len(data) {
		state.bin_step = read_u16_le(data, 16)
	} else {
		log.error("Failed to read bin_step at offset 16")
		return {}, false
	}

	// Read active_id at offset 28
	if 28 + 4 <= len(data) {
		state.active_id = read_i32_le(data, 28)
	} else {
		log.error("Failed to read active_id at offset 28")
		return {}, false
	}

	// TODO: Find correct offsets for these fields
	// For now, set reasonable defaults
	state.token_x_decimals = 9  // Default to SOL-like
	state.token_y_decimals = 9
	state.protocol_fee = 0
	state.base_fee = 0

	// ASSERTION 2: Validate bin_step
	if state.bin_step == 0 {
		log.error("Invalid bin_step: must be > 0")
		return {}, false
	}

	// ASSERTION 3: Validate active_id (should be within reasonable range)
	// Meteora uses 24-bit signed integers, range: [-8388608, 8388607]
	// But in practice, active_id should be close to center (8388608 = price 1.0)
	MAX_BIN_OFFSET :: 5_000_000  // Allow ±5M bins from center
	if state.active_id < -MAX_BIN_OFFSET || state.active_id > MAX_BIN_OFFSET {
		log.warnf("Active ID seems unusual: %d (far from center)", state.active_id)
		// Don't fail - just warn (some pools might have extreme prices)
	}

	log.debugf("Decoded Meteora DLMM: active_id=%d, bin_step=%d, decimals=(%d,%d)",
		state.active_id, state.bin_step,
		state.token_x_decimals, state.token_y_decimals)

	return state, true
}

// Calculate price from Meteora DLMM pool state
//
// ASSERTION 1: Validate bin_step > 0
//
// Formula:
//   price = (1 + bin_step / 10_000) ^ active_id
//
// Note: Unlike traditional AMMs, Meteora doesn't need to adjust for
// decimals in the price calculation because the active_id already
// represents the price ratio between token X and Y.
//
// Returns: Price of token X in terms of token Y (quote token)
calculate_meteora_dlmm_price :: proc(state: MeteoraDLMMPoolState) -> f64 {
	assert(state.bin_step > 0, "Bin step must be positive")

	// Calculate the price multiplier per bin
	// bin_step is in basis points, so divide by 10_000
	price_per_bin := 1.0 + f64(state.bin_step) / 10_000.0

	// Calculate price = price_per_bin ^ active_id using logarithms to avoid overflow
	// price = exp(active_id * ln(price_per_bin))
	log_price := f64(state.active_id) * math.ln(price_per_bin)

	// Check if log_price is too large (would cause overflow in exp)
	MAX_LOG_PRICE :: 700.0  // exp(700) ≈ 10^304, beyond this we get inf
	if math.abs(log_price) > MAX_LOG_PRICE {
		log.errorf("Meteora price calculation would overflow: log_price=%.2f (bin_step=%d, active_id=%d)",
			log_price, state.bin_step, state.active_id)
		return 0.0  // Return 0 to indicate failure
	}

	price := math.exp(log_price)

	// Check for infinity or invalid values
	if math.is_inf(price, 0) || math.is_nan(price) {
		log.errorf("Meteora price calculation resulted in invalid value: log_price=%.2f",
			log_price)
		return 0.0
	}

	// Adjust for decimals
	decimals_diff := i32(state.token_y_decimals) - i32(state.token_x_decimals)
	decimal_adjustment := math.pow(10.0, f64(decimals_diff))

	final_price := price * decimal_adjustment

	// Final check for infinity
	if math.is_inf(final_price, 0) || math.is_nan(final_price) {
		log.errorf("Meteora price after decimal adjustment is invalid: %.18e * %.6f",
			price, decimal_adjustment)
		return 0.0
	}

	log.debugf("Meteora price calculation: bin_step=%d, active_id=%d, log_price=%.2f, raw_price=%.18e, decimal_adj=%.6f, final=%.18f",
		state.bin_step, state.active_id, log_price, price, decimal_adjustment, final_price)

	return final_price
}
