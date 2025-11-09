// Minimal example showing how to use the AppKit bindings
// This creates a simple menu bar app that displays "Hello Hound!"
package menubar

import "core:fmt"
import "base:runtime"

// Global app state
g_status_item: ^NSStatusItem

// Application delegate (must be zero-size for @(objc_class))
@(objc_class="MinimalAppDelegate")
AppDelegate :: struct {}

// Called when app finishes launching
@(objc_type=AppDelegate, objc_name="applicationDidFinishLaunching", objc_is_class_method=false)
app_did_finish_launching :: proc "c" (self: ^AppDelegate, _: SEL, notification: ^NSNotification) {
    context = runtime.default_context()

    fmt.println("App launched!")

    // Get system status bar
    status_bar := NSStatusBar_systemStatusBar()

    // Create status item with variable length
    g_status_item = NSStatusBar_statusItemWithLength(status_bar, NSVariableStatusItemLength)

    // Get button and set title
    button := NSStatusItem_button(g_status_item)
    title := NSString_fromString("Hello Hound! 🐕")
    NSButton_setTitle(button, title)

    // Create menu
    menu := NSMenu_new()

    // Add menu items
    item1 := NSMenuItem_new(
        NSString_fromString("Price: $0.42"),
        selector("dummyAction:"),
        NSString_fromString(""),
    )
    NSMenu_addItem(menu, item1)

    // Separator
    NSMenu_addItem(menu, NSMenuItem_separatorItem())

    // Quit item
    quit_item := NSMenuItem_new(
        NSString_fromString("Quit"),
        selector("terminate:"),
        NSString_fromString("q"),
    )
    NSMenu_addItem(menu, quit_item)

    // Attach menu to status item
    NSStatusItem_setMenu(g_status_item, menu)

    fmt.println("Status item created!")
}

main :: proc() {
    // Register delegate class
    delegate_class := objc_allocateClassPair(NSObject_class(), "MinimalAppDelegate", 0)

    // Add method
    class_addMethod(
        delegate_class,
        selector("applicationDidFinishLaunching:"),
        auto_cast app_did_finish_launching,
        "v@:@",  // void return, self, SEL, NSNotification
    )

    objc_registerClassPair(delegate_class)

    // Create app
    app := NSApplication_sharedApplication()

    // Create and set delegate (use class_createInstance for custom class)
    delegate := class_createInstance(delegate_class, 0)
    NSApplication_setDelegate(app, delegate)

    // Hide dock icon (menu bar only mode)
    NSApplication_setActivationPolicy(app, .Accessory)

    fmt.println("Starting app...")

    // Run app (blocks until quit)
    NSApplication_run(app)
}
