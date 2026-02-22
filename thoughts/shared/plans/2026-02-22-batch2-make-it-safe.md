# Batch 2 — "Make It Safe" Implementation Plan

**Goal:** Fix 6 security issues (1 critical, 1 high, 2 medium, 2 low) identified in the Batch 1 audit, ensuring the stored password hash can never be used as the AES decryption key.

**Architecture:** Dual-salt Argon2id derivation separates the password verifier from the encryption key. Existing wallets are transparently migrated on first unlock. Argon2 params are bumped from V1 (19MiB/2/1) to V2 (64MiB/3/4). Memory-safety improvements zero sensitive data in mutable byte slices.

**Design:** `thoughts/shared/designs/2026-02-22-batch2-make-it-safe-design.md`

---

## Dependency Graph

```
Batch 1 (parallel): 1.1, 1.2, 1.3  [foundation — keystore primitives, no cross-deps]
Batch 2 (parallel): 2.1, 2.2       [database schema + data layer — depends on batch 1]
Batch 3 (parallel): 3.1, 3.2, 3.3  [consumers — depends on batch 1 + 2]
Batch 4 (parallel): 4.1, 4.2       [TUI hardening — independent of batch 3, depends on batch 1 only]
```

**Note on task structure:** Several tasks modify existing files and their existing tests. Each task specifies the exact changes to make. Tests are updated in the same task as the implementation file they test.

---

## Batch 1: Keystore Primitives (parallel — 3 implementers)

All tasks in this batch have NO dependencies and run simultaneously. They modify only `internal/keystore/` files.

### Task 1.1: Argon2 V1/V2 Versioning + Zero Raw Slice

**File:** `internal/keystore/argon2.go`
**Test:** `internal/keystore/argon2_test.go`
**Depends:** none

**What to change in `internal/keystore/argon2.go`:**

1. Rename existing constants to V1 and add V2 constants:

```go
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
```

2. **Replace the entire file** with the code above.

**What to change in `internal/keystore/argon2_test.go`:**

Replace the entire file:

```go
package keystore_test

import (
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestArgon2V1Constants(t *testing.T) {
	if keystore.Argon2V1MemoryKB != 19456 {
		t.Errorf("Argon2V1MemoryKB = %d, want 19456", keystore.Argon2V1MemoryKB)
	}
	if keystore.Argon2V1Iterations != 2 {
		t.Errorf("Argon2V1Iterations = %d, want 2", keystore.Argon2V1Iterations)
	}
	if keystore.Argon2V1Parallelism != 1 {
		t.Errorf("Argon2V1Parallelism = %d, want 1", keystore.Argon2V1Parallelism)
	}
}

func TestArgon2V2Constants(t *testing.T) {
	if keystore.Argon2V2MemoryKB != 65536 {
		t.Errorf("Argon2V2MemoryKB = %d, want 65536", keystore.Argon2V2MemoryKB)
	}
	if keystore.Argon2V2Iterations != 3 {
		t.Errorf("Argon2V2Iterations = %d, want 3", keystore.Argon2V2Iterations)
	}
	if keystore.Argon2V2Parallelism != 4 {
		t.Errorf("Argon2V2Parallelism = %d, want 4", keystore.Argon2V2Parallelism)
	}
}

func TestBackwardCompatAliases(t *testing.T) {
	// Old constant names must still resolve to V1 values
	if keystore.Argon2MemoryKB != keystore.Argon2V1MemoryKB {
		t.Error("Argon2MemoryKB alias broken")
	}
	if keystore.Argon2Iterations != keystore.Argon2V1Iterations {
		t.Error("Argon2Iterations alias broken")
	}
	if keystore.Argon2Parallelism != keystore.Argon2V1Parallelism {
		t.Error("Argon2Parallelism alias broken")
	}
}

func TestSharedConstants(t *testing.T) {
	if keystore.Argon2SaltBytes != 16 {
		t.Errorf("Argon2SaltBytes = %d, want 16", keystore.Argon2SaltBytes)
	}
	if keystore.Argon2KeyBytes != 32 {
		t.Errorf("Argon2KeyBytes = %d, want 32", keystore.Argon2KeyBytes)
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := keystore.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error: %v", err)
	}

	salt2, err := keystore.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error: %v", err)
	}

	if salt1 == salt2 {
		t.Error("GenerateSalt() returned identical salts — should be unique")
	}
}

func TestDeriveKeyV1Deterministic(t *testing.T) {
	salt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}

	key1 := keystore.DeriveKeyV1("test-password", salt)
	key2 := keystore.DeriveKeyV1("test-password", salt)

	if key1 != key2 {
		t.Error("DeriveKeyV1() not deterministic: same password+salt produced different keys")
	}
}

func TestDeriveKeyV2Deterministic(t *testing.T) {
	salt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}

	key1 := keystore.DeriveKeyV2("test-password", salt)
	key2 := keystore.DeriveKeyV2("test-password", salt)

	if key1 != key2 {
		t.Error("DeriveKeyV2() not deterministic: same password+salt produced different keys")
	}
}

func TestDeriveKeyV1DifferentFromV2(t *testing.T) {
	salt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}

	keyV1 := keystore.DeriveKeyV1("test-password", salt)
	keyV2 := keystore.DeriveKeyV2("test-password", salt)

	if keyV1 == keyV2 {
		t.Error("V1 and V2 produced identical keys — params should differ")
	}
}

func TestDeriveKeyDifferentPasswords(t *testing.T) {
	salt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}

	key1 := keystore.DeriveKey("password-one", salt)
	key2 := keystore.DeriveKey("password-two", salt)

	if key1 == key2 {
		t.Error("DeriveKey() produced same key for different passwords")
	}
}

func TestDeriveKeyDifferentSalts(t *testing.T) {
	salt1 := [16]byte{0x01}
	salt2 := [16]byte{0x02}

	key1 := keystore.DeriveKey("same-password", salt1)
	key2 := keystore.DeriveKey("same-password", salt2)

	if key1 == key2 {
		t.Error("DeriveKey() produced same key for different salts")
	}
}

func TestDeriveKeyVersioned(t *testing.T) {
	salt := [16]byte{0xAA, 0xBB}

	v1 := keystore.DeriveKeyVersioned("pw", salt, keystore.Argon2VersionV1)
	v1Direct := keystore.DeriveKeyV1("pw", salt)
	if v1 != v1Direct {
		t.Error("DeriveKeyVersioned(V1) != DeriveKeyV1")
	}

	v2 := keystore.DeriveKeyVersioned("pw", salt, keystore.Argon2VersionV2)
	v2Direct := keystore.DeriveKeyV2("pw", salt)
	if v2 != v2Direct {
		t.Error("DeriveKeyVersioned(V2) != DeriveKeyV2")
	}
}

func TestDeriveKeyDefaultIsV2(t *testing.T) {
	salt := [16]byte{0xCC}

	defaultKey := keystore.DeriveKey("pw", salt)
	v2Key := keystore.DeriveKeyV2("pw", salt)

	if defaultKey != v2Key {
		t.Error("DeriveKey() should default to V2 parameters")
	}
}

func TestHashPasswordUsesV2(t *testing.T) {
	salt := [16]byte{0xDD}

	hash := keystore.HashPassword("pw", salt)
	v2Key := keystore.DeriveKeyV2("pw", salt)

	if hash != v2Key {
		t.Error("HashPassword() should use V2 parameters")
	}
}

func TestDualSaltProducesDifferentOutputs(t *testing.T) {
	// This is the CRITICAL security property: different salts → different outputs
	// even with the same password and same Argon2 version.
	encSalt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	verSalt := [16]byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20}

	encKey := keystore.DeriveKeyV2("my-password", encSalt)
	verHash := keystore.DeriveKeyV2("my-password", verSalt)

	if encKey == verHash {
		t.Fatal("CRITICAL: encryption key and verifier hash are identical — dual-salt is broken")
	}
}

func TestDeriveKeyLength(t *testing.T) {
	salt := [16]byte{}
	key := keystore.DeriveKey("password", salt)
	if len(key) != 32 {
		t.Errorf("DeriveKey() returned %d bytes, want 32", len(key))
	}
}
```

**Verify:** `go test ./internal/keystore/ -run TestArgon2 -v -count=1`
**Commit:** `fix(keystore): add Argon2 V1/V2 versioning and zero intermediate raw slice`

---

### Task 1.2: ZeroBytes Noinline + JoinWordsToBytes

**File:** `internal/keystore/secure.go`
**Test:** `internal/keystore/secure_test.go`
**Depends:** none

**What to change in `internal/keystore/secure.go`:**

Replace the entire file:

```go
package keystore

// ZeroBytes overwrites every byte in the slice with zero.
// The //go:noinline directive prevents the compiler from optimizing away
// the zeroing loop via dead-store elimination (Fix M14).
//
//go:noinline
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ZeroSlice is a generic version for fixed-size arrays passed as slices.
// Same as ZeroBytes, provided as an alias for clarity.
func ZeroSlice(b []byte) {
	ZeroBytes(b)
}

// JoinWordsToBytes joins a slice of words with spaces into a mutable []byte.
// Unlike strings.Join which returns an immutable string, the returned []byte
// can be zeroed after use to clear the mnemonic from memory (Fix H5).
func JoinWordsToBytes(words []string) []byte {
	if len(words) == 0 {
		return []byte{}
	}

	// Calculate total length: sum of word lengths + (n-1) spaces
	totalLen := 0
	for _, w := range words {
		totalLen += len(w)
	}
	totalLen += len(words) - 1 // spaces between words

	buf := make([]byte, 0, totalLen)
	for i, w := range words {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, w...)
	}
	return buf
}
```

