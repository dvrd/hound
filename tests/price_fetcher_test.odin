#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "core:math"
import "../src"

// =============================================================================
// PRICE CALCULATION TESTS
// =============================================================================
// These tests document and verify the AMM (Automated Market Maker) price
// calculation logic used for Raydium pools.
//
// AMM Formula: Constant Product (x * y = k)
// Price = quote_reserve / base_reserve (adjusted for decimals)
//
// Real Example (AURA/SOL pool):
// - Base Reserve: 33,091,969.63 AURA (6 decimals)
// - Quote Reserve: 12,410.68 SOL (9 decimals)
// - Price: 12,410.68 / 33,091,969.63 = 0.000375 SOL per AURA
// - USD Price: 0.000375 * $162.50 = $0.0609
// =============================================================================

@(test)
test_calculate_price_from_reserves_basic :: proc(t: ^testing.T) {
	// DOCUMENTATION: Calculate token price using AMM constant product formula
	//
	// Example: A pool with equal reserves (1:1 ratio)
	// - Base: 1000 tokens (6 decimals) = 1,000,000,000 raw
	// - Quote: 1000 tokens (6 decimals) = 1,000,000,000 raw
	// - Expected price: 1.0 (1 base token = 1 quote token)

	base_reserve := u64(1_000_000_000)    // 1000 tokens with 6 decimals
	quote_reserve := u64(1_000_000_000)   // 1000 tokens with 6 decimals
	base_decimals := u64(6)
	quote_decimals := u64(6)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 1.0
	tolerance := 0.0001

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("Equal reserves should give 1:1 price. Expected ~%.4f, got %.4f",
			expected, price))
}

@(test)
test_calculate_price_from_reserves_different_decimals :: proc(t: ^testing.T) {
	// DOCUMENTATION: Handles tokens with different decimal places
	//
	// Real-world example: AURA (6 decimals) / SOL (9 decimals)
	// - Base: 33,091,969.63 AURA = 33,091,969,630,000 raw (6 decimals)
	// - Quote: 12,410.68 SOL = 12,410,680,000,000 raw (9 decimals)
	// - Expected: 12,410.68 / 33,091,969.63 ≈ 0.000375

	base_reserve := u64(33_091_969_630_000)    // ~33M AURA tokens
	quote_reserve := u64(12_410_680_000_000)   // ~12.4K SOL tokens
	base_decimals := u64(6)
	quote_decimals := u64(9)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 0.000375
	tolerance := 0.000001

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("AURA/SOL price calculation failed. Expected ~%.6f, got %.6f",
			expected, price))
}

@(test)
test_calculate_price_from_reserves_expensive_token :: proc(t: ^testing.T) {
	// DOCUMENTATION: Calculate price when quote token is much more expensive
	//
	// Example: Small amount of expensive quote token (like BTC)
	// - Base: 1,000,000 cheap tokens (6 decimals)
	// - Quote: 10 expensive tokens (8 decimals, like BTC)
	// - Expected: 10 / 1,000,000 = 0.00001 (base token is very cheap)

	base_reserve := u64(1_000_000_000_000)   // 1M tokens with 6 decimals
	quote_reserve := u64(1_000_000_000)      // 10 tokens with 8 decimals
	base_decimals := u64(6)
	quote_decimals := u64(8)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 0.00001
	tolerance := 0.0000001

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("High-value quote token calculation failed. Expected %.8f, got %.8f",
			expected, price))
}

@(test)
test_calculate_price_from_reserves_zero_base :: proc(t: ^testing.T) {
	// DOCUMENTATION: Handle edge case of zero base reserve
	// This should return 0 to avoid division by zero

	base_reserve := u64(0)
	quote_reserve := u64(1_000_000_000)
	base_decimals := u64(6)
	quote_decimals := u64(6)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	testing.expect(t, price == 0.0,
		fmt.tprintf("Zero base reserve should return 0, got %.6f", price))
}

@(test)
test_calculate_price_from_reserves_high_decimals :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test with maximum Solana token decimals (9)
	//
	// Example: Both tokens with 9 decimals
	// - Base: 1 token = 1,000,000,000 raw
	// - Quote: 100 tokens = 100,000,000,000 raw
	// - Expected: 100 / 1 = 100.0

	base_reserve := u64(1_000_000_000)       // 1 token with 9 decimals
	quote_reserve := u64(100_000_000_000)    // 100 tokens with 9 decimals
	base_decimals := u64(9)
	quote_decimals := u64(9)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 100.0
	tolerance := 0.01

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("High decimal precision failed. Expected %.2f, got %.2f",
			expected, price))
}

