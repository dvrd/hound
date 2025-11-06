#+feature global-context
package main

import "core:fmt"
import "core:os"

run :: proc() -> ErrorType {
	// Check arguments
	if len(os.args) < 2 {
		fmt.eprintln("Usage: hound aura")
		return .InvalidResponse
	}

	token := os.args[1]

	// Only support "aura" for MVP
	if token != "aura" {
		fmt.eprintfln("Error: Token '%s' not supported in MVP", token)
		fmt.eprintln("Only 'aura' is supported")
		return .TokenNotFound
	}

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

	exit_code := 0

	// Map errors to exit codes and messages
	switch err {
	case .None:
		// Success
	case .TokenNotFound:
		fmt.eprintln("Error: Token not found on DexScreener")
		exit_code = 1
	case .NetworkError:
		fmt.eprintln("Error: Network request failed")
		exit_code = 1
	case .APITimeout:
		fmt.eprintln("Error: API request timed out")
		exit_code = 1
	case .InvalidResponse:
		fmt.eprintln("Error: Invalid response from API")
		exit_code = 1
	}

	os.exit(exit_code)
}
