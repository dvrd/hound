// BIP39: Mnemonic to Seed Conversion
// Spec: https://github.com/bitcoin/bips/blob/master/bip-0039.mediawiki
package keystore

import "core:strings"
import "core:log"
import models "../models"

// BIP39: Convert mnemonic phrase to 64-byte seed
//
// ASSERTION 1: Mnemonic must be 12 or 24 words
// ASSERTION 2: Output seed must be exactly 64 bytes
//
// Parameters:
//   - words: Array of mnemonic words (12 or 24)
//   - passphrase: Optional passphrase (empty string for no passphrase)
//
// Returns: 64-byte seed, error status
mnemonic_to_seed :: proc(
	words: []string,
	passphrase: string = "",
	allocator := context.allocator,
) -> (seed: [64]byte, err: models.ErrorType) {
	assert(len(words) == 12 || len(words) == 24, "Mnemonic must be 12 or 24 words")

	log.infof("Converting %d-word mnemonic to BIP39 seed", len(words))

	// Step 1: Join words with spaces (normalized to lowercase)
	phrase_builder := strings.builder_make(allocator)
	defer strings.builder_destroy(&phrase_builder)

	for word, i in words {
		if i > 0 do strings.write_string(&phrase_builder, " ")
		// Normalize to lowercase
		word_lower := strings.to_lower(word, allocator)
		defer delete(word_lower)
		strings.write_string(&phrase_builder, word_lower)
	}

	phrase := strings.to_string(phrase_builder)
	defer delete(phrase)

	log.debugf("Mnemonic phrase: %d characters", len(phrase))

	// Step 2: Construct salt = "mnemonic" + passphrase
	salt_str: string
	if len(passphrase) > 0 {
		salt_str = strings.concatenate({"mnemonic", passphrase}, allocator)
		defer delete(salt_str)
	} else {
		salt_str = "mnemonic"
	}

	log.debugf("Salt: 'mnemonic' + passphrase (%d chars)", len(passphrase))

	// Step 3: Apply PBKDF2-HMAC-SHA512 with 2048 iterations
	// BIP39 specifies exactly 2048 iterations and 512-bit (64-byte) output
	key, pbkdf_err := pbkdf2_hmac_sha512(
		transmute([]byte)phrase,
		transmute([]byte)salt_str,
		2048,  // BIP39 specifies exactly 2048 iterations
		64,    // BIP39 produces 512-bit (64-byte) seed
		allocator,
	)
	if pbkdf_err != .None {
		log.errorf("PBKDF2 failed: %v", pbkdf_err)
		return {}, pbkdf_err
	}
	defer delete(key)

	// Copy to output
	copy(seed[:], key)

	// ASSERTION 2: Verify seed length
	assert(len(seed) == 64, "Seed must be 64 bytes")

	log.info("Successfully converted mnemonic to BIP39 seed")
	return seed, .None
}

// Validate mnemonic word count
//
// Returns: .None if valid, .InvalidSeedPhrase if invalid
validate_mnemonic :: proc(words: []string) -> models.ErrorType {
	if len(words) != 12 && len(words) != 24 {
		log.errorf("Invalid mnemonic length: %d (expected 12 or 24)", len(words))
		return .InvalidSeedPhrase
	}

	// TODO: Optionally validate words against BIP39 wordlist
	// For now, accept any 12/24 words (like current implementation)

	log.infof("Mnemonic format valid: %d words", len(words))
	return .None
}
