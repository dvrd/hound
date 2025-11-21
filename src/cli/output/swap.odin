// Swap quote output formatting
// Displays quotes, routes, and confirmations
package output

import "core:fmt"
import "core:os"
import "core:strings"
import models "../../lib/models"

// format_swap_quote displays a formatted swap quote
//
// Parameters:
//   - quote: SwapQuote with routing info
//   - input_symbol: Human-readable input token (e.g., "SOL")
//   - output_symbol: Human-readable output token (e.g., "USDC")
format_swap_quote :: proc(
	quote: models.SwapQuote,
	input_symbol: string,
	output_symbol: string,
) {
	fmt.println("")
	fmt.println("Swap Quote")
	fmt.println("═══════════════════════════════════════════")

	// Input/Output
	fmt.printfln("From:       %.6f %s", quote.input_amount, input_symbol)
	fmt.printfln("To:         ~%.6f %s (estimate)", quote.output_amount, output_symbol)

	// Rate
	fmt.printfln("Rate:       1 %s = %.6f %s", input_symbol, quote.rate, output_symbol)

	// Route
	fmt.printfln("Route:      %s", quote.primary_dex)

	// Slippage
	slippage_pct := f64(quote.slippage_bps) / 100.0
	fmt.printfln("Slippage:   %.2f%% (min %.6f %s)",
		slippage_pct, quote.minimum_out, output_symbol)

	// Price Impact (warning if >5%)
	if quote.price_impact_pct > 5.0 {
		fmt.printfln("Price Impact: ⚠ %.2f%% (HIGH)", quote.price_impact_pct)
	} else {
		fmt.printfln("Price Impact: %.2f%%", quote.price_impact_pct)
	}

	// Network Fee
	fmt.printfln("Network Fee: ~%.6f SOL", quote.network_fee_sol)

	fmt.println("═══════════════════════════════════════════")
	fmt.println("")
}

// format_route_steps displays detailed routing information
format_route_steps :: proc(steps: []models.RouteStep) {
	if len(steps) == 0 {
		return
	}

	fmt.println("Route Details:")
	for step, i in steps {
		fmt.printfln("  %d. %s (%d%%)",
			i + 1,
			step.dex_label,
			step.percent)
	}
	fmt.println("")
}

// prompt_swap_confirmation shows Y/N prompt and returns user choice
//
// Returns: true if user confirms (y/Y), false otherwise
prompt_swap_confirmation :: proc() -> bool {
	fmt.print("Execute this swap? [y/N]: ")

	buffer: [256]byte
	n, err := os.read(os.stdin, buffer[:])
	if err != nil {
		return false
	}

	response := string(buffer[:n])
	response = strings.trim_space(response)
	response_lower := strings.to_lower(response)

	return response_lower == "y" || response_lower == "yes"
}
