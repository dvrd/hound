package database

import (
	"fmt"
	"time"
)

// HideToken marks a token mint as hidden for the given wallet.
// Safe to call multiple times — uses INSERT OR IGNORE.
func (d *Database) HideToken(walletAddr, mint string) error {
	now := time.Now().Unix()
	_, err := d.db.Exec(
		`INSERT OR IGNORE INTO hidden_tokens (wallet_address, mint, hidden_at) VALUES (?, ?, ?)`,
		walletAddr, mint, now,
	)
	if err != nil {
		return fmt.Errorf("hiding token %q for wallet %q: %w", mint, walletAddr, err)
	}
	return nil
}

// UnhideToken removes a token from the hidden list for the given wallet.
func (d *Database) UnhideToken(walletAddr, mint string) error {
	_, err := d.db.Exec(
		`DELETE FROM hidden_tokens WHERE wallet_address = ? AND mint = ?`,
		walletAddr, mint,
	)
	if err != nil {
		return fmt.Errorf("unhiding token %q for wallet %q: %w", mint, walletAddr, err)
	}
	return nil
}

// GetHiddenMints returns the set of hidden mint addresses for the given wallet.
func (d *Database) GetHiddenMints(walletAddr string) (map[string]bool, error) {
	rows, err := d.db.Query(
		`SELECT mint FROM hidden_tokens WHERE wallet_address = ?`,
		walletAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("querying hidden tokens for wallet %q: %w", walletAddr, err)
	}
	defer rows.Close()

	hidden := make(map[string]bool)
	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return nil, fmt.Errorf("scanning hidden token row: %w", err)
		}
		hidden[mint] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hidden token rows: %w", err)
	}
	return hidden, nil
}
