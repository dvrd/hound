package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// UpdateBalance inserts or replaces a token balance for a wallet.
func (d *Database) UpdateBalance(walletAddr, mint, symbol, name string, amount, usdPrice, usdValue float64) error {
	now := time.Now().Unix()

	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO balances (wallet_address, mint, symbol, name, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		walletAddr, mint, symbol, name, amount, usdPrice, usdValue, now,
	)
	if err != nil {
		return fmt.Errorf("updating balance for wallet %q mint %q: %w", walletAddr, mint, err)
	}
	return nil
}

// UpdateBalanceTx inserts or replaces a token balance within an existing transaction.
// Use this with BeginTx() for atomic batch writes.
func (d *Database) UpdateBalanceTx(tx *sql.Tx, walletAddr, mint, symbol, name string, amount, usdPrice, usdValue float64) error {
	now := time.Now().Unix()

	_, err := tx.Exec(
		`INSERT OR REPLACE INTO balances (wallet_address, mint, symbol, name, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		walletAddr, mint, symbol, name, amount, usdPrice, usdValue, now,
	)
	if err != nil {
		return fmt.Errorf("updating balance (tx) for wallet %q mint %q: %w", walletAddr, mint, err)
	}
	return nil
}

// UpdateBalancesBatch inserts or replaces multiple balances using a prepared statement
// within a transaction for better performance (avoids re-parsing SQL per row).
func (d *Database) UpdateBalancesBatch(tx *sql.Tx, walletAddr string, balances []models.TokenBalance) error {
	now := time.Now().Unix()
	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO balances (wallet_address, mint, symbol, name, amount, usd_price, usd_value, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing balance batch: %w", err)
	}
	defer stmt.Close()

	for _, b := range balances {
		_, err := stmt.Exec(walletAddr, b.Mint, b.Symbol, b.Name, b.Amount, b.USDPrice, b.USDValue, now)
		if err != nil {
			return fmt.Errorf("batch insert balance %q: %w", b.Symbol, err)
		}
	}
	return nil
}

// GetBalancesForWallet retrieves all token balances for a wallet, ordered by USD value descending.
func (d *Database) GetBalancesForWallet(walletAddr string) ([]models.TokenBalance, error) {
	rows, err := d.db.Query(
		`SELECT mint, symbol, COALESCE(name, ''), amount, usd_price, usd_value
		 FROM balances WHERE wallet_address = ? ORDER BY usd_value DESC`, walletAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("querying balances for wallet %q: %w", walletAddr, err)
	}
	defer rows.Close()

	var balances []models.TokenBalance
	for rows.Next() {
		var b models.TokenBalance
		if err := rows.Scan(&b.Mint, &b.Symbol, &b.Name, &b.Amount, &b.USDPrice, &b.USDValue); err != nil {
			return nil, fmt.Errorf("scanning balance row: %w", err)
		}
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating balance rows: %w", err)
	}

	return balances, nil
}
