#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:math"

// Orca Whirlpool account structure (653 bytes without discriminator, 661 with)
// Concentrated Liquidity Market Maker (CLMM) pool state
//
// Structure based on official Orca Whirlpool documentation:
// https://orca-so.github.io/whirlpools/
// https://github.com/orca-so/whirlpools
//
// Critical fields for price calculation:
// - sqrt_price (Q64.64 fixed-point format) at offset 65
// - token decimals from mint accounts
OrcaWhirlpoolState :: struct {
	// Configuration (offsets 0-48)
	whirlpools_config: [32]u8, // Offset 8 (after 8-byte discriminator)
	whirlpool_bump:    [1]u8, // Offset 40
	tick_spacing:      u16, // Offset 41
	tick_spacing_seed: [2]u8, // Offset 43
	fee_rate:          u16, // Offset 45 (in hundredths of a basis point)
	protocol_fee_rate: u16, // Offset 47 (in basis points)

	// Liquidity and Pricing (offsets 49-84) - CRITICAL
	liquidity:          u128, // Offset 49 (16 bytes)
	sqrt_price:         u128, // Offset 65 (16 bytes, Q64.64 format)
	tick_current_index: i32, // Offset 81 (4 bytes)

	// Protocol Fees (offsets 85-100)
	protocol_fee_owed_a: u64, // Offset 85
	protocol_fee_owed_b: u64, // Offset 93

	// Token A Details (offsets 101-180)
	token_mint_a:       [32]u8, // Offset 101
	token_vault_a:      [32]u8, // Offset 133
	fee_growth_global_a: u128, // Offset 165 (Q64.64)

	// Token B Details (offsets 181-260)
	token_mint_b:       [32]u8, // Offset 181
	token_vault_b:      [32]u8, // Offset 213
	fee_growth_global_b: u128, // Offset 245 (Q64.64)

	// Rewards (offsets 261-652)
	reward_last_updated_timestamp: u64, // Offset 261
	// reward_infos: [3]WhirlpoolRewardInfo (384 bytes) - not decoded for price fetching
}

// Decode Orca Whirlpool CLMM pool from 653 bytes (or 661 with discriminator)
//
// ASSERTION 1: TigerBeetle safety - validate input buffer size
// ASSERTION 2: TigerBeetle safety - validate sqrt_price is within bounds
// ASSERTION 3: TigerBeetle safety - validate tick is within bounds
decode_orca_whirlpool :: proc(data: []u8) -> (OrcaWhirlpoolState, bool) {
	// ASSERTION 1: Validate data length (must be 653 or 661 bytes)
	assert(
		len(data) == 653 || len(data) == 661,
		"Whirlpool account data must be 653 bytes (without discriminator) or 661 bytes (with discriminator)",
	)

	log.debugf("Decoding Orca Whirlpool account (%d bytes)", len(data))

	// Handle discriminator if present (8 bytes)
	offset := 0
	if len(data) == 661 {
		// Skip 8-byte Anchor discriminator
		offset = 8
		log.debug("Skipping 8-byte Anchor discriminator")
	}

	// Validate remaining data is 653 bytes
	if len(data) - offset != 653 {
		log.errorf("Invalid Whirlpool data length: %d (expected 653 after discriminator)", len(data) - offset)
		return {}, false
	}

	pool: OrcaWhirlpoolState

	// Read configuration fields
	pool.whirlpools_config = read_pubkey(data, offset + 0)
	pool.whirlpool_bump[0] = data[offset + 32]
	pool.tick_spacing = read_u16_le(data, offset + 33)
	pool.tick_spacing_seed[0] = data[offset + 35]
	pool.tick_spacing_seed[1] = data[offset + 36]
	pool.fee_rate = read_u16_le(data, offset + 37)
	pool.protocol_fee_rate = read_u16_le(data, offset + 39)

	// Read liquidity and pricing fields (CRITICAL)
	pool.liquidity = read_u128_le(data, offset + 41)
	pool.sqrt_price = read_u128_le(data, offset + 57)
	pool.tick_current_index = read_i32_le(data, offset + 73)

	log.debugf("sqrt_price: %v, tick: %d, liquidity: %v", pool.sqrt_price, pool.tick_current_index, pool.liquidity)

	// ASSERTION 2: Validate sqrt_price bounds (MIN_SQRT_PRICE to MAX_SQRT_PRICE)
	// MIN: 4295048016, MAX: 79226673515401279992447579055
	MIN_SQRT_PRICE: u128 = 4295048016
	MAX_SQRT_PRICE: u128 = 79226673515401279992447579055
	assert(
		pool.sqrt_price >= MIN_SQRT_PRICE && pool.sqrt_price <= MAX_SQRT_PRICE,
		fmt.tprintf("sqrt_price %v outside valid range [%v, %v]", pool.sqrt_price, MIN_SQRT_PRICE, MAX_SQRT_PRICE),
	)

	// ASSERTION 3: Validate tick bounds (MIN_TICK to MAX_TICK)
	// Range: -443636 to 443636
	MIN_TICK: i32 = -443636
	MAX_TICK: i32 = 443636
	assert(
		pool.tick_current_index >= MIN_TICK && pool.tick_current_index <= MAX_TICK,
		fmt.tprintf("tick_current_index %d outside valid range [%d, %d]", pool.tick_current_index, MIN_TICK, MAX_TICK),
	)

	// Read protocol fees
	pool.protocol_fee_owed_a = read_u64_le(data, offset + 77)
	pool.protocol_fee_owed_b = read_u64_le(data, offset + 85)

	// Read Token A details
	pool.token_mint_a = read_pubkey(data, offset + 93)
	pool.token_vault_a = read_pubkey(data, offset + 125)
	pool.fee_growth_global_a = read_u128_le(data, offset + 157)

	// Read Token B details
	pool.token_mint_b = read_pubkey(data, offset + 173)
	pool.token_vault_b = read_pubkey(data, offset + 205)
	pool.fee_growth_global_b = read_u128_le(data, offset + 237)

	// Read reward timestamp
	pool.reward_last_updated_timestamp = read_u64_le(data, offset + 253)

	// Note: reward_infos (384 bytes) at offset 261 not decoded for price fetching

	log.info("Orca Whirlpool account decoded successfully")
	return pool, true
}

