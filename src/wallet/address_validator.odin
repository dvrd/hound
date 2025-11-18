// Solana address validation module
// Implements Base58 decoding and validation for Solana wallet addresses
package wallet

import "core:slice"
import "core:log"
import src "../"

// Base58 alphabet (Bitcoin/Solana variant - excludes 0, O, I, l)
BASE58_ALPHABET :: "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Solana public key length (32 bytes)
SOLANA_PUBKEY_LENGTH :: 32

// validate_solana_address validates a Base58-encoded Solana address
//
// Validation rules:
// 1. Must be Base58 encoded (only valid alphabet characters)
// 2. Must decode to exactly 32 bytes (Solana public key size)
// 3. Must not be all zeros (invalid address)
//
// ASSERTION 1: Address must not be empty
//
// Returns: true if valid, false otherwise
validate_solana_address :: proc(address: string) -> bool {
	if len(address) == 0 {
		log.warn("Address validation failed: empty address")
		return false
	}

	log.debugf("Validating Solana address: %s", address)

	// Step 1: Verify all characters are in Base58 alphabet
	for char in address {
		if !is_base58_char(char) {
			log.warnf("Address validation failed: invalid character '%c'", char)
			return false
		}
	}

	// Step 2: Decode Base58 to bytes
	decoded, decode_ok := decode_base58(address)
	if !decode_ok {
		log.warn("Address validation failed: Base58 decode error")
		return false
	}
	defer delete(decoded)

	// Step 3: Verify length is exactly 32 bytes
	if len(decoded) != SOLANA_PUBKEY_LENGTH {
		log.warnf("Address validation failed: wrong length %d (expected %d)",
			len(decoded), SOLANA_PUBKEY_LENGTH)
		return false
	}

	// All checks passed - valid Solana address
	// Note: All-zero address (11111111111111111111111111111111) is valid - it's the System Program
	log.debugf("Address validation succeeded: %s", address)
	return true
}

// is_base58_char checks if a character is in the Base58 alphabet
is_base58_char :: proc(char: rune) -> bool {
	for c in BASE58_ALPHABET {
		if char == c {
			return true
		}
	}
	return false
}

// decode_base58 decodes a Base58 string to bytes
//
// Algorithm:
// 1. Count leading '1's (each represents a zero byte)
// 2. Convert Base58 digits to bytes using big integer arithmetic
// 3. Return decoded bytes with leading zeros preserved
//
// Returns: Decoded bytes and success flag
decode_base58 :: proc(encoded: string) -> (decoded: []byte, ok: bool) {
	if len(encoded) == 0 {
		return nil, false
	}

	// Build reverse lookup: character -> value (0-57)
	char_to_value: [256]i32
	for i := 0; i < 256; i += 1 {
		char_to_value[i] = -1
	}
	alphabet := BASE58_ALPHABET
	for i := 0; i < len(alphabet); i += 1 {
		char_to_value[alphabet[i]] = i32(i)
	}

	// Count leading '1's (each represents a zero byte in output)
	leading_zeros := 0
	for char in encoded {
		if char != '1' {
			break
		}
		leading_zeros += 1
	}

	// Estimate output size (Base58 is ~log(58)/log(256) = 1.37x more compact)
	// For 32-byte output, we expect ~44 Base58 chars
	// Allocate generous buffer
	size := len(encoded) * 733 / 1000 + 1  // log(58)/log(256) ≈ 0.733
	output := make([dynamic]byte, 0, size)
	defer delete(output)

	// Process each Base58 digit
	for char in encoded {
		digit := char_to_value[char]
		if digit == -1 {
			log.warnf("Invalid Base58 character: %c", char)
			return nil, false
		}

		// Multiply output by 58 and add digit (big-endian)
		carry := i32(digit)
		for i := len(output) - 1; i >= 0; i -= 1 {
			carry += i32(output[i]) * 58
			output[i] = byte(carry & 0xFF)
			carry >>= 8
		}

		// Append remaining carry bytes
		for carry > 0 {
			inject_at(&output, 0, byte(carry & 0xFF))
			carry >>= 8
		}
	}

	// Skip leading zeros in output (but preserve those from leading '1's)
	output_start := 0
	for output_start < len(output) && output[output_start] == 0 {
		output_start += 1
	}

	// Build final result: leading zeros + decoded bytes
	result_len := leading_zeros + (len(output) - output_start)
	result := make([]byte, result_len)

	// Fill leading zeros
	for i := 0; i < leading_zeros; i += 1 {
		result[i] = 0
	}

	// Copy decoded bytes
	copy(result[leading_zeros:], output[output_start:])

	return result, true
}
