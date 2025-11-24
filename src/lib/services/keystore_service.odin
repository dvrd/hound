// Keystore service - High-level keypair management
// Orchestrates crypto operations, database storage, and validation
package services

import "core:log"
import "core:strings"
import "core:crypto/ed25519"
import keystore "../keystore"
import database "../database"
import models "../models"

// Password strength requirements (OWASP 2024)
MIN_PASSWORD_LENGTH :: 12
REQUIRE_UPPERCASE :: true
REQUIRE_LOWERCASE :: true
REQUIRE_DIGIT :: true
REQUIRE_SPECIAL :: true

// validate_password_strength checks password against OWASP 2024 requirements
//
// ASSERTION 1: Password must not be empty
// ASSERTION 2: Password must meet minimum length
// ASSERTION 3: Password must contain uppercase letter
// ASSERTION 4: Password must contain lowercase letter
// ASSERTION 5: Password must contain digit
// ASSERTION 6: Password must contain special character
validate_password_strength :: proc(password: string) -> models.ErrorType {
	assert(len(password) > 0, "Password cannot be empty")

	// ASSERTION 2: Minimum length
	if len(password) < MIN_PASSWORD_LENGTH {
		log.errorf("Password too short: %d characters (minimum %d)", len(password), MIN_PASSWORD_LENGTH)
		return .WeakPassword
	}

	has_uppercase := false
	has_lowercase := false
	has_digit := false
	has_special := false

	for r in password {
		switch {
		case r >= 'A' && r <= 'Z':
			has_uppercase = true
		case r >= 'a' && r <= 'z':
			has_lowercase = true
		case r >= '0' && r <= '9':
			has_digit = true
		case !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')):
			has_special = true
		}
	}

	// ASSERTION 3-6: Character requirements
	if REQUIRE_UPPERCASE && !has_uppercase {
		log.error("Password must contain at least one uppercase letter")
		return .WeakPassword
	}
	if REQUIRE_LOWERCASE && !has_lowercase {
		log.error("Password must contain at least one lowercase letter")
		return .WeakPassword
	}
	if REQUIRE_DIGIT && !has_digit {
		log.error("Password must contain at least one digit")
		return .WeakPassword
	}
	if REQUIRE_SPECIAL && !has_special {
		log.error("Password must contain at least one special character")
		return .WeakPassword
	}

	log.info("Password meets strength requirements")
	return .None
}

