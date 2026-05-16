package services

import (
	"fmt"
	"time"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
)

// PoolService discovers and stores liquidity pools for tokens.
type PoolService struct {
	dexscreener *dex.DexScreenerClient
	db          *database.Database
}

// NewPoolService creates a new PoolService.
func NewPoolService(dexscreener *dex.DexScreenerClient, db *database.Database) *PoolService {
	return &PoolService{
		dexscreener: dexscreener,
		db:          db,
	}
}

// DiscoverAndStorePools fetches pools from DexScreener, filters and ranks them,
// and stores the top 3 in the database. Returns the best pool.
// If forceRefresh is true, existing pools are deleted before storing new ones.
func (s *PoolService) DiscoverAndStorePools(token models.Token, forceRefresh bool) (models.PoolInfo, error) {
	// Fetch from DexScreener
	pairs, err := s.dexscreener.FetchPoolsForToken(token.ContractAddress)
	if err != nil {
		return models.PoolInfo{}, fmt.Errorf("discover pools for %s: %w", token.Symbol, err)
	}

	// Filter pools
	filtered := dex.FilterPools(pairs)
	if len(filtered) == 0 {
		return models.PoolInfo{}, fmt.Errorf("discover pools for %s: %w", token.Symbol, models.ErrNoPoolsFound)
	}

	// Rank pools
	ranked := dex.RankPools(filtered)

	// If forceRefresh, delete old pools first
	if forceRefresh {
		// Best-effort: old pools are stale, continue even if delete fails.
		_ = s.db.DeletePoolsForToken(token.Symbol)
	}

	// Store top 3 (or fewer if less available)
	limit := dex.TopPoolsToStore
	if len(ranked) < limit {
		limit = len(ranked)
	}

	now := time.Now().Unix()
	for i := 0; i < limit; i++ {
		pool := dex.PairToPoolInfo(ranked[i])
		pool.DiscoveredAt = now
		// Best-effort: continue storing remaining pools on individual failure.
		_ = s.db.InsertPool(token.Symbol, pool)
	}

	// Return best pool (first ranked)
	best := dex.PairToPoolInfo(ranked[0])
	best.DiscoveredAt = now
	return best, nil
}
