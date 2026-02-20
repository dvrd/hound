package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorText      = lipgloss.Color("#F9FAFB") // White
	ColorSubtext   = lipgloss.Color("#9CA3AF") // Light gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
	ColorHighlight = lipgloss.Color("#8B5CF6") // Light purple
	ColorPositive  = lipgloss.Color("#10B981") // Green (for gains)
	ColorNegative  = lipgloss.Color("#EF4444") // Red (for losses)

	// Text styles
	StyleTitle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	StyleSubtitle = lipgloss.NewStyle().Foreground(ColorSubtext)
	StyleBold     = lipgloss.NewStyle().Bold(true).Foreground(ColorText)
	StyleMuted    = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleSuccess  = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning  = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleError    = lipgloss.NewStyle().Foreground(ColorError)

	// Container styles
	StyleApp = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(ColorSubtext).
			Padding(0, 1)

	// Table styles
	StyleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				Padding(0, 1)

	StyleTableRow = lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1)

	StyleTableRowSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHighlight).
				Padding(0, 1)

	// Badge styles
	StylePrimaryBadge = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	StyleTypeBadge = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// FormatChange returns a styled 24h change string with color.
func FormatChange(change float64) string {
	switch {
	case change > 0:
		return lipgloss.NewStyle().Foreground(ColorPositive).Render(fmt.Sprintf("+%.2f%%", change))
	case change < 0:
		return lipgloss.NewStyle().Foreground(ColorNegative).Render(fmt.Sprintf("%.2f%%", change))
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("0.00%")
	}
}
