// BIP32: Hierarchical Deterministic Key Derivation for Ed25519
// Spec: SLIP-0010 (https://github.com/satoshilabs/slips/blob/master/slip-0010.md)
// Note: Ed25519 uses "ed25519 seed" (not "Bitcoin seed") and only supports hardened derivation
package keystore

import "core:crypto/hmac"
import "core:crypto/hash"
import "core:crypto/ed25519"
import "core:strings"
import "core:strconv"
import "core:log"
import models "../models"

// BIP32 Hierarchical Deterministic Key
//
// ASSERTION 1: Chain code must be exactly 32 bytes
// ASSERTION 2: Private key must be exactly 32 bytes
HDKey :: struct {
	key:          [32]byte,  // Private key (32 bytes for Ed25519)
	chain_code:   [32]byte,  // Chain code for child derivation
	depth:        u8,        // Depth in hierarchy (0 = master)
	fingerprint:  [4]byte,   // Parent key fingerprint
	child_index:  u32,       // Child index in parent
}

// BIP32 Hardened derivation flag (2^31)
HARDENED_OFFSET :: 0x80000000

// BIP32: Derive master key from BIP39 seed
//
// ASSERTION 1: Seed must be exactly 64 bytes (BIP39 output)
// ASSERTION 2: HMAC-SHA512 must produce exactly 64 bytes
// ASSERTION 3: Master key must be exactly 32 bytes
// ASSERTION 4: Chain code must be exactly 32 bytes
//
// SLIP-0010 Specification:
//   - Use "ed25519 seed" as HMAC key (NOT "Bitcoin seed")
//   - Apply HMAC-SHA512(key="ed25519 seed", data=seed)
//   - Left 32 bytes = master private key
//   - Right 32 bytes = chain code
//
// Parameters:
//   - seed: 64-byte BIP39 seed
//
// Returns: Master HDKey, error status
derive_master_key :: proc(
	seed: [64]byte,
	allocator := context.allocator,
) -> (master: HDKey, err: models.ErrorType) {
	assert(len(seed) == 64, "Seed must be 64 bytes")

	log.info("Deriving BIP32 master key from seed")

	// SLIP-0010: Use "ed25519 seed" for Ed25519 curves
	hmac_key := "ed25519 seed"

	// Apply HMAC-SHA512
	hmac_output := make([]byte, 64, allocator)
	defer delete(hmac_output)

	// Create a local copy to get addressable slice
	seed_copy := seed
	hmac.sum(
		hash.Algorithm.SHA512,
		hmac_output,
		seed_copy[:],
		transmute([]byte)hmac_key,
	)

	// ASSERTION 2: Verify HMAC output length
	assert(len(hmac_output) == 64, "HMAC-SHA512 must produce 64 bytes")

	// Split result: left 32 bytes = key, right 32 bytes = chain code
	master.key = [32]byte{}
	master.chain_code = [32]byte{}

	copy(master.key[:], hmac_output[:32])
	copy(master.chain_code[:], hmac_output[32:64])

	// Initialize master key metadata
	master.depth = 0
	master.fingerprint = {0, 0, 0, 0}
	master.child_index = 0

	// ASSERTION 3 & 4: Verify key and chain code lengths
	assert(len(master.key) == 32, "Master key must be 32 bytes")
	assert(len(master.chain_code) == 32, "Chain code must be 32 bytes")

	log.info("Successfully derived BIP32 master key")
	return master, .None
}

