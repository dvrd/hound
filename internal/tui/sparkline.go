package tui

import "github.com/charmbracelet/lipgloss"

// sparklineBlocks are the 8 Unicode block characters used to render sparklines,
// from lowest to highest fill.
var sparklineBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline color styles — hoisted to avoid allocation per render.
var (
	sparklinePositive = lipgloss.NewStyle().Foreground(ColorPositive)
	sparklineNegative = lipgloss.NewStyle().Foreground(ColorNegative)
	sparklineMuted    = lipgloss.NewStyle().Foreground(ColorMuted)
)

// RenderSparkline renders a slice of price points as a Unicode block sparkline.
//
// The width parameter controls how many characters wide the output is. If len(prices)
// differs from width, the input is resampled to fit. If width <= 0, it defaults to
// len(prices).
//
// Color: green (ColorPositive) if the last value is higher than the first,
// red (ColorNegative) if lower, muted (ColorMuted) if flat or only one point.
//
// Returns an empty string if prices is nil or empty.
func RenderSparkline(prices []float64, width int) string {
	if len(prices) == 0 {
		return ""
	}

	if width <= 0 {
		width = len(prices)
	}

	// Resample prices to the target width.
	sampled := resample(prices, width)

	// Find min and max for normalization.
	min, max := sampled[0], sampled[0]
	for _, p := range sampled[1:] {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}

	// Build the block string.
	blocks := make([]rune, len(sampled))
	for i, p := range sampled {
		var idx int
		if max > min {
			normalized := (p - min) / (max - min)
			idx = int(normalized * float64(len(sparklineBlocks)-1))
			// Clamp to valid range.
			if idx < 0 {
				idx = 0
			}
			if idx >= len(sparklineBlocks) {
				idx = len(sparklineBlocks) - 1
			}
		} else {
			// Flat — use the middle block.
			idx = len(sparklineBlocks) / 2
		}
		blocks[i] = sparklineBlocks[idx]
	}

	// Choose color based on trend.
	first, last := sampled[0], sampled[len(sampled)-1]
	switch {
	case last > first:
		return sparklinePositive.Render(string(blocks))
	case last < first:
		return sparklineNegative.Render(string(blocks))
	default:
		return sparklineMuted.Render(string(blocks))
	}
}

// resample linearly interpolates or downsamples prices to produce exactly n points.
func resample(prices []float64, n int) []float64 {
	if len(prices) == n {
		return prices
	}
	if len(prices) == 1 {
		out := make([]float64, n)
		for i := range out {
			out[i] = prices[0]
		}
		return out
	}

	// n=1: division by zero in the loop — just take the middle value.
	if n == 1 {
		return []float64{prices[len(prices)/2]}
	}

	out := make([]float64, n)
	for i := 0; i < n; i++ {
		// Map output index i to a fractional position in the input.
		t := float64(i) * float64(len(prices)-1) / float64(n-1)
		lo := int(t)
		hi := lo + 1
		if hi >= len(prices) {
			hi = len(prices) - 1
		}
		frac := t - float64(lo)
		out[i] = prices[lo]*(1-frac) + prices[hi]*frac
	}
	return out
}

// PricePathFromChanges reconstructs a 5-point synthetic price path from a current
// price and four percentage change values (M5, H1, H6, H24).
//
// The returned slice is in chronological order: [P_24h, P_6h, P_1h, P_5m, P_now].
// If currentPrice is 0, returns nil.
func PricePathFromChanges(currentPrice, m5, h1, h6, h24 float64) []float64 {
	if currentPrice == 0 {
		return nil
	}
	// Work backwards: P_t = P_now / (1 + change/100)
	// Guard against -100% change to avoid division by zero.
	safe := func(change float64) float64 {
		divisor := 1 + change/100
		if divisor == 0 {
			return currentPrice
		}
		return currentPrice / divisor
	}
	return []float64{
		safe(h24), // oldest
		safe(h6),
		safe(h1),
		safe(m5),
		currentPrice, // newest
	}
}
