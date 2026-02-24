package walletstatus

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
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

// autoRefreshTickMsg is sent by the auto-refresh timer.
type autoRefreshTickMsg struct{}

// Model is the wallet status/detail view.
type Model struct {
	wallet    models.Wallet
	portfolio models.PortfolioBalance
	cursor    int
	sortMode  SortMode
	showAll   bool // show zero-balance tokens
	loading   bool // true while a network fetch is in flight
	hasData   bool // true once we have received at least one portfolio response
	spinner   components.SpinnerModel
	walletMgr *wallet.WalletManager
	address   string
	width     int
	height    int
	err       error

	// Rename mode
	renaming    bool
	renameInput textinput.Model
	renameErr   error
	db          *database.Database

	// Auto-refresh
	lastRefresh time.Time
}

// New creates a new wallet status view.
func New(walletMgr *wallet.WalletManager, address string, db *database.Database) Model {
	m := Model{
		walletMgr: walletMgr,
		address:   address,
		loading:   true,
		spinner:   components.NewSpinner("Loading portfolio..."),
		sortMode:  SortByValue,
		db:        db,
	}
	if db != nil {
		if w, err := db.GetWalletByAddress(address); err == nil {
			m.wallet = w
		}
	}
	return m
}

// Init loads the portfolio from cache (populated by the startup preload goroutine)
// and schedules a background refresh every 30s.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadPortfolio(), m.scheduleRefresh())
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
		portfolio, err := m.walletMgr.RefreshPortfolio(context.Background(), m.address)
		return tui.PortfolioRefreshedMsg{Portfolio: portfolio, Err: err}
	}
}

func (m Model) scheduleRefresh() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle rename mode first
	if m.renaming {
		return m.updateRename(msg)
	}

	switch msg := msg.(type) {
	case tui.PortfolioRefreshedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			// Keep existing portfolio visible on error — don't wipe data.
			// H1: Reschedule even on error so we keep trying.
			return m, m.scheduleRefresh()
		}
		m.portfolio = msg.Portfolio
		m.hasData = true
		m.err = nil
		m.lastRefresh = time.Now()
		// H1: Reschedule next refresh only after this one completes (one-shot timer).
		return m, m.scheduleRefresh()

	case autoRefreshTickMsg:
		// H1: Tick fires once; start refresh but do NOT reschedule here.
		// The next tick is scheduled inside PortfolioRefreshedMsg.
		if !m.loading {
			m.loading = true
			m.spinner = components.NewSpinner("Refreshing...")
			return m, tea.Batch(m.spinner.Init(), m.refreshPortfolio())
		}
		// Already loading — reschedule so we don't lose the timer entirely.
		return m, m.scheduleRefresh()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.renaming {
			maxW := m.width - 10
			if maxW < 10 {
				maxW = 10
			}
			m.renameInput.Width = min(30, maxW)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "r":
			m.loading = true
			m.spinner = components.NewSpinner("Refreshing portfolio...")
			return m, tea.Batch(m.spinner.Init(), m.refreshPortfolio())
		case "R":
			m.renaming = true
			m.renameErr = nil
			ri := textinput.New()
			ri.Placeholder = "New wallet name"
			ri.CharLimit = 32
			maxW := m.width - 10
			if maxW < 10 {
				maxW = 10
			}
			ri.Width = min(30, maxW)
			ri.Focus()
			m.renameInput = ri
			return m, ri.Focus()
		case "a":
			m.showAll = !m.showAll
			m.clampCursor()
		case "1":
			m.sortMode = SortByValue
			m.clampCursor()
		case "2":
			m.sortMode = SortBySymbol
			m.clampCursor()
		case "3":
			m.sortMode = SortByBalance
			m.clampCursor()
		case "s":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "send", Data: m.address}
			}
		case "c":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "receive", Data: m.address}
			}
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