// BIP32: Derive child key from parent
//
// ASSERTION 1: Parent key must be exactly 32 bytes
// ASSERTION 2: Parent chain code must be exactly 32 bytes
// ASSERTION 3: Index must be hardened (>= 2^31) for Ed25519
// ASSERTION 4: HMAC-SHA512 must produce exactly 64 bytes
//
// SLIP-0010 Ed25519 Specification:
//   - Only hardened derivation supported (index >= 2^31)
//   - data = 0x00 || parent_key || index (big-endian u32)
//   - Apply HMAC-SHA512(key=parent_chain_code, data=data)
//   - Left 32 bytes = child private key
//   - Right 32 bytes = child chain code
//
// Parameters:
//   - parent: Parent HDKey
//   - index: Child index (must be hardened)
//   - hardened: Must be true for Ed25519
//
// Returns: Child HDKey, error status
derive_child_key :: proc(
	parent: HDKey,
	index: u32,
	hardened: bool,
	allocator := context.allocator,
) -> (child: HDKey, err: models.ErrorType) {
	assert(len(parent.key) == 32, "Parent key must be 32 bytes")
	assert(len(parent.chain_code) == 32, "Parent chain code must be 32 bytes")

	// SLIP-0010: Ed25519 only supports hardened derivation
	if !hardened {
		log.error("Ed25519 only supports hardened derivation")
		return {}, .CryptoOperationFailed
	}

	// Calculate hardened index
	hardened_index := index | HARDENED_OFFSET

	// ASSERTION 3: Verify hardened index
	assert(hardened_index >= HARDENED_OFFSET, "Index must be hardened for Ed25519")

	log.debugf("Deriving hardened child key at index %d'", index)

	// Construct HMAC data: 0x00 || parent_key (32 bytes) || index (4 bytes big-endian)
	data := make([]byte, 1 + 32 + 4, allocator)
	defer delete(data)

	data[0] = 0x00  // Hardened derivation prefix
	// Copy parent key
	parent_key_copy := parent.key
	copy(data[1:33], parent_key_copy[:])

	// Append index as big-endian u32
	data[33] = u8((hardened_index >> 24) & 0xFF)
	data[34] = u8((hardened_index >> 16) & 0xFF)
	data[35] = u8((hardened_index >> 8) & 0xFF)
	data[36] = u8(hardened_index & 0xFF)

	// Apply HMAC-SHA512 with parent chain code as key
	hmac_output := make([]byte, 64, allocator)
	defer delete(hmac_output)

	chain_code_copy := parent.chain_code
	hmac.sum(
		hash.Algorithm.SHA512,
		hmac_output,
		data,
		chain_code_copy[:],
	)

	// ASSERTION 4: Verify HMAC output length
	assert(len(hmac_output) == 64, "HMAC-SHA512 must produce 64 bytes")

	// Split result: left 32 bytes = child key, right 32 bytes = child chain code
	child.key = [32]byte{}
	child.chain_code = [32]byte{}

	copy(child.key[:], hmac_output[:32])
	copy(child.chain_code[:], hmac_output[32:64])

	// Update child metadata
	child.depth = parent.depth + 1
	child.child_index = hardened_index

	// TODO: Calculate parent fingerprint (hash of parent public key)
	// For now, use placeholder
	child.fingerprint = {0, 0, 0, 0}

	log.debugf("Successfully derived child key at depth %d", child.depth)
	return child, .None
}

// BIP32: Parse derivation path string
//
// ASSERTION 1: Path must start with "m/" or "m"
// ASSERTION 2: Each index must be valid u32
// ASSERTION 3: Ed25519 requires all indices to be hardened (marked with ')
//
// Examples:
//   - "m/44'/501'/0'/0'" -> [44', 501', 0', 0']
//   - "m/44'/501'/0'"    -> [44', 501', 0']
//
// Parameters:
//   - path: Derivation path string (e.g., "m/44'/501'/0'/0'")
//
// Returns: Array of hardened indices, error status
parse_derivation_path :: proc(
	path: string,
	allocator := context.allocator,
) -> (indices: []u32, err: models.ErrorType) {
	// ASSERTION 1: Verify path starts with "m/" or "m"
	if !strings.has_prefix(path, "m/") && path != "m" {
		log.errorf("Invalid derivation path: must start with 'm/' or 'm', got '%s'", path)
		return nil, .InvalidSeedPhrase
	}

	// Handle "m" (master only)
	if path == "m" {
		log.info("Parsing master-only path")
		return make([]u32, 0, allocator), .None
	}

	// Remove "m/" prefix
	path_trimmed := strings.trim_prefix(path, "m/")

	// Split by '/'
	parts := strings.split(path_trimmed, "/", allocator)
	defer delete(parts)

	if len(parts) == 0 {
		log.info("Parsing master-only path (empty after m/)")
		return make([]u32, 0, allocator), .None
	}

	log.debugf("Parsing derivation path with %d components", len(parts))

	// Parse each index
	indices = make([]u32, len(parts), allocator)

	for part, i in parts {
		// Check for hardened marker
		is_hardened := strings.has_suffix(part, "'") || strings.has_suffix(part, "h")

		// ASSERTION 3: Ed25519 requires hardened derivation
		if !is_hardened {
			log.errorf("Ed25519 requires hardened derivation, but index %d is not hardened: '%s'", i, part)
			delete(indices)
			return nil, .InvalidSeedPhrase
		}

		// Remove hardened marker
		index_str := strings.trim_suffix(strings.trim_suffix(part, "'"), "h")

		// Parse index
		index, parse_ok := strconv.parse_u64(index_str)
		if !parse_ok || index > u64(max(u32)) {
			log.errorf("Invalid index at position %d: '%s'", i, part)
			delete(indices)
			return nil, .InvalidSeedPhrase
		}

		// ASSERTION 2: Verify valid u32
		assert(index <= u64(max(u32)), "Index must fit in u32")

		indices[i] = u32(index)
		log.debugf("Parsed index %d: %d' (hardened)", i, indices[i])
	}

	log.infof("Successfully parsed %d-level derivation path", len(indices))
	return indices, .None
}

