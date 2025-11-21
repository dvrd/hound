// AES-256-GCM authenticated encryption
// Implements secure encryption with authentication tags
package keystore

import "core:crypto/aead"
import "core:log"
import "core:c"
import models "../models"

AES_KEY_BYTES :: 32      // 256 bits
GCM_NONCE_BYTES :: 12    // 96 bits (standard for GCM)
GCM_TAG_BYTES :: 16      // 128 bits

// Import randombytes_buf from libsodium (declared once for whole package)
foreign import sodium "system:sodium"

@(default_calling_convention="c")
foreign sodium {
	@(link_name="randombytes_buf")
	_randombytes_buf :: proc(buf: rawptr, size: c.size_t) ---
}

// Encrypted data container with all components needed for decryption
EncryptedData :: struct {
	ciphertext: []byte,
	nonce:      [GCM_NONCE_BYTES]byte,
	tag:        [GCM_TAG_BYTES]byte,
}

// Generate cryptographically secure random nonce
//
// ASSERTION 1: Nonce must be exactly GCM_NONCE_BYTES
generate_nonce :: proc() -> [GCM_NONCE_BYTES]byte {
	nonce: [GCM_NONCE_BYTES]byte
	nonce_slice := nonce[:]  // Create addressable slice

	// Use libsodium's secure random number generator
	_randombytes_buf(raw_data(nonce_slice), GCM_NONCE_BYTES)

	// ASSERTION 1: Verify nonce was generated
	assert(len(nonce) == GCM_NONCE_BYTES, "Nonce must be 12 bytes")

	log.debug("Generated cryptographically secure nonce")
	return nonce
}

// Encrypt data with AES-256-GCM
//
// ASSERTION 1: Plaintext must not be empty
// ASSERTION 2: Key must be exactly 32 bytes
// ASSERTION 3: Nonce must be exactly 12 bytes
// ASSERTION 4: Ciphertext and tag must be generated
encrypt_aes256gcm :: proc(
	plaintext: []byte,
	key: [AES_KEY_BYTES]byte,
	nonce: [GCM_NONCE_BYTES]byte,
) -> (encrypted: EncryptedData, err: models.ErrorType) {
	assert(len(plaintext) > 0, "Plaintext cannot be empty")
	assert(len(key) == AES_KEY_BYTES, "Key must be 32 bytes")
	assert(len(nonce) == GCM_NONCE_BYTES, "Nonce must be 12 bytes")

	log.debugf("Encrypting %d bytes with AES-256-GCM", len(plaintext))

	// Allocate ciphertext and tag buffers
	ciphertext := make([]byte, len(plaintext))
	tag: [GCM_TAG_BYTES]byte

	// Create addressable slices
	key_data := key
	nonce_data := nonce
	tag_data := tag

	key_slice := key_data[:]
	nonce_slice := nonce_data[:]
	tag_slice := tag_data[:]

	// Use seal_oneshot for AES-256-GCM encryption
	// seal_oneshot(algo, dst, tag, key, iv, aad, plaintext)
	aead.seal_oneshot(.AES_GCM_256, ciphertext, tag_slice, key_slice, nonce_slice, nil, plaintext)

	// ASSERTION 4: Verify encryption succeeded
	assert(len(ciphertext) == len(plaintext), "Ciphertext length mismatch")
	assert(len(tag) == GCM_TAG_BYTES, "Tag must be 16 bytes")

	encrypted.ciphertext = ciphertext
	encrypted.nonce = nonce
	encrypted.tag = tag

	log.info("Successfully encrypted data with AES-256-GCM")
	return encrypted, .None
}

// Decrypt data with AES-256-GCM
//
// ASSERTION 1: Ciphertext must not be empty
// ASSERTION 2: Key must be exactly 32 bytes
// ASSERTION 3: Nonce must be exactly 12 bytes
// ASSERTION 4: Tag must be exactly 16 bytes
// ASSERTION 5: Plaintext must be recovered
decrypt_aes256gcm :: proc(
	encrypted: EncryptedData,
	key: [AES_KEY_BYTES]byte,
) -> (plaintext: []byte, err: models.ErrorType) {
	assert(len(encrypted.ciphertext) > 0, "Ciphertext cannot be empty")
	assert(len(key) == AES_KEY_BYTES, "Key must be 32 bytes")
	assert(len(encrypted.nonce) == GCM_NONCE_BYTES, "Nonce must be 12 bytes")
	assert(len(encrypted.tag) == GCM_TAG_BYTES, "Tag must be 16 bytes")

	log.debugf("Decrypting %d bytes with AES-256-GCM", len(encrypted.ciphertext))

	// Allocate plaintext buffer
	plaintext = make([]byte, len(encrypted.ciphertext))

	// Create addressable slices
	key_data := key
	nonce_data := encrypted.nonce
	tag_data := encrypted.tag

	key_slice := key_data[:]
	nonce_slice := nonce_data[:]
	tag_slice := tag_data[:]

	// Use open_oneshot for AES-256-GCM decryption
	// open_oneshot(algo, dst, key, iv, aad, ciphertext, tag) -> bool
	success := aead.open_oneshot(.AES_GCM_256, plaintext, key_slice, nonce_slice, nil, encrypted.ciphertext, tag_slice)

	if !success {
		log.error("AES-GCM decryption failed (wrong key or corrupted data)")
		delete(plaintext)
		return nil, .CryptoOperationFailed
	}

	// ASSERTION 5: Verify decryption succeeded
	assert(len(plaintext) == len(encrypted.ciphertext), "Plaintext length mismatch")

	log.info("Successfully decrypted data with AES-256-GCM")
	return plaintext, .None
}
