package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
)

// ErrorDismissMsg is sent when the error bar should auto-dismiss.
type ErrorDismissMsg struct{}

// ErrorBar displays an error message that auto-dismisses after 5 seconds.
type ErrorBar struct {
	err       error
	message   string
	visible   bool
	showSince time.Time
}

// NewErrorBar creates a new hidden error bar.
func NewErrorBar() ErrorBar {
	return ErrorBar{}
}

// Update handles the auto-dismiss timer.
func (e ErrorBar) Update(msg tea.Msg) (ErrorBar, tea.Cmd) {
	switch msg.(type) {
	case ErrorDismissMsg:
		e.visible = false
		e.message = ""
		e.err = nil
	}
	return e, nil
}

// View renders the error bar. Returns empty string if not visible.
func (e ErrorBar) View() string {
	if !e.visible || e.message == "" {
		return ""
	}
	style := lipgloss.NewStyle().
		Background(tui.ColorError).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)
	return style.Render(e.message)
}

// Show sets the error and makes the bar visible, starting a 5s dismiss timer.
func (e *ErrorBar) Show(err error) ErrorBar {
	e.err = err
	e.message = models.UserMessage(err)
	e.visible = true
	e.showSince = time.Now()
	return *e
}

// ShowCmd sets the error and returns a tea.Cmd for the dismiss timer.
func (e *ErrorBar) ShowCmd(err error) tea.Cmd {
	e.Show(err)
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return ErrorDismissMsg{}
	})
}

// ShowMessage shows a custom message string.
func (e *ErrorBar) ShowMessage(msg string) ErrorBar {
	e.err = nil
	e.message = msg
	e.visible = true
	e.showSince = time.Now()
	return *e
}

// ShowMessageCmd shows a custom message and returns a tea.Cmd for the dismiss timer.
func (e *ErrorBar) ShowMessageCmd(msg string) tea.Cmd {
	e.ShowMessage(msg)
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return ErrorDismissMsg{}
	})
}

// Dismiss hides the error bar.
func (e *ErrorBar) Dismiss() {
	e.visible = false
	e.message = ""
	e.err = nil
}

// Visible returns whether the error bar is currently shown.
func (e ErrorBar) Visible() bool {
	return e.visible
}