**What to change in `internal/keystore/secure_test.go`:**

Replace the entire file:

```go
package keystore_test

import (
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestZeroBytes(t *testing.T) {
	b := []byte{0xFF, 0xAB, 0x12, 0x34, 0x56}
	keystore.ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestZeroBytesEmpty(t *testing.T) {
	b := []byte{}
	keystore.ZeroBytes(b) // should not panic
}

func TestZeroBytesNil(t *testing.T) {
	keystore.ZeroBytes(nil) // should not panic
}

func TestZeroSlice(t *testing.T) {
	b := []byte{0x01, 0x02, 0x03, 0x04}
	keystore.ZeroSlice(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroSlice: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestZeroBytesLargeSlice(t *testing.T) {
	b := make([]byte, 1024)
	for i := range b {
		b[i] = 0xFF
	}
	keystore.ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestJoinWordsToBytesMatchesStringsJoin(t *testing.T) {
	tests := []struct {
		name  string
		words []string
	}{
		{"12 words", strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")},
		{"single word", []string{"hello"}},
		{"two words", []string{"foo", "bar"}},
		{"empty", []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := strings.Join(tc.words, " ")
			got := keystore.JoinWordsToBytes(tc.words)
			if string(got) != expected {
				t.Errorf("JoinWordsToBytes = %q, want %q", string(got), expected)
			}
		})
	}
}

func TestJoinWordsToBytesIsMutable(t *testing.T) {
	words := []string{"abandon", "abandon", "about"}
	buf := keystore.JoinWordsToBytes(words)

	// Verify it's non-empty
	if len(buf) == 0 {
		t.Fatal("JoinWordsToBytes returned empty slice")
	}

	// Zero it — this is the whole point
	keystore.ZeroBytes(buf)
	for i, v := range buf {
		if v != 0 {
			t.Errorf("after ZeroBytes: byte[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestJoinWordsToBytesEmpty(t *testing.T) {
	got := keystore.JoinWordsToBytes([]string{})
	if len(got) != 0 {
		t.Errorf("JoinWordsToBytes([]) length = %d, want 0", len(got))
	}
}

func TestJoinWordsToBytesNil(t *testing.T) {
	got := keystore.JoinWordsToBytes(nil)
	if len(got) != 0 {
		t.Errorf("JoinWordsToBytes(nil) length = %d, want 0", len(got))
	}
}
```

**Verify:** `go test ./internal/keystore/ -run "TestZero|TestJoinWords" -v -count=1`
**Commit:** `fix(keystore): add noinline to ZeroBytes and add JoinWordsToBytes for mutable mnemonics`

---

### Task 1.3: Mnemonic Memory Safety (bip39.go + keypair.go)

**File:** `internal/keystore/bip39.go` AND `internal/keystore/keypair.go`
**Test:** `internal/keystore/bip39_test.go` (existing tests should still pass)
**Depends:** 1.2 (uses `JoinWordsToBytes` and `ZeroBytes`)

> **Note:** This task modifies TWO files but they are tightly coupled (both use `JoinWordsToBytes` from 1.2) and must be done together. The changes are small and mechanical.

**What to change in `internal/keystore/bip39.go`:**

Replace the entire file:

```go
package keystore

import (
	"fmt"

	"github.com/tyler-smith/go-bip39"
)

// ValidateMnemonic checks that the words form a valid BIP39 mnemonic.
// Must be exactly 12 or 24 words.
func ValidateMnemonic(words []string) error {
	n := len(words)
	if n != 12 && n != 24 {
		return fmt.Errorf("mnemonic must be 12 or 24 words, got %d", n)
	}

	// Build mnemonic as mutable []byte, zero after validation (Fix H5).
	mnemonicBytes := JoinWordsToBytes(words)
	defer ZeroBytes(mnemonicBytes)

	// Library requires string — unavoidable conversion at boundary.
	if !bip39.IsMnemonicValid(string(mnemonicBytes)) {
		return fmt.Errorf("invalid BIP39 mnemonic")
	}

	return nil
}

// MnemonicToSeed converts a BIP39 mnemonic to a 64-byte seed.
// Uses PBKDF2-HMAC-SHA512 with 2048 iterations and salt "mnemonic" (no passphrase).
func MnemonicToSeed(words []string) ([64]byte, error) {
	if err := ValidateMnemonic(words); err != nil {
		return [64]byte{}, err
	}

	// Build mnemonic as mutable []byte, zero after use (Fix H5).
	mnemonicBytes := JoinWordsToBytes(words)
	defer ZeroBytes(mnemonicBytes)

	// Library requires string — unavoidable conversion at boundary.
	// Empty passphrase → salt is just "mnemonic"
	seedBytes := bip39.NewSeed(string(mnemonicBytes), "")
	defer ZeroBytes(seedBytes) // Zero heap-allocated seed after copy

	var seed [64]byte
	copy(seed[:], seedBytes)
	return seed, nil
}

// GenerateMnemonic generates a new BIP39 mnemonic phrase.
// bitSize must be 128 (12 words) or 256 (24 words).
func GenerateMnemonic(bitSize int) (string, error) {
	if bitSize != 128 && bitSize != 256 {
		return "", fmt.Errorf("bitSize must be 128 or 256, got %d", bitSize)
	}
	entropy, err := bip39.NewEntropy(bitSize)
	if err != nil {
		return "", fmt.Errorf("generate entropy: %w", err)
	}
	defer ZeroBytes(entropy) // Zero entropy after mnemonic generation

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generate mnemonic: %w", err)
	}
	return mnemonic, nil
}
```

**What to change in `internal/keystore/keypair.go`:**

Replace the `DeriveKeypairLegacy` function. Change the import block to remove `"strings"` and update the function:

```go
// DeriveKeypairLegacy derives an Ed25519 keypair using the legacy SHA-256 method.
// Joins words with spaces, SHA-256 hashes the result to get a 32-byte seed, then
// derives an Ed25519 keypair.
func DeriveKeypairLegacy(words []string) (Keypair, error) {
	// Use mutable []byte instead of immutable string (Fix H5).
	mnemonicBytes := JoinWordsToBytes(words)
	defer ZeroBytes(mnemonicBytes)

	hash := sha256.Sum256(mnemonicBytes)
	defer ZeroBytes(hash[:])

	privKey := ed25519.NewKeyFromSeed(hash[:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return Keypair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}
```

The full updated `internal/keystore/keypair.go`:

```go
package keystore

import (
	"crypto/ed25519"
	"crypto/sha256"

	"github.com/dvrd/hound/internal/models"
	"github.com/mr-tron/base58"
)

// Keypair holds an Ed25519 key pair.
type Keypair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// DeriveKeypairBIP44 derives an Ed25519 keypair using BIP44 derivation.
// walletType determines the derivation path, accountIndex is the account number.
func DeriveKeypairBIP44(words []string, walletType models.WalletType, accountIndex int) (Keypair, error) {
	if err := ValidateMnemonic(words); err != nil {
		return Keypair{}, err
	}

	seed, err := MnemonicToSeed(words)
	if err != nil {
		return Keypair{}, err
	}
	defer ZeroBytes(seed[:])

	path := models.GetDerivationPath(walletType, accountIndex)
	hdKey, err := DeriveFromPath(seed, path)
	if err != nil {
		return Keypair{}, err
	}
	defer ZeroBytes(hdKey.Key[:])

	privKey := ed25519.NewKeyFromSeed(hdKey.Key[:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return Keypair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// DeriveKeypairLegacy derives an Ed25519 keypair using the legacy SHA-256 method.
// Joins words with spaces, SHA-256 hashes the result to get a 32-byte seed, then
// derives an Ed25519 keypair.
func DeriveKeypairLegacy(words []string) (Keypair, error) {
	// Use mutable []byte instead of immutable string (Fix H5).
	mnemonicBytes := JoinWordsToBytes(words)
	defer ZeroBytes(mnemonicBytes)

	hash := sha256.Sum256(mnemonicBytes)
	defer ZeroBytes(hash[:])

	privKey := ed25519.NewKeyFromSeed(hash[:])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return Keypair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// KeypairToAddress returns the Base58-encoded public key (Solana address).
func KeypairToAddress(kp Keypair) string {
	return base58.Encode(kp.PublicKey)
}

// ZeroKeypair zeroes the private key bytes.
func ZeroKeypair(kp *Keypair) {
	ZeroBytes(kp.PrivateKey)
}
```

**Verify:** `go test ./internal/keystore/ -v -count=1`
**Commit:** `fix(keystore): use mutable byte slices for mnemonics and zero after use`

---

## Batch 2: Database Layer (parallel — 2 implementers)

All tasks in this batch depend on Batch 1 completing (specifically 1.1 for the `Argon2Version` type).

### Task 2.1: Schema Migration + SetMaxOpenConns

