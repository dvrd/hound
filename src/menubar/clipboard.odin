package menubar

import "core:fmt"
import "core:log"

// Clipboard operations using NSPasteboard
// Reference: PRPs/ai_docs/menubar-ui-patterns.md (Clipboard section)

// Copy text to system clipboard
//
// Parameters:
//   - text: String to copy to clipboard
//
// Returns: true if successful, false otherwise
//
// Uses NSPasteboard.generalPasteboard() for system clipboard
copy_to_clipboard :: proc(text: string) -> bool {
	if len(text) == 0 {
		log.error("Cannot copy empty string to clipboard")
		return false
	}

	// Get system clipboard
	pasteboard := NSPasteboard_generalPasteboard()
	if pasteboard == nil {
		log.error("Failed to access system clipboard")
		return false
	}

	// Clear existing clipboard contents
	NSPasteboard_clearContents(pasteboard)

	// Convert string to NSString
	ns_text := NSString_fromString(text)

	// Set string with type NSPasteboardTypeString
	success := NSPasteboard_setString(pasteboard, ns_text, NSPasteboardTypeString)

	if success {
		log.infof("Copied to clipboard: %d characters", len(text))
	} else {
		log.error("Failed to copy text to clipboard")
	}

	return success
}

// Read text from system clipboard
//
// Returns: (clipboard_text, true) if successful, ("", false) if failed
//
// Useful for paste operations (though not needed for current swap feature)
paste_from_clipboard :: proc() -> (string, bool) {
	// Get system clipboard
	pasteboard := NSPasteboard_generalPasteboard()
	if pasteboard == nil {
		log.error("Failed to access system clipboard")
		return "", false
	}

	// Read string content
	ns_text := NSPasteboard_stringForType(pasteboard, NSPasteboardTypeString)
	if ns_text == nil {
		log.debug("Clipboard is empty or contains non-text data")
		return "", false
	}

	// Convert NSString to Odin string
	text := NSString_toString(ns_text)
	log.debugf("Read from clipboard: %d characters", len(text))

	return text, true
}

// Check if clipboard contains text
//
// Returns: true if clipboard has text content
has_clipboard_text :: proc() -> bool {
	pasteboard := NSPasteboard_generalPasteboard()
	if pasteboard == nil {
		return false
	}

	ns_text := NSPasteboard_stringForType(pasteboard, NSPasteboardTypeString)
	return ns_text != nil
}

// Clear clipboard contents
//
// Returns: true if successful
clear_clipboard :: proc() -> bool {
	pasteboard := NSPasteboard_generalPasteboard()
	if pasteboard == nil {
		log.error("Failed to access system clipboard")
		return false
	}

	NSPasteboard_clearContents(pasteboard)
	log.debug("Clipboard cleared")

	return true
}
