// PBKDF2-HMAC-SHA512 implementation (RFC 2898)
// Used by BIP39 for mnemonic-to-seed conversion
package keystore

import "core:crypto/hmac"
import "core:crypto/hash"
import models "../models"

// PBKDF2 with HMAC-SHA512 (RFC 2898)
// Used by BIP39 to convert mnemonic + passphrase to seed
//
// ASSERTION 1: Iterations must be positive
// ASSERTION 2: Key length must be positive
// ASSERTION 3: Output key must match requested length
pbkdf2_hmac_sha512 :: proc(
	password: []byte,
	salt: []byte,
	iterations: int,
	key_len: int,
	allocator := context.allocator,
) -> (key: []byte, err: models.ErrorType) {
	assert(iterations > 0, "Iterations must be positive")
	assert(key_len > 0, "Key length must be positive")

	// HMAC-SHA512 produces 64-byte output
	hash_len := 64

	// Calculate number of blocks needed
	block_count := (key_len + hash_len - 1) / hash_len

	// Allocate output buffer
	key = make([]byte, key_len, allocator)

	// For each block
	for i := 1; i <= block_count; i += 1 {
		// U1 = HMAC(password, salt || INT_32_BE(i))
		block, block_err := pbkdf2_derive_block(password, salt, iterations, i, allocator)
		if block_err != .None {
			delete(key)
			return nil, block_err
		}
		defer delete(block)

		// Copy to output (handle last block partial copy)
		offset := (i - 1) * hash_len
		remaining := key_len - offset
		copy_len := min(remaining, hash_len)
		copy(key[offset:offset+copy_len], block[:copy_len])
	}

	// ASSERTION 3: Verify output length
	assert(len(key) == key_len, "Output key length mismatch")

	return key, .None
}

// Derive a single PBKDF2 block
//
// ASSERTION 1: Block must be exactly 64 bytes (HMAC-SHA512 output)
// ASSERTION 2: Block index must be positive
@(private)
pbkdf2_derive_block :: proc(
	password: []byte,
	salt: []byte,
	iterations: int,
	block_index: int,
	allocator := context.allocator,
) -> (block: []byte, err: models.ErrorType) {
	assert(block_index > 0, "Block index must be positive")

	hash_len := 64
	block = make([]byte, hash_len, allocator)
	u := make([]byte, hash_len, allocator)
	defer delete(u)

	// First iteration: U1 = HMAC(password, salt || INT_32_BE(block_index))
	salt_with_index := make([]byte, len(salt) + 4, allocator)
	defer delete(salt_with_index)
	copy(salt_with_index, salt)

	// Append block index as big-endian u32
	salt_with_index[len(salt) + 0] = u8((block_index >> 24) & 0xFF)
	salt_with_index[len(salt) + 1] = u8((block_index >> 16) & 0xFF)
	salt_with_index[len(salt) + 2] = u8((block_index >> 8) & 0xFF)
	salt_with_index[len(salt) + 3] = u8(block_index & 0xFF)

	// U1 = HMAC-SHA512(password, salt || block_index)
	hmac.sum(hash.Algorithm.SHA512, u, salt_with_index, password)
	copy(block, u)

	// Remaining iterations: Ui = HMAC(password, U{i-1})
	// PERFORMANCE: Pre-allocate temp buffer ONCE instead of 2048 times
	temp := make([]byte, hash_len, allocator)
	defer delete(temp)

	for j := 2; j <= iterations; j += 1 {
		copy(temp, u)
		hmac.sum(hash.Algorithm.SHA512, u, temp, password)

		// XOR with accumulated result
		for k := 0; k < hash_len; k += 1 {
			block[k] ~= u[k]
		}
	}

	// ASSERTION 1: Verify block length
	assert(len(block) == hash_len, "Block must be 64 bytes")

	return block, .None
}