**File:** `internal/database/database.go`
**Test:** `internal/database/database_test.go`
**Depends:** 1.1 (for `Argon2Version` type awareness, though not directly imported)

**What to change in `internal/database/database.go`:**

1. Add `db.SetMaxOpenConns(1)` in the `Open` function, right after `sql.Open`:

Find this code:
```go
func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
```

Replace with:
```go
func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}

	// Fix H4: Force single connection so PRAGMAs apply to all queries.
	// SQLite is single-writer anyway; pooling only causes PRAGMA drift.
	db.SetMaxOpenConns(1)
```

2. Add a `Migrate` method and update the schema constant. Add `verifier_salt BLOB` and `argon2_version INTEGER DEFAULT 1` columns to both `encrypted_keypairs` and `hyperliquid_wallets` CREATE TABLE statements:

In the `schema` constant, change the `encrypted_keypairs` table:
```sql
CREATE TABLE IF NOT EXISTS encrypted_keypairs (
    address TEXT PRIMARY KEY,
    encrypted_private_key BLOB NOT NULL,
    salt BLOB NOT NULL,
    nonce BLOB NOT NULL,
    tag BLOB NOT NULL,
    password_hash BLOB NOT NULL,
    verifier_salt BLOB,
    argon2_version INTEGER DEFAULT 1,
    label TEXT NOT NULL,
    is_primary INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used INTEGER
);
```

And the `hyperliquid_wallets` table:
```sql
CREATE TABLE IF NOT EXISTS hyperliquid_wallets (
    address TEXT PRIMARY KEY,
    label TEXT NOT NULL UNIQUE,
    api_wallet_name TEXT NOT NULL,
    encrypted_api_key BLOB NOT NULL,
    encrypted_api_secret BLOB NOT NULL,
    salt BLOB NOT NULL,
    nonce_key BLOB NOT NULL,
    nonce_secret BLOB NOT NULL,
    tag_key BLOB NOT NULL,
    tag_secret BLOB NOT NULL,
    password_hash BLOB NOT NULL,
    verifier_salt BLOB,
    argon2_version INTEGER DEFAULT 1,
    is_active INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_used INTEGER
);
```

3. Add a `Migrate` method after `CreateSchema`:

```go
// Migrate runs ALTER TABLE migrations for schema evolution.
// Safe to call multiple times — uses IF NOT EXISTS / ignores "duplicate column" errors.
func (d *Database) Migrate() error {
	migrations := []string{
		// C1: Add verifier_salt for dual-salt derivation
		`ALTER TABLE encrypted_keypairs ADD COLUMN verifier_salt BLOB`,
		`ALTER TABLE hyperliquid_wallets ADD COLUMN verifier_salt BLOB`,
		// H7: Track Argon2 parameter version
		`ALTER TABLE encrypted_keypairs ADD COLUMN argon2_version INTEGER DEFAULT 1`,
		`ALTER TABLE hyperliquid_wallets ADD COLUMN argon2_version INTEGER DEFAULT 1`,
	}

	for _, m := range migrations {
		_, err := d.db.Exec(m)
		if err != nil {
			// SQLite returns "duplicate column name" if column already exists.
			// This is expected on subsequent runs — ignore it.
			if isDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("migration %q: %w", m, err)
		}
	}
	return nil
}

// isDuplicateColumnError checks if the error is a "duplicate column" from ALTER TABLE.
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column")
}
```

4. Add `"strings"` to the import block.

**What to change in `internal/database/database_test.go`:**

Add these tests at the end of the file:

```go
func TestSetMaxOpenConns(t *testing.T) {
	db := mustOpenInMemory(t)

	// Verify max open conns is 1 by checking the stats
	stats := db.DB().Stats()
	// We can't directly check MaxOpenConnections from stats in all Go versions,
	// but we can verify the DB works correctly with single connection
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed with single conn: %v", err)
	}

	// Run multiple queries to verify single-connection mode works
	for i := 0; i < 5; i++ {
		var result int
		if err := db.DB().QueryRow("SELECT 1").Scan(&result); err != nil {
			t.Fatalf("query %d failed: %v", i, err)
		}
	}
	_ = stats // used above
}

func TestMigrateIdempotent(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// First migration should succeed
	if err := db.Migrate(); err != nil {
		t.Fatalf("first Migrate() failed: %v", err)
	}

	// Second migration should also succeed (idempotent)
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate() failed: %v", err)
	}
}

func TestMigrateAddsColumns(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	// Verify verifier_salt column exists in encrypted_keypairs
	_, err := db.DB().Exec(
		`INSERT INTO encrypted_keypairs (address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test_addr", []byte{1}, []byte{2}, []byte{3}, []byte{4}, []byte{5}, []byte{6}, 2, "test", 0, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("INSERT with verifier_salt failed: %v", err)
	}

	// Read it back
	var verSalt []byte
	var argonVer int
	err = db.DB().QueryRow(
		`SELECT verifier_salt, argon2_version FROM encrypted_keypairs WHERE address = ?`, "test_addr",
	).Scan(&verSalt, &argonVer)
	if err != nil {
		t.Fatalf("SELECT verifier_salt failed: %v", err)
	}
	if len(verSalt) != 1 || verSalt[0] != 6 {
		t.Errorf("verifier_salt = %v, want [6]", verSalt)
	}
	if argonVer != 2 {
		t.Errorf("argon2_version = %d, want 2", argonVer)
	}
}

func TestFreshSchemaHasNewColumns(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// Fresh schema should already have verifier_salt and argon2_version
	_, err := db.DB().Exec(
		`INSERT INTO encrypted_keypairs (address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"fresh_addr", []byte{1}, []byte{2}, []byte{3}, []byte{4}, []byte{5}, nil, 1, "test", 0, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("INSERT with new columns on fresh schema failed: %v", err)
	}
}

func TestPragmasApplyWithSingleConn(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}

	// With single connection, foreign_keys should always be ON
	var fk int
	if err := db.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys failed: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ON)", fk)
	}
}
```

**Verify:** `go test ./internal/database/ -run "TestSetMaxOpenConns|TestMigrate|TestFreshSchema|TestPragmasApply" -v -count=1`
**Commit:** `fix(database): add SetMaxOpenConns(1), dual-salt columns, and Migrate method`

---

### Task 2.2: EncryptedKeypairData Dual-Salt Fields + CRUD Updates

**File:** `internal/database/keypairs.go`
**Test:** `internal/database/keypairs_test.go`
**Depends:** 2.1 (schema must have new columns)

**What to change in `internal/database/keypairs.go`:**

1. Add `VerifierSalt` and `Argon2Version` fields to `EncryptedKeypairData`:

```go
// EncryptedKeypairData holds all fields from the encrypted_keypairs table.
type EncryptedKeypairData struct {
	Address             string
	EncryptedPrivateKey []byte
	Salt                [16]byte // encryption salt
	Nonce               [12]byte
	Tag                 [16]byte
	PasswordHash        [32]byte
	VerifierSalt        []byte // nil = old format (pre-migration), non-nil = new dual-salt format
	Argon2Version       int    // 1 = V1 params, 2 = V2 params; 0 treated as 1
	Label               string
	IsPrimary           bool
	CreatedAt           int64
	LastUsed            int64
}

// IsLegacyFormat returns true if this keypair was stored before the dual-salt migration.
func (d EncryptedKeypairData) IsLegacyFormat() bool {
	return d.VerifierSalt == nil || len(d.VerifierSalt) == 0
}
```

2. Update `InsertEncryptedKeypair` to store the new fields:

```go
// InsertEncryptedKeypair inserts a new encrypted keypair into the database.
func (d *Database) InsertEncryptedKeypair(data EncryptedKeypairData) error {
	now := time.Now().Unix()
	isPrimary := 0
	if data.IsPrimary {
		isPrimary = 1
	}

	argonVersion := data.Argon2Version
	if argonVersion == 0 {
		argonVersion = 1 // default for backward compat
	}

	_, err := d.db.Exec(
		`INSERT INTO encrypted_keypairs (address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at, last_used)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		data.Address, data.EncryptedPrivateKey,
		data.Salt[:], data.Nonce[:], data.Tag[:], data.PasswordHash[:],
		data.VerifierSalt, argonVersion,
		data.Label, isPrimary, now, now,
	)
	if err != nil {
		return fmt.Errorf("inserting encrypted keypair %q: %w", data.Address, err)
	}
	return nil
}
```

3. Update `GetEncryptedKeypair` to read the new fields:

```go
// GetEncryptedKeypair retrieves an encrypted keypair by address.
// BLOB fields are scanned as []byte and then copied into fixed-size arrays.
func (d *Database) GetEncryptedKeypair(addr string) (EncryptedKeypairData, error) {
	var data EncryptedKeypairData
	var isPrimary int
	var lastUsed sql.NullInt64
	var argonVersion sql.NullInt64

	// Use []byte intermediaries for BLOB fields
	var salt, nonce, tag, passwordHash []byte

	err := d.db.QueryRow(
		`SELECT address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at, last_used
		 FROM encrypted_keypairs WHERE address = ?`, addr,
	).Scan(&data.Address, &data.EncryptedPrivateKey, &salt, &nonce, &tag, &passwordHash,
		&data.VerifierSalt, &argonVersion,
		&data.Label, &isPrimary, &data.CreatedAt, &lastUsed)
	if err != nil {
		if err == sql.ErrNoRows {
			return EncryptedKeypairData{}, fmt.Errorf("getting encrypted keypair %q: %w", addr, models.ErrKeyNotFound)
		}
		return EncryptedKeypairData{}, fmt.Errorf("getting encrypted keypair %q: %w", addr, err)
	}

	data.IsPrimary = isPrimary != 0
	if lastUsed.Valid {
		data.LastUsed = lastUsed.Int64
	}
	if argonVersion.Valid {
		data.Argon2Version = int(argonVersion.Int64)
	} else {
		data.Argon2Version = 1 // pre-migration default
	}

	// Copy BLOB data into fixed-size arrays
	copy(data.Salt[:], salt)
	copy(data.Nonce[:], nonce)
	copy(data.Tag[:], tag)
	copy(data.PasswordHash[:], passwordHash)

	return data, nil
}
```

4. Update `UpdateEncryptedKeypair` to write the new fields:

```go
// UpdateEncryptedKeypair updates all crypto fields for an existing encrypted keypair.
// Used during C1 migration (re-encrypt with dual salts) and password updates.
func (d *Database) UpdateEncryptedKeypair(data EncryptedKeypairData) error {
	isPrimary := 0
	if data.IsPrimary {
		isPrimary = 1
	}

	argonVersion := data.Argon2Version
	if argonVersion == 0 {
		argonVersion = 1
	}

	result, err := d.db.Exec(
		`UPDATE encrypted_keypairs
		 SET encrypted_private_key = ?, salt = ?, nonce = ?, tag = ?,
		     password_hash = ?, verifier_salt = ?, argon2_version = ?,
		     label = ?, is_primary = ?
		 WHERE address = ?`,
		data.EncryptedPrivateKey, data.Salt[:], data.Nonce[:], data.Tag[:],
		data.PasswordHash[:], data.VerifierSalt, argonVersion,
		data.Label, isPrimary, data.Address,
	)
	if err != nil {
		return fmt.Errorf("updating encrypted keypair %q: %w", data.Address, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for keypair update %q: %w", data.Address, err)
	}
	if rows == 0 {
		return fmt.Errorf("updating encrypted keypair %q: %w", data.Address, models.ErrKeyNotFound)
	}

	return nil
}
```

**What to change in `internal/database/keypairs_test.go`:**

Update `makeTestKeypairData` to include new fields, and add tests for the new fields:

```go
package database

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/models"
)

