package keystore_test

import (
	"bytes"
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestAESConstants(t *testing.T) {
	if keystore.AESKeyBytes != 32 {
		t.Errorf("AESKeyBytes = %d, want 32", keystore.AESKeyBytes)
	}
	if keystore.GCMNonceBytes != 12 {
		t.Errorf("GCMNonceBytes = %d, want 12", keystore.GCMNonceBytes)
	}
	if keystore.GCMTagBytes != 16 {
		t.Errorf("GCMTagBytes = %d, want 16", keystore.GCMTagBytes)
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1, err := keystore.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error: %v", err)
	}

	nonce2, err := keystore.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error: %v", err)
	}

	if nonce1 == nonce2 {
		t.Error("GenerateNonce() returned identical nonces — should be unique")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("hello, solana wallet!")
	key := [32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
	}
	nonce := [12]byte{0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAB, 0xAC}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	decrypted, err := keystore.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptTagSeparation(t *testing.T) {
	plaintext := []byte("test data for tag check")
	key := [32]byte{0xFF}
	nonce := [12]byte{0xBB}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	// Tag must be exactly 16 bytes
	if len(encrypted.Tag) != 16 {
		t.Errorf("tag length = %d, want 16", len(encrypted.Tag))
	}

	// Ciphertext length should equal plaintext length (GCM is a stream cipher)
	if len(encrypted.Ciphertext) != len(plaintext) {
		t.Errorf("ciphertext length = %d, want %d (same as plaintext)", len(encrypted.Ciphertext), len(plaintext))
	}

	// Nonce should be preserved
	if encrypted.Nonce != nonce {
		t.Error("nonce was not preserved in EncryptedData")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	plaintext := []byte("secret data")
	key := [32]byte{0x01}
	nonce := [12]byte{0x02}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	wrongKey := [32]byte{0xFF}
	_, err = keystore.Decrypt(encrypted, wrongKey)
	if err == nil {
		t.Error("Decrypt() with wrong key should return error")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	plaintext := []byte("tamper test")
	key := [32]byte{0x42}
	nonce := [12]byte{0x43}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	// Tamper with ciphertext
	if len(encrypted.Ciphertext) > 0 {
		encrypted.Ciphertext[0] ^= 0xFF
	}

	_, err = keystore.Decrypt(encrypted, key)
	if err == nil {
		t.Error("Decrypt() with tampered ciphertext should return error")
	}
}

func TestDecryptTamperedTag(t *testing.T) {
	plaintext := []byte("tag tamper test")
	key := [32]byte{0x42}
	nonce := [12]byte{0x43}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	// Tamper with tag
	encrypted.Tag[0] ^= 0xFF

	_, err = keystore.Decrypt(encrypted, key)
	if err == nil {
		t.Error("Decrypt() with tampered tag should return error")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	plaintext := []byte{}
	key := [32]byte{0x01}
	nonce := [12]byte{0x02}

	encrypted, err := keystore.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	if len(encrypted.Ciphertext) != 0 {
		t.Errorf("ciphertext length = %d, want 0 for empty plaintext", len(encrypted.Ciphertext))
	}

	decrypted, err := keystore.Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("decrypted length = %d, want 0", len(decrypted))
	}
}
