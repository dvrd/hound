// Wallet view - pure UI layer for portfolio display mode
// No business logic, no service calls, only AppKit UI creation
package views

import "core:fmt"
import "core:log"
import "core:strings"
import wallet "../../lib/wallet"
import state "../state"
import formatters "../formatters"
import appkit "../../appkit"

// ============================================================================
// Menu Creation
// ============================================================================

// create_wallet_menu creates the portfolio display menu
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Portfolio total must be non-negative
//
// Returns: Newly created NSMenu with portfolio display items
create_wallet_menu :: proc(
	st: ^state.MenuBarState,
	portfolio: wallet.PortfolioBalance,
) -> ^appkit.NSMenu {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(portfolio.total_usd >= 0, "Portfolio total must be non-negative")

	log.debugf("[wallet_view] Creating wallet menu: $%.2f total", portfolio.total_usd)

	menu := appkit.NSMenu_new()

	// Total USD value item
	total_text := formatters.format_portfolio_total(portfolio.total_usd)
	total_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString(total_text),
		nil,
		nil,
	)
	appkit.NSMenu_addItem(menu, total_item)

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// SOL balance
	sol_text := formatters.format_sol_balance(portfolio.sol_balance)
	sol_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString(sol_text),
		nil,
		nil,
	)
	appkit.NSMenu_addItem(menu, sol_item)

	// Token balances (dynamic)
	for token in portfolio.token_balances {
		token_text := formatters.format_token_balance(token)
		token_item := appkit.NSMenuItem_new(
			appkit.NSString_fromString(token_text),
			nil,
			nil,
		)
		appkit.NSMenu_addItem(menu, token_item)
	}

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// Refresh button
	refresh_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Refresh Portfolio"),
		appkit.selector("refreshPortfolio:"),
		appkit.NSString_fromString("r"),
	)
	appkit.NSMenu_addItem(menu, refresh_item)

	// Switch to Price Mode button
	switch_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Switch to Price Mode"),
		appkit.selector("switchToPriceMode:"),
		appkit.NSString_fromString("p"),
	)
	appkit.NSMenu_addItem(menu, switch_item)

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// Swap Tokens button (optional - Phase 2 feature)
	swap_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Swap Tokens..."),
		appkit.selector("showSwapDialog:"),
		appkit.NSString_fromString("s"),
	)
	appkit.NSMenu_addItem(menu, swap_item)

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// Quit button
	quit_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Quit"),
		appkit.selector("quitApp:"),
		appkit.NSString_fromString("q"),
	)
	appkit.NSMenu_addItem(menu, quit_item)

	log.debugf("[wallet_view] Wallet menu created with %d tokens", len(portfolio.token_balances))
	return menu
}

// ============================================================================
// Display Update
// ============================================================================

// update_wallet_display updates the status bar button with portfolio data
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Portfolio total must be non-negative
//
// Side effects: Updates status bar button title with portfolio total
update_wallet_display :: proc(
	st: ^state.MenuBarState,
	portfolio: wallet.PortfolioBalance,
) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(portfolio.total_usd >= 0, "Portfolio total must be non-negative")

	log.debugf("[wallet_view] Updating wallet display: $%.2f total", portfolio.total_usd)

	// Safety check
	if st.status_item == nil {
		log.warn("[wallet_view] Status item is nil, cannot update display")
		return
	}

	button := appkit.NSStatusItem_button(cast(^appkit.NSStatusItem)st.status_item)
	if button == nil {
		log.warn("[wallet_view] Status button is nil, cannot update display")
		return
	}

	// Format title: "Portfolio: $1,234.56"
	title_text := fmt.tprintf("Portfolio: %s", formatters.format_usd_value(portfolio.total_usd))

	// Create simple title (no color for portfolio view)
	ns_title := appkit.NSString_fromString(title_text)
	appkit.NSButton_setTitle(button, ns_title)

	// Ensure status item is visible
	appkit.NSStatusItem_setVisible(cast(^appkit.NSStatusItem)st.status_item, true)

	log.debug("[wallet_view] Wallet display updated successfully")
}
