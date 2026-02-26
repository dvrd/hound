package walletstatus

import (
	"context"
	"fmt"
	"os/exec"
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

// clipboardCopiedMsg is sent after copying the wallet address to the clipboard.
type clipboardCopiedMsg struct{ err error }

func copyAddressToClipboard(addr string) tea.Cmd {
	return func() tea.Msg {
		for _, args := range [][]string{
			{"pbcopy"},
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
			{"clip.exe"},
		} {
			c := exec.Command(args[0], args[1:]...)
			c.Stdin = strings.NewReader(addr)
			if err := c.Run(); err == nil {
				return clipboardCopiedMsg{}
			}
		}
		return clipboardCopiedMsg{err: fmt.Errorf("no clipboard command available")}
	}
}

// FilterMode controls which tokens are shown in the list.
type FilterMode int

const (
	FilterDefault FilterMode = iota // hide dust (<$1) and zero-balance tokens
	FilterAll                       // show everything including zero-balance tokens
)

// String returns the status-bar label for each filter mode.
func (f FilterMode) String() string {
	switch f {
	case FilterAll:
		return "show all"
	default:
		return "hide dust"
	}
}

// Model is the wallet status/detail view.
type Model struct {
	wallet      models.Wallet
	portfolio   models.PortfolioBalance
	cursor      int
	sortMode    SortMode
	filterMode  FilterMode      // cycles: hide dust → show dust → show all
	showHidden  bool            // show manually hidden tokens (off by default)
	hiddenMints map[string]bool // mints hidden by the user for this wallet
	loading     bool            // true while a network fetch is in flight
	hasData     bool            // true once we have received at least one portfolio response
	spinner     components.SpinnerModel
	walletMgr   *wallet.WalletManager
	address     string
	width       int
	height      int
	err         error

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
		walletMgr:   walletMgr,
		address:     address,
		loading:     true,
		spinner:     components.NewSpinner("Loading portfolio..."),
		sortMode:    SortByValue,
		db:          db,
		hiddenMints: make(map[string]bool),
		// showDust = false: hide tokens < $1 by default
		// showHidden = false: hide manually hidden tokens by default
	}
	if db != nil {
		if w, err := db.GetWalletByAddress(address); err == nil {
			m.wallet = w
		}
		if hidden, err := db.GetHiddenMints(address); err == nil {
			m.hiddenMints = hidden
		}
	}
	return m
}

// Init fetches the portfolio live and schedules a background refresh every 30s.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.refreshPortfolio(), m.scheduleRefresh())
}