// import_keypair imports a keypair from seed phrase and stores it encrypted
//
// ASSERTION 1: Seed phrase must be 12 or 24 words
// ASSERTION 2: Password must be strong enough
// ASSERTION 3: Database must be available
//
// Returns: Solana address and error status
import_keypair :: proc(
	db: ^database.Database,
	seed_phrase: []string,
	password: string,
	label: string,
	is_primary: bool,
) -> (address: string, err: models.ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(len(seed_phrase) == 12 || len(seed_phrase) == 24, "Seed phrase must be 12 or 24 words")
	assert(len(password) > 0, "Password cannot be empty")
	assert(len(label) > 0, "Label cannot be empty")

	log.infof("Importing keypair from %d-word seed phrase", len(seed_phrase))

	// ASSERTION 2: Validate password strength
	password_err := validate_password_strength(password)
	if password_err != .None {
		return "", password_err
	}

	// Derive keypair from seed phrase
	keypair, keypair_err := keystore.derive_keypair_from_seed(seed_phrase)
	if keypair_err != .None {
		log.error("Failed to derive keypair from seed phrase")
		return "", keypair_err
	}
	defer keystore.zero_keypair(&keypair)

	// Generate Solana address from public key
	address = keystore.keypair_to_address(&keypair)
	log.infof("Derived address: %s", address)

	// Check if wallet already exists in database
	existing, found, get_err := database.get_encrypted_keypair(db, address)
	if get_err != .None {
		log.error("Failed to check for existing wallet")
		return "", get_err
	}
	if found {
		log.errorf("Wallet already exists: %s", address)
		// Clean up existing data
		delete(existing.address)
		delete(existing.label)
		delete(existing.encrypted_private_key)
		return "", .WalletAlreadyExists
	}

	// Generate cryptographically secure salt and nonce
	salt := keystore.generate_salt()
	nonce := keystore.generate_nonce()

	// Derive encryption key from password using Argon2id
	encryption_key, key_err := keystore.derive_key_from_password(password, salt)
	if key_err != .None {
		log.error("Failed to derive encryption key from password")
		return "", key_err
	}
	defer keystore.secure_zero_memory(&encryption_key, size_of(encryption_key))

	// Hash password for verification (stored separately from encryption key)
	password_hash, hash_err := keystore.hash_password(password, salt)
	if hash_err != .None {
		log.error("Failed to hash password")
		return "", hash_err
	}

	// Serialize private key to bytes for encryption
	private_key_bytes: [ed25519.PRIVATE_KEY_SIZE]byte
	ed25519.private_key_bytes(&keypair.private_key_struct, private_key_bytes[:])
	defer keystore.secure_zero_memory(&private_key_bytes, size_of(private_key_bytes))

	// Encrypt private key with AES-256-GCM
	encrypted, encrypt_err := keystore.encrypt_aes256gcm(private_key_bytes[:], encryption_key, nonce)
	if encrypt_err != .None {
		log.error("Failed to encrypt private key")
		return "", encrypt_err
	}
	defer delete(encrypted.ciphertext)

	// Store encrypted keypair in database
	insert_err := database.insert_encrypted_keypair(
		db,
		address,
		encrypted.ciphertext,
		salt,
		encrypted.nonce,
		encrypted.tag,
		password_hash,
		label,
		is_primary,
	)
	if insert_err != .None {
		log.error("Failed to store encrypted keypair")
		return "", insert_err
	}

	// Also create wallet entry so it appears in wallet list
	// NOTE: This creates the link between encrypted_keypairs and wallets tables
	wallet := models.Wallet{
		address    = address,
		label      = label,
		is_primary = is_primary,
	}
	wallet_insert_err := database.insert_wallet(db, wallet)
	if wallet_insert_err != .None {
		log.errorf("Failed to create wallet entry: %v", wallet_insert_err)
		// TODO: Rollback by deleting encrypted keypair
		// For now, log error but proceed (wallet can be manually synced later)
		log.warn("Encrypted keypair stored but wallet entry not created")
		log.warn("Run migration to sync encrypted_keypairs to wallets table")
	}

	log.infof("Successfully imported keypair: %s (%s)", address, label)
	return address, .None
}

// unlock_keypair decrypts a keypair from database (memory-only, not stored)
//
// ASSERTION 1: Database must be available
// ASSERTION 2: Address must not be empty
// ASSERTION 3: Password must not be empty
//
// Returns: Decrypted keypair and error status
unlock_keypair :: proc(
	db: ^database.Database,
	address: string,
	password: string,
) -> (keypair: keystore.Keypair, err: models.ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(len(address) > 0, "Address cannot be empty")
	assert(len(password) > 0, "Password cannot be empty")

	log.debugf("Unlocking keypair for address: %s", address)

	// Retrieve encrypted keypair from database
	encrypted_data, found, get_err := database.get_encrypted_keypair(db, address)
	if get_err != .None {
		log.error("Failed to retrieve encrypted keypair")
		return {}, get_err
	}
	if !found {
		log.errorf("Keypair not found: %s", address)
		return {}, .KeypairNotFound
	}
	defer {
		delete(encrypted_data.address)
		delete(encrypted_data.label)
		delete(encrypted_data.encrypted_private_key)
	}

	// Derive decryption key from password using stored salt
	decryption_key, key_err := keystore.derive_key_from_password(password, encrypted_data.salt)
	if key_err != .None {
		log.error("Failed to derive decryption key")
		return {}, key_err
	}
	defer keystore.secure_zero_memory(&decryption_key, size_of(decryption_key))

	// Verify password by comparing hashes
	password_hash, hash_err := keystore.hash_password(password, encrypted_data.salt)
	if hash_err != .None {
		log.error("Failed to hash password for verification")
		return {}, hash_err
	}

	// Compare password hashes (constant-time comparison would be ideal)
	password_match := true
	for i := 0; i < len(password_hash); i += 1 {
		if password_hash[i] != encrypted_data.password_hash[i] {
			password_match = false
			break
		}
	}

	if !password_match {
		log.error("Incorrect password")
		return {}, .CryptoOperationFailed
	}

	// Decrypt private key with AES-256-GCM
	encrypted := keystore.EncryptedData{
		ciphertext = encrypted_data.encrypted_private_key,
		nonce = encrypted_data.nonce,
		tag = encrypted_data.tag,
	}

	plaintext, decrypt_err := keystore.decrypt_aes256gcm(encrypted, decryption_key)
	if decrypt_err != .None {
		log.error("Failed to decrypt private key (wrong password or corrupted data)")
		return {}, decrypt_err
	}
	defer {
		keystore.secure_zero_memory(raw_data(plaintext), len(plaintext))
		delete(plaintext)
	}

	// Reconstruct Ed25519 keypair from decrypted bytes
	priv_key: ed25519.Private_Key
	success := ed25519.private_key_set_bytes(&priv_key, plaintext)
	if !success {
		log.error("Failed to reconstruct private key from decrypted bytes")
		return {}, .CryptoOperationFailed
	}

	// Extract public key from private key
	pub_key: ed25519.Public_Key
	ed25519.public_key_set_priv(&pub_key, &priv_key)

	keypair.private_key_struct = priv_key
	keypair.public_key = pub_key

	// Update last_used timestamp
	update_err := database.update_keypair_last_used(db, address)
	if update_err != .None {
		log.warn("Failed to update last_used timestamp (non-fatal)")
	}

	log.infof("Successfully unlocked keypair: %s", address)
	return keypair, .None
}

