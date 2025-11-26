// Error display and exit code mapping
// Comprehensive user-friendly error messages with guidance
package output

import "core:fmt"
import models "../../lib/models"
import version "../../version"

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
		fmt.eprintfln("%s", version.get_version_info())
		fmt.eprintln("")
		fmt.eprintln("Usage: hound <command> [subcommand] [arguments]")
		fmt.eprintln("")
		fmt.eprintln("COMMANDS:")
		fmt.eprintln("  tokens <subcommand>      Manage and query token information")
		fmt.eprintln("  wallet <subcommand>      Manage wallets and perform swaps")
		fmt.eprintln("  history [--limit N]      Show price history")
		fmt.eprintln("  version                  Show version information")
		fmt.eprintln("")
		fmt.eprintln("TOKEN SUBCOMMANDS:")
		fmt.eprintln("  list                     List all configured tokens")
		fmt.eprintln("  fetch <symbol|address>   Fetch detailed token information")
		fmt.eprintln("  add <symbol> <name> <address>  Add a new token")
		fmt.eprintln("")
		fmt.eprintln("WALLET SUBCOMMANDS:")
		fmt.eprintln("  status                   Show wallet balances")
		fmt.eprintln("  list                     List configured wallets")
		fmt.eprintln("  swap <from> <to> <amt>   Swap tokens")
		fmt.eprintln("  import                   Import wallet from seed phrase")
		fmt.eprintln("")
		fmt.eprintln("EXAMPLES:")
		fmt.eprintln("  hound tokens list")
		fmt.eprintln("  hound tokens fetch aura")
		fmt.eprintln("  hound tokens fetch EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
		fmt.eprintln("  hound tokens add AURA \"AURA Memecoin\" DtR4D9FtVoTX2569gaL...")
		fmt.eprintln("  hound wallet status")
		fmt.eprintln("  hound wallet swap sol usdc 1.0")
		fmt.eprintln("")
		fmt.eprintln("For detailed help on a command:")
		fmt.eprintln("  hound tokens --help")
		fmt.eprintln("  hound wallet --help")

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
		fmt.eprintfln("Error: Token '%s' not found in database", token)
		fmt.eprintln("Run 'hound tokens list' to see available tokens.")
		fmt.eprintln("Add new tokens with: hound tokens add <symbol> <name> <address>")

	case .ConfigNotFound:
		fmt.eprintln("Error: Database not found")
		fmt.eprintln("Expected location: ~/.config/hound/hound.db")
		fmt.eprintln("")
		fmt.eprintln("Add your first token to create the database:")
		fmt.eprintln("  hound tokens add <symbol> <name> <contract_address>")
		fmt.eprintln("")
		fmt.eprintln("Example:")
		fmt.eprintln("  hound tokens add aura \"AURA Memecoin\" DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")

	case .ConfigParseError:
		fmt.eprintln("Error: Failed to read database")
		fmt.eprintln("The database at ~/.config/hound/hound.db may be corrupted.")
		fmt.eprintln("Try deleting the file and re-adding your tokens with 'hound tokens add'.")

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
		fmt.eprintln("Delete the file and re-add your tokens with 'hound tokens add'.")

	case .MigrationFailed:
		fmt.eprintln("Error: Database initialization failed")
		fmt.eprintln("Could not create or update database schema.")
		fmt.eprintln("Check file permissions at ~/.config/hound/hound.db")

	// Pool Discovery errors
	case .PoolSearchFailed:
		fmt.eprintln("Error: Pool search failed")
		fmt.eprintln("Could not retrieve pool data from DexScreener API.")
		fmt.eprintln("This may be a temporary API issue. Try again in a few moments.")

	case .NoPoolsFound:
		fmt.eprintfln("Error: No liquidity pools found for token '%s'", token)
		fmt.eprintln("This token may not have active trading pools yet.")
		fmt.eprintln("Pools must have at least $1,000 liquidity and max 1% fees.")

	// Keystore errors (Phase 1: Secure Keystore)
	case .WeakPassword:
		fmt.eprintln("Error: Password does not meet strength requirements")
		fmt.eprintln("Your password must:")
		fmt.eprintln("  - Be at least 12 characters long")
		fmt.eprintln("  - Contain uppercase and lowercase letters")
		fmt.eprintln("  - Contain at least one digit")
		fmt.eprintln("  - Contain at least one special character")

	case .InvalidSeedPhrase:
		fmt.eprintln("Error: Invalid seed phrase")
		fmt.eprintln("Seed phrase must be exactly 12 or 24 words.")
		fmt.eprintln("Ensure you've copied the phrase correctly with proper spacing.")

	case .CryptoOperationFailed:
		fmt.eprintln("Error: Cryptographic operation failed")
		fmt.eprintln("This may be due to:")
		fmt.eprintln("  - Incorrect password")
		fmt.eprintln("  - Corrupted encrypted data")
		fmt.eprintln("  - Invalid seed phrase")
		fmt.eprintln("Please verify your credentials and try again.")

	case .KeypairNotFound:
		fmt.eprintfln("Error: Wallet keypair not found")
		fmt.eprintln("No encrypted wallet found in database.")
		fmt.eprintln("Import a wallet first with: hound wallet import")

	case .WalletAlreadyExists:
		fmt.eprintln("Error: Wallet already exists")
		fmt.eprintln("This seed phrase has already been imported.")
		fmt.eprintln("Each wallet can only be imported once.")

	// Swap errors
	case .QuoteExpired:
		fmt.eprintln("Error: Quote expired")
		fmt.eprintln("The swap quote is older than 90 seconds and can no longer be executed.")
		fmt.eprintln("Please fetch a new quote and try again.")

	case .HighPriceImpact:
		fmt.eprintln("Error: Price impact too high")
		fmt.eprintln("The swap would significantly move the market price (>5% impact).")
		fmt.eprintln("Consider splitting into smaller trades or waiting for better liquidity.")

	case .InsufficientBalance:
		fmt.eprintln("Error: Insufficient balance")
		fmt.eprintln("Your wallet doesn't have enough tokens to complete this swap.")
		fmt.eprintln("Please deposit more tokens and try again.")

	case .SlippageExceeded:
		fmt.eprintln("Error: Slippage exceeded")
		fmt.eprintln("The price moved beyond your slippage tolerance during execution.")
		fmt.eprintln("Try increasing slippage with --slippage <bps> or wait for price to stabilize.")

	case .InvalidTransaction:
		fmt.eprintln("Error: Transaction validation failed")
		fmt.eprintln("The transaction could not be executed. Common causes:")
		fmt.eprintln("  - Quote expired (>90 seconds old)")
		fmt.eprintln("  - Transaction signature invalid")
		fmt.eprintln("  - Insufficient SOL for network fees")
		fmt.eprintln("  - Route no longer available")
		fmt.eprintln("")
		fmt.eprintln("Please fetch a new quote and try again.")
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

	// Keystore errors
	case .WeakPassword:
		return 1  // General error (user input)

	case .InvalidSeedPhrase:
		return 1  // General error (user input)

	case .CryptoOperationFailed:
		return 70  // Internal software error

	case .KeypairNotFound:
		return 1  // General error

	case .WalletAlreadyExists:
		return 1  // General error

	// Swap errors
	case .QuoteExpired:
		return 1  // General error

	case .HighPriceImpact:
		return 1  // General error

	case .InsufficientBalance:
		return 1  // General error

	case .SlippageExceeded:
		return 1  // General error

	case .InvalidTransaction:
		return 1  // General error

	case:
		return 1  // Default to general error
	}
}
