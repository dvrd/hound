package wallet

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
)

// WalletManager coordinates wallet operations and portfolio caching.
type WalletManager struct {
	db             *database.Database
	balanceFetcher *BalanceFetcher
	portfolioCache map[string]models.PortfolioBalance
	mu             sync.RWMutex

	// preloadErrors stores per-wallet errors from the last background refresh.
	preloadErrors map[string]error
	preloadErrMu  sync.Mutex

	// refreshing tracks in-flight RefreshPortfolio calls per address to prevent
	// duplicate concurrent fetches. Guarded by refreshMu.
	refreshing map[string]bool
	refreshMu  sync.Mutex
}

// NewWalletManager creates a new WalletManager.
func NewWalletManager(db *database.Database, balanceFetcher *BalanceFetcher) *WalletManager {
	return &WalletManager{
		db:             db,
		balanceFetcher: balanceFetcher,
		portfolioCache: make(map[string]models.PortfolioBalance),
		refreshing:     make(map[string]bool),
		preloadErrors:  make(map[string]error),
	}
}

// GetWallets returns all wallets from the database.
func (m *WalletManager) GetWallets() ([]models.Wallet, error) {
	return m.db.GetAllWallets()
}

// GetPrimaryWallet returns the primary wallet.
func (m *WalletManager) GetPrimaryWallet() (models.Wallet, error) {
	return m.db.GetPrimaryWallet()
}

// GetPortfolio returns the portfolio for a wallet address, handling cache,
// live fetch, dedup, metadata resolution, pricing, and preload errors
// behind a single seam. Callers no longer need to know whether data comes
// from cache or a live RPC call.
//
// Priority: cached data → live refresh if no cache → fallback to DB balances.
func (m *WalletManager) GetPortfolio(ctx context.Context, address string) (models.PortfolioBalance, error) {
	// 1. Surface preload errors early so callers see them instead of a generic cache-miss.
	if preloadErr := m.PreloadError(address); preloadErr != nil {
		return models.PortfolioBalance{}, preloadErr
	}

	// 2. Try cache first (populated by preload or prior refresh).
	cached, err := m.GetCachedPortfolio(address)
	if err == nil && cached.WalletAddress != "" {
		return cached, nil
	}

	// 3. No useful cache — fetch live.
	return m.RefreshPortfolio(ctx, address)
}

// RefreshPortfolio fetches live balances and caches the result.
// If a refresh for the same address is already in-flight, it returns the cached
// portfolio immediately (or waits for the in-flight call to complete via the cache).
func (m *WalletManager) RefreshPortfolio(ctx context.Context, address string) (models.PortfolioBalance, error) {
	if m.balanceFetcher == nil {
		return models.PortfolioBalance{}, fmt.Errorf("balance fetcher not configured")
	}

	// Deduplication: skip if a refresh is already in-flight for this address.
	m.refreshMu.Lock()
	if m.refreshing[address] {
		m.refreshMu.Unlock()
		// Return whatever is cached; the in-flight goroutine will update it.
		return m.GetCachedPortfolio(address)
	}
	m.refreshing[address] = true
	m.refreshMu.Unlock()

	defer func() {
		m.refreshMu.Lock()
		delete(m.refreshing, address)
		m.refreshMu.Unlock()
	}()

	portfolio, err := m.balanceFetcher.FetchPortfolioBalance(address)
	if err != nil {
		return models.PortfolioBalance{}, fmt.Errorf("refreshing portfolio for %s: %w", address, err)
	}

	m.mu.Lock()
	m.portfolioCache[address] = portfolio
	m.mu.Unlock()

	return portfolio, nil
}

// RefreshAllPortfolios refreshes all wallets (best-effort: continues on failure).
func (m *WalletManager) RefreshAllPortfolios(ctx context.Context) (map[string]models.PortfolioBalance, error) {
	wallets, err := m.db.GetAllWallets()
	if err != nil {
		return nil, fmt.Errorf("listing wallets: %w", err)
	}

	results := make(map[string]models.PortfolioBalance, len(wallets))
	var lastErr error

	for _, w := range wallets {
		if ctx.Err() != nil {
			break // context cancelled — stop early
		}
		portfolio, err := m.RefreshPortfolio(ctx, w.Address)
		if err != nil {
			lastErr = err
			m.preloadErrMu.Lock()
			m.preloadErrors[w.Address] = err
			m.preloadErrMu.Unlock()
			continue
		}
		m.preloadErrMu.Lock()
		delete(m.preloadErrors, w.Address)
		m.preloadErrMu.Unlock()
		results[w.Address] = portfolio
	}

	if len(results) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all portfolio refreshes failed: %w", lastErr)
	}

	return results, nil
}

// PreloadError returns the last preload error for a wallet address, or nil if none.
func (m *WalletManager) PreloadError(walletAddr string) error {
	m.preloadErrMu.Lock()
	defer m.preloadErrMu.Unlock()
	return m.preloadErrors[walletAddr]
}

