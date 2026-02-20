package history

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
)

// HistoryLoadedMsg is sent when swap history has been loaded.
type HistoryLoadedMsg struct {
	Entries []models.SwapHistoryEntry
	Total   int
	Err     error
}

// Model is the swap history view.
type Model struct {
	entries    []models.SwapHistoryEntry
	total      int
	page       int
	pageSize   int
	cursor     int
	walletAddr string
	db         *database.Database
	loading    bool
	spinner    components.SpinnerModel
	width      int
	height     int
	err        error
}

// New creates a new swap history view.
func New(walletAddr string, db *database.Database) Model {
	return Model{
		walletAddr: walletAddr,
		db:         db,
		pageSize:   20,
		loading:    true,
		spinner:    components.NewSpinner("Loading history..."),
	}
}

// Init starts loading history.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadHistory())
}

func (m Model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return HistoryLoadedMsg{Err: fmt.Errorf("database not available")}
		}

		total, err := m.db.GetSwapHistoryCount(m.walletAddr)
		if err != nil {
			return HistoryLoadedMsg{Err: err}
		}

		// Calculate offset for current page
		offset := m.page * m.pageSize
		// GetSwapHistory uses LIMIT but not OFFSET directly,
		// so we fetch enough and slice. For simplicity, fetch limit from offset.
		// Actually, the DB method uses LIMIT only. We'll fetch all up to offset+pageSize
		// and take the last pageSize entries.
		entries, err := m.db.GetSwapHistory(m.walletAddr, offset+m.pageSize)
		if err != nil {
			return HistoryLoadedMsg{Err: err}
		}

		// Slice to current page
		if offset < len(entries) {
			end := offset + m.pageSize
			if end > len(entries) {
				end = len(entries)
			}
			entries = entries[offset:end]
		} else {
			entries = nil
		}

		return HistoryLoadedMsg{Entries: entries, Total: total}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case HistoryLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.entries = msg.Entries
		m.total = msg.Total
		m.cursor = 0
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "n":
			totalPages := m.totalPages()
			if m.page < totalPages-1 {
				m.page++
				m.loading = true
				m.spinner = components.NewSpinner("Loading history...")
				return m, tea.Batch(m.spinner.Init(), m.loadHistory())
			}
		case "p":
			if m.page > 0 {
				m.page--
				m.loading = true
				m.spinner = components.NewSpinner("Loading history...")
				return m, tea.Batch(m.spinner.Init(), m.loadHistory())
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

func (m Model) totalPages() int {
	if m.total == 0 {
		return 1
	}
	pages := m.total / m.pageSize
	if m.total%m.pageSize > 0 {
		pages++
	}
	return pages
}

// View renders the swap history.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Swap History")
	b.WriteString(title + "\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	if len(m.entries) == 0 {
		b.WriteString(tui.StyleMuted.Render("No swap history found.") + "\n")
	} else {
		header := fmt.Sprintf("%-12s %-20s %12s %-12s %-10s",
			"Date", "Trade", "Rate", "Status", "DEX")
		b.WriteString(tui.StyleTableHeader.Render(header) + "\n")

		for i, e := range m.entries {
			dateStr := FormatRelativeTime(e.CreatedAt)
			trade := fmt.Sprintf("%s -> %s", e.InputSymbol, e.OutputSymbol)
			if trade == " -> " {
				trade = fmt.Sprintf("%.4s -> %.4s", e.InputMint, e.OutputMint)
			}

			var rateStr string
			if e.InputAmount > 0 {
				rate := e.OutputAmount / e.InputAmount
				rateStr = fmt.Sprintf("%.6f", rate)
			} else {
				rateStr = "-"
			}

			statusStr := e.Status
			switch e.Status {
			case "finalized", "confirmed":
				statusStr = tui.StyleSuccess.Render(e.Status)
			case "failed":
				statusStr = tui.StyleError.Render(e.Status)
			}

			dex := e.Dex
			if dex == "" {
				dex = "-"
			}

			row := fmt.Sprintf("%-12s %-20s %12s %-12s %-10s",
				dateStr,
				truncate(trade, 20),
				rateStr,
				statusStr,
				truncate(dex, 10),
			)

			if i == m.cursor {
				b.WriteString(tui.StyleTableRowSelected.Render(row) + "\n")
			} else {
				b.WriteString(tui.StyleTableRow.Render(row) + "\n")
			}
		}
	}

	// Pagination
	b.WriteString("\n")
	totalPages := m.totalPages()
	b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("Page %d/%d (%d total)", m.page+1, totalPages, m.total)) + "\n")

	// Status bar
	b.WriteString(tui.StyleStatusBar.Render("[n]ext [p]rev [esc]back"))

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}

// FormatRelativeTime formats a unix timestamp as a relative time string.
func FormatRelativeTime(unixTimestamp int64) string {
	if unixTimestamp == 0 {
		return "-"
	}

	t := time.Unix(unixTimestamp, 0)
	now := time.Now()
	d := now.Sub(t)

	if d < 0 {
		return t.Format("Jan 02, 2006")
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes())

	switch {
	case minutes < 60:
		return fmt.Sprintf("%dm ago", minutes)
	case hours < 24:
		m := minutes - hours*60
		return fmt.Sprintf("%dh %dm ago", hours, m)
	case hours < 7*24:
		days := hours / 24
		h := hours - days*24
		return fmt.Sprintf("%dd %dh ago", days, h)
	default:
		return t.Format("Jan 02, 2006")
	}
}

// GetPage returns the current page for testing.
func (m Model) GetPage() int {
	return m.page
}

// GetCursor returns the current cursor position for testing.
func (m Model) GetCursor() int {
	return m.cursor
}

// GetEntries returns the loaded entries for testing.
func (m Model) GetEntries() []models.SwapHistoryEntry {
	return m.entries
}

// GetTotal returns the total entry count for testing.
func (m Model) GetTotal() int {
	return m.total
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
