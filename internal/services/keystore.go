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

	// 5. Generate salt
	salt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("import keypair: generate salt: %w", err)
	}

	// 6. Derive AES key
	aesKey := keystore.DeriveKey(password, salt)
	defer keystore.ZeroBytes(aesKey[:])

	// 7. Hash password
	passwordHash := keystore.HashPassword(password, salt)

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

	// 11. Build EncryptedKeypairData
	ekd := database.EncryptedKeypairData{
		Address:             address,
		EncryptedPrivateKey: encrypted.Ciphertext,
		Salt:                salt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
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

	// 2. Derive AES key from password + stored salt
	aesKey := keystore.DeriveKey(password, ekd.Salt)
	defer keystore.ZeroBytes(aesKey[:])

	// 3. Hash password and compare with stored hash
	passwordHash := keystore.HashPassword(password, ekd.Salt)
	if subtle.ConstantTimeCompare(passwordHash[:], ekd.PasswordHash[:]) != 1 {
		return nil, fmt.Errorf("unlock keypair: wrong password: %w", models.ErrCryptoFailed)
	}

	// 4. Decrypt
	encData := keystore.EncryptedData{
		Ciphertext: ekd.EncryptedPrivateKey,
		Nonce:      ekd.Nonce,
		Tag:        ekd.Tag,
	}
	plaintext, err := keystore.Decrypt(encData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("unlock keypair: decrypt: %w", models.ErrCryptoFailed)
	}

	// 5. Reconstruct ed25519 key from 32-byte seed
	privKey := ed25519.NewKeyFromSeed(plaintext)

	// 6. Update last_used timestamp (best-effort)
	_ = db.UpdateKeypairLastUsed(address)

	// 7. Return private key
	return privKey, nil
}

// UpdatePassword re-encrypts a keypair with a new password.
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

	// 4. Re-encrypt with new password
	salt, err := keystore.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("update password: generate salt: %w", err)
	}

	aesKey := keystore.DeriveKey(newPassword, salt)
	defer keystore.ZeroBytes(aesKey[:])

	passwordHash := keystore.HashPassword(newPassword, salt)

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
		Salt:                salt,
		Nonce:               encrypted.Nonce,
		Tag:                 encrypted.Tag,
		PasswordHash:        passwordHash,
		Label:               existingEKD.Label,
		IsPrimary:           existingEKD.IsPrimary,
	}

	if err := db.UpdateEncryptedKeypair(ekd); err != nil {
		return "", fmt.Errorf("update password: update encrypted keypair: %w", err)
	}

	return address, nil
}
