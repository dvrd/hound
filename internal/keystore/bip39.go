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
