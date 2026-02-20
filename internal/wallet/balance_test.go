package wallet

import (
	"testing"
)

func TestFormatBalance(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   string
	}{
		{"large number", 1234.5, "1234.50"},
		{"medium number", 12.3456, "12.3456"},
		{"small number", 0.05, "0.050000"},
		{"very small number", 0.001234, "0.00123400"},
		{"zero", 0, "0.00"},
		{"exactly 1000", 1000.0, "1000.00"},
		{"just under 1", 0.9999, "0.999900"},
		{"just under 0.01", 0.009, "0.00900000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBalance(tt.amount)
			if got != tt.want {
				t.Errorf("FormatBalance(%v) = %q, want %q", tt.amount, got, tt.want)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		want  string
	}{
		{"high price", 145.32, "$145.32"},
		{"medium price", 0.45, "$0.4500"},
		{"low price", 0.00028, "$0.000280"},
		{"zero", 0, "$0.00"},
		{"exactly 1", 1.0, "$1.00"},
		{"just under 0.01", 0.005, "$0.005000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPrice(tt.price)
			if got != tt.want {
				t.Errorf("FormatPrice(%v) = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

func TestFormatLargeNumber(t *testing.T) {
	tests := []struct {
		name string
		n    float64
		want string
	}{
		{"billions", 1.23e9, "$1.23B"},
		{"millions", 12.34e6, "$12.34M"},
		{"thousands", 45670, "$45.67K"},
		{"small", 123.45, "$123.45"},
		{"zero", 0, "$0.00"},
		{"exactly 1B", 1e9, "$1.00B"},
		{"exactly 1M", 1e6, "$1.00M"},
		{"exactly 1K", 1000, "$1.00K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLargeNumber(tt.n)
			if got != tt.want {
				t.Errorf("FormatLargeNumber(%v) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatChange24h(t *testing.T) {
	tests := []struct {
		name   string
		change float64
		want   string
	}{
		{"positive", 2.3, "+2.30%"},
		{"negative", -1.5, "-1.50%"},
		{"zero", 0, "0.00%"},
		{"large positive", 100.5, "+100.50%"},
		{"small negative", -0.01, "-0.01%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatChange24h(tt.change)
			if got != tt.want {
				t.Errorf("FormatChange24h(%v) = %q, want %q", tt.change, got, tt.want)
			}
		})
	}
}
