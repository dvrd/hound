package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/tui"
)

// SpinnerModel is a loading spinner with a message.
type SpinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(tui.ColorPrimary)
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

// Init starts the spinner animation.
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles spinner tick messages.
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if m.done {
		return m, nil
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner with its message.
func (m SpinnerModel) View() string {
	if m.done {
		return ""
	}
	return m.spinner.View() + " " + lipgloss.NewStyle().Foreground(tui.ColorSubtext).Render(m.message)
}

// SetMessage updates the spinner message.
func (m *SpinnerModel) SetMessage(msg string) {
	m.message = msg
}

// SetDone marks the spinner as complete, hiding it.
func (m *SpinnerModel) SetDone() {
	m.done = true
}
