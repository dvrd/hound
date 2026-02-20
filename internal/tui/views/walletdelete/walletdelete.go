package walletdelete

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
)

// WalletDeletedMsg is sent when a wallet deletion completes.
type WalletDeletedMsg struct {
	Err error
}

// Model is the wallet delete confirmation view.
type Model struct {
	wallet       models.Wallet
	confirmInput textinput.Model
	confirmed    bool
	deleting     bool
	err          error
	db           *database.Database
	walletCount  int
	width        int
	height       int
}

// New creates a new wallet delete confirmation view.
func New(w models.Wallet, db *database.Database, walletCount int) Model {
	ti := textinput.New()
	ti.Placeholder = "Paste wallet address to confirm"
	ti.CharLimit = 64
	ti.Width = 50
	ti.Focus()

	return Model{
		wallet:       w,
		confirmInput: ti,
		db:           db,
		walletCount:  walletCount,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WalletDeletedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.deleting = false
			return m, nil
		}
		return m, func() tea.Msg { return tui.NavigateBackMsg{} }

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "enter":
			if m.walletCount <= 1 {
				m.err = fmt.Errorf("cannot delete your only wallet")
				return m, nil
			}
			if m.deleting {
				return m, nil
			}
			input := strings.TrimSpace(m.confirmInput.Value())
			if input != m.wallet.Address {
				m.err = fmt.Errorf("address does not match")
				return m, nil
			}
			m.confirmed = true
			m.deleting = true
			m.err = nil
			return m, m.doDelete()
		}

		if !m.deleting && m.walletCount > 1 {
			var cmd tea.Cmd
			m.confirmInput, cmd = m.confirmInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) doDelete() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return WalletDeletedMsg{Err: fmt.Errorf("database not available")}
		}
		err := m.db.DeleteWallet(m.wallet.Address)
		return WalletDeletedMsg{Err: err}
	}
}

// View renders the delete confirmation.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleError.Render("Delete Wallet")
	b.WriteString(title + "\n\n")

	// Wallet details
	b.WriteString(tui.StyleBold.Render("Address: ") + m.wallet.Address + "\n")
	b.WriteString(tui.StyleBold.Render("Label:   ") + m.wallet.Label + "\n")
	b.WriteString(tui.StyleBold.Render("Type:    ") + m.wallet.WalletType.String() + "\n\n")

	if m.walletCount <= 1 {
		b.WriteString(tui.StyleWarning.Render("Cannot delete your only wallet") + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Esc to go back"))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
	}

	if m.deleting {
		b.WriteString(tui.StyleMuted.Render("Deleting...") + "\n")
		return b.String()
	}

	b.WriteString(tui.StyleWarning.Render("This action cannot be undone!") + "\n\n")
	b.WriteString("Type the full wallet address to confirm deletion:\n\n")
	b.WriteString(m.confirmInput.View() + "\n\n")
	b.WriteString(tui.StyleMuted.Render("Press Enter to confirm, Esc to cancel"))

	return b.String()
}

// IsConfirmed returns whether the deletion was confirmed.
func (m Model) IsConfirmed() bool {
	return m.confirmed
}

// IsDeleting returns whether the deletion is in progress.
func (m Model) IsDeleting() bool {
	return m.deleting
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
