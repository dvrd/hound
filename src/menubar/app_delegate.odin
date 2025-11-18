// Objective-C delegate and callbacks
package menubar

import "core:fmt"
import "base:runtime"
import src "../"

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

    // Parse symbol from command line args (or default to AURA)
    // For MVP, hardcode AURA (TODO: Add arg parsing)
    symbol := "aura"
    g_current_symbol = symbol

    // Load token config (once at startup)
    config, config_err := src.load_token_config()
    if config_err != .None {
        fmt.eprintln("ERROR: Failed to load token configuration")
        fmt.eprintln("Please create ~/.config/hound/tokens.json with your token definitions.")
        // Continue anyway - will show error on each fetch attempt
    } else {
        g_token_config = config
        fmt.printfln("Loaded %d tokens from configuration", len(config.tokens))
    }

    // Initialize database
    db, db_err := init_price_db(symbol)
    if db_err != .None {
        fmt.eprintln("ERROR: Failed to initialize database")
        // Continue anyway - app still usable without history
    }
    g_price_db = db

    // Create status item
    g_status_item = create_status_item()

    // Create menu and store reference for dynamic updates
    g_menu = create_menu(symbol)
    NSStatusItem_setMenu(g_status_item, g_menu)

    // Fetch initial price
    fetch_and_update(symbol)

    // Start timer (5 second interval, repeating)
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

    fmt.println("Timer fired - fetching new price")

    // Fetch and update display
    fetch_and_update(g_current_symbol)
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

    // Close database
    close_price_db(&g_price_db)
    fmt.println("Database closed")

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
