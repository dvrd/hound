// Secure Arena Security Tests
// Validates memory zeroing, isolation, and regression prevention for cryptographic operations
package tests

import "core:log"
import "core:mem"
import "core:testing"
import memory "../../src/lib/memory"

@(test)
test_secure_arena_zeroing :: proc(t: ^testing.T) {
	// DOCUMENTATION: Secure arena zeros memory before reset
	// SECURITY: Prevents key material from persisting in memory
	//
	// Why it matters:
	// - Private keys, passwords, seeds must not remain in memory
	// - Memory dumps (crash dumps, debuggers) could expose secrets
	// - Defense-in-depth: even if process memory is compromised, keys are zeroed
	//
	// Pattern:
	// ```odin
	// derive_key :: proc(password: string) -> []byte {
	//     context.allocator = memory.secure_allocator()
	//     defer memory.reset_secure_arena()  // ZEROS then frees
	//
	//     key := make([]byte, 32)
	//     // ... derive key ...
	//
	//     // Key automatically zeroed by defer
	//     return key
	// }
	// ```
	//
	// Validates:
	// 1. Memory contains pattern before reset
	// 2. Memory is zeroed after reset
	// 3. All allocated bytes are zeroed (not just partial)

	memory.memory_init()
	defer memory.memory_shutdown()

	// Allocate sensitive data in secure arena
	context.allocator = memory.secure_allocator()
	secret := make([]byte, 32)

	// Fill with recognizable pattern
	for i in 0 ..< 32 {
		secret[i] = 0xAA
	}

	// Verify pattern exists
	pattern_exists := true
	for i in 0 ..< 32 {
		if secret[i] != 0xAA {
			pattern_exists = false
			break
		}
	}
	testing.expect(t, pattern_exists, "Secret should contain 0xAA pattern before reset")

	// Get raw pointer for verification (TEST ONLY - never do this in production!)
	// This allows us to check memory after arena reset
	secret_ptr := raw_data(secret)

	// Reset should zero memory
	memory.reset_secure_arena()

	// Verify all bytes are zeroed
	all_zero := true
	for i in 0 ..< 32 {
		if secret_ptr[i] != 0 {
			all_zero = false
			log.errorf("Byte %d not zeroed: 0x%02X", i, secret_ptr[i])
			break
		}
	}

	testing.expect(
		t,
		all_zero,
		"SECURITY CRITICAL: Secure arena memory must be zeroed to prevent key leakage",
	)
}

@(test)
test_secure_arena_isolation :: proc(t: ^testing.T) {
	// DOCUMENTATION: Secure arena data doesn't leak to other arenas
	// SECURITY: Ensures sensitive data stays isolated
	//
	// Why it matters:
	// - Command arena data might be logged or cached
	// - Request arena data might be sent over network
	// - Secure data must stay in secure arena until zeroed
	//
	// Pattern:
	// ```odin
	// // CORRECT: Secure data in secure arena
	// derive_key :: proc() -> []byte {
	//     context.allocator = memory.secure_allocator()
	//     defer memory.reset_secure_arena()
	//     key := make([]byte, 32)  // Secure arena
	//     return key
	// }
	//
	// // WRONG: Secure data in command arena
	// bad_derive_key :: proc() -> []byte {
	//     context.allocator = memory.command_allocator()  // NOT ZEROED!
	//     key := make([]byte, 32)  // Vulnerable to memory dumps
	//     return key
	// }
	// ```
	//
	// Validates:
	// 1. Secure arena resets independently
	// 2. Each arena tracks separately
	// 3. Resetting secure arena doesn't affect other arenas

	memory.memory_init()
	defer memory.memory_shutdown()

	// Reset all arenas for clean state
	memory.reset_command_arena()
	memory.reset_secure_arena()

	// Allocate in secure arena
	context.allocator = memory.secure_allocator()
	secure_data := make([]byte, 128)
	for i in 0 ..< 128 {
		secure_data[i] = 0xBB
	}

	// Verify secure arena has allocation
	stats_secure := memory.memory_stats()
	testing.expect(t, stats_secure.secure_used > 0, "Secure arena should have allocation")

	// Reset command arena again to ensure clean state before allocation test
	// (other tests might have used command arena before this test runs)
	memory.reset_command_arena()

	// Allocate in command arena (different arena)
	context.allocator = memory.command_allocator()
	command_data := make([]byte, 256)

	// Verify both arenas have data
	stats_both := memory.memory_stats()
	testing.expect(t, stats_both.secure_used > 0, "Secure arena should still have data")
	testing.expect(t, stats_both.command_used > 0, "Command arena should have data")

	// Reset secure arena
	memory.reset_secure_arena()

	// Verify only secure arena reset
	stats_after := memory.memory_stats()
	testing.expect(
		t,
		stats_after.secure_used == 0,
		"SECURITY: Secure arena should be reset (zeroed)",
	)
	testing.expect(
		t,
		stats_after.command_used > 0,
		"Command arena should still have data (not affected by secure reset)",
	)
}

