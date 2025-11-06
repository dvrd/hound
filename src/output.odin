package main

import "core:fmt"

format_price_output :: proc(symbol: string, data: PriceData) {
	sign := data.change_24h >= 0 ? "+" : ""
	fmt.printfln("%s: $%.6f (%s%.1f%%)", 
		symbol, data.price_usd, sign, data.change_24h)
}
