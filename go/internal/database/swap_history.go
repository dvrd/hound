package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// InsertSwapHistory inserts a new swap history entry.
func (d *Database) InsertSwapHistory(entry models.SwapHistoryEntry) error {
	now := time.Now().Unix()
	if entry.CreatedAt != 0 {
		now = entry.CreatedAt
	}

	_, err := d.db.Exec(
		`INSERT INTO swap_history (wallet_address, input_mint, output_mint, input_symbol, output_symbol,
		 input_amount, output_amount, price_impact, slippage_bps, signature, status, dex, network_fee, priority_fee, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.WalletAddress, entry.InputMint, entry.OutputMint,
		entry.InputSymbol, entry.OutputSymbol,
		entry.InputAmount, entry.OutputAmount,
		entry.PriceImpact, entry.SlippageBps,
		entry.Signature, entry.Status, entry.Dex,
		entry.NetworkFee, entry.PriorityFee, entry.ErrorMessage,
		now,
	)
	if err != nil {
		return fmt.Errorf("inserting swap history: %w", err)
	}
	return nil
}

// GetSwapHistory retrieves swap history entries, optionally filtered by wallet address.
// If walletAddr is empty, all entries are returned. Results are ordered by created_at DESC.
func (d *Database) GetSwapHistory(walletAddr string, limit int) ([]models.SwapHistoryEntry, error) {
	var query string
	var args []any

	if walletAddr == "" {
		query = `SELECT id, wallet_address, input_mint, output_mint, input_symbol, output_symbol,
				 input_amount, output_amount, price_impact, slippage_bps, signature, status, dex,
				 network_fee, priority_fee, error_message, created_at
				 FROM swap_history ORDER BY created_at DESC LIMIT ?`
		args = []any{limit}
	} else {
		query = `SELECT id, wallet_address, input_mint, output_mint, input_symbol, output_symbol,
				 input_amount, output_amount, price_impact, slippage_bps, signature, status, dex,
				 network_fee, priority_fee, error_message, created_at
				 FROM swap_history WHERE wallet_address = ? ORDER BY created_at DESC LIMIT ?`
		args = []any{walletAddr, limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying swap history: %w", err)
	}
	defer rows.Close()

	var entries []models.SwapHistoryEntry
	for rows.Next() {
		var e models.SwapHistoryEntry
		var sig, dex, errMsg sql.NullString

		if err := rows.Scan(&e.ID, &e.WalletAddress, &e.InputMint, &e.OutputMint,
			&e.InputSymbol, &e.OutputSymbol, &e.InputAmount, &e.OutputAmount,
			&e.PriceImpact, &e.SlippageBps, &sig, &e.Status, &dex,
			&e.NetworkFee, &e.PriorityFee, &errMsg, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning swap history row: %w", err)
		}
		if sig.Valid {
			e.Signature = sig.String
		}
		if dex.Valid {
			e.Dex = dex.String
		}
		if errMsg.Valid {
			e.ErrorMessage = errMsg.String
		}

		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating swap history rows: %w", err)
	}

	return entries, nil
}

// GetSwapHistoryCount returns the number of swap history entries for a wallet.
// If walletAddr is empty, returns the total count.
func (d *Database) GetSwapHistoryCount(walletAddr string) (int, error) {
	var count int
	var err error

	if walletAddr == "" {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM swap_history`).Scan(&count)
	} else {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM swap_history WHERE wallet_address = ?`, walletAddr).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("counting swap history: %w", err)
	}

	return count, nil
}
