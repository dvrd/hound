package database

import (
	"database/sql"
	"fmt"

	"github.com/dvrd/hound/internal/models"
)

// InsertPool inserts a pool for a token identified by symbol.
// It first looks up the token_id, then inserts or replaces the pool row.
func (d *Database) InsertPool(tokenSymbol string, pool models.PoolInfo) error {
	var tokenID int64
	err := d.db.QueryRow(
		`SELECT id FROM tokens WHERE symbol = ? COLLATE NOCASE`, tokenSymbol,
	).Scan(&tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("inserting pool for token %q: %w", tokenSymbol, models.ErrTokenNotFound)
		}
		return fmt.Errorf("looking up token %q for pool insert: %w", tokenSymbol, err)
	}

	_, err = d.db.Exec(
		`INSERT OR REPLACE INTO pools (token_id, dex, pool_address, quote_token, pool_type, liquidity_usd, volume_24h, fee_percent, discovered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tokenID, pool.Dex, pool.PoolAddress, pool.QuoteToken, pool.PoolType,
		pool.LiquidityUSD, pool.Volume24h, pool.FeePercent, pool.DiscoveredAt,
	)
	if err != nil {
		return fmt.Errorf("inserting pool %q for token %q: %w", pool.PoolAddress, tokenSymbol, err)
	}
	return nil
}

// GetPoolsForToken retrieves all pools for a given token ID.
func (d *Database) GetPoolsForToken(tokenID int64) ([]models.PoolInfo, error) {
	rows, err := d.db.Query(
		`SELECT dex, pool_address, quote_token, pool_type, liquidity_usd, volume_24h, fee_percent, discovered_at
		 FROM pools WHERE token_id = ?`, tokenID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying pools for token_id %d: %w", tokenID, err)
	}
	defer rows.Close()

	var pools []models.PoolInfo
	for rows.Next() {
		var p models.PoolInfo
		if err := rows.Scan(&p.Dex, &p.PoolAddress, &p.QuoteToken, &p.PoolType,
			&p.LiquidityUSD, &p.Volume24h, &p.FeePercent, &p.DiscoveredAt); err != nil {
			return nil, fmt.Errorf("scanning pool row: %w", err)
		}
		pools = append(pools, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pool rows: %w", err)
	}

	return pools, nil
}

// DeletePoolsForToken deletes all pools for a token identified by symbol.
func (d *Database) DeletePoolsForToken(tokenSymbol string) error {
	var tokenID int64
	err := d.db.QueryRow(
		`SELECT id FROM tokens WHERE symbol = ? COLLATE NOCASE`, tokenSymbol,
	).Scan(&tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("deleting pools for token %q: %w", tokenSymbol, models.ErrTokenNotFound)
		}
		return fmt.Errorf("looking up token %q for pool delete: %w", tokenSymbol, err)
	}

	_, err = d.db.Exec(`DELETE FROM pools WHERE token_id = ?`, tokenID)
	if err != nil {
		return fmt.Errorf("deleting pools for token %q: %w", tokenSymbol, err)
	}
	return nil
}

// GetPoolStats returns aggregate pool statistics for a token identified by symbol.
func (d *Database) GetPoolStats(tokenSymbol string) (models.PoolStats, error) {
	var tokenID int64
	err := d.db.QueryRow(
		`SELECT id FROM tokens WHERE symbol = ? COLLATE NOCASE`, tokenSymbol,
	).Scan(&tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.PoolStats{}, fmt.Errorf("getting pool stats for token %q: %w", tokenSymbol, models.ErrTokenNotFound)
		}
		return models.PoolStats{}, fmt.Errorf("looking up token %q for pool stats: %w", tokenSymbol, err)
	}

	var stats models.PoolStats
	var totalLiq sql.NullFloat64
	err = d.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(liquidity_usd), 0) FROM pools WHERE token_id = ?`, tokenID,
	).Scan(&stats.PoolCount, &totalLiq)
	if err != nil {
		return models.PoolStats{}, fmt.Errorf("querying pool stats for token %q: %w", tokenSymbol, err)
	}
	if totalLiq.Valid {
		stats.TotalLiquidity = totalLiq.Float64
	}

	return stats, nil
}
