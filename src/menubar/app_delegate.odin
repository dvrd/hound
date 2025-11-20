// Objective-C delegate and callbacks
// PATTERN: Callbacks orchestrate handlers + views (no direct service calls)
package menubar

import "core:fmt"
import "core:log"
import "base:runtime"
import appkit "../appkit"
import state "./state"
import handlers "./handlers"
import views "./views"

// ============================================================================
// Global State
// ============================================================================

// Global state pointer (required for Objective-C callback context)
// GOTCHA: Cannot avoid this global due to Objective-C callback ABI
g_menubar_state: ^state.MenuBarState

// ============================================================================
// Delegate Class Definition
// ============================================================================

@(objc_class="HoundAppDelegate")
HoundAppDelegate :: struct {}  // MUST be zero-size

// ============================================================================
// App Lifecycle Callbacks
// ============================================================================

// PATTERN: applicationDidFinishLaunching is now a minimal callback
// All initialization happens in main.odin before NSApplication_run
@(objc_type=HoundAppDelegate, objc_name="applicationDidFinishLaunching", objc_is_class_method=false)
app_did_finish_launching :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    notification: ^appkit.NSNotification,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    fmt.println("Hound menu bar app launched!")

    // PATTERN: State is already initialized in main.odin
    // This callback just confirms the app is ready
    if g_menubar_state == nil {
        log.error("ERROR: MenuBarState not initialized - app will not function correctly")
        return
    }

    log.infof("App launched in %v mode", g_menubar_state.mode)
}

// ============================================================================
// Timer Callback
// ============================================================================

// PATTERN: Timer callback orchestrates handlers + views
// NO direct service calls - only handler calls
@(objc_type=HoundAppDelegate, objc_name="timerFired", objc_is_class_method=false)
timer_fired :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    timer: ^appkit.NSTimer,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    // Create autorelease pool for this callback (drains autoreleased objects)
    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    // PATTERN: Access global state (unavoidable in callback context)
    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.debug("Timer fired - refreshing data")

    // PATTERN: Orchestrate: handler -> view update
    switch g_menubar_state.mode {
    case .Price:
        // PATTERN: Call handler for business logic
        price_usd, change_24h, err := handlers.handle_fetch_price(
            g_menubar_state,
            g_menubar_state.current_symbol,
        )
        if err != .None {
            log.errorf("Price fetch failed: %v", err)
            views.show_error_alert("Price fetch failed")
            return
        }

        // PATTERN: Update view with new data
        views.update_price_display(g_menubar_state, g_menubar_state.current_symbol, price_usd, change_24h)

    case .Wallet:
        // PATTERN: Same structure for wallet mode
        portfolio, err := handlers.handle_fetch_portfolio(g_menubar_state)
        if err != .None {
            log.errorf("Portfolio fetch failed: %v", err)
            views.show_error_alert("Portfolio fetch failed")
            return
        }

        g_menubar_state.current_portfolio = portfolio
        views.update_wallet_display(g_menubar_state, portfolio)
    }
}

// ============================================================================
// Menu Action Callbacks
// ============================================================================

// PATTERN: Action callbacks orchestrate handlers + views
@(objc_type=HoundAppDelegate, objc_name="refreshPrice", objc_is_class_method=false)
refresh_price_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Manual price refresh triggered")

    // Force immediate update
    price_usd, change_24h, err := handlers.handle_fetch_price(
        g_menubar_state,
        g_menubar_state.current_symbol,
    )
    if err != .None {
        log.errorf("Price fetch failed: %v", err)
        views.show_error_alert("Price fetch failed")
        return
    }

    views.update_price_display(g_menubar_state, g_menubar_state.current_symbol, price_usd, change_24h)
}

@(objc_type=HoundAppDelegate, objc_name="refreshPortfolio", objc_is_class_method=false)
refresh_portfolio_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Manual portfolio refresh triggered")

    // Force immediate update
    portfolio, err := handlers.handle_fetch_portfolio(g_menubar_state)
    if err != .None {
        log.errorf("Portfolio fetch failed: %v", err)
        views.show_error_alert("Portfolio fetch failed")
        return
    }

    g_menubar_state.current_portfolio = portfolio
    views.update_wallet_display(g_menubar_state, portfolio)
}

@(objc_type=HoundAppDelegate, objc_name="showSwapDialog", objc_is_class_method=false)
show_swap_dialog_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Swap Tokens triggered")

    // Launch swap dialog flow
    // TODO: Implement full swap dialog orchestration
    // This requires coordinating multiple handlers and views:
    // 1. views.show_token_selection_dialog (from token)
    // 2. views.show_amount_input_dialog
    // 3. views.show_destination_token_dialog (to token)
    // 4. handlers.handle_get_swap_quote
    // 5. views.show_quote_preview_dialog
    // 6. handlers.handle_build_swap_transaction
    // 7. views.show_transaction_export_dialog

    views.show_error_alert("Swap feature not yet implemented in new architecture")
    log.warn("Swap feature requires full multi-step dialog orchestration")
}

