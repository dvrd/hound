// Wallet UI components for menubar app
// Displays portfolio balances and provides wallet management UI
package menubar

import "core:fmt"
import "core:strings"
import wallet "../wallet"
import hound "../"

// ============================================================================
// Menu Creation for Wallet Portfolio
// ============================================================================

// create_wallet_menu creates a menu showing portfolio balances
create_wallet_menu :: proc(manager: ^wallet.WalletManager) -> ^NSMenu {
	menu := NSMenu_new()

	// Header: App name
	header_item := NSMenuItem_new(
		NSString_fromString("🐕 Hound Wallet"),
		nil,
		NSString_fromString(""),
	)
	NSMenu_addItem(menu, header_item)

	NSMenu_addItem(menu, NSMenuItem_separatorItem())

	// Total portfolio value (placeholder, updated dynamically)
	total_item := NSMenuItem_new(
		NSString_fromString("Total: $0.00 (0.0%)"),
		nil,
		NSString_fromString(""),
	)
	NSMenu_addItem(menu, total_item)

	NSMenu_addItem(menu, NSMenuItem_separatorItem())

	// Token balances section (populated dynamically)
	// We'll add placeholders that get updated
	balance_header := NSMenuItem_new(
		NSString_fromString("Balances"),
		nil,
		NSString_fromString(""),
	)
	NSMenu_addItem(menu, balance_header)

	// Add placeholder balance items (will be replaced with actual tokens)
	for i in 0..<10 {  // Max 10 tokens shown
		balance_item := NSMenuItem_new(
			NSString_fromString("  --"),
			nil,
			NSString_fromString(""),
		)
		NSMenu_addItem(menu, balance_item)
	}

	NSMenu_addItem(menu, NSMenuItem_separatorItem())

	// Swap button (Phase 2)
	swap_item := NSMenuItem_new(
		NSString_fromString("💱 Swap Tokens"),
		selector("showSwapDialog:"),
		NSString_fromString("w"),
	)
	NSMenu_addItem(menu, swap_item)

	// Manage Wallets button
	manage_item := NSMenuItem_new(
		NSString_fromString("⚙ Manage Wallets"),
		selector("manageWallets:"),
		NSString_fromString("m"),
	)
	NSMenu_addItem(menu, manage_item)

	// Refresh button
	refresh_item := NSMenuItem_new(
		NSString_fromString("🔄 Refresh Now"),
		selector("refreshPortfolio:"),
		NSString_fromString("r"),
	)
	NSMenu_addItem(menu, refresh_item)

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

// update_wallet_display updates menubar title with total portfolio value
update_wallet_display :: proc(portfolio: wallet.PortfolioBalance) {
	if g_status_item == nil do return

	button := NSStatusItem_button(g_status_item)
	if button == nil do return

	// Format: "🐕 $12,345.67 (+2.3%)"
	// For MVP, use simple percentage (TODO: Calculate 24h change from database)
	change_pct := 0.0  // Placeholder for 24h change

	arrow := change_pct >= 0 ? "↑" : "↓"
	text := fmt.tprintf("🐕 $%.2f %s", portfolio.total_usd, arrow)

	ns_str := NSString_fromString(text)
	NSButton_setTitle(button, ns_str)

	NSStatusItem_setVisible(g_status_item, true)
}

// update_wallet_menu updates menu items with portfolio data
update_wallet_menu :: proc(portfolio: wallet.PortfolioBalance) {
	if g_menu == nil do return

	// Menu structure (hardcoded indices):
	// 0: Header (🐕 Hound Wallet)
	// 1: Separator
	// 2: Total value
	// 3: Separator
	// 4: "Balances" header
	// 5-14: Token balance items (up to 10 tokens)
	// 15: Separator
	// 16: Manage Wallets
	// 17: Refresh
	// 18: Separator
	// 19: Quit

	MENU_INDEX_TOTAL :: 2
	MENU_INDEX_BALANCES_START :: 5
	MAX_BALANCE_ITEMS :: 10

	// Update total value
	total_item := NSMenuItem_itemAtIndex(g_menu, MENU_INDEX_TOTAL)
	if total_item != nil {
		change_pct := 0.0  // Placeholder
		change_str := change_pct >= 0 ? fmt.tprintf("+%.1f%%", change_pct) : fmt.tprintf("%.1f%%", change_pct)
		total_text := fmt.tprintf("Total: $%.2f (%s 24h)", portfolio.total_usd, change_str)
		NSMenuItem_setTitle(total_item, NSString_fromString(total_text))
	}

	// Update token balances
	// First, show SOL
	balance_items := make([dynamic]wallet.TokenBalance, 0, MAX_BALANCE_ITEMS)
	defer delete(balance_items)

	append(&balance_items, portfolio.sol_balance)

	// Then add SPL tokens
	for token in portfolio.token_balances {
		if len(balance_items) >= MAX_BALANCE_ITEMS do break
		append(&balance_items, token)
	}

	// Update menu items
	for i in 0..<MAX_BALANCE_ITEMS {
		menu_idx := MENU_INDEX_BALANCES_START + i
		balance_item := NSMenuItem_itemAtIndex(g_menu, menu_idx)
		if balance_item == nil do continue

		if i < len(balance_items) {
			token := balance_items[i]

			// Format: "SOL    10.5    $2,310.00  🟢"
			// For MVP, skip the indicator emoji (TODO: Add 24h change indicator)
			balance_text := fmt.tprintf("%s    %.4f    $%.2f",
				token.symbol,
				token.amount,
				token.usd_value)

			NSMenuItem_setTitle(balance_item, NSString_fromString(balance_text))
		} else {
			// No more tokens - hide this item
			NSMenuItem_setTitle(balance_item, NSString_fromString(""))
		}
	}
}

// ============================================================================
// Wallet Management Dialog
// ============================================================================

// show_add_wallet_dialog shows a dialog to add a new wallet address
show_add_wallet_dialog :: proc(manager: ^wallet.WalletManager) -> (success: bool) {
	// Create alert
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString("Add Watch Address"))
	NSAlert_setInformativeText(alert, NSString_fromString("Enter Solana wallet address to watch:"))

	// Add text field for address
	address_field := NSTextField_newWithFrame(NSRect{x = 0, y = 24, width = 400, height = 24})
	NSTextField_setPlaceholderString(address_field, NSString_fromString("Base58 address (e.g., 11111111111111111111111111111111)"))

	// Add text field for label
	label_field := NSTextField_newWithFrame(NSRect{x = 0, y = 0, width = 400, height = 24})
	NSTextField_setPlaceholderString(label_field, NSString_fromString("Label (e.g., My Wallet)"))

	// Create container view for both fields
	container := NSView_newWithFrame(NSRect{x = 0, y = 0, width = 400, height = 52})
	NSView_addSubview(container, cast(^NSView)address_field)
	NSView_addSubview(container, cast(^NSView)label_field)

	NSAlert_setAccessoryView(alert, container)

	// Add buttons
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Add"))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("Cancel"))

	// Show alert (blocks until user responds)
	response := NSAlert_runModal(alert)

	// NSAlertFirstButtonReturn = 1000
	if response == 1000 {
		// Get values from text fields
		address_str := NSTextField_stringValue(address_field)
		address := NSString_toString(address_str)

		label_str := NSTextField_stringValue(label_field)
		label := NSString_toString(label_str)

		// Validate inputs
		if len(address) == 0 {
			show_error_alert("Please enter a wallet address")
			return false
		}

		if len(label) == 0 {
			label = "Unnamed Wallet"
		}

		// Add to wallet manager
		err := wallet.add_wallet(manager, address, label, false)
		if err != .None {
			error_msg := fmt.tprintf("Failed to add wallet: %v", err)
			show_error_alert(error_msg)
			return false
		}

		return true
	}

	return false
}

// show_error_alert shows an error message dialog
show_error_alert :: proc(message: string) {
	alert := NSAlert_new()
	NSAlert_setMessageText(alert, NSString_fromString("Error"))
	NSAlert_setInformativeText(alert, NSString_fromString(message))
	NSAlert_addButtonWithTitle(alert, NSString_fromString("OK"))
	NSAlert_runModal(alert)
}