func (m Model) updateRename(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Pass non-key messages through (e.g., blink)
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "esc":
		m.renaming = false
		m.renameErr = nil
		return m, nil
	case "enter":
		newLabel := strings.TrimSpace(m.renameInput.Value())
		if newLabel == "" {
			m.renameErr = fmt.Errorf("name cannot be empty")
			return m, nil
		}
		if m.db != nil {
			if err := m.db.UpdateWalletLabel(m.address, newLabel); err != nil {
				m.renameErr = err
				return m, nil
			}
		}
		// M8: Update in-memory label so UI reflects change immediately
		m.wallet.Label = newLabel
		m.renaming = false
		m.renameErr = nil
		return m, nil
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
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

// clampCursor ensures the cursor is within the bounds of visible tokens.
func (m *Model) clampCursor() {
	tokens := m.visibleTokens()
	if len(tokens) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(tokens) {
		m.cursor = len(tokens) - 1
	}
}

// View renders the wallet status.
func (m Model) View() string {
	var b strings.Builder

	// Header
	title := tui.StyleTitle.Render("Wallet Status")
	b.WriteString(title + "\n")

	addrDisplay := tui.TruncateAddress(m.address)
	b.WriteString(tui.StyleSubtitle.Render(addrDisplay) + "\n\n")

	// Rename overlay
	if m.renaming {
		b.WriteString("Rename wallet:\n\n")
		b.WriteString(m.renameInput.View() + "\n\n")
		if m.renameErr != nil {
			b.WriteString(tui.StyleError.Render("Error: "+m.renameErr.Error()) + "\n\n")
		}
		b.WriteString(tui.StyleMuted.Render("Enter to save, Esc to cancel"))
		return b.String()
	}

	// First load: no data yet.
	if !m.hasData {
		if m.err != nil {
			// Failed before we ever got data — show the error.
			b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		} else {
			b.WriteString(m.spinner.View() + "\n")
		}
		return b.String()
	}

	// Error with no prior data is already handled above (!hasData).
	// If we have data and an error, show the error inline but keep the table.

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

	// Token table with proportional columns
	tokens := m.visibleTokens()
	w := m.width
	if w <= 0 {
		w = 80
	}
	colSym := max(6, w*11/100)
	colName := max(10, w*18/100)
	colBal := max(8, w*13/100)
	colPrice := max(8, w*12/100)
	colVal := max(8, w*12/100)
	colChg := max(6, w*8/100)

	if len(tokens) > 0 {
		headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%%ds %%%ds %%%ds %%%ds", colSym, colName, colBal, colPrice, colVal, colChg)
		header := fmt.Sprintf(headerFmt, "Symbol", "Name", "Balance", "Price", "Value", "24h")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Cap visible rows
		maxRows := len(tokens)
		if m.height > 0 {
			visible := m.height - 10
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
		if endIdx > len(tokens) {
			endIdx = len(tokens)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		rowFmt := fmt.Sprintf("%%-%ds %%-%ds %%%ds %%%ds %%%ds %%%ds", colSym, colName, colBal, colPrice, colVal, colChg)
		for i := startIdx; i < endIdx; i++ {
			t := tokens[i]
			row := fmt.Sprintf(rowFmt,
				tui.Truncate(t.Symbol, colSym),
				tui.Truncate(t.Name, colName),
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

		// Scroll indicator
		if len(tokens) > maxRows {
			hidden := len(tokens) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	} else {
		b.WriteString(tui.StyleMuted.Render("No tokens found") + "\n")
	}

	// Sort indicator + refresh status line
	b.WriteString("\n")
	sortLine := fmt.Sprintf("Sort: %s", m.sortMode.String())
	if !m.lastRefresh.IsZero() {
		sortLine += fmt.Sprintf("  |  %s", m.lastRefresh.Format("15:04:05"))
	}
	b.WriteString(tui.StyleMuted.Render(sortLine) + "\n")

	// Inline refresh indicator — shown while a background fetch is in flight.
	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
	} else if m.err != nil {
		b.WriteString(tui.StyleError.Render("⚠ "+m.err.Error()) + "\n")
	}

	return b.String()
}

// Footer implements tui.FooterProvider — returns the pinned status bar text.
func (m Model) Footer() string {
	showAllLabel := "[a]ll"
	if m.showAll {
		showAllLabel = "[a]ll*"
	}
	if m.width > 0 && m.width < 80 {
		return fmt.Sprintf("[s]end [c]rcv [r]ef [R]en %s [1][2][3] [esc]", showAllLabel)
	}
	return fmt.Sprintf("[s]end re[c]eive [r]efresh [R]ename %s [1]value [2]symbol [3]balance [esc]back", showAllLabel)
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

// IsRenaming returns whether the rename mode is active for testing.
func (m Model) IsRenaming() bool {
	return m.renaming
}
