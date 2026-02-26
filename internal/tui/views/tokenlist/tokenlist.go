package tokenlist

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// ─── Messages ────────────────────────────────────────────────────────────────

// TokensLoadedMsg is sent when the saved tokens have been loaded from DB.
type TokensLoadedMsg struct {
	Tokens []TokenRow
	Err    error
}

// searchResultsMsg is sent when a Jupiter search completes.
type searchResultsMsg struct {
	query   string
	results []SearchResult
	err     error
}

// debounceMsg fires after the debounce delay to trigger a Jupiter search.
type debounceMsg struct {
	generation int // matches m.searchGen; stale ticks are discarded
}

// ─── Types ───────────────────────────────────────────────────────────────────

// TokenRow holds a saved token and its aggregate pool stats.
type TokenRow struct {
	Token     models.Token
	PoolStats models.PoolStats
}

// SearchResult is a single entry in the live search results list.
type SearchResult struct {
	Symbol  string
	Name    string
	Address string
	Saved   bool // true if this token is already stored in DB
}

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tui.ColorPrimary).
			Padding(0, 1)

	savedBadgeStyle = lipgloss.NewStyle().
			Foreground(tui.ColorSuccess).
			Bold(true)
)

// ─── Model ───────────────────────────────────────────────────────────────────

// Model is the token list / search view.
type Model struct {
	// Saved tokens (loaded from DB on init)
	tokens []TokenRow

	// Search state
	searchInput  textinput.Model
	searchGen    int            // incremented on every keystroke for debounce
	searching    bool           // true while Jupiter fetch is in flight
	searchErr    error          // last search error
	results      []SearchResult // live results from Jupiter
	inSearchMode bool           // true when a query is active (show results pane)

	// Navigation
	cursor int

	// Infrastructure
	db      *database.Database
	jupiter *dex.JupiterClient
	loading bool
	spinner components.SpinnerModel
	width   int
	height  int
	err     error
}

// New creates a new token list / search view.
func New(db *database.Database) Model {
	return NewWithJupiter(db, dex.NewJupiterClient())
}

// NewWithJupiter creates the model with a custom Jupiter client (for testing).
func NewWithJupiter(db *database.Database, jupiter *dex.JupiterClient) Model {
	si := textinput.New()
	si.Placeholder = "Search tokens…"
	si.Prompt = ""
	si.CharLimit = 128
	si.Width = 40 // corrected on first WindowSizeMsg
	si.PromptStyle = lipgloss.NewStyle().Foreground(tui.ColorPrimary).Bold(true)
	si.TextStyle = lipgloss.NewStyle().Foreground(tui.ColorText)
	si.PlaceholderStyle = lipgloss.NewStyle().Foreground(tui.ColorMuted)
	si.Focus()

	return Model{
		db:          db,
		jupiter:     jupiter,
		loading:     true,
		spinner:     components.NewSpinner("Loading tokens..."),
		searchInput: si,
	}
}

// ─── Init ────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadTokens())
}

func (m Model) loadTokens() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return TokensLoadedMsg{Err: fmt.Errorf("database not available")}
		}
		tokens, err := m.db.GetAllTokens()
		if err != nil {
			return TokensLoadedMsg{Err: err}
		}
		rows := make([]TokenRow, 0, len(tokens))
		for _, t := range tokens {
			stats, _ := m.db.GetPoolStats(t.Symbol)
			rows = append(rows, TokenRow{Token: t, PoolStats: stats})
		}
		return TokensLoadedMsg{Tokens: rows}
	}
}

// ─── Debounce ────────────────────────────────────────────────────────────────

const searchDebounce = 300 * time.Millisecond

func (m Model) scheduleSearch() tea.Cmd {
	gen := m.searchGen
	return tea.Tick(searchDebounce, func(_ time.Time) tea.Msg {
		return debounceMsg{generation: gen}
	})
}

