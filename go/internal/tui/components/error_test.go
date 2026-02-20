package components_test

import (
	"errors"
	"testing"

	"github.com/dvrd/hound/internal/tui/components"
)

func TestErrorBarInitiallyHidden(t *testing.T) {
	bar := components.NewErrorBar()
	if bar.Visible() {
		t.Error("new error bar should not be visible")
	}
	if bar.View() != "" {
		t.Error("hidden error bar should render empty string")
	}
}

func TestErrorBarShow(t *testing.T) {
	bar := components.NewErrorBar()
	bar.Show(errors.New("something went wrong"))
	if !bar.Visible() {
		t.Error("error bar should be visible after Show")
	}
	view := bar.View()
	if view == "" {
		t.Error("visible error bar should render non-empty string")
	}
}

func TestErrorBarShowMessage(t *testing.T) {
	bar := components.NewErrorBar()
	bar.ShowMessage("custom message")
	if !bar.Visible() {
		t.Error("error bar should be visible after ShowMessage")
	}
}

func TestErrorBarDismiss(t *testing.T) {
	bar := components.NewErrorBar()
	bar.Show(errors.New("test error"))
	if !bar.Visible() {
		t.Error("error bar should be visible after Show")
	}
	bar.Dismiss()
	if bar.Visible() {
		t.Error("error bar should not be visible after Dismiss")
	}
}

func TestErrorBarAutoDismiss(t *testing.T) {
	bar := components.NewErrorBar()
	bar.Show(errors.New("test error"))
	if !bar.Visible() {
		t.Error("error bar should be visible after Show")
	}

	// Simulate the ErrorDismissMsg
	bar, _ = bar.Update(components.ErrorDismissMsg{})
	if bar.Visible() {
		t.Error("error bar should not be visible after ErrorDismissMsg")
	}
}
