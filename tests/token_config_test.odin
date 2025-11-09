#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "core:strings"
import "../src"

// =============================================================================
// TOKEN CONFIGURATION TESTS
// =============================================================================
// These tests document and verify the token configuration system.
//
// Configuration Structure (~/.config/hound/tokens.json):
// {
//   "version": "1.0",
//   "tokens": [
//     {
//       "symbol": "AURA",
//       "name": "Aura Token",
//       "contract_address": "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
//       "chain": "solana",
//       "pools": [{
//         "dex": "raydium",
//         "pool_address": "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
//         "quote_token": "sol",
//         "pool_type": "amm_v4"
//       }],
//       "is_quote_token": false,
//       "usd_price": 0.0
//     }
//   ]
// }
//
// Configuration Loading:
// 1. Looks for file at $HOME/.config/hound/tokens.json
// 2. Parses JSON structure
// 3. Validates that at least one token exists
// 4. Returns TokenConfig or error
// =============================================================================

@(test)
test_token_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: Token structure contains all necessary information
	// for both API-based and on-chain price fetching
	//
	// Fields:
	// - symbol: Short identifier (e.g., "AURA", "SOL")
	// - name: Full token name
	// - contract_address: On-chain token mint address
	// - chain: Blockchain name (currently only "solana")
	// - pools: Array of liquidity pools for on-chain pricing
	// - is_quote_token: true for SOL, USDC (used as price references)
	// - usd_price: Pre-set USD price for quote tokens

	token := src.Token{
		symbol = "AURA",
		name = "Aura Token",
		contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
		chain = "solana",
		pools = []src.PoolInfo{
			{
				dex = "raydium",
				pool_address = "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
				quote_token = "sol",
				pool_type = "amm_v4",
			},
		},
		is_quote_token = false,
		usd_price = 0.0,
	}

	testing.expect(t, len(token.symbol) > 0, "Symbol should not be empty")
	testing.expect(t, len(token.name) > 0, "Name should not be empty")
	testing.expect(t, len(token.contract_address) > 0, "Contract address should not be empty")
	testing.expect(t, token.chain == "solana", "Chain should be solana")
	testing.expect(t, len(token.pools) > 0, "Should have at least one pool configured")
}

@(test)
test_pool_info_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: PoolInfo contains data for locating a liquidity pool
	//
	// Fields:
	// - dex: DEX name (e.g., "raydium", "orca")
	// - pool_address: On-chain address of the pool account
	// - quote_token: The quote token symbol in the pair (e.g., "sol", "usdc")
	// - pool_type: Pool program version (e.g., "amm_v4" for Raydium V4)

	pool := src.PoolInfo{
		dex = "raydium",
		pool_address = "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
		quote_token = "sol",
		pool_type = "amm_v4",
	}

	testing.expect(t, pool.dex == "raydium",
		"DEX should be raydium")

	testing.expect(t, len(pool.pool_address) == 44,
		fmt.tprintf("Solana addresses are 44 characters in base58, got %d",
			len(pool.pool_address)))

	testing.expect(t, pool.quote_token == "sol" || pool.quote_token == "usdc",
		"Quote token should be a known quote token like sol or usdc")

	testing.expect(t, pool.pool_type == "amm_v4",
		"Pool type should specify the AMM version")
}

@(test)
test_find_token_by_symbol_exact_match :: proc(t: ^testing.T) {
	// DOCUMENTATION: find_token_by_symbol searches for tokens by symbol
	// with case-insensitive matching
	//
	// This allows users to query tokens as:
	// - "AURA", "aura", "Aura" (all match)
	// - "SOL", "sol", "Sol" (all match)

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
				symbol = "AURA",
				name = "Aura Token",
				contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
				chain = "solana",
				pools = []src.PoolInfo{},
			},
			{
				symbol = "SOL",
				name = "Solana",
				contract_address = "So11111111111111111111111111111111111111112",
				chain = "solana",
				pools = []src.PoolInfo{},
			},
		},
	}

	// Test exact uppercase match
	token, found := src.find_token_by_symbol(config, "AURA")
	testing.expect(t, found, "Should find AURA token")
	testing.expect(t, token.symbol == "AURA", "Should return correct token")
}

@(test)
test_find_token_by_symbol_case_insensitive :: proc(t: ^testing.T) {
	// DOCUMENTATION: Token search is case-insensitive for better UX
	//
	// Users can type symbols in any case:
	// $ hound aura    (lowercase)
	// $ hound AURA    (uppercase)
	// $ hound Aura    (mixed case)

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
				symbol = "AURA",
				name = "Aura Token",
				contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
				chain = "solana",
				pools = []src.PoolInfo{},
			},
		},
	}

	// Test lowercase
	token1, found1 := src.find_token_by_symbol(config, "aura")
	testing.expect(t, found1, "Should find token with lowercase symbol")

	// Test mixed case
	token2, found2 := src.find_token_by_symbol(config, "AuRa")
	testing.expect(t, found2, "Should find token with mixed case symbol")

	// Test uppercase
	token3, found3 := src.find_token_by_symbol(config, "AURA")
	testing.expect(t, found3, "Should find token with uppercase symbol")

	// All should return the same token
	testing.expect(t, token1.symbol == token2.symbol && token2.symbol == token3.symbol,
		"All case variations should return the same token")
}

