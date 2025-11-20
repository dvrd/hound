// Objective-C delegate and callbacks
package menubar

import "core:fmt"
import "core:os"
import "core:path/filepath"
import "base:runtime"
import models "../lib/models"
import db "../lib/database"
import wallet "../lib/wallet"
import token_cfg "../config"

// ============================================================================
// Delegate Class Definition
// ============================================================================

@(objc_class="HoundAppDelegate")
HoundAppDelegate :: struct {}  // MUST be zero-size

// ============================================================================
// App Lifecycle Callbacks
// ============================================================================

@(objc_type=HoundAppDelegate, objc_name="applicationDidFinishLaunching", objc_is_class_method=false)
app_did_finish_launching :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    notification: ^NSNotification,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    fmt.println("Hound menu bar app launched!")

    // Load token config (once at startup)
    config, config_err := token_cfg.load_token_config()
    if config_err != .None {
        fmt.eprintln("ERROR: Failed to load token configuration")
        fmt.eprintln("Please create ~/.config/hound/tokens.json with your token definitions.")
        // Continue anyway - will show error on each fetch attempt
    } else {
        g_token_config = config
        fmt.printfln("Loaded %d tokens from configuration", len(config.tokens))
    }

    // Check if database exists to determine mode
    home, found := os.lookup_env("HOME")
    if found && len(home) > 0 {
        db_path := filepath.join({home, ".config", "hound", "hound.db"})
        if os.exists(db_path) {
            fmt.println("Database found - enabling wallet mode")
            g_wallet_mode_enabled = true

            // Open database
            database, db_err := db.database_open(db_path)
            if db_err != .None {
                fmt.eprintfln("ERROR: Failed to open database: %v", db_err)
                g_wallet_mode_enabled = false
            } else {
                // Initialize wallet manager
                rpc_endpoint := "https://api.mainnet-beta.solana.com"
                backup_endpoints: []string = nil

                manager, init_err := wallet.init_wallet_manager(&g_token_config, database, rpc_endpoint, backup_endpoints)
                if init_err != .None {
                    fmt.eprintfln("ERROR: Failed to initialize wallet manager: %v", init_err)
                    g_wallet_mode_enabled = false
                } else {
                    g_wallet_manager = manager
                    // CRITICAL: Fix pointers after struct copy
                    // When we copy the manager struct, all pointers still point to the local 'manager' variable
                    // We need to update them to point to g_wallet_manager fields
                    g_wallet_manager.balance_fetcher.rpc_client = &g_wallet_manager.rpc_client
                    fmt.println("Wallet manager initialized")
                }
            }
        } else {
            fmt.println("Database not found - using price tracking mode")
        }
    }

    // Create status item
    g_status_item = create_status_item()

    // Create menu based on mode
    if g_wallet_mode_enabled {
        g_menu = create_wallet_menu(&g_wallet_manager)
        fmt.println("Created wallet portfolio menu")
    } else {
        // Fallback to price tracking mode
        symbol := "aura"
        g_current_symbol = symbol

        // Price database initialization removed - managed by core/ services

        g_menu = create_menu(symbol)
        fmt.println("Created price tracking menu")
    }

    NSStatusItem_setMenu(g_status_item, g_menu)

    // Schedule initial fetch after 2s delay (avoid crash during app launch)
    // This gives macOS networking stack time to fully initialize
    initial_timer := NSTimer_scheduledTimerWithTimeInterval(
        2.0,                          // 2 second delay
        id(self),                     // target (retained by timer)
        selector("timerFired:"),      // selector
        nil,                          // userInfo
        false,                        // repeats = false (one-shot)
    )
    fmt.println("Scheduled initial fetch (2s delay)")

    // Start recurring timer (5 second interval, repeating)
    g_timer = NSTimer_scheduledTimerWithTimeInterval(
        5.0,                          // seconds
        id(self),                     // target (retained by timer)
        selector("timerFired:"),      // selector
        nil,                          // userInfo
        true,                         // repeats
    )

    fmt.println("Timer started (5s interval)")
}

// ============================================================================
// Timer Callback
// ============================================================================

