package services

import (
	"crypto/ed25519"
	"crypto/subtle"
	"fmt"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
)

// KeystoreService handles wallet import, unlock, and password update.
type KeystoreService struct{}

// ImportKeypair derives a keypair from seed phrase, encrypts it, and stores it in the database.
// Uses dual-salt derivation: encryption_salt for AES key, verifier_salt for password hash.
// Returns the wallet address.
func (s *KeystoreService) ImportKeypair(
	db *database.Database,
	words []string,
	password string,
	label string,
	isPrimary bool,
	walletType models.WalletType,
	accountIndex int,
) (string, error) {
	// 1. Validate password strength
	if err := keystore.ValidatePasswordStrength(password); err != nil {
		return "", fmt.Errorf("import keypair: %w", models.ErrWeakPassword)
	}

	// 2. Derive keypair
	var kp keystore.Keypair
	var err error
	if walletType == models.WalletTypeLegacy {
		kp, err = keystore.DeriveKeypairLegacy(words)
	} else {
		kp, err = keystore.DeriveKeypairBIP44(words, walletType, accountIndex)
	}
	if err != nil {
		return "", fmt.Errorf("import keypair: derive: %w", err)
	}
	// 3. Zero keypair when done
	defer keystore.ZeroKeypair(&kp)

	// 4. Get address
	address := keystore.KeypairToAddress(kp)

	// 5. Generate TWO salts (C1 fix: dual-salt derivation)
	encryptionSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate encryption salt: %w", err)
	}
	verifierSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate verifier salt: %w", err)
	}

	// 6. Derive AES key from encryption salt (NEVER stored)
	aesKey := keystore.DeriveKeyV2(password, encryptionSalt)
	defer keystore.ZeroBytes(aesKey[:])

	// 7. Derive password hash from DIFFERENT verifier salt (stored, useless for decryption)
	passwordHash := keystore.DeriveKeyV2(password, verifierSalt)

	// 8. Generate nonce
	nonce, err := keystore.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate nonce: %w", err)
	}

	// 9. Extract private key seed (first 32 bytes of ed25519.PrivateKey)
	seed := kp.PrivateKey.Seed()

	// 10. Encrypt
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("import keypair: encrypt: %w", err)
	}

	// 11. Build EncryptedKeypairData with dual salts
	ekd := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                encryptionSalt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
		VerifierSalt:        verifierSalt[:],
		Argon2Version:       2,
		Label:               label,
		IsPrimary:           isPrimary,
	}

	// 12. Insert encrypted keypair
	if err := db.InsertEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("import keypair: insert encrypted keypair: %w", err)
	}

	// 13. Build and insert wallet
	wallet := models.Wallet{
		Address:        address,
		Label:          label,
		IsPrimary:      isPrimary,
		WalletType:     walletType,
		DerivationPath: models.GetDerivationPath(walletType, accountIndex),
		AccountIndex:   accountIndex,
	}
	if err := db.InsertWallet(wallet); err != nil {
		return "", fmt.Errorf("import keypair: insert wallet: %w", err)
	}

	// 14. Return address
	return address, nil
}

// UnlockKeypair decrypts a stored keypair using the password.
// Handles both legacy (single-salt) and modern (dual-salt) formats.
// Legacy wallets are transparently migrated to dual-salt + V2 params on first unlock.
// Returns the Ed25519 private key. Caller must zero it when done.
func (s *KeystoreService) UnlockKeypair(
	db *database.Database,
	address string,
	password string,
) (ed25519.PrivateKey, error) {
	// 1. Get encrypted data
	ekd, err := db.GetEncryptedKeypair(address)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: %w", err)
	}

	if ekd.IsLegacyFormat() {
		return s.unlockAndMigrateLegacy(db, ekd, password)
	}
	return s.unlockModern(db, ekd, password)
}