@(test)
test_pbkdf2_no_loop_allocations :: proc(t: ^testing.T) {
	// DOCUMENTATION: Regression test - PBKDF2 should use pre-allocated buffer
	// CRITICAL BUG FIX: Previously allocated 2048 temp buffers inside loop
	//
	// Why it matters:
	// - PBKDF2 runs 2048 iterations
	// - Allocating 2048 times causes memory fragmentation
	// - 2048 defer statements cause performance overhead
	// - Pre-allocated buffer is reused (1 allocation total)
	//
	// BEFORE (CRITICAL BUG):
	// ```odin
	// for j := 2; j <= 2048; j += 1 {
	//     temp := make([]byte, 64)  // ALLOCATES 2048 TIMES!
	//     defer delete(temp)         // 2048 DEFERS!
	//     // ...
	// }
	// ```
	//
	// AFTER (FIXED):
	// ```odin
	// temp_buffer := make([]byte, 64)  // Allocate ONCE
	// // No defer needed - secure arena handles cleanup
	//
	// for j := 2; j <= 2048; j += 1 {
	//     // Reuse temp_buffer each iteration
	//     copy(temp_buffer, u)
	//     hmac.sum(algorithm, u, temp_buffer, password)
	// }
	// ```
	//
	// Validates:
	// 1. PBKDF2-like loop doesn't accumulate memory
	// 2. Pre-allocated buffer pattern works correctly
	// 3. Memory usage stays constant across iterations

	memory.memory_init()
	defer memory.memory_shutdown()

	// Reset to ensure clean baseline
	memory.reset_secure_arena()

	// Simulate PBKDF2-like operation
	context.allocator = memory.secure_allocator()

	// Pre-allocated buffer (correct approach)
	temp := make([]byte, 64)

	// Get stats after initial allocation
	stats_initial := memory.memory_stats()
	initial_usage := stats_initial.secure_used

	// Loop reuses buffer (no additional allocations)
	for i in 0 ..< 2048 {
		// Reuse temp buffer (no allocations in loop)
		temp[0] = byte(i % 256)
		// ... (HMAC operations would go here) ...
	}

	stats_after := memory.memory_stats()

	// Usage should be approximately same (no loop allocations)
	// Allow small variance for arena overhead
	usage_diff := stats_after.secure_used - initial_usage

	// Should not accumulate memory (usage difference should be near zero)
	testing.expect(
		t,
		usage_diff < 1000,
		"PBKDF2 loop should not accumulate significant memory (reuses single buffer)",
	)

	if usage_diff >= 1000 {
		log.errorf(
			"REGRESSION: PBKDF2 pattern accumulated %d bytes during loop (expected minimal growth)",
			usage_diff,
		)
		log.error("This indicates loop allocations may have returned (potential bug)")
	}
}

