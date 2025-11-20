// User-facing message output
// Progress indicators, success/error messages, and status updates
package output

import "core:fmt"

// ============================================================================
// Progress & Status Messages
// ============================================================================

// print_progress displays a progress message to stderr
//
// Used for: Pool discovery, price fetching, token operations
// Pattern: Simple message without emoji prefix
print_progress :: proc(message: string) {
	fmt.eprintln(message)
}

// print_success displays a success message to stderr
//
// Used for: Successful token additions, pool discoveries
// Pattern: ✓ prefix indicates successful operation
print_success :: proc(message: string) {
	fmt.eprintfln("✓ %s", message)
}

// print_warning displays a warning message to stderr
//
// Used for: Fallback operations, non-critical issues
// Pattern: ⚠ prefix indicates caution
print_warning :: proc(message: string) {
	fmt.eprintfln("⚠ %s", message)
}

// print_error displays an error message to stderr
//
// Used for: Operation failures before detailed error display
// Pattern: Simple "Error:" prefix
print_error :: proc(message: string) {
	fmt.eprintfln("Error: %s", message)
}

// print_info displays an informational message to stderr
//
// Used for: General information, status updates
// Pattern: No prefix, clean message
print_info :: proc(message: string) {
	fmt.eprintln(message)
}