// GetCachedPortfolio returns the cached portfolio, or fetches from DB balances.
func (m *WalletManager) GetCachedPortfolio(address string) (models.PortfolioBalance, error) {
	// Check cache first
	m.mu.RLock()
	cached, ok := m.portfolioCache[address]
	m.mu.RUnlock()

	if ok {
		return cached, nil
	}

	// Fall back to DB balances
	balances, err := m.db.GetBalancesForWallet(address)
	if err != nil {
		return models.PortfolioBalance{}, fmt.Errorf("fetching cached balances for %s: %w", address, err)
	}

	portfolio := models.PortfolioBalance{
		WalletAddress: address,
	}

	var totalUSD float64
	for _, b := range balances {
		if b.Mint == "So11111111111111111111111111111111111111112" {
			portfolio.SOLBalance = b
		} else {
			portfolio.TokenBalances = append(portfolio.TokenBalances, b)
		}
		totalUSD += b.USDValue
	}
	portfolio.TotalUSD = totalUSD

	return portfolio, nil
}

// AggregatePortfolios sums balances across multiple portfolios.
func (m *WalletManager) AggregatePortfolios(portfolios map[string]models.PortfolioBalance) models.PortfolioBalance {
	agg := models.PortfolioBalance{
		WalletAddress: "AGGREGATED",
		SOLBalance: models.TokenBalance{
			Mint:   "So11111111111111111111111111111111111111112",
			Symbol: "SOL",
		},
	}

	// Merge token balances by mint
	tokenMap := make(map[string]*models.TokenBalance)

	for _, p := range portfolios {
		// Sum SOL
		agg.SOLBalance.Amount += p.SOLBalance.Amount
		agg.SOLBalance.USDValue += p.SOLBalance.USDValue
		if p.SOLBalance.USDPrice > 0 {
			agg.SOLBalance.USDPrice = p.SOLBalance.USDPrice
		}
		agg.SOLBalance.Decimals = 9

		// Merge tokens
		for _, tb := range p.TokenBalances {
			existing, ok := tokenMap[tb.Mint]
			if ok {
				existing.Amount += tb.Amount
				existing.USDValue += tb.USDValue
				if tb.USDPrice > 0 {
					existing.USDPrice = tb.USDPrice
				}
				if tb.Change24h != 0 {
					existing.Change24h = tb.Change24h
				}
			} else {
				copy := tb
				tokenMap[tb.Mint] = &copy
			}
		}
	}

	// Collect token balances
	for _, tb := range tokenMap {
		agg.TokenBalances = append(agg.TokenBalances, *tb)
	}

	// Calculate total USD
	agg.TotalUSD = agg.SOLBalance.USDValue
	for _, tb := range agg.TokenBalances {
		agg.TotalUSD += tb.USDValue
	}

	return agg
}

// ResolveWallet resolves a wallet by full address, partial address (first 8 chars), or label.
func (m *WalletManager) ResolveWallet(identifier string) (models.Wallet, error) {
	// 1. Empty identifier → return primary wallet
	if identifier == "" {
		return m.db.GetPrimaryWallet()
	}

	// 2. Try exact address match
	w, err := m.db.GetWalletByAddress(identifier)
	if err == nil {
		return w, nil
	}

	// 3. Try partial address match and label match against all wallets
	wallets, err := m.db.GetAllWallets()
	if err != nil {
		return models.Wallet{}, fmt.Errorf("listing wallets for resolve: %w", err)
	}

	// Partial address match (8+ chars prefix)
	if len(identifier) >= 8 {
		for _, w := range wallets {
			if strings.HasPrefix(w.Address, identifier) {
				return w, nil
			}
		}
	}

	// 4. Case-insensitive label match
	lowerID := strings.ToLower(identifier)
	for _, w := range wallets {
		if strings.ToLower(w.Label) == lowerID {
			return w, nil
		}
	}

	// 5. Not found
	return models.Wallet{}, &models.WalletNotFoundError{Identifier: identifier}
}

// PersistPortfolio saves portfolio balances to the database atomically.
// All balance writes happen in a single transaction — all-or-nothing.
func (m *WalletManager) PersistPortfolio(portfolio models.PortfolioBalance) error {
	tx, err := m.db.BeginTx()
	if err != nil {
		return fmt.Errorf("persisting portfolio: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit; guards against early returns

	// Save SOL balance
	sol := portfolio.SOLBalance
	if err := m.db.UpdateBalanceTx(tx, portfolio.WalletAddress, sol.Mint, sol.Symbol, sol.Name,
		sol.Amount, sol.USDPrice, sol.USDValue,
	); err != nil {
		return fmt.Errorf("persisting SOL balance: %w", err)
	}

	// Save token balances
	for _, tb := range portfolio.TokenBalances {
		if err := m.db.UpdateBalanceTx(tx, portfolio.WalletAddress, tb.Mint, tb.Symbol, tb.Name,
			tb.Amount, tb.USDPrice, tb.USDValue,
		); err != nil {
			return fmt.Errorf("persisting balance for %s: %w", tb.Symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("persisting portfolio: commit: %w", err)
	}

	return nil
}
