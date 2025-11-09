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
    class_createInstance :: proc(cls: Class, extraBytes: uint) -> id ---
    object_getIndexedIvars :: proc(obj: id) -> rawptr ---
}

// Core Objective-C types
id       :: ^intrinsics.objc_object
Class    :: ^intrinsics.objc_class
SEL      :: ^intrinsics.objc_selector
IMP      :: rawptr
CGFloat  :: f64

// Helper for msgSend
msgSend :: intrinsics.objc_send

// Common constants
NSVariableStatusItemLength :: CGFloat(-2.0)

// Activation policies for NSApplication
NSApplicationActivationPolicy :: enum i64 {
    Regular   = 0,
    Accessory = 1,
    Prohibited = 2,
}

// Base object type
Object :: struct {using _: intrinsics.objc_object}

// NSObject - the root class
@(objc_class="NSObject")
NSObject :: struct {using _: Object}

// Opaque types with objc_class attribute
@(objc_class="NSApplication")
NSApplication :: struct {using _: Object}

@(objc_class="NSStatusBar")
NSStatusBar :: struct {using _: Object}

@(objc_class="NSStatusItem")
NSStatusItem :: struct {using _: Object}

@(objc_class="NSMenu")
NSMenu :: struct {using _: Object}

@(objc_class="NSMenuItem")
NSMenuItem :: struct {using _: Object}

@(objc_class="NSButton")
NSButton :: struct {using _: Object}

@(objc_class="NSString")
NSString :: struct {using _: Object}

@(objc_class="NSColor")
NSColor :: struct {using _: Object}

@(objc_class="NSAttributedString")
NSAttributedString :: struct {using _: Object}

@(objc_class="NSDictionary")
NSDictionary :: struct {using _: Object}

@(objc_class="NSTimer")
NSTimer :: struct {using _: Object}

@(objc_class="NSNotification")
NSNotification :: struct {using _: Object}

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

NSApplication_sharedApplication :: proc() -> ^NSApplication {
    return msgSend(^NSApplication, NSApplication, "sharedApplication")
}

NSApplication_setActivationPolicy :: proc(app: ^NSApplication, policy: NSApplicationActivationPolicy) {
    msgSend(nil, app, "setActivationPolicy:", policy)
}

NSApplication_run :: proc(app: ^NSApplication) {
    msgSend(nil, app, "run")
}

NSApplication_terminate :: proc(app: ^NSApplication, sender: id = nil) {
    msgSend(nil, app, "terminate:", sender)
}

NSApplication_setDelegate :: proc(app: ^NSApplication, delegate: id) {
    msgSend(nil, app, "setDelegate:", delegate)
}

// ============================================================================
// NSStatusBar
// ============================================================================

NSStatusBar_systemStatusBar :: proc() -> ^NSStatusBar {
    return msgSend(^NSStatusBar, NSStatusBar, "systemStatusBar")
}

NSStatusBar_statusItemWithLength :: proc(bar: ^NSStatusBar, length: CGFloat) -> ^NSStatusItem {
    return msgSend(^NSStatusItem, bar, "statusItemWithLength:", length)
}

// ============================================================================
// NSStatusItem
// ============================================================================

NSStatusItem_button :: proc(item: ^NSStatusItem) -> ^NSButton {
    return msgSend(^NSButton, item, "button")
}

NSStatusItem_setMenu :: proc(item: ^NSStatusItem, menu: ^NSMenu) {
    msgSend(nil, item, "setMenu:", menu)
}

// ============================================================================
// NSButton
// ============================================================================

NSButton_setTitle :: proc(button: ^NSButton, title: ^NSString) {
    msgSend(nil, button, "setTitle:", title)
}

NSButton_setAttributedTitle :: proc(button: ^NSButton, title: ^NSAttributedString) {
    msgSend(nil, button, "setAttributedTitle:", title)
}

// ============================================================================
// NSMenu
// ============================================================================

NSMenu_alloc :: proc() -> ^NSMenu {
    return msgSend(^NSMenu, NSMenu, "alloc")
}

NSMenu_init :: proc(menu: ^NSMenu) -> ^NSMenu {
    return msgSend(^NSMenu, menu, "init")
}

NSMenu_new :: proc() -> ^NSMenu {
    menu := NSMenu_alloc()
    return NSMenu_init(menu)
}

NSMenu_addItem :: proc(menu: ^NSMenu, item: ^NSMenuItem) {
    msgSend(nil, menu, "addItem:", item)
}

