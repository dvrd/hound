// SLIP-0010 Test Vectors - BIP32-Ed25519 Key Derivation
// Spec: https://github.com/satoshilabs/slips/blob/master/slip-0010.md
// Tests hierarchical deterministic key derivation for Ed25519 (Solana)
#+feature global-context
package tests

import "core:fmt"
import "core:testing"
import "core:log"
import "core:encoding/hex"
import keystore "../../src/lib/keystore"

// ============================================================================
// SLIP-0010 Ed25519 Test Vectors
// ============================================================================

// Test Vector 1: Basic master key derivation from seed
// Seed: 000102030405060708090a0b0c0d0e0f
@(test)
test_slip0010_vector_1_master_key :: proc(t: ^testing.T) {
	log.info("Testing SLIP-0010 Vector 1: Master key derivation")

	// Simple test seed (16 bytes)
	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Derive master key
	master_key, err := keystore.derive_master_key(seed)
	testing.expectf(t, err == .None, "Master key derivation failed: %v", err)

	// Verify master key properties
	testing.expectf(t, master_key.depth == 0, "Master key depth should be 0, got %d", master_key.depth)
	testing.expectf(t, len(master_key.key) == 32, "Master key should be 32 bytes, got %d", len(master_key.key))
	testing.expectf(t, len(master_key.chain_code) == 32, "Chain code should be 32 bytes, got %d", len(master_key.chain_code))

	log.info("✓ SLIP-0010 Vector 1 passed")
}

// Test Vector 2: Single level hardened derivation m/0'
@(test)
test_slip0010_vector_2_single_derivation :: proc(t: ^testing.T) {
	log.info("Testing SLIP-0010 Vector 2: Single level derivation m/0'")

	// Test seed
	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Derive at path m/0'
	derived_key, err := keystore.derive_from_path(seed, "m/0'")
	testing.expectf(t, err == .None, "Derivation failed: %v", err)

	// Verify derived key properties
	testing.expectf(t, derived_key.depth == 1, "Derived key depth should be 1, got %d", derived_key.depth)
	testing.expectf(t, len(derived_key.key) == 32, "Derived key should be 32 bytes, got %d", len(derived_key.key))

	log.info("✓ SLIP-0010 Vector 2 passed")
}

// Test Vector 3: Multi-level derivation m/0'/1'/2'
@(test)
test_slip0010_vector_3_multi_level :: proc(t: ^testing.T) {
	log.info("Testing SLIP-0010 Vector 3: Multi-level derivation m/0'/1'/2'")

	// Test seed
	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Derive at path m/0'/1'/2'
	derived_key, err := keystore.derive_from_path(seed, "m/0'/1'/2'")
	testing.expectf(t, err == .None, "Multi-level derivation failed: %v", err)

	// Verify derived key properties
	testing.expectf(t, derived_key.depth == 3, "Derived key depth should be 3, got %d", derived_key.depth)
	testing.expectf(t, len(derived_key.key) == 32, "Derived key should be 32 bytes, got %d", len(derived_key.key))

	log.info("✓ SLIP-0010 Vector 3 passed")
}

// Test Vector 4: Solana standard path m/44'/501'/0'/0'
@(test)
test_slip0010_vector_4_solana_path :: proc(t: ^testing.T) {
	log.info("Testing SLIP-0010 Vector 4: Solana standard path m/44'/501'/0'/0'")

	// Test seed from known mnemonic
	mnemonic := []string{"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
	                     "abandon", "abandon", "abandon", "abandon", "abandon", "about"}
	seed, seed_err := keystore.mnemonic_to_seed(mnemonic, "")
	testing.expectf(t, seed_err == .None, "Mnemonic to seed failed: %v", seed_err)

	// Derive at Solana path
	derived_key, err := keystore.derive_from_path(seed, "m/44'/501'/0'/0'")
	testing.expectf(t, err == .None, "Solana path derivation failed: %v", err)

	// Verify derived key properties
	testing.expectf(t, derived_key.depth == 4, "Derived key depth should be 4, got %d", derived_key.depth)
	testing.expectf(t, len(derived_key.key) == 32, "Derived key should be 32 bytes, got %d", len(derived_key.key))

	// Convert to keypair
	keypair, keypair_err := keystore.hd_key_to_keypair(derived_key)
	testing.expectf(t, keypair_err == .None, "HDKey to keypair conversion failed: %v", keypair_err)
	testing.expectf(t, keypair.public_key._is_initialized, "Public key should be initialized")

	log.info("✓ SLIP-0010 Vector 4 (Solana path) passed")
}

// ============================================================================
// Derivation Properties Tests
// ============================================================================

