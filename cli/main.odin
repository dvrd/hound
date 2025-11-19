#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:os"
import "core:strconv"
import "core:strings"

import models "../core/models"
import db "../core/database"
import memory "../core/memory"
import blockchain "../core/blockchain"
import dex "../core/dex"
import wallet_backend "../core/wallet"
import wallet_mgr "../src/wallet_manager"
import token_cfg "../src/token_config"
import version "../src/version"

// handle_fetch_command implements the "hound fetch <symbol>" workflow
//
// Workflow:
// 1. Load token configuration
// 2. Check if --refresh flag present
// 3. Perform pool discovery (with force_refresh if --refresh)
// 4. Fetch price using discovered/configured pools
// 5. Display result
//
// Progress indicators for slow operations (>1s)
handle_fetch_command :: proc(symbol: string, force_refresh: bool) -> models.ErrorType {
	log.debugf("Fetch command: symbol=%s, force_refresh=%v", symbol, force_refresh)

	// Load token configuration
	log.debug("Loading token configuration")
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Find token by symbol
	log.debugf("Looking up token: %s", symbol)
	token, found := token_cfg.find_token_by_symbol(config, symbol)
	if !found {
		log.warnf("Token not found in configuration: %s", symbol)
		return .TokenNotConfigured
	}
	log.infof("Found token: %s (%s)", token.symbol, token.name)

	// Try on-chain fetch with existing or discovered pools
	price_data: models.PriceData
	err: models.ErrorType

	if len(token.pools) > 0 && !force_refresh {
		// Use existing configured pools
		log.infof("Using %d configured pool(s)", len(token.pools))
		price_data, err = dex.fetch_onchain_price(token)
		if err != .None {
			log.warnf("On-chain fetch failed (%v), falling back to API", err)
			fmt.eprintln("On-chain fetch failed, falling back to API...")
			price_data, err = dex.fetch_price(token.contract_address)
		} else {
			log.info("On-chain price fetch successful")
		}
	} else {
		// Pool discovery needed (no pools or force refresh)
		if force_refresh {
			fmt.eprintln("Refreshing pool discovery...")
		} else {
			fmt.eprintln("Discovering liquidity pools...")
		}

		// Pass force_refresh to bypass cache
		pool_info, discovery_err := discover_and_store_pools_with_refresh(token, force_refresh)
		if discovery_err == .None {
			log.infof("Pool discovery succeeded: %s pool at %s", pool_info.dex, pool_info.pool_address)
			fmt.eprintfln("Found pool on %s (stored for future use)", pool_info.dex)

			// Create temporary token with discovered pool
			token_with_pool := token
			token_with_pool.pools = []models.PoolInfo{pool_info}

			// Fetch price from discovered pool
			price_data, err = dex.fetch_onchain_price(token_with_pool)
			if err != .None {
				log.warnf("On-chain fetch failed after discovery (%v), falling back to API", err)
				fmt.eprintln("Pool fetch failed, falling back to API...")
				price_data, err = dex.fetch_price(token.contract_address)
			} else {
				log.info("On-chain price fetch successful from discovered pool")
			}
		} else {
			// Pool discovery failed - fallback to API
			log.warnf("Pool discovery failed (%v), falling back to API", discovery_err)
			fmt.eprintln("Pool discovery failed, using API...")
			price_data, err = dex.fetch_price(token.contract_address)
		}
	}

	// Reset request arena after all RPC operations complete
	memory.reset_request_arena()

	if err != .None {
		log.errorf("Price fetch failed with error: %v", err)
		return err
	}

	log.infof("Price fetched successfully: $%.6f", price_data.price_usd)

	// Display result
	format_price_output(token.symbol, price_data)

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

// Helper wrapper to pass force_refresh to pool discovery
discover_and_store_pools_with_refresh :: proc(token: models.Token, force_refresh: bool) -> (models.PoolInfo, models.ErrorType) {
	return token_cfg.discover_and_store_pools(token, force_refresh)
}

// handle_add_command implements "hound add <symbol> <name> <address>" workflow
//
// Workflow:
// 1. Validate arguments (symbol, name, contract address)
// 2. Validate address format (basic Solana address check)
// 3. Check if token already exists in database
// 4. Insert token into database
// 5. Optionally run pool discovery
//
// Returns: ErrorType
handle_add_command :: proc(symbol: string, name: string, address: string) -> models.ErrorType {
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
		fmt.eprintln("Error: Could not open database")
		return .DatabaseError
	}
	defer db.database_close(database)

	// Create schema if database is new
	if !db_exists {
		log.debug("Creating new database schema")
		schema_err := db.create_schema(database)
		if schema_err != .None {
			log.errorf("Failed to create schema: %v", schema_err)
			fmt.eprintln("Error: Could not create database schema")
			return .DatabaseError
		}
	}

	// Check if token already exists (case-insensitive)
	existing_token, found, lookup_err := db.get_token_by_symbol(database, symbol)
	if lookup_err != .None {
		log.errorf("Database lookup failed: %v", lookup_err)
		fmt.eprintln("Error: Database operation failed")
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
		fmt.eprintln("Error: Failed to add token to database")
		return .DatabaseError
	}

	fmt.eprintfln("✓ Token added successfully!")
	fmt.eprintln("")

	// Ask if user wants to discover pools now
	fmt.eprintln("Discover liquidity pools for this token? (y/n)")
	fmt.eprint("> ")

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
		fmt.eprintln("Discovering liquidity pools...")

		pool_info, discovery_err := token_cfg.discover_and_store_pools(token, false)
		if discovery_err == .None {
			fmt.eprintfln("✓ Found pool on %s", pool_info.dex)
			fmt.eprintfln("  Address: %s", pool_info.pool_address)
			if pool_info.liquidity_usd > 0 {
				fmt.eprintfln("  Liquidity: $%.0f", pool_info.liquidity_usd)
			}
			fmt.eprintln("")
			fmt.eprintfln("Try it: hound fetch %s", symbol)
		} else {
			fmt.eprintln("⚠ Pool discovery failed")
			fmt.eprintfln("You can try again later: hound fetch %s --refresh", symbol)
		}
	} else {
		fmt.eprintln("")
		fmt.eprintfln("Pool discovery skipped. Run 'hound fetch %s' when ready.", symbol)
	}

	// Reset command arena and log stats
	memory.reset_command_arena()
	memory.log_memory_stats()

	return .None
}

