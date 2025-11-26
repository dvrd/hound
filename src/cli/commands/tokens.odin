// Token commands implementation
// Handles all token-related operations: list, fetch, add
package commands

import "core:fmt"
import "core:log"
import "core:os"
import "core:strings"
import models "../../lib/models"
import db "../../lib/database"
import memory "../../lib/memory"
import token_cfg "../../lib/config"
import services "../../lib/services"
import output "../output"

// ============================================================================
// Help Message
// ============================================================================

print_tokens_help :: proc() {
	fmt.println("")
	fmt.println("hound tokens - Manage and query token information")
	fmt.println("")
	fmt.println("USAGE:")
	fmt.println("  hound tokens <subcommand> [arguments]")
	fmt.println("")
	fmt.println("SUBCOMMANDS:")
	fmt.println("  list                     List all configured tokens")
	fmt.println("  fetch <symbol|address>   Fetch detailed token information")
	fmt.println("  add <symbol> <name> <address>  Add a new token")
	fmt.println("")
	fmt.println("EXAMPLES:")
	fmt.println("  hound tokens list")
	fmt.println("  hound tokens fetch aura")
	fmt.println("  hound tokens fetch EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	fmt.println("  hound tokens add AURA \"AURA Memecoin\" DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")
	fmt.println("")
}

// ============================================================================
// Subcommand Router
// ============================================================================

// handle_tokens routes to the appropriate token subcommand
//
// Subcommands:
// - list: Display all configured tokens
// - fetch: Get detailed token information
// - add: Add a new token to the database
handle_tokens :: proc(args: []string) -> models.ErrorType {
	log.debug("Tokens command invoked")

	// Check for subcommand
	if len(args) < 1 {
		print_tokens_help()
		return .None
	}

	subcommand := args[0]

	// Route to subcommand handlers
	switch subcommand {
	case "help", "--help", "-h":
		print_tokens_help()
		return .None

	case "list":
		// Load token configuration
		config, config_err := token_cfg.load_token_config()
		if config_err != .None {
			log.errorf("Failed to load token config: %v", config_err)
			return config_err
		}
		return handle_tokens_list(config)

	case "fetch":
		if len(args) < 2 {
			log.error("Missing token symbol/address for fetch command")
			fmt.eprintln("")
			fmt.eprintln("Usage: hound tokens fetch <symbol|address> [--refresh]")
			fmt.eprintln("")
			fmt.eprintln("Examples:")
			fmt.eprintln("  hound tokens fetch aura")
			fmt.eprintln("  hound tokens fetch EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
			fmt.eprintln("  hound tokens fetch sol --refresh  # Force pool rediscovery")
			return .MissingArgument
		}

		symbol_or_address := strings.to_lower(args[1])
		force_refresh := false

		// Check for --refresh flag
		for arg in args[2:] {
			if arg == "--refresh" {
				force_refresh = true
				break
			}
		}

		return handle_tokens_fetch(symbol_or_address, force_refresh)

	case "add":
		if len(args) < 4 {
			log.error("Missing arguments for add command")
			fmt.eprintln("")
			fmt.eprintln("Usage: hound tokens add <symbol> <name> <contract_address>")
			fmt.eprintln("")
			fmt.eprintln("Example:")
			fmt.eprintln("  hound tokens add AURA \"AURA Memecoin\" DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")
			return .MissingArgument
		}

		symbol := strings.to_lower(args[1])
		name := args[2]
		address := args[3]

		return handle_tokens_add(symbol, name, address)

	case:
		// Unknown subcommand - show help
		log.errorf("Unknown tokens subcommand: %s", subcommand)
		fmt.eprintfln("\nError: Unknown subcommand '%s'", subcommand)
		print_tokens_help()
		return .InvalidToken
	}
}

// ============================================================================
// Subcommand Implementations
// ============================================================================

// handle_tokens_list implements "hound tokens list" workflow
//
// Workflow:
// 1. Open database for pool statistics
// 2. Display tokens with pool stats using formatter
// 3. Fallback to basic list if database unavailable
handle_tokens_list :: proc(config: models.TokenConfig) -> models.ErrorType {
	log.debug("Listing all configured tokens with statistics")

	// Try to open database for stats
	db_path := token_cfg.get_database_path()
	database, db_err := db.database_open(db_path)
	if db_err != .None {
		log.warnf("Database unavailable, falling back to basic list")
		output.format_basic_token_list(config.tokens)

		// Reset command arena and log stats
		memory.reset_command_arena()
		memory.log_memory_stats()

		return .None
	}
	defer db.database_close(database)

	// Display tokens with comprehensive stats
	output.format_token_list(config.tokens, database)

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// handle_tokens_fetch implements "hound tokens fetch <symbol|address>" workflow
//
// Fetches comprehensive token information including:
// - Market cap, FDV, liquidity (from DexScreener)
// - Total supply, top holders (from Solana RPC)
// - 24h volume, transactions, price changes (from DexScreener)
// - Multiple symbols (if multiple pools exist)
handle_tokens_fetch :: proc(symbol_or_address: string, force_refresh: bool) -> models.ErrorType {
	log.debugf("Fetching extended token info for: %s (refresh=%v)", symbol_or_address, force_refresh)

	// Determine if input is a token address or symbol
	mint_address := ""

	// Check if it's a valid Solana address (32-44 characters, base58)
	if len(symbol_or_address) >= 32 && len(symbol_or_address) <= 44 {
		// Treat as direct mint address
		mint_address = symbol_or_address
	} else {
		// Treat as symbol - look up in database
		db_path := token_cfg.get_database_path()
		database, db_err := db.database_open(db_path)
		if db_err != .None {
			log.errorf("Failed to open database: %v", db_err)
			return .DatabaseError
		}
		defer db.database_close(database)

		token, found, lookup_err := db.get_token_by_symbol(database, symbol_or_address)
		if lookup_err != .None {
			log.errorf("Database lookup failed: %v", lookup_err)
			return .DatabaseError
		}

		if !found {
			log.warnf("Token '%s' not found in database", symbol_or_address)
			return .TokenNotConfigured
		}

		mint_address = token.contract_address
	}

	log.debugf("Resolved mint address: %s", mint_address)

	// Get RPC endpoint (use Helius if available, fallback to public)
	rpc_endpoint := os.get_env_alloc("HELIUS_RPC_URL", context.temp_allocator)
	if len(rpc_endpoint) == 0 {
		rpc_endpoint = "https://api.mainnet-beta.solana.com"
	}

	// Fetch extended token information
	token_info, info_err := services.fetch_extended_token_info(mint_address, rpc_endpoint)
	if info_err != .None {
		log.errorf("Failed to fetch token info: %v", info_err)
		return info_err
	}

	// Display formatted output
	output.format_extended_token_info(token_info)

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// handle_tokens_add implements "hound tokens add <symbol> <name> <address>" workflow
//
// Delegates to the existing add implementation
handle_tokens_add :: proc(symbol: string, name: string, address: string) -> models.ErrorType {
	return handle_add(symbol, name, address)
}
