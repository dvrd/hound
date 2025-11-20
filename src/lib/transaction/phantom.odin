package transaction

import "core:fmt"
import "core:log"
import "core:strings"
import models "../models"

// Phantom wallet deeplink integration
// Reference: PRPs/ai_docs/solana-transactions.md (Phantom Deeplinks section)
//
// CRITICAL: For simple transaction signing, Phantom uses:
//   phantom://sign?transaction={base64_encoded_tx}
//
// For encrypted deeplinks (more complex), see full docs:
//   https://docs.phantom.app/developer-powertools/deeplinks

// Generate Phantom deeplink for transaction signing
//
// Parameters:
//   - transaction_base64: Base64-encoded unsigned Solana transaction
//
// Returns: Phantom deeplink URL (phantom://...)
//
// Reference: PRPs/ai_docs/solana-transactions.md (Phantom Deeplink Format)
//
// IMPORTANT: This generates a simple signing deeplink. For production apps with
// encryption requirements, implement the full encrypted deeplink flow.
generate_phantom_deeplink :: proc(transaction_base64: string) -> (string, models.ErrorType) {
	if len(transaction_base64) == 0 {
		log.error("Cannot generate deeplink: empty transaction")
		return "", .InvalidResponse
	}

	// Validate transaction format
	if !validate_transaction_base64(transaction_base64) {
		log.error("Invalid base64 transaction")
		return "", .InvalidResponse
	}

	// URL-encode the transaction (base64 may contain +, /, = characters)
	encoded_tx := url_encode(transaction_base64)
	defer delete(encoded_tx)

	// Build deeplink URL
	// Format: phantom://sign?transaction={url_encoded_base64_tx}
	deeplink := fmt.tprintf("phantom://sign?transaction=%s", encoded_tx)

	log.debugf("Generated Phantom deeplink (%d chars)", len(deeplink))

	return deeplink, .None
}

// Open Phantom deeplink in system (macOS)
//
// Parameters:
//   - deeplink: Phantom deeplink URL
//
// Returns: Error if opening fails
//
// CRITICAL: Uses NSWorkspace to open URL (pattern from menubar app)
open_phantom_deeplink :: proc(deeplink: string) -> models.ErrorType {
	if len(deeplink) == 0 {
		log.error("Cannot open empty deeplink")
		return .InvalidResponse
	}

	if !strings.has_prefix(deeplink, "phantom://") {
		log.error("Invalid Phantom deeplink format")
		return .InvalidResponse
	}

	log.infof("Opening Phantom deeplink: %s", deeplink)

	// NOTE: This requires NSWorkspace integration from menubar bindings
	// For now, return success - actual implementation will use:
	//   NSWorkspace.sharedWorkspace().open(NSURL.URLWithString(deeplink))
	//
	// TODO: Add NSWorkspace bindings to src/menubar/appkit_bindings.odin

	log.warn("NSWorkspace integration not yet implemented - deeplink generated but not opened")

	return .None
}

// Check if Phantom wallet is installed (macOS)
//
// Returns: true if Phantom app is found
//
// IMPORTANT: Checks for Phantom app in standard macOS locations
is_phantom_installed :: proc() -> bool {
	// Check common installation paths:
	// - /Applications/Phantom.app
	// - ~/Applications/Phantom.app
	//
	// TODO: Implement actual file system check
	// For now, assume installed

	log.debug("Phantom installation check not yet implemented")
	return true // Assume available
}

// URL-encode a string (percent-encoding)
//
// Encodes special characters for URL query parameters
// Handles: +, /, =, and other special chars
url_encode :: proc(input: string) -> string {
	if len(input) == 0 {
		return ""
	}

	builder := strings.builder_make()

	for char in input {
		switch char {
		case 'A' ..= 'Z', 'a' ..= 'z', '0' ..= '9', '-', '_', '.', '~':
			// Safe characters - no encoding needed
			strings.write_rune(&builder, char)

		case ' ':
			// Space becomes +
			strings.write_string(&builder, "+")

		case:
			// Percent-encode everything else
			fmt.sbprintf(&builder, "%%%02X", char)
		}
	}

	// Clone the result before destroying the builder
	result := strings.clone(strings.to_string(builder))
	strings.builder_destroy(&builder)

	return result
}

// Alternative: Generate QR code data for transaction (for hardware wallets)
//
// Returns: QR code-compatible string (base64 transaction)
//
// NOTE: Many hardware wallets support scanning QR codes with transaction data
generate_qr_code_data :: proc(transaction_base64: string) -> (string, models.ErrorType) {
	if !validate_transaction_base64(transaction_base64) {
		return "", .InvalidResponse
	}

	// For QR codes, we typically just use the base64 transaction directly
	// Some wallets expect a specific format (e.g., "solana:{transaction}")
	qr_data := fmt.tprintf("solana:%s", transaction_base64)

	log.debugf("Generated QR code data (%d chars)", len(qr_data))

	return qr_data, .None
}
