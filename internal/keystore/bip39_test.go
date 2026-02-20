package keystore_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestValidateMnemonic12Words(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")
	if err := keystore.ValidateMnemonic(words); err != nil {
		t.Errorf("ValidateMnemonic(12 words) error: %v", err)
	}
}

func TestValidateMnemonic24Words(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art", " ")
	if err := keystore.ValidateMnemonic(words); err != nil {
		t.Errorf("ValidateMnemonic(24 words) error: %v", err)
	}
}

func TestValidateMnemonicReject11Words(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon", " ")
	if err := keystore.ValidateMnemonic(words); err == nil {
		t.Error("ValidateMnemonic(11 words) should return error")
	}
}

func TestValidateMnemonicReject0Words(t *testing.T) {
	if err := keystore.ValidateMnemonic([]string{}); err == nil {
		t.Error("ValidateMnemonic(0 words) should return error")
	}
}

func TestValidateMnemonicRejectInvalidWords(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon zzzzz", " ")
	if err := keystore.ValidateMnemonic(words); err == nil {
		t.Error("ValidateMnemonic(invalid word) should return error")
	}
}

func TestMnemonicToSeedKnownVector(t *testing.T) {
	// BIP39 test vector: "abandon" x11 + "about"
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	expectedHex := "5eb00bbddcf069084889a8ab9155568165f5c453ccb85e70811aaed6f6da5fc19a5ac40b389cd370d086206dec8aa6c43daea6690f20ad3d8d48b2d2ce9e38e4"

	seed, err := keystore.MnemonicToSeed(words)
	if err != nil {
		t.Fatalf("MnemonicToSeed() error: %v", err)
	}

	gotHex := hex.EncodeToString(seed[:])
	if gotHex != expectedHex {
		t.Errorf("MnemonicToSeed() =\n  %s\nwant:\n  %s", gotHex, expectedHex)
	}
}

func TestMnemonicToSeedInvalidMnemonic(t *testing.T) {
	words := []string{"not", "a", "valid", "mnemonic"}
	_, err := keystore.MnemonicToSeed(words)
	if err == nil {
		t.Error("MnemonicToSeed(invalid) should return error")
	}
}

func TestMnemonicToSeedLength(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	seed, err := keystore.MnemonicToSeed(words)
	if err != nil {
		t.Fatalf("MnemonicToSeed() error: %v", err)
	}

	if len(seed) != 64 {
		t.Errorf("seed length = %d, want 64", len(seed))
	}
}
