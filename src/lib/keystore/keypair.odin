// Ed25519 keypair generation from seed phrases
// Simplified BIP39-style derivation for Phase 1
package keystore

import "core:crypto/ed25519"
import "core:crypto/sha2"
import "core:strings"
import "core:fmt"
import "core:log"
import models "../models"

// Keypair represents an Ed25519 keypair for Solana transactions
Keypair :: struct {
	public_key:          ed25519.Public_Key,   // Ed25519 public key struct
	private_key_struct:  ed25519.Private_Key,   // Ed25519 private key struct
}

// Validate seed phrase format
//
// ASSERTION 1: Words array must be 12 or 24 length
// ASSERTION 2: Each word must not be empty
validate_seed_phrase_format :: proc(words: []string) -> models.ErrorType {
	// ASSERTION 1: Check length
	if len(words) != 12 && len(words) != 24 {
		log.errorf("Invalid seed phrase length: %d (expected 12 or 24)", len(words))
		return .InvalidSeedPhrase
	}

	// ASSERTION 2: Check for empty words
	for word, i in words {
		if len(strings.trim_space(word)) == 0 {
			log.errorf("Empty word at position %d in seed phrase", i + 1)
			return .InvalidSeedPhrase
		}
	}

	log.infof("Seed phrase format valid: %d words", len(words))
	return .None
}

// Derive entropy from seed phrase using SHA-256
//
// Phase 1 Simplified Approach:
// - Concatenate all words with spaces
// - Hash with SHA-256 to get 32 bytes of entropy
// - Use as Ed25519 seed
//
// ASSERTION 1: Words must not be empty
// ASSERTION 2: Entropy must be exactly 32 bytes
derive_entropy_from_seed :: proc(words: []string) -> (entropy: [32]byte, err: models.ErrorType) {
	assert(len(words) > 0, "Words array cannot be empty")

	// Concatenate words with single space separator
	phrase := strings.join(words, " ")
	defer delete(phrase)

	log.debugf("Deriving entropy from %d-word seed phrase", len(words))

	// Hash phrase with SHA-256 to get deterministic 32-byte entropy
	ctx: sha2.Context_256
	hash_output: [sha2.DIGEST_SIZE_256]byte

	sha2.init_256(&ctx)
	sha2.update(&ctx, transmute([]byte)phrase)
	sha2.final(&ctx, hash_output[:])

	// Copy hash to entropy array using builtin copy
	copy(entropy[:], hash_output[:])

	// ASSERTION 2: Verify entropy generated
	assert(len(entropy) == 32, "Entropy must be 32 bytes")

	log.debug("Successfully derived entropy from seed phrase")
	return entropy, .None
}

// Derive Ed25519 keypair from seed phrase (Legacy SHA-256 derivation)
//
// NOTE: This is the Phase 1 legacy implementation for backward compatibility.
// For new wallets, use derive_keypair_from_seed_bip44() instead.
//
// ASSERTION 1: Seed phrase must be valid (12/24 words)
// ASSERTION 2: Derived keypair must have valid public key
derive_keypair_from_seed :: proc(seed_phrase: []string) -> (keypair: Keypair, err: models.ErrorType) {
	assert(len(seed_phrase) == 12 || len(seed_phrase) == 24, "Invalid seed phrase length")

	// Validate seed phrase format
	validate_err := validate_seed_phrase_format(seed_phrase)
	if validate_err != .None {
		return {}, validate_err
	}

	// Derive entropy from seed phrase
	entropy, entropy_err := derive_entropy_from_seed(seed_phrase)
	if entropy_err != .None {
		return {}, entropy_err
	}

	log.debug("Generating Ed25519 keypair from entropy")

	// Initialize private key from 32-byte entropy (seed)
	priv_key: ed25519.Private_Key
	entropy_slice := entropy[:]
	success := ed25519.private_key_set_bytes(&priv_key, entropy_slice)

	if !success {
		log.error("Failed to initialize Ed25519 private key from entropy")
		return {}, .CryptoOperationFailed
	}

	// Extract public key from private key
	pub_key: ed25519.Public_Key
	ed25519.public_key_set_priv(&pub_key, &priv_key)

	// Store both keys
	keypair.private_key_struct = priv_key
	keypair.public_key = pub_key

	// ASSERTION 2: Verify public key derived
	assert(keypair.public_key._is_initialized, "Public key must be initialized")

	log.info("Successfully derived Ed25519 keypair from seed phrase")
	return keypair, .None
}

