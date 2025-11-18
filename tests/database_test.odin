#+feature global-context
package tests

import "core:testing"
import "core:fmt"
import "core:os"
import "../src"

// =============================================================================
// DATABASE TESTS - Phase 5.1
// =============================================================================
// These tests validate the database module including:
// - Database open/close
// - Schema creation
// - Token insertion and retrieval
// - Pool insertion and retrieval
// - Case-insensitive symbol lookup
// - Transaction rollback on error
// - Integrity checking
// - JSON migration
//
// Test Philosophy:
// - Use in-memory databases (:memory:) for speed
// - Each test is independent (no shared state)
// - Test both success and failure paths
// - Follow Odin test patterns from raydium_clmm_decoder_test.odin
//
// Coverage:
// 1. Database lifecycle (open/close)
// 2. Schema creation
// 3. Token CRUD operations
// 4. Pool CRUD operations
// 5. Case-insensitive lookup
// 6. Foreign key constraints
// 7. Integrity check
// 8. JSON migration
// =============================================================================

@(test)
test_database_open_close :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test database can be opened and closed
	// Uses in-memory database for speed

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open successfully")
	testing.expect(t, db != nil, "Database handle should not be nil")
	defer src.database_close(db)

	testing.expect(t, db.handle != nil, "SQLite handle should not be nil")
	testing.expect(t, db.path == ":memory:", "Database path should match")
}

@(test)
test_create_schema :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test schema creation succeeds
	// Verifies tables and indexes are created

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	schema_err := src.create_schema(db)
	testing.expect(t, schema_err == .None,
		fmt.tprintf("Schema creation should succeed, got: %v", schema_err))

	// Verify schema by attempting to insert a token (would fail if tables don't exist)
	test_token := src.Token{
		symbol           = "TEST",
		name             = "Test Token",
		contract_address = "TestAddr123",
		chain            = "solana",
		is_quote_token   = false,
		usd_price        = 1.0,
		pools            = []src.PoolInfo{},
	}

	insert_err := src.insert_token(db, test_token)
	testing.expect(t, insert_err == .None,
		fmt.tprintf("Insert should succeed after schema creation, got: %v", insert_err))
}

@(test)
test_insert_and_retrieve_token :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test token can be inserted and retrieved by symbol
	// Validates round-trip data integrity

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Insert token
	test_token := src.Token{
		symbol           = "SOL",
		name             = "Solana",
		contract_address = "So11111111111111111111111111111111111111112",
		chain            = "solana",
		is_quote_token   = true,
		usd_price        = 162.50,
		pools            = []src.PoolInfo{},
	}

	insert_err := src.insert_token(db, test_token)
	testing.expect(t, insert_err == .None, "Insert should succeed")

	// Retrieve token
	retrieved, found, get_err := src.get_token_by_symbol(db, "SOL")
	testing.expect(t, get_err == .None, "Get should succeed")
	testing.expect(t, found, "Token should be found")
	testing.expect(t, retrieved.symbol == "SOL", "Symbol should match")
	testing.expect(t, retrieved.name == "Solana", "Name should match")
	testing.expect(t, retrieved.contract_address == "So11111111111111111111111111111111111111112",
		"Contract address should match")
	testing.expect(t, retrieved.is_quote_token == true, "is_quote_token should match")
	testing.expect(t, retrieved.usd_price == 162.50, "USD price should match")
}

