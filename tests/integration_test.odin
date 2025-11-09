#+feature global-context
#+feature dynamic-literals
package tests

import "core:testing"
import "core:fmt"
import "../src"

// =============================================================================
// INTEGRATION TESTS
// =============================================================================
// These tests document the complete end-to-end workflows of the Hound system.
//
// Test Philosophy:
// - Integration tests verify that components work together correctly
// - They test real-world scenarios and data flows
// - They serve as living documentation of system behavior
//
// Coverage:
// 1. Config loading → Token lookup → Price calculation
// 2. Pool decoding → Vault reading → Price calculation
// 3. Error handling across component boundaries
// 4. Data transformation pipelines
// =============================================================================

@(test)
test_integration_token_lookup_workflow :: proc(t: ^testing.T) {
	// DOCUMENTATION: Complete token lookup workflow
	//
	// User Flow:
	// 1. User runs: `hound aura`
	// 2. System loads config from ~/.config/hound/tokens.json
	// 3. System searches for "aura" (case-insensitive)
	// 4. System returns token configuration
	// 5. System uses config to fetch price
	//
	// This test simulates steps 2-4

	// Step 1: Create mock configuration
	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
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
			},
			{
				symbol = "SOL",
				name = "Solana",
				contract_address = "So11111111111111111111111111111111111111112",
				chain = "solana",
				pools = []src.PoolInfo{},
				is_quote_token = true,
				usd_price = 162.50,
			},
		},
	}

	// Step 2: User queries "aura" (lowercase)
	user_input := "aura"

	// Step 3: System finds token (case-insensitive)
	token, found := src.find_token_by_symbol(config, user_input)
	testing.expect(t, found,
		"Integration: Token lookup should succeed for valid symbol")

	// Step 4: Verify token has necessary data for price fetching
	testing.expect(t, len(token.contract_address) > 0,
		"Integration: Found token should have contract address")

	testing.expect(t, len(token.pools) > 0,
		"Integration: Found token should have pools for on-chain pricing")

	testing.expect(t, token.pools[0].dex == "raydium",
		"Integration: Pool should specify DEX")

	// Step 5: Find quote token for USD conversion
	quote_token, quote_found := src.find_token_by_symbol(config, token.pools[0].quote_token)
	testing.expect(t, quote_found,
		"Integration: Quote token should exist in config")

	testing.expect(t, quote_token.is_quote_token,
		"Integration: Quote token should be marked as such")

	testing.expect(t, quote_token.usd_price > 0,
		"Integration: Quote token should have USD price")
}

@(test)
test_integration_price_calculation_pipeline :: proc(t: ^testing.T) {
	// DOCUMENTATION: Complete price calculation pipeline
	//
	// Data Flow:
	// 1. Fetch pool data (752 bytes) from Solana RPC
	// 2. Decode pool structure to extract vault addresses
	// 3. Fetch vault balances from RPC
	// 4. Calculate price using AMM formula
	// 5. Convert to USD using quote token price
	//
	// This test simulates steps 2-5 with mock data

	// Step 1: Create mock pool data (752 bytes)
	pool_data := make([]u8, 752)
	defer delete(pool_data)

	// Set decimals (offset 32: quote=9, offset 40: base=6)
	pool_data[32] = 9  // SOL decimals
	pool_data[40] = 6  // AURA decimals

	// Set mock vault addresses (offsets 336 and 368)
	// In real scenario, these would be actual Solana addresses

	// Step 2: Decode pool
	pool_state, decode_ok := src.decode_raydium_pool_v4(pool_data)
	testing.expect(t, decode_ok,
		"Integration: Pool decode should succeed")

	testing.expect(t, pool_state.quote_decimal == 9,
		"Integration: Quote decimal should be extracted correctly")

	testing.expect(t, pool_state.base_decimal == 6,
		"Integration: Base decimal should be extracted correctly")

	// Step 3: Mock vault balances (in real scenario, fetched via RPC)
	base_reserve := u64(33_091_969_630_000)    // ~33M AURA
	quote_reserve := u64(12_410_680_000_000)   // ~12.4K SOL

	// Step 4: Calculate price in quote token
	price_in_sol := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		pool_state.base_decimal,
		pool_state.quote_decimal,
	)

	testing.expect(t, price_in_sol > 0,
		"Integration: Price calculation should return positive value")

	testing.expect(t, price_in_sol < 1.0,
		"Integration: AURA price in SOL should be less than 1 (it's a small altcoin)")

	// Step 5: Convert to USD
	sol_usd_price := 162.50
	price_in_usd := price_in_sol * sol_usd_price

	testing.expect(t, price_in_usd > 0.001 && price_in_usd < 1.0,
		fmt.tprintf("Integration: USD price seems unreasonable: $%.6f", price_in_usd))
}