@(objc_type=HoundAppDelegate, objc_name="timerFired", objc_is_class_method=false)
timer_fired :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    timer: ^NSTimer,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    // Create autorelease pool for this callback (drains autoreleased objects)
    pool := NSAutoreleasePool_new()
    defer NSAutoreleasePool_drain(pool)

    fmt.println("Timer fired - refreshing data")

    // Handle both modes
    if g_wallet_mode_enabled {
        fetch_and_update_portfolio()
    } else {
        fetch_and_update(g_current_symbol)
    }
}

// ============================================================================
// Menu Action Callbacks
// ============================================================================

@(objc_type=HoundAppDelegate, objc_name="refreshPrice", objc_is_class_method=false)
refresh_price_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    sender: id,
) {
    context = runtime.default_context()

    fmt.println("Manual refresh triggered")
    fetch_and_update(g_current_symbol)
}

@(objc_type=HoundAppDelegate, objc_name="refreshPortfolio", objc_is_class_method=false)
refresh_portfolio_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    sender: id,
) {
    context = runtime.default_context()

    fmt.println("Manual portfolio refresh triggered")
    fetch_and_update_portfolio()
}

@(objc_type=HoundAppDelegate, objc_name="showSwapDialog", objc_is_class_method=false)
show_swap_dialog_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    sender: id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    fmt.println("Swap Tokens triggered")

    // Show swap dialog
    success := show_swap_dialog(&g_wallet_manager)
    if success {
        fmt.println("Swap transaction created successfully")
    }
}

@(objc_type=HoundAppDelegate, objc_name="manageWallets", objc_is_class_method=false)
manage_wallets_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    sender: id,
) {
    context = runtime.default_context()

    fmt.println("Manage Wallets triggered")

    // Show add wallet dialog
    success := show_add_wallet_dialog(&g_wallet_manager)
    if success {
        fmt.println("Wallet added successfully")
        // Refresh portfolio to include new wallet
        fetch_and_update_portfolio()
    }
}

@(objc_type=HoundAppDelegate, objc_name="quitApp", objc_is_class_method=false)
quit_app_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: SEL,
    sender: id,
) {
    context = runtime.default_context()

    fmt.println("Quit action triggered")

    // Invalidate timer (CRITICAL: prevents retain cycle leak)
    if g_timer != nil {
        NSTimer_invalidate(g_timer)
        g_timer = nil
        fmt.println("Timer invalidated")
    }

    // Database closing removed - managed by core/ services
    fmt.println("MenuBar app terminating")

    // Terminate app
    app := NSApplication_sharedApplication()
    NSApplication_terminate(app, nil)
}

// ============================================================================
// Main Entry Point
// ============================================================================

main :: proc() {
    // Register custom delegate class
    delegate_class := objc_allocateClassPair(NSObject_class(), "HoundAppDelegate", 0)

    // Add methods
    class_addMethod(
        delegate_class,
        selector("applicationDidFinishLaunching:"),
        auto_cast app_did_finish_launching,
        "v@:@",  // void, self, SEL, NSNotification
    )

    class_addMethod(
        delegate_class,
        selector("timerFired:"),
        auto_cast timer_fired,
        "v@:@",  // void, self, SEL, NSTimer
    )

    class_addMethod(
        delegate_class,
        selector("refreshPrice:"),
        auto_cast refresh_price_action,
        "v@:@",  // void, self, SEL, id
    )

    class_addMethod(
        delegate_class,
        selector("refreshPortfolio:"),
        auto_cast refresh_portfolio_action,
        "v@:@",  // void, self, SEL, id
    )

    class_addMethod(
        delegate_class,
        selector("showSwapDialog:"),
        auto_cast show_swap_dialog_action,
        "v@:@",  // void, self, SEL, id
    )

    class_addMethod(
        delegate_class,
        selector("manageWallets:"),
        auto_cast manage_wallets_action,
        "v@:@",  // void, self, SEL, id
    )

    class_addMethod(
        delegate_class,
        selector("quitApp:"),
        auto_cast quit_app_action,
        "v@:@",  // void, self, SEL, id
    )

    objc_registerClassPair(delegate_class)

    // Create app
    app := NSApplication_sharedApplication()

    // Create delegate instance
    delegate := class_createInstance(delegate_class, 0)
    NSApplication_setDelegate(app, delegate)

    // Hide dock icon (menu bar only mode)
    NSApplication_setActivationPolicy(app, .Accessory)

    fmt.println("Starting Hound menu bar app...")

    // Run app (blocks until quit)
    NSApplication_run(app)
}
