package tui

import "testing"

func BenchmarkInjectBorderTitle(b *testing.B) {
	// Simulate a typical rendered box (120-char wide border line + content)
	rendered := "╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮\n│ content here │\n╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯"
	title := " Portfolio "
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = injectBorderTitle(rendered, title)
	}
}
