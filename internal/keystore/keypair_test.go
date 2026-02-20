package keystore_test

import (
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
)

func TestDeriveKeypairBIP44Standard(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44() error: %v", err)
	}

	if len(kp.PublicKey) != 32 {
		t.Errorf("public key length = %d, want 32", len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != 64 {
		t.Errorf("private key length = %d, want 64", len(kp.PrivateKey))
	}

	addr := keystore.KeypairToAddress(kp)
	if addr == "" {
		t.Error("KeypairToAddress() returned empty string")
	}
	t.Logf("BIP44 Standard account 0 address: %s", addr)
}

func TestDeriveKeypairBIP44Deterministic(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp1, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44() error: %v", err)
	}

	kp2, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44() error: %v", err)
	}

	addr1 := keystore.KeypairToAddress(kp1)
	addr2 := keystore.KeypairToAddress(kp2)
	if addr1 != addr2 {
		t.Errorf("DeriveKeypairBIP44() not deterministic: %s != %s", addr1, addr2)
	}
}

func TestDeriveKeypairBIP44DifferentAccounts(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp0, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44(account 0) error: %v", err)
	}

	kp1, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 1)
	if err != nil {
		t.Fatalf("DeriveKeypairBIP44(account 1) error: %v", err)
	}

	addr0 := keystore.KeypairToAddress(kp0)
	addr1 := keystore.KeypairToAddress(kp1)
	if addr0 == addr1 {
		t.Error("different account indices should produce different addresses")
	}
}

func TestDeriveKeypairBIP44InvalidMnemonic(t *testing.T) {
	words := []string{"invalid", "words"}
	_, err := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Error("DeriveKeypairBIP44(invalid) should return error")
	}
}

func TestDeriveKeypairLegacy(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp, err := keystore.DeriveKeypairLegacy(words)
	if err != nil {
		t.Fatalf("DeriveKeypairLegacy() error: %v", err)
	}

	if len(kp.PublicKey) != 32 {
		t.Errorf("public key length = %d, want 32", len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != 64 {
		t.Errorf("private key length = %d, want 64", len(kp.PrivateKey))
	}

	addr := keystore.KeypairToAddress(kp)
	if addr == "" {
		t.Error("KeypairToAddress() returned empty string")
	}
	t.Logf("Legacy address: %s", addr)
}

func TestDeriveKeypairLegacyDeterministic(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp1, _ := keystore.DeriveKeypairLegacy(words)
	kp2, _ := keystore.DeriveKeypairLegacy(words)

	addr1 := keystore.KeypairToAddress(kp1)
	addr2 := keystore.KeypairToAddress(kp2)
	if addr1 != addr2 {
		t.Errorf("DeriveKeypairLegacy() not deterministic: %s != %s", addr1, addr2)
	}
}

func TestDeriveKeypairLegacyDiffersFromBIP44(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kpLegacy, _ := keystore.DeriveKeypairLegacy(words)
	kpBIP44, _ := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)

	addrLegacy := keystore.KeypairToAddress(kpLegacy)
	addrBIP44 := keystore.KeypairToAddress(kpBIP44)
	if addrLegacy == addrBIP44 {
		t.Error("legacy and BIP44 should produce different addresses")
	}
}

func TestKeypairToAddressBase58(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp, _ := keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	addr := keystore.KeypairToAddress(kp)

	// Solana addresses are Base58-encoded 32-byte public keys
	// Typical length is 32-44 characters
	if len(addr) < 32 || len(addr) > 44 {
		t.Errorf("address length = %d, expected 32-44 characters", len(addr))
	}
}

func TestZeroKeypair(t *testing.T) {
	words := strings.Split("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", " ")

	kp, _ := keystore.DeriveKeypairLegacy(words)
	keystore.ZeroKeypair(&kp)

	allZero := true
	for _, b := range kp.PrivateKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("ZeroKeypair() did not zero private key")
	}
}
