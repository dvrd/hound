package tui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Ensure strings is used (for stripANSI helper below)
var _ = strings.Builder{}

func TestRenderSparkline_Empty(t *testing.T) {
	got := RenderSparkline(nil, 10)
	if got != "" {
		t.Errorf("expected empty string for nil input, got %q", got)
	}
	got = RenderSparkline([]float64{}, 10)
	if got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestRenderSparkline_Width(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}
	for _, width := range []int{1, 5, 10, 24} {
		got := RenderSparkline(prices, width)
		// Strip ANSI escape codes by counting runes after stripping.
		// Lipgloss adds ANSI; count only printable rune width.
		stripped := stripANSI(got)
		runeCount := utf8.RuneCountInString(stripped)
		if runeCount != width {
			t.Errorf("width=%d: expected %d runes, got %d (stripped: %q)", width, width, runeCount, stripped)
		}
	}
}

func TestRenderSparkline_Ascending(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5}
	got := RenderSparkline(prices, 5)
	stripped := stripANSI(got)
	runes := []rune(stripped)
	// Each successive block should be >= the previous.
	for i := 1; i < len(runes); i++ {
		if runes[i] < runes[i-1] {
			t.Errorf("ascending prices: expected non-decreasing blocks, got %q", stripped)
			break
		}
	}
	// Last block should be the highest (█).
	if runes[len(runes)-1] != '█' {
		t.Errorf("ascending prices: expected last block to be █, got %q", string(runes[len(runes)-1]))
	}
}

func TestRenderSparkline_Descending(t *testing.T) {
	prices := []float64{5, 4, 3, 2, 1}
	got := RenderSparkline(prices, 5)
	stripped := stripANSI(got)
	runes := []rune(stripped)
	// Each successive block should be <= the previous.
	for i := 1; i < len(runes); i++ {
		if runes[i] > runes[i-1] {
			t.Errorf("descending prices: expected non-increasing blocks, got %q", stripped)
			break
		}
	}
	// Last block should be the lowest (▁).
	if runes[len(runes)-1] != '▁' {
		t.Errorf("descending prices: expected last block to be ▁, got %q", string(runes[len(runes)-1]))
	}
}

func TestRenderSparkline_Flat(t *testing.T) {
	prices := []float64{3, 3, 3, 3, 3}
	got := RenderSparkline(prices, 5)
	stripped := stripANSI(got)
	runes := []rune(stripped)
	// All blocks should be identical.
	for i := 1; i < len(runes); i++ {
		if runes[i] != runes[0] {
			t.Errorf("flat prices: expected all identical blocks, got %q", stripped)
			break
		}
	}
	// All blocks should be the middle block (▅, index 4 = len(sparklineBlocks)/2).
	if runes[0] != '▅' {
		t.Errorf("flat prices: expected middle block ▅, got %q", string(runes[0]))
	}
}

func TestRenderSparkline_SinglePoint(t *testing.T) {
	got := RenderSparkline([]float64{42.0}, 1)
	stripped := stripANSI(got)
	if utf8.RuneCountInString(stripped) != 1 {
		t.Errorf("single point: expected 1 rune, got %q", stripped)
	}
}

func TestRenderSparkline_DefaultWidth(t *testing.T) {
	prices := []float64{1, 2, 3}
	got := RenderSparkline(prices, 0)
	stripped := stripANSI(got)
	if utf8.RuneCountInString(stripped) != len(prices) {
		t.Errorf("default width: expected %d runes, got %q", len(prices), stripped)
	}
}

func TestPricePathFromChanges_ZeroPrice(t *testing.T) {
	got := PricePathFromChanges(0, 1, 2, 3, 4)
	if got != nil {
		t.Errorf("expected nil for zero price, got %v", got)
	}
}

func TestPricePathFromChanges_Length(t *testing.T) {
	got := PricePathFromChanges(100, 2, 5, -1, 8)
	if len(got) != 5 {
		t.Errorf("expected 5 points, got %d", len(got))
	}
}

func TestPricePathFromChanges_LastIsCurrentPrice(t *testing.T) {
	currentPrice := 1.23456
	got := PricePathFromChanges(currentPrice, 2, 5, -1, 8)
	if got[4] != currentPrice {
		t.Errorf("last point should be currentPrice %f, got %f", currentPrice, got[4])
	}
}

func TestPricePathFromChanges_Reconstruction(t *testing.T) {
	// If H24 = +100%, then P_24h = P_now / 2.
	got := PricePathFromChanges(200, 0, 0, 0, 100)
	want := 100.0 // 200 / (1 + 1.0)
	if got[0] != want {
		t.Errorf("P_24h: expected %f, got %f", want, got[0])
	}
}

func TestPricePathFromChanges_NegativeHundredPercent(t *testing.T) {
	// -100% change would cause division by zero; should not panic.
	got := PricePathFromChanges(100, 0, 0, 0, -100)
	if got == nil {
		t.Error("expected non-nil result even for -100% change")
	}
}

// stripANSI removes ANSI escape sequences from a string, leaving only printable content.
// This is a minimal implementation sufficient for color code stripping in tests.
func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case inEscape:
			// still in escape sequence
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