// Convert Q64.64 sqrt_price to real price with decimal adjustment
//
// Formula: price = (sqrt_price / 2^64)^2 * 10^(decimals_a - decimals_b)
//
// Q64.64 Format:
// - 64 bits for integer part, 64 bits for fractional part
// - Scale factor: 2^64 (18446744073709551616)
//
// ASSERTION 1: Validate decimals are reasonable (0-18)
// ASSERTION 2: Validate calculated price is non-negative
sqrt_price_to_price :: proc(sqrt_price_x64: u128, decimals_a: u8, decimals_b: u8) -> f64 {
	log.debugf("Converting sqrt_price %v with decimals (%d, %d)", sqrt_price_x64, decimals_a, decimals_b)

	// ASSERTION 1: Validate decimals (typical range: 0-18)
	assert(
		decimals_a <= 18 && decimals_b <= 18,
		fmt.tprintf("Token decimals out of range: decimals_a=%d, decimals_b=%d", decimals_a, decimals_b),
	)

	// Step 1: Convert u128 sqrt_price to f64
	// Note: Potential precision loss for very large numbers, but acceptable for price calculation
	sqrt_price_f64 := f64(sqrt_price_x64)

	// Step 2: Calculate (sqrt_price / 2^64)^2
	// Equivalent to: (sqrt_price)^2 / 2^128
	SCALE_FACTOR_F64: f64 = 18446744073709551616.0 // 2^64
	sqrt_price_real := sqrt_price_f64 / SCALE_FACTOR_F64
	price_raw := sqrt_price_real * sqrt_price_real

	log.debugf("sqrt_price_real: %.18f, price_raw: %.18f", sqrt_price_real, price_raw)

	// Step 3: Adjust for token decimals
	// price_adjusted = price_raw * 10^(decimals_a - decimals_b)
	decimal_diff := i32(decimals_a) - i32(decimals_b)
	decimal_adjustment := math.pow(10.0, f64(decimal_diff))
	price_adjusted := price_raw * decimal_adjustment

	log.debugf("decimal_diff: %d, adjustment: %.6f, price_adjusted: %.18f", decimal_diff, decimal_adjustment, price_adjusted)

	// ASSERTION 2: Validate price is non-negative
	assert(price_adjusted >= 0, "Calculated price cannot be negative")

	return price_adjusted
}

// Read little-endian u128 from byte array at offset
read_u128_le :: proc(data: []u8, offset: int) -> u128 {
	if offset + 16 > len(data) {
		log.errorf("Buffer overflow: read_u128_le at offset %d, data length %d", offset, len(data))
		return 0
	}

	// Little-endian: least significant byte first
	// Build u128 from two u64s
	low := u64(data[offset]) | u64(data[offset + 1]) << 8 | u64(data[offset + 2]) << 16 |
		u64(data[offset + 3]) << 24 | u64(data[offset + 4]) << 32 |
		u64(data[offset + 5]) << 40 | u64(data[offset + 6]) << 48 |
		u64(data[offset + 7]) << 56

	high := u64(data[offset + 8]) | u64(data[offset + 9]) << 8 |
		u64(data[offset + 10]) << 16 | u64(data[offset + 11]) << 24 |
		u64(data[offset + 12]) << 32 | u64(data[offset + 13]) << 40 |
		u64(data[offset + 14]) << 48 | u64(data[offset + 15]) << 56

	// Combine: high bits in upper 64, low bits in lower 64
	return u128(high) << 64 | u128(low)
}

// Read little-endian i32 from byte array at offset
read_i32_le :: proc(data: []u8, offset: int) -> i32 {
	if offset + 4 > len(data) {
		log.errorf("Buffer overflow: read_i32_le at offset %d, data length %d", offset, len(data))
		return 0
	}

	// Little-endian: least significant byte first
	unsigned := u32(data[offset]) | u32(data[offset + 1]) << 8 | u32(data[offset + 2]) << 16 |
		u32(data[offset + 3]) << 24

	// Transmute to signed i32
	return i32(unsigned)
}

// Read little-endian u16 from byte array at offset
read_u16_le :: proc(data: []u8, offset: int) -> u16 {
	if offset + 2 > len(data) {
		log.errorf("Buffer overflow: read_u16_le at offset %d, data length %d", offset, len(data))
		return 0
	}

	// Little-endian: least significant byte first
	return u16(data[offset]) | u16(data[offset + 1]) << 8
}
