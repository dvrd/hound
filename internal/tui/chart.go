package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/models"
)

// sparklineChars are the 8 Unicode block characters used to render sparklines,
// from lowest to highest fill.
var sparklineChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a slice of PriceCandle values as a Unicode block sparkline
// using the Close price of each candle.
//
// The width parameter controls how many characters wide the output is. If
// len(candles) exceeds width, the last width candles are used. If len(candles)
// is less than width, the output is padded with spaces on the left.
//
// Color: green (#00FF88) if the last Close is higher than the first Close,
// red (#FF2D78) if lower, gray (ColorMuted) if flat or fewer than 2 candles.
//
// Returns an empty string if candles is nil or empty.
func Sparkline(candles []models.PriceCandle, width int) string {
	if len(candles) == 0 {
		return ""
	}
	if width <= 0 {
		width = len(candles)
	}

	// Trim to last `width` candles if we have more than needed.
	if len(candles) > width {
		candles = candles[len(candles)-width:]
	}

	// Find min and max Close prices for normalization.
	minClose := candles[0].Close
	maxClose := candles[0].Close
	for _, c := range candles[1:] {
		if c.Close < minClose {
			minClose = c.Close
		}
		if c.Close > maxClose {
			maxClose = c.Close
		}
	}

	// Build the block string using a strings.Builder.
	var sb strings.Builder
	for _, c := range candles {
		var idx int
		if maxClose > minClose {
			level := (c.Close - minClose) / (maxClose - minClose) * 7
			idx = int(level)
			if idx < 0 {
				idx = 0
			}
			if idx > 7 {
				idx = 7
			}
		} else {
			// All prices equal — flat line at middle block (▄, index 3).
			idx = 3
		}
		sb.WriteRune(sparklineChars[idx])
	}

	// Pad with spaces on the left if we have fewer candles than width.
	result := sb.String()
	runeCount := len([]rune(result))
	if runeCount < width {
		result = strings.Repeat(" ", width-runeCount) + result
	}

	// Choose color based on trend.
	first := candles[0].Close
	last := candles[len(candles)-1].Close

	var color lipgloss.Color
	switch {
	case last > first:
		color = lipgloss.Color("#00FF88")
	case last < first:
		color = lipgloss.Color("#FF2D78")
	default:
		color = ColorMuted
	}

	return lipgloss.NewStyle().Foreground(color).Render(result)
}
