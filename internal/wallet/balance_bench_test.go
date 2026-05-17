package wallet

import "testing"

func BenchmarkFormatBalance(b *testing.B) {
	amounts := []float64{0, 0.00001234, 0.05, 1.5, 1000.50, 1234567.89}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, a := range amounts {
			_ = FormatBalance(a)
		}
	}
}

func BenchmarkFormatPrice(b *testing.B) {
	prices := []float64{0, 0.000123, 0.05, 1.50, 142.50, 50000.00}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range prices {
			_ = FormatPrice(p)
		}
	}
}
