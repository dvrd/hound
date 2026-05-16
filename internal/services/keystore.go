package services

import (
	"crypto/ed25519"
	"crypto/subtle"
	"fmt"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
)

// KeyCustodian is the interface for wallet custody operations used by service consumers
// (SwapService, TransferService). It defines the seam where callers cross into the
// custody module; tests can supply an adapter.
//
// Note: UpdateWalletPassword lives on *KeystoreService directly (not on this interface)
// because no service consumer needs it — it's only called from TUI/CLI flows.
type KeyCustodian interface {
	ImportWallet(words []string, password, label string, walletType models.WalletType, accountIndex int) (string, error)
	UnlockWallet(address, password string) (ed25519.PrivateKey, error)
}

// KeystoreService handles wallet import, unlock, and password update.
// It holds the database reference so callers don't need to supply it per-call —
// deepening the module by hiding persistence behind the seam.
type KeystoreService struct {
	db *database.Database
}

// NewKeystoreService creates a new KeystoreService with the given database.
func NewKeystoreService(db *database.Database) *KeystoreService {
	return &KeystoreService{db: db}
}

// ImportWallet derives a keypair from seed phrase, encrypts it, and stores both the
// encrypted keypair and wallet record atomically in a single transaction.
// Uses dual-salt derivation: encryption_salt for AES key, verifier_salt for password hash.
// Returns the wallet address.
func (s *KeystoreService) ImportWallet(
	words []string,
	password string,
	label string,
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
	defer keystore.ZeroBytes(seed) // H5: zero seed after use

	// 10. Encrypt
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("import keypair: encrypt: %w", err)
	}

	// 11. Build EncryptedKeypairData with dual salts
	// 11b. Only set as primary if no wallets exist yet.
	isPrimary := false
	if count, err := s.db.WalletCount(); err == nil && count == 0 {
		isPrimary = true
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
		Label:               label,
		IsPrimary:           isPrimary,
	}

	// 12. Insert encrypted keypair and wallet atomically in a single transaction.
	tx, err := s.db.BeginTx()
	if err != nil {
		return "", fmt.Errorf("import keypair: begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.db.InsertEncryptedKeypairTx(tx, ekd); err != nil {
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
	if err := s.db.InsertWalletTx(tx, wallet); err != nil {
		return "", fmt.Errorf("import keypair: insert wallet: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("import keypair: commit: %w", err)
	}

	// 14. Return address
	return address, nil
}

// UnlockWallet decrypts a stored keypair using the password.
// Handles both legacy (single-salt) and modern (dual-salt) formats.
// Legacy wallets are transparently migrated to dual-salt + V2 params on first unlock.
// Returns the Ed25519 private key. Caller must zero it when done.
func (s *KeystoreService) UnlockWallet(
	address string,
	password string,
) (ed25519.PrivateKey, error) {
	// 1. Get encrypted data
	ekd, err := s.db.GetEncryptedKeypair(address)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: %w", err)
	}

	if ekd.IsLegacyFormat() {
		return s.unlockAndMigrateLegacy(ekd, password)
	}
	return s.unlockModern(ekd, password)
}

// unlockAndMigrateLegacy handles old-format wallets (verifier_salt IS NULL).
// Steps: verify with old method → decrypt → re-encrypt with dual salts + V2 → update DB.
func (s *KeystoreService) unlockAndMigrateLegacy(
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
		_ = s.db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}
	newVerSalt, err := keystore.GenerateSalt()
	if err != nil {
		_ = s.db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newKey := keystore.DeriveKeyV2(password, newEncSalt)
	defer keystore.ZeroBytes(newKey[:])

	newHash := keystore.DeriveKeyV2(password, newVerSalt)

	newNonce, err := keystore.GenerateNonce()
	if err != nil {
		_ = s.db.UpdateKeypairLastUsed(ekd.Address)
		return privKey, nil
	}

	newEncrypted, err := keystore.Encrypt(plaintext, newKey, newNonce)
	if err != nil {
		_ = s.db.UpdateKeypairLastUsed(ekd.Address)
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
	_ = s.db.UpdateEncryptedKeypair(migratedEKD)
	_ = s.db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// unlockModern handles new-format wallets (verifier_salt IS NOT NULL).
// Steps: verify password with verifier_salt → derive key from encryption salt → decrypt.
func (s *KeystoreService) unlockModern(
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
	_ = s.db.UpdateKeypairLastUsed(ekd.Address)

	return privKey, nil
}

// UpdateWalletPassword re-encrypts a keypair with a new password using dual-salt derivation.
// Requires the seed phrase to verify identity.
func (s *KeystoreService) UpdateWalletPassword(
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
	_, err = s.db.GetWalletByAddress(address)
	if err != nil {
		// Try legacy if BIP44 address wasn't found
		kpLegacy, errLegacy := keystore.DeriveKeypairLegacy(words)
		if errLegacy != nil {
			return "", fmt.Errorf("update password: %w", err)
		}
		defer keystore.ZeroKeypair(&kpLegacy)

		legacyAddr := keystore.KeypairToAddress(kpLegacy)
		_, errWallet := s.db.GetWalletByAddress(legacyAddr)
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
	defer keystore.ZeroBytes(seed) // H5: zero seed after use
	encrypted, err := keystore.Encrypt(seed, aesKey, nonce)
	if err != nil {
		return "", fmt.Errorf("update password: encrypt: %w", err)
	}

	// Get existing keypair data for label/isPrimary
	existingEKD, err := s.db.GetEncryptedKeypair(address)
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

	if err := s.db.UpdateEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("update password: update encrypted keypair: %w", err)
	}

	return address, nil
}
