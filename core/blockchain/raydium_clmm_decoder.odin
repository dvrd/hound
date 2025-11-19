#+feature global-context
package blockchain

import "core:fmt"
import "core:log"
import "core:math"

// Raydium CLMM PoolState structure
// Based on: https://github.com/raydium-io/raydium-clmm/blob/master/programs/amm/src/states/pool.rs
//
// Account structure: #[repr(C, packed)] with zero_copy
// Total size: ~1544 bytes (with 8-byte Anchor discriminator)
//
// Critical fields for price calculation:
// - sqrt_price_x64 (Q64.64 fixed-point format)
// - mint_decimals_0 and mint_decimals_1 (embedded in pool state)
// - tick_current (log base 1.0001 of price)
RaydiumCLMMPoolState :: struct {
	// Configuration (first ~237 bytes before liquidity)
	bump:             [1]u8,      // Offset 8 (after discriminator)
	amm_config:       [32]u8,     // Offset 9
	owner:            [32]u8,     // Offset 41
	token_mint_0:     [32]u8,     // Offset 73
	token_mint_1:     [32]u8,     // Offset 105
	token_vault_0:    [32]u8,     // Offset 137
	token_vault_1:    [32]u8,     // Offset 169
	observation_key:  [32]u8,     // Offset 201
	mint_decimals_0:  u8,         // Offset 233 - CRITICAL (embedded decimal)
	mint_decimals_1:  u8,         // Offset 234 - CRITICAL (embedded decimal)
	tick_spacing:     u16,        // Offset 235

	// Price and Liquidity (CRITICAL for price calculation)
	liquidity:        u128,       // Offset 237 (16 bytes)
	sqrt_price_x64:   u128,       // Offset 253 (16 bytes, Q64.64 format)
	tick_current:     i32,        // Offset 269 (4 bytes)
	padding3:         u16,        // Offset 273
	padding4:         u16,        // Offset 275

	// Fee tracking (not needed for basic price fetching)
	fee_growth_global_0_x64: u128, // Offset 277
	fee_growth_global_1_x64: u128, // Offset 293
	protocol_fees_token_0:   u64,  // Offset 309
	protocol_fees_token_1:   u64,  // Offset 317

	// Swap statistics (not needed for basic price fetching)
	swap_in_amount_token_0:  u128, // Offset 325
	swap_out_amount_token_1: u128, // Offset 341
	swap_in_amount_token_1:  u128, // Offset 357
	swap_out_amount_token_0: u128, // Offset 373

	// Status and timing
	status:                  u8,   // Offset 389
	// Note: Additional fields exist (reward_infos, tick_array_bitmap, etc.)
	// but are not needed for price calculation
}

