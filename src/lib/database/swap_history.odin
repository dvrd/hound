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

// ============================================================================
// Insert Functions
// ============================================================================

// insert_swap_history records a completed swap transaction
//
// ASSERTION 1: Database handle must not be nil
// ASSERTION 2: Wallet address must not be empty
// ASSERTION 3: Signature must not be empty
//
// Parameters:
//   - db: Database handle
//   - wallet_address: Wallet address that executed the swap
//   - result: SwapTransactionResult from transaction execution
//   - slippage_bps: Slippage tolerance used (in basis points)
//
// Returns: Error status
insert_swap_history :: proc(
	db: ^Database,
	wallet_address: string,
	result: models.SwapTransactionResult,
	slippage_bps: int,
) -> models.ErrorType {
	// ASSERTION 1: Validate database handle
	assert(db != nil, "Database handle cannot be nil")
	assert(db.handle != nil, "Database connection cannot be nil")

	// ASSERTION 2: Validate wallet address
	assert(len(wallet_address) > 0, "Wallet address cannot be empty")

	// ASSERTION 3: Validate signature
	assert(len(result.signature) > 0, "Transaction signature cannot be empty")

	log.debugf("Inserting swap history: wallet=%s, signature=%s", wallet_address, result.signature)

	// Use current timestamp if block_time is 0
	timestamp := result.block_time
	if timestamp == 0 {
		timestamp = time.time_to_unix(time.now())
	}

	// Build INSERT statement
	sql := `INSERT INTO swap_history (
		wallet_address, signature, timestamp,
		input_mint, input_symbol, input_amount,
		output_mint, output_symbol, output_amount,
		slot, status, price_impact, slippage_bps,
		network_fee, priority_fee, dex, error_message
	) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17)`

	stmt: ^sqlite3.Statement
	prep_result := sqlite3.prepare_v2(db.handle, cstring(raw_data(sql)), i32(len(sql)), &stmt, nil)
	if prep_result != .Ok {
		log.errorf("Failed to prepare insert statement: %v", prep_result)
		return .DatabaseError
	}
	defer sqlite3.finalize(stmt)

	// Bind parameters (1-indexed)
	sqlite3.bind_text(stmt, 1, cstring(raw_data(wallet_address)), i32(len(wallet_address)), nil)
	sqlite3.bind_text(stmt, 2, cstring(raw_data(result.signature)), i32(len(result.signature)), nil)
	sqlite3.bind_int64(stmt, 3, timestamp)

	sqlite3.bind_text(stmt, 4, cstring(raw_data(result.input_mint)), i32(len(result.input_mint)), nil)
	sqlite3.bind_text(stmt, 5, cstring(raw_data(result.input_symbol)), i32(len(result.input_symbol)), nil)
	sqlite3.bind_double(stmt, 6, result.input_amount)

	sqlite3.bind_text(stmt, 7, cstring(raw_data(result.output_mint)), i32(len(result.output_mint)), nil)
	sqlite3.bind_text(stmt, 8, cstring(raw_data(result.output_symbol)), i32(len(result.output_symbol)), nil)
	sqlite3.bind_double(stmt, 9, result.output_amount)

	sqlite3.bind_int64(stmt, 10, i64(result.slot))
	sqlite3.bind_text(stmt, 11, cstring(raw_data(result.status)), i32(len(result.status)), nil)
	sqlite3.bind_double(stmt, 12, result.price_impact)
	sqlite3.bind_int64(stmt, 13, i64(slippage_bps))

	sqlite3.bind_int64(stmt, 14, i64(result.network_fee))
	sqlite3.bind_int64(stmt, 15, i64(result.priority_fee))
	sqlite3.bind_text(stmt, 16, cstring(raw_data(result.dex)), i32(len(result.dex)), nil)

	// Bind error_message (NULL if empty)
	if len(result.error_message) > 0 {
		sqlite3.bind_text(stmt, 17, cstring(raw_data(result.error_message)), i32(len(result.error_message)), nil)
	} else {
		sqlite3.bind_null(stmt, 17)
	}

	// Execute insert
	step_result := sqlite3.step(stmt)
	if step_result != .Done {
		log.errorf("Failed to insert swap history: %v", step_result)
		return .DatabaseError
	}

	log.infof("Swap history recorded: signature=%s", result.signature)
	return .None
}