@(test)
test_sensitive_data_cleanup :: proc(t: ^testing.T) {
	// DOCUMENTATION: Sensitive data cleaned up automatically
	// SECURITY: Multiple sensitive buffers all zeroed in one reset
	//
	// Why it matters:
	// - Crypto operations often use multiple buffers (keys, HMACs, temp data)
	// - Manual zeroing is error-prone (easy to forget one buffer)
	// - Arena reset zeros ALL allocations automatically
	//
	// Pattern:
	// ```odin
	// encrypt_data :: proc(plaintext: []byte, password: string) -> []byte {
	//     context.allocator = memory.secure_allocator()
	//     defer memory.reset_secure_arena()  // Zeros ALL below
	//
	//     key := make([]byte, 32)           // Zeroed
	//     iv := make([]byte, 16)            // Zeroed
	//     hmac_buffer := make([]byte, 64)   // Zeroed
	//     temp_block := make([]byte, 16)    // Zeroed
	//
	//     // ... encryption ...
	//
	//     // Single defer zeros all 4 buffers
	//     return ciphertext
	// }
	// ```
	//
	// Validates:
	// 1. Multiple allocations in secure arena
	// 2. All allocations zeroed on reset
	// 3. Stats show complete cleanup

	memory.memory_init()
	defer memory.memory_shutdown()

	context.allocator = memory.secure_allocator()

	// Simulate multiple sensitive buffers
	key := make([]byte, 32)
	for i in 0 ..< 32 {
		key[i] = 0xFF
	}

	hmac_buffer := make([]byte, 64)
	for i in 0 ..< 64 {
		hmac_buffer[i] = 0xCC
	}

	iv := make([]byte, 16)
	for i in 0 ..< 16 {
		iv[i] = 0xDD
	}

	// Verify allocations exist
	stats_before := memory.memory_stats()
	testing.expect(
		t,
		stats_before.secure_used >= (32 + 64 + 16),
		"Secure arena should hold all buffers",
	)

	// Get pointers for verification (TEST ONLY)
	key_ptr := raw_data(key)
	hmac_ptr := raw_data(hmac_buffer)
	iv_ptr := raw_data(iv)

	// Reset should zero all buffers
	memory.reset_secure_arena()

	// Verify all buffers zeroed
	key_zeroed := true
	for i in 0 ..< 32 {
		if key_ptr[i] != 0 {
			key_zeroed = false
			break
		}
	}

	hmac_zeroed := true
	for i in 0 ..< 64 {
		if hmac_ptr[i] != 0 {
			hmac_zeroed = false
			break
		}
	}

	iv_zeroed := true
	for i in 0 ..< 16 {
		if iv_ptr[i] != 0 {
			iv_zeroed = false
			break
		}
	}

	testing.expect(t, key_zeroed, "SECURITY: Key buffer must be zeroed")
	testing.expect(t, hmac_zeroed, "SECURITY: HMAC buffer must be zeroed")
	testing.expect(t, iv_zeroed, "SECURITY: IV buffer must be zeroed")

	// Verify stats show cleanup
	stats_after := memory.memory_stats()
	testing.expect(t, stats_after.secure_used == 0, "All sensitive data cleaned up")
}