// Test deterministic derivation (same seed + path = same key)
@(test)
test_bip32_deterministic :: proc(t: ^testing.T) {
	log.info("Testing BIP32 deterministic behavior")

	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Derive twice at same path
	key1, err1 := keystore.derive_from_path(seed, "m/44'/501'/0'/0'")
	testing.expectf(t, err1 == .None, "First derivation failed: %v", err1)

	key2, err2 := keystore.derive_from_path(seed, "m/44'/501'/0'/0'")
	testing.expectf(t, err2 == .None, "Second derivation failed: %v", err2)

	// Verify keys match
	keys_match := true
	for i := 0; i < 32; i += 1 {
		if key1.key[i] != key2.key[i] {
			keys_match = false
			break
		}
	}

	testing.expectf(t, keys_match, "Same path produced different keys (non-deterministic!)")

	log.info("✓ BIP32 deterministic test passed")
}

// Test different paths produce different keys
@(test)
test_bip32_different_paths :: proc(t: ^testing.T) {
	log.info("Testing BIP32 different paths produce different keys")

	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Derive at different paths
	key1, err1 := keystore.derive_from_path(seed, "m/44'/501'/0'/0'")
	testing.expectf(t, err1 == .None, "First derivation failed: %v", err1)

	key2, err2 := keystore.derive_from_path(seed, "m/44'/501'/1'/0'")
	testing.expectf(t, err2 == .None, "Second derivation failed: %v", err2)

	// Verify keys are different
	keys_match := true
	for i := 0; i < 32; i += 1 {
		if key1.key[i] != key2.key[i] {
			keys_match = false
			break
		}
	}

	testing.expectf(t, !keys_match, "Different paths produced identical keys (collision!)")

	log.info("✓ BIP32 different paths test passed")
}

// Test chain code changes with derivation
@(test)
test_bip32_chain_code_changes :: proc(t: ^testing.T) {
	log.info("Testing BIP32 chain code changes with derivation")

	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	// Get master key
	master, master_err := keystore.derive_master_key(seed)
	testing.expectf(t, master_err == .None, "Master key derivation failed: %v", master_err)

	// Derive child
	child, child_err := keystore.derive_from_path(seed, "m/0'")
	testing.expectf(t, child_err == .None, "Child derivation failed: %v", child_err)

	// Verify chain codes are different
	chain_codes_match := true
	for i := 0; i < 32; i += 1 {
		if master.chain_code[i] != child.chain_code[i] {
			chain_codes_match = false
			break
		}
	}

	testing.expectf(t, !chain_codes_match, "Parent and child have identical chain codes!")

	log.info("✓ BIP32 chain code changes test passed")
}

// ============================================================================
// Path Parsing Tests
// ============================================================================

// Test valid path parsing
@(test)
test_bip32_path_parsing_valid :: proc(t: ^testing.T) {
	log.info("Testing BIP32 valid path parsing")

	test_cases := []struct {
		path: string,
		expected_depth: int,
	}{
		{"m", 0},
		{"m/0'", 1},
		{"m/44'/501'", 2},
		{"m/44'/501'/0'", 3},
		{"m/44'/501'/0'/0'", 4},
	}

	for test_case in test_cases {
		indices, err := keystore.parse_derivation_path(test_case.path)
		defer if err == .None do delete(indices)

		testing.expectf(t, err == .None, "Path parsing failed for '%s': %v", test_case.path, err)
		testing.expectf(t, len(indices) == test_case.expected_depth,
			"Path '%s': expected depth %d, got %d",
			test_case.path, test_case.expected_depth, len(indices))
	}

	log.info("✓ BIP32 path parsing test passed")
}

// Test hardened path parsing and derivation
@(test)
test_bip32_hardened_indices :: proc(t: ^testing.T) {
	log.info("Testing BIP32 hardened indices")

	// All Solana paths use hardened derivation
	path := "m/44'/501'/0'/0'"
	indices, err := keystore.parse_derivation_path(path)
	defer if err == .None do delete(indices)

	testing.expectf(t, err == .None, "Path parsing failed: %v", err)
	testing.expectf(t, len(indices) == 4, "Expected 4 indices, got %d", len(indices))

	// Verify indices match expected values (44, 501, 0, 0)
	expected := []u32{44, 501, 0, 0}
	for i := 0; i < len(expected); i += 1 {
		testing.expectf(t, indices[i] == expected[i],
			"Index %d mismatch: expected %d, got %d", i, expected[i], indices[i])
	}

	// Test that derivation with hardened path succeeds
	seed: [64]byte
	test_bytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	                     0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	copy(seed[:], test_bytes)

	derived, derive_err := keystore.derive_from_path(seed, path)
	testing.expectf(t, derive_err == .None, "Hardened path derivation failed: %v", derive_err)

	log.info("✓ BIP32 hardened indices test passed")
}