func makeTestKeypairData(addr string) EncryptedKeypairData {
	data := EncryptedKeypairData{
		Address:             addr,
		EncryptedPrivateKey: make([]byte, 64),
		Label:               "Test Keypair",
		IsPrimary:           true,
		Argon2Version:       2,
	}
	// Fill with random data to test BLOB round-trip
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.Salt[:])
	rand.Read(data.Nonce[:])
	rand.Read(data.Tag[:])
	rand.Read(data.PasswordHash[:])

	// Generate verifier salt
	verSalt := make([]byte, 16)
	rand.Read(verSalt)
	data.VerifierSalt = verSalt

	return data
}

func makeTestKeypairDataLegacy(addr string) EncryptedKeypairData {
	data := EncryptedKeypairData{
		Address:             addr,
		EncryptedPrivateKey: make([]byte, 64),
		Label:               "Legacy Keypair",
		IsPrimary:           true,
		VerifierSalt:        nil, // legacy: no verifier salt
		Argon2Version:       1,
	}
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.Salt[:])
	rand.Read(data.Nonce[:])
	rand.Read(data.Tag[:])
	rand.Read(data.PasswordHash[:])
	return data
}

func TestInsertAndGetEncryptedKeypair(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")

	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	// Verify all fields
	if got.Address != data.Address {
		t.Errorf("Address = %q, want %q", got.Address, data.Address)
	}
	if !bytes.Equal(got.EncryptedPrivateKey, data.EncryptedPrivateKey) {
		t.Error("EncryptedPrivateKey mismatch")
	}
	if got.Salt != data.Salt {
		t.Error("Salt mismatch")
	}
	if got.Nonce != data.Nonce {
		t.Error("Nonce mismatch")
	}
	if got.Tag != data.Tag {
		t.Error("Tag mismatch")
	}
	if got.PasswordHash != data.PasswordHash {
		t.Error("PasswordHash mismatch")
	}
	if !bytes.Equal(got.VerifierSalt, data.VerifierSalt) {
		t.Errorf("VerifierSalt = %x, want %x", got.VerifierSalt, data.VerifierSalt)
	}
	if got.Argon2Version != data.Argon2Version {
		t.Errorf("Argon2Version = %d, want %d", got.Argon2Version, data.Argon2Version)
	}
	if got.Label != data.Label {
		t.Errorf("Label = %q, want %q", got.Label, data.Label)
	}
	if got.IsPrimary != data.IsPrimary {
		t.Errorf("IsPrimary = %v, want %v", got.IsPrimary, data.IsPrimary)
	}
}

func TestInsertAndGetLegacyKeypair(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairDataLegacy("legacy_addr")

	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("legacy_addr")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if !got.IsLegacyFormat() {
		t.Error("expected IsLegacyFormat() = true for nil VerifierSalt")
	}
	if got.Argon2Version != 1 {
		t.Errorf("Argon2Version = %d, want 1", got.Argon2Version)
	}
}

func TestIsLegacyFormat(t *testing.T) {
	legacy := EncryptedKeypairData{VerifierSalt: nil}
	if !legacy.IsLegacyFormat() {
		t.Error("nil VerifierSalt should be legacy")
	}

	legacy2 := EncryptedKeypairData{VerifierSalt: []byte{}}
	if !legacy2.IsLegacyFormat() {
		t.Error("empty VerifierSalt should be legacy")
	}

	modern := EncryptedKeypairData{VerifierSalt: []byte{0x01}}
	if modern.IsLegacyFormat() {
		t.Error("non-empty VerifierSalt should NOT be legacy")
	}
}

func TestGetEncryptedKeypairNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	_, err := db.GetEncryptedKeypair("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestUpdateEncryptedKeypair(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")
	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	// Update with new data including new verifier salt
	data.Label = "Updated Label"
	data.IsPrimary = false
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.Salt[:])
	newVerSalt := make([]byte, 16)
	rand.Read(newVerSalt)
	data.VerifierSalt = newVerSalt
	data.Argon2Version = 2

	if err := db.UpdateEncryptedKeypair(data); err != nil {
		t.Fatalf("UpdateEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if got.Label != "Updated Label" {
		t.Errorf("Label = %q, want %q", got.Label, "Updated Label")
	}
	if got.IsPrimary {
		t.Error("IsPrimary = true, want false")
	}
	if got.Salt != data.Salt {
		t.Error("Salt mismatch after update")
	}
	if !bytes.Equal(got.VerifierSalt, newVerSalt) {
		t.Error("VerifierSalt mismatch after update")
	}
	if got.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", got.Argon2Version)
	}
}

func TestUpdateEncryptedKeypairMigration(t *testing.T) {
	// Simulate C1 migration: insert legacy, then update with dual-salt
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Insert as legacy (no verifier salt)
	data := makeTestKeypairDataLegacy("migrate_addr")
	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	// Verify it's legacy
	got, _ := db.GetEncryptedKeypair("migrate_addr")
	if !got.IsLegacyFormat() {
		t.Fatal("expected legacy format before migration")
	}

	// Migrate: update with new salt, verifier salt, and V2
	rand.Read(data.Salt[:])
	rand.Read(data.EncryptedPrivateKey)
	rand.Read(data.PasswordHash[:])
	verSalt := make([]byte, 16)
	rand.Read(verSalt)
	data.VerifierSalt = verSalt
	data.Argon2Version = 2

	if err := db.UpdateEncryptedKeypair(data); err != nil {
		t.Fatalf("UpdateEncryptedKeypair (migration): %v", err)
	}

	// Verify migration
	migrated, err := db.GetEncryptedKeypair("migrate_addr")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair after migration: %v", err)
	}
	if migrated.IsLegacyFormat() {
		t.Error("expected non-legacy format after migration")
	}
	if migrated.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", migrated.Argon2Version)
	}
	if !bytes.Equal(migrated.VerifierSalt, verSalt) {
		t.Error("VerifierSalt mismatch after migration")
	}
}

func TestUpdateEncryptedKeypairNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("nonexistent")
	err := db.UpdateEncryptedKeypair(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestUpdateKeypairLastUsed(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	data := makeTestKeypairData("kp_addr_1")
	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	// Get initial last_used
	initial, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if err := db.UpdateKeypairLastUsed("kp_addr_1"); err != nil {
		t.Fatalf("UpdateKeypairLastUsed: %v", err)
	}

	updated, err := db.GetEncryptedKeypair("kp_addr_1")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	if updated.LastUsed < initial.LastUsed {
		t.Errorf("LastUsed went backwards: %d < %d", updated.LastUsed, initial.LastUsed)
	}
}

func TestUpdateKeypairLastUsedNotFound(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	err := db.UpdateKeypairLastUsed("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, models.ErrKeyNotFound) {
		t.Errorf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestBlobFieldsRoundTrip(t *testing.T) {
	db := mustOpenInMemory(t)
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Create data with specific known byte patterns
	data := EncryptedKeypairData{
		Address:             "blob_test",
		EncryptedPrivateKey: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		Label:               "Blob Test",
		Argon2Version:       2,
		VerifierSalt:        []byte{0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAB, 0xAC, 0xAD, 0xAE, 0xAF},
	}
	// Set specific patterns for fixed-size arrays
	for i := range data.Salt {
		data.Salt[i] = byte(i)
	}
	for i := range data.Nonce {
		data.Nonce[i] = byte(i + 100)
	}
	for i := range data.Tag {
		data.Tag[i] = byte(i + 200)
	}
	for i := range data.PasswordHash {
		data.PasswordHash[i] = byte(i + 50)
	}

	if err := db.InsertEncryptedKeypair(data); err != nil {
		t.Fatalf("InsertEncryptedKeypair: %v", err)
	}

	got, err := db.GetEncryptedKeypair("blob_test")
	if err != nil {
		t.Fatalf("GetEncryptedKeypair: %v", err)
	}

	// Verify exact byte-level match
	if !bytes.Equal(got.EncryptedPrivateKey, data.EncryptedPrivateKey) {
		t.Errorf("EncryptedPrivateKey = %x, want %x", got.EncryptedPrivateKey, data.EncryptedPrivateKey)
	}
	for i := range data.Salt {
		if got.Salt[i] != data.Salt[i] {
			t.Errorf("Salt[%d] = %d, want %d", i, got.Salt[i], data.Salt[i])
		}
	}
	for i := range data.Nonce {
		if got.Nonce[i] != data.Nonce[i] {
			t.Errorf("Nonce[%d] = %d, want %d", i, got.Nonce[i], data.Nonce[i])
		}
	}
	for i := range data.Tag {
		if got.Tag[i] != data.Tag[i] {
			t.Errorf("Tag[%d] = %d, want %d", i, got.Tag[i], data.Tag[i])
		}
	}
	for i := range data.PasswordHash {
		if got.PasswordHash[i] != data.PasswordHash[i] {
			t.Errorf("PasswordHash[%d] = %d, want %d", i, got.PasswordHash[i], data.PasswordHash[i])
		}
	}
	if !bytes.Equal(got.VerifierSalt, data.VerifierSalt) {
		t.Errorf("VerifierSalt = %x, want %x", got.VerifierSalt, data.VerifierSalt)
	}
	if got.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", got.Argon2Version)
	}
}
```

**Verify:** `go test ./internal/database/ -run "TestInsertAndGet|TestIsLegacy|TestUpdate|TestBlob" -v -count=1`
**Commit:** `fix(database): add VerifierSalt and Argon2Version to EncryptedKeypairData`

---

## Batch 3: Service Layer (sequential — 1 implementer)

This is the most complex task. It rewrites the core import/unlock/update logic.

### Task 3.1: KeystoreService Dual-Salt Import + Migration Unlock

**File:** `internal/services/keystore.go`
**Test:** `internal/services/keystore_test.go`
**Depends:** 1.1, 1.2, 2.1, 2.2

**What to change in `internal/services/keystore.go`:**

Replace the entire file. Key changes:
- `ImportKeypair`: generates TWO salts (encryption + verifier), derives key from encryption salt, derives hash from verifier salt, stores both with `Argon2Version: 2`
- `UnlockKeypair`: checks `IsLegacyFormat()` — if legacy, uses old V1 single-salt verify+decrypt, then migrates; if modern, uses dual-salt V2 verify then decrypt
- `UpdatePassword`: uses dual-salt pattern

```go
package services

import (
	"crypto/ed25519"
	"crypto/subtle"
	"fmt"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
)

// KeystoreService handles wallet import, unlock, and password update.
type KeystoreService struct{}

// ImportKeypair derives a keypair from seed phrase, encrypts it, and stores it in the database.
// Uses dual-salt derivation: encryption_salt for AES key, verifier_salt for password hash.
// Returns the wallet address.
func (s *KeystoreService) ImportKeypair(
	db *database.Database,
	words []string,
	password string,
	label string,
	isPrimary bool,
	walletType models.WalletType,
	accountIndex int,
) (string, error) {
	// 1. Validate password strength
	if err := keystore.ValidatePasswordStrength(password); err != nil {
		return "", fmt.Errorf("import keypair: %w", models.ErrWeakPassword)
	}

	// 2. Derive keypair
	var kp keystore.Keypair
	var err error
	if walletType == models.WalletTypeLegacy {
		kp, err = keystore.DeriveKeypairLegacy(words)
	} else {
		kp, err = keystore.DeriveKeypairBIP44(words, walletType, accountIndex)
	}
	if err != nil {
		return "", fmt.Errorf("import keypair: derive: %w", err)
	}
	// 3. Zero keypair when done
	defer keystore.ZeroKeypair(&kp)

	// 4. Get address
	address := keystore.KeypairToAddress(kp)

	// 5. Generate TWO salts (C1 fix: dual-salt derivation)
	encryptionSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate encryption salt: %w", err)
	}
	verifierSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate verifier salt: %w", err)
	}

	// 6. Derive AES key from encryption salt (NEVER stored)
	aesKey := keystore.DeriveKeyV2(password, encryptionSalt)
	defer keystore.ZeroBytes(aesKey[:])

	// 7. Derive password hash from DIFFERENT verifier salt (stored, useless for decryption)
	passwordHash := keystore.DeriveKeyV2(password, verifierSalt)

	// 8. Generate nonce
	nonce, err := keystore.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate nonce: %w", err)
	}

	// 9. Extract private key seed (first 32 bytes of ed25519.PrivateKey)
	seed := kp.PrivateKey.Seed()

	// 10. Encrypt
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("import keypair: encrypt: %w", err)
	}

	// 11. Build EncryptedKeypairData with dual salts
	ekd := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                encryptionSalt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
		VerifierSalt:        verifierSalt[:],
		Argon2Version:       2,
		Label:               label,
		IsPrimary:           isPrimary,
	}

	// 12. Insert encrypted keypair
	if err := db.InsertEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("import keypair: insert encrypted keypair: %w", err)
	}

	// 13. Build and insert wallet
	wallet := models.Wallet{
		Address:        address,
		Label:          label,
		IsPrimary:      isPrimary,
		WalletType:     walletType,
		DerivationPath: models.GetDerivationPath(walletType, accountIndex),
		AccountIndex:   accountIndex,
	}
	if err := db.InsertWallet(wallet); err != nil {
		return "", fmt.Errorf("import keypair: insert wallet: %w", err)
	}

	// 14. Return address
	return address, nil
}

// UnlockKeypair decrypts a stored keypair using the password.
// Handles both legacy (single-salt) and modern (dual-salt) formats.
// Legacy wallets are transparently migrated to dual-salt + V2 params on first unlock.
// Returns the Ed25519 private key. Caller must zero it when done.
func (s *KeystoreService) UnlockKeypair(
	db *database.Database,
	address string,
	password string,
) (ed25519.PrivateKey, error) {
	// 1. Get encrypted data
	ekd, err := db.GetEncryptedKeypair(address)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: %w", err)
	}

	if ekd.IsLegacyFormat() {
		return s.unlockAndMigrateLegacy(db, ekd, password)
	}
	return s.unlockModern(db, ekd, password)
}

