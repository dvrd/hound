package keystore_test

import (
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestArgon2Constants(t *testing.T) {
	if keystore.Argon2MemoryKB != 19456 {
		t.Errorf("Argon2MemoryKB = %d, want 19456", keystore.Argon2MemoryKB)
	}
	if keystore.Argon2Iterations != 2 {
		t.Errorf("Argon2Iterations = %d, want 2", keystore.Argon2Iterations)
	}
	if keystore.Argon2Parallelism != 1 {
		t.Errorf("Argon2Parallelism = %d, want 1", keystore.Argon2Parallelism)
	}
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

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}

	key1 := keystore.DeriveKey("test-password", salt)
	key2 := keystore.DeriveKey("test-password", salt)

	if key1 != key2 {
		t.Error("DeriveKey() not deterministic: same password+salt produced different keys")
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

func TestHashPasswordSameAsDeriveKey(t *testing.T) {
	salt := [16]byte{0xAA, 0xBB, 0xCC, 0xDD}

	key := keystore.DeriveKey("my-password", salt)
	hash := keystore.HashPassword("my-password", salt)

	if key != hash {
		t.Error("HashPassword() and DeriveKey() should produce identical output")
	}
}

func TestDeriveKeyLength(t *testing.T) {
	salt := [16]byte{}
	key := keystore.DeriveKey("password", salt)
	if len(key) != 32 {
		t.Errorf("DeriveKey() returned %d bytes, want 32", len(key))
	}
}
