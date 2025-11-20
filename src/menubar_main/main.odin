// MenuBar Application Entry Point
// PATTERN: Phase 4 BASE - Initialize services → create state → setup UI → run event loop
// NOTE: This file defines a NEW main entry point to replace app_delegate.odin's main()
// The app_delegate.odin main() will be removed in the next refactoring step
package menubar_main

import "core:fmt"
import "core:log"
import "core:os"
import "core:path/filepath"
import "base:runtime"
import models "../lib/models"
import memory "../lib/memory"
import db "../lib/database"
import wallet "../lib/wallet"
import token_cfg "../lib/config"
import appkit "../appkit"
import menubar "../menubar"
import state "../menubar/state"

// ============================================================================
// Global State (required for Objective-C callbacks)
// ============================================================================

// Note: Global state is actually stored in menubar.g_menubar_state
// We reference it here for convenience

// ============================================================================
// Main Entry Point
// ============================================================================

main :: proc() {
	// 1. Initialize logger
	log_level := log.Level.Info
	if ODIN_DEBUG {
		log_level = log.Level.Debug
	}

	context.logger = log.create_console_logger(log_level, {.Level, .Terminal_Color})
	defer log.destroy_console_logger(context.logger)

	log.debug("Hound menu bar app starting")
	log.debugf("Log level: %v", log_level)

	// 2. Initialize memory arenas
	mem_err := memory.memory_init()
	if mem_err != .None {
		log.errorf("Failed to initialize memory system: %v", mem_err)
		os.exit(1)
	}
	defer memory.memory_shutdown()

	// 3. Open database
	home, home_found := os.lookup_env("HOME")
	if !home_found {
		log.error("HOME environment variable not found")
		os.exit(1)
	}

	db_path := filepath.join({home, ".config", "hound", "hound.db"})
	log.debugf("Database path: %s", db_path)

	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		os.exit(1)
	}
	defer db.database_close(database)

	log.info("Database opened successfully")

	// 4. Initialize RPC client
	rpc_endpoint := "https://api.mainnet-beta.solana.com"
	backup_endpoints: []string = nil  // No backup endpoints for now

	log.debugf("RPC endpoint: %s", rpc_endpoint)
	rpc_client := wallet.init_rpc_client(rpc_endpoint, backup_endpoints)

	// 5. Initialize balance fetcher
	price_fetcher := wallet.PriceFetcher{}
	balance_fetcher := wallet.init_balance_fetcher(&rpc_client, &price_fetcher)

	log.info("RPC client and balance fetcher initialized")

	// 6. Load token config
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		log.error("Please create ~/.config/hound/tokens.json with your token definitions")
		os.exit(1)
	}
	log.infof("Loaded %d tokens from configuration", len(config.tokens))

	// 7. Create MenuBarState
	menubar_state := state.create_state(database, rpc_client, balance_fetcher, config)

	// 8. Set global state for callbacks
	menubar.g_menubar_state = &menubar_state

	log.info("MenuBar state initialized")

	// 9. Initialize NSApplication
	app := appkit.NSApplication_sharedApplication()

	// 10. Register AppDelegate class
	delegate_class := appkit.objc_allocateClassPair(appkit.NSObject_class(), "HoundAppDelegate", 0)

	// Add delegate methods
	appkit.class_addMethod(
		delegate_class,
		appkit.selector("applicationDidFinishLaunching:"),
		auto_cast menubar.app_did_finish_launching,
		"v@:@",  // void, self, SEL, NSNotification
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("timerFired:"),
		auto_cast menubar.timer_fired,
		"v@:@",  // void, self, SEL, NSTimer
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("refreshPrice:"),
		auto_cast menubar.refresh_price_action,
		"v@:@",  // void, self, SEL, id
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("refreshPortfolio:"),
		auto_cast menubar.refresh_portfolio_action,
		"v@:@",  // void, self, SEL, id
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("showSwapDialog:"),
		auto_cast menubar.show_swap_dialog_action,
		"v@:@",  // void, self, SEL, id
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("manageWallets:"),
		auto_cast menubar.manage_wallets_action,
		"v@:@",  // void, self, SEL, id
	)

	appkit.class_addMethod(
		delegate_class,
		appkit.selector("quitApp:"),
		auto_cast menubar.quit_app_action,
		"v@:@",  // void, self, SEL, id
	)

	appkit.objc_registerClassPair(delegate_class)

	// Create delegate instance
	delegate := appkit.class_createInstance(delegate_class, 0)
	appkit.NSApplication_setDelegate(app, delegate)

	log.info("AppDelegate registered and set")

	// 11. Set activation policy to .Accessory (menu bar only, no dock icon)
	appkit.NSApplication_setActivationPolicy(app, .Accessory)

	log.info("Activation policy set to Accessory (menu bar only)")

	// 12. Create status bar item
	status_bar := appkit.NSStatusBar_systemStatusBar()
	status_item := appkit.NSStatusBar_statusItemWithLength(status_bar, appkit.NSVariableStatusItemLength)
	appkit.retain(status_item)  // Retain to prevent deallocation
	appkit.NSStatusItem_setVisible(status_item, true)

	menubar.g_menubar_state.status_item = cast(rawptr)status_item

	button := appkit.NSStatusItem_button(status_item)
	if button != nil {
		appkit.NSButton_setTitle(button, appkit.NSString_fromString("🐕"))
	}

	log.info("Status bar item created")

	// 13. Create initial empty menu (will be populated after first fetch)
	initial_menu := appkit.NSMenu_new()
	appkit.NSStatusItem_setMenu(status_item, initial_menu)
	menubar.g_menubar_state.menu = cast(rawptr)initial_menu

	log.info("Initial menu created")

	// 14. Start refresh timer (5.0 seconds, repeating)
	timer := appkit.NSTimer_scheduledTimerWithTimeInterval(
		menubar.g_menubar_state.refresh_interval,
		appkit.id(delegate),
		appkit.selector("timerFired:"),
		nil,
		true,  // Repeats
	)
	menubar.g_menubar_state.timer = cast(rawptr)timer

	log.infof("Timer started (%.1f second interval)", menubar.g_menubar_state.refresh_interval)

	fmt.println("Starting Hound menu bar app...")

	// 15. Run app event loop (blocks until quit)
	appkit.NSApplication_run(app)

	// 16. Cleanup happens via defers when app terminates
	log.info("MenuBar app terminated")
}