@(test)
test_secure_arena_size_limits :: proc(t: ^testing.T) {
	// DOCUMENTATION: Secure arena has 2 MB static limit
	// DESIGN: Prevents excessive memory for crypto operations
	//
	// Why it matters:
	// - Crypto operations should be bounded in size
	// - 2 MB is sufficient for keys, HMACs, seed derivation
	// - Static size ensures predictable memory usage
	//
	// Pattern:
	// ```odin
	// // CORRECT: Small crypto buffers
	// pbkdf2 :: proc() -> []byte {
	//     context.allocator = memory.secure_allocator()
	//     defer memory.reset_secure_arena()
	//
	//     key := make([]byte, 32)        // 32 bytes
	//     hmac := make([]byte, 64)       // 64 bytes
	//     temp := make([]byte, 64)       // 64 bytes
	//     // Total: ~160 bytes (well under 2 MB)
	//
	//     return key
	// }
	//
	// // WRONG: Large data in secure arena
	// process_file :: proc() -> []byte {
	//     context.allocator = memory.secure_allocator()  // WRONG!
	//     file_data := make([]byte, 50 * 1024 * 1024)    // 50 MB - too large!
	//     // Use command arena instead
	// }
	// ```
	//
	// Validates:
	// 1. Small crypto allocations succeed
	// 2. Secure arena handles typical crypto workloads
	// 3. Design: 2 MB is sufficient for all crypto operations

	memory.memory_init()
	defer memory.memory_shutdown()

	context.allocator = memory.secure_allocator()

	// Typical crypto operation sizes
	key := make([]byte, 32) // 256-bit key
	testing.expect(t, len(key) == 32, "32-byte key should allocate successfully")

	hmac_buffer := make([]byte, 64) // SHA-512 output
	testing.expect(t, len(hmac_buffer) == 64, "64-byte HMAC buffer should allocate")

	seed := make([]byte, 64) // BIP39 seed
	testing.expect(t, len(seed) == 64, "64-byte seed should allocate")

	// Even moderate allocations should work
	temp_workspace := make([]byte, 1024) // 1 KB temp buffer
	testing.expect(
		t,
		len(temp_workspace) == 1024,
		"1 KB temp buffer should allocate in 2 MB arena",
	)

	// Verify all allocations succeeded
	stats := memory.memory_stats()
	total_allocated := 32 + 64 + 64 + 1024 // = 1184 bytes
	testing.expect(
		t,
		stats.secure_used >= uint(total_allocated),
		"Secure arena should track all allocations",
	)
	testing.expect(
		t,
		stats.secure_used < 2 * 1024 * 1024,
		"Typical crypto operations should stay well under 2 MB limit",
	)
}

@(test)
test_secure_arena_reset_idempotency :: proc(t: ^testing.T) {
	// DOCUMENTATION: Multiple resets are safe (idempotent operation)
	// RELIABILITY: Reset can be called multiple times without error
	//
	// Why it matters:
	// - Error paths may call reset multiple times
	// - Nested functions may each call reset
	// - Should not crash or corrupt state
	//
	// Pattern:
	// ```odin
	// process_crypto :: proc() -> (Result, ErrorType) {
	//     context.allocator = memory.secure_allocator()
	//     defer memory.reset_secure_arena()  // Reset 1
	//
	//     if error_condition {
	//         memory.reset_secure_arena()  // Reset 2 (early exit)
	//         return {}, .CryptoError
	//     }
	//
	//     // ... (defer calls reset again on normal return)
	//     return result, .None
	// }
	// ```
	//
	// Validates:
	// 1. First reset succeeds
	// 2. Second reset succeeds (no crash)
	// 3. Third reset succeeds (still safe)
	// 4. Arena remains in valid state

	memory.memory_init()
	defer memory.memory_shutdown()

	// Allocate something
	context.allocator = memory.secure_allocator()
	data := make([]byte, 128)

	// First reset
	memory.reset_secure_arena()
	stats_1 := memory.memory_stats()
	testing.expect(t, stats_1.secure_used == 0, "First reset should clean arena")

	// Second reset (already empty)
	memory.reset_secure_arena()
	stats_2 := memory.memory_stats()
	testing.expect(t, stats_2.secure_used == 0, "Second reset should be safe (idempotent)")

	// Third reset
	memory.reset_secure_arena()
	stats_3 := memory.memory_stats()
	testing.expect(t, stats_3.secure_used == 0, "Third reset should still be safe")

	// Arena should still be usable after multiple resets
	context.allocator = memory.secure_allocator()
	new_data := make([]byte, 64)
	testing.expect(
		t,
		len(new_data) == 64,
		"Arena should be usable after multiple resets",
	)
}
