package transaction

import "core:encoding/base64"
import "core:log"
import models "../../core/models"

// Transaction serialization utilities
// Reference: PRPs/ai_docs/solana-transactions.md

// Jupiter v6 returns transactions already base64-encoded
// These utilities provide validation and format conversion

// Validate that a string is valid base64-encoded transaction data
//
// Parameters:
//   - transaction_base64: Base64-encoded transaction string
//
// Returns: true if valid base64, false otherwise
//
// Reference: PRPs/ai_docs/solana-transactions.md (Base64 Encoding section)
validate_transaction_base64 :: proc(transaction_base64: string) -> bool {
	if len(transaction_base64) == 0 {
		log.error("Transaction string is empty")
		return false
	}

	// Try to decode base64 to verify validity
	decoded, alloc_err := base64.decode(transaction_base64)
	if alloc_err != .None {
		log.errorf("Failed to decode base64 transaction: %v", alloc_err)
		return false
	}
	defer delete(decoded)

	// Verify transaction size constraints
	// Solana transactions have max size of 1232 bytes
	// Reference: PRPs/ai_docs/solana-transactions.md (Transaction Structure)
	if len(decoded) > 1232 {
		log.errorf("Transaction too large: %d bytes (max 1232)", len(decoded))
		return false
	}

	if len(decoded) < 32 {
		log.error("Transaction too small to be valid")
		return false
	}

	log.debugf("Transaction validated: %d bytes", len(decoded))
	return true
}

// Decode base64 transaction to raw bytes
//
// Parameters:
//   - transaction_base64: Base64-encoded transaction
//
// Returns: Raw transaction bytes, or error
//
// NOTE: Caller is responsible for freeing the returned byte slice
decode_transaction :: proc(transaction_base64: string) -> ([]u8, models.ErrorType) {
	if len(transaction_base64) == 0 {
		return nil, .InvalidResponse
	}

	decoded, alloc_err := base64.decode(transaction_base64)
	if alloc_err != .None {
		log.errorf("Failed to decode transaction: %v", alloc_err)
		return nil, .InvalidResponse
	}

	// Validate size
	if len(decoded) > 1232 {
		log.error("Transaction exceeds maximum size")
		delete(decoded)
		return nil, .InvalidResponse
	}

	return decoded, .None
}

// Encode raw transaction bytes to base64 string
//
// Parameters:
//   - transaction_bytes: Raw Solana transaction bytes
//
// Returns: Base64-encoded string, or error
//
// NOTE: Caller is responsible for freeing the returned string
encode_transaction_base64 :: proc(transaction_bytes: []u8) -> (string, models.ErrorType) {
	if len(transaction_bytes) == 0 {
		return "", .InvalidResponse
	}

	if len(transaction_bytes) > 1232 {
		log.error("Transaction exceeds maximum size of 1232 bytes")
		return "", .InvalidResponse
	}

	encoded := base64.encode(transaction_bytes)
	log.debugf("Encoded transaction to base64: %d bytes → %d chars", len(transaction_bytes), len(encoded))

	return encoded, .None
}

// Get transaction summary info (useful for logging/display)
TransactionInfo :: struct {
	size_bytes:       int,
	size_base64:      int,
	is_valid:         bool,
	exceeds_max_size: bool,
}

get_transaction_info :: proc(transaction_base64: string) -> TransactionInfo {
	info := TransactionInfo {
		size_base64 = len(transaction_base64),
		is_valid    = false,
	}

	// Try to decode to get actual size
	decoded, alloc_err := base64.decode(transaction_base64)
	if alloc_err != .None {
		return info
	}
	defer delete(decoded)

	info.size_bytes = len(decoded)
	info.exceeds_max_size = len(decoded) > 1232
	info.is_valid = !info.exceeds_max_size && len(decoded) >= 32

	return info
}
