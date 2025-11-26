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

// ============================================================================
// Extended Token Info Display
// ============================================================================

// format_extended_token_info displays comprehensive token information
//
// Shows:
// - Basic identification (symbols, name, network, mint address)
// - Market data (price, market cap, FDV, liquidity)
// - Supply information
// - Trading activity (24h volume, transactions, buys/sells)
// - Price changes (5m, 1h, 6h, 24h)
// - Top 10 holders with ownership percentages
// - Creation date and status
format_extended_token_info :: proc(info: models.TokenExtendedInfo) {
	fmt.println("")
	fmt.println("═══════════════════════════════════════════════════════════════")

	// Symbols line
	fmt.print("  Symbols: ")
	if len(info.symbols) > 0 {
		for symbol, i in info.symbols {
			if i > 0 {
				fmt.print(", ")
			}
			fmt.print(symbol)
		}
		fmt.println("")
	} else {
		fmt.println("N/A")
	}

	// Basic info
	if len(info.name) > 0 {
		fmt.printfln("  Name: %s", info.name)
	}
	fmt.printfln("  Network: %s", info.network)
	fmt.printfln("  Mint: %s", info.mint_address)
	fmt.println("")

	// Market data
	fmt.println("MARKET DATA")
	fmt.println("───────────────────────────────────────────────────────────────")

	if info.price_usd > 0.0 {
		fmt.printfln("  Price: $%.6f", info.price_usd)
	} else {
		fmt.println("  Price: N/A")
	}

	if info.market_cap > 0.0 {
		format_large_number("  Market Cap", info.market_cap)
	}

	if info.fdv > 0.0 {
		format_large_number("  FDV", info.fdv)
	}

	if info.liquidity > 0.0 {
		format_large_number("  Liquidity", info.liquidity)
	}

	// Supply
	if info.total_supply > 0.0 {
		format_large_number("  Total Supply", info.total_supply)
	}
	fmt.println("")

	// Trading activity (24h)
	fmt.println("24H TRADING ACTIVITY")
	fmt.println("───────────────────────────────────────────────────────────────")

	if info.volume_24h > 0.0 {
		format_large_number("  Volume", info.volume_24h)
	}

	if info.txns_24h > 0 {
		fmt.printfln("  Transactions: %d", info.txns_24h)
		if info.buys_24h > 0 || info.sells_24h > 0 {
			fmt.printfln("    Buys: %d  |  Sells: %d", info.buys_24h, info.sells_24h)
		}
	}
	fmt.println("")

	// Price changes
	fmt.println("PRICE CHANGES")
	fmt.println("───────────────────────────────────────────────────────────────")
	format_price_change("  5m", info.price_change_5m)
	format_price_change("  1h", info.price_change_1h)
	format_price_change("  6h", info.price_change_6h)
	format_price_change("  24h", info.price_change_24h)
	fmt.println("")

	// Top holders
	if len(info.top_holders) > 0 {
		fmt.println("TOP HOLDERS")
		fmt.println("───────────────────────────────────────────────────────────────")

		// Show top 10 only
		max_display := min(10, len(info.top_holders))
		for i in 0..<max_display {
			holder := info.top_holders[i]
			fmt.printfln("  %2d. %s", i + 1, shorten_address(holder.address))
			fmt.printfln("      Balance: %.2f (%.2f%%)", holder.balance, holder.ownership_pct)
		}
		fmt.println("")
	}

	// Footer
	if len(info.created_at) > 0 {
		fmt.printfln("  Created: %s", format_timestamp(info.created_at))
	}
	fmt.printfln("  Status: %s", info.is_active ? "Active" : "Inactive")

	fmt.println("═══════════════════════════════════════════════════════════════")
	fmt.println("")
}

// Helper to format large numbers with K/M/B suffixes
format_large_number :: proc(label: string, value: f64) {
	if value >= 1_000_000_000 {
		fmt.printfln("%s: $%.2fB", label, value / 1_000_000_000)
	} else if value >= 1_000_000 {
		fmt.printfln("%s: $%.2fM", label, value / 1_000_000)
	} else if value >= 1_000 {
		fmt.printfln("%s: $%.2fK", label, value / 1_000)
	} else {
		fmt.printfln("%s: $%.2f", label, value)
	}
}

// Helper to format price changes with color indicators
format_price_change :: proc(label: string, change: f64) {
	if change == 0.0 {
		fmt.printfln("%s: N/A", label)
		return
	}

	sign := change > 0 ? "+" : ""
	fmt.printfln("%s: %s%.2f%%", label, sign, change)
}

// Helper to shorten Solana addresses for display
shorten_address :: proc(address: string) -> string {
	if len(address) <= 12 {
		return address
	}
	return fmt.tprintf("%s...%s", address[:8], address[len(address)-4:])
}

// Helper to format Unix timestamp (milliseconds)
format_timestamp :: proc(timestamp_str: string) -> string {
	// For now, just return the raw timestamp
	// Could be enhanced to parse and format as human-readable date
	return timestamp_str
}
