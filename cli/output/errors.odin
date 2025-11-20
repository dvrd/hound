// Error display and exit code mapping
// Comprehensive user-friendly error messages with guidance
package output

import "core:fmt"
import models "../../core/models"

// ============================================================================
// Error Display Functions
// ============================================================================

// display_error shows comprehensive error message with user guidance
//
// Used for: All command failures
// Pattern: Context-specific message + actionable guidance
// Output: stderr for error messages
display_error :: proc(err: models.ErrorType, token: string = "") {
	#partial switch err {
	case .None:
		// Success - no message

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

	case .InvalidToken:
		fmt.eprintfln("Error: Invalid token address: %s", token)
		fmt.eprintln("Token address must be a valid Solana contract address.")
		fmt.eprintln("Example: DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")

	case .TokenNotFound:
		fmt.eprintln("Error: Token not found on DexScreener")
		fmt.eprintln("This token may not be listed yet or the address is incorrect.")
		fmt.eprintln("Visit https://dexscreener.com to verify the token exists.")

	case .NetworkTimeout:
		fmt.eprintln("Error: Request timed out")
		fmt.eprintln("Could not connect to DexScreener API within 10 seconds.")
		fmt.eprintln("Check your internet connection and try again.")

	case .ConnectionFailed:
		fmt.eprintln("Error: Cannot connect to DexScreener API")
		fmt.eprintln("The service may be temporarily down.")
		fmt.eprintln("Try again in a few minutes.")

	case .RateLimited:
		fmt.eprintln("Error: Rate limit exceeded")
		fmt.eprintln("DexScreener allows 300 requests per minute.")
		fmt.eprintln("Wait 60 seconds before trying again.")

	case .ServerError:
		fmt.eprintln("Error: DexScreener API error")
		fmt.eprintln("The service is experiencing issues.")
		fmt.eprintln("Try again in a few minutes.")

	case .InvalidResponse:
		fmt.eprintln("Error: Invalid response from DexScreener")
		fmt.eprintln("Received malformed data. This may be temporary.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")

	case .TokenNotConfigured:
		fmt.eprintfln("Error: Token '%s' not found in configuration", token)
		fmt.eprintln("Run 'hound list' to see available tokens.")
		fmt.eprintln("Add new tokens to ~/.config/hound/tokens.json")

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

	case .ConfigParseError:
		fmt.eprintln("Error: Failed to parse configuration file")
		fmt.eprintln("Check that ~/.config/hound/tokens.json is valid JSON.")
		fmt.eprintln("Required format:")
		fmt.eprintln("  - version: string")
		fmt.eprintln("  - tokens: array of token objects")
		fmt.eprintln("  - Each token needs: symbol, name, contract_address, chain")

	case .RPCConnectionFailed:
		fmt.eprintln("Error: Cannot connect to Solana RPC")
		fmt.eprintln("The Solana network may be temporarily unavailable.")
		fmt.eprintln("Try again in a few minutes.")

	case .RPCInvalidResponse:
		fmt.eprintln("Error: Invalid response from Solana RPC")
		fmt.eprintln("Received malformed data from blockchain node.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")

	case .PoolDataInvalid:
		fmt.eprintln("Error: Invalid pool data")
		fmt.eprintln("Pool structure validation failed.")
		fmt.eprintln("The pool address may be incorrect or the pool format changed.")

	case .VaultFetchFailed:
		fmt.eprintln("Error: Failed to fetch vault balances")
		fmt.eprintln("Could not retrieve token reserves from the pool.")
		fmt.eprintln("The RPC node may be experiencing issues.")

	// Oracle errors
	case .OracleConnectionFailed:
		fmt.eprintln("Error: Cannot fetch SOL price")
		fmt.eprintln("Unable to connect to Jupiter or CoinGecko APIs.")
		fmt.eprintln("Check your internet connection and try again.")

	case .OracleParseFailed:
		fmt.eprintln("Error: Invalid SOL price response")
		fmt.eprintln("Received malformed data from price API.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")

	case .OraclePriceInvalid:
		fmt.eprintln("Error: SOL price validation failed")
		fmt.eprintln("Received unreasonable price from API.")
		fmt.eprintln("Try again or report at https://github.com/dvrd/hound/issues")

	// Database errors
	case .DatabaseError:
		fmt.eprintln("Error: Database operation failed")
		fmt.eprintln("Could not read or write to the token database.")
		fmt.eprintln("Check file permissions at ~/.config/hound/hound.db")

	case .DatabaseCorrupted:
		fmt.eprintln("Error: Database integrity check failed")
		fmt.eprintln("The database file at ~/.config/hound/hound.db is corrupted.")
		fmt.eprintln("Delete the file to recreate it from tokens.json backup.")

	case .MigrationFailed:
		fmt.eprintln("Error: JSON to database migration failed")
		fmt.eprintln("Could not migrate tokens.json to database.")
		fmt.eprintln("Check file permissions and ensure tokens.json is valid.")

	// Pool Discovery errors
	case .PoolSearchFailed:
		fmt.eprintln("Error: Pool search failed")
		fmt.eprintln("Could not retrieve pool data from DexScreener API.")
		fmt.eprintln("This may be a temporary API issue. Try again in a few moments.")

	case .NoPoolsFound:
		fmt.eprintfln("Error: No liquidity pools found for token '%s'", token)
		fmt.eprintln("This token may not have active trading pools yet.")
		fmt.eprintln("Pools must have at least $1,000 liquidity and max 1% fees.")
	}
}

// map_error_to_exit_code converts ErrorType to Unix exit code
//
// Exit code conventions:
//   0  - Success
//   1  - General error
//   2  - Usage error (missing arguments)
//   65 - Data format error (migration)
//   69 - Service unavailable (network/API)
//   70 - Internal software error (parse failures)
//   74 - I/O error (database)
//   78 - Configuration error (config file issues)
map_error_to_exit_code :: proc(err: models.ErrorType) -> int {
	#partial switch err {
	case .None:
		return 0

	case .MissingArgument:
		return 2  // Usage error

	case .InvalidToken:
		return 78  // Configuration error

	case .TokenNotFound:
		return 1  // General error

	case .NetworkTimeout:
		return 69  // Service unavailable

	case .ConnectionFailed:
		return 69  // Service unavailable

	case .RateLimited:
		return 69  // Service unavailable

	case .ServerError:
		return 69  // Service unavailable

	case .InvalidResponse:
		return 70  // Internal software error

	case .TokenNotConfigured:
		return 1  // General error

	case .ConfigNotFound:
		return 78  // Configuration error

	case .ConfigParseError:
		return 78  // Configuration error

	case .RPCConnectionFailed:
		return 69  // Service unavailable

	case .RPCInvalidResponse:
		return 70  // Internal software error

	case .PoolDataInvalid:
		return 70  // Internal software error

	case .VaultFetchFailed:
		return 69  // Service unavailable

	case .OracleConnectionFailed:
		return 69  // Service unavailable

	case .OracleParseFailed:
		return 70  // Internal software error

	case .OraclePriceInvalid:
		return 70  // Internal software error

	case .DatabaseError:
		return 74  // I/O error

	case .DatabaseCorrupted:
		return 74  // I/O error

	case .MigrationFailed:
		return 65  // Data format error

	case .PoolSearchFailed:
		return 69  // Service unavailable

	case .NoPoolsFound:
		return 1  // General error

	case:
		return 1  // Default to general error
	}
}