// BIP32: Derive key from full derivation path
//
// ASSERTION 1: Seed must be exactly 64 bytes
// ASSERTION 2: Path must parse successfully
//
// This is the main entry point for BIP44 derivation.
//
// Example:
//   seed := [64]byte{...}  // From BIP39
//   key, err := derive_from_path(seed, "m/44'/501'/0'/0'")
//
// Parameters:
//   - seed: 64-byte BIP39 seed
//   - path: Derivation path (e.g., "m/44'/501'/0'/0'")
//
// Returns: Final derived HDKey, error status
derive_from_path :: proc(
	seed: [64]byte,
	path: string,
	allocator := context.allocator,
) -> (key: HDKey, err: models.ErrorType) {
	assert(len(seed) == 64, "Seed must be 64 bytes")

	log.infof("Deriving key from path: %s", path)

	// Step 1: Parse path
	indices, parse_err := parse_derivation_path(path, allocator)
	if parse_err != .None {
		log.errorf("Failed to parse path: %v", parse_err)
		return {}, parse_err
	}
	defer delete(indices)

	// ASSERTION 2: Verify path parsed
	if parse_err != .None {
		return {}, parse_err
	}

	// Step 2: Derive master key
	derived_key, master_err := derive_master_key(seed, allocator)
	if master_err != .None {
		log.errorf("Failed to derive master key: %v", master_err)
		return {}, master_err
	}

	// Step 3: Derive each child in path
	for index, i in indices {
		child, child_err := derive_child_key(derived_key, index, true, allocator)
		if child_err != .None {
			log.errorf("Failed to derive child at index %d': %v", index, child_err)
			return {}, child_err
		}
		derived_key = child
		log.debugf("Derived child %d/%d at index %d'", i + 1, len(indices), index)
	}

	log.infof("Successfully derived key at path %s (depth %d)", path, derived_key.depth)
	return derived_key, .None
}

// Convert HDKey to Ed25519 keypair
//
// ASSERTION 1: HDKey private key must be exactly 32 bytes
//
// Parameters:
//   - hd_key: BIP32 HDKey with 32-byte private key
//
// Returns: Ed25519 keypair (public + private)
hd_key_to_keypair :: proc(
	hd_key: HDKey,
) -> (keypair: Keypair, err: models.ErrorType) {
	assert(len(hd_key.key) == 32, "HDKey must have 32-byte private key")

	log.debug("Converting HDKey to Ed25519 keypair")

	// Ed25519: Initialize private key from 32-byte seed
	key_copy := hd_key.key
	if !ed25519.private_key_set_bytes(&keypair.private_key_struct, key_copy[:]) {
		log.error("Failed to initialize Ed25519 private key")
		return {}, .CryptoOperationFailed
	}

	// Extract public key from initialized private key
	ed25519.public_key_set_priv(&keypair.public_key, &keypair.private_key_struct)

	log.debug("Successfully converted HDKey to keypair")
	return keypair, .None
}
