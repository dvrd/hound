package keystore

import (
	"fmt"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

// ValidateMnemonic checks that the words form a valid BIP39 mnemonic.
// Must be exactly 12 or 24 words.
func ValidateMnemonic(words []string) error {
	n := len(words)
	if n != 12 && n != 24 {
		return fmt.Errorf("mnemonic must be 12 or 24 words, got %d", n)
	}

	mnemonic := strings.Join(words, " ")
	if !bip39.IsMnemonicValid(mnemonic) {
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

	mnemonic := strings.Join(words, " ")
	// Empty passphrase → salt is just "mnemonic"
	seedBytes := bip39.NewSeed(mnemonic, "")

	var seed [64]byte
	copy(seed[:], seedBytes)
	return seed, nil
}
