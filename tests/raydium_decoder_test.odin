#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "../src"

// =============================================================================
// RAYDIUM POOL DECODER TESTS
// =============================================================================
// These tests document and verify the Raydium LIQUIDITY_STATE_LAYOUT_V4
// binary structure decoding. The structure is 752 bytes and contains:
// - 256 bytes: u64 fields (32 fields)
// - 80 bytes: u128/u64 swap fee fields (6 fields)
// - 384 bytes: publicKey fields (12 fields of 32 bytes each)
// - 32 bytes: lpReserve + padding
//
// Key discovery through reverse engineering:
// - Vaults are at offsets 336 (quote) and 368 (base), NOT 192/224 as initially documented
// - Decimals are at offsets 32 (quote) and 40 (base), swapped from initial assumption
// =============================================================================

@(test)
test_read_u64_le :: proc(t: ^testing.T) {
	// DOCUMENTATION: read_u64_le reads an 8-byte little-endian unsigned integer
	// Little-endian means the least significant byte comes first.
	//
	// Example: bytes [0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08]
	// represent the number 0x0807060504030201
	//
	// This is crucial for reading Solana on-chain data which uses little-endian encoding.

	data := [8]u8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	result := src.read_u64_le(data[:], 0)
	expected := u64(0x0807060504030201)

	testing.expect(t, result == expected,
		fmt.tprintf("Little-endian decode failed: Expected %d, got %d", expected, result))
}

@(test)
test_read_u64_le_zeros :: proc(t: ^testing.T) {
	data := [8]u8{0, 0, 0, 0, 0, 0, 0, 0}

	result := src.read_u64_le(data[:], 0)
	expected := u64(0)

	testing.expect(t, result == expected,
		fmt.tprintf("Expected %d, got %d", expected, result))
}

@(test)
test_read_u64_le_max :: proc(t: ^testing.T) {
	data := [8]u8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	result := src.read_u64_le(data[:], 0)
	expected := u64(0xFFFFFFFFFFFFFFFF)

	testing.expect(t, result == expected,
		fmt.tprintf("Expected %d, got %d", expected, result))
}

@(test)
test_read_pubkey :: proc(t: ^testing.T) {
	// Test reading 32-byte public key
	data: [64]u8

	// Fill first 32 bytes with pattern
	for i in 0..<32 {
		data[i] = u8(i)
	}

	result := src.read_pubkey(data[:], 0)

	// Verify all bytes match
	for i in 0..<32 {
		testing.expect(t, result[i] == u8(i),
			fmt.tprintf("Byte %d: expected %d, got %d", i, u8(i), result[i]))
	}
}

@(test)
test_read_pubkey_offset :: proc(t: ^testing.T) {
	data: [64]u8

	// Fill bytes at offset 32 with pattern
	for i in 0..<32 {
		data[32 + i] = u8(i + 100)
	}

	result := src.read_pubkey(data[:], 32)

	// Verify all bytes match from offset
	for i in 0..<32 {
		testing.expect(t, result[i] == u8(i + 100),
			fmt.tprintf("Byte %d: expected %d, got %d", i, u8(i + 100), result[i]))
	}
}

@(test)
test_pubkey_to_base58_all_zeros :: proc(t: ^testing.T) {
	pubkey: [32]u8 = {}

	result := src.pubkey_to_base58(pubkey)

	// All zeros should encode to 32 '1's in base58
	expected := "11111111111111111111111111111111"

	testing.expect(t, result == expected,
		fmt.tprintf("Expected '%s', got '%s'", expected, result))
}

