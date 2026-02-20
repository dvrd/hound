package components_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui/components"
)

func TestHelpInitiallyHidden(t *testing.T) {
	h := components.NewHelp("Help", []components.KeyBinding{
		{Key: "q", Description: "Quit"},
	})
	if h.Visible() {
		t.Error("help should be hidden initially")
	}
	if h.View() != "" {
		t.Error("hidden help should render empty string")
	}
}

func TestHelpToggle(t *testing.T) {
	h := components.NewHelp("Help", []components.KeyBinding{
		{Key: "q", Description: "Quit"},
	})
	h.Toggle()
	if !h.Visible() {
		t.Error("help should be visible after Toggle")
	}
	h.Toggle()
	if h.Visible() {
		t.Error("help should be hidden after second Toggle")
	}
}

func TestHelpQuestionMarkToggle(t *testing.T) {
	h := components.NewHelp("Help", []components.KeyBinding{
		{Key: "q", Description: "Quit"},
	})
	h, _ = h.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !h.Visible() {
		t.Error("help should be visible after '?' key")
	}
	h, _ = h.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if h.Visible() {
		t.Error("help should be hidden after second '?' key")
	}
}

func TestHelpViewRendersBindings(t *testing.T) {
	h := components.NewHelp("Keyboard Shortcuts", []components.KeyBinding{
		{Key: "q", Description: "Quit"},
		{Key: "?", Description: "Toggle help"},
	})
	h.Toggle()
	view := h.View()
	if view == "" {
		t.Error("visible help should render non-empty string")
	}
}

func TestHelpSetBindings(t *testing.T) {
	h := components.NewHelp("Help", []components.KeyBinding{
		{Key: "q", Description: "Quit"},
	})
	h.SetBindings([]components.KeyBinding{
		{Key: "a", Description: "Action A"},
		{Key: "b", Description: "Action B"},
	})
	h.Toggle()
	view := h.View()
	if view == "" {
		t.Error("help with new bindings should render non-empty string")
	}
}

func TestHelpEmptyBindings(t *testing.T) {
	h := components.NewHelp("Help", nil)
	h.Toggle()
	if h.View() != "" {
		t.Error("help with no bindings should render empty string even when visible")
	}
}
