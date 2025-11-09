#+feature global-context
package main

import "core:fmt"
import "core:os"

run :: proc() -> ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		return .MissingArgument
	}

	symbol := os.args[1]

	// Load token configuration
	config, config_err := load_token_config()
	if config_err != .None {
		return config_err
	}

	// Handle "list" command
	if symbol == "list" {
		list_tokens(config)
		return .None
	}

	// Find token by symbol (case-insensitive)
	token, found := find_token_by_symbol(config, symbol)
	if !found {
		return .TokenNotConfigured
	}

	// Fetch price using the contract address
	price_data, err := fetch_price(token.contract_address)
	if err != .None {
		return err
	}

	// Display result with the actual token symbol
	format_price_output(token.symbol, price_data)

	return .None
}

main :: proc() {
	err := run()

	// Get token for error messages that need it
	token := ""
	if len(os.args) >= 2 {
		token = os.args[1]
	}

	exit_code := 0

	// Map errors to exit codes and messages
	switch err {
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
	}

	os.exit(exit_code)
}