func (m Model) loadPortfolio() tea.Cmd {
	return func() tea.Msg {
		if m.walletMgr == nil {
			return tui.PortfolioRefreshedMsg{Err: fmt.Errorf("wallet manager not available")}
		}
		// If the background preload recorded an error for this wallet, surface it
		// immediately so the user sees it rather than a generic cache-miss error.
		if preloadErr := m.walletMgr.PreloadError(m.address); preloadErr != nil {
			return tui.PortfolioRefreshedMsg{Err: preloadErr}
		}
		// Try cached first — cache is populated either by the walletlist preload
		// or by a previous refresh in this session.
		portfolio, err := m.walletMgr.GetCachedPortfolio(m.address)
		if err != nil {
			// Cache miss (e.g. app launched directly to wallet-status with 1 wallet).
			// Fall back to a live fetch so the user sees real data immediately.
			portfolio, err = m.walletMgr.RefreshPortfolio(context.Background(), m.address)
			return tui.PortfolioRefreshedMsg{Portfolio: portfolio, Err: err}
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
	case clipboardCopiedMsg:
		if msg.err == nil {
			return m, func() tea.Msg { return tui.NotifMsg{Text: "✓ Address copied!"} }
		}
		return m, nil

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
			// Toggle filter: hide dust ↔ show all
			if m.filterMode == FilterDefault {
				m.filterMode = FilterAll
			} else {
				m.filterMode = FilterDefault
			}
			m.clampCursor()
		case "t":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "token-list"}
			}
		case "h":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "history", Data: m.address}
			}
		case "<":
			// Hide the token under the cursor
			tokens := m.visibleTokens()
			if len(tokens) > 0 && m.cursor < len(tokens) {
				t := tokens[m.cursor]
				if m.db != nil {
					_ = m.db.HideToken(m.address, t.Mint)
				}
				m.hiddenMints[t.Mint] = true
				m.clampCursor()
			}
		case ">":
			// Unhide the token under the cursor
			tokens := m.visibleTokens()
			if len(tokens) > 0 && m.cursor < len(tokens) {
				t := tokens[m.cursor]
				if m.hiddenMints[t.Mint] {
					if m.db != nil {
						_ = m.db.UnhideToken(m.address, t.Mint)
					}
					delete(m.hiddenMints, t.Mint)
					m.clampCursor()
				}
			}
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
		case "x":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "swap", Data: m.address}
			}
		case "c":
			// Copy wallet address to clipboard directly — no extra view needed.
			return m, copyAddressToClipboard(m.address)
		case "enter":
			tokens := m.visibleTokens()
			if m.cursor < len(tokens) {
				mint := tokens[m.cursor].Mint
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "token-fetch", Data: mint}
				}
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
		// Only forward non-key messages to the spinner so that filter keys
		// (x, u, h, U, a, etc.) are never swallowed during a background refresh.
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
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
		// Filter 1: manually hidden tokens (scam/spam)
		if m.hiddenMints[t.Mint] && !m.showHidden {
			continue
		}
		// Filter 2: value filter — FilterDefault hides dust (<$1) and zero-balance;
		// FilterAll shows everything.
		if m.filterMode == FilterDefault && (t.USDValue < 1.0 || (t.USDValue == 0 && t.Amount == 0)) {
			continue
		}
		tokens = append(tokens, t)
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

	// Header — show label if available, truncated address always.
	title := tui.StyleTitle.Render("Wallet Status")
	b.WriteString(title + "\n")

	if m.wallet.Label != "" {
		b.WriteString(tui.StyleBold.Render(m.wallet.Label) + "\n")
	}
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
			// Failed before we ever got data — show a prominent error with retry hint.
			b.WriteString(tui.StyleWarning.Render("  ⚠  Portfolio data unavailable") + "\n")
			b.WriteString(tui.StyleMuted.Render("     last error: "+m.err.Error()) + "\n\n")
			b.WriteString(tui.StyleMuted.Render("  [r] retry") + "\n")
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
	solLine := fmt.Sprintf("SOL  %s  %s  ",
		wallet.FormatBalance(sol.Amount),
		wallet.FormatPrice(sol.USDValue))
	b.WriteString(solLine + tui.FormatChange(sol.Change24h) + "\n\n")

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
		b.WriteString(tui.TableSeparator(w) + "\n")

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

		for i := startIdx; i < endIdx; i++ {
			t := tokens[i]
			// Build plain symbol+name columns, then styled numeric columns.
			symCell := fmt.Sprintf("%-*s", colSym, tui.Truncate(t.Symbol, colSym))
			nameCell := fmt.Sprintf("%-*s", colName, tui.Truncate(t.Name, colName))
			balCell := tui.StyleValue.Render(fmt.Sprintf("%*s", colBal, wallet.FormatBalance(t.Amount)))
			priceCell := tui.StyleValue.Render(fmt.Sprintf("%*s", colPrice, wallet.FormatPrice(t.USDPrice)))
			valCell := tui.StyleValue.Render(fmt.Sprintf("%*s", colVal, wallet.FormatPrice(t.USDValue)))
			plainChg := tui.FormatChangePlain(t.Change24h)
			paddedChg := fmt.Sprintf("%*s", colChg, plainChg)
			chgCell := tui.ColorizeChange(t.Change24h, paddedChg)
			row := symCell + " " + nameCell + " " + balCell + " " + priceCell + " " + valCell + " " + chgCell

			// Show a dim [hidden] tag so the user knows they can unhide it
			if m.hiddenMints[t.Mint] {
				row += " " + tui.StyleMuted.Render("[hidden]")
			}

			b.WriteString(tui.RenderRow(row, i == m.cursor) + "\n")
		}

		// Scroll indicator
		if len(tokens) > maxRows {
			hidden := len(tokens) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	} else {
		b.WriteString(tui.StyleMuted.Render("No tokens found") + "\n")
	}

	// Sort indicator + active filters + refresh time
	b.WriteString("\n")
	sortLine := fmt.Sprintf("Sort: %s  filter:%s", m.sortMode.String(), m.filterMode.String())
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
	// Error state with no data: show a minimal footer focused on retry.
	if !m.hasData && m.err != nil {
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "r", Action: "retry"}, {Key: "esc", Action: "back"}},
		)
	}
	return tui.RenderFooter(
		tui.FooterGroup{
			{Key: "w", Action: "wallets"}, {Key: "s", Action: "send"},
			{Key: "x", Action: "swap"}, {Key: "h", Action: "history"},
			{Key: "t", Action: "tokens"},
		},
		tui.FooterGroup{
			{Key: "?", Action: "help"},
		},
	)
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

// GetFilterMode returns the current filter mode for testing.
func (m Model) GetFilterMode() FilterMode {
	return m.filterMode
}

// WalletAddress returns the wallet address for this view.
// Used by the app shell to seed the menu with the correct address.
func (m Model) WalletAddress() string {
	return m.address
}

// IsRenaming returns whether the rename mode is active for testing.
func (m Model) IsRenaming() bool {
	return m.renaming
}