@(test)
test_pubkey_to_base58_known_address :: proc(t: ^testing.T) {
	// SOL mint address: So11111111111111111111111111111111111111112
	// In hex: 0x069b8857feab8184fb687f634618c035dac439dc1aeb3b5598a0f00000000001
	sol_mint := [32]u8{
		0x06, 0x9b, 0x88, 0x57, 0xfe, 0xab, 0x81, 0x84,
		0xfb, 0x68, 0x7f, 0x63, 0x46, 0x18, 0xc0, 0x35,
		0xda, 0xc4, 0x39, 0xdc, 0x1a, 0xeb, 0x3b, 0x55,
		0x98, 0xa0, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x01,
	}

	result := src.pubkey_to_base58(sol_mint)
	expected := "So11111111111111111111111111111111111111112"

	testing.expect(t, result == expected,
		fmt.tprintf("Expected '%s', got '%s'", expected, result))
}

@(test)
test_decode_raydium_pool_v4_invalid_size :: proc(t: ^testing.T) {
	// Test with wrong size data
	data := make([]u8, 100)
	defer delete(data)

	_, ok := src.decode_raydium_pool_v4(data)

	testing.expect(t, !ok, "Should reject data with wrong size")
}

@(test)
test_decode_raydium_pool_v4_correct_size :: proc(t: ^testing.T) {
	// Test with correct size (752 bytes) but dummy data
	data := make([]u8, 752)
	defer delete(data)

	// Set some recognizable values
	// Offset 32: quote_decimal = 9
	data[32] = 9
	// Offset 40: base_decimal = 6
	data[40] = 6

	pool, ok := src.decode_raydium_pool_v4(data)

	testing.expect(t, ok, "Should accept data with correct size")
	testing.expect(t, pool.quote_decimal == 9,
		fmt.tprintf("Expected quote_decimal=9, got %d", pool.quote_decimal))
	testing.expect(t, pool.base_decimal == 6,
		fmt.tprintf("Expected base_decimal=6, got %d", pool.base_decimal))
}

@(test)
test_decode_raydium_pool_v4_vault_offsets :: proc(t: ^testing.T) {
	data := make([]u8, 752)
	defer delete(data)

	// Set distinctive patterns at vault offsets
	// Quote vault at offset 336
	for i in 0..<32 {
		data[336 + i] = u8(i + 1)
	}

	// Base vault at offset 368
	for i in 0..<32 {
		data[368 + i] = u8(i + 50)
	}

	pool, ok := src.decode_raydium_pool_v4(data)

	testing.expect(t, ok, "Should decode successfully")

	// Verify quote vault pattern
	for i in 0..<32 {
		testing.expect(t, pool.quote_vault[i] == u8(i + 1),
			fmt.tprintf("Quote vault byte %d: expected %d, got %d",
				i, u8(i + 1), pool.quote_vault[i]))
	}

	// Verify base vault pattern
	for i in 0..<32 {
		testing.expect(t, pool.base_vault[i] == u8(i + 50),
			fmt.tprintf("Base vault byte %d: expected %d, got %d",
				i, u8(i + 50), pool.base_vault[i]))
	}
}

@(test)
test_decode_raydium_pool_v4_mint_offsets :: proc(t: ^testing.T) {
	data := make([]u8, 752)
	defer delete(data)

	// Set distinctive patterns at mint offsets
	// Quote mint at offset 400
	for i in 0..<32 {
		data[400 + i] = u8(i + 100)
	}

	// Base mint at offset 432
	for i in 0..<32 {
		data[432 + i] = u8(i + 150)
	}

	pool, ok := src.decode_raydium_pool_v4(data)

	testing.expect(t, ok, "Should decode successfully")

	// Verify quote mint pattern
	for i in 0..<32 {
		testing.expect(t, pool.quote_mint[i] == u8(i + 100),
			fmt.tprintf("Quote mint byte %d: expected %d, got %d",
				i, u8(i + 100), pool.quote_mint[i]))
	}

	// Verify base mint pattern
	for i in 0..<32 {
		testing.expect(t, pool.base_mint[i] == u8(i + 150),
			fmt.tprintf("Base mint byte %d: expected %d, got %d",
				i, u8(i + 150), pool.base_mint[i]))
	}
}
