// Integration tests for BIP39/BIP32/BIP44 implementation
// Tests the complete flow from mnemonic to Solana keypair
package test_bip44

import "core:fmt"
import "core:testing"
import "core:log"
import keystore "../src/lib/keystore"

// BIP39 Test Vector 1 (12 words)
// Source: https://github.com/trezor/python-mnemonic/blob/master/vectors.json
TEST_MNEMONIC_12 := []string{
	"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
	"abandon", "abandon", "abandon", "abandon", "abandon", "about",
}

// Expected BIP39 seed (first 32 bytes) for TEST_MNEMONIC_12 with empty passphrase
// Full seed: 5eb00bbddcf069084889a8ab9155568165f5c453ccb85e70811aaed6f6da5fc19a5ac40b389cd370d086206dec8aa6c43daea6690f20ad3d8d48b2d2ce9e38e4
EXPECTED_SEED_PREFIX_12 := []byte{
	0x5e, 0xb0, 0x0b, 0xbd, 0xdc, 0xf0, 0x69, 0x08,
	0x48, 0x89, 0xa8, 0xab, 0x91, 0x55, 0x56, 0x81,
	0x65, 0xf5, 0xc4, 0x53, 0xcc, 0xb8, 0x5e, 0x70,
	0x81, 0x1a, 0xae, 0xd6, 0xf6, 0xda, 0x5f, 0xc1,
}

@(test)
test_bip39_mnemonic_to_seed :: proc(t: ^testing.T) {
	// Test BIP39 mnemonic-to-seed conversion
	log.info("Testing BIP39 mnemonic-to-seed conversion")

	seed, err := keystore.mnemonic_to_seed(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		err == .None,
		"BIP39 mnemonic-to-seed failed with error: %v",
		err,
	)

	// Verify first 32 bytes of seed match expected
	matches := true
	for i := 0; i < 32; i += 1 {
		if seed[i] != EXPECTED_SEED_PREFIX_12[i] {
			matches = false
			break
		}
	}

	testing.expectf(
		t,
		matches,
		"BIP39 seed mismatch. Expected prefix: %x, got: %x",
		EXPECTED_SEED_PREFIX_12,
		seed[:32],
	)

	log.infof("✓ BIP39 test vector verified: first 32 bytes match")
}

@(test)
test_bip32_master_key_derivation :: proc(t: ^testing.T) {
	// Test BIP32 master key derivation
	log.info("Testing BIP32 master key derivation")

	seed, seed_err := keystore.mnemonic_to_seed(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		seed_err == .None,
		"BIP39 seed generation failed: %v",
		seed_err,
	)

	master_key, master_err := keystore.derive_master_key(seed)
	testing.expectf(
		t,
		master_err == .None,
		"BIP32 master key derivation failed: %v",
		master_err,
	)

	// Verify master key properties
	testing.expectf(
		t,
		master_key.depth == 0,
		"Master key depth should be 0, got %d",
		master_key.depth,
	)

	testing.expectf(
		t,
		len(master_key.key) == 32,
		"Master key should be 32 bytes, got %d",
		len(master_key.key),
	)

	testing.expectf(
		t,
		len(master_key.chain_code) == 32,
		"Chain code should be 32 bytes, got %d",
		len(master_key.chain_code),
	)

	log.infof("✓ BIP32 master key derived successfully")
}

@(test)
test_bip32_path_parsing :: proc(t: ^testing.T) {
	// Test BIP32 path parsing
	log.info("Testing BIP32 path parsing")

	test_cases := []struct {
		path:     string,
		expected: []u32,
		should_succeed: bool,
	}{
		{"m", {}, true},
		{"m/44'/501'/0'/0'", {44, 501, 0, 0}, true},
		{"m/44'/501'/1'/0'", {44, 501, 1, 0}, true},
		{"m/44'/501'/0'/1'", {44, 501, 0, 1}, true},
	}

	for test_case in test_cases {
		indices, err := keystore.parse_derivation_path(test_case.path)
		defer if err == .None do delete(indices)

		if test_case.should_succeed {
			testing.expectf(
				t,
				err == .None,
				"Path parsing failed for '%s': %v",
				test_case.path,
				err,
			)

			testing.expectf(
				t,
				len(indices) == len(test_case.expected),
				"Path '%s': expected %d indices, got %d",
				test_case.path,
				len(test_case.expected),
				len(indices),
			)

			// Verify indices match
			for i := 0; i < len(test_case.expected); i += 1 {
				testing.expectf(
					t,
					indices[i] == test_case.expected[i],
					"Path '%s': index %d mismatch. Expected %d, got %d",
					test_case.path,
					i,
					test_case.expected[i],
					indices[i],
				)
			}
		} else {
			testing.expectf(
				t,
				err != .None,
				"Path parsing should have failed for '%s'",
				test_case.path,
			)
		}
	}

	log.infof("✓ BIP32 path parsing tests passed")
}

