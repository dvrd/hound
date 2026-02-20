package components_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui/components"
)

func TestConfirmInitialState(t *testing.T) {
	m := components.NewConfirm("Delete wallet?", true)
	if m.Answered() {
		t.Error("new confirm should not be answered")
	}
	if m.Confirmed() {
		t.Error("new confirm should not be confirmed")
	}
}

func TestConfirmYes(t *testing.T) {
	m := components.NewConfirm("Delete wallet?", true)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.Answered() {
		t.Error("should be answered after 'y'")
	}
	if !m.Confirmed() {
		t.Error("should be confirmed after 'y'")
	}
	if cmd == nil {
		t.Error("should return a command after answering")
	}
}

func TestConfirmNo(t *testing.T) {
	m := components.NewConfirm("Delete wallet?", true)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !m.Answered() {
		t.Error("should be answered after 'n'")
	}
	if m.Confirmed() {
		t.Error("should not be confirmed after 'n'")
	}
	if cmd == nil {
		t.Error("should return a command after answering")
	}
}

func TestConfirmEnterDefaultNo(t *testing.T) {
	m := components.NewConfirm("Delete wallet?", true) // defaultNo = true
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Answered() {
		t.Error("should be answered after Enter")
	}
	if m.Confirmed() {
		t.Error("should not be confirmed when defaultNo=true and Enter pressed")
	}
}

func TestConfirmEnterDefaultYes(t *testing.T) {
	m := components.NewConfirm("Continue?", false) // defaultNo = false
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Answered() {
		t.Error("should be answered after Enter")
	}
	if !m.Confirmed() {
		t.Error("should be confirmed when defaultNo=false and Enter pressed")
	}
}

func TestConfirmIgnoresAfterAnswered(t *testing.T) {
	m := components.NewConfirm("Delete?", true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.Confirmed() {
		t.Error("should be confirmed after 'y'")
	}
	// Try pressing 'n' after already answered
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !m.Confirmed() {
		t.Error("should still be confirmed - already answered")
	}
}

func TestConfirmViewDefaultNo(t *testing.T) {
	m := components.NewConfirm("Delete?", true)
	view := m.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestConfirmViewDefaultYes(t *testing.T) {
	m := components.NewConfirm("Continue?", false)
	view := m.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}
