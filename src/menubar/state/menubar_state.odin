// MenuBar state management - replaces global variables with structured state
// Centralizes all application state for clean dependency injection
package state

import "core:time"
import models "../../lib/models"
import wallet "../../lib/wallet"
import db "../../lib/database"

// ============================================================================
// Types
// ============================================================================

// Application display mode
DisplayMode :: enum {
	Price,   // Single token price display
	Wallet,  // Full portfolio display
}

// Centralized application state (replaces 9 globals from app.odin)
MenuBarState :: struct {
	// Dependencies
	db:              ^db.Database,
	rpc_client:      wallet.RPCClient,
	balance_fetcher: wallet.BalanceFetcher,
	token_config:    models.TokenConfig,

	// UI handles (opaque pointers to avoid cyclic import)
	// Cast to actual AppKit types in menubar package
	status_item: rawptr,  // ^NSStatusItem
	menu:        rawptr,  // ^NSMenu
	timer:       rawptr,  // ^NSTimer

	// Display state
	mode:              DisplayMode,
	current_symbol:    string,
	current_portfolio: wallet.PortfolioBalance,

	// Configuration
	refresh_interval: f64,  // Seconds (default: 5.0)
	slippage_bps:     int,  // Basis points (default: 50 = 0.5%)
}

// ============================================================================
// Initialization
// ============================================================================

// create_state creates initial MenuBarState with services initialized
//
// ASSERTION 1: Database must not be nil
// ASSERTION 2: Token config must have at least one token
//
// Returns: Initialized MenuBarState
create_state :: proc(
	database: ^db.Database,
	rpc_client: wallet.RPCClient,
	balance_fetcher: wallet.BalanceFetcher,
	config: models.TokenConfig,
) -> MenuBarState {
	assert(database != nil, "Database cannot be nil")
	assert(len(config.tokens) > 0, "Token config must have at least one token")

	return MenuBarState{
		db = database,
		rpc_client = rpc_client,
		balance_fetcher = balance_fetcher,
		token_config = config,
		mode = .Price,
		current_symbol = "sol",
		refresh_interval = 5.0,
		slippage_bps = 50,
	}
}
