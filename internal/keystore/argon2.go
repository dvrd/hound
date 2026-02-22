package keystore

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"
)

// Argon2 V1 parameters (original — OWASP floor).
// Kept for backward-compatible verification of existing wallets.
const (
	Argon2V1MemoryKB    = 19456 // 19 MiB
	Argon2V1Iterations  = 2
	Argon2V1Parallelism = 1
)

// Argon2 V2 parameters (hardened — crypto wallet grade).
// Used for all new key derivations and migrations.
const (
	Argon2V2MemoryKB    = 65536 // 64 MiB
	Argon2V2Iterations  = 3
	Argon2V2Parallelism = 4
)

// Shared constants (unchanged across versions).
const (
	Argon2SaltBytes = 16
	Argon2KeyBytes  = 32
)

// Backward-compat aliases so existing code that references the old names still compiles.
// TODO: Remove these after all callers are migrated.
const (
	Argon2MemoryKB    = Argon2V1MemoryKB
	Argon2Iterations  = Argon2V1Iterations
	Argon2Parallelism = Argon2V1Parallelism
)

// Argon2Version identifies which parameter set was used.
type Argon2Version int

const (
	Argon2VersionV1 Argon2Version = 1
	Argon2VersionV2 Argon2Version = 2
)

// GenerateSalt returns 16 cryptographically random bytes.
func GenerateSalt() ([Argon2SaltBytes]byte, error) {
	var salt [Argon2SaltBytes]byte
	_, err := rand.Read(salt[:])
	return salt, err
}

// DeriveKey derives a 32-byte key using Argon2id with V2 (hardened) parameters.
// The intermediate heap-allocated slice is zeroed after copy.
func DeriveKey(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	return DeriveKeyV2(password, salt)
}

// DeriveKeyV1 derives a 32-byte key using the original V1 parameters.
// Used only for verifying/decrypting existing wallets before migration.
func DeriveKeyV1(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	raw := argon2.IDKey(
		[]byte(password),
		salt[:],
		Argon2V1Iterations,
		Argon2V1MemoryKB,
		Argon2V1Parallelism,
		Argon2KeyBytes,
	)
	var key [Argon2KeyBytes]byte
	copy(key[:], raw)
	ZeroBytes(raw) // M14: zero intermediate allocation
	return key
}

// DeriveKeyV2 derives a 32-byte key using the hardened V2 parameters.
// Used for all new imports and post-migration wallets.
func DeriveKeyV2(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	raw := argon2.IDKey(
		[]byte(password),
		salt[:],
		Argon2V2Iterations,
		Argon2V2MemoryKB,
		Argon2V2Parallelism,
		Argon2KeyBytes,
	)
	var key [Argon2KeyBytes]byte
	copy(key[:], raw)
	ZeroBytes(raw) // M14: zero intermediate allocation
	return key
}

// DeriveKeyVersioned derives a key using the specified version's parameters.
func DeriveKeyVersioned(password string, salt [Argon2SaltBytes]byte, version Argon2Version) [Argon2KeyBytes]byte {
	switch version {
	case Argon2VersionV1:
		return DeriveKeyV1(password, salt)
	case Argon2VersionV2:
		return DeriveKeyV2(password, salt)
	default:
		return DeriveKeyV2(password, salt)
	}
}

// HashPassword derives a password verifier using V2 parameters.
// IMPORTANT: The caller MUST use a DIFFERENT salt than the one used for DeriveKey.
// This ensures the stored hash cannot be used as the AES decryption key.
func HashPassword(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	return DeriveKeyV2(password, salt)
}