func (m Model) doSearch(query string) tea.Cmd {
	savedAddrs := make(map[string]bool, len(m.tokens))
	for _, tr := range m.tokens {
		savedAddrs[tr.Token.ContractAddress] = true
	}
	jupiter := m.jupiter
	return func() tea.Msg {
		results, err := jupiter.LookupTokenList(query)
		if err != nil {
			if errors.Is(err, models.ErrTokenNotFound) {
				return searchResultsMsg{query: query, results: nil}
			}
			return searchResultsMsg{query: query, err: err}
		}
		sr := make([]SearchResult, 0, len(results))
		for _, r := range results {
			sr = append(sr, SearchResult{
				Symbol:  r.Symbol,
				Name:    r.Name,
				Address: r.Address,
				Saved:   savedAddrs[r.Address],
			})
		}
		return searchResultsMsg{query: query, results: sr}
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case TokensLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.tokens = msg.Tokens
		return m, nil

	case debounceMsg:
		if msg.generation != m.searchGen {
			return m, nil
		}
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			return m, nil
		}
		m.searching = true
		return m, m.doSearch(query)

	case searchResultsMsg:
		if msg.query != strings.TrimSpace(m.searchInput.Value()) {
			return m, nil
		}
		m.searching = false
		m.searchErr = msg.err
		m.results = msg.results
		m.inSearchMode = true
		m.cursor = 0
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// The search box border (2) + padding (2) + outer container margin (2)
		// = 6 total cols to subtract so the box never overflows the terminal.
		w := m.width - 6
		if w < 10 {
			w = 10
		}
		m.searchInput.Width = w
		return m, nil

	case tea.KeyMsg:
		query := m.searchInput.Value()

		switch msg.String() {

		// ── Always exit ────────────────────────────────────────────────────
		case "esc":
			if query != "" {
				// First esc: clear search, return to idle
				m.searchInput.SetValue("")
				m.searchGen++
				m.results = nil
				m.inSearchMode = false
				m.searching = false
				m.searchErr = nil
				m.cursor = 0
				return m, nil
			}
			// Second esc (empty search): navigate back to main menu
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }

		case "h":
			if query == "" {
				// Only exit with h when not typing
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			}
			// h is a valid search character — fall through to input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		// ── Cursor navigation — always active ──────────────────────────────
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "ctrl+n":
			if max := m.listLen() - 1; m.cursor < max {
				m.cursor++
			}
			return m, nil

		// ── Vim nav — only when input is empty ────────────────────────────
		case "j":
			if query == "" {
				if max := m.listLen() - 1; m.cursor < max {
					m.cursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		case "k":
			if query == "" {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		case "l":
			if query == "" {
				addr := m.selectedAddress()
				if addr == "" {
					return m, nil
				}
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "token-fetch", Data: addr}
				}
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		// ── Enter: open selected OR trigger immediate search ───────────────
		case "enter":
			addr := m.selectedAddress()
			if addr != "" {
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "token-fetch", Data: addr}
				}
			}
			// No selection but query is present → trigger search now
			q := strings.TrimSpace(query)
			if q != "" {
				m.searching = true
				m.searchGen++
				return m, m.doSearch(q)
			}
			return m, nil

		// ── All other keys go to search input ─────────────────────────────
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)
		}
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// afterInput is called after forwarding a key to searchInput.Update.
// It bumps the generation counter and schedules a debounced search if the
// query is non-empty, or resets to idle mode if the field was cleared.
func (m *Model) afterInput(inputCmd tea.Cmd) tea.Cmd {
	query := strings.TrimSpace(m.searchInput.Value())
	m.searchGen++
	if query == "" {
		m.results = nil
		m.inSearchMode = false
		m.searching = false
		m.searchErr = nil
		m.cursor = 0
		return inputCmd
	}
	m.inSearchMode = true
	m.cursor = 0
	return tea.Batch(inputCmd, m.scheduleSearch())
}

func (m Model) listLen() int {
	if m.inSearchMode {
		return len(m.results)
	}
	return len(m.tokens)
}

func (m Model) selectedAddress() string {
	if m.inSearchMode {
		if m.cursor < len(m.results) {
			return m.results[m.cursor].Address
		}
		return ""
	}
	if m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].Token.ContractAddress
	}
	return ""
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(tui.StyleTitle.Render("Tokens") + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// ── Search box — single line, no overflow ───────────────────────────────
	// We render only the input inside the box. The "searching…" indicator
	// goes below the box to avoid expanding its height.
	b.WriteString(searchBoxStyle.Render(m.searchInput.View()) + "\n")
	if m.searching {
		b.WriteString(tui.StyleMuted.Render("  searching…") + "\n")
	}
	b.WriteString("\n")

	// ── Content ─────────────────────────────────────────────────────────────
	if m.inSearchMode {
		m.renderSearchResults(&b)
	} else {
		m.renderSavedTokens(&b)
	}

	return b.String()
}

