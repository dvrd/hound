package walletstatus

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// SortMode determines how tokens are sorted.
type SortMode int

const (
	SortByValue   SortMode = iota // Sort by USD value descending
	SortBySymbol                  // Sort alphabetically by symbol
	SortByBalance                 // Sort by token amount descending
)

// String returns the display name for a sort mode.
func (s SortMode) String() string {
	switch s {
	case SortByValue:
		return "Value"
	case SortBySymbol:
		return "Symbol"
	case SortByBalance:
		return "Balance"
	default:
		return "Unknown"
	}
}

// Model is the wallet status/detail view.
type Model struct {
	wallet    models.Wallet
	portfolio models.PortfolioBalance
	cursor    int
	sortMode  SortMode
	showAll   bool // show zero-balance tokens
	loading   bool
	spinner   components.SpinnerModel
	walletMgr *wallet.WalletManager
	address   string
	width     int
	height    int
	err       error
}

// New creates a new wallet status view.
func New(walletMgr *wallet.WalletManager, address string) Model {
	return Model{
		walletMgr: walletMgr,
		address:   address,
		loading:   true,
		spinner:   components.NewSpinner("Loading portfolio..."),
		sortMode:  SortByValue,
	}
}

// Init starts loading the portfolio.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadPortfolio())
}

func (m Model) loadPortfolio() tea.Cmd {
	return func() tea.Msg {
		if m.walletMgr == nil {
			return tui.PortfolioRefreshedMsg{Err: fmt.Errorf("wallet manager not available")}
		}
		// Try cached first
		portfolio, err := m.walletMgr.GetCachedPortfolio(m.address)
		if err != nil {
			return tui.PortfolioRefreshedMsg{Err: err}
		}
		return tui.PortfolioRefreshedMsg{Portfolio: portfolio}
	}
}

func (m Model) refreshPortfolio() tea.Cmd {
	return func() tea.Msg {
		if m.walletMgr == nil {
			return tui.PortfolioRefreshedMsg{Err: fmt.Errorf("wallet manager not available")}
		}
		portfolio, err := m.walletMgr.RefreshPortfolio(m.address)
		return tui.PortfolioRefreshedMsg{Portfolio: portfolio, Err: err}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.PortfolioRefreshedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.portfolio = msg.Portfolio
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "r":
			m.loading = true
			m.spinner = components.NewSpinner("Refreshing portfolio...")
			return m, tea.Batch(m.spinner.Init(), m.refreshPortfolio())
		case "a":
			m.showAll = !m.showAll
		case "1":
			m.sortMode = SortByValue
		case "2":
			m.sortMode = SortBySymbol
		case "3":
			m.sortMode = SortByBalance
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			tokens := m.visibleTokens()
			if m.cursor < len(tokens)-1 {
				m.cursor++
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

func (m Model) visibleTokens() []models.TokenBalance {
	tokens := make([]models.TokenBalance, 0, len(m.portfolio.TokenBalances))
	for _, t := range m.portfolio.TokenBalances {
		if m.showAll || t.USDValue > 0 || t.Amount > 0 {
			tokens = append(tokens, t)
		}
	}

	switch m.sortMode {
	case SortByValue:
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].USDValue > tokens[j].USDValue
		})
	case SortBySymbol:
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].Symbol < tokens[j].Symbol
		})
	case SortByBalance:
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].Amount > tokens[j].Amount
		})
	}

	return tokens
}

// View renders the wallet status.
func (m Model) View() string {
	var b strings.Builder

	// Header
	title := tui.StyleTitle.Render("Wallet Status")
	b.WriteString(title + "\n")

	addrDisplay := m.address
	if len(addrDisplay) > 11 {
		addrDisplay = addrDisplay[:4] + "..." + addrDisplay[len(addrDisplay)-4:]
	}
	b.WriteString(tui.StyleSubtitle.Render(addrDisplay) + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// Total USD
	totalStr := wallet.FormatPrice(m.portfolio.TotalUSD)
	b.WriteString(tui.StyleBold.Render("Total: "+totalStr) + "\n\n")

	// SOL balance
	sol := m.portfolio.SOLBalance
	solLine := fmt.Sprintf("SOL  %s  %s  %s",
		wallet.FormatBalance(sol.Amount),
		wallet.FormatPrice(sol.USDValue),
		tui.FormatChange(sol.Change24h))
	b.WriteString(solLine + "\n\n")

	// Token table
	tokens := m.visibleTokens()
	if len(tokens) > 0 {
		header := fmt.Sprintf("%-10s %12s %10s %12s %8s",
			"Symbol", "Balance", "Price", "Value", "24h")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		for i, t := range tokens {
			row := fmt.Sprintf("%-10s %12s %10s %12s %8s",
				truncate(t.Symbol, 10),
				wallet.FormatBalance(t.Amount),
				wallet.FormatPrice(t.USDPrice),
				wallet.FormatPrice(t.USDValue),
				tui.FormatChange(t.Change24h))

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}
	} else {
		b.WriteString(tui.StyleMuted.Render("No tokens found") + "\n")
	}

	// Sort indicator
	b.WriteString("\n")
	b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("Sort: %s", m.sortMode.String())) + "\n")

	// Status bar
	showAllLabel := "[a]ll"
	if m.showAll {
		showAllLabel = "[a]ll*"
	}
	b.WriteString(tui.StyleStatusBar.Render(
		fmt.Sprintf("[r]efresh %s [1]value [2]symbol [3]balance [esc]back", showAllLabel)))

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// GetSortMode returns the current sort mode for testing.
func (m Model) GetSortMode() SortMode {
	return m.sortMode
}

// GetShowAll returns whether zero-balance tokens are shown.
func (m Model) GetShowAll() bool {
	return m.showAll
}