run :: proc() -> models.ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		log.debug("No arguments provided")
		return .MissingArgument
	}

	// Parse --memory-stats flag
	for arg in os.args {
		if arg == "--memory-stats" {
			memory.enable_memory_stats()
			break
		}
	}

	first_arg := os.args[1]
	log.debugf("First argument: %s", first_arg)

	// Handle version flags
	if first_arg == "--version" || first_arg == "-v" || first_arg == "version" {
		log.debug("Version request")
		fmt.println(version.get_version_info())
		return .None
	}

	// Parse "add" command to add new tokens
	// Syntax: hound add <symbol> <name> <address>
	// Note: "add" command doesn't need existing config (it creates/updates it)
	if first_arg == "add" {
		if len(os.args) < 5 {
			log.error("Missing arguments for add command")
			fmt.eprintln("Error: Missing arguments")
			fmt.eprintln("")
			fmt.eprintln("Usage: hound add <symbol> <name> <address>")
			fmt.eprintln("")
			fmt.eprintln("Example:")
			fmt.eprintln("  hound add bonk \"Bonk\" DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263")
			return .MissingArgument
		}

		symbol := os.args[2]
		name := os.args[3]
		address := os.args[4]

		log.debugf("Add command: symbol=%s, name=%s", symbol, name)
		return handle_add_command(symbol, name, address)
	}

	// Load token configuration (needed for all other commands)
	log.debug("Loading token configuration")
	config, config_err := token_cfg.load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Handle "list" command (use enhanced list with stats)
	if first_arg == "list" {
		log.debug("Listing all configured tokens with statistics")
		token_cfg.list_tokens_with_stats(config)

		// Reset command arena and log stats
		memory.reset_command_arena()
		memory.log_memory_stats()

		return .None
	}

	// Parse "fetch" command with optional --refresh flag
	// Syntax: hound fetch <symbol> [--refresh]
	if first_arg == "fetch" {
		if len(os.args) < 3 {
			log.error("Missing symbol argument for fetch command")
			fmt.eprintln("Error: Missing token symbol")
			fmt.eprintln("Usage: hound fetch <symbol> [--refresh]")
			return .MissingArgument
		}

		symbol := os.args[2]
		force_refresh := false

		// Check for --refresh flag
		if len(os.args) >= 4 && os.args[3] == "--refresh" {
			force_refresh = true
			log.debug("Refresh flag detected")
		}

		log.debugf("Fetch command: symbol=%s, refresh=%v", symbol, force_refresh)
		return handle_fetch_command(symbol, force_refresh)
	}

	// Backward compatibility: treat first arg as symbol (hound <symbol>)
	symbol := first_arg
	log.debugf("Backward compatibility mode: treating '%s' as symbol", symbol)
	return handle_fetch_command(symbol, false)
}

