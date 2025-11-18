#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "../src"

// =============================================================================
// RAYDIUM CLMM DECODER TESTS
// =============================================================================
// These tests validate the Raydium CLMM decoder including:
// - Account size validation
// - Q64.64 sqrt_price_x64 conversion
// - Embedded decimal extraction
// - Field offset accuracy
// - Tick bounds validation
//
// Test Philosophy:
// - Unit tests for pure functions (decode, price calculation)
// - Document Raydium CLMM-specific behavior
// - Match Orca decoder test coverage (12+ tests)
//
// Coverage:
// 1. Size validation (with/without discriminator)
// 2. Field offset accuracy
// 3. Embedded decimals extraction
// 4. Q64.64 conversion with Raydium pools
// 5. Tick validation
// 6. Edge cases (zeros, max values, decimal combinations)
// =============================================================================

@(test)
test_decode_raydium_clmm_invalid_size :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test Raydium CLMM decoder rejects invalid sizes
	// Expected sizes: 1544 bytes (with discriminator) or 1536 bytes (without)
	// Any other size should fail

	invalid_sizes := []int{100, 500, 1000, 1535, 1537, 1543, 1545, 2000}

	for size in invalid_sizes {
		data := make([]u8, size)
		defer delete(data)

		_, ok := src.decode_raydium_clmm_pool(data)
		testing.expect(t, !ok,
			fmt.tprintf("Size %d should be rejected", size))
	}
}

@(test)
test_decode_raydium_clmm_correct_size_without_discriminator :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test decoder accepts 1536 bytes (without discriminator)
	// This validates that the decoder handles raw pool data

	data := make([]u8, 1536)
	defer delete(data)

	// Fill with minimal valid data
	// Set sqrt_price_x64 to minimum valid value at offset 245
	min_sqrt_price: u128 = 4295048016
	write_u128_le(data, 245, min_sqrt_price)

	// Set tick_current to valid value at offset 261
	write_i32_le(data, 261, 0)

	// Set decimals at offsets 225-226
	data[225] = 9  // SOL decimals
	data[226] = 6  // Token decimals

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "1536 bytes should be accepted")
	testing.expect(t, pool.sqrt_price_x64 == min_sqrt_price,
		fmt.tprintf("Expected sqrt_price_x64=%v, got %v", min_sqrt_price, pool.sqrt_price_x64))
}

@(test)
test_decode_raydium_clmm_correct_size_with_discriminator :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test decoder accepts 1544 bytes (with 8-byte discriminator)
	// Raydium CLMM uses Anchor with discriminator prefix

	data := make([]u8, 1544)
	defer delete(data)

	// Fill discriminator (8 bytes)
	for i in 0..<8 {
		data[i] = u8(i)
	}

	// Set sqrt_price_x64 to minimum valid value at offset 8 + 245 = 253
	min_sqrt_price: u128 = 4295048016
	write_u128_le(data, 253, min_sqrt_price)

	// Set tick_current to valid value at offset 8 + 261 = 269
	write_i32_le(data, 269, 0)

	// Set decimals at offsets 8 + 225 = 233, 8 + 226 = 234
	data[233] = 9  // SOL decimals
	data[234] = 6  // Token decimals

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "1544 bytes (with discriminator) should be accepted")
	testing.expect(t, pool.sqrt_price_x64 == min_sqrt_price,
		fmt.tprintf("Expected sqrt_price_x64=%v, got %v", min_sqrt_price, pool.sqrt_price_x64))
}

@(test)
test_raydium_clmm_field_offsets :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test critical field offsets are correct
	// Validates that decoder reads sqrt_price_x64, tick_current, and decimals
	// from the correct byte positions

	data := make([]u8, 1536)
	defer delete(data)

	// Test sqrt_price_x64 at offset 245 (16 bytes)
	// Use a valid value within bounds: MIN=4295048016, MAX=79226673515401279992447579055
	test_sqrt_price: u128 = 100000000000000000  // Valid mid-range value
	write_u128_le(data, 245, test_sqrt_price)

	// Test tick_current at offset 261 (4 bytes)
	test_tick: i32 = 12345
	write_i32_le(data, 261, test_tick)

	// Test decimals at offsets 225, 226
	data[225] = 9  // mint_decimals_0
	data[226] = 6  // mint_decimals_1

	// Test tick_spacing at offset 227 (2 bytes)
	test_tick_spacing: u16 = 64
	write_u16_le(data, 227, test_tick_spacing)

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "Decoding should succeed")

	testing.expect(t, pool.sqrt_price_x64 == test_sqrt_price,
		fmt.tprintf("sqrt_price_x64 mismatch: expected %v, got %v",
			test_sqrt_price, pool.sqrt_price_x64))

	testing.expect(t, pool.tick_current == test_tick,
		fmt.tprintf("tick_current mismatch: expected %d, got %d",
			test_tick, pool.tick_current))

	testing.expect(t, pool.mint_decimals_0 == 9,
		fmt.tprintf("mint_decimals_0 mismatch: expected 9, got %d",
			pool.mint_decimals_0))

	testing.expect(t, pool.mint_decimals_1 == 6,
		fmt.tprintf("mint_decimals_1 mismatch: expected 6, got %d",
			pool.mint_decimals_1))

	testing.expect(t, pool.tick_spacing == test_tick_spacing,
		fmt.tprintf("tick_spacing mismatch: expected %d, got %d",
			test_tick_spacing, pool.tick_spacing))
}

