// Price output formatting
// Displays token prices in user-friendly format
package output

import "core:fmt"
import models "../../src/lib/models"

// ============================================================================
// Price Formatting
// ============================================================================

// format_price displays price data in user-friendly format
//
// Outputs to stdout (for piping/scripting):
//   SOL: $100.456789 (+2.3%)
//   USDC: $1.000123 (-0.1%)
//
// Pattern: symbol: $price (±change%)
format_price :: proc(symbol: string, data: models.PriceData) {
	sign := data.change_24h >= 0 ? "+" : ""
	fmt.printfln("%s: $%.6f (%s%.1f%%)",
		symbol, data.price_usd, sign, data.change_24h)
}
