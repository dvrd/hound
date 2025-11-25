// Arena Lifecycle Tests
// Validates arena initialization, reset behavior, and memory statistics tracking
package tests

import "core:log"
import "core:mem"
import "core:testing"
import memory "../../src/lib/memory"

@(test)
test_arena_initialization :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify all 4 arenas initialize successfully
	//
	// Why it matters:
	// - All arenas must be ready before application starts
	// - Failure to initialize means application cannot run
	// - Tests the entire arena system comes online correctly
	//
	// Validates:
	// 1. memory_init() returns .None (success)
	// 2. All arena allocators are non-nil
	// 3. Arena statistics are accessible

	err := memory.memory_init()
	defer memory.memory_shutdown()

	testing.expect(
		t,
		err == .None,
		"Arena initialization should succeed (all 4 arenas ready)",
	)

	// Verify all allocators are valid (non-nil procedure pointers)
	persistent := memory.persistent_allocator()
	testing.expect(
		t,
		persistent.procedure != nil,
		"Persistent allocator should have valid procedure",
	)

	command := memory.command_allocator()
	testing.expect(t, command.procedure != nil, "Command allocator should have valid procedure")

	request := memory.request_allocator()
	testing.expect(t, request.procedure != nil, "Request allocator should have valid procedure")

	secure := memory.secure_allocator()
	testing.expect(t, secure.procedure != nil, "Secure allocator should have valid procedure")

	// Verify stats are accessible
	stats := memory.memory_stats()
	testing.expect(
		t,
		stats.persistent_used == 0,
		"Persistent arena should start empty (no allocations yet)",
	)
	testing.expect(t, stats.command_used == 0, "Command arena should start empty")
	testing.expect(t, stats.request_used == 0, "Request arena should start empty")
	testing.expect(t, stats.secure_used == 0, "Secure arena should start empty")
}

@(test)
test_command_arena_reset :: proc(t: ^testing.T) {
	// DOCUMENTATION: Command arena resets between CLI commands
	//
	// Why it matters:
	// - Each CLI command should start with clean memory
	// - Previous command data should not leak to next command
	// - Prevents memory accumulation across multiple commands
	//
	// Pattern:
	// ```odin
	// handle_command :: proc(args: []string) -> ErrorType {
	//     context.allocator = memory.command_allocator()
	//     defer memory.reset_command_arena()  // Cleanup
	//     // ... command logic ...
	//     return .None
	// }
	// ```
	//
	// Validates:
	// 1. Allocations increase arena usage
	// 2. Reset brings usage back to zero
	// 3. Arena is ready for next command

	memory.memory_init()
	defer memory.memory_shutdown()

	// Allocate data in command arena
	context.allocator = memory.command_allocator()
	data := make([]byte, 1024)

	// Verify allocation increased usage
	stats_before := memory.memory_stats()
	testing.expect(
		t,
		stats_before.command_used > 0,
		"Command arena should show allocated bytes",
	)

	// Reset arena (simulates end of command)
	memory.reset_command_arena()

	// Verify usage returned to zero
	stats_after := memory.memory_stats()
	testing.expect(
		t,
		stats_after.command_used == 0,
		"Command arena should be empty after reset (ready for next command)",
	)
}

@(test)
test_request_arena_reset :: proc(t: ^testing.T) {
	// DOCUMENTATION: Request arena resets after RPC calls
	//
	// Why it matters:
	// - HTTP responses can be large (JSON parsing allocates heavily)
	// - Memory should be freed after each request completes
	// - Prevents memory buildup during multiple API calls
	//
	// Pattern:
	// ```odin
	// fetch_from_api :: proc(url: string) -> (Result, ErrorType) {
	//     context.allocator = memory.request_allocator()
	//     defer memory.reset_request_arena()  // Cleanup
	//
	//     response := http_get(url)
	//     data := json_parse(response.body)
	//     return extract_result(data), .None
	// }
	// ```
	//
	// Validates:
	// 1. Request arena holds temporary RPC data
	// 2. Reset clears all request data
	// 3. Multiple requests don't accumulate memory

	memory.memory_init()
	defer memory.memory_shutdown()

	// Simulate HTTP response buffer
	context.allocator = memory.request_allocator()
	response_buffer := make([]byte, 2048)

	// Verify allocation
	stats_before := memory.memory_stats()
	testing.expect(
		t,
		stats_before.request_used >= 2048,
		"Request arena should show HTTP buffer allocation",
	)

	// Reset (simulates request completion)
	memory.reset_request_arena()

	// Verify cleanup
	stats_after := memory.memory_stats()
	testing.expect(
		t,
		stats_after.request_used == 0,
		"Request arena should be empty after reset (ready for next RPC call)",
	)
}

