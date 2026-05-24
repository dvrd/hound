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
