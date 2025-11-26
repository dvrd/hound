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

	// Account layout (offsets from Meteora DLMM SDK):
	// Bytes 0-7:   Discriminator (account type identifier)
	// Bytes 8-39:  Token X Mint (32 bytes)
	// Bytes 40-71: Token Y Mint (32 bytes)
	// Bytes 72-79: Bin Step (u16) + padding
	// Bytes 80-83: Active ID (i32)
	// Bytes 84-85: Token X Decimals (u8) + Token Y Decimals (u8)
	// Bytes 86-89: Protocol Fee (u16) + Base Fee (u16)

	offset := 8  // Skip discriminator

	// Token X Mint (32 bytes)
	if offset + 32 <= len(data) {
		mem.copy(&state.token_x_mint[0], &data[offset], 32)
		offset += 32
	} else {
		log.error("Failed to read token_x_mint")
		return {}, false
	}

	// Token Y Mint (32 bytes)
	if offset + 32 <= len(data) {
		mem.copy(&state.token_y_mint[0], &data[offset], 32)
		offset += 32
	} else {
		log.error("Failed to read token_y_mint")
		return {}, false
	}

	// Bin Step (u16)
	if offset + 2 <= len(data) {
		state.bin_step = read_u16_le(data, offset)
		offset += 8  // Include padding
	} else {
		log.error("Failed to read bin_step")
		return {}, false
	}

	// Active ID (i32)
	if offset + 4 <= len(data) {
		state.active_id = read_i32_le(data, offset)
		offset += 4
	} else {
		log.error("Failed to read active_id")
		return {}, false
	}

	// Token Decimals (u8 + u8)
	if offset + 2 <= len(data) {
		state.token_x_decimals = data[offset]
		state.token_y_decimals = data[offset + 1]
		offset += 2
	} else {
		log.error("Failed to read token decimals")
		return {}, false
	}

	// Fees (u16 + u16)
	if offset + 4 <= len(data) {
		state.protocol_fee = read_u16_le(data, offset)
		state.base_fee = read_u16_le(data, offset + 2)
		offset += 4
	} else {
		log.error("Failed to read fees")
		return {}, false
	}

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

	// Calculate price = price_per_bin ^ active_id
	// Use math.pow for arbitrary exponentiation
	price := math.pow(price_per_bin, f64(state.active_id))

	// Adjust for decimals
	decimals_diff := i32(state.token_y_decimals) - i32(state.token_x_decimals)
	decimal_adjustment := math.pow(10.0, f64(decimals_diff))

	final_price := price * decimal_adjustment

	log.debugf("Meteora price calculation: base=%.6f, exponent=%d, raw_price=%.18f, decimal_adj=%.6f, final=%.18f",
		price_per_bin, state.active_id, price, decimal_adjustment, final_price)

	return final_price
}
