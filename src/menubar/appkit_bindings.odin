// Minimal AppKit and Foundation bindings for macOS menu bar app
// Only includes what's needed - no bloat!
package menubar

import "base:intrinsics"
import "base:runtime"

when ODIN_OS != .Darwin {
    #panic("This package only works on macOS")
}

// Foreign imports
foreign import AppKit "system:AppKit.framework"
foreign import Foundation "system:Foundation.framework"
foreign import objc "system:objc"

// Objective-C runtime
foreign objc {
    objc_getClass :: proc(name: cstring) -> Class ---
    sel_registerName :: proc(name: cstring) -> SEL ---
    class_addMethod :: proc(cls: Class, name: SEL, imp: IMP, types: cstring) -> bool ---
    objc_allocateClassPair :: proc(superclass: Class, name: cstring, extraBytes: uint) -> Class ---
    objc_registerClassPair :: proc(cls: Class) ---
}

// Core Objective-C types (use Odin's built-in types)
id       :: intrinsics.objc_id
Class    :: intrinsics.objc_Class
SEL      :: intrinsics.objc_SEL
IMP      :: rawptr
CGFloat  :: f64

// Common constants
NSVariableStatusItemLength :: CGFloat(-2.0)

// Activation policies for NSApplication
NSApplicationActivationPolicy :: enum i64 {
    Regular   = 0,
    Accessory = 1,
    Prohibited = 2,
}

// Opaque types
NSApplication      :: distinct id
NSStatusBar        :: distinct id
NSStatusItem       :: distinct id
NSMenu             :: distinct id
NSMenuItem         :: distinct id
NSButton           :: distinct id
NSString           :: distinct id
NSColor            :: distinct id
NSAttributedString :: distinct id
NSDictionary       :: distinct id
NSTimer            :: distinct id
NSUserDefaults     :: distinct id
NSNotification     :: distinct id
NSObject           :: struct {}  // Zero-size for @(objc_class)

// Helper to get class
get_class :: proc(name: cstring) -> Class {
    return objc_getClass(name)
}

// Helper to get selector
selector :: proc(name: cstring) -> SEL {
    return sel_registerName(name)
}

// ============================================================================
// NSApplication
// ============================================================================

NSApplication_sharedApplication :: proc() -> NSApplication {
    class := get_class("NSApplication")
    return NSApplication(intrinsics.objc_send(id, class, selector("sharedApplication")))
}

NSApplication_setActivationPolicy :: proc(app: NSApplication, policy: NSApplicationActivationPolicy) {
    intrinsics.objc_send(nil, app, selector("setActivationPolicy:"), policy)
}

NSApplication_run :: proc(app: NSApplication) {
    intrinsics.objc_send(nil, app, selector("run"))
}

NSApplication_terminate :: proc(app: NSApplication, sender: id = nil) {
    intrinsics.objc_send(nil, app, selector("terminate:"), sender)
}

NSApplication_setDelegate :: proc(app: NSApplication, delegate: id) {
    intrinsics.objc_send(nil, app, selector("setDelegate:"), delegate)
}

// ============================================================================
// NSStatusBar
// ============================================================================

NSStatusBar_systemStatusBar :: proc() -> NSStatusBar {
    class := get_class("NSStatusBar")
    return NSStatusBar(intrinsics.objc_send(id, class, selector("systemStatusBar")))
}

NSStatusBar_statusItemWithLength :: proc(bar: NSStatusBar, length: CGFloat) -> NSStatusItem {
    return NSStatusItem(intrinsics.objc_send(id, bar, selector("statusItemWithLength:"), length))
}

// ============================================================================
// NSStatusItem
// ============================================================================

NSStatusItem_button :: proc(item: NSStatusItem) -> NSButton {
    return NSButton(intrinsics.objc_send(id, item, selector("button")))
}

NSStatusItem_setMenu :: proc(item: NSStatusItem, menu: NSMenu) {
    intrinsics.objc_send(nil, item, selector("setMenu:"), menu)
}

// ============================================================================
// NSButton
// ============================================================================

NSButton_setTitle :: proc(button: NSButton, title: NSString) {
    intrinsics.objc_send(nil, button, selector("setTitle:"), title)
}

NSButton_setAttributedTitle :: proc(button: NSButton, title: NSAttributedString) {
    intrinsics.objc_send(nil, button, selector("setAttributedTitle:"), title)
}

// ============================================================================
// NSMenu
// ============================================================================

NSMenu_alloc :: proc() -> NSMenu {
    class := get_class("NSMenu")
    return NSMenu(intrinsics.objc_send(id, class, selector("alloc")))
}

NSMenu_init :: proc(menu: NSMenu) -> NSMenu {
    return NSMenu(intrinsics.objc_send(id, menu, selector("init")))
}

NSMenu_new :: proc() -> NSMenu {
    menu := NSMenu_alloc()
    return NSMenu_init(menu)
}

NSMenu_addItem :: proc(menu: NSMenu, item: NSMenuItem) {
    intrinsics.objc_send(nil, menu, selector("addItem:"), item)
}

// ============================================================================
// NSMenuItem
// ============================================================================

