// Pool information formatting
// Displays liquidity pool details in user-friendly format
package output

import "core:fmt"
import models "../../src/lib/models"

// ============================================================================
// Pool Display Functions
// ============================================================================

// format_pool_info displays detailed pool information
//
// Shows:
// - DEX name
// - Pool address
// - Liquidity (if > 0)
//
// Pattern: Indented details after success message
format_pool_info :: proc(pool: models.PoolInfo) {
	fmt.eprintfln("✓ Found pool on %s", pool.dex)
	fmt.eprintfln("  Address: %s", pool.pool_address)
	if pool.liquidity_usd > 0 {
		fmt.eprintfln("  Liquidity: $%.0f", pool.liquidity_usd)
	}
}

// format_pool_summary displays compact pool information
//
// Used for: Quick pool info display without full details
// Pattern: Single line with key info
format_pool_summary :: proc(dex: string, address: string, liquidity: f64) {
	fmt.eprintfln("Found pool on %s (stored for future use)", dex)
}

// print_pool_discovery_prompt displays prompt for pool discovery
//
// Used in: add command when pool discovery is optional
print_pool_discovery_prompt :: proc(symbol: string) {
	fmt.eprintln("")
	fmt.eprintln("Would you like to discover liquidity pools now? (y/n)")
	fmt.eprint("> ")
}

// print_pool_discovery_skip displays skip message
//
// Used when: user chooses not to discover pools
print_pool_discovery_skip :: proc(symbol: string) {
	fmt.eprintln("")
	fmt.eprintfln("Pool discovery skipped. Run 'hound fetch %s' when ready.", symbol)
}

// print_pool_discovery_failed displays failure message with retry guidance
//
// Used when: pool discovery fails
print_pool_discovery_failed :: proc(symbol: string) {
	fmt.eprintln("⚠ Pool discovery failed")
	fmt.eprintfln("You can try again later: hound fetch %s --refresh", symbol)
}