@(test)
test_bip44_full_derivation :: proc(t: ^testing.T) {
	// Test complete BIP44 derivation flow
	log.info("Testing BIP44 full derivation")

	seed, seed_err := keystore.mnemonic_to_seed(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		seed_err == .None,
		"BIP39 seed generation failed: %v",
		seed_err,
	)

	// Derive at Solana path: m/44'/501'/0'/0'
	path := "m/44'/501'/0'/0'"
	hd_key, derive_err := keystore.derive_from_path(seed, path)
	testing.expectf(
		t,
		derive_err == .None,
		"BIP44 derivation failed for path %s: %v",
		path,
		derive_err,
	)

	// Verify derived key properties
	testing.expectf(
		t,
		hd_key.depth == 4,
		"Derived key depth should be 4, got %d",
		hd_key.depth,
	)

	testing.expectf(
		t,
		len(hd_key.key) == 32,
		"Derived key should be 32 bytes, got %d",
		len(hd_key.key),
	)

	// Convert to keypair
	keypair, keypair_err := keystore.hd_key_to_keypair(hd_key)
	testing.expectf(
		t,
		keypair_err == .None,
		"HDKey to keypair conversion failed: %v",
		keypair_err,
	)

	testing.expectf(
		t,
		keypair.public_key._is_initialized,
		"Public key should be initialized",
	)

	log.infof("✓ BIP44 full derivation successful")
}

@(test)
test_derive_keypair_from_seed_bip44 :: proc(t: ^testing.T) {
	// Test the high-level BIP44 derivation function
	log.info("Testing derive_keypair_from_seed_bip44")

	// Test with default account (0) and change (0)
	keypair, err := keystore.derive_keypair_from_seed_bip44(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		err == .None,
		"derive_keypair_from_seed_bip44 failed: %v",
		err,
	)

	testing.expectf(
		t,
		keypair.public_key._is_initialized,
		"Public key should be initialized",
	)

	// Get Solana address
	address := keystore.keypair_to_address(&keypair)
	testing.expectf(
		t,
		len(address) > 0,
		"Address should not be empty",
	)

	log.infof("✓ Derived Solana address: %s", address)

	// Test with different account indices
	keypair1, err1 := keystore.derive_keypair_from_seed_bip44(TEST_MNEMONIC_12, "", 0, 0)
	keypair2, err2 := keystore.derive_keypair_from_seed_bip44(TEST_MNEMONIC_12, "", 1, 0)

	testing.expectf(t, err1 == .None, "Account 0 derivation failed: %v", err1)
	testing.expectf(t, err2 == .None, "Account 1 derivation failed: %v", err2)

	// Verify different accounts produce different keys
	addr1 := keystore.keypair_to_address(&keypair1)
	addr2 := keystore.keypair_to_address(&keypair2)

	testing.expectf(
		t,
		addr1 != addr2,
		"Different accounts should produce different addresses. Got: %s == %s",
		addr1,
		addr2,
	)

	log.infof("✓ Account 0 address: %s", addr1)
	log.infof("✓ Account 1 address: %s", addr2)
}

@(test)
test_backward_compatibility :: proc(t: ^testing.T) {
	// Verify legacy derive_keypair_from_seed still works
	log.info("Testing backward compatibility with legacy derivation")

	keypair_legacy, err_legacy := keystore.derive_keypair_from_seed(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		err_legacy == .None,
		"Legacy derivation failed: %v",
		err_legacy,
	)

	testing.expectf(
		t,
		keypair_legacy.public_key._is_initialized,
		"Legacy public key should be initialized",
	)

	addr_legacy := keystore.keypair_to_address(&keypair_legacy)
	testing.expectf(
		t,
		len(addr_legacy) > 0,
		"Legacy address should not be empty",
	)

	// Derive with new BIP44 method
	keypair_bip44, err_bip44 := keystore.derive_keypair_from_seed_bip44(TEST_MNEMONIC_12)
	testing.expectf(
		t,
		err_bip44 == .None,
		"BIP44 derivation failed: %v",
		err_bip44,
	)

	addr_bip44 := keystore.keypair_to_address(&keypair_bip44)

	// They should produce different addresses (different derivation methods)
	testing.expectf(
		t,
		addr_legacy != addr_bip44,
		"Legacy and BIP44 should produce different addresses (different derivation methods)",
	)

	log.infof("✓ Legacy address: %s", addr_legacy)
	log.infof("✓ BIP44 address:  %s", addr_bip44)
	log.infof("✓ Backward compatibility verified: both methods work independently")
}
