// Token list formatting
// Displays token information in user-friendly table format
package output

import "core:fmt"
import models "../../lib/models"
import db "../../lib/database"

// ============================================================================
// Token List Display
// ============================================================================

// PoolStats represents pool statistics for a token
PoolStats :: struct {
	pool_count:      int,
	total_liquidity: f64,
	has_discovered:  bool,  // Whether any pool was auto-discovered
}

// format_token_list displays comprehensive token list with pool stats
//
// Shows for each token:
// - Symbol and name
// - Pool count
// - Total liquidity
// - ✨ indicator for auto-discovered tokens
//
// Fallback: Shows "(no pools)" if no pool data available
format_token_list :: proc(tokens: []models.Token, database: ^db.Database) {
	fmt.println("Available tokens:")
	fmt.println("")

	for token in tokens {
		// Get pool stats for this token
		stats, stats_err := db.get_pool_stats(database, token.symbol)

		if stats_err != .None || stats.pool_count == 0 {
			// No pools configured
			fmt.printfln("  %s - %s (no pools)", token.symbol, token.name)
			continue
		}

		// Check if any pool was auto-discovered
		has_discovered_pools := false
		for pool in token.pools {
			if pool.discovered_at > 0 {
				has_discovered_pools = true
				break
			}
		}

		// Format output with pool stats
		discovery_indicator := has_discovered_pools ? " ✨" : ""
		if stats.total_liquidity > 0 {
			fmt.printfln("  %s - %s (%d pool%s, $%.0f liquidity)%s",
				token.symbol, token.name,
				stats.pool_count, stats.pool_count == 1 ? "" : "s",
				stats.total_liquidity, discovery_indicator)
		} else {
			// Pools exist but no liquidity data
			fmt.printfln("  %s - %s (%d pool%s)%s",
				token.symbol, token.name,
				stats.pool_count, stats.pool_count == 1 ? "" : "s",
				discovery_indicator)
		}
	}
}

// format_token_summary displays single token summary
//
// Used for: Quick token info display
// Pattern: symbol - name (pool info)
format_token_summary :: proc(token: models.Token, stats: PoolStats) {
	discovery_indicator := stats.has_discovered ? " ✨" : ""

	if stats.pool_count == 0 {
		fmt.printfln("  %s - %s (no pools)", token.symbol, token.name)
	} else if stats.total_liquidity > 0 {
		fmt.printfln("  %s - %s (%d pool%s, $%.0f liquidity)%s",
			token.symbol, token.name,
			stats.pool_count, stats.pool_count == 1 ? "" : "s",
			stats.total_liquidity, discovery_indicator)
	} else {
		fmt.printfln("  %s - %s (%d pool%s)%s",
			token.symbol, token.name,
			stats.pool_count, stats.pool_count == 1 ? "" : "s",
			discovery_indicator)
	}
}

// format_basic_token_list displays simple token list without pool stats
//
// Fallback when: Database unavailable or pool stats not needed
// Pattern: Simple symbol - name format
format_basic_token_list :: proc(tokens: []models.Token) {
	fmt.println("Available tokens:")
	fmt.println("")

	for token in tokens {
		fmt.printfln("  %s - %s", token.symbol, token.name)
	}
}