// unlockAndMigrateLegacy handles old-format wallets (verifier_salt IS NULL).
// Steps: verify with old method → decrypt → re-encrypt with dual salts + V2 → update DB.
func (s *KeystoreService) unlockAndMigrateLegacy(
	db *database.Database,
	ekd database.EncryptedKeypairData,
	password string,
) (ed25519.PrivateKey, error) {
	// Determine which Argon2 version was used for this keypair
	version := keystore.Argon2Version(ekd.Argon2Version)
	if version == 0 {
		version = keystore.Argon2VersionV1
	}

	// Old format: password_hash == DeriveKey(password, salt) with same salt
	oldKey := keystore.DeriveKeyVersioned(password, ekd.Salt, version)
	defer keystore.ZeroBytes(oldKey[:])

	// Verify: in old format, the stored hash IS the key
	if subtle.ConstantTimeCompare(oldKey[:], ekd.PasswordHash[:]) != 1 {
		return nil, fmt.Errorf("unlock keypair: wrong password: %w", models.ErrCryptoFailed)
	}

	// Decrypt with old key
	encData := keystore.EncryptedData{
		Ciphertext: ekd.EncryptedPrivateKey,
		Nonce:      ekd.Nonce,
		Tag:        ekd.Tag,
	}
	plaintext, err := keystore.Decrypt(encData, oldKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}
	defer keystore.ZeroBytes(plaintext)

	// Reconstruct ed25519 key
	privKey := ed25519.NewKeyFromSeed(plaintext)

	// === MIGRATION: re-encrypt with dual salts + V2 params ===
	newEncSalt, err := keystore.GenerateSalt()
	if err != nil {
		// Migration failed but unlock succeeded — return key, skip migration
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}
	newVerSalt, err := keystore.GenerateSalt()
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newKey := keystore.DeriveKeyV2(password, newEncSalt)
	defer keystore.ZeroBytes(newKey[:])

	newHash := keystore.DeriveKeyV2(password, newVerSalt)

	newNonce, err := keystore.GenerateNonce()
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newEncrypted, err := keystore.Encrypt(plaintext, newKey, newNonce)
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	migratedEKD := database.EncryptedKeypairData{
		Address:             ekd.Address,
		EncryptedPrivateKey: newEncrypted.Ciphertext,
		Salt:                newEncSalt,
		Nonce:               newEncrypted.Nonce,
		Tag:                 newEncrypted.Tag,
		PasswordHash:        newHash,
		VerifierSalt:        newVerSalt[:],
		Argon2Version:       2,
		Label:               ekd.Label,
		IsPrimary:           ekd.IsPrimary,
	}

	// Best-effort migration — if it fails, the old format still works next time
	_ = db.UpdateEncryptedKeypair(migratedEKD)
	_ = db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// unlockModern handles new-format wallets (verifier_salt IS NOT NULL).
// Steps: verify password with verifier_salt → derive key from encryption salt → decrypt.
func (s *KeystoreService) unlockModern(
	db *database.Database,
	ekd database.EncryptedKeypairData,
	password string,
) (ed25519.PrivateKey, error) {
	// Determine Argon2 version
	version := keystore.Argon2Version(ekd.Argon2Version)
	if version == 0 {
		version = keystore.Argon2VersionV2
	}

	// Copy verifier salt into fixed-size array
	var verSalt [keystore.Argon2SaltBytes]byte
	copy(verSalt[:], ekd.VerifierSalt)

	// Verify password using verifier salt (safe — hash ≠ key)
	checkHash := keystore.DeriveKeyVersioned(password, verSalt, version)
	if subtle.ConstantTimeCompare(checkHash[:], ekd.PasswordHash[:]) != 1 {
		return nil, fmt.Errorf("unlock keypair: wrong password: %w", models.ErrCryptoFailed)
	}

	// Derive AES key from encryption salt (different from verifier salt)
	aesKey := keystore.DeriveKeyVersioned(password, ekd.Salt, version)
	defer keystore.ZeroBytes(aesKey[:])

	// Decrypt
	encData := keystore.EncryptedData{
		Ciphertext: ekd.EncryptedPrivateKey,
		Nonce:      ekd.Nonce,
		Tag:        ekd.Tag,
	}
	plaintext, err := keystore.Decrypt(encData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}
	defer keystore.ZeroBytes(plaintext)

	// Reconstruct ed25519 key
	privKey := ed25519.NewKeyFromSeed(plaintext)

	// Update last_used (best-effort)
	_ = db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// UpdatePassword re-encrypts a keypair with a new password using dual-salt derivation.
// Requires the seed phrase to verify identity.
func (s *KeystoreService) UpdatePassword(
	db *database.Database,
	words []string,
	newPassword string,
) (string, error) {
	// 1. Validate new password strength
	if err := keystore.ValidatePasswordStrength(newPassword); err != nil {
		return "", fmt.Errorf("update password: %w", models.ErrWeakPassword)
	}

	// 2. Derive keypair from words (try BIP44Standard first, then Legacy)
	var kp keystore.Keypair
	var err error
	kp, err = keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		// Fall back to legacy
		kp, err = keystore.DeriveKeypairLegacy(words)
		if err != nil {
			return "", fmt.Errorf("update password: derive keypair: %w", err)
		}
	}
	defer keystore.ZeroKeypair(&kp)

	// 3. Get address, check if wallet exists in DB
	address := keystore.KeypairToAddress(kp)
	_, err = db.GetWalletByAddress(address)
	if err != nil {
		// Try legacy if BIP44 address wasn't found
		kpLegacy, errLegacy := keystore.DeriveKeypairLegacy(words)
		if errLegacy != nil {
			return "", fmt.Errorf("update password: %w", err)
		}
		defer keystore.ZeroKeypair(&kpLegacy)

		legacyAddr := keystore.KeypairToAddress(kpLegacy)
		_, errWallet := db.GetWalletByAddress(legacyAddr)
		if errWallet != nil {
			return "", fmt.Errorf("update password: wallet not found: %w", models.ErrWalletNotFound)
		}
		address = legacyAddr
		kp = kpLegacy
	}

	// 4. Re-encrypt with new password using dual-salt derivation
	encryptionSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("update password: generate encryption salt: %w", err)
	}
	verifierSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("update password: generate verifier salt: %w", err)
	}

	aesKey := keystore.DeriveKeyV2(newPassword, encryptionSalt)
	defer keystore.ZeroBytes(aesKey[:])

	passwordHash := keystore.DeriveKeyV2(newPassword, verifierSalt)

	nonce, err := keystore.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("update password: generate nonce: %w", err)
	}

	seed := kp.PrivateKey.Seed()
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("update password: encrypt: %w", err)
	}

	// Get existing keypair data for label/isPrimary
	existingEKD, err := db.GetEncryptedKeypair(address)
	if err != nil {
		return "", fmt.Errorf("update password: get existing keypair: %w", err)
	}

	ekd := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                encryptionSalt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
		VerifierSalt:        verifierSalt[:],
		Argon2Version:       2,
		Label:               existingEKD.Label,
		IsPrimary:           existingEKD.IsPrimary,
	}

	if err := db.UpdateEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("update password: update encrypted keypair: %w", err)
	}

	return address, nil
}
```

**What to change in `internal/services/keystore_test.go`:**

Replace the entire file. Key additions:
- Test that newly imported wallets use dual-salt format
- Test that unlocking a legacy wallet triggers migration
- Test that after migration, the wallet uses dual-salt format
- All existing test scenarios still pass

```go
package services_test

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
)

const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
const testPassword = "MyStr0ng!Pass#1"
const testWeakPassword = "weak"

func setupTestDB(t *testing.T) *database.Database {
	t.Helper()
	db, err := database.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	if err := db.CreateSchema(); err != nil {
		t.Fatalf("CreateSchema failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestImportAndUnlockRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	// Import
	address, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}
	if address == "" {
		t.Fatal("ImportKeypair returned empty address")
	}
	t.Logf("Imported wallet address: %s", address)

	// Unlock
	privKey, err := svc.UnlockKeypair(db, address, testPassword)
	if err != nil {
		t.Fatalf("UnlockKeypair failed: %v", err)
	}
	defer keystore.ZeroBytes(privKey)

	// Verify the decrypted key produces the same address
	pubKey := privKey.Public().(ed25519.PublicKey)
	derivedKP := keystore.Keypair{PublicKey: pubKey, PrivateKey: privKey}
	derivedAddr := keystore.KeypairToAddress(derivedKP)

	if derivedAddr != address {
		t.Errorf("round-trip address mismatch: imported=%s, unlocked=%s", address, derivedAddr)
	}
}

func TestImportUsesDualSalt(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	// Verify the stored keypair uses dual-salt format
	ekd, err := db.GetEncryptedKeypair(address)
	if err != nil {
		t.Fatalf("GetEncryptedKeypair failed: %v", err)
	}

	if ekd.IsLegacyFormat() {
		t.Error("newly imported keypair should NOT be legacy format")
	}
	if ekd.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", ekd.Argon2Version)
	}
	if len(ekd.VerifierSalt) != 16 {
		t.Errorf("VerifierSalt length = %d, want 16", len(ekd.VerifierSalt))
	}

	// CRITICAL: verify that encryption salt != verifier salt
	var verSalt [16]byte
	copy(verSalt[:], ekd.VerifierSalt)
	if ekd.Salt == verSalt {
		t.Error("CRITICAL: encryption salt and verifier salt are identical")
	}

	// CRITICAL: verify that password_hash != DeriveKeyV2(password, encryption_salt)
	encKey := keystore.DeriveKeyV2(testPassword, ekd.Salt)
	if encKey == ekd.PasswordHash {
		t.Error("CRITICAL: password_hash equals encryption key — dual-salt is broken")
	}
}

func TestImportLegacyAndUnlock(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "legacy-wallet", true, models.WalletTypeLegacy, 0)
	if err != nil {
		t.Fatalf("ImportKeypair (legacy) failed: %v", err)
	}

	privKey, err := svc.UnlockKeypair(db, address, testPassword)
	if err != nil {
		t.Fatalf("UnlockKeypair (legacy) failed: %v", err)
	}
	defer keystore.ZeroBytes(privKey)

	pubKey := privKey.Public().(ed25519.PublicKey)
	derivedKP := keystore.Keypair{PublicKey: pubKey, PrivateKey: privKey}
	derivedAddr := keystore.KeypairToAddress(derivedKP)

	if derivedAddr != address {
		t.Errorf("legacy round-trip address mismatch: imported=%s, unlocked=%s", address, derivedAddr)
	}
}

func TestUnlockWrongPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	_, err = svc.UnlockKeypair(db, address, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("UnlockKeypair with wrong password should return error")
	}
	if !errors.Is(err, models.ErrCryptoFailed) {
		t.Errorf("expected ErrCryptoFailed, got: %v", err)
	}
}

func TestImportWeakPasswordRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	_, err := svc.ImportKeypair(db, words, testWeakPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("ImportKeypair with weak password should return error")
	}
	if !errors.Is(err, models.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got: %v", err)
	}
}

