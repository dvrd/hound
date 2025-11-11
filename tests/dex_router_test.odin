#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "../src"

// =============================================================================
// DEX ROUTER TESTS
// =============================================================================
// These tests validate the DEX routing logic including:
// - DEX type parsing
// - Pool config conversion
// - Priority-based routing
// - Fallback mechanisms
//
// Test Philosophy:
// - Unit tests for pure functions (parse_dex_type, pool_info_to_dex_config)
// - Integration tests for routing logic (require network/RPC)
// - Document routing strategies and error handling
//
// Coverage:
// 1. DEX type parsing (Orca, Jupiter, Raydium, Unknown)
// 2. PoolInfo to DexPoolConfig conversion
// 3. Priority-based pool ordering
// 4. Jupiter API fallback
// 5. Error propagation
// =============================================================================

@(test)
test_parse_dex_type_orca :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test Orca DEX type parsing
	// Orca Whirlpool supports multiple name variants:
	// - "orca"
	// - "orca_whirlpool"
	// - "whirlpool"
	// All should parse to .Orca_Whirlpool

	test_cases := []string{"orca", "ORCA", "orca_whirlpool", "Orca_Whirlpool", "whirlpool", "WHIRLPOOL"}

	for test_case in test_cases {
		dex_type := src.parse_dex_type(test_case)
		testing.expect(t, dex_type == .Orca_Whirlpool,
			fmt.tprintf("'%s' should parse to .Orca_Whirlpool, got %v", test_case, dex_type))
	}
}

@(test)
test_parse_dex_type_jupiter :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test Jupiter DEX type parsing
	// Jupiter API supports multiple name variants:
	// - "jupiter"
	// - "jupiter_api"
	// - "jupiter_aggregator"
	// All should parse to .Jupiter_API

	test_cases := []string{"jupiter", "JUPITER", "jupiter_api", "Jupiter_API", "jupiter_aggregator"}

	for test_case in test_cases {
		dex_type := src.parse_dex_type(test_case)
		testing.expect(t, dex_type == .Jupiter_API,
			fmt.tprintf("'%s' should parse to .Jupiter_API, got %v", test_case, dex_type))
	}
}

@(test)
test_parse_dex_type_raydium :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test Raydium DEX type parsing
	// Raydium CLMM (deferred to Phase 4.5):
	// - "raydium"
	// - "raydium_clmm"
	// Should parse to .Raydium_CLMM

	test_cases := []string{"raydium", "RAYDIUM", "raydium_clmm", "Raydium_CLMM"}

	for test_case in test_cases {
		dex_type := src.parse_dex_type(test_case)
		testing.expect(t, dex_type == .Raydium_CLMM,
			fmt.tprintf("'%s' should parse to .Raydium_CLMM, got %v", test_case, dex_type))
	}
}

@(test)
test_parse_dex_type_unknown :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test unknown DEX type parsing
	// Unknown or unsupported DEX names should return .Unknown

	test_cases := []string{"uniswap", "sushiswap", "pancakeswap", "invalid", ""}

	for test_case in test_cases {
		// Skip empty string as it will trigger assertion
		if len(test_case) == 0 do continue

		dex_type := src.parse_dex_type(test_case)
		testing.expect(t, dex_type == .Unknown,
			fmt.tprintf("'%s' should parse to .Unknown, got %v", test_case, dex_type))
	}
}

@(test)
test_pool_info_to_dex_config :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test PoolInfo to DexPoolConfig conversion
	// Validates that configuration from tokens.json is correctly
	// converted to internal routing configuration.

	pool_info := src.PoolInfo{
		dex          = "orca_whirlpool",
		pool_address = "HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ",
		quote_token  = "sol",
		pool_type    = "whirlpool",
	}

	priority := 1
	dex_config := src.pool_info_to_dex_config(pool_info, priority)

	// Verify DEX type parsing
	testing.expect(t, dex_config.dex_type == .Orca_Whirlpool,
		fmt.tprintf("Expected .Orca_Whirlpool, got %v", dex_config.dex_type))

	// Verify fields preserved
	testing.expect(t, dex_config.pool_address == pool_info.pool_address,
		fmt.tprintf("Pool address mismatch: expected %s, got %s",
			pool_info.pool_address, dex_config.pool_address))

	testing.expect(t, dex_config.quote_token == pool_info.quote_token,
		fmt.tprintf("Quote token mismatch: expected %s, got %s",
			pool_info.quote_token, dex_config.quote_token))

	testing.expect(t, dex_config.priority == priority,
		fmt.tprintf("Priority mismatch: expected %d, got %d",
			priority, dex_config.priority))

	testing.expect(t, dex_config.pool_type == pool_info.pool_type,
		fmt.tprintf("Pool type mismatch: expected %s, got %s",
			pool_info.pool_type, dex_config.pool_type))
}