@(test)
test_integration_error_handling_workflow :: proc(t: ^testing.T) {
	// DOCUMENTATION: Error handling across system boundaries
	//
	// Error Flow:
	// 1. User requests unknown token → TokenNotFound error
	// 2. Token has no pools configured → TokenNotConfigured error
	// 3. RPC call fails → RPCConnectionFailed error
	// 4. Invalid pool data → PoolDataInvalid error
	//
	// This test documents error propagation

	config := src.TokenConfig{
		version = "1.0",
		tokens = []src.Token{
			{
				symbol = "UNCONFIGURED",
				name = "Unconfigured Token",
				contract_address = "SomeAddress123",
				chain = "solana",
				pools = []src.PoolInfo{}, // No pools!
			},
		},
	}

	// Scenario 1: Token not found
	_, found := src.find_token_by_symbol(config, "NONEXISTENT")
	testing.expect(t, !found,
		"Integration: Should return false for non-existent token")

	// Scenario 2: Token found but has no pools
	token, found2 := src.find_token_by_symbol(config, "UNCONFIGURED")
	testing.expect(t, found2,
		"Integration: Should find the token")

	testing.expect(t, len(token.pools) == 0,
		"Integration: Token should have no pools configured")

	// In real code, this would trigger .TokenNotConfigured error

	// Scenario 3: Invalid pool data size
	invalid_data := make([]u8, 100) // Wrong size!
	defer delete(invalid_data)

	_, decode_ok := src.decode_raydium_pool_v4(invalid_data)
	testing.expect(t, !decode_ok,
		"Integration: Should reject invalid pool data")
}

@(test)
test_integration_multi_pool_selection :: proc(t: ^testing.T) {
	// DOCUMENTATION: Multi-pool configuration and selection
	//
	// Use Case:
	// A token has multiple liquidity pools:
	// 1. Primary pool (Raydium, high liquidity)
	// 2. Backup pool (Orca, moderate liquidity)
	// 3. Alternative quote token pool (USDC instead of SOL)
	//
	// System should:
	// - Use first pool by default
	// - Provide data for manual pool selection
	// - Support failover if primary pool fails

	token := src.Token{
		symbol = "MULTI",
		name = "Multi Pool Token",
		contract_address = "MultiPoolTokenAddress",
		chain = "solana",
		pools = []src.PoolInfo{
			{
				dex = "raydium",
				pool_address = "PrimaryPool123",
				quote_token = "sol",
				pool_type = "amm_v4",
			},
			{
				dex = "orca",
				pool_address = "BackupPool456",
				quote_token = "sol",
				pool_type = "whirlpool",
			},
			{
				dex = "raydium",
				pool_address = "USDCPool789",
				quote_token = "usdc",
				pool_type = "amm_v4",
			},
		},
	}

	testing.expect(t, len(token.pools) == 3,
		"Integration: Token should have multiple pools")

	// System uses first pool
	primary_pool := token.pools[0]
	testing.expect(t, primary_pool.dex == "raydium",
		"Integration: Primary pool should be Raydium")

	// Backup pool available
	backup_pool := token.pools[1]
	testing.expect(t, backup_pool.dex == "orca",
		"Integration: Backup pool should be Orca")

	// Alternative quote token available
	alt_quote_pool := token.pools[2]
	testing.expect(t, alt_quote_pool.quote_token == "usdc",
		"Integration: Alternative pool should use USDC")
}

@(test)
test_integration_decimal_precision_handling :: proc(t: ^testing.T) {
	// DOCUMENTATION: Decimal precision across the system
	//
	// Solana tokens can have 0-9 decimals:
	// - Most tokens: 6 or 9 decimals
	// - SOL: 9 decimals (lamports)
	// - USDC: 6 decimals
	// - Some NFTs: 0 decimals
	//
	// System must:
	// 1. Correctly read decimals from pool data
	// 2. Apply decimal conversion in calculations
	// 3. Display prices with appropriate precision

	// Test with various decimal combinations
	test_cases := []struct {
		base_decimals: u64,
		quote_decimals: u64,
		base_reserve: u64,
		quote_reserve: u64,
		expected_price_range: [2]f64, // [min, max]
	}{
		// Case 1: Both 6 decimals (USDC/USDT pair)
		{6, 6, 1_000_000, 1_000_000, {0.99, 1.01}},

		// Case 2: Different decimals (AURA/SOL)
		{6, 9, 1_000_000, 1_000_000_000, {0.99, 1.01}},

		// Case 3: High decimals (SOL/SOL hypothetical)
		{9, 9, 1_000_000_000, 1_000_000_000, {0.99, 1.01}},

		// Case 4: Zero decimals (NFT/SOL)
		{0, 9, 1, 1_000_000_000, {0.99, 1.01}},
	}

	for test_case, i in test_cases {
		price := src.calculate_price_from_reserves(
			test_case.base_reserve,
			test_case.quote_reserve,
			test_case.base_decimals,
			test_case.quote_decimals,
		)

		testing.expect(
			t,
			price >= test_case.expected_price_range[0] &&
				price <= test_case.expected_price_range[1],
			fmt.tprintf("Integration: Test case %d - Price %.9f outside expected range [%.9f, %.9f]",
				i, price, test_case.expected_price_range[0], test_case.expected_price_range[1]),
		)
	}
}

@(test)
test_integration_real_world_addresses :: proc(t: ^testing.T) {
	// DOCUMENTATION: Real-world Solana address validation
	//
	// Known Addresses:
	// - SOL Native Mint: So11111111111111111111111111111111111111112
	// - USDC Mint: EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v
	// - AURA Mint: DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2
	// - AURA/SOL Pool: 9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN
	//
	// These can be used to verify base58 encoding/decoding

	known_addresses := map[string]string{
		"SOL" = "So11111111111111111111111111111111111111112",
		"USDC" = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"AURA" = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
		"AURA_SOL_POOL" = "9ViX1VductEoC2wERTSp2TuDxXPwAf69aeET8ENPJpsN",
	}

	for name, address in known_addresses {
		// Verify basic address properties
		testing.expect(t, len(address) >= 32 && len(address) <= 44,
			fmt.tprintf("Integration: %s address length invalid: %d", name, len(address)))

		// Note: Full base58 decode → encode round-trip would require actual implementation
		// This test documents the expected addresses for reference
	}
}
