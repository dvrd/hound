package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/tui"
)

// ConfirmMsg is sent when the user confirms or cancels.
type ConfirmMsg struct {
	Confirmed bool
}

// ConfirmModel is a yes/no confirmation dialog.
type ConfirmModel struct {
	question  string
	confirmed bool
	answered  bool
	defaultNo bool // If true, default is No [y/N], otherwise Yes [Y/n]
}

// NewConfirm creates a new confirmation dialog.
func NewConfirm(question string, defaultNo bool) ConfirmModel {
	return ConfirmModel{
		question:  question,
		defaultNo: defaultNo,
	}
}

// Init returns nil (no initial command).
func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

// Update handles key presses for the confirmation dialog.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if m.answered {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.answered = true
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: true} }
		case "n", "N":
			m.confirmed = false
			m.answered = true
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: false} }
		case "enter":
			m.confirmed = !m.defaultNo
			m.answered = true
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: m.confirmed} }
		}
	}

	return m, nil
}

// View renders the confirmation prompt.
func (m ConfirmModel) View() string {
	if m.answered {
		answer := "Yes"
		if !m.confirmed {
			answer = "No"
		}
		return lipgloss.NewStyle().Foreground(tui.ColorSubtext).Render(m.question) +
			" " + lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary).Render(answer)
	}

	hint := "[Y/n]"
	if m.defaultNo {
		hint = "[y/N]"
	}

	return lipgloss.NewStyle().Foreground(tui.ColorText).Render(m.question) +
		" " + lipgloss.NewStyle().Foreground(tui.ColorMuted).Render(hint) + ": "
}

// Answered returns whether the user has responded.
func (m ConfirmModel) Answered() bool {
	return m.answered
}

// Confirmed returns whether the user confirmed (true) or cancelled (false).
func (m ConfirmModel) Confirmed() bool {
	return m.confirmed
}
