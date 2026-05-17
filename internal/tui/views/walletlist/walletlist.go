package walletlist

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	PartialErr error // set when some wallets failed to refresh
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
	partialErr error // set when some wallet refreshes failed
	help       components.HelpModel
}

// New creates a new wallet list view.
func New(walletMgr *wallet.WalletManager, db *database.Database) Model {
	return Model{
		walletMgr:  walletMgr,
		db:         db,
		portfolios: make(map[string]models.PortfolioBalance),
		loading:    true,
		spinner:    components.NewSpinner("Loading wallets..."),
		help: components.NewHelp("Wallet List", []components.KeyBinding{
			{Key: "↑/↓ j/k", Description: "navigate"},
			{Key: "enter", Description: "open status"},
			{Key: "S", Description: "send"},
			{Key: "x", Description: "swap"},
			{Key: "h", Description: "history"},
			{Key: "t", Description: "tokens"},
			{Key: "i", Description: "import wallet"},
			{Key: "d", Description: "delete wallet"},
			{Key: "r", Description: "refresh portfolios"},
			{Key: "?", Description: "toggle help"},
		}),
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
		m.partialErr = msg.PartialErr
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
		// Let the help overlay consume '?' before other key handling.
		var helpCmd tea.Cmd
		m.help, helpCmd = m.help.Update(msg)
		if helpCmd != nil {
			return m, helpCmd
		}
		// If help is visible, swallow all other keys.
		if m.help.Visible() {
			return m, nil
		}

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
		case "x":
			addr := ""
			if len(m.wallets) > 0 {
				addr = m.wallets[m.cursor].Address
			}
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "swap", Data: addr}
			}
		case "S":
			if len(m.wallets) > 0 {
				addr := m.wallets[m.cursor].Address
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "send", Data: addr}
				}
			}
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
		// Use the parallelized RefreshAllPortfolios (bounded concurrency).
		portfolios, err := m.walletMgr.RefreshAllPortfolios(context.Background())
		if err != nil {
			return WalletsLoadedMsg{Err: err}
		}
		wallets, wErr := m.db.GetAllWallets()
		if wErr != nil {
			return WalletsLoadedMsg{Err: wErr}
		}
		var partialErr error
		if len(portfolios) < len(wallets) {
			failed := len(wallets) - len(portfolios)
			partialErr = fmt.Errorf("%d wallet(s) could not be refreshed", failed)
		}
		return WalletsLoadedMsg{Wallets: wallets, Portfolios: portfolios, PartialErr: partialErr}
	}
}

// View renders the wallet list.
func (m Model) View() string {
	// Help overlay takes priority over all other content.
	if overlay := m.help.View(); overlay != "" {
		return overlay
	}

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

	if m.partialErr != nil {
		b.WriteString(tui.StyleWarning.Render("⚠ "+m.partialErr.Error()) + "\n")
	}

	if len(m.wallets) == 0 {
		b.WriteString(tui.StyleMuted.Render("No wallets found. Press [i] to import one.") + "\n")
	} else {
		// Compute proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colLabel := max(6, w*15/100)
		colAddr := max(8, w*18/100)
		colType := max(8, w*20/100)
		colBal := max(8, w*12/100)

		// Table header — 4-char prefix ("  " badge + "  " cursor) to align with rows.
		headerFmt := fmt.Sprintf("    %%-%ds %%-%ds %%-%ds %%%ds", colLabel, colAddr, colType, colBal)
		header := fmt.Sprintf(headerFmt, "Label", "Address", "Type", "Balance")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")
		b.WriteString(tui.TableSeparator(w) + "\n")

		// Cap visible rows
		maxRows := len(m.wallets)
		if m.height > 0 {
			visible := m.height - 8
			if visible < 1 {
				visible = 1
			}
			if visible < maxRows {
				maxRows = visible
			}
		}

		// Determine visible window around cursor
		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(m.wallets) {
			endIdx = len(m.wallets)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		// Table rows — build each cell independently so ANSI codes from one cell
		// never affect adjacent column width calculations.
		for i := startIdx; i < endIdx; i++ {
			w := m.wallets[i]

			// 2-char primary badge (plain, fixed width).
			primaryPlain := "  "
			if w.IsPrimary {
				primaryPlain = "* "
			}
			var primaryStyled string
			if w.IsPrimary {
				primaryStyled = tui.StylePrimaryBadge.Render(primaryPlain)
			} else {
				primaryStyled = primaryPlain
			}

			// Build each cell to its exact column width using plain strings.
			labelCell := fmt.Sprintf("%-*s", colLabel, tui.Truncate(w.Label, colLabel))
			addrCell := fmt.Sprintf("%-*s", colAddr, tui.TruncateAddress(w.Address))
			typePlain := fmt.Sprintf("%-*s", colType, tui.Truncate(w.WalletType.String(), colType))
			balPlain := "$0.00"
			if p, ok := m.portfolios[w.Address]; ok {
				balPlain = wallet.FormatPrice(p.TotalUSD)
			}
			balCell := tui.StyleValue.Render(fmt.Sprintf("%*s", colBal, balPlain))

			// Apply color only to the pre-padded type cell — width is already fixed.
			coloredType := tui.StyleTypeBadge.Render(typePlain)

			// Assemble: badge(2) + label + addr + coloredType + balance.
			// Header uses 4-char indent ("    ") to match badge(2) + RenderRow indent(2).
			coloredRow := primaryStyled + labelCell + " " + addrCell + " " + coloredType + " " + balCell

			b.WriteString(tui.RenderRow(coloredRow, i == m.cursor) + "\n")
		}

		// Scroll indicator
		if len(m.wallets) > maxRows {
			hidden := len(m.wallets) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}

		// Footer: total USD
		var totalUSD float64
		for _, p := range m.portfolios {
			totalUSD += p.TotalUSD
		}
		b.WriteString("\n")
		b.WriteString(tui.StyleBold.Render(fmt.Sprintf("  Total: %s", wallet.FormatPrice(totalUSD))) + "\n")
	}

	return b.String()
}

// Footer implements tui.FooterProvider — returns the pinned status bar text.
// Pre-rendered static footer.
var listFooter = tui.RenderFooter(
	tui.FooterGroup{
		{Key: "enter", Action: "status"}, {Key: "S", Action: "send"},
		{Key: "x", Action: "swap"}, {Key: "h", Action: "history"},
		{Key: "t", Action: "tokens"},
	},
	tui.FooterGroup{{Key: "?", Action: "help"}},
)

func (m Model) Footer() string {
	return listFooter
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


