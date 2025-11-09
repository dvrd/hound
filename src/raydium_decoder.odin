#+feature global-context
package main

import "core:fmt"
import "core:encoding/hex"

// Raydium pool state structure (752 bytes)
// LIQUIDITY_STATE_LAYOUT_V4
RaydiumPoolState :: struct {
	status:           u64,
	nonce:            u64,
	max_order:        u64,
	depth:            u64,
	base_decimal:     u64, // Offset 32
	quote_decimal:    u64, // Offset 40
	state:            u64,
	reset_flag:       u64,
	min_size:         u64,
	vol_max_cut:      u64,
	amount_wave:      u64,
	base_lot_size:    u64,
	quote_lot_size:   u64,
	min_price_mult:   u64,
	max_price_mult:   u64,
	system_decimal:   u64,
	base_mint:        [32]u8, // Offset 128
	quote_mint:       [32]u8, // Offset 160
	base_vault:       [32]u8, // Offset 192 - CRITICAL
	quote_vault:      [32]u8, // Offset 224 - CRITICAL
	base_withdraw_q:  [32]u8,
	quote_withdraw_q: [32]u8,
	lp_mint:          [32]u8,
	lp_vault:         [32]u8,
	open_orders:      [32]u8,
	market_id:        [32]u8,
	market_program:   [32]u8,
	target_orders:    [32]u8,
}

// Decode Raydium LIQUIDITY_STATE_LAYOUT_V4 from 752 bytes
decode_raydium_pool_v4 :: proc(data: []u8) -> (RaydiumPoolState, bool) {
	// Validate data length
	if len(data) != 752 {
		return {}, false
	}

	pool: RaydiumPoolState

	// Read u64 fields (little-endian)
	pool.status = read_u64_le(data, 0)
	pool.nonce = read_u64_le(data, 8)
	pool.max_order = read_u64_le(data, 16)
	pool.depth = read_u64_le(data, 24)
	pool.base_decimal = read_u64_le(data, 32) // CRITICAL
	pool.quote_decimal = read_u64_le(data, 40) // CRITICAL
	pool.state = read_u64_le(data, 48)
	pool.reset_flag = read_u64_le(data, 56)
	pool.min_size = read_u64_le(data, 64)
	pool.vol_max_cut = read_u64_le(data, 72)
	pool.amount_wave = read_u64_le(data, 80)
	pool.base_lot_size = read_u64_le(data, 88)
	pool.quote_lot_size = read_u64_le(data, 96)
	pool.min_price_mult = read_u64_le(data, 104)
	pool.max_price_mult = read_u64_le(data, 112)
	pool.system_decimal = read_u64_le(data, 120)

	// Read PublicKey fields (32 bytes each)
	pool.base_mint = read_pubkey(data, 128)
	pool.quote_mint = read_pubkey(data, 160)
	pool.base_vault = read_pubkey(data, 192) // CRITICAL
	pool.quote_vault = read_pubkey(data, 224) // CRITICAL
	pool.base_withdraw_q = read_pubkey(data, 256)
	pool.quote_withdraw_q = read_pubkey(data, 288)
	pool.lp_mint = read_pubkey(data, 320)
	pool.lp_vault = read_pubkey(data, 352)
	pool.open_orders = read_pubkey(data, 384)
	pool.market_id = read_pubkey(data, 416)
	pool.market_program = read_pubkey(data, 448)
	pool.target_orders = read_pubkey(data, 480)

	return pool, true
}

// Read little-endian u64 from byte array at offset
read_u64_le :: proc(data: []u8, offset: int) -> u64 {
	if offset+8 > len(data) {
		return 0
	}

	// Little-endian: least significant byte first
	return u64(data[offset]) | u64(data[offset + 1]) << 8 | u64(data[offset + 2]) << 16 |
		u64(data[offset + 3]) << 24 | u64(data[offset + 4]) << 32 |
		u64(data[offset + 5]) << 40 | u64(data[offset + 6]) << 48 |
		u64(data[offset + 7]) << 56
}

// Read 32-byte public key from byte array at offset
read_pubkey :: proc(data: []u8, offset: int) -> [32]u8 {
	result: [32]u8
	if offset+32 > len(data) {
		return result
	}

	// Copy 32 bytes
	copy(result[:], data[offset:offset + 32])
	return result
}

// Convert 32-byte public key to base58 string
pubkey_to_base58 :: proc(pubkey: [32]u8) -> string {
	alphabet := [58]u8{
		'1',
		'2',
		'3',
		'4',
		'5',
		'6',
		'7',
		'8',
		'9',
		'A',
		'B',
		'C',
		'D',
		'E',
		'F',
		'G',
		'H',
		'J',
		'K',
		'L',
		'M',
		'N',
		'P',
		'Q',
		'R',
		'S',
		'T',
		'U',
		'V',
		'W',
		'X',
		'Y',
		'Z',
		'a',
		'b',
		'c',
		'd',
		'e',
		'f',
		'g',
		'h',
		'i',
		'j',
		'k',
		'm',
		'n',
		'o',
		'p',
		'q',
		'r',
		's',
		't',
		'u',
		'v',
		'w',
		'x',
		'y',
		'z',
	}

	// Convert bytes to bigint representation
	num: [64]u8 = {} // Large enough for base conversion
	num_len := 0

	// Copy input
	for i := 0; i < 32; i += 1 {
		num[i] = pubkey[i]
	}
	num_len = 32

	// Build result in reverse
	result := make([dynamic]u8, context.temp_allocator)
	reserve(&result, 44) // Max base58 length for 32 bytes

	// Count leading zeros
	leading_zeros := 0
	for i := 0; i < 32; i += 1 {
		if pubkey[i] == 0 {
			leading_zeros += 1
		} else {
			break
		}
	}

	// Encode
	zero_mask: [64]bool = {}
	for {
		// Check if all zero
		all_zero := true
		for i := 0; i < num_len; i += 1 {
			if !zero_mask[i] && num[i] != 0 {
				all_zero = false
				break
			}
		}
		if all_zero {
			break
		}

		// Divide by 58
		remainder: u16 = 0
		for i := 0; i < num_len; i += 1 {
			if zero_mask[i] {
				continue
			}
			current := u16(num[i]) + remainder * 256
			num[i] = u8(current / 58)
			remainder = current % 58
			if num[i] == 0 && i == 0 {
				zero_mask[i] = true
			}
		}

		append_elem(&result, alphabet[remainder])
	}

	// Add leading '1's for leading zeros
	for i := 0; i < leading_zeros; i += 1 {
		append_elem(&result, '1')
	}

	// Reverse result
	result_len := len(result)
	for i := 0; i < result_len / 2; i += 1 {
		result[i], result[result_len - 1 - i] = result[result_len - 1 - i], result[i]
	}

	return string(result[:])
}
