// Secure memory management for cryptographic operations
// Implements memory zeroing to prevent key leakage
package keystore

import "core:mem"
import "core:log"

// Securely zero memory to prevent key leakage
//
// ASSERTION 1: Pointer must not be nil
// ASSERTION 2: Size must be positive
secure_zero_memory :: proc(ptr: rawptr, size: int) {
	assert(ptr != nil, "Cannot zero nil pointer")
	assert(size > 0, "Size must be positive")

	mem.zero(ptr, size)
	log.debugf("Zeroed %d bytes of sensitive memory", size)
}

// Zero fixed-size byte array (keypair component)
//
// ASSERTION 1: Data must not be empty
zero_bytes :: proc(data: []byte) {
	assert(len(data) > 0, "Cannot zero empty byte array")

	secure_zero_memory(raw_data(data), len(data))
}
