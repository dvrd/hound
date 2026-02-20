package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/tui"
)

// KeyBinding represents a key and its description for the help overlay.
type KeyBinding struct {
	Key         string
	Description string
}

// HelpModel is a toggleable help overlay showing key bindings.
type HelpModel struct {
	bindings []KeyBinding
	visible  bool
	title    string
}

// NewHelp creates a new help overlay with the given title and bindings.
func NewHelp(title string, bindings []KeyBinding) HelpModel {
	return HelpModel{
		title:    title,
		bindings: bindings,
	}
}

// Update handles the '?' key to toggle the help overlay.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "?" {
			m.visible = !m.visible
		}
	}
	return m, nil
}

// View renders the help overlay as a bordered box with key bindings.
func (m HelpModel) View() string {
	if !m.visible || len(m.bindings) == 0 {
		return ""
	}

	// Find the longest key for alignment
	maxKeyLen := 0
	for _, b := range m.bindings {
		if len(b.Key) > maxKeyLen {
			maxKeyLen = len(b.Key)
		}
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(tui.ColorPrimary).
		Bold(true).
		Width(maxKeyLen + 2)

	descStyle := lipgloss.NewStyle().
		Foreground(tui.ColorText)

	var rows []string
	for _, b := range m.bindings {
		row := keyStyle.Render(b.Key) + descStyle.Render(b.Description)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tui.ColorPrimary).
		MarginBottom(1)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorBorder).
		Padding(1, 2)

	return box.Render(titleStyle.Render(m.title) + "\n" + content)
}

// Toggle flips the visibility of the help overlay.
func (m *HelpModel) Toggle() {
	m.visible = !m.visible
}

// SetBindings replaces the current key bindings.
func (m *HelpModel) SetBindings(bindings []KeyBinding) {
	m.bindings = bindings
}

// Visible returns whether the help overlay is currently shown.
func (m HelpModel) Visible() bool {
	return m.visible
}