NSMenuItem_alloc :: proc() -> NSMenuItem {
    class := get_class("NSMenuItem")
    return NSMenuItem(intrinsics.objc_send(id, class, selector("alloc")))
}

NSMenuItem_initWithTitle :: proc(item: NSMenuItem, title: NSString, action: SEL, keyEquivalent: NSString) -> NSMenuItem {
    return NSMenuItem(intrinsics.objc_send(id, item, selector("initWithTitle:action:keyEquivalent:"), title, action, keyEquivalent))
}

NSMenuItem_new :: proc(title: NSString, action: SEL = nil, keyEquivalent: NSString = nil) -> NSMenuItem {
    item := NSMenuItem_alloc()
    key_equiv := keyEquivalent
    if key_equiv == nil {
        key_equiv = NSString_fromString("")
    }
    return NSMenuItem_initWithTitle(item, title, action, key_equiv)
}

NSMenuItem_separatorItem :: proc() -> NSMenuItem {
    class := get_class("NSMenuItem")
    return NSMenuItem(intrinsics.objc_send(id, class, selector("separatorItem")))
}

NSMenuItem_setTarget :: proc(item: NSMenuItem, target: id) {
    intrinsics.objc_send(nil, item, selector("setTarget:"), target)
}

// ============================================================================
// NSString
// ============================================================================

NSUTF8StringEncoding :: uint(4)

NSString_alloc :: proc() -> NSString {
    class := get_class("NSString")
    return NSString(intrinsics.objc_send(id, class, selector("alloc")))
}

NSString_initWithBytes :: proc(str: NSString, bytes: rawptr, length: uint, encoding: uint) -> NSString {
    return NSString(intrinsics.objc_send(id, str, selector("initWithBytes:length:encoding:"), bytes, length, encoding))
}

NSString_fromString :: proc(s: string) -> NSString {
    if len(s) == 0 {
        class := get_class("NSString")
        return NSString(intrinsics.objc_send(id, class, selector("string")))
    }
    ns_str := NSString_alloc()
    return NSString_initWithBytes(ns_str, raw_data(s), uint(len(s)), NSUTF8StringEncoding)
}

NSString_UTF8String :: proc(str: NSString) -> cstring {
    return cstring(intrinsics.objc_send(rawptr, str, selector("UTF8String")))
}

NSString_toString :: proc(str: NSString) -> string {
    cstr := NSString_UTF8String(str)
    return string(cstr)
}

// ============================================================================
// NSColor
// ============================================================================

NSColor_systemRedColor :: proc() -> NSColor {
    class := get_class("NSColor")
    return NSColor(intrinsics.objc_send(id, class, selector("systemRedColor")))
}

NSColor_systemGreenColor :: proc() -> NSColor {
    class := get_class("NSColor")
    return NSColor(intrinsics.objc_send(id, class, selector("systemGreenColor")))
}

NSColor_systemGrayColor :: proc() -> NSColor {
    class := get_class("NSColor")
    return NSColor(intrinsics.objc_send(id, class, selector("systemGrayColor")))
}

// ============================================================================
// NSAttributedString
// ============================================================================

NSAttributedString_alloc :: proc() -> NSAttributedString {
    class := get_class("NSAttributedString")
    return NSAttributedString(intrinsics.objc_send(id, class, selector("alloc")))
}

NSAttributedString_initWithString :: proc(attr_str: NSAttributedString, str: NSString, attributes: NSDictionary) -> NSAttributedString {
    return NSAttributedString(intrinsics.objc_send(id, attr_str, selector("initWithString:attributes:"), str, attributes))
}

NSAttributedString_new :: proc(str: NSString, attributes: NSDictionary = nil) -> NSAttributedString {
    attr_str := NSAttributedString_alloc()
    return NSAttributedString_initWithString(attr_str, str, attributes)
}

// ============================================================================
// NSDictionary
// ============================================================================

NSDictionary_dictionaryWithObject :: proc(object: id, key: id) -> NSDictionary {
    class := get_class("NSDictionary")
    return NSDictionary(intrinsics.objc_send(id, class, selector("dictionaryWithObject:forKey:"), object, key))
}

// ============================================================================
// NSTimer
// ============================================================================

NSTimer_scheduledTimerWithTimeInterval :: proc(
    interval: f64,
    target: id,
    sel: SEL,
    userInfo: id,
    repeats: bool,
) -> NSTimer {
    class := get_class("NSTimer")
    return NSTimer(intrinsics.objc_send(
        id,
        class,
        selector("scheduledTimerWithTimeInterval:target:selector:userInfo:repeats:"),
        interval,
        target,
        sel,
        userInfo,
        repeats,
    ))
}

// ============================================================================
// NSObject (for custom classes)
// ============================================================================

NSObject_alloc :: proc() -> id {
    class := get_class("NSObject")
    return intrinsics.objc_send(id, class, selector("alloc"))
}

NSObject_init :: proc(obj: id) -> id {
    return intrinsics.objc_send(id, obj, selector("init"))
}

NSObject_class :: proc() -> Class {
    return get_class("NSObject")
}
