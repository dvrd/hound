package tui_test

import (
	"strings"
	"testing"

	"github.com/dvrd/hound/internal/tui"
)

func TestFormatChangePositive(t *testing.T) {
	result := tui.FormatChange(2.30)
	if !strings.Contains(result, "+2.30%") {
		t.Errorf("FormatChange(2.30) = %q, want to contain +2.30%%", result)
	}
}

func TestFormatChangeNegative(t *testing.T) {
	result := tui.FormatChange(-1.50)
	if !strings.Contains(result, "-1.50%") {
		t.Errorf("FormatChange(-1.50) = %q, want to contain -1.50%%", result)
	}
}

func TestFormatChangeZero(t *testing.T) {
	result := tui.FormatChange(0)
	if !strings.Contains(result, "0.00%") {
		t.Errorf("FormatChange(0) = %q, want to contain 0.00%%", result)
	}
}
