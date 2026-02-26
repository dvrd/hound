package tokenlist

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// TokensLoadedMsg is sent when tokens and pool stats have been loaded.
type TokensLoadedMsg struct {
	Tokens []TokenRow
	Err    error
}

// TokenRow holds a token and its aggregate pool stats.
type TokenRow struct {
	Token     models.Token
	PoolStats models.PoolStats
}

// Model is the token list view.
type Model struct {
	tokens  []TokenRow
	cursor  int
	db      *database.Database
	loading bool
	spinner components.SpinnerModel
	width   int
	height  int
	err     error
}

// New creates a new token list view.
func New(db *database.Database) Model {
	return Model{
		db:      db,
		loading: true,
		spinner: components.NewSpinner("Loading tokens..."),
	}
}

// Init starts loading tokens.
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

// Update handles messages.
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "h":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tokens)-1 {
				m.cursor++
			}
		case "enter", "l":
			if len(m.tokens) > 0 && m.cursor < len(m.tokens) {
				addr := m.tokens[m.cursor].Token.ContractAddress
				return m, func() tea.Msg {
					return tui.NavigateMsg{View: "token-fetch", Data: addr}
				}
			}
		case "a":
			return m, func() tea.Msg {
				return tui.NavigateMsg{View: "token-add"}
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

// View renders the token list.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Token List")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.tokens) == 0 {
		b.WriteString(tui.StyleMuted.Render("No tokens found. Press [a] to add one.") + "\n")
	} else {
		// Proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colSym := max(6, w*13/100)
		colName := max(10, w*25/100)
		colPools := max(5, w*8/100)
		colLiq := max(10, w*18/100)

		headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%%ds %%%ds %%s", colSym, colName, colPools, colLiq)
		header := fmt.Sprintf(headerFmt, "Symbol", "Name", "Pools", "Liquidity", "")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		// Cap visible rows
		maxRows := len(m.tokens)
		if m.height > 0 {
			visible := m.height - 6
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
		if endIdx > len(m.tokens) {
			endIdx = len(m.tokens)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		rowFmt := fmt.Sprintf("%%-%ds %%-%ds %%%dd %%%ds %%s", colSym, colName, colPools, colLiq)
		for i := startIdx; i < endIdx; i++ {
			tr := m.tokens[i]
			autoDiscovered := ""
			for _, p := range tr.Token.Pools {
				if p.DiscoveredAt > 0 {
					autoDiscovered = "auto"
					break
				}
			}

			row := fmt.Sprintf(rowFmt,
				tui.Truncate(tr.Token.Symbol, colSym),
				tui.Truncate(tr.Token.Name, colName),
				tr.PoolStats.PoolCount,
				wallet.FormatLargeNumber(tr.PoolStats.TotalLiquidity),
				autoDiscovered,
			)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}

		// Scroll indicator
		if len(m.tokens) > maxRows {
			hidden := len(m.tokens) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	}

	return b.String()
}

// Footer implements tui.FooterProvider — returns the pinned status bar text.
func (m Model) Footer() string {
	return "[j/k]navigate [l/enter]details [a]dd [h/esc]back"
}

// GetCursor returns the current cursor position for testing.
func (m Model) GetCursor() int {
	return m.cursor
}

// GetTokens returns the loaded tokens for testing.
func (m Model) GetTokens() []TokenRow {
	return m.tokens
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