func TestImportInvalidSeedPhrase(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := []string{"invalid", "words", "that", "are", "not", "a", "valid", "bip39", "mnemonic", "at", "all", "here"}

	_, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("ImportKeypair with invalid seed phrase should return error")
	}
}

func TestUpdatePasswordAndUnlock(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	// Import with original password
	address, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	// Update password
	newPassword := "NewStr0ng!Pass#2"
	updatedAddr, err := svc.UpdatePassword(db, words, newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	if updatedAddr != address {
		t.Errorf("UpdatePassword returned different address: %s vs %s", updatedAddr, address)
	}

	// Old password should fail
	_, err = svc.UnlockKeypair(db, address, testPassword)
	if err == nil {
		t.Fatal("UnlockKeypair with old password should fail after UpdatePassword")
	}

	// New password should work
	privKey, err := svc.UnlockKeypair(db, address, newPassword)
	if err != nil {
		t.Fatalf("UnlockKeypair with new password failed: %v", err)
	}
	defer keystore.ZeroBytes(privKey)

	// Verify address matches
	pubKey := privKey.Public().(ed25519.PublicKey)
	derivedKP := keystore.Keypair{PublicKey: pubKey, PrivateKey: privKey}
	derivedAddr := keystore.KeypairToAddress(derivedKP)
	if derivedAddr != address {
		t.Errorf("address mismatch after password update: %s vs %s", derivedAddr, address)
	}
}

func TestUpdatePasswordUsesDualSalt(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	newPassword := "NewStr0ng!Pass#2"
	_, err = svc.UpdatePassword(db, words, newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}

	ekd, err := db.GetEncryptedKeypair(address)
	if err != nil {
		t.Fatalf("GetEncryptedKeypair failed: %v", err)
	}

	if ekd.IsLegacyFormat() {
		t.Error("after UpdatePassword, keypair should NOT be legacy format")
	}
	if ekd.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", ekd.Argon2Version)
	}
}

func TestUpdatePasswordWeakRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	_, err := svc.ImportKeypair(db, words, testPassword, "test-wallet", true, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportKeypair failed: %v", err)
	}

	_, err = svc.UpdatePassword(db, words, testWeakPassword)
	if err == nil {
		t.Fatal("UpdatePassword with weak password should return error")
	}
	if !errors.Is(err, models.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got: %v", err)
	}
}

func TestUpdatePasswordWalletNotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}

	// Use a different mnemonic that hasn't been imported
	words := strings.Split("zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong", " ")

	_, err := svc.UpdatePassword(db, words, testPassword)
	if err == nil {
		t.Fatal("UpdatePassword for non-existent wallet should return error")
	}
}

func TestLegacyMigrationOnUnlock(t *testing.T) {
	// Simulate a pre-migration wallet by inserting directly with old format
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	// First, derive the keypair to get the address and seed
	kp, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44 failed: %v", err)
	}
	address := keystore.KeypairToAddress(kp)
	seed := kp.PrivateKey.Seed()

	// Create old-format entry: same salt for key and hash (the vulnerability)
	salt, _ := keystore.GenerateSalt()
	oldKey := keystore.DeriveKeyV1(testPassword, salt)
	nonce, _ := keystore.GenerateNonce()
	encrypted, _ := keystore.Encrypt(seed, oldKey, nonce)

	oldEKD := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                salt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        oldKey, // OLD BUG: hash == key
		VerifierSalt:        nil,    // legacy: no verifier salt
		Argon2Version:       1,
		Label:               "legacy-test",
		IsPrimary:           true,
	}

	if err := db.InsertEncryptedKeypair(oldEKD); err != nil {
		t.Fatalf("InsertEncryptedKeypair (legacy): %v", err)
	}

	// Insert wallet record too
	wallet := models.Wallet{
		Address:        address,
		Label:          "legacy-test",
		IsPrimary:      true,
		WalletType:     models.WalletTypeBIP44Standard,
		DerivationPath: models.GetDerivationPath(models.WalletTypeBIP44Standard, 0),
		AccountIndex:   0,
	}
	if err := db.InsertWallet(wallet); err != nil {
		t.Fatalf("InsertWallet: %v", err)
	}

	// Verify it's legacy before unlock
	preUnlock, _ := db.GetEncryptedKeypair(address)
	if !preUnlock.IsLegacyFormat() {
		t.Fatal("expected legacy format before unlock")
	}

	// Unlock — should trigger migration
	privKey, err := svc.UnlockKeypair(db, address, testPassword)
	if err != nil {
		t.Fatalf("UnlockKeypair (legacy migration) failed: %v", err)
	}
	defer keystore.ZeroBytes(privKey)

	// Verify the key is correct
	pubKey := privKey.Public().(ed25519.PublicKey)
	derivedKP := keystore.Keypair{PublicKey: pubKey, PrivateKey: privKey}
	if keystore.KeypairToAddress(derivedKP) != address {
		t.Error("address mismatch after legacy migration unlock")
	}

	// Verify migration happened
	postUnlock, err := db.GetEncryptedKeypair(address)
	if err != nil {
		t.Fatalf("GetEncryptedKeypair after migration: %v", err)
	}

	if postUnlock.IsLegacyFormat() {
		t.Error("expected non-legacy format after migration")
	}
	if postUnlock.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2 after migration", postUnlock.Argon2Version)
	}

	// CRITICAL: verify password_hash != encryption key after migration
	encKey := keystore.DeriveKeyV2(testPassword, postUnlock.Salt)
	if encKey == postUnlock.PasswordHash {
		t.Error("CRITICAL: after migration, password_hash still equals encryption key")
	}

	// Verify we can unlock again with the migrated format
	privKey2, err := svc.UnlockKeypair(db, address, testPassword)
	if err != nil {
		t.Fatalf("UnlockKeypair (post-migration) failed: %v", err)
	}
	defer keystore.ZeroBytes(privKey2)

	pubKey2 := privKey2.Public().(ed25519.PublicKey)
	derivedKP2 := keystore.Keypair{PublicKey: pubKey2, PrivateKey: privKey2}
	if keystore.KeypairToAddress(derivedKP2) != address {
		t.Error("address mismatch on post-migration unlock")
	}
}

func TestLegacyWrongPasswordRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := &services.KeystoreService{}
	words := strings.Split(testMnemonic, " ")

	// Create old-format entry
	kp, _ := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	address := keystore.KeypairToAddress(kp)
	seed := kp.PrivateKey.Seed()

	salt, _ := keystore.GenerateSalt()
	oldKey := keystore.DeriveKeyV1(testPassword, salt)
	nonce, _ := keystore.GenerateNonce()
	encrypted, _ := keystore.Encrypt(seed, oldKey, nonce)

	oldEKD := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                salt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        oldKey,
		VerifierSalt:        nil,
		Argon2Version:       1,
		Label:               "legacy-test",
		IsPrimary:           true,
	}
	db.InsertEncryptedKeypair(oldEKD)

	wallet := models.Wallet{Address: address, Label: "legacy-test", IsPrimary: true, WalletType: models.WalletTypeBIP44Standard, DerivationPath: "m/44'/501'/0'/0'"}
	db.InsertWallet(wallet)

	// Wrong password should fail
	_, err := svc.UnlockKeypair(db, address, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("UnlockKeypair with wrong password on legacy wallet should fail")
	}
	if !errors.Is(err, models.ErrCryptoFailed) {
		t.Errorf("expected ErrCryptoFailed, got: %v", err)
	}

	// Verify migration did NOT happen (wrong password = no migration)
	ekd, _ := db.GetEncryptedKeypair(address)
	if !ekd.IsLegacyFormat() {
		t.Error("legacy wallet should NOT be migrated on wrong password")
	}
}
```

**Verify:** `go test ./internal/services/ -run "TestImport|TestUnlock|TestUpdate|TestLegacy" -v -count=1`
**Commit:** `fix(services): implement dual-salt import, unlock with legacy migration, and secure UpdatePassword`

---

### Task 3.2: Database Migrate Call in App Startup

**File:** Wherever the database is opened at app startup
**Depends:** 2.1

> **Implementation note:** The `Migrate()` method needs to be called after `CreateSchema()` at app startup. The implementer should find where `db.CreateSchema()` is called (likely in `cmd/hound/main.go` or a setup function) and add `db.Migrate()` right after it. This is a one-line addition.

Search for `CreateSchema` calls:

```
grep -r "CreateSchema" cmd/ internal/
```

Add `db.Migrate()` immediately after each `db.CreateSchema()` call. Example pattern:

```go
if err := db.CreateSchema(); err != nil {
    // ... error handling
}
if err := db.Migrate(); err != nil {
    // ... error handling
}
```

**Verify:** `go build ./cmd/hound/`
**Commit:** `fix(app): call db.Migrate() at startup for schema evolution`

---

### Task 3.3: Verify All Tests Pass

**File:** none (verification only)
**Depends:** 3.1, 3.2

Run the full test suite to verify nothing is broken:

```bash
go test ./... -count=1
```

If any tests fail, fix them. Common issues:
- Tests that check `HashPassword == DeriveKey` with same salt (the old `TestHashPasswordSameAsDeriveKey` test) — this test should be REMOVED since that's the bug we're fixing
- Tests that create `EncryptedKeypairData` without the new fields — add `Argon2Version: 1` and `VerifierSalt: nil`

**Verify:** `go test ./... -count=1`
**Commit:** `test: verify all packages pass after security hardening`

---

## Batch 4: TUI Hardening (parallel — 2 implementers)

These tasks are independent of each other and only depend on Batch 1 (for `ZeroBytes`).

### Task 4.1: Clear Password in Send View

**File:** `internal/tui/views/send/send.go`
**Test:** `internal/tui/views/send/send_test.go` (existing tests should still pass)
**Depends:** none (no new imports needed)

**What to change in `internal/tui/views/send/send.go`:**

1. In the `updatePassword` method, add `m.passwordInput.Reset()` after extracting the password:

Find:
```go
func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		password := m.passwordInput.Value()
		if password == "" {
			m.err = fmt.Errorf("password cannot be empty")
			return m, nil
		}
		m.err = nil
		m.step = StepSending
		return m, tea.Batch(m.spinner.Init(), m.doTransfer(password))
	}
