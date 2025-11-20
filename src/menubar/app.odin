package menubar

import "core:fmt"
import "core:strings"
import "core:time"
import "base:runtime"
import models "../lib/models"
import wallet_mgr "../wallet_manager"
import wallet_backend "../lib/wallet"
import token_cfg "../token_config"
import dex "../lib/dex"

// ============================================================================
// Global State
// ============================================================================

g_status_item: ^NSStatusItem
g_menu: ^NSMenu  // Store menu reference for dynamic updates
g_timer: ^NSTimer
g_current_symbol: string
g_current_price: models.PriceData
g_token_config: models.TokenConfig  // Cached config (loaded once)

// Wallet support (Phase 1)
g_wallet_manager: wallet_mgr.WalletManager
g_current_portfolio: wallet_backend.PortfolioBalance
g_wallet_mode_enabled: bool = false  // Toggle between price tracking and wallet tracking

// Menu item indices (hardcoded based on create_menu structure)
MENU_INDEX_PRICE :: 2
MENU_INDEX_HISTORY_START :: 7
MENU_INDEX_HISTORY_COUNT :: 5

// ============================================================================
// Status Item Creation
// ============================================================================

create_status_item :: proc() -> ^NSStatusItem {
    status_bar := NSStatusBar_systemStatusBar()
    item := NSStatusBar_statusItemWithLength(status_bar, NSVariableStatusItemLength)

    // Retain the status item to prevent it from being deallocated
    retain(item)

    // Make item visible
    NSStatusItem_setVisible(item, true)

    button := NSStatusItem_button(item)
    if button != nil {
        title := NSString_fromString("🔄 Loading...")
        NSButton_setTitle(button, title)
    }

    return item
}

// ============================================================================
// Menu Creation
// ============================================================================

create_menu :: proc(symbol: string) -> ^NSMenu {
    menu := NSMenu_new()

    // Header: Token name
    header_item := NSMenuItem_new(
        NSString_fromString(fmt.tprintf("%s Price Tracker", strings.to_upper(symbol))),
        nil,  // No action
        NSString_fromString(""),
    )
    NSMenu_addItem(menu, header_item)

    NSMenu_addItem(menu, NSMenuItem_separatorItem())

    // Current price placeholder (updated dynamically)
    price_item := NSMenuItem_new(
        NSString_fromString("Fetching price..."),
        nil,
        NSString_fromString(""),
    )
    NSMenu_addItem(menu, price_item)

    NSMenu_addItem(menu, NSMenuItem_separatorItem())

    // Refresh button
    refresh_item := NSMenuItem_new(
        NSString_fromString("Refresh Now"),
        selector("refreshPrice:"),
        NSString_fromString("r"),
    )
    NSMenu_addItem(menu, refresh_item)

    NSMenu_addItem(menu, NSMenuItem_separatorItem())

    // History section header
    history_header := NSMenuItem_new(
        NSString_fromString("Recent Prices"),
        nil,
        NSString_fromString(""),
    )
    NSMenu_addItem(menu, history_header)

    // History entries placeholder (filled after first fetch)
    for i in 0..<5 {
        history_item := NSMenuItem_new(
            NSString_fromString("  --"),
            nil,
            NSString_fromString(""),
        )
        NSMenu_addItem(menu, history_item)
    }

    NSMenu_addItem(menu, NSMenuItem_separatorItem())

    // Quit button
    quit_item := NSMenuItem_new(
        NSString_fromString("Quit"),
        selector("quitApp:"),
        NSString_fromString("q"),
    )
    NSMenu_addItem(menu, quit_item)

    return menu
}

// ============================================================================
// Display Update
// ============================================================================

update_menu_bar_display :: proc(symbol: string, data: models.PriceData) {
    if g_status_item == nil do return

    button := NSStatusItem_button(g_status_item)
    if button == nil {
        fmt.eprintln("ERROR: button is nil!")
        return
    }

    // Format text: "$0.0499↓" (compact for menu bar)
    arrow := data.change_24h >= 0 ? "↑" : "↓"
    text := fmt.tprintf("$%.4f %s", data.price_usd, arrow)

    // For now use simple title without color to ensure visibility
    ns_str := NSString_fromString(text)
    NSButton_setTitle(button, ns_str)

    // Make status item visible
    NSStatusItem_setVisible(g_status_item, true)

    fmt.printfln("[%s] Updated display: %s (%s)", time_now_string(), text, strings.to_upper(symbol))
}