main :: proc() {
	log_level := log.Level.Info
	if ODIN_DEBUG {
	  log_level = log.Level.Debug
	}

	context.logger = log.create_console_logger(log_level, {.Level, .Terminal_Color})

	log.debug("Hound price fetcher starting")
	log.debugf("Log level: %v", log_level)

	// Initialize memory arenas
	mem_err := memory.memory_init()
	if mem_err != .None {
		log.errorf("Failed to initialize memory system: %v", mem_err)
		fmt.eprintln("Error: Memory initialization failed")
		os.exit(1)
	}

	err := run()

	// Get token for error messages that need it
	token := ""
	if len(os.args) >= 2 {
		token = os.args[1]
	}

	exit_code: int

	// Map errors to exit codes and messages
	#partial switch err {
	case .None:
		// Success - no message
		exit_code = 0

	case .MissingArgument:
		fmt.eprintln("Error: Missing token symbol")
		fmt.eprintln("")
		fmt.eprintln("Usage: hound <symbol>")
		fmt.eprintln("       hound list")
		fmt.eprintln("")
		fmt.eprintln("Examples:")
		fmt.eprintln("  hound aura       # Check AURA price")
		fmt.eprintln("  hound sol        # Check SOL price")
		fmt.eprintln("  hound list       # List all configured tokens")
		exit_code = 2  // Usage error

	case .InvalidToken:
		fmt.eprintfln("Error: Invalid token address: %s", token)
		fmt.eprintln("Token address must be a valid Solana contract address.")
		fmt.eprintln("Example: DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")
		exit_code = 78  // Configuration error

	case .TokenNotFound:
		fmt.eprintln("Error: Token not found on DexScreener")
		fmt.eprintln("This token may not be listed yet or the address is incorrect.")
		fmt.eprintln("Visit https://dexscreener.com to verify the token exists.")
		exit_code = 1  // General error

	case .NetworkTimeout:
		fmt.eprintln("Error: Request timed out")
		fmt.eprintln("Could not connect to DexScreener API within 10 seconds.")
		fmt.eprintln("Check your internet connection and try again.")
		exit_code = 69  // Service unavailable

	case .ConnectionFailed:
		fmt.eprintln("Error: Cannot connect to DexScreener API")
		fmt.eprintln("The service may be temporarily down.")
		fmt.eprintln("Try again in a few minutes.")
		exit_code = 69  // Service unavailable

	case .RateLimited:
		fmt.eprintln("Error: Rate limit exceeded")
		fmt.eprintln("DexScreener allows 300 requests per minute.")
		fmt.eprintln("Wait 60 seconds before trying again.")
		exit_code = 69  // Service unavailable

	case .ServerError:
		fmt.eprintln("Error: DexScreener API error")
		fmt.eprintln("The service is experiencing issues.")
		fmt.eprintln("Try again in a few minutes.")
		exit_code = 69  // Service unavailable

	case .InvalidResponse:
		fmt.eprintln("Error: Invalid response from DexScreener")
		fmt.eprintln("Received malformed data. This may be temporary.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")
		exit_code = 70  // Internal software error

	case .TokenNotConfigured:
		fmt.eprintfln("Error: Token '%s' not found in configuration", token)
		fmt.eprintln("Run 'hound list' to see available tokens.")
		fmt.eprintln("Add new tokens to ~/.config/hound/tokens.json")
		exit_code = 1  // General error

	case .ConfigNotFound:
		fmt.eprintln("Error: Configuration file not found")
		fmt.eprintln("Expected location: ~/.config/hound/tokens.json")
		fmt.eprintln("")
		fmt.eprintln("Create a config file with your token definitions:")
		fmt.eprintln("{")
		fmt.eprintln("  \"version\": \"1.0.0\",")
		fmt.eprintln("  \"tokens\": [")
		fmt.eprintln("    {")
		fmt.eprintln("      \"symbol\": \"aura\",")
		fmt.eprintln("      \"name\": \"AURA Memecoin\",")
		fmt.eprintln("      \"contract_address\": \"DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2\",")
		fmt.eprintln("      \"chain\": \"solana\"")
		fmt.eprintln("    }")
		fmt.eprintln("  ]")
		fmt.eprintln("}")
		exit_code = 78  // Configuration error

	case .ConfigParseError:
		fmt.eprintln("Error: Failed to parse configuration file")
		fmt.eprintln("Check that ~/.config/hound/tokens.json is valid JSON.")
		fmt.eprintln("Required format:")
		fmt.eprintln("  - version: string")
		fmt.eprintln("  - tokens: array of token objects")
		fmt.eprintln("  - Each token needs: symbol, name, contract_address, chain")
		exit_code = 78  // Configuration error

	case .RPCConnectionFailed:
		fmt.eprintln("Error: Cannot connect to Solana RPC")
		fmt.eprintln("The Solana network may be temporarily unavailable.")
		fmt.eprintln("Try again in a few minutes.")
		exit_code = 69  // Service unavailable

	case .RPCInvalidResponse:
		fmt.eprintln("Error: Invalid response from Solana RPC")
		fmt.eprintln("Received malformed data from blockchain node.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")
		exit_code = 70  // Internal software error

	case .PoolDataInvalid:
		fmt.eprintln("Error: Invalid pool data")
		fmt.eprintln("Pool structure validation failed.")
		fmt.eprintln("The pool address may be incorrect or the pool format changed.")
		exit_code = 70  // Internal software error

	case .VaultFetchFailed:
		fmt.eprintln("Error: Failed to fetch vault balances")
		fmt.eprintln("Could not retrieve token reserves from the pool.")
		fmt.eprintln("The RPC node may be experiencing issues.")
		exit_code = 69  // Service unavailable

	// Oracle errors
	case .OracleConnectionFailed:
		fmt.eprintln("Error: Cannot fetch SOL price")
		fmt.eprintln("Unable to connect to Jupiter or CoinGecko APIs.")
		fmt.eprintln("Check your internet connection and try again.")
		exit_code = 69  // Service unavailable

	case .OracleParseFailed:
		fmt.eprintln("Error: Invalid SOL price response")
		fmt.eprintln("Received malformed data from price API.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")
		exit_code = 70  // Internal software error

	case .OraclePriceInvalid:
		fmt.eprintln("Error: SOL price validation failed")
		fmt.eprintln("Received unreasonable price from API.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")
		exit_code = 70  // Internal software error

	// Database errors
	case .DatabaseError:
		fmt.eprintln("Error: Database operation failed")
		fmt.eprintln("Could not read or write to the token database.")
		fmt.eprintln("Check file permissions at ~/.config/hound/hound.db")
		exit_code = 74  // I/O error

	case .DatabaseCorrupted:
		fmt.eprintln("Error: Database integrity check failed")
		fmt.eprintln("The database file at ~/.config/hound/hound.db is corrupted.")
		fmt.eprintln("Delete the file to recreate it from tokens.json backup.")
		exit_code = 74  // I/O error

	case .MigrationFailed:
		fmt.eprintln("Error: JSON to database migration failed")
		fmt.eprintln("Could not migrate tokens.json to database.")
		fmt.eprintln("Check file permissions and ensure tokens.json is valid.")
		exit_code = 65  // Data format error

	// Pool Discovery errors
	case .PoolSearchFailed:
		fmt.eprintln("Error: Pool search failed")
		fmt.eprintln("Could not retrieve pool data from DexScreener API.")
		fmt.eprintln("This may be a temporary API issue. Try again in a few moments.")
		exit_code = 69  // Service unavailable

	case .NoPoolsFound:
		fmt.eprintfln("Error: No liquidity pools found for token '%s'", token)
		fmt.eprintln("This token may not have active trading pools yet.")
		fmt.eprintln("Pools must have at least $1,000 liquidity and max 1% fees.")
		exit_code = 1  // General error
	}

	// Cleanup memory arenas and logger before exit
	memory.memory_shutdown()
	log.destroy_console_logger(context.logger)
	os.exit(exit_code)
}

// ============================================================================
// Helper Functions
// ============================================================================

// format_price_output displays price data in a user-friendly format
format_price_output :: proc(symbol: string, data: models.PriceData) {
	sign := data.change_24h >= 0 ? "+" : ""
	fmt.printfln("%s: $%.6f (%s%.1f%%)",
		symbol, data.price_usd, sign, data.change_24h)
}