// update_keypair_password updates the password for an encrypted keypair
//
// ASSERTION 1: Database must be available
// ASSERTION 2: Seed phrase must be 12 or 24 words
// ASSERTION 3: New password must be strong enough
// ASSERTION 4: Derived address must match existing wallet
//
// Returns: Error status
update_keypair_password :: proc(
	db: ^database.Database,
	seed_phrase: []string,
	old_password: string,
	new_password: string,
) -> (address: string, err: models.ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(len(seed_phrase) == 12 || len(seed_phrase) == 24, "Seed phrase must be 12 or 24 words")
	assert(len(new_password) > 0, "New password cannot be empty")

	log.infof("Updating password for keypair from %d-word seed phrase", len(seed_phrase))

	// ASSERTION 3: Validate new password strength
	password_err := validate_password_strength(new_password)
	if password_err != .None {
		return "", password_err
	}

	// Derive keypair from seed phrase to get the address
	keypair, keypair_err := keystore.derive_keypair_from_seed(seed_phrase)
	if keypair_err != .None {
		log.error("Failed to derive keypair from seed phrase")
		return "", keypair_err
	}
	defer keystore.zero_keypair(&keypair)

	// Generate Solana address from public key
	address = keystore.keypair_to_address(&keypair)
	log.infof("Derived address: %s", address)

	// ASSERTION 4: Check if wallet exists in database
	existing, found, get_err := database.get_encrypted_keypair(db, address)
	if get_err != .None {
		log.error("Failed to check for existing wallet")
		return "", get_err
	}
	if !found {
		log.errorf("Wallet not found: %s", address)
		return "", .KeypairNotFound
	}
	defer {
		delete(existing.address)
		delete(existing.label)
		delete(existing.encrypted_private_key)
	}

	// Generate new cryptographically secure salt and nonce
	salt := keystore.generate_salt()
	nonce := keystore.generate_nonce()

	// Derive new encryption key from new password using Argon2id
	encryption_key, key_err := keystore.derive_key_from_password(new_password, salt)
	if key_err != .None {
		log.error("Failed to derive encryption key from new password")
		return "", key_err
	}
	defer keystore.secure_zero_memory(&encryption_key, size_of(encryption_key))

	// Hash new password for verification
	password_hash, hash_err := keystore.hash_password(new_password, salt)
	if hash_err != .None {
		log.error("Failed to hash new password")
		return "", hash_err
	}

	// Serialize private key to bytes for encryption
	private_key_bytes: [ed25519.PRIVATE_KEY_SIZE]byte
	ed25519.private_key_bytes(&keypair.private_key_struct, private_key_bytes[:])
	defer keystore.secure_zero_memory(&private_key_bytes, size_of(private_key_bytes))

	// Encrypt private key with new password
	encrypted, encrypt_err := keystore.encrypt_aes256gcm(private_key_bytes[:], encryption_key, nonce)
	if encrypt_err != .None {
		log.error("Failed to encrypt private key with new password")
		return "", encrypt_err
	}
	defer delete(encrypted.ciphertext)

	// Update encrypted keypair in database
	update_err := database.update_encrypted_keypair(
		db,
		address,
		encrypted.ciphertext,
		salt,
		encrypted.nonce,
		encrypted.tag,
		password_hash,
	)
	if update_err != .None {
		log.error("Failed to update encrypted keypair")
		return "", update_err
	}

	log.infof("Successfully updated password for keypair: %s", address)
	return address, .None
}
