package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// execer abstracts *sql.DB and *sql.Tx so insert methods work with either.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// EncryptedKeypairData holds all fields from the encrypted_keypairs table.
type EncryptedKeypairData struct {
	Address             string
	EncryptedPrivateKey []byte
	Salt                [16]byte // encryption salt
	Nonce               [12]byte
	Tag                 [16]byte
	PasswordHash        [32]byte
	VerifierSalt        []byte // nil = old format (pre-migration), non-nil = new dual-salt format
	Argon2Version       int    // 1 = V1 params, 2 = V2 params; 0 treated as 1
	Label               string
	IsPrimary           bool
	CreatedAt           int64
	LastUsed            int64
}

// IsLegacyFormat returns true if this keypair was stored before the dual-salt migration.
func (d EncryptedKeypairData) IsLegacyFormat() bool {
	return d.VerifierSalt == nil || len(d.VerifierSalt) == 0
}

// InsertEncryptedKeypair inserts a new encrypted keypair into the database.
func (d *Database) InsertEncryptedKeypair(data EncryptedKeypairData) error {
	now := time.Now().Unix()
	return d.insertEncryptedKeypair(d.db, data, now)
}

// InsertEncryptedKeypairTx inserts an encrypted keypair within a transaction.
func (d *Database) InsertEncryptedKeypairTx(tx *sql.Tx, data EncryptedKeypairData) error {
	now := time.Now().Unix()
	return d.insertEncryptedKeypair(tx, data, now)
}

func (d *Database) insertEncryptedKeypair(exec execer, data EncryptedKeypairData, now int64) error {
	isPrimary := 0
	if data.IsPrimary {
		isPrimary = 1
	}

	argonVersion := data.Argon2Version
	if argonVersion == 0 {
		argonVersion = 1 // default for backward compat
	}

	_, err := exec.Exec(
		`INSERT INTO encrypted_keypairs (address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at, last_used)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		data.Address, data.EncryptedPrivateKey,
		data.Salt[:], data.Nonce[:], data.Tag[:], data.PasswordHash[:],
		data.VerifierSalt, argonVersion,
		data.Label, isPrimary, now, now,
	)
	if err != nil {
		return fmt.Errorf("inserting encrypted keypair %q: %w", data.Address, err)
	}
	return nil
}

// GetEncryptedKeypair retrieves an encrypted keypair by address.
// BLOB fields are scanned as []byte and then copied into fixed-size arrays.
func (d *Database) GetEncryptedKeypair(addr string) (EncryptedKeypairData, error) {
	var data EncryptedKeypairData
	var isPrimary int
	var lastUsed sql.NullInt64
	var argonVersion sql.NullInt64

	// Use []byte intermediaries for BLOB fields
	var salt, nonce, tag, passwordHash []byte

	err := d.db.QueryRow(
		`SELECT address, encrypted_private_key, salt, nonce, tag, password_hash, verifier_salt, argon2_version, label, is_primary, created_at, last_used
		 FROM encrypted_keypairs WHERE address = ?`, addr,
	).Scan(&data.Address, &data.EncryptedPrivateKey, &salt, &nonce, &tag, &passwordHash,
		&data.VerifierSalt, &argonVersion,
		&data.Label, &isPrimary, &data.CreatedAt, &lastUsed)
	if err != nil {
		if err == sql.ErrNoRows {
			return EncryptedKeypairData{}, fmt.Errorf("getting encrypted keypair %q: %w", addr, models.ErrKeyNotFound)
		}
		return EncryptedKeypairData{}, fmt.Errorf("getting encrypted keypair %q: %w", addr, err)
	}

	data.IsPrimary = isPrimary != 0
	if lastUsed.Valid {
		data.LastUsed = lastUsed.Int64
	}
	if argonVersion.Valid {
		data.Argon2Version = int(argonVersion.Int64)
	} else {
		data.Argon2Version = 1 // pre-migration default
	}

	// Copy BLOB data into fixed-size arrays
	copy(data.Salt[:], salt)
	copy(data.Nonce[:], nonce)
	copy(data.Tag[:], tag)
	copy(data.PasswordHash[:], passwordHash)

	return data, nil
}

// UpdateEncryptedKeypair updates all crypto fields for an existing encrypted keypair.
// Used during C1 migration (re-encrypt with dual salts) and password updates.
func (d *Database) UpdateEncryptedKeypair(data EncryptedKeypairData) error {
	isPrimary := 0
	if data.IsPrimary {
		isPrimary = 1
	}

	argonVersion := data.Argon2Version
	if argonVersion == 0 {
		argonVersion = 1
	}

	result, err := d.db.Exec(
		`UPDATE encrypted_keypairs
		 SET encrypted_private_key = ?, salt = ?, nonce = ?, tag = ?,
		     password_hash = ?, verifier_salt = ?, argon2_version = ?,
		     label = ?, is_primary = ?
		 WHERE address = ?`,
		data.EncryptedPrivateKey, data.Salt[:], data.Nonce[:], data.Tag[:],
		data.PasswordHash[:], data.VerifierSalt, argonVersion,
		data.Label, isPrimary, data.Address,
	)
	if err != nil {
		return fmt.Errorf("updating encrypted keypair %q: %w", data.Address, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for keypair update %q: %w", data.Address, err)
	}
	if rows == 0 {
		return fmt.Errorf("updating encrypted keypair %q: %w", data.Address, models.ErrKeyNotFound)
	}

	return nil
}

// UpdateKeypairLastUsed updates the last_used timestamp for a keypair.
func (d *Database) UpdateKeypairLastUsed(addr string) error {
	now := time.Now().Unix()

	result, err := d.db.Exec(
		`UPDATE encrypted_keypairs SET last_used = ? WHERE address = ?`, now, addr,
	)
	if err != nil {
		return fmt.Errorf("updating last_used for keypair %q: %w", addr, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for keypair last_used %q: %w", addr, err)
	}
	if rows == 0 {
		return fmt.Errorf("updating last_used for keypair %q: %w", addr, models.ErrKeyNotFound)
	}

	return nil
}
