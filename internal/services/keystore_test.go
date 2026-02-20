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
