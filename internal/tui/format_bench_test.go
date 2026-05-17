package tui

import (
	"fmt"
	"testing"
)

func BenchmarkPadRight(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PadRight("BONK", 10)
	}
}

func BenchmarkPadLeft(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PadLeft("$142.50", 10)
	}
}

func BenchmarkFmtSprintfPadLeft(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%*s", 10, "$142.50")
	}
}

func BenchmarkFmtSprintfPadRight(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%-*s", 10, "BONK")
	}
}