@(test)
test_raydium_clmm_decimals_extraction :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test embedded decimals are correctly extracted
	// Raydium CLMM embeds decimals in pool state (unlike Orca)
	// This is a key advantage - no need to fetch mint accounts

	test_cases := []struct{decimals_0: u8, decimals_1: u8}{
		{9, 6},   // SOL/AURA
		{6, 6},   // USDC/Token
		{9, 9},   // SOL/Token
		{0, 18},  // Edge case: min/max
		{18, 0},  // Edge case: max/min
	}

	for test_case in test_cases {
		data := make([]u8, 1536)
		defer delete(data)

		// Set decimals
		data[225] = test_case.decimals_0
		data[226] = test_case.decimals_1

		// Set minimal valid sqrt_price and tick
		write_u128_le(data, 245, 4295048016)
		write_i32_le(data, 261, 0)

		pool, ok := src.decode_raydium_clmm_pool(data)
		testing.expect(t, ok, "Decoding should succeed")

		testing.expect(t, pool.mint_decimals_0 == test_case.decimals_0,
			fmt.tprintf("Expected decimals_0=%d, got %d",
				test_case.decimals_0, pool.mint_decimals_0))

		testing.expect(t, pool.mint_decimals_1 == test_case.decimals_1,
			fmt.tprintf("Expected decimals_1=%d, got %d",
				test_case.decimals_1, pool.mint_decimals_1))
	}
}

@(test)
test_sqrt_price_conversion_raydium :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test Q64.64 conversion with Raydium CLMM pool
	// Uses same formula as Orca: price = (sqrt_price / 2^64)^2 * 10^(decimals_a - decimals_b)

	data := make([]u8, 1536)
	defer delete(data)

	// Example: AURA/SOL pool
	// sqrt_price_x64 representing ~0.00006 SOL per AURA
	// sqrt(0.00006) ≈ 0.00774597, * 2^64 ≈ 142868617405711
	test_sqrt_price: u128 = 142868617405711

	write_u128_le(data, 245, test_sqrt_price)
	write_i32_le(data, 261, 0)

	// SOL = 9 decimals, AURA = 6 decimals
	// Price is token_1/token_0, so we want AURA/SOL price
	data[225] = 9  // mint_decimals_0 (SOL - base)
	data[226] = 6  // mint_decimals_1 (AURA - quote)

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "Decoding should succeed")

	price := src.calculate_raydium_clmm_price(pool)

	// Expected price ≈ 0.00000006 (with decimal adjustment 9-6=3, gives 0.001x)
	// The sqrt_price represents the reciprocal, so actual price is 1000x smaller
	expected_price := 0.00000006
	tolerance := 0.00000001

	diff := price - expected_price
	if diff < 0 do diff = -diff

	testing.expect(t, diff < tolerance,
		fmt.tprintf("Price %.9f outside tolerance of %.9f (diff: %.9f)",
			price, expected_price, diff))
}

@(test)
test_tick_current_bounds :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test tick_current validation bounds
	// Standard CLMM tick range: -443636 to 443636
	// tick represents log base 1.0001 of price

	valid_ticks := []i32{-443636, -100000, -1, 0, 1, 100000, 443636}

	for tick in valid_ticks {
		data := make([]u8, 1536)
		defer delete(data)

		write_u128_le(data, 245, 4295048016)  // Min sqrt_price
		write_i32_le(data, 261, tick)
		data[225] = 9
		data[226] = 6

		pool, ok := src.decode_raydium_clmm_pool(data)
		testing.expect(t, ok,
			fmt.tprintf("Tick %d should be valid", tick))
		testing.expect(t, pool.tick_current == tick,
			fmt.tprintf("Expected tick=%d, got %d", tick, pool.tick_current))
	}
}

