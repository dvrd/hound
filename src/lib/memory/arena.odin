#+feature global-context
package memory

import "core:fmt"
import "core:log"
import "core:mem"
import vmem "core:mem/virtual"
import "../models"

// Global arenas (package-level, initialized by memory_init)
g_persistent_arena: vmem.Arena
g_command_arena: vmem.Arena
g_request_arena: vmem.Arena
g_secure_arena: vmem.Arena  // For cryptographic operations requiring memory zeroing
g_memory_stats_enabled: bool

// Memory statistics structure
MemoryStats :: struct {
	persistent_used: uint,
	command_used:    uint,
	request_used:    uint,
	secure_used:     uint,
}

// Initialize all memory arenas
// MUST be called after logger initialization in main()
// Returns .None on success, .DatabaseError on failure
memory_init :: proc() -> models.ErrorType {
	log.debug("Initializing memory arenas")

	// 1. Persistent Arena (10 MB static, program lifetime)
	// CRITICAL: Static arena reserves virtual memory, commits on-demand
	persistent_err := vmem.arena_init_static(&g_persistent_arena, 10 * mem.Megabyte)
	if persistent_err != nil {
		log.errorf("Failed to initialize persistent arena: %v", persistent_err)
		return .DatabaseError
	}
	log.debug("Persistent arena initialized (10 MB)")

	// 2. Command Arena (50 MB growing, per-command lifecycle)
	// PATTERN: Use default growing minimum block size (8 MB)
	command_err := vmem.arena_init_growing(&g_command_arena)
	if command_err != nil {
		log.errorf("Failed to initialize command arena: %v", command_err)
		vmem.arena_destroy(&g_persistent_arena)  // Cleanup on failure
		return .DatabaseError
	}
	log.debug("Command arena initialized (growing, 50 MB max)")

	// 3. Request Arena (5 MB static, per-RPC lifecycle)
	request_err := vmem.arena_init_static(&g_request_arena, 5 * mem.Megabyte)
	if request_err != nil {
		log.errorf("Failed to initialize request arena: %v", request_err)
		vmem.arena_destroy(&g_persistent_arena)
		vmem.arena_destroy(&g_command_arena)
		return .DatabaseError
	}
	log.debug("Request arena initialized (5 MB)")

	// 4. Secure Arena (2 MB static, for cryptographic operations)
	// SECURITY: This arena is explicitly zeroed before reset to protect sensitive data
	secure_err := vmem.arena_init_static(&g_secure_arena, 2 * mem.Megabyte)
	if secure_err != nil {
		log.errorf("Failed to initialize secure arena: %v", secure_err)
		vmem.arena_destroy(&g_persistent_arena)
		vmem.arena_destroy(&g_command_arena)
		vmem.arena_destroy(&g_request_arena)
		return .DatabaseError
	}
	log.debug("Secure arena initialized (2 MB)")

	log.debug("Memory arena system initialized successfully")
	return .None
}

// Cleanup all memory arenas
// MUST be called with defer after memory_init()
memory_shutdown :: proc() {
	log.debug("Shutting down memory arenas")

	// Zero secure arena before destruction
	if g_secure_arena.curr_block != nil {
		used := g_secure_arena.curr_block.used
		if used > 0 {
			mem.zero(rawptr(g_secure_arena.curr_block.base), int(used))
			log.debugf("[SECURITY] Zeroed %d bytes in secure arena before shutdown", used)
		}
	}

	vmem.arena_destroy(&g_secure_arena)
	vmem.arena_destroy(&g_request_arena)
	vmem.arena_destroy(&g_command_arena)
	vmem.arena_destroy(&g_persistent_arena)
	log.debug("Memory arenas destroyed")
}

// Get persistent arena allocator
// Use for: Configuration, database handles, program-lifetime data
persistent_allocator :: proc() -> mem.Allocator {
	return vmem.arena_allocator(&g_persistent_arena)
}

// Get command arena allocator
// Use for: Command-specific data (token lists, query results)
command_allocator :: proc() -> mem.Allocator {
	return vmem.arena_allocator(&g_command_arena)
}

// Get request arena allocator
// Use for: RPC responses, HTTP buffers, temporary parsing
request_allocator :: proc() -> mem.Allocator {
	return vmem.arena_allocator(&g_request_arena)
}

// Get secure arena allocator
// Use for: Cryptographic operations (BIP32, BIP39, PBKDF2, encryption)
// SECURITY: Memory is explicitly zeroed before reset
secure_allocator :: proc() -> mem.Allocator {
	return vmem.arena_allocator(&g_secure_arena)
}

// Check if secure arena is currently the active allocator
// Use for: Determining if a function should set up/tear down arena or inherit from caller
is_secure_arena_active :: proc() -> bool {
	secure_alloc := vmem.arena_allocator(&g_secure_arena)
	return context.allocator.procedure == secure_alloc.procedure &&
	       context.allocator.data == secure_alloc.data
}

// Reset command arena (call between commands)
// PATTERN: Call at end of each command handler
reset_command_arena :: proc() {
	// GOTCHA: arena_free_all() keeps first block, deallocates rest
	vmem.arena_free_all(&g_command_arena)
	log.debug("Command arena reset")
}

// Reset request arena (call after RPC operations)
// PATTERN: Call after each network operation completes
reset_request_arena :: proc() {
	if g_memory_stats_enabled {
		stats := memory_stats()
		log.debugf("[MEMORY] Request arena before reset: %d KB used", stats.request_used / 1024)
	}
	vmem.arena_free_all(&g_request_arena)
	log.debug("Request arena reset")
}

// Reset secure arena (call after cryptographic operations)
// SECURITY: Explicitly zeros all used memory before freeing
// PATTERN: Call after BIP32/BIP39/PBKDF2/encryption operations complete
reset_secure_arena :: proc() {
	// Zero all used memory in secure arena
	if g_secure_arena.curr_block != nil {
		used := g_secure_arena.curr_block.used
		if used > 0 {
			mem.zero(rawptr(g_secure_arena.curr_block.base), int(used))
			log.debugf("[SECURITY] Zeroed %d bytes in secure arena", used)
		}
	}

	// Free arena (keeps first block, deallocates rest)
	vmem.arena_free_all(&g_secure_arena)
	log.debug("Secure arena reset (memory zeroed)")
}

// Get current memory statistics
memory_stats :: proc() -> MemoryStats {
	stats := MemoryStats{
		persistent_used = g_persistent_arena.curr_block != nil ? g_persistent_arena.curr_block.used : 0,
		command_used    = g_command_arena.curr_block != nil ? g_command_arena.curr_block.used : 0,
		request_used    = g_request_arena.curr_block != nil ? g_request_arena.curr_block.used : 0,
		secure_used     = g_secure_arena.curr_block != nil ? g_secure_arena.curr_block.used : 0,
	}
	return stats
}

// Log memory statistics (only if enabled)
log_memory_stats :: proc() {
	if !g_memory_stats_enabled {
		return
	}

	stats := memory_stats()
	log.debugf(
		"[MEMORY] Arena usage: persistent=%d KB, command=%d KB, request=%d KB, secure=%d KB",
		stats.persistent_used / 1024,
		stats.command_used / 1024,
		stats.request_used / 1024,
		stats.secure_used / 1024,
	)
}

// Enable memory statistics logging
enable_memory_stats :: proc() {
	g_memory_stats_enabled = true
	log.debug("Memory statistics logging enabled")
}