// unlockAndMigrateLegacy handles old-format wallets (verifier_salt IS NULL).
// Steps: verify with old method → decrypt → re-encrypt with dual salts + V2 → update DB.
func (s *KeystoreService) unlockAndMigrateLegacy(
	db *database.Database,
	ekd database.EncryptedKeypairData,
	password string,
) (ed25519.PrivateKey, error) {
	// Determine which Argon2 version was used for this keypair
	version := keystore.Argon2Version(ekd.Argon2Version)
	if version == 0 {
		version = keystore.Argon2VersionV1
	}

	// Old format: password_hash == DeriveKey(password, salt) with same salt
	oldKey := keystore.DeriveKeyVersioned(password, ekd.Salt, version)
	defer keystore.ZeroBytes(oldKey[:])

	// Verify: in old format, the stored hash IS the key
	if subtle.ConstantTimeCompare(oldKey[:], ekd.PasswordHash[:]) != 1 {
		return nil, fmt.Errorf("unlock keypair: wrong password: %w", models.ErrCryptoFailed)
	}

	// Decrypt with old key
	encData := keystore.EncryptedData{
		Ciphertext: ekd.EncryptedPrivateKey,
		Nonce:      ekd.Nonce,
		Tag:        ekd.Tag,
	}
	plaintext, err := keystore.Decrypt(encData, oldKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}
	defer keystore.ZeroBytes(plaintext)

	// Reconstruct ed25519 key
	privKey := ed25519.NewKeyFromSeed(plaintext)

	// === MIGRATION: re-encrypt with dual salts + V2 params ===
	newEncSalt, err := keystore.GenerateSalt()
	if err != nil {
		// Migration failed but unlock succeeded — return key, skip migration
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}
	newVerSalt, err := keystore.GenerateSalt()
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newKey := keystore.DeriveKeyV2(password, newEncSalt)
	defer keystore.ZeroBytes(newKey[:])

	newHash := keystore.DeriveKeyV2(password, newVerSalt)

	newNonce, err := keystore.GenerateNonce()
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newEncrypted, err := keystore.Encrypt(plaintext, newKey, newNonce)
	if err != nil {
		_ = db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	migratedEKD := database.EncryptedKeypairData{
		Address:             ekd.Address,
		EncryptedPrivateKey: newEncrypted.Ciphertext,
		Salt:                newEncSalt,
		Nonce:               newEncrypted.Nonce,
		Tag:                 newEncrypted.Tag,
		PasswordHash:        newHash,
		VerifierSalt:        newVerSalt[:],
		Argon2Version:       2,
		Label:               ekd.Label,
		IsPrimary:           ekd.IsPrimary,
	}

	// Best-effort migration — if it fails, the old format still works next time
	_ = db.UpdateEncryptedKeypair(migratedEKD)
	_ = db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// unlockModern handles new-format wallets (verifier_salt IS NOT NULL).
// Steps: verify password with verifier_salt → derive key from encryption salt → decrypt.
func (s *KeystoreService) unlockModern(
	db *database.Database,
	ekd database.EncryptedKeypairData,
	password string,
) (ed25519.PrivateKey, error) {
	// Determine Argon2 version
	version := keystore.Argon2Version(ekd.Argon2Version)
	if version == 0 {
		version = keystore.Argon2VersionV2
	}

	// Copy verifier salt into fixed-size array
	var verSalt [keystore.Argon2SaltBytes]byte
	copy(verSalt[:], ekd.VerifierSalt)

	// Verify password using verifier salt (safe — hash ≠ key)
	checkHash := keystore.DeriveKeyVersioned(password, verSalt, version)
	if subtle.ConstantTimeCompare(checkHash[:], ekd.PasswordHash[:]) != 1 {
		return nil, fmt.Errorf("unlock keypair: wrong password: %w", models.ErrCryptoFailed)
	}

	// Derive AES key from encryption salt (different from verifier salt)
	aesKey := keystore.DeriveKeyVersioned(password, ekd.Salt, version)
	defer keystore.ZeroBytes(aesKey[:])

	// Decrypt
	encData := keystore.EncryptedData{
		Ciphertext: ekd.EncryptedPrivateKey,
		Nonce:      ekd.Nonce,
		Tag:        ekd.Tag,
	}
	plaintext, err := keystore.Decrypt(encData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}
	defer keystore.ZeroBytes(plaintext)

	// Reconstruct ed25519 key
	privKey := ed25519.NewKeyFromSeed(plaintext)

	// Update last_used (best-effort)
	_ = db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// UpdatePassword re-encrypts a keypair with a new password using dual-salt derivation.
// Requires the seed phrase to verify identity.
func (s *KeystoreService) UpdatePassword(
	db *database.Database,
	words []string,
	newPassword string,
) (string, error) {
	// 1. Validate new password strength
	if err := keystore.ValidatePasswordStrength(newPassword); err != nil {
		return "", fmt.Errorf("update password: %w", models.ErrWeakPassword)
	}

	// 2. Derive keypair from words (try BIP44Standard first, then Legacy)
	var kp keystore.Keypair
	var err error
	kp, err = keystore.DeriveKeypairBIP44(words, models.WalletTypeBIP44Standard, 0)
	if err != nil {
		// Fall back to legacy
		kp, err = keystore.DeriveKeypairLegacy(words)
		if err != nil {
			return "", fmt.Errorf("update password: derive keypair: %w", err)
		}
	}
	defer keystore.ZeroKeypair(&kp)

	// 3. Get address, check if wallet exists in DB
	address := keystore.KeypairToAddress(kp)
	_, err = db.GetWalletByAddress(address)
	if err != nil {
		// Try legacy if BIP44 address wasn't found
		kpLegacy, errLegacy := keystore.DeriveKeypairLegacy(words)
		if errLegacy != nil {
			return "", fmt.Errorf("update password: %w", err)
		}
		defer keystore.ZeroKeypair(&kpLegacy)

		legacyAddr := keystore.KeypairToAddress(kpLegacy)
		_, errWallet := db.GetWalletByAddress(legacyAddr)
		if errWallet != nil {
			return "", fmt.Errorf("update password: wallet not found: %w", models.ErrWalletNotFound)
		}
		address = legacyAddr
		kp = kpLegacy
	}

	// 4. Re-encrypt with new password using dual-salt derivation
	encryptionSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("update password: generate encryption salt: %w", err)
	}
	verifierSalt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("update password: generate verifier salt: %w", err)
	}

	aesKey := keystore.DeriveKeyV2(newPassword, encryptionSalt)
	defer keystore.ZeroBytes(aesKey[:])

	passwordHash := keystore.DeriveKeyV2(newPassword, verifierSalt)

	nonce, err := keystore.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("update password: generate nonce: %w", err)
	}

	seed := kp.PrivateKey.Seed()
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("update password: encrypt: %w", err)
	}

	// Get existing keypair data for label/isPrimary
	existingEKD, err := db.GetEncryptedKeypair(address)
	if err != nil {
		return "", fmt.Errorf("update password: get existing keypair: %w", err)
	}

	ekd := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                encryptionSalt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
		VerifierSalt:        verifierSalt[:],
		Argon2Version:       2,
		Label:               existingEKD.Label,
		IsPrimary:           existingEKD.IsPrimary,
	}

	if err := db.UpdateEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("update password: update encrypted keypair: %w", err)
	}

	return address, nil
}