@(test)
test_calculate_price_from_reserves_asymmetric_decimals :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test extreme decimal differences
	//
	// Example: Token with 0 decimals (like some NFTs) vs token with 9 decimals
	// - Base: 1000 tokens (0 decimals) = 1000 raw
	// - Quote: 0.001 tokens (9 decimals) = 1,000,000 raw
	// - Expected: 0.001 / 1000 = 0.000001

	base_reserve := u64(1000)              // 1000 tokens with 0 decimals
	quote_reserve := u64(1_000_000)        // 0.001 tokens with 9 decimals
	base_decimals := u64(0)
	quote_decimals := u64(9)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 0.000001
	tolerance := 0.0000001

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("Asymmetric decimals failed. Expected %.9f, got %.9f",
			expected, price))
}

@(test)
test_calculate_price_real_world_sol_usdc :: proc(t: ^testing.T) {
	// DOCUMENTATION: Real-world example with SOL/USDC pool
	//
	// Typical SOL/USDC pool (at SOL = $162.50):
	// - Base: 1,000 SOL (9 decimals) = 1,000,000,000,000 raw
	// - Quote: 162,500 USDC (6 decimals) = 162,500,000,000 raw
	// - Expected: 162,500 / 1,000 = 162.5 USDC per SOL

	base_reserve := u64(1_000_000_000_000)   // 1,000 SOL
	quote_reserve := u64(162_500_000_000)    // 162,500 USDC
	base_decimals := u64(9)                  // SOL has 9 decimals
	quote_decimals := u64(6)                 // USDC has 6 decimals

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 162.5
	tolerance := 0.1

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("SOL/USDC price calculation failed. Expected %.2f, got %.2f",
			expected, price))
}

@(test)
test_calculate_price_microcap_token :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test with microcap token (very low price)
	//
	// Example: A microcap token worth 0.0000001 SOL
	// - Base: 10,000,000 tokens (6 decimals)
	// - Quote: 1 SOL (9 decimals)
	// - Expected: 1 / 10,000,000 = 0.0000001

	base_reserve := u64(10_000_000_000_000)   // 10M tokens with 6 decimals
	quote_reserve := u64(1_000_000_000)       // 1 SOL with 9 decimals
	base_decimals := u64(6)
	quote_decimals := u64(9)

	price := src.calculate_price_from_reserves(
		base_reserve,
		quote_reserve,
		base_decimals,
		quote_decimals,
	)

	expected := 0.0000001
	tolerance := 0.00000001

	testing.expect(t, math.abs(price - expected) < tolerance,
		fmt.tprintf("Microcap token price failed. Expected %.10f, got %.10f",
			expected, price))
}

// =============================================================================
// PRICE DATA STRUCTURE TESTS
// =============================================================================

@(test)
test_price_data_structure :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify PriceData structure can hold typical values
	//
	// PriceData contains:
	// - price_usd: f64 - The token price in USD
	// - change_24h: f64 - 24-hour price change percentage

	price_data := src.PriceData{
		price_usd = 0.060876,
		change_24h = 5.25,
	}

	testing.expect(t, price_data.price_usd > 0,
		"Price should be positive")

	testing.expect(t, price_data.change_24h >= -100 && price_data.change_24h <= 1000,
		fmt.tprintf("Change percentage seems unreasonable: %.2f%%", price_data.change_24h))
}

@(test)
test_error_type_enum :: proc(t: ^testing.T) {
	// DOCUMENTATION: Verify all error types are distinct
	//
	// ErrorType enum includes granular errors for:
	// - Network issues (NetworkTimeout, ConnectionFailed)
	// - API issues (RateLimited, ServerError)
	// - Data issues (InvalidToken, TokenNotFound, InvalidResponse)
	// - RPC issues (RPCConnectionFailed, RPCInvalidResponse)
	// - Pool issues (PoolDataInvalid, VaultFetchFailed, TokenNotConfigured)

	none := src.ErrorType.None
	network_timeout := src.ErrorType.NetworkTimeout
	token_not_found := src.ErrorType.TokenNotFound

	testing.expect(t, none != network_timeout,
		"Error types should be distinct")

	testing.expect(t, network_timeout != token_not_found,
		"Error types should be distinct")
}
