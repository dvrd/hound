package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dvrd/hound/internal/models"
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

// --- New tests: Sparkline (chart.go) ---

func TestSparkline_Empty(t *testing.T) {
	got := Sparkline(nil, 10)
	if got != "" {
		t.Errorf("Sparkline(nil, 10) = %q, want empty string", got)
	}
	got = Sparkline([]models.PriceCandle{}, 10)
	if got != "" {
		t.Errorf("Sparkline([], 10) = %q, want empty string", got)
	}
}

func TestSparkline_Width(t *testing.T) {
	candles := []models.PriceCandle{
		{Close: 1}, {Close: 2}, {Close: 3}, {Close: 4}, {Close: 5},
	}
	for _, width := range []int{1, 5, 10, 24} {
		got := Sparkline(candles, width)
		stripped := stripANSI(got)
		runeCount := utf8.RuneCountInString(stripped)
		if runeCount != width {
			t.Errorf("Sparkline width=%d: expected %d runes, got %d (stripped: %q)", width, width, runeCount, stripped)
		}
	}
}

func TestSparkline_Ascending_BlockChars(t *testing.T) {
	// Ascending prices → last block should be the highest (█).
	candles := []models.PriceCandle{
		{Close: 1}, {Close: 2}, {Close: 3}, {Close: 4}, {Close: 5},
	}
	got := Sparkline(candles, 5)
	stripped := stripANSI(got)
	runes := []rune(stripped)
	if len(runes) == 0 {
		t.Fatal("ascending Sparkline should not be empty")
	}
	// Last block should be the highest (█).
	if runes[len(runes)-1] != '█' {
		t.Errorf("ascending Sparkline: expected last block to be █, got %q (stripped: %q)", string(runes[len(runes)-1]), stripped)
	}
}

func TestSparkline_Descending_BlockChars(t *testing.T) {
	// Descending prices → last block should be the lowest (▁).
	candles := []models.PriceCandle{
		{Close: 5}, {Close: 4}, {Close: 3}, {Close: 2}, {Close: 1},
	}
	got := Sparkline(candles, 5)
	stripped := stripANSI(got)
	runes := []rune(stripped)
	if len(runes) == 0 {
		t.Fatal("descending Sparkline should not be empty")
	}
	// Last block should be the lowest (▁).
	if runes[len(runes)-1] != '▁' {
		t.Errorf("descending Sparkline: expected last block to be ▁, got %q (stripped: %q)", string(runes[len(runes)-1]), stripped)
	}
}

func TestSparkline_Flat_NeutralColor(t *testing.T) {
	// Flat prices → last == first → muted/neutral color.
	candles := []models.PriceCandle{
		{Close: 3}, {Close: 3}, {Close: 3}, {Close: 3}, {Close: 3},
	}
	got := Sparkline(candles, 5)
	// Should NOT use green or red.
	if strings.Contains(got, "00FF88") {
		t.Error("flat Sparkline should not use green color")
	}
	if strings.Contains(got, "FF2D78") {
		t.Error("flat Sparkline should not use red color")
	}
	// Should still render something.
	if got == "" {
		t.Error("flat Sparkline should not be empty")
	}
}

func TestSparkline_Flat_MiddleBlock(t *testing.T) {
	// Flat prices → all blocks should be the middle block (▄, index 3).
	candles := []models.PriceCandle{
		{Close: 5}, {Close: 5}, {Close: 5}, {Close: 5}, {Close: 5},
	}
	got := Sparkline(candles, 5)
	stripped := stripANSI(got)
	for _, r := range stripped {
		if r != ' ' && r != '▄' {
			t.Errorf("flat Sparkline should use only ▄ blocks, got %q in %q", string(r), stripped)
			break
		}
	}
}

func TestSparkline_TruncatesToWidth(t *testing.T) {
	// More candles than width → last `width` candles used.
	candles := make([]models.PriceCandle, 20)
	for i := range candles {
		candles[i] = models.PriceCandle{Close: float64(i + 1)}
	}
	got := Sparkline(candles, 5)
	stripped := stripANSI(got)
	runeCount := utf8.RuneCountInString(stripped)
	if runeCount != 5 {
		t.Errorf("Sparkline with 20 candles and width=5 should produce 5 runes, got %d", runeCount)
	}
}

func TestSparkline_PadsWithSpaces(t *testing.T) {
	// Fewer candles than width → padded with spaces on the left.
	candles := []models.PriceCandle{
		{Close: 1}, {Close: 2},
	}
	got := Sparkline(candles, 5)
	stripped := stripANSI(got)
	runeCount := utf8.RuneCountInString(stripped)
	if runeCount != 5 {
		t.Errorf("Sparkline with 2 candles and width=5 should produce 5 runes (padded), got %d (stripped: %q)", runeCount, stripped)
	}
	// First 3 chars should be spaces.
	runes := []rune(stripped)
	for i := 0; i < 3; i++ {
		if runes[i] != ' ' {
			t.Errorf("Sparkline padding: expected space at index %d, got %q", i, string(runes[i]))
		}
	}
}

func TestSparkline_SingleCandle(t *testing.T) {
	candles := []models.PriceCandle{{Close: 42.0}}
	got := Sparkline(candles, 1)
	stripped := stripANSI(got)
	if utf8.RuneCountInString(stripped) != 1 {
		t.Errorf("single candle Sparkline should produce 1 rune, got %q", stripped)
	}
}

func TestSparkline_DefaultWidth(t *testing.T) {
	candles := []models.PriceCandle{
		{Close: 1}, {Close: 2}, {Close: 3},
	}
	got := Sparkline(candles, 0)
	stripped := stripANSI(got)
	// width=0 → uses len(candles) = 3.
	if utf8.RuneCountInString(stripped) != 3 {
		t.Errorf("Sparkline with width=0 should default to len(candles)=3, got %q", stripped)
	}
}
