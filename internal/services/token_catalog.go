package services

import (
	"fmt"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

// TokenCatalog consolidates token search, metadata resolution, and saved-token
// detection into one deepened module. Callers no longer need to know which
// external source provides token data or whether a token is saved locally.
type TokenCatalog struct {
	jupiter     *dex.JupiterClient
	dexscreener *dex.DexScreenerClient
	db          *database.Database
}

// TokenResult is a unified token result returned by the catalog.
// It replaces both dex.TokenMetadata and wallet.TokenMetadata — a single
// representation used everywhere a token identity is resolved.
type TokenResult struct {
	Symbol   string
	Name     string
	Address  string
	Decimals int
	Saved    bool // true if the token is in the local database
}

// NewTokenCatalog creates a new TokenCatalog.
func NewTokenCatalog(jupiter *dex.JupiterClient, dexscreener *dex.DexScreenerClient, db *database.Database) *TokenCatalog {
	return &TokenCatalog{
		jupiter:     jupiter,
		dexscreener: dexscreener,
		db:          db,
	}
}

// Search finds tokens matching the query (symbol, name, or address).
// Returns results from Jupiter enriched with saved-token status.
func (c *TokenCatalog) Search(query string) ([]TokenResult, error) {
	if c.jupiter == nil {
		return nil, fmt.Errorf("token catalog: Jupiter client not available")
	}
	raw, err := c.jupiter.LookupTokenList(query)
	if err != nil {
		return nil, fmt.Errorf("token catalog: search %q: %w", query, err)
	}

	// Build saved-address set from DB.
	savedAddrs := make(map[string]bool)
	if c.db != nil {
		tokens, err := c.db.GetAllTokens()
		if err == nil {
			for _, t := range tokens {
				savedAddrs[t.ContractAddress] = true
			}
		}
	}

	results := make([]TokenResult, len(raw))
	for i, r := range raw {
		results[i] = TokenResult{
			Symbol:   r.Symbol,
			Name:     r.Name,
			Address:  r.Address,
			Decimals: r.Decimals,
			Saved:    savedAddrs[r.Address],
		}
	}
	return results, nil
}

// LookupMetadata resolves token metadata (symbol, name, decimals) by mint address.
// Tries DB first, then falls back to Jupiter. Used during portfolio assembly to
// identify unknown tokens discovered on-chain.
func (c *TokenCatalog) LookupMetadata(mintAddr string) (TokenResult, error) {
	// Try DB first.
	if c.db != nil {
		token, err := c.db.GetTokenByContractAddress(mintAddr)
		if err == nil {
			return TokenResult{
				Symbol:   token.Symbol,
				Name:     token.Name,
				Address:  token.ContractAddress,
				Decimals: models.GetTokenDecimals(token),
				Saved:    true,
			}, nil
		}
	}

	// Fall back to Jupiter.
	if c.jupiter != nil {
		meta, err := c.jupiter.LookupTokenMetadata(mintAddr)
		if err == nil {
			result := TokenResult{
				Symbol:   meta.Symbol,
				Name:     meta.Name,
				Address:  meta.Address,
				Decimals: meta.Decimals,
				Saved:    false,
			}
			// Cache in DB so subsequent fetches skip the network call.
			// Guard: only cache if Jupiter returned valid-looking data.
			if c.db != nil && result.Symbol != "" && result.Address != "" {
				_ = c.db.InsertToken(models.Token{
					Symbol:          result.Symbol,
					Name:            result.Name,
					ContractAddress: result.Address,
					Chain:           "solana",
					Decimals:        result.Decimals,
				})
			}
			return result, nil
		}
	}

	return TokenResult{}, fmt.Errorf("token catalog: metadata for %s: %w", mintAddr, models.ErrTokenNotFound)
}
