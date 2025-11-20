// Add command implementation
// Adds new tokens to the database with optional pool discovery
package commands

import "core:fmt"
import "core:log"
import "core:os"
import "core:strings"
import models "../../core/models"
import db "../../core/database"
import memory "../../core/memory"
import token_cfg "../../src/token_config"
import output "../output"

// ============================================================================
// Add Command
// ============================================================================

// handle_add implements "hound add <symbol> <name> <address>" workflow
//
// Workflow:
// 1. Validate arguments (symbol, name, contract address)
// 2. Validate address format (basic Solana address check)
// 3. Check if token already exists in database
// 4. Insert token into database
// 5. Optionally run pool discovery
//
// Returns: ErrorType for consistent error handling
handle_add :: proc(symbol: string, name: string, address: string) -> models.ErrorType {
	log.debugf("Add command: symbol=%s, name=%s, address=%s", symbol, name, address)

	// Validate contract address format (Solana addresses are base58, typically 32-44 chars)
	if len(address) < 32 || len(address) > 44 {
		log.errorf("Invalid contract address length: %d", len(address))
		fmt.eprintln("Error: Invalid Solana contract address")
		fmt.eprintfln("Address must be 32-44 characters (base58)")
		fmt.eprintfln("Example: DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263")
		return .InvalidToken
	}

	// Open or create database
	db_path := token_cfg.get_database_path()
	db_exists := os.exists(db_path)

	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.errorf("Failed to open database: %v", db_err)
		output.print_error("Could not open database")
		return .DatabaseError
	}
	defer db.database_close(database)

	// Create schema if database is new
	if !db_exists {
		log.debug("Creating new database schema")
		schema_err := db.create_schema(database)
		if schema_err != .None {
			log.errorf("Failed to create schema: %v", schema_err)
			output.print_error("Could not create database schema")
			return .DatabaseError
		}
	}

	// Check if token already exists (case-insensitive)
	existing_token, found, lookup_err := db.get_token_by_symbol(database, symbol)
	if lookup_err != .None {
		log.errorf("Database lookup failed: %v", lookup_err)
		output.print_error("Database operation failed")
		return .DatabaseError
	}

	if found {
		log.warnf("Token '%s' already exists in database", symbol)
		fmt.eprintfln("Error: Token '%s' already exists", symbol)
		fmt.eprintfln("Existing: %s (%s)", existing_token.name, existing_token.contract_address)
		return .InvalidToken
	}

	// Create token struct
	token := models.Token{
		symbol           = symbol,
		name             = name,
		contract_address = address,
		chain            = "solana",
		is_quote_token   = false,
		usd_price        = 0.0,
		pools            = []models.PoolInfo{},
	}

	// Insert token into database
	fmt.eprintfln("Adding token: %s (%s)", name, symbol)
	insert_err := db.insert_token(database, token)
	if insert_err != .None {
		log.errorf("Failed to insert token: %v", insert_err)
		output.print_error("Failed to add token to database")
		return .DatabaseError
	}

	output.print_success("Token added successfully!")
	fmt.eprintln("")

	// Ask if user wants to discover pools now
	output.print_pool_discovery_prompt(symbol)

	// Read user input
	buffer: [256]byte
	n, read_err := os.read(os.stdin, buffer[:])
	if read_err != 0 {
		log.debug("Failed to read user input, skipping pool discovery")
		return .None
	}

	response := string(buffer[:n])
	response = strings.trim_space(response)
	response_lower := strings.to_lower(response)

	if response_lower == "y" || response_lower == "yes" {
		fmt.eprintln("")
		output.print_progress("Discovering liquidity pools...")

		pool_info, discovery_err := token_cfg.discover_and_store_pools(token, false)
		if discovery_err == .None {
			output.format_pool_info(pool_info)
			fmt.eprintln("")
			fmt.eprintfln("Try it: hound fetch %s", symbol)
		} else {
			output.print_pool_discovery_failed(symbol)
		}
	} else {
		output.print_pool_discovery_skip(symbol)
	}

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}
