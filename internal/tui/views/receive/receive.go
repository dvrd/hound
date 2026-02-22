package receive

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui"
)

// clipboardResultMsg is sent after a clipboard copy attempt.
type clipboardResultMsg struct {
	err error
}

// Model is the receive view.
type Model struct {
	walletAddr  string
	walletLabel string
	copied      bool
	copyErr     string
	width       int
	height      int
}

// New creates a new receive view.
func New(walletAddr, walletLabel string) Model {
	return Model{
		walletAddr:  walletAddr,
		walletLabel: walletLabel,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case clipboardResultMsg:
		if msg.err != nil {
			m.copyErr = "Clipboard not available — copy address manually"
			m.copied = false
		} else {
			m.copied = true
			m.copyErr = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "c":
			return m, m.copyToClipboard()
		}
	}

	return m, nil
}

func (m Model) copyToClipboard() tea.Cmd {
	return func() tea.Msg {
		err := copyToClipboard(m.walletAddr)
		return clipboardResultMsg{err: err}
	}
}

func copyToClipboard(text string) error {
	for _, cmd := range [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"clip.exe"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stdin = strings.NewReader(text)
		if err := c.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard command available")
}

// View renders the receive view.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Receive")
	b.WriteString(title + "\n\n")

	if m.walletLabel != "" {
		b.WriteString(tui.StyleBold.Render(m.walletLabel) + "\n")
	}

	b.WriteString(m.walletAddr + "\n\n")

	if m.copied {
		b.WriteString(tui.StyleSuccess.Render("Address copied!") + "\n\n")
	} else if m.copyErr != "" {
		b.WriteString(tui.StyleWarning.Render(m.copyErr) + "\n\n")
	} else {
		b.WriteString("Press c to copy address to clipboard\n\n")
	}

	b.WriteString(tui.StyleMuted.Render("Send SOL or SPL tokens to this address") + "\n\n")
	b.WriteString(tui.StyleStatusBar.Render("[c]opy [esc]back"))

	return b.String()
}

// IsCopied returns whether the address was copied for testing.
func (m Model) IsCopied() bool {
	return m.copied
}

// GetCopyErr returns the copy error for testing.
func (m Model) GetCopyErr() string {
	return m.copyErr
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
