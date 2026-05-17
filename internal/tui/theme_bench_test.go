package tui

import "testing"

func BenchmarkRenderRow_Selected(b *testing.B) {
	content := "SOL       Solana         1.2345   $142.50   $175.93   +2.45%"
	for i := 0; i < b.N; i++ {
		_ = RenderRow(content, true)
	}
}

func BenchmarkRenderRow_Normal(b *testing.B) {
	content := "BONK      Bonk           1000000  $0.0000   $12.34    -1.23%"
	for i := 0; i < b.N; i++ {
		_ = RenderRow(content, false)
	}
}

func BenchmarkColorizeChange_Positive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ColorizeChange(2.45, " +2.45%")
	}
}

func BenchmarkFormatChange(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatChange(2.45)
	}
}

func BenchmarkTableSeparator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = TableSeparator(80)
	}
}

func BenchmarkRenderFooter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RenderFooter(
			FooterGroup{
				{Key: "w", Action: "wallets"}, {Key: "s", Action: "send"},
				{Key: "x", Action: "swap"}, {Key: "h", Action: "history"},
			},
			FooterGroup{{Key: "?", Action: "help"}},
		)
	}
}