// ============================================================================
// NSMenuItem
// ============================================================================

NSMenuItem_alloc :: proc() -> ^NSMenuItem {
    return msgSend(^NSMenuItem, NSMenuItem, "alloc")
}

NSMenuItem_initWithTitle :: proc(item: ^NSMenuItem, title: ^NSString, action: SEL, keyEquivalent: ^NSString) -> ^NSMenuItem {
    return msgSend(^NSMenuItem, item, "initWithTitle:action:keyEquivalent:", title, action, keyEquivalent)
}

NSMenuItem_new :: proc(title: ^NSString, action: SEL = nil, keyEquivalent: ^NSString = nil) -> ^NSMenuItem {
    item := NSMenuItem_alloc()
    key_equiv := keyEquivalent
    if key_equiv == nil {
        key_equiv = NSString_fromString("")
    }
    return NSMenuItem_initWithTitle(item, title, action, key_equiv)
}

NSMenuItem_separatorItem :: proc() -> ^NSMenuItem {
    return msgSend(^NSMenuItem, NSMenuItem, "separatorItem")
}

NSMenuItem_setTarget :: proc(item: ^NSMenuItem, target: id) {
    msgSend(nil, item, "setTarget:", target)
}

// ============================================================================
// NSString
// ============================================================================

NSUTF8StringEncoding :: uint(4)

NSString_alloc :: proc() -> ^NSString {
    return msgSend(^NSString, NSString, "alloc")
}

NSString_initWithBytes :: proc(str: ^NSString, bytes: rawptr, length: uint, encoding: uint) -> ^NSString {
    return msgSend(^NSString, str, "initWithBytes:length:encoding:", bytes, length, encoding)
}

NSString_fromString :: proc(s: string) -> ^NSString {
    if len(s) == 0 {
        return msgSend(^NSString, NSString, "string")
    }
    ns_str := NSString_alloc()
    return NSString_initWithBytes(ns_str, raw_data(s), uint(len(s)), NSUTF8StringEncoding)
}

NSString_UTF8String :: proc(str: ^NSString) -> cstring {
    return msgSend(cstring, str, "UTF8String")
}

NSString_toString :: proc(str: ^NSString) -> string {
    cstr := NSString_UTF8String(str)
    return string(cstr)
}

// ============================================================================
// NSColor
// ============================================================================

NSColor_systemRedColor :: proc() -> ^NSColor {
    return msgSend(^NSColor, NSColor, "systemRedColor")
}

NSColor_systemGreenColor :: proc() -> ^NSColor {
    return msgSend(^NSColor, NSColor, "systemGreenColor")
}

NSColor_systemGrayColor :: proc() -> ^NSColor {
    return msgSend(^NSColor, NSColor, "systemGrayColor")
}

// ============================================================================
// NSAttributedString
// ============================================================================

NSAttributedString_alloc :: proc() -> ^NSAttributedString {
    return msgSend(^NSAttributedString, NSAttributedString, "alloc")
}

NSAttributedString_initWithString :: proc(attr_str: ^NSAttributedString, str: ^NSString, attributes: ^NSDictionary) -> ^NSAttributedString {
    return msgSend(^NSAttributedString, attr_str, "initWithString:attributes:", str, attributes)
}

NSAttributedString_new :: proc(str: ^NSString, attributes: ^NSDictionary = nil) -> ^NSAttributedString {
    attr_str := NSAttributedString_alloc()
    return NSAttributedString_initWithString(attr_str, str, attributes)
}

// ============================================================================
// NSDictionary
// ============================================================================

NSDictionary_dictionaryWithObject :: proc(object: id, key: id) -> ^NSDictionary {
    return msgSend(^NSDictionary, NSDictionary, "dictionaryWithObject:forKey:", object, key)
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
) -> ^NSTimer {
    return msgSend(
        ^NSTimer,
        NSTimer,
        "scheduledTimerWithTimeInterval:target:selector:userInfo:repeats:",
        interval,
        target,
        sel,
        userInfo,
        repeats,
    )
}

// ============================================================================
// NSObject (for custom classes)
// ============================================================================

NSObject_alloc :: proc() -> ^NSObject {
    return msgSend(^NSObject, NSObject, "alloc")
}

NSObject_init :: proc(obj: ^NSObject) -> ^NSObject {
    return msgSend(^NSObject, obj, "init")
}

NSObject_class :: proc() -> Class {
    return intrinsics.objc_find_class("NSObject")
}
