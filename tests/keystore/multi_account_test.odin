// Multi-Account Test - Phase 2 Validation
// Tests BIP44 account derivation (m/44'/501'/ACCOUNT'/0')
#+feature global-context
package tests

import "core:fmt"
import "core:testing"
import "core:log"
import keystore "../../src/lib/keystore"

// ============================================================================
// Multi-Account Derivation Tests
// ============================================================================

// Test deriving multiple accounts (0, 1, 2, 3, 4)
@(test)
test_multi_account_derivation :: proc(t: ^testing.T) {
	log.info("Testing multi-account derivation (accounts 0-4)")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	addresses: [5]string

	// Derive 5 accounts
	for account_index in 0 ..< 5 {
		keypair, err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", u32(account_index), 0)
		testing.expectf(t, err == .None, "Account %d derivation failed: %v", account_index, err)

		address := keystore.keypair_to_address(&keypair)
		keystore.zero_keypair(&keypair)

		addresses[account_index] = address
		log.infof("Account %d: %s", account_index, address)
	}

	// Verify all addresses are different
	for i := 0; i < 5; i += 1 {
		for j := i + 1; j < 5; j += 1 {
			testing.expectf(
				t,
				addresses[i] != addresses[j],
				"Account %d and %d produced same address!",
				i,
				j,
			)
		}
	}

	log.info("✓ Multi-account derivation test passed")
}

// Test account 0 (default)
@(test)
test_account_0_default :: proc(t: ^testing.T) {
	log.info("Testing account 0 (default)")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Explicit account 0
	keypair1, err1 := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 0, 0)
	testing.expectf(t, err1 == .None, "Explicit account 0 failed: %v", err1)
	address1 := keystore.keypair_to_address(&keypair1)
	keystore.zero_keypair(&keypair1)

	// Default (should be account 0)
	keypair2, err2 := keystore.derive_keypair_from_seed_bip44(mnemonic)
	testing.expectf(t, err2 == .None, "Default account failed: %v", err2)
	address2 := keystore.keypair_to_address(&keypair2)
	keystore.zero_keypair(&keypair2)

	// Should be identical
	testing.expectf(
		t,
		address1 == address2,
		"Explicit account 0 and default should match!\n  Explicit: %s\n  Default:  %s",
		address1,
		address2,
	)

	log.infof("Account 0 address: %s", address1)
	log.info("✓ Account 0 default test passed")
}

// Test high account indices (100, 1000, 10000)
@(test)
test_high_account_indices :: proc(t: ^testing.T) {
	log.info("Testing high account indices")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	high_indices := []u32{100, 1000, 10000}

	for index in high_indices {
		keypair, err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", index, 0)
		testing.expectf(t, err == .None, "Account %d derivation failed: %v", index, err)

		address := keystore.keypair_to_address(&keypair)
		keystore.zero_keypair(&keypair)

		testing.expectf(t, len(address) > 0, "Account %d produced empty address", index)
		log.infof("Account %d: %s", index, address)
	}

	log.info("✓ High account indices test passed")
}

// Test deterministic account derivation
@(test)
test_account_deterministic :: proc(t: ^testing.T) {
	log.info("Testing deterministic account derivation")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Derive account 1 twice
	keypair1, err1 := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 1, 0)
	testing.expectf(t, err1 == .None, "First derivation failed: %v", err1)
	address1 := keystore.keypair_to_address(&keypair1)
	keystore.zero_keypair(&keypair1)

	keypair2, err2 := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 1, 0)
	testing.expectf(t, err2 == .None, "Second derivation failed: %v", err2)
	address2 := keystore.keypair_to_address(&keypair2)
	keystore.zero_keypair(&keypair2)

	testing.expectf(
		t,
		address1 == address2,
		"Account 1 not deterministic!\n  First:  %s\n  Second: %s",
		address1,
		address2,
	)

	log.infof("Deterministic account 1: %s", address1)
	log.info("✓ Account deterministic test passed")
}

// Test account sequence (each account is independent)
@(test)
test_account_independence :: proc(t: ^testing.T) {
	log.info("Testing account independence")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Derive accounts 0, 1, 2
	addresses: [3]string

	for i in 0 ..< 3 {
		keypair, err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", u32(i), 0)
		testing.expectf(t, err == .None, "Account %d failed: %v", i, err)
		addresses[i] = keystore.keypair_to_address(&keypair)
		keystore.zero_keypair(&keypair)
	}

	// Each account should be different
	testing.expectf(t, addresses[0] != addresses[1], "Account 0 == Account 1")
	testing.expectf(t, addresses[0] != addresses[2], "Account 0 == Account 2")
	testing.expectf(t, addresses[1] != addresses[2], "Account 1 == Account 2")

	// Derive them in reverse order - should get same addresses
	for i in 0 ..< 3 {
		reverse_idx := 2 - i
		keypair, err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", u32(reverse_idx), 0)
		testing.expectf(t, err == .None, "Reverse account %d failed: %v", reverse_idx, err)
		reverse_address := keystore.keypair_to_address(&keypair)
		keystore.zero_keypair(&keypair)

		testing.expectf(
			t,
			reverse_address == addresses[reverse_idx],
			"Account %d changed when derived in different order!",
			reverse_idx,
		)
	}

	log.info("✓ Account independence test passed")
}

// Test account with different mnemonics
@(test)
test_accounts_different_mnemonics :: proc(t: ^testing.T) {
	log.info("Testing accounts with different mnemonics")

	mnemonic1 := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	mnemonic2 := []string{
		"zoo", "zoo", "zoo", "zoo", "zoo", "zoo",
		"zoo", "zoo", "zoo", "zoo", "zoo", "wrong",
	}

	// Same account index, different mnemonics
	keypair1, err1 := keystore.derive_keypair_from_seed_bip44(mnemonic1, "", 0, 0)
	testing.expectf(t, err1 == .None, "Mnemonic 1 failed: %v", err1)
	address1 := keystore.keypair_to_address(&keypair1)
	keystore.zero_keypair(&keypair1)

	keypair2, err2 := keystore.derive_keypair_from_seed_bip44(mnemonic2, "", 0, 0)
	testing.expectf(t, err2 == .None, "Mnemonic 2 failed: %v", err2)
	address2 := keystore.keypair_to_address(&keypair2)
	keystore.zero_keypair(&keypair2)

	testing.expectf(
		t,
		address1 != address2,
		"Different mnemonics produced same address for account 0!",
	)

	log.infof("Mnemonic 1, Account 0: %s", address1)
	log.infof("Mnemonic 2, Account 0: %s", address2)
	log.info("✓ Different mnemonics test passed")
}