@(test)
test_realistic_raydium_clmm_price :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test realistic Raydium CLMM price calculation
	// Uses values similar to real CLMM pools

	data := make([]u8, 1536)
	defer delete(data)

	// Realistic sqrt_price for a token worth ~$0.05 in SOL terms
	// If token = 0.0003 SOL, sqrt_price = sqrt(0.0003) * 2^64
	// sqrt(0.0003) ≈ 0.01732, * 2^64 ≈ 319458891533823
	realistic_sqrt_price: u128 = 319458891533823

	write_u128_le(data, 245, realistic_sqrt_price)
	write_i32_le(data, 261, -12345)  // Arbitrary tick

	// SOL = 9 decimals, Token = 6 decimals
	// Price is token_1/token_0, so we want Token/SOL price
	data[225] = 9  // mint_decimals_0 (SOL - base)
	data[226] = 6  // mint_decimals_1 (Token - quote)

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "Decoding should succeed")

	price := src.calculate_raydium_clmm_price(pool)

	// Expected price ≈ 0.0000003 SOL (with 10% tolerance)
	// The sqrt_price with decimal adjustment gives 0.001x the intuitive price
	expected_price := 0.0000003
	tolerance := expected_price * 0.1  // 10% tolerance

	diff := price - expected_price
	if diff < 0 do diff = -diff

	testing.expect(t, diff < tolerance,
		fmt.tprintf("Price %.9f outside 10%% tolerance of %.9f (diff: %.9f)",
			price, expected_price, diff))
}

@(test)
test_raydium_clmm_zero_decimals :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test edge case with zero decimals
	// Some tokens have 0 decimals (rare but valid)

	data := make([]u8, 1536)
	defer delete(data)

	write_u128_le(data, 245, 4295048016)
	write_i32_le(data, 261, 0)
	data[225] = 0  // 0 decimals
	data[226] = 0  // 0 decimals

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "Decoding with 0 decimals should succeed")

	price := src.calculate_raydium_clmm_price(pool)
	testing.expect(t, price >= 0,
		fmt.tprintf("Price should be non-negative, got %.9f", price))
}

@(test)
test_raydium_clmm_max_decimals :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test edge case with maximum decimals
	// Maximum decimals = 18 (assertion boundary)

	data := make([]u8, 1536)
	defer delete(data)

	write_u128_le(data, 245, 4295048016)
	write_i32_le(data, 261, 0)
	data[225] = 18  // Max decimals
	data[226] = 18  // Max decimals

	pool, ok := src.decode_raydium_clmm_pool(data)
	testing.expect(t, ok, "Decoding with max decimals should succeed")

	price := src.calculate_raydium_clmm_price(pool)
	testing.expect(t, price >= 0,
		fmt.tprintf("Price should be non-negative, got %.9f", price))
}

@(test)
test_raydium_clmm_decimal_combinations :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test various decimal combinations
	// Validates decimal adjustment works correctly

	test_cases := []struct{decimals_0: u8, decimals_1: u8, sqrt_price: u128}{
		{6, 9, 100000000000000000}, // Token < Quote (common)
		{9, 6, 100000000000000000}, // Token > Quote
		{6, 6, 100000000000000000}, // Equal decimals
		{0, 9, 100000000000000000}, // Min/High
		{9, 0, 100000000000000000}, // High/Min
	}

	for test_case in test_cases {
		data := make([]u8, 1536)
		defer delete(data)

		write_u128_le(data, 245, test_case.sqrt_price)
		write_i32_le(data, 261, 0)
		data[225] = test_case.decimals_0
		data[226] = test_case.decimals_1

		pool, ok := src.decode_raydium_clmm_pool(data)
		testing.expect(t, ok,
			fmt.tprintf("Decoding with decimals (%d, %d) should succeed",
				test_case.decimals_0, test_case.decimals_1))

		price := src.calculate_raydium_clmm_price(pool)
		testing.expect(t, price >= 0,
			fmt.tprintf("Price should be non-negative for decimals (%d, %d), got %.9f",
				test_case.decimals_0, test_case.decimals_1, price))
	}
}

// =============================================================================
// HELPER FUNCTIONS FOR TESTS
// =============================================================================

// Write little-endian u128 to byte array
write_u128_le :: proc(data: []u8, offset: int, value: u128) {
	if offset + 16 > len(data) do return

	low := u64(value & 0xFFFFFFFFFFFFFFFF)
	high := u64(value >> 64)

	// Write low 64 bits
	for i in 0..<8 {
		data[offset + i] = u8((low >> (u64(i) * 8)) & 0xFF)
	}

	// Write high 64 bits
	for i in 0..<8 {
		data[offset + 8 + i] = u8((high >> (u64(i) * 8)) & 0xFF)
	}
}

// Write little-endian i32 to byte array
write_i32_le :: proc(data: []u8, offset: int, value: i32) {
	if offset + 4 > len(data) do return

	unsigned := u32(value)
	for i in 0..<4 {
		data[offset + i] = u8((unsigned >> (u32(i) * 8)) & 0xFF)
	}
}

// Write little-endian u16 to byte array
write_u16_le :: proc(data: []u8, offset: int, value: u16) {
	if offset + 2 > len(data) do return

	data[offset] = u8(value & 0xFF)
	data[offset + 1] = u8((value >> 8) & 0xFF)
}
