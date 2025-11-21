// Swap history database operations
// Query and retrieval functions for historical swap transactions
package database

import "core:fmt"
import "core:log"
import "core:time"
import models "../../lib/models"
import sqlite3 "../../sqlite3"

// ============================================================================
// Swap History Types
// ============================================================================

// SwapHistoryEntry represents a single swap transaction from history
SwapHistoryEntry :: struct {
	id:              i64,
	wallet_address:  string,
	signature:       string,
	timestamp:       i64,

	// Token details
	input_mint:      string,
	input_symbol:    string,
	input_amount:    f64,
	output_mint:     string,
	output_symbol:   string,
	output_amount:   f64,

	// Execution details
	slot:            i64,
	status:          string,
	price_impact:    f64,
	slippage_bps:    i64,

	// Fees (lamports)
	network_fee:     i64,
	priority_fee:    i64,

	// DEX routing
	dex:             string,

	// Error tracking
	error_message:   string,
}

// ============================================================================
// Query Functions
// ============================================================================

// get_swap_history retrieves swap history for a wallet address
//
// Parameters:
//   - db: Database handle
//   - wallet_address: Wallet address to query (empty string = all wallets)
//   - limit: Maximum number of results (0 = no limit)
//
// Returns: Array of SwapHistoryEntry, error status
get_swap_history :: proc(
	db: ^Database,
	wallet_address: string = "",
	limit: int = 10,
) -> (entries: []SwapHistoryEntry, err: models.ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")
	assert(limit >= 0, "Limit cannot be negative")

	log.debugf("Querying swap history: wallet=%s, limit=%d", wallet_address, limit)

	// Build SQL query based on whether wallet_address is provided
	sql: string
	if len(wallet_address) > 0 {
		sql = `SELECT id, wallet_address, signature, timestamp,
		              input_mint, input_symbol, input_amount,
		              output_mint, output_symbol, output_amount,
		              slot, status, price_impact, slippage_bps,
		              network_fee, priority_fee, dex, error_message
		       FROM swap_history
		       WHERE wallet_address = ?1
		       ORDER BY timestamp DESC`

		if limit > 0 {
			sql = fmt.tprintf("%s LIMIT %d", sql, limit)
		}
	} else {
		sql = `SELECT id, wallet_address, signature, timestamp,
		              input_mint, input_symbol, input_amount,
		              output_mint, output_symbol, output_amount,
		              slot, status, price_impact, slippage_bps,
		              network_fee, priority_fee, dex, error_message
		       FROM swap_history
		       ORDER BY timestamp DESC`

		if limit > 0 {
			sql = fmt.tprintf("%s LIMIT %d", sql, limit)
		}
	}

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare statement: %v", prep_result)
		return nil, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	// Bind wallet address parameter if provided
	if len(wallet_address) > 0 {
		sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet_address)), i32(len(wallet_address)), nil)
	}

	// Collect results
	result_entries := make([dynamic]SwapHistoryEntry)

	for {
		step_result := sqlite3.step(stmt)

		if step_result == .Row {
			entry: SwapHistoryEntry

			// Extract columns (0-indexed for sqlite3.column_*)
			entry.id = sqlite3.column_int64(stmt, 0)
			entry.wallet_address = string(sqlite3.column_text(stmt, 1))
			entry.signature = string(sqlite3.column_text(stmt, 2))
			entry.timestamp = sqlite3.column_int64(stmt, 3)

			entry.input_mint = string(sqlite3.column_text(stmt, 4))
			entry.input_symbol = string(sqlite3.column_text(stmt, 5))
			entry.input_amount = sqlite3.column_double(stmt, 6)

			entry.output_mint = string(sqlite3.column_text(stmt, 7))
			entry.output_symbol = string(sqlite3.column_text(stmt, 8))
			entry.output_amount = sqlite3.column_double(stmt, 9)

			entry.slot = sqlite3.column_int64(stmt, 10)
			entry.status = string(sqlite3.column_text(stmt, 11))
			entry.price_impact = sqlite3.column_double(stmt, 12)
			entry.slippage_bps = sqlite3.column_int64(stmt, 13)

			entry.network_fee = sqlite3.column_int64(stmt, 14)
			entry.priority_fee = sqlite3.column_int64(stmt, 15)

			entry.dex = string(sqlite3.column_text(stmt, 16))

			// error_message can be NULL
			error_text := sqlite3.column_text(stmt, 17)
			if error_text != nil {
				entry.error_message = string(error_text)
			}

			append(&result_entries, entry)
		} else if step_result == .Done {
			break
		} else {
			log.errorf("Error stepping through results: %v", step_result)
			delete(result_entries)
			return nil, .DatabaseError
		}
	}

	log.debugf("Found %d swap history entries", len(result_entries))
	return result_entries[:], .None
}

// get_swap_count retrieves the total count of swaps for a wallet
//
// Parameters:
//   - db: Database handle
//   - wallet_address: Wallet address to query (empty string = all wallets)
//
// Returns: Count of swaps, error status
get_swap_count :: proc(
	db: ^Database,
	wallet_address: string = "",
) -> (count: i64, err: models.ErrorType) {
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	sql: string
	if len(wallet_address) > 0 {
		sql = `SELECT COUNT(*) FROM swap_history WHERE wallet_address = ?1`
	} else {
		sql = `SELECT COUNT(*) FROM swap_history`
	}

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare count statement: %v", prep_result)
		return 0, .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	// Bind wallet address parameter if provided
	if len(wallet_address) > 0 {
		sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet_address)), i32(len(wallet_address)), nil)
	}

	step_result := sqlite3.step(stmt)
	if step_result != .Row {
		log.errorf("Failed to get count: %v", step_result)
		return 0, .DatabaseError
	}

	count = sqlite3.column_int64(stmt, 0)
	return count, .None
}