// Derive Ed25519 keypair from seed phrase using BIP39/BIP32/BIP44 standards
//
// This is the standard-compliant implementation that matches Phantom wallet:
// 1. BIP39: Convert mnemonic → 64-byte seed (PBKDF2-HMAC-SHA512)
// 2. BIP32: Derive master key from seed (SLIP-0010 Ed25519)
// 3. BIP44: Derive at path m/44'/501'/account'/change' (Solana derivation path)
//
// Default derivation path: m/44'/501'/0'/0' (first Solana account)
//
// ASSERTION 1: Seed phrase must be valid (12/24 words)
// ASSERTION 2: Derived keypair must have valid public key
//
// Parameters:
//   - seed_phrase: Array of 12 or 24 BIP39 mnemonic words
//   - passphrase: Optional BIP39 passphrase (empty string for no passphrase)
//   - account_index: BIP44 account index (default: 0)
//   - change_index: BIP44 change index (default: 0)
//
// Returns: Ed25519 keypair, error status
derive_keypair_from_seed_bip44 :: proc(
	seed_phrase: []string,
	passphrase: string = "",
	account_index: u32 = 0,
	change_index: u32 = 0,
	allocator := context.allocator,
) -> (keypair: Keypair, err: models.ErrorType) {
	assert(len(seed_phrase) == 12 || len(seed_phrase) == 24, "Invalid seed phrase length")

	log.infof(
		"Deriving keypair from seed using BIP44 path: m/44'/501'/%d'/%d'",
		account_index,
		change_index,
	)

	// Step 1: Validate seed phrase format
	validate_err := validate_seed_phrase_format(seed_phrase)
	if validate_err != .None {
		return {}, validate_err
	}

	// Step 2: BIP39 - Convert mnemonic to 64-byte seed
	bip39_seed, bip39_err := mnemonic_to_seed(seed_phrase, passphrase, allocator)
	if bip39_err != .None {
		log.errorf("BIP39 mnemonic-to-seed failed: %v", bip39_err)
		return {}, bip39_err
	}

	// Step 3: BIP32/BIP44 - Derive key at Solana path
	// Path: m/44'/501'/account'/change'
	// 44' = BIP44 purpose
	// 501' = Solana coin type
	// account' = account index (hardened)
	// change' = change index (hardened, 0 for external chain)
	derivation_path := fmt.aprintf("m/44'/501'/%d'/%d'", account_index, change_index, allocator = allocator)
	defer delete(derivation_path)

	log.debugf("Using derivation path: %s", derivation_path)

	hd_key, derive_err := derive_from_path(bip39_seed, derivation_path, allocator)
	if derive_err != .None {
		log.errorf("BIP32 derivation failed: %v", derive_err)
		return {}, derive_err
	}

	// Step 4: Convert HDKey to Ed25519 keypair
	derived_keypair, keypair_err := hd_key_to_keypair(hd_key)
	if keypair_err != .None {
		log.errorf("HDKey to keypair conversion failed: %v", keypair_err)
		return {}, keypair_err
	}

	// ASSERTION 2: Verify public key derived
	assert(derived_keypair.public_key._is_initialized, "Public key must be initialized")

	log.info("Successfully derived Ed25519 keypair using BIP44")
	return derived_keypair, .None
}

// Base58 alphabet for Solana addresses (must be variable for indexing)
BASE58_ALPHABET := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Encode bytes to Base58 (Bitcoin/Solana alphabet)
//
// ASSERTION 1: Data must not be empty
encode_base58 :: proc(data: []byte) -> string {
	assert(len(data) > 0, "Cannot encode empty data")

	// Count leading zeros
	leading_zeros := 0
	for b in data {
		if b != 0 {
			break
		}
		leading_zeros += 1
	}

	// Estimate output size (Base58 is ~1.37x longer than binary)
	size := len(data) * 138 / 100 + 1
	output := make([dynamic]byte, 0, size)
	defer delete(output)

	// Convert binary to Base58 using big integer arithmetic
	input := make([]byte, len(data))
	defer delete(input)
	copy(input, data)

	// Process each byte
	for i := leading_zeros; i < len(data); i += 1 {
		carry := int(input[i])
		for j := 0; j < len(output); j += 1 {
			carry += int(output[j]) * 256
			output[j] = byte(carry % 58)
			carry /= 58
		}

		// Append remaining carry
		for carry > 0 {
			append(&output, byte(carry % 58))
			carry /= 58
		}
	}

	// Build result string: leading '1's + reversed Base58 digits
	result_len := leading_zeros + len(output)
	result := make([]byte, result_len)
	defer delete(result)

	// Add leading '1's for leading zero bytes
	for i := 0; i < leading_zeros; i += 1 {
		result[i] = '1'
	}

	// Convert Base58 digits to characters (reverse order)
	for i := 0; i < len(output); i += 1 {
		result[leading_zeros + i] = BASE58_ALPHABET[output[len(output) - 1 - i]]
	}

	return strings.clone_from_bytes(result)
}

// Get base58-encoded Solana address from keypair
//
// ASSERTION 1: Public key must be initialized
keypair_to_address :: proc(keypair: ^Keypair) -> string {
	assert(keypair != nil, "Keypair pointer cannot be nil")
	assert(keypair.public_key._is_initialized, "Public key must be initialized")

	log.debug("Converting public key to Solana address")

	// Extract public key bytes from Ed25519 struct
	public_key_bytes: [ed25519.PUBLIC_KEY_SIZE]byte
	ed25519.public_key_bytes(&keypair.public_key, public_key_bytes[:])

	// Base58 encode public key
	address := encode_base58(public_key_bytes[:])

	log.infof("Generated Solana address: %s", address)
	return address
}

// Zero keypair memory securely
//
// ASSERTION 1: Keypair pointer cannot be nil
zero_keypair :: proc(keypair: ^Keypair) {
	assert(keypair != nil, "Keypair pointer cannot be nil")

	// Zero the private key using ed25519's clear function
	ed25519.private_key_clear(&keypair.private_key_struct)

	// Zero public key bytes
	public_key_bytes: [ed25519.PUBLIC_KEY_SIZE]byte
	secure_zero_memory(&public_key_bytes, size_of(public_key_bytes))

	log.debug("Keypair memory securely zeroed")
}