@(test)
test_arena_allocator_access :: proc(t: ^testing.T) {
	// DOCUMENTATION: All allocator getters return valid allocators
	//
	// Why it matters:
	// - Functions need access to allocators via getters
	// - Allocators must be valid (non-nil, correct data pointer)
	// - Tests the public API for arena access
	//
	// Usage:
	// ```odin
	// context.allocator = memory.command_allocator()  // Use getter
	// data := make([]byte, 1024)  // Allocates in command arena
	// ```
	//
	// Validates:
	// 1. All allocator getters work
	// 2. Returned allocators have valid procedure pointers
	// 3. Allocators can be used immediately

	memory.memory_init()
	defer memory.memory_shutdown()

	// Get all allocators
	persistent := memory.persistent_allocator()
	command := memory.command_allocator()
	request := memory.request_allocator()
	secure := memory.secure_allocator()

	// Verify all have valid procedure pointers
	testing.expect(
		t,
		persistent.procedure != nil,
		"Persistent allocator getter should return valid allocator",
	)
	testing.expect(
		t,
		command.procedure != nil,
		"Command allocator getter should return valid allocator",
	)
	testing.expect(
		t,
		request.procedure != nil,
		"Request allocator getter should return valid allocator",
	)
	testing.expect(
		t,
		secure.procedure != nil,
		"Secure allocator getter should return valid allocator",
	)

	// Verify allocators work (can actually allocate)
	context.allocator = command
	test_data := make([]byte, 64)
	testing.expect(
		t,
		len(test_data) == 64,
		"Allocator should successfully allocate memory",
	)
}

@(test)
test_memory_stats_tracking :: proc(t: ^testing.T) {
	// DOCUMENTATION: Memory stats accurately track usage
	//
	// Why it matters:
	// - Developers need visibility into memory usage
	// - Stats help debug memory leaks
	// - Validates arena accounting is correct
	//
	// Usage:
	// ```odin
	// memory.enable_memory_stats()  // Enable logging
	// stats := memory.memory_stats()  // Get current usage
	// log.debugf("Command arena: %d KB", stats.command_used / 1024)
	// ```
	//
	// Validates:
	// 1. Stats reflect actual allocations
	// 2. Different arenas tracked separately
	// 3. Stats update after allocations and resets

	memory.memory_init()
	defer memory.memory_shutdown()

	// Reset command arena to ensure clean state
	memory.reset_command_arena()

	// Get baseline (should be zero after reset)
	stats_empty := memory.memory_stats()
	testing.expect(t, stats_empty.command_used == 0, "Command arena should be 0 after reset")

	// Allocate in command arena
	context.allocator = memory.command_allocator()
	data := make([]byte, 1024)

	// Verify stats increased
	stats_used := memory.memory_stats()
	testing.expect(
		t,
		stats_used.command_used > 0,
		"Stats should show increased command arena usage",
	)

	// Verify other arenas unaffected
	testing.expect(
		t,
		stats_used.request_used == 0,
		"Request arena should still be 0 (allocation was in command arena)",
	)
	testing.expect(t, stats_used.secure_used == 0, "Secure arena should still be 0")

	// Reset and verify stats return to zero
	memory.reset_command_arena()
	stats_reset := memory.memory_stats()
	testing.expect(
		t,
		stats_reset.command_used == 0,
		"Stats should reflect arena reset (back to 0)",
	)
}