@(test)
test_find_token_by_symbol_not_found :: proc(t: ^testing.T) {
	// DOCUMENTATION: find_token_by_symbol returns false when token doesn't exist
	//
	// This allows the caller to show appropriate error messages:
	// "Token 'XYZ' not found. Use 'hound list' to see available tokens."

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
				symbol = "AURA",
				name = "Aura Token",
				contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
				chain = "solana",
				pools = []src.PoolInfo{},
			},
		},
	}

	_, found := src.find_token_by_symbol(config, "NONEXISTENT")
	testing.expect(t, !found, "Should not find non-existent token")
}

@(test)
test_find_token_by_symbol_empty_config :: proc(t: ^testing.T) {
	// DOCUMENTATION: Searching in an empty config returns not found
	//
	// This can happen if config file exists but has no tokens defined

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{},
	}

	_, found := src.find_token_by_symbol(config, "AURA")
	testing.expect(t, !found, "Should not find token in empty config")
}

@(test)
test_token_config_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: TokenConfig is the root structure of the JSON file
	//
	// Structure:
	// {
	//   "version": "1.0",  // Config file format version
	//   "tokens": [...]     // Array of token definitions
	// }
	//
	// The version field allows for future format changes without breaking
	// compatibility with older config files

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
				symbol = "AURA",
				name = "Aura Token",
				contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
				chain = "solana",
				pools = []src.PoolInfo{},
			},
		},
	}

	testing.expect(t, config.version == "1.0",
		"Version should be specified")

	testing.expect(t, len(config.tokens) > 0,
		"Config should contain at least one token")
}

@(test)
test_quote_token_configuration :: proc(t: ^testing.T) {
	// DOCUMENTATION: Quote tokens (SOL, USDC) have special configuration
	//
	// Quote tokens are used as price references. They have:
	// - is_quote_token: true
	// - usd_price: Pre-set USD price (e.g., 162.50 for SOL)
	// - No pools needed (they are the reference)
	//
	// Regular tokens use quote tokens to calculate USD prices:
	// token_usd_price = token_quote_price * quote_usd_price

	sol_token := src.Token{
		symbol = "SOL",
		name = "Solana",
		contract_address = "So11111111111111111111111111111111111111112",
		chain = "solana",
		pools = []src.PoolInfo{}, // Quote tokens don't need pools
		is_quote_token = true,
		usd_price = 162.50,
	}

	testing.expect(t, sol_token.is_quote_token,
		"SOL should be marked as quote token")

	testing.expect(t, sol_token.usd_price > 0,
		"Quote token should have USD price set")

	testing.expect(t, len(sol_token.pools) == 0,
		"Quote tokens don't need pool configuration")
}

@(test)
test_multiple_pools_configuration :: proc(t: ^testing.T) {
	// DOCUMENTATION: Tokens can have multiple liquidity pools
	//
	// Use cases:
	// 1. Multiple DEXs (Raydium + Orca)
	// 2. Multiple quote tokens (SOL pool + USDC pool)
	// 3. Fallback pools if primary pool has low liquidity
	//
	// The system uses the first pool in the array for pricing

	token := src.Token{
		symbol = "AURA",
		name = "Aura Token",
		contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
		chain = "solana",
		pools = []src.PoolInfo{
			{
				dex = "raydium",
				pool_address = "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
				quote_token = "sol",
				pool_type = "amm_v4",
			},
			{
				dex = "raydium",
				pool_address = "AnotherPoolAddress123456789",
				quote_token = "usdc",
				pool_type = "amm_v4",
			},
		},
	}

	testing.expect(t, len(token.pools) == 2,
		"Token should have two pools configured")

	testing.expect(t, token.pools[0].quote_token != token.pools[1].quote_token,
		"Multiple pools should use different quote tokens for redundancy")
}

@(test)
test_solana_address_format :: proc(t: ^testing.T) {
	// DOCUMENTATION: Solana addresses are base58 encoded and 32-44 characters
	//
	// Examples:
	// - Token mint: "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2" (44 chars)
	// - SOL native: "So11111111111111111111111111111111111111112" (44 chars)
	// - Pool address: "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN" (44 chars)
	//
	// Valid characters: [1-9A-HJ-NP-Za-km-z] (base58 alphabet, no 0, O, I, l)

	addresses := []string{
		"DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
		"So11111111111111111111111111111111111111112",
		"9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
	}

	for addr in addresses {
		// Check length (typically 44 for Solana, but 32-44 is valid)
		testing.expect(t, len(addr) >= 32 && len(addr) <= 44,
			fmt.tprintf("Solana address length should be 32-44 chars, got %d", len(addr)))

		// Check for invalid base58 characters (0, O, I, l)
		testing.expect(t, !strings.contains_any(addr, "0OIl"),
			fmt.tprintf("Address '%s' contains invalid base58 characters", addr))
	}
}