@(objc_type=HoundAppDelegate, objc_name="manageWallets", objc_is_class_method=false)
manage_wallets_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Manage Wallets triggered")

    // Show add wallet dialog
    // TODO: Implement wallet management dialog
    // This requires UI dialogs for:
    // 1. List existing wallets
    // 2. Add new wallet (address input)
    // 3. Remove wallet
    // 4. Refresh portfolio after changes

    views.show_error_alert("Wallet management not yet implemented in new architecture")
    log.warn("Wallet management requires dialog implementation")
}

// PATTERN: Mode switching actions
@(objc_type=HoundAppDelegate, objc_name="switchToPriceMode", objc_is_class_method=false)
switch_to_price_mode :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Switching to Price mode")

    // Switch mode
    g_menubar_state.mode = .Price

    // Fetch and display price immediately
    price_usd, change_24h, err := handlers.handle_fetch_price(
        g_menubar_state,
        g_menubar_state.current_symbol,
    )
    if err != .None {
        log.errorf("Price fetch failed: %v", err)
        views.show_error_alert("Price fetch failed")
        return
    }

    views.update_price_display(g_menubar_state, g_menubar_state.current_symbol, price_usd, change_24h)
    log.info("Switched to Price mode")
}

@(objc_type=HoundAppDelegate, objc_name="switchToWalletMode", objc_is_class_method=false)
switch_to_wallet_mode :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    if g_menubar_state == nil {
        log.error("MenuBarState not initialized")
        return
    }

    log.info("Switching to Wallet mode")

    // Switch mode
    g_menubar_state.mode = .Wallet

    // Fetch and display portfolio immediately
    portfolio, err := handlers.handle_fetch_portfolio(g_menubar_state)
    if err != .None {
        log.errorf("Portfolio fetch failed: %v", err)
        views.show_error_alert("Portfolio fetch failed")
        return
    }

    g_menubar_state.current_portfolio = portfolio
    views.update_wallet_display(g_menubar_state, portfolio)
    log.info("Switched to Wallet mode")
}

@(objc_type=HoundAppDelegate, objc_name="quitApp", objc_is_class_method=false)
quit_app_action :: proc "c" (
    self: ^HoundAppDelegate,
    _: appkit.SEL,
    sender: appkit.id,
) {
    context = runtime.default_context()  // CRITICAL: Set context FIRST

    pool := appkit.NSAutoreleasePool_new()
    defer appkit.NSAutoreleasePool_drain(pool)

    log.info("Quit action triggered")

    // PATTERN: Clean shutdown - invalidate timer
    if g_menubar_state != nil && g_menubar_state.timer != nil {
        appkit.NSTimer_invalidate(cast(^appkit.NSTimer)g_menubar_state.timer)
        g_menubar_state.timer = nil
        log.debug("Timer invalidated")
    }

    // PATTERN: Database closing handled by main.odin deferred cleanup
    log.info("MenuBar app terminating")

    // Terminate app
    app := appkit.NSApplication_sharedApplication()
    appkit.NSApplication_terminate(app, nil)
}

// ============================================================================
// Delegate Class Registration
// ============================================================================

// PATTERN: Register AppDelegate class with Objective-C runtime
// Called from main.odin before NSApplication_run
register_app_delegate_class :: proc() -> rawptr {
    // Register custom delegate class
    delegate_class := appkit.objc_allocateClassPair(appkit.NSObject_class(), "HoundAppDelegate", 0)

    // Add lifecycle methods
    appkit.class_addMethod(
        delegate_class,
        appkit.selector("applicationDidFinishLaunching:"),
        auto_cast app_did_finish_launching,
        "v@:@",  // void, self, SEL, NSNotification
    )

    // Add timer callback
    appkit.class_addMethod(
        delegate_class,
        appkit.selector("timerFired:"),
        auto_cast timer_fired,
        "v@:@",  // void, self, SEL, NSTimer
    )

    // Add action methods
    appkit.class_addMethod(
        delegate_class,
        appkit.selector("refreshPrice:"),
        auto_cast refresh_price_action,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("refreshPortfolio:"),
        auto_cast refresh_portfolio_action,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("showSwapDialog:"),
        auto_cast show_swap_dialog_action,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("manageWallets:"),
        auto_cast manage_wallets_action,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("switchToPriceMode:"),
        auto_cast switch_to_price_mode,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("switchToWalletMode:"),
        auto_cast switch_to_wallet_mode,
        "v@:@",  // void, self, SEL, id
    )

    appkit.class_addMethod(
        delegate_class,
        appkit.selector("quitApp:"),
        auto_cast quit_app_action,
        "v@:@",  // void, self, SEL, id
    )

    appkit.objc_registerClassPair(delegate_class)

    return delegate_class
}
