// Price view - pure UI layer for price display mode
// No business logic, no service calls, only AppKit UI creation
package views

import "core:fmt"
import "core:log"
import state "../state"
import formatters "../formatters"
import appkit "../../appkit"

// ============================================================================
// Menu Creation
// ============================================================================

// create_price_menu creates the price display menu
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Symbol must not be empty
// ASSERTION 3: Price must be non-negative
//
// Returns: Newly created NSMenu with price display items
create_price_menu :: proc(
	st: ^state.MenuBarState,
	symbol: string,
	price_usd: f64,
	change_24h: f64,
) -> ^appkit.NSMenu {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")
	assert(price_usd >= 0, "Price must be non-negative")

	log.debugf("[price_view] Creating price menu: %s @ $%.2f (%.2f%%)", symbol, price_usd, change_24h)

	menu := appkit.NSMenu_new()

	// Title item with formatted price
	title_text := formatters.format_price_menu_title(symbol, price_usd, change_24h)
	title_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString(title_text),
		nil,
		nil,
	)
	appkit.NSMenu_addItem(menu, title_item)

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// Refresh button
	refresh_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Refresh Price"),
		appkit.selector("refreshPrice:"),
		appkit.NSString_fromString("r"),
	)
	appkit.NSMenu_addItem(menu, refresh_item)

	// Switch to Wallet Mode button
	switch_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Switch to Wallet Mode"),
		appkit.selector("switchToWalletMode:"),
		appkit.NSString_fromString("w"),
	)
	appkit.NSMenu_addItem(menu, switch_item)

	// Separator
	appkit.NSMenu_addItem(menu, appkit.NSMenuItem_separatorItem())

	// Quit button
	quit_item := appkit.NSMenuItem_new(
		appkit.NSString_fromString("Quit"),
		appkit.selector("quitApp:"),
		appkit.NSString_fromString("q"),
	)
	appkit.NSMenu_addItem(menu, quit_item)

	log.debug("[price_view] Price menu created successfully")
	return menu
}

// ============================================================================
// Display Update
// ============================================================================

// update_price_display updates the status bar button with new price data
//
// ASSERTION 1: State must not be nil
// ASSERTION 2: Symbol must not be empty
// ASSERTION 3: Price must be non-negative
//
// Side effects: Updates status bar button title with colored attributed text
update_price_display :: proc(
	st: ^state.MenuBarState,
	symbol: string,
	price_usd: f64,
	change_24h: f64,
) {
	assert(st != nil, "MenuBarState cannot be nil")
	assert(len(symbol) > 0, "Symbol cannot be empty")
	assert(price_usd >= 0, "Price must be non-negative")

	log.debugf("[price_view] Updating price display: %s @ $%.2f (%.2f%%)", symbol, price_usd, change_24h)

	// Safety check
	if st.status_item == nil {
		log.warn("[price_view] Status item is nil, cannot update display")
		return
	}

	button := appkit.NSStatusItem_button(cast(^appkit.NSStatusItem)st.status_item)
	if button == nil {
		log.warn("[price_view] Status button is nil, cannot update display")
		return
	}

	// Format title text using formatter
	title_text := formatters.format_price_menu_title(symbol, price_usd, change_24h)

	// Choose color based on 24h change
	color: ^appkit.NSColor
	if change_24h > 0 {
		color = appkit.NSColor_systemGreenColor()
	} else if change_24h < 0 {
		color = appkit.NSColor_systemRedColor()
	} else {
		color = appkit.NSColor_systemGrayColor()
	}

	// Create attributed string with color
	ns_title := appkit.NSString_fromString(title_text)
	color_dict := appkit.NSDictionary_withColorAttribute(color)
	attributed_title := appkit.NSAttributedString_new(ns_title, color_dict)

	// Update button title
	appkit.NSButton_setAttributedTitle(button, attributed_title)

	// Ensure status item is visible
	appkit.NSStatusItem_setVisible(cast(^appkit.NSStatusItem)st.status_item, true)

	log.debug("[price_view] Price display updated successfully")
}