@(test)
test_pool_info_to_dex_config_priority :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test priority assignment in conversion
	// Priority determines routing order (lower = higher priority)

	pool_info := src.PoolInfo{
		dex          = "jupiter",
		pool_address = "",  // Jupiter API doesn't need pool address
		quote_token  = "usd",
		pool_type    = "api",
	}

	// Test different priority values
	priorities := []int{1, 5, 10, 100}

	for priority in priorities {
		dex_config := src.pool_info_to_dex_config(pool_info, priority)

		testing.expect(t, dex_config.priority == priority,
			fmt.tprintf("Expected priority %d, got %d", priority, dex_config.priority))
	}
}

@(test)
test_dex_pool_config_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test DexPoolConfig structure initialization
	// Verify structure can hold all required routing information

	config := src.DexPoolConfig{
		dex_type     = .Orca_Whirlpool,
		pool_address = "HJPjoWUrhoZzkNfRpHuieeFk9WcZWjwy6PBjZ81ngndJ",
		quote_token  = "sol",
		priority     = 1,
		pool_type    = "whirlpool",
	}

	testing.expect(t, config.dex_type == .Orca_Whirlpool,
		"DEX type should match")

	testing.expect(t, len(config.pool_address) > 0,
		"Pool address should not be empty")

	testing.expect(t, config.quote_token == "sol",
		fmt.tprintf("Expected quote_token='sol', got '%s'", config.quote_token))

	testing.expect(t, config.priority == 1,
		fmt.tprintf("Expected priority=1, got %d", config.priority))

	testing.expect(t, config.pool_type == "whirlpool",
		fmt.tprintf("Expected pool_type='whirlpool', got '%s'", config.pool_type))
}

// =============================================================================
// INTEGRATION TESTS - Require Network Access
// =============================================================================
// The following tests make actual RPC/API calls and require network access.
// They may fail if:
// - Network is unavailable
// - RPC endpoints are down
// - Rate limits are exceeded
// - Token/pool addresses become invalid
//
// These tests validate real-world routing behavior.
// =============================================================================

@(test)
test_fetch_jupiter_api_price_integration :: proc(t: ^testing.T) {
	// INTEGRATION TEST: Test direct Jupiter API price fetch via router
	// This validates the fetch_jupiter_api_price() function

	token := src.Token{
		symbol           = "SOL",
		name             = "Solana",
		contract_address = "So11111111111111111111111111111111111111112",
		chain            = "solana",
		pools            = {},
		is_quote_token   = true,
		usd_price        = 0,
	}

	result, err := src.fetch_jupiter_api_price(token)

	// Skip if API is unavailable
	if err != .None {
		// API may be down or rate limited - this is expected
		return
	}

	// Validate result
	testing.expect(t, result.price_usd > 0,
		fmt.tprintf("Expected positive price, got %.6f", result.price_usd))

	testing.expect(t, result.source == .Jupiter_API,
		fmt.tprintf("Expected source=.Jupiter_API, got %v", result.source))

	testing.expect(t, len(result.pool_address) == 0,
		"Jupiter API should not have pool_address")
}

@(test)
test_route_price_query_jupiter_fallback :: proc(t: ^testing.T) {
	// INTEGRATION TEST: Test routing with Jupiter API fallback
	// Token with no pools should fall back to Jupiter API

	token := src.Token{
		symbol           = "USDC",
		name             = "USD Coin",
		contract_address = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		chain            = "solana",
		pools            = {}, // No pools configured - should use Jupiter
		is_quote_token   = true,
		usd_price        = 0,
	}

	result, err := src.route_price_query(token)

	// Skip if API is unavailable
	if err != .None {
		// API may be down or rate limited - this is expected
		return
	}

	// Validate result
	testing.expect(t, result.price_usd > 0,
		fmt.tprintf("Expected positive price, got %.6f", result.price_usd))

	testing.expect(t, result.source == .Jupiter_API,
		fmt.tprintf("Expected source=.Jupiter_API, got %v", result.source))

	// USDC should be close to $1.00 (stablecoin)
	price_diff := result.price_usd - 1.0
	if price_diff < 0 do price_diff = -price_diff

	testing.expect(t, price_diff < 0.1,
		fmt.tprintf("USDC price should be close to $1.00, got $%.6f", result.price_usd))
}
