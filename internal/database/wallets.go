package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// InsertWallet inserts a new wallet into the database.
func (d *Database) InsertWallet(wallet models.Wallet) error {
	now := time.Now().Unix()
	isPrimary := 0
	if wallet.IsPrimary {
		isPrimary = 1
	}

	_, err := d.db.Exec(
		`INSERT INTO wallets (address, label, is_primary, added_at, wallet_type, derivation_path, account_index)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		wallet.Address, wallet.Label, isPrimary, now,
		wallet.WalletType.String(), wallet.DerivationPath, wallet.AccountIndex,
	)
	if err != nil {
		return fmt.Errorf("inserting wallet %q: %w", wallet.Address, err)
	}
	return nil
}

// GetAllWallets retrieves all wallets ordered by is_primary DESC, added_at ASC.
func (d *Database) GetAllWallets() ([]models.Wallet, error) {
	rows, err := d.db.Query(
		`SELECT address, label, is_primary, wallet_type, derivation_path, account_index
		 FROM wallets ORDER BY is_primary DESC, added_at ASC, address ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying all wallets: %w", err)
	}
	defer rows.Close()

	var wallets []models.Wallet
	for rows.Next() {
		var w models.Wallet
		var isPrimary int
		var walletType string

		if err := rows.Scan(&w.Address, &w.Label, &isPrimary, &walletType, &w.DerivationPath, &w.AccountIndex); err != nil {
			return nil, fmt.Errorf("scanning wallet row: %w", err)
		}
		w.IsPrimary = isPrimary != 0
		w.WalletType = models.ParseWalletType(walletType)

		wallets = append(wallets, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating wallet rows: %w", err)
	}

	return wallets, nil
}

// GetPrimaryWallet retrieves the primary wallet.
func (d *Database) GetPrimaryWallet() (models.Wallet, error) {
	var w models.Wallet
	var isPrimary int
	var walletType string

	err := d.db.QueryRow(
		`SELECT address, label, is_primary, wallet_type, derivation_path, account_index
		 FROM wallets WHERE is_primary = 1`,
	).Scan(&w.Address, &w.Label, &isPrimary, &walletType, &w.DerivationPath, &w.AccountIndex)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Wallet{}, fmt.Errorf("getting primary wallet: %w", models.ErrWalletNotFound)
		}
		return models.Wallet{}, fmt.Errorf("getting primary wallet: %w", err)
	}

	w.IsPrimary = isPrimary != 0
	w.WalletType = models.ParseWalletType(walletType)

	return w, nil
}

// GetWalletByAddress retrieves a wallet by its address.
func (d *Database) GetWalletByAddress(addr string) (models.Wallet, error) {
	var w models.Wallet
	var isPrimary int
	var walletType string

	err := d.db.QueryRow(
		`SELECT address, label, is_primary, wallet_type, derivation_path, account_index
		 FROM wallets WHERE address = ?`, addr,
	).Scan(&w.Address, &w.Label, &isPrimary, &walletType, &w.DerivationPath, &w.AccountIndex)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Wallet{}, fmt.Errorf("getting wallet by address %q: %w", addr, models.ErrWalletNotFound)
		}
		return models.Wallet{}, fmt.Errorf("getting wallet by address %q: %w", addr, err)
	}

	w.IsPrimary = isPrimary != 0
	w.WalletType = models.ParseWalletType(walletType)

	return w, nil
}

// SetPrimaryWallet sets the given wallet as primary in a transaction.
// It first verifies the wallet exists, then unsets all primary flags, then sets the new one.
func (d *Database) SetPrimaryWallet(addr string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for set primary wallet: %w", err)
	}
	defer tx.Rollback()

	// Verify wallet exists
	var exists int
	err = tx.QueryRow(`SELECT COUNT(*) FROM wallets WHERE address = ?`, addr).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking wallet existence %q: %w", addr, err)
	}
	if exists == 0 {
		return fmt.Errorf("setting primary wallet %q: %w", addr, models.ErrWalletNotFound)
	}

	// Unset all primary flags
	_, err = tx.Exec(`UPDATE wallets SET is_primary = 0 WHERE is_primary = 1`)
	if err != nil {
		return fmt.Errorf("unsetting primary wallets: %w", err)
	}

	// Set new primary
	_, err = tx.Exec(`UPDATE wallets SET is_primary = 1 WHERE address = ?`, addr)
	if err != nil {
		return fmt.Errorf("setting primary wallet %q: %w", addr, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing set primary wallet: %w", err)
	}

	return nil
}

// DeleteWallet deletes a wallet and its associated encrypted keypair in a transaction.
func (d *Database) DeleteWallet(addr string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for delete wallet: %w", err)
	}
	defer tx.Rollback()

	// Delete encrypted keypair first (may not exist)
	_, err = tx.Exec(`DELETE FROM encrypted_keypairs WHERE address = ?`, addr)
	if err != nil {
		return fmt.Errorf("deleting keypair for wallet %q: %w", addr, err)
	}

	// Delete wallet
	result, err := tx.Exec(`DELETE FROM wallets WHERE address = ?`, addr)
	if err != nil {
		return fmt.Errorf("deleting wallet %q: %w", addr, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for wallet delete %q: %w", addr, err)
	}
	if rows == 0 {
		return fmt.Errorf("deleting wallet %q: %w", addr, models.ErrWalletNotFound)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing delete wallet: %w", err)
	}

	return nil
}

// WalletCount returns the total number of wallets.
func (d *Database) WalletCount() (int, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM wallets`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting wallets: %w", err)
	}
	return count, nil
}

// UpdateWalletLabel updates the label for a wallet.
func (d *Database) UpdateWalletLabel(address, label string) error {
	result, err := d.db.Exec(
		`UPDATE wallets SET label = ? WHERE address = ?`,
		label, address,
	)
	if err != nil {
		return fmt.Errorf("updating wallet label %q: %w", address, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for wallet label update %q: %w", address, err)
	}
	if rows == 0 {
		return fmt.Errorf("updating wallet label %q: %w", address, models.ErrWalletNotFound)
	}
	// Also update encrypted_keypairs label for consistency
	_, _ = d.db.Exec(`UPDATE encrypted_keypairs SET label = ? WHERE address = ?`, label, address)
	return nil
}