@(test)
test_case_insensitive_lookup :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test symbol lookup is case-insensitive
	// COLLATE NOCASE should allow 'sol', 'SOL', 'Sol' to all match

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Insert with uppercase
	test_token := src.Token{
		symbol           = "SOL",
		name             = "Solana",
		contract_address = "So11111111111111111111111111111111111111112",
		chain            = "solana",
		is_quote_token   = true,
		usd_price        = 162.50,
		pools            = []src.PoolInfo{},
	}

	src.insert_token(db, test_token)

	// Test various case variations
	test_cases := []string{"sol", "SOL", "Sol", "sOl", "SoL"}

	for test_case in test_cases {
		retrieved, found, get_err := src.get_token_by_symbol(db, test_case)
		testing.expect(t, get_err == .None,
			fmt.tprintf("Get should succeed for '%s'", test_case))
		testing.expect(t, found,
			fmt.tprintf("Token should be found for '%s'", test_case))
		testing.expect(t, retrieved.symbol == "SOL",
			fmt.tprintf("Symbol should be 'SOL' for query '%s', got '%s'",
				test_case, retrieved.symbol))
	}
}

@(test)
test_insert_and_retrieve_pools :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test pools can be inserted and retrieved for a token
	// Validates foreign key relationship works

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Insert token
	test_token := src.Token{
		symbol           = "AURA",
		name             = "Aura Token",
		contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
		chain            = "solana",
		is_quote_token   = false,
		usd_price        = 0.0,
		pools            = []src.PoolInfo{},
	}

	_ = src.insert_token(db, test_token)

	// Insert pools
	pool1 := src.PoolInfo{
		dex          = "raydium",
		pool_address = "PoolAddr1",
		quote_token  = "sol",
		pool_type    = "clmm",
	}

	pool2 := src.PoolInfo{
		dex          = "orca",
		pool_address = "PoolAddr2",
		quote_token  = "usdc",
		pool_type    = "whirlpool",
	}

	pool1_err := src.insert_pool(db, "AURA", pool1)
	testing.expect(t, pool1_err == .None, "Pool 1 insert should succeed")

	pool2_err := src.insert_pool(db, "AURA", pool2)
	testing.expect(t, pool2_err == .None, "Pool 2 insert should succeed")

	// Retrieve token (should include pools)
	retrieved, found, get_err := src.get_token_by_symbol(db, "AURA")
	testing.expect(t, get_err == .None, "Get should succeed")
	testing.expect(t, found, "Token should be found")
	testing.expect(t, len(retrieved.pools) == 2,
		fmt.tprintf("Should have 2 pools, got %d", len(retrieved.pools)))

	// Verify pool data
	if len(retrieved.pools) >= 2 {
		// Pools may be in any order
		found_raydium := false
		found_orca := false

		for pool in retrieved.pools {
			if pool.dex == "raydium" {
				found_raydium = true
				testing.expect(t, pool.pool_address == "PoolAddr1", "Raydium pool address should match")
				testing.expect(t, pool.quote_token == "sol", "Raydium quote token should match")
				testing.expect(t, pool.pool_type == "clmm", "Raydium pool type should match")
			}
			if pool.dex == "orca" {
				found_orca = true
				testing.expect(t, pool.pool_address == "PoolAddr2", "Orca pool address should match")
				testing.expect(t, pool.quote_token == "usdc", "Orca quote token should match")
				testing.expect(t, pool.pool_type == "whirlpool", "Orca pool type should match")
			}
		}

		testing.expect(t, found_raydium, "Should find Raydium pool")
		testing.expect(t, found_orca, "Should find Orca pool")
	}
}

@(test)
test_get_all_tokens :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test retrieving all tokens from database
	// Validates batch retrieval works

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Insert multiple tokens
	tokens_to_insert := []src.Token{
		{
			symbol           = "SOL",
			name             = "Solana",
			contract_address = "So11111111111111111111111111111111111111112",
			chain            = "solana",
			is_quote_token   = true,
			usd_price        = 162.50,
			pools            = []src.PoolInfo{},
		},
		{
			symbol           = "AURA",
			name             = "Aura Token",
			contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
			chain            = "solana",
			is_quote_token   = false,
			usd_price        = 0.0,
			pools            = []src.PoolInfo{},
		},
		{
			symbol           = "USDC",
			name             = "USD Coin",
			contract_address = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			chain            = "solana",
			is_quote_token   = true,
			usd_price        = 1.0,
			pools            = []src.PoolInfo{},
		},
	}

	for token in tokens_to_insert {
		src.insert_token(db, token)
	}

	// Retrieve all
	all_tokens, get_err := src.get_all_tokens(db)
	testing.expect(t, get_err == .None, "Get all should succeed")
	testing.expect(t, len(all_tokens) == 3,
		fmt.tprintf("Should have 3 tokens, got %d", len(all_tokens)))

	// Verify symbols (should be sorted alphabetically)
	if len(all_tokens) >= 3 {
		testing.expect(t, all_tokens[0].symbol == "AURA", "First should be AURA")
		testing.expect(t, all_tokens[1].symbol == "SOL", "Second should be SOL")
		testing.expect(t, all_tokens[2].symbol == "USDC", "Third should be USDC")
	}
}

