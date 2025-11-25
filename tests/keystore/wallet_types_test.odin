// Wallet Types Test - Phase 2 Validation
// Tests all 4 wallet derivation types: Legacy, BIP44_Standard, BIP44_Change, Solana_CLI
#+feature global-context
package tests

import "core:fmt"
import "core:testing"
import "core:log"
import keystore "../../src/lib/keystore"

// ============================================================================
// Wallet Type Tests - All 4 Types
// ============================================================================

// Test all 4 wallet types produce valid addresses
@(test)
test_all_wallet_types :: proc(t: ^testing.T) {
	log.info("Testing all 4 wallet types")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Test each wallet type
	wallet_types := []string{"Legacy", "BIP44_Standard", "BIP44_Change", "Solana_CLI"}
	addresses: [4]string

	// Legacy (SHA-256)
	keypair_legacy, err_legacy := keystore.derive_keypair_from_seed(mnemonic)
	testing.expectf(t, err_legacy == .None, "Legacy derivation failed: %v", err_legacy)
	addresses[0] = keystore.keypair_to_address(&keypair_legacy)
	keystore.zero_keypair(&keypair_legacy)
	log.infof("Legacy:          %s", addresses[0])

	// BIP44 Standard: m/44'/501'/0'/0'
	keypair_std, err_std := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 0, 0)
	testing.expectf(t, err_std == .None, "BIP44_Standard derivation failed: %v", err_std)
	addresses[1] = keystore.keypair_to_address(&keypair_std)
	keystore.zero_keypair(&keypair_std)
	log.infof("BIP44_Standard:  %s", addresses[1])

	// BIP44-Change: m/44'/501'/0'
	seed, seed_err := keystore.mnemonic_to_seed(mnemonic, "")
	testing.expectf(t, seed_err == .None, "Seed generation failed: %v", seed_err)

	hd_key_change, derive_err := keystore.derive_from_path(seed, "m/44'/501'/0'")
	testing.expectf(t, derive_err == .None, "BIP44_Change derivation failed: %v", derive_err)

	keypair_change, keypair_err := keystore.hd_key_to_keypair(hd_key_change)
	testing.expectf(t, keypair_err == .None, "HDKey conversion failed: %v", keypair_err)
	addresses[2] = keystore.keypair_to_address(&keypair_change)
	keystore.zero_keypair(&keypair_change)
	log.infof("BIP44_Change:    %s", addresses[2])

	// Solana CLI: m/44'/501'
	hd_key_cli, cli_err := keystore.derive_from_path(seed, "m/44'/501'")
	testing.expectf(t, cli_err == .None, "Solana_CLI derivation failed: %v", cli_err)

	keypair_cli, cli_keypair_err := keystore.hd_key_to_keypair(hd_key_cli)
	testing.expectf(t, cli_keypair_err == .None, "CLI HDKey conversion failed: %v", cli_keypair_err)
	addresses[3] = keystore.keypair_to_address(&keypair_cli)
	keystore.zero_keypair(&keypair_cli)
	log.infof("Solana_CLI:      %s", addresses[3])

	// Verify all addresses are different
	for i := 0; i < 4; i += 1 {
		for j := i + 1; j < 4; j += 1 {
			testing.expectf(
				t,
				addresses[i] != addresses[j],
				"Wallet types %s and %s produced same address!",
				wallet_types[i],
				wallet_types[j],
			)
		}
	}

	// Verify all addresses are valid format
	for address, i in addresses {
		testing.expectf(t, len(address) >= 32 && len(address) <= 44,
			"Wallet type %s produced invalid address length: %d",
			wallet_types[i], len(address))
	}

	log.info("✓ All 4 wallet types test passed")
}

// Test Legacy wallet type (SHA-256 derivation)
@(test)
test_legacy_wallet_type :: proc(t: ^testing.T) {
	log.info("Testing Legacy wallet type")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Derive twice to verify determinism
	keypair1, err1 := keystore.derive_keypair_from_seed(mnemonic)
	testing.expectf(t, err1 == .None, "First derivation failed: %v", err1)
	address1 := keystore.keypair_to_address(&keypair1)
	keystore.zero_keypair(&keypair1)

	keypair2, err2 := keystore.derive_keypair_from_seed(mnemonic)
	testing.expectf(t, err2 == .None, "Second derivation failed: %v", err2)
	address2 := keystore.keypair_to_address(&keypair2)
	keystore.zero_keypair(&keypair2)

	testing.expectf(t, address1 == address2, "Legacy derivation not deterministic!")

	log.infof("Legacy address: %s", address1)
	log.info("✓ Legacy wallet type test passed")
}

// Test BIP44 Standard wallet type (Phantom, Solflare, Ledger)
@(test)
test_bip44_standard_wallet_type :: proc(t: ^testing.T) {
	log.info("Testing BIP44_Standard wallet type")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Test default account (0)
	keypair, err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 0, 0)
	testing.expectf(t, err == .None, "Derivation failed: %v", err)
	address := keystore.keypair_to_address(&keypair)
	keystore.zero_keypair(&keypair)

	testing.expectf(t, len(address) > 0, "Address should not be empty")

	log.infof("BIP44_Standard address: %s", address)
	log.info("✓ BIP44_Standard wallet type test passed")
}

// Test backward compatibility (Legacy still works after Phase 2)
@(test)
test_backward_compatibility :: proc(t: ^testing.T) {
	log.info("Testing backward compatibility with Legacy wallets")

	mnemonic := []string{
		"abandon", "abandon", "abandon", "abandon", "abandon", "abandon",
		"abandon", "abandon", "abandon", "abandon", "abandon", "about",
	}

	// Both methods should work
	legacy_keypair, legacy_err := keystore.derive_keypair_from_seed(mnemonic)
	testing.expectf(t, legacy_err == .None, "Legacy method failed: %v", legacy_err)
	legacy_address := keystore.keypair_to_address(&legacy_keypair)
	keystore.zero_keypair(&legacy_keypair)

	bip44_keypair, bip44_err := keystore.derive_keypair_from_seed_bip44(mnemonic, "", 0, 0)
	testing.expectf(t, bip44_err == .None, "BIP44 method failed: %v", bip44_err)
	bip44_address := keystore.keypair_to_address(&bip44_keypair)
	keystore.zero_keypair(&bip44_keypair)

	// They should produce different addresses (different derivation methods)
	testing.expectf(
		t,
		legacy_address != bip44_address,
		"Legacy and BIP44 should produce different addresses!",
	)

	log.infof("Legacy address: %s", legacy_address)
	log.infof("BIP44 address:  %s", bip44_address)
	log.info("✓ Backward compatibility test passed")
}
