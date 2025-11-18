// Tests for Solana address validation
package test_wallet

import "core:fmt"
import "core:testing"
import wallet "../../src/wallet"

@(test)
test_valid_solana_addresses :: proc(t: ^testing.T) {
	// Known valid Solana addresses
	valid_addresses := []string{
		"11111111111111111111111111111111", // System Program
		"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", // Token Program
		"So11111111111111111111111111111111111111112", // Wrapped SOL
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
		"DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2", // AURA from config
	}

	for address in valid_addresses {
		result := wallet.validate_solana_address(address)
		testing.expectf(t, result == true,
			"Expected address '%s' to be valid, but validation failed",
			address)
	}
}

@(test)
test_invalid_solana_addresses :: proc(t: ^testing.T) {
	// Invalid addresses
	invalid_addresses := []string{
		"", // Empty
		"invalid", // Too short
		"11111111111111111111111111111111111111111111111111111", // Too long
		"000000000000000000000000000000000", // Invalid characters (0 not in Base58)
		"OOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOO", // Invalid characters (O not in Base58)
		"IIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIII", // Invalid characters (I not in Base58)
		"lllllllllllllllllllllllllllllllll", // Invalid characters (l not in Base58)
	}

	for address in invalid_addresses {
		result := wallet.validate_solana_address(address)
		testing.expectf(t, result == false,
			"Expected address '%s' to be invalid, but validation passed",
			address)
	}
}

@(test)
test_base58_character_validation :: proc(t: ^testing.T) {
	// Valid Base58 characters
	valid_chars := []rune{'1', '2', '9', 'A', 'Z', 'a', 'z'}
	for char in valid_chars {
		result := wallet.is_base58_char(char)
		testing.expectf(t, result == true,
			"Expected '%c' to be valid Base58 character", char)
	}

	// Invalid Base58 characters (0, O, I, l)
	invalid_chars := []rune{'0', 'O', 'I', 'l', '@', '#', ' '}
	for char in invalid_chars {
		result := wallet.is_base58_char(char)
		testing.expectf(t, result == false,
			"Expected '%c' to be invalid Base58 character", char)
	}
}