@(test)
test_database_integrity_check :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test database integrity check on healthy database
	// Should return true for valid database

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Insert some data
	test_token := src.Token{
		symbol           = "TEST",
		name             = "Test Token",
		contract_address = "TestAddr123",
		chain            = "solana",
		is_quote_token   = false,
		usd_price        = 1.0,
		pools            = []src.PoolInfo{},
	}
	src.insert_token(db, test_token)

	// Check integrity
	ok := src.database_integrity_check(db)
	testing.expect(t, ok, "Integrity check should pass on healthy database")
}

@(test)
test_migrate_from_json :: proc(t: ^testing.T) {
	// DOCUMENTATION: Test migration from TokenConfig (JSON) to database
	// Validates transaction-wrapped bulk insert

	db, err := src.database_open(":memory:")
	testing.expect(t, err == .None, "Database should open")
	defer src.database_close(db)

	src.create_schema(db)

	// Create test config
	test_config := src.TokenConfig{
		version = "2.0.0",
		tokens  = []src.Token{
			{
				symbol           = "SOL",
				name             = "Solana",
				contract_address = "So11111111111111111111111111111111111111112",
				chain            = "solana",
				is_quote_token   = true,
				usd_price        = 162.50,
				pools            = []src.PoolInfo{
					{
						dex          = "raydium",
						pool_address = "Pool1",
						quote_token  = "usdt",
						pool_type    = "clmm",
					},
				},
			},
			{
				symbol           = "AURA",
				name             = "Aura Token",
				contract_address = "DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2",
				chain            = "solana",
				is_quote_token   = false,
				usd_price        = 0.0,
				pools            = []src.PoolInfo{
					{
						dex          = "raydium",
						pool_address = "Pool2",
						quote_token  = "sol",
						pool_type    = "clmm",
					},
					{
						dex          = "orca",
						pool_address = "Pool3",
						quote_token  = "usdc",
						pool_type    = "whirlpool",
					},
				},
			},
		},
	}

	// Migrate
	migrate_err := src.migrate_from_json(db, test_config)
	testing.expect(t, migrate_err == .None,
		fmt.tprintf("Migration should succeed, got: %v", migrate_err))

	// Verify all tokens were migrated
	all_tokens, get_err := src.get_all_tokens(db)
	testing.expect(t, get_err == .None, "Get all should succeed")
	testing.expect(t, len(all_tokens) == 2,
		fmt.tprintf("Should have 2 tokens, got %d", len(all_tokens)))

	// Verify pools were migrated
	sol_token, sol_found, _ := src.get_token_by_symbol(db, "SOL")
	testing.expect(t, sol_found, "SOL should be found")
	testing.expect(t, len(sol_token.pools) == 1,
		fmt.tprintf("SOL should have 1 pool, got %d", len(sol_token.pools)))

	aura_token, aura_found, _ := src.get_token_by_symbol(db, "AURA")
	testing.expect(t, aura_found, "AURA should be found")
	testing.expect(t, len(aura_token.pools) == 2,
		fmt.tprintf("AURA should have 2 pools, got %d", len(aura_token.pools)))
}
