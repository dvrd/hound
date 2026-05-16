package services

import (
	"fmt"
	"sync"
	"time"

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

	// Cached saved-address set to avoid querying DB on every keystroke.
	savedAddrs    map[string]bool
	savedAddrsMu  sync.RWMutex
	savedAddrsAt  time.Time
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

	// Use cached saved-address set (refreshed every 30s).
	savedAddrs := c.getSavedAddrs()

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

const savedAddrsCacheTTL = 30 * time.Second

// getSavedAddrs returns the cached set of saved contract addresses,
// refreshing from DB at most every 30 seconds.
func (c *TokenCatalog) getSavedAddrs() map[string]bool {
	c.savedAddrsMu.RLock()
	if c.savedAddrs != nil && time.Since(c.savedAddrsAt) < savedAddrsCacheTTL {
		defer c.savedAddrsMu.RUnlock()
		return c.savedAddrs
	}
	c.savedAddrsMu.RUnlock()

	// Refresh.
	c.savedAddrsMu.Lock()
	defer c.savedAddrsMu.Unlock()

	// Double-check after acquiring write lock.
	if c.savedAddrs != nil && time.Since(c.savedAddrsAt) < savedAddrsCacheTTL {
		return c.savedAddrs
	}

	addrs := make(map[string]bool)
	if c.db != nil {
		if tokenMap, err := c.db.GetTokenMapByContract(); err == nil {
			for addr := range tokenMap {
				addrs[addr] = true
			}
		}
	}
	c.savedAddrs = addrs
	c.savedAddrsAt = time.Now()
	return addrs
}
