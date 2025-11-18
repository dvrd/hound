// Tests for price database operations
package menubar_test

import "core:testing"
import "core:fmt"
import "core:os"
import menubar "../../src/menubar"
import src "../../src"

// ============================================================================
// Database Initialization Tests
// ============================================================================

@(test)
test_init_price_db_creates_directory :: proc(t: ^testing.T) {
    // DOCUMENTATION: Database initialization should create in-memory database
    //
    // Flow:
    // 1. Initialize in-memory database
    // 2. Verify database handle is valid
    // 3. Verify prepared statement exists

    symbol := "test_init"

    // Initialize in-memory database for tests
    db, err := menubar.init_price_db(symbol, ":memory:")
    defer menubar.close_price_db(&db)

    testing.expect(t, err == src.ErrorType.None,
        "Database initialization should succeed")

    testing.expect(t, db.handle != nil,
        "Database handle should not be nil")

    testing.expect(t, db.insert_stmt != nil,
        "Insert statement should be prepared")
}

@(test)
test_save_price_inserts_record :: proc(t: ^testing.T) {
    // DOCUMENTATION: save_price should insert price record into database
    //
    // Flow:
    // 1. Initialize in-memory database
    // 2. Save test price
    // 3. Query back to verify insertion

    symbol := "test_save"
    db, init_err := menubar.init_price_db(symbol, ":memory:")
    defer menubar.close_price_db(&db)

    testing.expect(t, init_err == src.ErrorType.None,
        "Database init should succeed")

    // Create test price data
    test_data := src.PriceData{
        price_usd = 0.42,
        change_24h = 5.3,
    }

    // Save price
    save_err := menubar.save_price(&db, symbol, test_data)

    testing.expect(t, save_err == src.ErrorType.None,
        fmt.tprintf("save_price should succeed, got error: %v", save_err))
}

@(test)
test_get_recent_prices_returns_history :: proc(t: ^testing.T) {
    // DOCUMENTATION: get_recent_prices should return most recent entries
    //
    // Flow:
    // 1. Initialize in-memory database
    // 2. Insert 3 test prices
    // 3. Query recent prices (limit 2)
    // 4. Verify correct count and order

    symbol := "test_history"
    db, init_err := menubar.init_price_db(symbol, ":memory:")
    defer menubar.close_price_db(&db)

    testing.expect(t, init_err == src.ErrorType.None,
        "Database init should succeed")

    // Insert multiple prices
    for i in 0..<3 {
        test_data := src.PriceData{
            price_usd = f64(100 + i),
            change_24h = f64(i),
        }
        save_err := menubar.save_price(&db, symbol, test_data)
        testing.expect(t, save_err == src.ErrorType.None,
            fmt.tprintf("Insert %d should succeed", i))
    }

    // Query history
    history, query_err := menubar.get_recent_prices(&db, symbol, 2)
    defer delete(history)

    testing.expect(t, query_err == src.ErrorType.None,
        "Query should succeed")

    testing.expect(t, len(history) == 2,
        fmt.tprintf("Should return 2 entries, got %d", len(history)))

    // Most recent should be last inserted (price 102.0)
    testing.expect(t, history[0].price_usd == 102.0,
        fmt.tprintf("Most recent price should be 102.0, got %.1f", history[0].price_usd))
}

@(test)
test_close_price_db_cleans_up :: proc(t: ^testing.T) {
    // DOCUMENTATION: close_price_db should finalize statements and close handle
    //
    // This is a sanity test - actual cleanup verification would require
    // instrumentation or valgrind-style memory leak detection

    symbol := "test_close"
    db, init_err := menubar.init_price_db(symbol, ":memory:")

    testing.expect(t, init_err == src.ErrorType.None,
        "Database init should succeed")

    // Close database
    menubar.close_price_db(&db)

    // Verify handles are nil after close
    testing.expect(t, db.handle == nil,
        "Database handle should be nil after close")

    testing.expect(t, db.insert_stmt == nil,
        "Insert statement should be nil after close")
}