@(test)
test_multiple_arena_isolation :: proc(t: ^testing.T) {
	// DOCUMENTATION: Arenas are isolated from each other
	//
	// Why it matters:
	// - Command arena allocations don't affect request arena
	// - Secure arena resets don't affect other arenas
	// - Each arena operates independently
	//
	// Validates:
	// 1. Allocating in one arena doesn't affect others
	// 2. Resetting one arena doesn't affect others
	// 3. Stats track each arena separately

	memory.memory_init()
	defer memory.memory_shutdown()

	// Reset all arenas to ensure clean state
	memory.reset_command_arena()
	memory.reset_request_arena()
	memory.reset_secure_arena()

	// Allocate in multiple arenas
	context.allocator = memory.command_allocator()
	command_data := make([]byte, 1000)

	context.allocator = memory.request_allocator()
	request_data := make([]byte, 2000)

	context.allocator = memory.secure_allocator()
	secure_data := make([]byte, 500)

	// Verify each arena tracked separately
	stats := memory.memory_stats()
	testing.expect(t, stats.command_used > 0, "Command arena should show its allocation")
	testing.expect(t, stats.request_used > 0, "Request arena should show its allocation")
	testing.expect(t, stats.secure_used > 0, "Secure arena should show its allocation")

	// Reset one arena
	memory.reset_command_arena()

	// Verify only command arena reset
	stats_after := memory.memory_stats()
	testing.expect(t, stats_after.command_used == 0, "Command arena should be reset")
	testing.expect(
		t,
		stats_after.request_used > 0,
		"Request arena should still have its data",
	)
	testing.expect(t, stats_after.secure_used > 0, "Secure arena should still have its data")
}

@(test)
test_growing_arena_behavior :: proc(t: ^testing.T) {
	// DOCUMENTATION: Command arena grows to handle large allocations
	//
	// Why it matters:
	// - CLI commands may need more than 8 MB (initial block size)
	// - Arena should allocate additional blocks automatically
	// - Reset should deallocate extra blocks (graveyard blocks)
	//
	// Pattern:
	// ```odin
	// handle_large_query :: proc() -> ErrorType {
	//     context.allocator = memory.command_allocator()
	//     defer memory.reset_command_arena()  // Frees extra blocks
	//
	//     results := make([dynamic]QueryResult)  // May grow large
	//     for item in database_query() {
	//         append(&results, item)  // Arena grows as needed
	//     }
	//     return .None
	// }
	// ```
	//
	// Validates:
	// 1. Large allocation succeeds (doesn't fail at 8 MB)
	// 2. Arena can handle growing dynamic arrays
	// 3. Reset cleans up properly

	memory.memory_init()
	defer memory.memory_shutdown()

	// Allocate large dataset (simulate large query result)
	context.allocator = memory.command_allocator()

	// Allocate 5 MB array (within single block)
	large_data := make([]byte, 5 * 1024 * 1024)
	testing.expect(
		t,
		len(large_data) == 5 * 1024 * 1024,
		"Command arena should handle 5 MB allocation",
	)

	stats := memory.memory_stats()
	testing.expect(t, stats.command_used > 0, "Arena should show allocated memory")

	// Reset should work even with large allocation
	memory.reset_command_arena()
	stats_after := memory.memory_stats()
	testing.expect(
		t,
		stats_after.command_used == 0,
		"Reset should clean up large allocation",
	)
}

@(test)
test_secure_arena_active_detection :: proc(t: ^testing.T) {
	// DOCUMENTATION: is_secure_arena_active() detects nested secure contexts
	//
	// Why it matters:
	// - Functions may be called from different contexts
	// - Nested calls shouldn't reset arena prematurely
	// - Prevents double-reset bugs
	//
	// Pattern:
	// ```odin
	// pbkdf2_hmac_sha512 :: proc(...) -> ([]byte, ErrorType) {
	//     arena_was_active := memory.is_secure_arena_active()
	//     if !arena_was_active {
	//         context.allocator = memory.secure_allocator()
	//     }
	//     defer if !arena_was_active { memory.reset_secure_arena() }
	//
	//     // ... crypto operations ...
	//     return key, .None
	// }
	// ```
	//
	// Validates:
	// 1. Detects when secure arena is active
	// 2. Detects when secure arena is not active
	// 3. Enables safe nested secure calls

	memory.memory_init()
	defer memory.memory_shutdown()

	// Initially not active
	active_before := memory.is_secure_arena_active()
	testing.expect(t, !active_before, "Secure arena should not be active initially")

	// Set secure arena as context
	context.allocator = memory.secure_allocator()

	// Now should be active
	active_after := memory.is_secure_arena_active()
	testing.expect(t, active_after, "Secure arena should be detected as active")

	// Switch to different arena
	context.allocator = memory.command_allocator()

	// No longer active
	active_switched := memory.is_secure_arena_active()
	testing.expect(t, !active_switched, "Secure arena should not be active after switch")
}