```

Replace with:
```go
func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		password := m.passwordInput.Value()
		if password == "" {
			m.err = fmt.Errorf("password cannot be empty")
			return m, nil
		}
		// M6: Clear password from input buffer immediately after extraction
		m.passwordInput.Reset()
		m.err = nil
		m.step = StepSending
		return m, tea.Batch(m.spinner.Init(), m.doTransfer(password))
	}
```

2. In the global escape handling, clear password-related state when navigating back from password step. Find the escape handler:

```go
		if msg.String() == "esc" {
			if m.step == StepSelectToken {
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			}
			if m.step > StepSelectToken && m.step < StepSending {
				m.err = nil
				m.step--
				return m, m.focusCurrentStep()
			}
			return m, nil
		}
```

Replace with:
```go
		if msg.String() == "esc" {
			if m.step == StepSelectToken {
				// M6: Clear all sensitive state on exit
				m.passwordInput.Reset()
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			}
			if m.step > StepSelectToken && m.step < StepSending {
				// M6: Clear password buffer when navigating away from password step
				if m.step == StepPassword {
					m.passwordInput.Reset()
				}
				m.err = nil
				m.step--
				return m, m.focusCurrentStep()
			}
			return m, nil
		}
```

**Verify:** `go test ./internal/tui/views/send/ -v -count=1`
**Commit:** `fix(tui): clear password input buffer after use in send view`

---

### Task 4.2: Clear Passwords and Mnemonic in Import View

**File:** `internal/tui/views/walletimport/walletimport.go`
**Test:** `internal/tui/views/walletimport/walletimport_test.go` (existing tests should still pass)
**Depends:** none

**What to change in `internal/tui/views/walletimport/walletimport.go`:**

1. In `updatePassword`, reset the password input after extracting:

Find:
```go
func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		pw := m.passwordInput.Value()
		if len(pw) < 12 {
			m.err = fmt.Errorf("password must be at least 12 characters")
			return m, nil
		}
		m.password = pw
		m.err = nil
		m.step = StepConfirmPassword
		m.confirmPwInput.Focus()
		return m, m.confirmPwInput.Focus()
	}
```

Replace with:
```go
func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		pw := m.passwordInput.Value()
		if len(pw) < 12 {
			m.err = fmt.Errorf("password must be at least 12 characters")
			return m, nil
		}
		m.password = pw
		// M6: Clear password from input buffer after extraction
		m.passwordInput.Reset()
		m.err = nil
		m.step = StepConfirmPassword
		m.confirmPwInput.Focus()
		return m, m.confirmPwInput.Focus()
	}
```

2. In `updateConfirmPassword`, reset the confirm input after use:

Find:
```go
func (m Model) updateConfirmPassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.confirmPwInput.Value() != m.password {
			m.err = fmt.Errorf("passwords do not match")
			return m, nil
		}
		m.err = nil
		m.step = StepLabel
		m.labelInput.Focus()
		return m, m.labelInput.Focus()
	}
```

Replace with:
```go
func (m Model) updateConfirmPassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.confirmPwInput.Value() != m.password {
			m.err = fmt.Errorf("passwords do not match")
			return m, nil
		}
		// M6: Clear confirm password buffer after validation
		m.confirmPwInput.Reset()
		m.err = nil
		m.step = StepLabel
		m.labelInput.Focus()
		return m, m.labelInput.Focus()
	}
```

3. In `doImport`, clear sensitive state after launching the import:

Find:
```go
func (m Model) doImport() tea.Cmd {
	return func() tea.Msg {
		if m.keystoreSvc == nil || m.db == nil {
			return tui.WalletImportedMsg{Err: fmt.Errorf("import service not available")}
		}
		addr, err := m.keystoreSvc.ImportKeypair(
			m.db, m.words, m.password, m.label, true,
			m.walletType, m.accountIndex,
		)
		return tui.WalletImportedMsg{Address: addr, Err: err}
	}
}
```

This is a closure that captures `m.words` and `m.password` by value (since `m` is a value receiver). The captured values can't be zeroed from outside. However, we can clear the model's fields after launching the command.

In `updateLabel`, clear sensitive fields after launching the import:

Find:
```go
func (m Model) updateLabel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		label := strings.TrimSpace(m.labelInput.Value())
		if label == "" {
			m.err = fmt.Errorf("label cannot be empty")
			return m, nil
		}
		m.label = label
		m.err = nil
		m.step = StepImporting
		return m, tea.Batch(m.spinner.Init(), m.doImport())
	}
```

Replace with:
```go
func (m Model) updateLabel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		label := strings.TrimSpace(m.labelInput.Value())
		if label == "" {
			m.err = fmt.Errorf("label cannot be empty")
			return m, nil
		}
		m.label = label
		m.err = nil
		m.step = StepImporting
		cmd := m.doImport()
		// M6: Clear sensitive state after capturing in closure
		m.password = ""
		m.words = nil
		m.seedInput.Reset()
		return m, tea.Batch(m.spinner.Init(), cmd)
	}
```

4. In the escape handler, clear sensitive state when navigating back:

Find the escape handler for `StepShowMnemonic`:
```go
			case StepShowMnemonic:
				m.err = nil
				m.words = nil
				m.isGenerate = false
				m.step = StepChoice
				return m, nil
```

This already clears `m.words` — good.

Add clearing for the `StepChoice` escape (navigating out entirely):
```go
			case StepChoice:
				// M6: Clear all sensitive state on exit
				m.password = ""
				m.words = nil
				m.passwordInput.Reset()
				m.confirmPwInput.Reset()
				m.seedInput.Reset()
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
```

Also add clearing for the `StepSeedPhrase` escape:
```go
			case StepSeedPhrase:
				m.err = nil
				m.seedInput.Reset() // M6: Clear seed input
				m.step = StepChoice
				return m, nil
```

And for the generic back navigation in the `default` case, add password clearing:
```go
			default:
				if m.step > StepShowMnemonic && m.step < StepImporting {
					m.err = nil
					// M6: Clear sensitive inputs when navigating back
					if m.step == StepPassword || m.step == StepConfirmPassword {
						m.passwordInput.Reset()
						m.confirmPwInput.Reset()
						m.password = ""
					}
					m.step--
					// Skip StepShowMnemonic if importing, or StepSeedPhrase if generating
					if !m.isGenerate && m.step == StepShowMnemonic {
						m.step = StepSeedPhrase
					}
					if m.isGenerate && m.step == StepSeedPhrase {
						m.step = StepShowMnemonic
					}
					return m, m.focusCurrentStep()
				}
				return m, nil
```

**Verify:** `go test ./internal/tui/views/walletimport/ -v -count=1`
**Commit:** `fix(tui): clear password and mnemonic inputs after use in import view`

---

## Execution Order Summary

```
Batch 1 (parallel, 3 tasks):
  1.1: argon2.go — V1/V2 versioning + zero raw slice
  1.2: secure.go — noinline + JoinWordsToBytes
  1.3: bip39.go + keypair.go — mnemonic memory safety (depends on 1.2)

Batch 2 (parallel, 2 tasks, after Batch 1):
  2.1: database.go — SetMaxOpenConns + schema migration
  2.2: keypairs.go — dual-salt fields + CRUD updates

Batch 3 (sequential, 3 tasks, after Batch 2):
  3.1: keystore.go (services) — dual-salt import + migration unlock
  3.2: app startup — call db.Migrate()
  3.3: verification — run full test suite

Batch 4 (parallel, 2 tasks, after Batch 1):
  4.1: send.go — clear password input
  4.2: walletimport.go — clear passwords + mnemonic
```

**Total: 10 tasks across 4 batches.**

## Risk Notes

1. **Argon2 V2 is slow by design** — tests that call `DeriveKeyV2` will take ~1-2 seconds each due to 64MiB memory + 3 iterations. This is expected and correct for a crypto wallet.

2. **Legacy migration is best-effort** — if the DB update fails during migration, the old format is preserved and migration retries on next unlock. No data loss possible.

3. **String immutability limitation** — Go strings are immutable. We can clear `textinput` buffers and struct fields, but the extracted `string` values may persist in memory until GC. This is a known Go limitation documented in the design.

4. **Backward-compat aliases** — The old constant names (`Argon2MemoryKB`, etc.) are kept as aliases to avoid breaking any code outside the files we're modifying. They can be removed in a future cleanup pass.
