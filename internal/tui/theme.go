package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan — used for numeric data values
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
	StyleValue    = lipgloss.NewStyle().Foreground(ColorSecondary) // numeric data values

	// Container styles — StyleApp is the default; app.go builds a dynamic version
	// with the view name embedded in the top border title.
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

	// StyleTableRowSelected: accent bar ┃ is prepended by RenderRow — no background needed.
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

	// Internal accent / separator / footer styles.
	styleAccentBar = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	styleSeparator = lipgloss.NewStyle().Foreground(ColorBorder)
	styleFooterKey = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	styleFooterSep = lipgloss.NewStyle().Foreground(ColorMuted)
)

// ─── Row rendering helpers ────────────────────────────────────────────────────

// RenderRow renders a table row with an accent bar ┃ when selected, or a plain
// 2-space indent when not. This replaces the old "> " cursor convention and
// works even when TrueColor is unavailable, since the bar is a visible glyph.
//
//	selected=true  → "┃ " + StyleTableRowSelected(content)
//	selected=false → "  " + StyleTableRow(content)
func RenderRow(content string, selected bool) string {
	if selected {
		bar := styleAccentBar.Render("┃")
		return bar + " " + StyleTableRowSelected.Render(content)
	}
	return "  " + StyleTableRow.Render(content)
}

// TableSeparator returns a full-width ─── line styled in the border color,
// to be placed between the table header and the first data row.
func TableSeparator(width int) string {
	if width <= 0 {
		width = 40
	}
	return styleSeparator.Render(strings.Repeat("─", width))
}

// ─── Footer helpers ───────────────────────────────────────────────────────────

// FooterBinding is a single key+action pair for the footer bar.
type FooterBinding struct {
	Key    string
	Action string
}

// FooterGroup is a slice of bindings that belong together visually.
type FooterGroup []FooterBinding

// RenderFooter renders groups of key bindings separated by a styled │.
// Format: "key action  key action  │  key action"
// Keys are rendered in highlight color, actions in plain subtext.
func RenderFooter(groups ...FooterGroup) string {
	var parts []string
	for _, g := range groups {
		var bindings []string
		for _, b := range g {
			bindings = append(bindings, styleFooterKey.Render(b.Key)+" "+b.Action)
		}
		parts = append(parts, strings.Join(bindings, "  "))
	}
	sep := "  " + styleFooterSep.Render("│") + "  "
	return strings.Join(parts, sep)
}

// ─── Color helpers ────────────────────────────────────────────────────────────

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

// FormatChangePlain returns the plain (unstyled) change string for use in
// fmt.Sprintf width specifiers, where ANSI bytes would inflate apparent length.
func FormatChangePlain(change float64) string {
	switch {
	case change > 0:
		return fmt.Sprintf("+%.2f%%", change)
	case change < 0:
		return fmt.Sprintf("%.2f%%", change)
	default:
		return "0.00%"
	}
}

// ColorizeChange wraps an already-padded plain string with the appropriate
// color for the given change value.
func ColorizeChange(change float64, plain string) string {
	switch {
	case change > 0:
		return lipgloss.NewStyle().Foreground(ColorPositive).Render(plain)
	case change < 0:
		return lipgloss.NewStyle().Foreground(ColorNegative).Render(plain)
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render(plain)
	}
}
