package keystore

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"
)

const (
	Argon2MemoryKB    = 19456 // 19 MiB — matches Odin version
	Argon2Iterations  = 2
	Argon2Parallelism = 1
	Argon2SaltBytes   = 16
	Argon2KeyBytes    = 32
)

// GenerateSalt returns 16 cryptographically random bytes.
func GenerateSalt() ([Argon2SaltBytes]byte, error) {
	var salt [Argon2SaltBytes]byte
	_, err := rand.Read(salt[:])
	return salt, err
}

// DeriveKey derives a 32-byte key from password and salt using Argon2id.
func DeriveKey(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	raw := argon2.IDKey(
		[]byte(password),
		salt[:],
		Argon2Iterations,
		Argon2MemoryKB,
		Argon2Parallelism,
		Argon2KeyBytes,
	)
	var key [Argon2KeyBytes]byte
	copy(key[:], raw)
	return key
}

// HashPassword hashes a password with the given salt.
// Identical to DeriveKey — provided for semantic clarity.
func HashPassword(password string, salt [Argon2SaltBytes]byte) [Argon2KeyBytes]byte {
	return DeriveKey(password, salt)
}
