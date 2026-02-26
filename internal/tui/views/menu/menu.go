package menu

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui"
)

// item represents a single menu entry.
type item struct {
	label string
	view  string
}

var items = []item{
	{"Portfolio", "wallet-status"},
	{"Swap", "swap"},
	{"History", "history"},
	{"Tokens", "token-list"},
	{"Send", "send"},
}

// Model is the main menu view.
type Model struct {
	cursor  int
	address string // active wallet address, seeded into views that need it
	width   int
	height  int
}

// New creates a new menu view. address is the active wallet address.
func New(address string) Model {
	return Model{address: address}
}

// Init is a no-op — the menu has no async work.
func (m Model) Init() tea.Cmd { return nil }

// Update handles key input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(items)-1 {
				m.cursor++
			}
		case "enter", "l", "right":
			return m, m.navigateTo(items[m.cursor])
		case "h", "left", "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}
	return m, nil
}

func (m Model) navigateTo(it item) tea.Cmd {
	var data interface{}
	// Views that need a wallet address use the one passed at construction.
	switch it.view {
	case "wallet-status", "swap", "history", "send":
		if m.address != "" {
			data = m.address
		}
	}
	return func() tea.Msg {
		return tui.NavigateMsg{View: it.view, Data: data}
	}
}

// View renders the menu.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(tui.StyleTitle.Render("Menu") + "\n\n")

	for i, it := range items {
		b.WriteString(tui.RenderRow(it.label, i == m.cursor) + "\n")
	}

	return b.String()
}

// Footer implements tui.FooterProvider.
func (m Model) Footer() string {
	return tui.RenderFooter(
		tui.FooterGroup{{Key: "j/k", Action: "navigate"}, {Key: "enter", Action: "select"}, {Key: "esc", Action: "back"}},
		tui.FooterGroup{{Key: "?", Action: "help"}},
	)
}
