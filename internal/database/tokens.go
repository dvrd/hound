package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/models"
)

// InsertToken inserts a new token into the database.
func (d *Database) InsertToken(token models.Token) error {
	now := time.Now().Unix()
	isQuote := 0
	if token.IsQuoteToken {
		isQuote = 1
	}

	_, err := d.db.Exec(
		`INSERT INTO tokens (symbol, name, contract_address, chain, is_quote_token, usd_price, discovered_at, last_updated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		token.Symbol, token.Name, token.ContractAddress, token.Chain,
		isQuote, token.USDPrice, now, now,
	)
	if err != nil {
		return fmt.Errorf("inserting token %q: %w", token.Symbol, err)
	}
	return nil
}

// GetTokenBySymbol retrieves a token by its symbol (case-insensitive) and loads its pools.
func (d *Database) GetTokenBySymbol(symbol string) (models.Token, error) {
	var token models.Token
	var id int64
	var isQuote int

	err := d.db.QueryRow(
		`SELECT id, symbol, name, contract_address, chain, is_quote_token, usd_price
		 FROM tokens WHERE symbol = ? COLLATE NOCASE`, symbol,
	).Scan(&id, &token.Symbol, &token.Name, &token.ContractAddress, &token.Chain, &isQuote, &token.USDPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Token{}, fmt.Errorf("getting token by symbol %q: %w", symbol, models.ErrTokenNotFound)
		}
		return models.Token{}, fmt.Errorf("getting token by symbol %q: %w", symbol, err)
	}

	token.IsQuoteToken = isQuote != 0

	pools, err := d.GetPoolsForToken(id)
	if err != nil {
		return models.Token{}, fmt.Errorf("loading pools for token %q: %w", symbol, err)
	}
	token.Pools = pools

	return token, nil
}

// GetTokenByContractAddress retrieves a token by its contract address.
func (d *Database) GetTokenByContractAddress(addr string) (models.Token, error) {
	var token models.Token
	var id int64
	var isQuote int

	err := d.db.QueryRow(
		`SELECT id, symbol, name, contract_address, chain, is_quote_token, usd_price
		 FROM tokens WHERE contract_address = ?`, addr,
	).Scan(&id, &token.Symbol, &token.Name, &token.ContractAddress, &token.Chain, &isQuote, &token.USDPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Token{}, fmt.Errorf("getting token by contract address %q: %w", addr, models.ErrTokenNotFound)
		}
		return models.Token{}, fmt.Errorf("getting token by contract address %q: %w", addr, err)
	}

	token.IsQuoteToken = isQuote != 0

	pools, err := d.GetPoolsForToken(id)
	if err != nil {
		return models.Token{}, fmt.Errorf("loading pools for token at %q: %w", addr, err)
	}
	token.Pools = pools

	return token, nil
}

// GetTokenMapByContract returns all tokens indexed by contract_address.
// Does NOT load pools — optimized for batch lookup during portfolio assembly.
func (d *Database) GetTokenMapByContract() (map[string]models.Token, error) {
	rows, err := d.db.Query(
		`SELECT symbol, name, contract_address, chain, is_quote_token, usd_price FROM tokens`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying token map: %w", err)
	}
	defer rows.Close()

	result := make(map[string]models.Token)
	for rows.Next() {
		var t models.Token
		var isQuote int
		if err := rows.Scan(&t.Symbol, &t.Name, &t.ContractAddress, &t.Chain, &isQuote, &t.USDPrice); err != nil {
			return nil, fmt.Errorf("scanning token map row: %w", err)
		}
		t.IsQuoteToken = isQuote != 0
		result[t.ContractAddress] = t
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating token map rows: %w", err)
	}
	return result, nil
}

// GetAllTokens retrieves all tokens ordered by symbol, with pools loaded.
// Tokens and their IDs are collected first, then pools are loaded in a
// separate pass to avoid nested queries on the same connection.
func (d *Database) GetAllTokens() ([]models.Token, error) {
	rows, err := d.db.Query(
		`SELECT id, symbol, name, contract_address, chain, is_quote_token, usd_price
		 FROM tokens ORDER BY symbol`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying all tokens: %w", err)
	}
	defer rows.Close()

	type tokenWithID struct {
		token models.Token
		id    int64
	}

	var items []tokenWithID
	for rows.Next() {
		var tw tokenWithID
		var isQuote int

		if err := rows.Scan(&tw.id, &tw.token.Symbol, &tw.token.Name, &tw.token.ContractAddress, &tw.token.Chain, &isQuote, &tw.token.USDPrice); err != nil {
			return nil, fmt.Errorf("scanning token row: %w", err)
		}
		tw.token.IsQuoteToken = isQuote != 0
		items = append(items, tw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating token rows: %w", err)
	}

	// Batch-load all pools in a single query (eliminates N+1).
	allPools, err := d.getAllPoolsByTokenID()
	if err != nil {
		return nil, fmt.Errorf("loading all pools: %w", err)
	}

	tokens := make([]models.Token, 0, len(items))
	for _, tw := range items {
		tw.token.Pools = allPools[tw.id]
		tokens = append(tokens, tw.token)
	}

	return tokens, nil
}

