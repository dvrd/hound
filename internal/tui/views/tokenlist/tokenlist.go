package tokenlist

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
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

// debounceMsg is an alias for the shared DebounceTickMsg.
type debounceMsg = components.DebounceTickMsg

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
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(tui.ColorPrimary).
				Bold(true)

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
	debouncer    components.Debouncer
	searching    bool           // true while Jupiter fetch is in flight
	searchErr    error          // last search error
	results      []SearchResult // live results from Jupiter
	inSearchMode bool           // true when a query is active (show results pane)

	// Navigation
	cursor int

	// Infrastructure
	db           *database.Database
	tokenCatalog *services.TokenCatalog
	loading      bool
	spinner components.SpinnerModel
	width   int
	height  int
	err     error
}

// New creates a new token list / search view using the given TokenCatalog.
func New(catalog *services.TokenCatalog, db *database.Database) Model {
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
		db:           db,
		tokenCatalog: catalog,
		loading:      true,
		spinner:      components.NewSpinner("Loading tokens..."),
		searchInput:  si,
		debouncer:    components.NewDebouncer(),
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

func (m *Model) scheduleSearch() tea.Cmd {
	return m.debouncer.Bump()
}

func (m Model) doSearch(query string) tea.Cmd {
	catalog := m.tokenCatalog
	return func() tea.Msg {
		results, err := catalog.Search(query)
		if err != nil {
			return searchResultsMsg{query: query, err: err}
		}
		sr := make([]SearchResult, 0, len(results))
		for _, r := range results {
			sr = append(sr, SearchResult{
				Symbol:  r.Symbol,
				Name:    r.Name,
				Address: r.Address,
				Saved:   r.Saved,
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
		if !m.debouncer.IsCurrent(msg.Generation) {
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
		// Prompt "/ " is 2 chars; leave at least 2 cols spare.
		w := m.width - 4
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
				_ = m.debouncer.Bump()
				m.results = nil
				m.inSearchMode = false
				m.searching = false
				m.searchErr = nil
				m.cursor = 0
				return m, nil
			}
			// Second esc (empty search): navigate back to main menu
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }

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

		// j/k navigate in saved-tokens mode; in search mode they type into the input.
		case "j":
			if !m.inSearchMode {
				if max := m.listLen() - 1; m.cursor < max {
					m.cursor++
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		case "k":
			if !m.inSearchMode {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, m.afterInput(cmd)

		// ── Enter: open selected (search results only) OR trigger search ─────
		case "enter":
			q := strings.TrimSpace(query)
			if q == "" {
				// Nothing typed → enter does nothing
				return m, nil
			}
			// Query present: open selected result if any, otherwise search now
			addr := m.selectedAddress()
			if addr != "" {
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "token-fetch", Data: addr}
				}
			}
			m.searching = true
			_ = m.debouncer.Bump()
			return m, m.doSearch(q)

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

	// ── Search bar — prompt prefix + input, no border box ──────────────────
	// A lipgloss border box at full content width causes wrapping artifacts.
	// Instead: colored prompt glyph + the raw textinput, then a separator line.
	// Use m.width-2 for the separator — exact-width strings can wrap due to
	// lipgloss off-by-one when the app container Width == content width.
	prompt := searchPromptStyle.Render("/ ")
	indicator := ""
	if m.searching {
		indicator = "  " + tui.StyleMuted.Render("searching…")
	}
	b.WriteString(prompt + m.searchInput.View() + indicator + "\n")
	sepW := m.width - 4
	if sepW < 4 {
		sepW = 4
	}
	b.WriteString(tui.StyleMuted.Render(strings.Repeat("─", sepW)) + "\n")

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
	// StyleTableHeader and StyleTableRow both have Padding(0,1) = 2 cols.
	// RenderRow prefixes 2 chars ("  " or "┃ "). Total overhead = 4 cols.
	// Subtract so the rendered row fits exactly in w.
	cw := w - 4
	if cw < 20 {
		cw = 20
	}
	colSym := max(6, cw*15/100)
	colName := max(10, cw*32/100)
	colPools := max(5, cw*9/100)
	colLiq := max(10, cw*21/100)

	headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%%ds %%%ds", colSym, colName, colPools, colLiq)
	b.WriteString(tui.StyleTableHeader.Render(fmt.Sprintf(headerFmt, "Symbol", "Name", "Pools", "Liquidity")) + "\n")
	b.WriteString(tui.TableSeparator(w-4) + "\n")

	maxRows := len(m.tokens)
	if m.height > 0 {
		// Title(1) + blank(1) + search(1) + sep(1) + blank(1) + header(1) + sep(1) + footer(1) = 8
		visible := m.height - 8
		if visible < 1 {
			visible = 1
		}
		if visible < maxRows {
			maxRows = visible
		}
	}

	startIdx, endIdx := components.ViewWindow(m.cursor, maxRows, len(m.tokens))
	for i := startIdx; i < endIdx; i++ {
		tr := m.tokens[i]
		row := tui.PadRight(tr.Token.Symbol, colSym) + " " +
			tui.PadRight(tr.Token.Name, colName) + " " +
			tui.PadLeft(strconv.Itoa(tr.PoolStats.PoolCount), colPools) + " " +
			tui.PadLeft(wallet.FormatLargeNumber(tr.PoolStats.TotalLiquidity), colLiq)
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
	// Same 4-col overhead as saved view (padding + RenderRow prefix).
	// Badge "●" is 3 chars (including the space before it), reserve that too.
	cw := w - 4 - 3
	if cw < 24 {
		cw = 24
	}
	colSym := max(6, cw*15/100)
	colName := max(12, cw*35/100)
	colAddr := max(12, cw*24/100)

	headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%-%ds", colSym, colName, colAddr)
	b.WriteString(tui.StyleTableHeader.Render(fmt.Sprintf(headerFmt, "Symbol", "Name", "Address")) + "\n")
	b.WriteString(tui.TableSeparator(w-4) + "\n")

	maxRows := len(m.results)
	if m.height > 0 {
		visible := m.height - 8
		if visible < 1 {
			visible = 1
		}
		if visible < maxRows {
			maxRows = visible
		}
	}

	startIdx, endIdx := components.ViewWindow(m.cursor, maxRows, len(m.results))
	for i := startIdx; i < endIdx; i++ {
		r := m.results[i]
		badge := ""
		if r.Saved {
			badge = " " + savedBadgeStyle.Render("●")
		}
		row := tui.PadRight(r.Symbol, colSym) + " " +
			tui.PadRight(r.Name, colName) + " " +
			tui.PadRight(tui.TruncateAddress(r.Address), colAddr) + badge
		b.WriteString(tui.RenderRow(row, i == m.cursor) + "\n")
	}

	if len(m.results) > maxRows {
		b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", len(m.results)-maxRows)) + "\n")
	}
}



// ─── Footer ──────────────────────────────────────────────────────────────────

func (m Model) Footer() string {
	if m.searchInput.Value() != "" {
		return tui.RenderFooter(
			tui.FooterGroup{
				{Key: "↑/↓", Action: "navigate"}, {Key: "enter", Action: "open"}, {Key: "esc", Action: "clear"},
			},
			tui.FooterGroup{{Key: "?", Action: "help"}},
		)
	}
	return tui.RenderFooter(
		tui.FooterGroup{
			{Key: "↑/↓", Action: "navigate"}, {Key: "enter", Action: "search"}, {Key: "esc", Action: "back"},
		},
		tui.FooterGroup{{Key: "?", Action: "help"}},
	)
}

// ─── Test helpers ────────────────────────────────────────────────────────────

func (m Model) GetCursor() int             { return m.cursor }
func (m Model) GetTokens() []TokenRow      { return m.tokens }
func (m Model) GetResults() []SearchResult { return m.results }
func (m Model) IsInSearchMode() bool       { return m.inSearchMode }
func (m Model) GetSearchValue() string     { return m.searchInput.Value() }
func (m *Model) SetSize(w, h int)          { m.width = w; m.height = h }
