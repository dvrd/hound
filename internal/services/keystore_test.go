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
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	// Import
	address, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
	}
	if address == "" {
		t.Fatal("ImportWallet returned empty address")
	}
	t.Logf("Imported wallet address: %s", address)

	// Unlock
	privKey, err := svc.UnlockWallet(address, testPassword)
	if err != nil {
		t.Fatalf("UnlockWallet failed: %v", err)
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
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
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
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportWallet(words, testPassword, "legacy-wallet", models.WalletTypeLegacy, 0)
	if err != nil {
		t.Fatalf("ImportWallet (legacy) failed: %v", err)
	}

	privKey, err := svc.UnlockWallet(address, testPassword)
	if err != nil {
		t.Fatalf("UnlockWallet (legacy) failed: %v", err)
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
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
	}

	_, err = svc.UnlockWallet(address, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("UnlockWallet with wrong password should return error")
	}
	if !errors.Is(err, models.ErrCryptoFailed) {
		t.Errorf("expected ErrCryptoFailed, got: %v", err)
	}
}

func TestImportWeakPasswordRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	_, err := svc.ImportWallet(words, testWeakPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("ImportWallet with weak password should return error")
	}
	if !errors.Is(err, models.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got: %v", err)
	}
}

func TestImportInvalidSeedPhrase(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := []string{"invalid", "words", "that", "are", "not", "a", "valid", "bip39", "mnemonic", "at", "all", "here"}

	_, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("ImportWallet with invalid seed phrase should return error")
	}
}

func TestUpdateWalletPasswordAndUnlock(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	// Import with original password
	address, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
	}

	// Update password
	newPassword := "NewStr0ng!Pass#2"
	updatedAddr, err := svc.UpdateWalletPassword(words, newPassword)
	if err != nil {
		t.Fatalf("UpdateWalletPassword failed: %v", err)
	}
	if updatedAddr != address {
		t.Errorf("UpdateWalletPassword returned different address: %s vs %s", updatedAddr, address)
	}

	// Old password should fail
	_, err = svc.UnlockWallet(address, testPassword)
	if err == nil {
		t.Fatal("UnlockWallet with old password should fail after UpdateWalletPassword")
	}

	// New password should work
	privKey, err := svc.UnlockWallet(address, newPassword)
	if err != nil {
		t.Fatalf("UnlockWallet with new password failed: %v", err)
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

func TestUpdateWalletPasswordUsesDualSalt(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	address, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
	}

	newPassword := "NewStr0ng!Pass#2"
	_, err = svc.UpdateWalletPassword(words, newPassword)
	if err != nil {
		t.Fatalf("UpdateWalletPassword failed: %v", err)
	}

	ekd, err := db.GetEncryptedKeypair(address)
	if err != nil {
		t.Fatalf("GetEncryptedKeypair failed: %v", err)
	}

	if ekd.IsLegacyFormat() {
		t.Error("after UpdateWalletPassword, keypair should NOT be legacy format")
	}
	if ekd.Argon2Version != 2 {
		t.Errorf("Argon2Version = %d, want 2", ekd.Argon2Version)
	}
}

func TestUpdateWalletPasswordWeakRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	_, err := svc.ImportWallet(words, testPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("ImportWallet failed: %v", err)
	}

	_, err = svc.UpdateWalletPassword(words, testWeakPassword)
	if err == nil {
		t.Fatal("UpdateWalletPassword with weak password should return error")
	}
	if !errors.Is(err, models.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got: %v", err)
	}
}

func TestUpdateWalletPasswordWalletNotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)

	// Use a different mnemonic that hasn't been imported
	words := strings.Split("zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong", " ")

	_, err := svc.UpdateWalletPassword(words, testPassword)
	if err == nil {
		t.Fatal("UpdateWalletPassword for non-existent wallet should return error")
	}
}

func TestLegacyMigrationOnUnlock(t *testing.T) {
	// Simulate a pre-migration wallet by inserting directly with old format
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
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
	privKey, err := svc.UnlockWallet(address, testPassword)
	if err != nil {
		t.Fatalf("UnlockWallet (legacy migration) failed: %v", err)
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
	privKey2, err := svc.UnlockWallet(address, testPassword)
	if err != nil {
		t.Fatalf("UnlockWallet (post-migration) failed: %v", err)
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
	svc := services.NewKeystoreService(db)
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
	_, err := svc.UnlockWallet(address, "Wr0ng!Password#9")
	if err == nil {
		t.Fatal("UnlockWallet with wrong password on legacy wallet should fail")
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

// TestImportSameSeedTwice verifies K10: importing the same seed phrase twice returns an error.
// The second import should fail because the wallet address (PRIMARY KEY) already exists.
func TestImportSameSeedTwice(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	// First import should succeed
	address, err := svc.ImportWallet(words, testPassword, "wallet-one", models.WalletTypeBIP44Standard, 0)
	if err != nil {
		t.Fatalf("first ImportWallet failed: %v", err)
	}
	if address == "" {
		t.Fatal("first ImportWallet returned empty address")
	}

	// Second import of the same seed should fail (UNIQUE constraint on address)
	_, err = svc.ImportWallet(words, testPassword, "wallet-two", models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("second ImportWallet with same seed should return error")
	}
	// The error should be a database-level constraint violation (not a sentinel we wrap)
	// but it should definitely be non-nil
	t.Logf("duplicate import error (expected): %v", err)
}

// TestImportPasswordTooShort verifies K3: password shorter than 12 chars is rejected at service level.
func TestImportPasswordTooShort(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	// 11 chars — meets all char class requirements but is too short
	shortPassword := "Abcdefghi1!"
	_, err := svc.ImportWallet(words, shortPassword, "test-wallet", models.WalletTypeBIP44Standard, 0)
	if err == nil {
		t.Fatal("ImportWallet with 11-char password should return error")
	}
	if !errors.Is(err, models.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword for short password, got: %v", err)
	}
}

// TestImportPasswordMissingCharClasses verifies K4: passwords missing required char classes are rejected.
func TestImportPasswordMissingCharClasses(t *testing.T) {
	db := setupTestDB(t)
	svc := services.NewKeystoreService(db)
	words := strings.Split(testMnemonic, " ")

	tests := []struct {
		name     string
		password string
	}{
		{"no uppercase", "nouppercase1!"},
		{"no lowercase", "NOLOWERCASE1!"},
		{"no digit", "NoDigitHere!!"},
		{"no special", "NoSpecial1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ImportWallet(words, tt.password, "test-wallet", models.WalletTypeBIP44Standard, 0)
			if err == nil {
				t.Fatalf("ImportWallet with password %q (missing char class) should return error", tt.password)
			}
			if !errors.Is(err, models.ErrWeakPassword) {
				t.Errorf("expected ErrWeakPassword, got: %v", err)
			}
		})
	}
}
