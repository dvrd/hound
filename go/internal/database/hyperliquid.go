package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// EncryptedHyperliquidData holds all fields from the hyperliquid_wallets table.
type EncryptedHyperliquidData struct {
	Address            string
	Label              string
	APIWalletName      string
	EncryptedAPIKey    []byte
	EncryptedAPISecret []byte
	Salt               [16]byte
	NonceKey           [12]byte
	NonceSecret        [12]byte
	TagKey             [16]byte
	TagSecret          [16]byte
	PasswordHash       [32]byte
	IsActive           bool
	CreatedAt          int64
	LastUsed           int64
}

// SaveHyperliquidWallet inserts or replaces a Hyperliquid wallet.
func (d *Database) SaveHyperliquidWallet(data EncryptedHyperliquidData) error {
	now := time.Now().Unix()
	isActive := 0
	if data.IsActive {
		isActive = 1
	}

	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO hyperliquid_wallets
		 (address, label, api_wallet_name, encrypted_api_key, encrypted_api_secret,
		  salt, nonce_key, nonce_secret, tag_key, tag_secret, password_hash,
		  is_active, created_at, last_used)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		data.Address, data.Label, data.APIWalletName,
		data.EncryptedAPIKey, data.EncryptedAPISecret,
		data.Salt[:], data.NonceKey[:], data.NonceSecret[:],
		data.TagKey[:], data.TagSecret[:], data.PasswordHash[:],
		isActive, now, now,
	)
	if err != nil {
		return fmt.Errorf("saving hyperliquid wallet %q: %w", data.Address, err)
	}
	return nil
}

// LoadHyperliquidWallets retrieves all Hyperliquid wallets with minimal fields (no encrypted data).
func (d *Database) LoadHyperliquidWallets() ([]EncryptedHyperliquidData, error) {
	rows, err := d.db.Query(
		`SELECT address, label, api_wallet_name, is_active, created_at, last_used
		 FROM hyperliquid_wallets`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying hyperliquid wallets: %w", err)
	}
	defer rows.Close()

	var wallets []EncryptedHyperliquidData
	for rows.Next() {
		var w EncryptedHyperliquidData
		var isActive int
		var lastUsed sql.NullInt64

		if err := rows.Scan(&w.Address, &w.Label, &w.APIWalletName, &isActive, &w.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scanning hyperliquid wallet row: %w", err)
		}
		w.IsActive = isActive != 0
		if lastUsed.Valid {
			w.LastUsed = lastUsed.Int64
		}

		wallets = append(wallets, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hyperliquid wallet rows: %w", err)
	}

	return wallets, nil
}

// LoadHyperliquidWalletCredentials retrieves a full Hyperliquid wallet including all encrypted fields.
func (d *Database) LoadHyperliquidWalletCredentials(addr string) (EncryptedHyperliquidData, error) {
	var data EncryptedHyperliquidData
	var isActive int
	var lastUsed sql.NullInt64

	// Use []byte intermediaries for BLOB fields
	var salt, nonceKey, nonceSecret, tagKey, tagSecret, passwordHash []byte

	err := d.db.QueryRow(
		`SELECT address, label, api_wallet_name, encrypted_api_key, encrypted_api_secret,
		 salt, nonce_key, nonce_secret, tag_key, tag_secret, password_hash,
		 is_active, created_at, last_used
		 FROM hyperliquid_wallets WHERE address = ?`, addr,
	).Scan(&data.Address, &data.Label, &data.APIWalletName,
		&data.EncryptedAPIKey, &data.EncryptedAPISecret,
		&salt, &nonceKey, &nonceSecret, &tagKey, &tagSecret, &passwordHash,
		&isActive, &data.CreatedAt, &lastUsed)
	if err != nil {
		if err == sql.ErrNoRows {
			return EncryptedHyperliquidData{}, fmt.Errorf("loading hyperliquid wallet %q: %w", addr, models.ErrWalletNotFound)
		}
		return EncryptedHyperliquidData{}, fmt.Errorf("loading hyperliquid wallet %q: %w", addr, err)
	}

	data.IsActive = isActive != 0
	if lastUsed.Valid {
		data.LastUsed = lastUsed.Int64
	}

	// Copy BLOB data into fixed-size arrays
	copy(data.Salt[:], salt)
	copy(data.NonceKey[:], nonceKey)
	copy(data.NonceSecret[:], nonceSecret)
	copy(data.TagKey[:], tagKey)
	copy(data.TagSecret[:], tagSecret)
	copy(data.PasswordHash[:], passwordHash)

	return data, nil
}

// SetActiveHyperliquidWallet sets the given wallet as active in a transaction.
func (d *Database) SetActiveHyperliquidWallet(addr string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for set active hyperliquid wallet: %w", err)
	}
	defer tx.Rollback()

	// Verify wallet exists
	var exists int
	err = tx.QueryRow(`SELECT COUNT(*) FROM hyperliquid_wallets WHERE address = ?`, addr).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking hyperliquid wallet existence %q: %w", addr, err)
	}
	if exists == 0 {
		return fmt.Errorf("setting active hyperliquid wallet %q: %w", addr, models.ErrWalletNotFound)
	}

	// Unset all active flags
	_, err = tx.Exec(`UPDATE hyperliquid_wallets SET is_active = 0 WHERE is_active = 1`)
	if err != nil {
		return fmt.Errorf("unsetting active hyperliquid wallets: %w", err)
	}

	// Set new active
	_, err = tx.Exec(`UPDATE hyperliquid_wallets SET is_active = 1 WHERE address = ?`, addr)
	if err != nil {
		return fmt.Errorf("setting active hyperliquid wallet %q: %w", addr, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing set active hyperliquid wallet: %w", err)
	}

	return nil
}

// GetActiveHyperliquidWallet retrieves the currently active Hyperliquid wallet with full credentials.
func (d *Database) GetActiveHyperliquidWallet() (EncryptedHyperliquidData, error) {
	var addr string
	err := d.db.QueryRow(
		`SELECT address FROM hyperliquid_wallets WHERE is_active = 1`,
	).Scan(&addr)
	if err != nil {
		if err == sql.ErrNoRows {
			return EncryptedHyperliquidData{}, fmt.Errorf("getting active hyperliquid wallet: %w", models.ErrWalletNotFound)
		}
		return EncryptedHyperliquidData{}, fmt.Errorf("getting active hyperliquid wallet: %w", err)
	}

	return d.LoadHyperliquidWalletCredentials(addr)
}
