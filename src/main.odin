#+feature global-context
package main

import "core:fmt"
import "core:log"
import "core:os"
import "core:strconv"

// handle_fetch_command implements the "hound fetch <symbol>" workflow (Phase 5.3)
//
// Workflow:
// 1. Load token configuration
// 2. Check if --refresh flag present
// 3. Perform pool discovery (with force_refresh if --refresh)
// 4. Fetch price using discovered/configured pools
// 5. Display result
//
// Progress indicators for slow operations (>1s)
handle_fetch_command :: proc(symbol: string, force_refresh: bool) -> ErrorType {
	log.debugf("Fetch command: symbol=%s, force_refresh=%v", symbol, force_refresh)

	// Load token configuration
	log.debug("Loading token configuration")
	config, config_err := load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Find token by symbol
	log.debugf("Looking up token: %s", symbol)
	token, found := find_token_by_symbol(config, symbol)
	if !found {
		log.warnf("Token not found in configuration: %s", symbol)
		return .TokenNotConfigured
	}
	log.infof("Found token: %s (%s)", token.symbol, token.name)

	// Try on-chain fetch with existing or discovered pools
	price_data: PriceData
	err: ErrorType

	if len(token.pools) > 0 && !force_refresh {
		// Use existing configured pools
		log.infof("Using %d configured pool(s)", len(token.pools))
		price_data, err = fetch_onchain_price(token)
		if err != .None {
			log.warnf("On-chain fetch failed (%v), falling back to API", err)
			fmt.eprintln("On-chain fetch failed, falling back to API...")
			price_data, err = fetch_price(token.contract_address)
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
			token_with_pool.pools = []PoolInfo{pool_info}

			// Fetch price from discovered pool
			price_data, err = fetch_onchain_price(token_with_pool)
			if err != .None {
				log.warnf("On-chain fetch failed after discovery (%v), falling back to API", err)
				fmt.eprintln("Pool fetch failed, falling back to API...")
				price_data, err = fetch_price(token.contract_address)
			} else {
				log.info("On-chain price fetch successful from discovered pool")
			}
		} else {
			// Pool discovery failed - fallback to API
			log.warnf("Pool discovery failed (%v), falling back to API", discovery_err)
			fmt.eprintln("Pool discovery failed, using API...")
			price_data, err = fetch_price(token.contract_address)
		}
	}

	if err != .None {
		log.errorf("Price fetch failed with error: %v", err)
		return err
	}

	log.infof("Price fetched successfully: $%.6f", price_data.price_usd)

	// Display result
	format_price_output(token.symbol, price_data)

	return .None
}

// Helper wrapper to pass force_refresh to pool discovery
discover_and_store_pools_with_refresh :: proc(token: Token, force_refresh: bool) -> (PoolInfo, ErrorType) {
	return discover_and_store_pools(token, force_refresh)
}

run :: proc() -> ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		log.debug("No arguments provided")
		return .MissingArgument
	}

	first_arg := os.args[1]
	log.debugf("First argument: %s", first_arg)

	// Handle version flags
	if first_arg == "--version" || first_arg == "-v" || first_arg == "version" {
		log.debug("Version request")
		fmt.println(get_version_info())
		return .None
	}

	// Load token configuration (needed for all commands except version)
	log.debug("Loading token configuration")
	config, config_err := load_token_config()
	if config_err != .None {
		log.errorf("Failed to load token config: %v", config_err)
		return config_err
	}
	log.debugf("Loaded %d tokens from configuration", len(config.tokens))

	// Handle "list" command (Phase 5.3: use enhanced list with stats)
	if first_arg == "list" {
		log.debug("Listing all configured tokens with statistics")
		list_tokens_with_stats(config)
		return .None
	}

	// Phase 5.3: Parse "fetch" command with optional --refresh flag
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

	err := run()

	// Get token for error messages that need it
	token := ""
	if len(os.args) >= 2 {
		token = os.args[1]
	}

	exit_code := 0

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

	// Oracle errors (Phase 4.2)
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

	// Database errors (Phase 5.1)
	case .DatabaseError:
		fmt.eprintln("Error: Database operation failed")
		fmt.eprintln("Could not read or write to the token database.")
		fmt.eprintln("Check file permissions at ~/.config/hound/tokens.db")
		exit_code = 74  // I/O error

	case .DatabaseCorrupted:
		fmt.eprintln("Error: Database integrity check failed")
		fmt.eprintln("The database file at ~/.config/hound/tokens.db is corrupted.")
		fmt.eprintln("Delete the file to recreate it from tokens.json backup.")
		exit_code = 74  // I/O error

	case .MigrationFailed:
		fmt.eprintln("Error: JSON to database migration failed")
		fmt.eprintln("Could not migrate tokens.json to database.")
		fmt.eprintln("Check file permissions and ensure tokens.json is valid.")
		exit_code = 65  // Data format error

	// Pool Discovery errors (Phase 5.2)
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

	// Cleanup logger before exit
	log.destroy_console_logger(context.logger)
	os.exit(exit_code)
}