update_menu_prices :: proc(symbol: string, data: models.PriceData) {
    if g_menu == nil do return

    // Update main price item
    price_item := NSMenuItem_itemAtIndex(g_menu, MENU_INDEX_PRICE)
    if price_item != nil {
        arrow := data.change_24h >= 0 ? "↑" : "↓"
        // Use explicit string formatting to avoid precision issues
        price_str := fmt.tprintf("%.6f", data.price_usd)
        change_str := fmt.tprintf("%.1f", data.change_24h)
        price_text := fmt.tprintf("Price: $%s %s %s%%", price_str, arrow, change_str)
        NSMenuItem_setTitle(price_item, NSString_fromString(price_text))
    }

    // Price history display removed - menubar focuses on current price only
    // Historical tracking moved to core/ services if needed
}

// ============================================================================
// Price Fetching (Reuse existing infrastructure)
// ============================================================================

fetch_and_update :: proc(symbol: string) {
    fmt.printfln("[%s] Fetching price for %s...", time_now_string(), symbol)

    // Use cached config (loaded once at startup)
    token, found := token_cfg.find_token_by_symbol(g_token_config, symbol)
    if !found {
        fmt.eprintfln("[%s] Token %s not found in config", time_now_string(), symbol)
        button := NSStatusItem_button(g_status_item)
        error_text := NSString_fromString(fmt.tprintf("%s: Not Found", strings.to_upper(symbol)))
        NSButton_setTitle(button, error_text)
        return
    }

    // Fetch price using existing Hound infrastructure
    data, err := dex.fetch_onchain_price(token)

    if err != .None {
        fmt.eprintfln("[%s] Error fetching price: %v", time_now_string(), err)

        // Update display with error indicator
        button := NSStatusItem_button(g_status_item)
        error_text := NSString_fromString(fmt.tprintf("%s: Error", strings.to_upper(symbol)))
        NSButton_setTitle(button, error_text)

        return
    }

    // Update global state
    g_current_price = data

    // Update display
    update_menu_bar_display(symbol, data)

    // Price history saving removed - menubar focuses on current prices only
    // Database operations moved to core/ services

    // Update menu items
    update_menu_prices(symbol, data)

    fmt.printfln("[%s] Price updated: $%.6f (%+.1f%%)",
        time_now_string(), data.price_usd, data.change_24h)
}

// ============================================================================
// Helpers
// ============================================================================

time_now_string :: proc() -> string {
    // Format current time as HH:MM:SS
    now := time.now()
    hour, min, sec := time.clock(now)
    return fmt.tprintf("%02d:%02d:%02d", hour, min, sec)
}

// ============================================================================
// Wallet Portfolio Functions (Phase 1)
// ============================================================================

// fetch_and_update_portfolio refreshes wallet portfolios and updates UI
fetch_and_update_portfolio :: proc() {
	fmt.printfln("[%s] Refreshing wallet portfolios...", time_now_string())

	// Refresh all portfolios
	err := wallet_mgr.refresh_all_portfolios(&g_wallet_manager)
	if err != .None {
		fmt.eprintfln("[%s] Error refreshing portfolios: %v", time_now_string(), err)

		// Update display with error indicator
		button := NSStatusItem_button(g_status_item)
		error_text := NSString_fromString("🐕 Error")
		NSButton_setTitle(button, error_text)

		return
	}

	// Get aggregated portfolio
	portfolio := wallet_mgr.get_aggregated_portfolio(&g_wallet_manager)
	g_current_portfolio = portfolio

	// Update display
	update_wallet_display(portfolio)

	// Update menu
	update_wallet_menu(portfolio)

	fmt.printfln("[%s] Portfolio updated: $%.2f total",
		time_now_string(), portfolio.total_usd)
}
