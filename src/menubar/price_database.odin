// Price history database operations
package menubar

import "core:fmt"
import "core:os"
import "core:path/filepath"
import "core:time"
import "core:c"
import models "../../core/models"
import memory "../../core/memory"
import sqlite3 "../../vendor/odin-sqlite3"

// ============================================================================
// Types
// ============================================================================

PriceDB :: struct {
    handle: sqlite3.Connection,
    insert_stmt: sqlite3.Statement,  // Cached prepared statement
}

PriceHistoryEntry :: struct {
    id: i64,
    symbol: string,
    price_usd: f64,
    change_24h: f64,
    timestamp: i64,  // Unix timestamp
}

// ============================================================================
// Database Initialization
// ============================================================================

init_price_db :: proc(symbol: string, db_path: string = "") -> (PriceDB, models.ErrorType) {
    // Determine database path
    db_path_to_use: string

    if len(db_path) > 0 {
        // Custom path provided (e.g., ":memory:" for tests)
        db_path_to_use = db_path
    } else {
        // Default production path
        home, found := os.lookup_env("HOME")
        if !found || len(home) == 0 {
            fmt.eprintln("ERROR: Could not determine home directory")
            return {}, .ConfigNotFound
        }

        config_dir := filepath.join({home, ".config", "hound"})

        // Create directory if needed
        if !os.exists(config_dir) {
            err := os.make_directory(config_dir, 0o755)
            if err != nil {
                fmt.eprintfln("ERROR: Failed to create config directory: %v", err)
                return {}, .ConfigNotFound
            }
        }

        db_path_to_use = filepath.join({config_dir, "prices.db"})
    }

    db_path_cstr := fmt.ctprintf("%s", db_path_to_use)

    // Open database
    db_storage: sqlite3.Connection  // Storage for the connection handle
    db := &db_storage  // Pointer to the storage
    result := sqlite3.open(db_path_cstr, &db)
    if result != .Ok {
        errmsg := sqlite3.errmsg(db)
        fmt.eprintfln("ERROR: Failed to open database: %s", errmsg)
        return {}, .ConfigNotFound
    }

    // Create table if not exists
    create_table_sql := `
        CREATE TABLE IF NOT EXISTS prices (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL,
            price_usd REAL NOT NULL,
            change_24h REAL NOT NULL,
            timestamp INTEGER NOT NULL
        );
        CREATE INDEX IF NOT EXISTS idx_symbol_timestamp
            ON prices(symbol, timestamp DESC);
    `

    create_table_cstr := cstring(raw_data(create_table_sql))
    exec_result := sqlite3.exec(db, create_table_cstr, nil, nil, nil)
    if exec_result != .Ok {
        errmsg := sqlite3.errmsg(db)
        fmt.eprintfln("ERROR: Failed to create table: %s", errmsg)
        sqlite3.close(db)
        return {}, .ConfigNotFound
    }

    // Prepare insert statement
    insert_sql := `INSERT INTO prices (symbol, price_usd, change_24h, timestamp)
                    VALUES (?, ?, ?, ?)`

    stmt_storage: sqlite3.Statement  // Storage for the statement handle
    stmt := &stmt_storage  // Pointer to the storage
    insert_sql_cstr := cstring(raw_data(insert_sql))
    prep_result := sqlite3.prepare_v2(
        db,
        insert_sql_cstr,
        -1,  // Read to null terminator
        &stmt,
        nil,
    )

    if prep_result != .Ok {
        errmsg := sqlite3.errmsg(db)
        fmt.eprintfln("ERROR: Failed to prepare statement: %s", errmsg)
        sqlite3.close(db)
        return {}, .ConfigNotFound
    }

    return PriceDB{handle = db_storage, insert_stmt = stmt_storage}, .None
}

// ============================================================================
// Cleanup
// ============================================================================

close_price_db :: proc(db: ^PriceDB) {
    if db.insert_stmt != nil {
        sqlite3.finalize(&db.insert_stmt)
        db.insert_stmt = nil
    }

    if db.handle != nil {
        sqlite3.close(&db.handle)
        db.handle = nil
    }
}

// ============================================================================
// Insert Price
// ============================================================================

save_price :: proc(
    db: ^PriceDB,
    symbol: string,
    data: models.PriceData,
) -> models.ErrorType {
    if db.insert_stmt == nil {
        fmt.eprintln("ERROR: Database not initialized")
        return .ConfigNotFound
    }

    // Bind parameters (1-indexed!)
    symbol_cstr := fmt.ctprintf("%s", symbol)
    sqlite3.bind_text(&db.insert_stmt, 1, symbol_cstr, -1, .Transient)
    sqlite3.bind_double(&db.insert_stmt, 2, data.price_usd)
    sqlite3.bind_double(&db.insert_stmt, 3, data.change_24h)

    timestamp := time.now()
    unix_time := time.time_to_unix(timestamp)
    sqlite3.bind_int64(&db.insert_stmt, 4, c.longlong(unix_time))

    // Execute
    result := sqlite3.step(&db.insert_stmt)
    if result != .Done {
        errmsg := sqlite3.errmsg(&db.handle)
        fmt.eprintfln("ERROR: Failed to insert price: %s", errmsg)
        sqlite3.reset(&db.insert_stmt)  // Reset statement even on error
        return .ConfigNotFound  // Degrade gracefully - don't crash app
    }

    // Reset statement for next use
    sqlite3.reset(&db.insert_stmt)

    return .None
}

// ============================================================================
// Query History
// ============================================================================

get_recent_prices :: proc(
    db: ^PriceDB,
    symbol: string,
    limit: int = 10,
) -> ([]PriceHistoryEntry, models.ErrorType) {
    // Use command arena for all allocations - data lives until command completes
    context.allocator = memory.command_allocator()

    query_sql := fmt.ctprintf(
        "SELECT id, symbol, price_usd, change_24h, timestamp FROM prices WHERE symbol = ? ORDER BY timestamp DESC, id DESC LIMIT %d",
        limit,
    )

    stmt_storage: sqlite3.Statement
    stmt := &stmt_storage
    result := sqlite3.prepare_v2(&db.handle, query_sql, -1, &stmt, nil)
    if result != .Ok {
        errmsg := sqlite3.errmsg(&db.handle)
        fmt.eprintfln("ERROR: Failed to prepare query: %s", errmsg)
        return nil, .ConfigNotFound
    }
    defer sqlite3.finalize(stmt)

    // Bind symbol
    symbol_cstr := fmt.ctprintf("%s", symbol)
    sqlite3.bind_text(stmt, 1, symbol_cstr, -1, .Transient)

    // Fetch rows
    entries := make([dynamic]PriceHistoryEntry)

    for {
        step_result := sqlite3.step(stmt)

        if step_result == .Row {
            entry: PriceHistoryEntry
            entry.id = i64(sqlite3.column_int64(stmt, 0))
            entry.symbol = string(sqlite3.column_text(stmt, 1))
            entry.price_usd = f64(sqlite3.column_double(stmt, 2))
            entry.change_24h = f64(sqlite3.column_double(stmt, 3))
            entry.timestamp = i64(sqlite3.column_int64(stmt, 4))

            append(&entries, entry)
        } else if step_result == .Done {
            break
        } else {
            errmsg := sqlite3.errmsg(&db.handle)
            fmt.eprintfln("ERROR: Query failed: %s", errmsg)
            // NO delete needed - command arena will clean up
            return nil, .ConfigNotFound
        }
    }

    return entries[:], .None
}
