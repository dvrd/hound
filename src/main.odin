#+feature global-context
package main

import "core:fmt"
import "core:os"

run :: proc() -> ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		return .MissingArgument
	}

	token := os.args[1]

	// For Phase 2, remove "aura only" restriction since we're fixing errors
	// All Solana token addresses should work
	// (Phase 3 will add named token resolution)

	// Fetch price
	price_data, err := fetch_price_for_aura()
	if err != .None {
		return err
	}

	// Display result
	format_price_output("AURA", price_data)

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
		fmt.eprintln("Error: Missing token address")
		fmt.eprintln("")
		fmt.eprintln("Usage: hound <token-address>")
		fmt.eprintln("Example: hound DtR4D9FtVoTX2569gaL837ZgrB6wNjj6tkmnX9Rdk9B2")
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
	}

	os.exit(exit_code)
}
