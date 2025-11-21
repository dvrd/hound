// Argon2id password-based key derivation using libsodium
// Implements OWASP 2024 recommended parameters for secure password hashing
package keystore

import "core:c"
import "core:log"
import models "../models"

// Argon2id parameters (OWASP 2024 minimum)
ARGON2_MEMLIMIT_KB :: 19456  // 19 MiB
ARGON2_OPSLIMIT :: 2         // 2 iterations
ARGON2_PARALLELISM :: 1      // 1 thread
ARGON2_SALT_BYTES :: 16      // 128 bits
ARGON2_KEY_BYTES :: 32       // 256 bits for AES-256

// FFI bindings to libsodium
when ODIN_OS == .Darwin {
	foreign import sodium "/opt/homebrew/lib/libsodium.dylib"
} else when ODIN_OS == .Linux {
	foreign import sodium "system:sodium"
}

@(default_calling_convention="c")
foreign sodium {
	// int crypto_pwhash(unsigned char * const out, unsigned long long outlen,
	//                   const char * const passwd, unsigned long long passwdlen,
	//                   const unsigned char * const salt,
	//                   unsigned long long opslimit, size_t memlimit, int alg);
	crypto_pwhash :: proc(
		out: [^]byte,
		outlen: c.ulonglong,
		passwd: cstring,
		passwdlen: c.ulonglong,
		salt: [^]byte,
		opslimit: c.ulonglong,
		memlimit: c.size_t,
		alg: c.int,
	) -> c.int ---

	// void randombytes_buf(void * const buf, const size_t size);
	@(link_name="randombytes_buf")
	_randombytes_buf_argon2 :: proc(buf: rawptr, size: c.size_t) ---
}

// Algorithm constants
CRYPTO_PWHASH_ALG_ARGON2ID13 :: 2

// Generate cryptographically secure random salt
//
// ASSERTION 1: Salt must be exactly ARGON2_SALT_BYTES
generate_salt :: proc() -> [ARGON2_SALT_BYTES]byte {
	salt: [ARGON2_SALT_BYTES]byte
	salt_slice := salt[:]  // Create addressable slice
	_randombytes_buf_argon2(raw_data(salt_slice), ARGON2_SALT_BYTES)

	// ASSERTION 1: Verify salt was generated
	assert(len(salt) == ARGON2_SALT_BYTES, "Salt must be 16 bytes")

	log.debug("Generated cryptographically secure salt")
	return salt
}

// Derive encryption key from password using Argon2id
//
// ASSERTION 1: Password must not be empty
// ASSERTION 2: Salt must be exactly 16 bytes
// ASSERTION 3: Derived key must be 32 bytes
derive_key_from_password :: proc(
	password: string,
	salt: [ARGON2_SALT_BYTES]byte,
) -> (key: [ARGON2_KEY_BYTES]byte, err: models.ErrorType) {
	assert(len(password) > 0, "Password cannot be empty")
	assert(len(salt) == ARGON2_SALT_BYTES, "Salt must be 16 bytes")

	log.debugf("Deriving key with Argon2id (m=%dKB, t=%d, p=%d)",
		ARGON2_MEMLIMIT_KB, ARGON2_OPSLIMIT, ARGON2_PARALLELISM)

	// Convert password to cstring for C FFI
	password_cstr := cstring(raw_data(password))

	// Create addressable slices for FFI
	key_data := key
	salt_data := salt

	key_slice := key_data[:]
	salt_slice := salt_data[:]

	result := crypto_pwhash(
		raw_data(key_slice),
		ARGON2_KEY_BYTES,
		password_cstr,
		c.ulonglong(len(password)),
		raw_data(salt_slice),
		ARGON2_OPSLIMIT,
		ARGON2_MEMLIMIT_KB * 1024,
		CRYPTO_PWHASH_ALG_ARGON2ID13,
	)

	if result != 0 {
		log.error("Argon2id key derivation failed")
		return {}, .CryptoOperationFailed
	}

	// ASSERTION 3: Key derived
	assert(len(key) == ARGON2_KEY_BYTES, "Derived key must be 32 bytes")

	log.info("Successfully derived encryption key from password")
	return key, .None
}

// Hash password for verification (store this, never the password itself)
//
// ASSERTION 1: Password must not be empty
// ASSERTION 2: Salt must be exactly 16 bytes
hash_password :: proc(
	password: string,
	salt: [ARGON2_SALT_BYTES]byte,
) -> (hash: [ARGON2_KEY_BYTES]byte, err: models.ErrorType) {
	assert(len(password) > 0, "Password cannot be empty")
	assert(len(salt) == ARGON2_SALT_BYTES, "Salt must be 16 bytes")

	// Reuse derive_key_from_password for hashing
	return derive_key_from_password(password, salt)
}