// Decode Raydium CLMM PoolState from account data
//
// Account includes 8-byte Anchor discriminator prefix
// Expected sizes: 1544 bytes (with discriminator) or 1536 bytes (without)
//
// ASSERTION 1: TigerBeetle safety - validate input buffer size
// ASSERTION 2: TigerBeetle safety - validate sqrt_price_x64 is within bounds
// ASSERTION 3: TigerBeetle safety - validate tick_current is within bounds
decode_raydium_clmm_pool :: proc(data: []u8) -> (RaydiumCLMMPoolState, bool) {
	// VALIDATION 1: Check data length
	// Note: Raydium CLMM uses Anchor with 8-byte discriminator
	// Accept both sizes to handle with/without discriminator
	expected_size_with_disc := 1544
	expected_size_without_disc := 1536

	if len(data) != expected_size_with_disc && len(data) != expected_size_without_disc {
		// Size validation - return false instead of asserting
		// This allows tests to verify rejection of invalid sizes
		log.debugf("Invalid Raydium CLMM account size: %d (expected %d or %d)",
			len(data), expected_size_with_disc, expected_size_without_disc)
		return {}, false
	}

	log.debugf("Decoding Raydium CLMM account (%d bytes)", len(data))

	// Handle discriminator if present (8 bytes)
	offset := 0
	if len(data) == expected_size_with_disc {
		// Skip 8-byte Anchor discriminator
		offset = 8
		log.debug("Skipping 8-byte Anchor discriminator")
	}

	pool: RaydiumCLMMPoolState

	// Read configuration fields
	pool.bump[0] = data[offset + 0]
	pool.amm_config = read_pubkey(data, offset + 1)
	pool.owner = read_pubkey(data, offset + 33)
	pool.token_mint_0 = read_pubkey(data, offset + 65)
	pool.token_mint_1 = read_pubkey(data, offset + 97)
	pool.token_vault_0 = read_pubkey(data, offset + 129)
	pool.token_vault_1 = read_pubkey(data, offset + 161)
	pool.observation_key = read_pubkey(data, offset + 193)

	// Read decimals (CRITICAL - embedded in pool state)
	pool.mint_decimals_0 = data[offset + 225]
	pool.mint_decimals_1 = data[offset + 226]
	pool.tick_spacing = read_u16_le(data, offset + 227)

	log.debugf("Pool decimals: token_0=%d, token_1=%d, tick_spacing=%d",
		pool.mint_decimals_0, pool.mint_decimals_1, pool.tick_spacing)

	// Read liquidity and pricing fields (CRITICAL)
	pool.liquidity = read_u128_le(data, offset + 229)
	pool.sqrt_price_x64 = read_u128_le(data, offset + 245)
	pool.tick_current = read_i32_le(data, offset + 261)
	pool.padding3 = read_u16_le(data, offset + 265)
	pool.padding4 = read_u16_le(data, offset + 267)

	log.debugf("sqrt_price_x64: %v, tick_current: %d, liquidity: %v",
		pool.sqrt_price_x64, pool.tick_current, pool.liquidity)

	// ASSERTION 2: Validate sqrt_price_x64 bounds
	// Same bounds as Orca (Q64.64 format is universal)
	MIN_SQRT_PRICE: u128 = 4295048016
	MAX_SQRT_PRICE: u128 = 79226673515401279992447579055
	assert(
		pool.sqrt_price_x64 >= MIN_SQRT_PRICE && pool.sqrt_price_x64 <= MAX_SQRT_PRICE,
		fmt.tprintf("sqrt_price_x64 %v outside valid range [%v, %v]",
			pool.sqrt_price_x64, MIN_SQRT_PRICE, MAX_SQRT_PRICE),
	)

	// ASSERTION 3: Validate tick_current bounds
	// Standard CLMM tick range
	MIN_TICK: i32 = -443636
	MAX_TICK: i32 = 443636
	assert(
		pool.tick_current >= MIN_TICK && pool.tick_current <= MAX_TICK,
		fmt.tprintf("tick_current %d outside valid range [%d, %d]",
			pool.tick_current, MIN_TICK, MAX_TICK),
	)

	// Read fee tracking fields
	pool.fee_growth_global_0_x64 = read_u128_le(data, offset + 269)
	pool.fee_growth_global_1_x64 = read_u128_le(data, offset + 285)
	pool.protocol_fees_token_0 = read_u64_le(data, offset + 301)
	pool.protocol_fees_token_1 = read_u64_le(data, offset + 309)

	// Read swap statistics
	pool.swap_in_amount_token_0 = read_u128_le(data, offset + 317)
	pool.swap_out_amount_token_1 = read_u128_le(data, offset + 333)
	pool.swap_in_amount_token_1 = read_u128_le(data, offset + 349)
	pool.swap_out_amount_token_0 = read_u128_le(data, offset + 365)

	// Read status
	pool.status = data[offset + 381]

	log.info("Raydium CLMM account decoded successfully")
	return pool, true
}

// Calculate price from Raydium CLMM pool using Q64.64 conversion
//
// REUSES sqrt_price_to_price() from orca_decoder.odin
// Both Raydium and Orca use identical Q64.64 format for sqrt_price
//
// ASSERTION 1: Validate decimals are embedded and reasonable
calculate_raydium_clmm_price :: proc(pool: RaydiumCLMMPoolState) -> f64 {
	log.debugf("Calculating price from Raydium CLMM pool")

	// ASSERTION 1: Validate embedded decimals
	assert(
		pool.mint_decimals_0 <= 18 && pool.mint_decimals_1 <= 18,
		fmt.tprintf("Embedded decimals out of range: decimals_0=%d, decimals_1=%d",
			pool.mint_decimals_0, pool.mint_decimals_1),
	)

	// REUSE sqrt_price_to_price from orca_decoder.odin
	// Q64.64 format is identical between Orca and Raydium
	price := sqrt_price_to_price(
		pool.sqrt_price_x64,
		pool.mint_decimals_0,
		pool.mint_decimals_1,
	)

	log.debugf("Calculated Raydium CLMM price: %.18f", price)

	return price
}
