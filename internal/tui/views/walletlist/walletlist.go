package walletlist

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// WalletsLoadedMsg is sent when wallets and portfolios have been loaded.
type WalletsLoadedMsg struct {
	Wallets    []models.Wallet
	Portfolios map[string]models.PortfolioBalance
	Err        error
}

// Model is the wallet list view.
type Model struct {
	wallets    []models.Wallet
	portfolios map[string]models.PortfolioBalance
	cursor     int
	walletMgr  *wallet.WalletManager
	db         *database.Database
	width      int
	height     int
	loading    bool
	spinner    components.SpinnerModel
	err        error
}

// New creates a new wallet list view.
func New(walletMgr *wallet.WalletManager, db *database.Database) Model {
	return Model{
		walletMgr:  walletMgr,
		db:         db,
		portfolios: make(map[string]models.PortfolioBalance),
		loading:    true,
		spinner:    components.NewSpinner("Loading wallets..."),
	}
}

// Init starts loading wallets.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadWallets())
}

func (m Model) loadWallets() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return WalletsLoadedMsg{Err: fmt.Errorf("database not available")}
		}
		wallets, err := m.db.GetAllWallets()
		if err != nil {
			return WalletsLoadedMsg{Err: err}
		}

		portfolios := make(map[string]models.PortfolioBalance)
		if m.walletMgr != nil {
			for _, w := range wallets {
				p, err := m.walletMgr.GetCachedPortfolio(w.Address)
				if err == nil {
					portfolios[w.Address] = p
				}
			}
		}

		return WalletsLoadedMsg{Wallets: wallets, Portfolios: portfolios}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WalletsLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.wallets = msg.Wallets
		m.portfolios = msg.Portfolios
		return m, nil

	case tui.PortfolioRefreshedMsg:
		if msg.Err == nil {
			m.portfolios[msg.Portfolio.WalletAddress] = msg.Portfolio
		}
		return m, nil

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
			if m.cursor < len(m.wallets)-1 {
				m.cursor++
			}
		case "enter", "s":
			if len(m.wallets) > 0 {
				w := m.wallets[m.cursor]
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "wallet-status", Data: w.Address}
				}
			}
		case "i":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "wallet-import"}
			}
		case "d":
			if len(m.wallets) > 0 {
				w := m.wallets[m.cursor]
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "wallet-delete", Data: w}
				}
			}
		case "t":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "token-list"}
			}
		case "h":
			addr := ""
			if len(m.wallets) > 0 {
				addr = m.wallets[m.cursor].Address
			}
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "history", Data: addr}
			}
		case "w":
			addr := ""
			if len(m.wallets) > 0 {
				addr = m.wallets[m.cursor].Address
			}
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "swap", Data: addr}
			}
		case "q":
			return m, tea.Quit
		case "r":
			if m.walletMgr != nil {
				m.loading = true
				m.spinner = components.NewSpinner("Refreshing portfolios...")
				return m, tea.Batch(m.spinner.Init(), m.refreshAll())
			}
		}
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) refreshAll() tea.Cmd {
	return func() tea.Msg {
		portfolios := make(map[string]models.PortfolioBalance)
		wallets, err := m.db.GetAllWallets()
		if err != nil {
			return WalletsLoadedMsg{Err: err}
		}
		for _, w := range wallets {
			p, err := m.walletMgr.RefreshPortfolio(w.Address)
			if err == nil {
				portfolios[w.Address] = p
			}
		}
		return WalletsLoadedMsg{Wallets: wallets, Portfolios: portfolios}
	}
}

// View renders the wallet list.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Hound - Wallet Manager")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.wallets) == 0 {
		b.WriteString(tui.StyleMuted.Render("No wallets found. Press [i] to import one.") + "\n")
	} else {
		// Table header
		header := fmt.Sprintf("  %-3s %-12s %-15s %-16s %10s",
			"", "Label", "Address", "Type", "Balance")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Table rows
		for i, w := range m.wallets {
			primary := "  "
			if w.IsPrimary {
				primary = tui.StylePrimaryBadge.Render("* ")
			}

			addr := TruncateAddress(w.Address)
			typeBadge := tui.StyleTypeBadge.Render(w.WalletType.String())

			balance := "$0.00"
			if p, ok := m.portfolios[w.Address]; ok {
				balance = wallet.FormatPrice(p.TotalUSD)
			}

			row := fmt.Sprintf("%s%-12s %-15s %-16s %10s",
				primary, truncate(w.Label, 12), addr, typeBadge, balance)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render("> "+row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render("  "+row) + "\n")
			}
		}

		// Footer: total USD
		var totalUSD float64
		for _, p := range m.portfolios {
			totalUSD += p.TotalUSD
		}
		b.WriteString("\n")
		b.WriteString(tui.StyleBold.Render(fmt.Sprintf("  Total: %s", wallet.FormatPrice(totalUSD))) + "\n")
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(tui.StyleStatusBar.Render("[i]mport [s]tatus [d]elete [t]okens [w]swap [h]istory [r]efresh [q]uit"))

	return b.String()
}

// TruncateAddress shows first 4 + "..." + last 4 chars.
func TruncateAddress(addr string) string {
	if len(addr) <= 11 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}

// SelectedWallet returns the currently selected wallet, if any.
func (m Model) SelectedWallet() (models.Wallet, bool) {
	if len(m.wallets) == 0 || m.cursor >= len(m.wallets) {
		return models.Wallet{}, false
	}
	return m.wallets[m.cursor], true
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Ensure lipgloss is used (it's used in View via tui styles).
var _ = lipgloss.NewStyle