func (m Model) renderSavedTokens(b *strings.Builder) {
	if len(m.tokens) == 0 {
		b.WriteString(tui.StyleMuted.Render("No tokens saved yet. Start typing to search.") + "\n")
		return
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	colSym := max(6, w*13/100)
	colName := max(10, w*28/100)
	colPools := max(5, w*8/100)
	colLiq := max(10, w*18/100)

	headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%%ds %%%ds", colSym, colName, colPools, colLiq)
	b.WriteString(tui.StyleTableHeader.Render(fmt.Sprintf(headerFmt, "Symbol", "Name", "Pools", "Liquidity")) + "\n")
	b.WriteString(tui.TableSeparator(w) + "\n")

	maxRows := len(m.tokens)
	if m.height > 0 {
		// Title(1) + blank(1) + searchbox(3) + blank(1) + header(1) + sep(1) + footer(1) = 9
		visible := m.height - 9
		if visible < 1 {
			visible = 1
		}
		if visible < maxRows {
			maxRows = visible
		}
	}

	startIdx, endIdx := viewWindow(m.cursor, maxRows, len(m.tokens))
	rowFmt := fmt.Sprintf("%%-%ds %%-%ds %%%dd %%%ds", colSym, colName, colPools, colLiq)
	for i := startIdx; i < endIdx; i++ {
		tr := m.tokens[i]
		row := fmt.Sprintf(rowFmt,
			tui.Truncate(tr.Token.Symbol, colSym),
			tui.Truncate(tr.Token.Name, colName),
			tr.PoolStats.PoolCount,
			wallet.FormatLargeNumber(tr.PoolStats.TotalLiquidity),
		)
		b.WriteString(tui.RenderRow(row, i == m.cursor) + "\n")
	}

	if len(m.tokens) > maxRows {
		b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", len(m.tokens)-maxRows)) + "\n")
	}
}

func (m Model) renderSearchResults(b *strings.Builder) {
	if m.searchErr != nil {
		b.WriteString(tui.StyleError.Render("Search error: "+m.searchErr.Error()) + "\n")
		return
	}

	if len(m.results) == 0 {
		q := strings.TrimSpace(m.searchInput.Value())
		b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("No results for %q", q)) + "\n")
		return
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	colSym := max(6, w*13/100)
	colName := max(12, w*30/100)
	colAddr := max(12, w*20/100)

	headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%s", colSym, colName, colAddr)
	b.WriteString(tui.StyleTableHeader.Render(fmt.Sprintf(headerFmt, "Symbol", "Name", "Address", "")) + "\n")
	b.WriteString(tui.TableSeparator(w) + "\n")

	maxRows := len(m.results)
	if m.height > 0 {
		visible := m.height - 9
		if visible < 1 {
			visible = 1
		}
		if visible < maxRows {
			maxRows = visible
		}
	}

	startIdx, endIdx := viewWindow(m.cursor, maxRows, len(m.results))
	rowFmt := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%s", colSym, colName, colAddr)
	for i := startIdx; i < endIdx; i++ {
		r := m.results[i]
		badge := ""
		if r.Saved {
			badge = savedBadgeStyle.Render("●")
		}
		row := fmt.Sprintf(rowFmt,
			tui.Truncate(r.Symbol, colSym),
			tui.Truncate(r.Name, colName),
			tui.TruncateAddress(r.Address),
			badge,
		)
		b.WriteString(tui.RenderRow(row, i == m.cursor) + "\n")
	}

	if len(m.results) > maxRows {
		b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", len(m.results)-maxRows)) + "\n")
	}
}

// viewWindow computes [startIdx, endIdx) keeping cursor within a maxRows window.
func viewWindow(cursor, maxRows, total int) (startIdx, endIdx int) {
	startIdx = 0
	if cursor >= maxRows {
		startIdx = cursor - maxRows + 1
	}
	endIdx = startIdx + maxRows
	if endIdx > total {
		endIdx = total
		startIdx = endIdx - maxRows
		if startIdx < 0 {
			startIdx = 0
		}
	}
	return
}

// ─── Footer ──────────────────────────────────────────────────────────────────

func (m Model) Footer() string {
	if m.searchInput.Value() != "" {
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "↑/↓", Action: "navigate"}, {Key: "enter", Action: "open"}, {Key: "esc", Action: "clear"}, {Key: "esc×2", Action: "back"}},
		)
	}
	return tui.RenderFooter(
		tui.FooterGroup{{Key: "j/k", Action: "navigate"}, {Key: "enter", Action: "open"}, {Key: "esc", Action: "back"}},
	)
}

// ─── Test helpers ────────────────────────────────────────────────────────────

func (m Model) GetCursor() int             { return m.cursor }
func (m Model) GetTokens() []TokenRow      { return m.tokens }
func (m Model) GetResults() []SearchResult { return m.results }
func (m Model) IsInSearchMode() bool       { return m.inSearchMode }
func (m *Model) SetSize(w, h int)          { m.width = w; m.height = h }
