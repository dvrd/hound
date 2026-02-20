package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

const (
	AESKeyBytes   = 32
	GCMNonceBytes = 12
	GCMTagBytes   = 16
)

// EncryptedData holds the encrypted ciphertext with nonce and tag stored separately.
// This matches the Odin database storage format where ciphertext and tag are
// stored in separate columns.
type EncryptedData struct {
	Ciphertext []byte   // Just the ciphertext, WITHOUT the tag
	Nonce      [12]byte // GCM nonce
	Tag        [16]byte // GCM authentication tag
}

// GenerateNonce returns 12 cryptographically random bytes.
func GenerateNonce() ([GCMNonceBytes]byte, error) {
	var nonce [GCMNonceBytes]byte
	_, err := rand.Read(nonce[:])
	return nonce, err
}

// Encrypt encrypts plaintext with AES-256-GCM.
// The key must be 32 bytes. The nonce must be 12 bytes.
// Returns EncryptedData with ciphertext and tag stored separately.
func Encrypt(plaintext []byte, key [AESKeyBytes]byte, nonce [GCMNonceBytes]byte) (EncryptedData, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return EncryptedData{}, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedData{}, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	// Seal returns ciphertext || tag (tag is appended)
	sealed := gcm.Seal(nil, nonce[:], plaintext, nil)

	// Split: ciphertext is everything except the last GCMTagBytes
	ctLen := len(sealed) - GCMTagBytes
	ct := make([]byte, ctLen)
	copy(ct, sealed[:ctLen])

	var tag [GCMTagBytes]byte
	copy(tag[:], sealed[ctLen:])

	return EncryptedData{
		Ciphertext: ct,
		Nonce:      nonce,
		Tag:        tag,
	}, nil
}

// Decrypt decrypts EncryptedData with AES-256-GCM.
// Internally rejoins ciphertext+tag before calling cipher.AEAD.Open().
func Decrypt(data EncryptedData, key [AESKeyBytes]byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	// Rejoin ciphertext + tag for Go's AEAD.Open
	combined := make([]byte, len(data.Ciphertext)+GCMTagBytes)
	copy(combined, data.Ciphertext)
	copy(combined[len(data.Ciphertext):], data.Tag[:])

	plaintext, err := gcm.Open(nil, data.Nonce[:], combined, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm.Open: %w", err)
	}

	return plaintext, nil
}
