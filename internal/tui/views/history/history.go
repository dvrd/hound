package history

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
)

// ActivityLoadedMsg is sent when activity history has been loaded.
type ActivityLoadedMsg struct {
	Result     services.ActivityResult
	Err        error
	TargetPage int // the page number this load was for
}

// Model is the activity history view.
type Model struct {
	items       []services.ActivityItem
	cursor      int
	walletAddr  string
	activitySvc *services.ActivityService
	rpcClient   *blockchain.RPCClient
	// Pagination: cursors[i] is the `before` value used to load page i+1.
	// cursors[0] = "" (first page), cursors[1] = LastSig of page 1, etc.
	cursors []string
	// pageCache stores loaded items per page (1-indexed) so going back never re-fetches.
	pageCache   map[int][]services.ActivityItem
	page        int // 1-based current page number
	noMorePages bool
	loading     bool
	spinner     components.SpinnerModel
	width       int
	height      int
	err         error
}

// New creates a new activity history view.
func New(walletAddr string, activitySvc *services.ActivityService, rpcClient *blockchain.RPCClient) Model {
	return Model{
		walletAddr:  walletAddr,
		activitySvc: activitySvc,
		rpcClient:   rpcClient,
		loading:     true,
		spinner:     components.NewSpinner("Loading history..."),
		cursors:     []string{""}, // cursors[0] = "" → page 1
		pageCache:   make(map[int][]services.ActivityItem),
		page:        1,
	}
}

const pageSize = 100

// Init starts loading history.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadPage("", 1))
}

// loadPage fetches one page using `before` as the cursor.
// targetPage is echoed back in ActivityLoadedMsg so Update can commit the page
// number only on success — preventing drift when the RPC call fails.
func (m Model) loadPage(before string, targetPage int) tea.Cmd {
	svc := m.activitySvc
	rpc := m.rpcClient
	addr := m.walletAddr
	return func() tea.Msg {
		if svc == nil || rpc == nil {
			return ActivityLoadedMsg{Err: fmt.Errorf("activity service not available"), TargetPage: targetPage}
		}
		result, err := svc.GetActivity(context.Background(), rpc, addr, pageSize, before)
		if err != nil {
			return ActivityLoadedMsg{Err: err, TargetPage: targetPage}
		}
		return ActivityLoadedMsg{Result: result, TargetPage: targetPage}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ActivityLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			// Don't touch m.page or m.items — keep existing data visible.
			return m, nil
		}
		// Commit the page number only on success.
		m.page = msg.TargetPage
		m.err = nil
		// Replace current page items (not append) and cache them.
		m.items = msg.Result.Items
		m.pageCache[m.page] = msg.Result.Items
		m.cursor = 0
		m.noMorePages = !msg.Result.HasMore
		// Push next-page cursor if we don't already have it.
		// cursors[page-1] = cursor used to reach this page.
		// cursors[page]   = cursor to use for the next page.
		if msg.Result.HasMore && len(m.cursors) <= m.page {
			m.cursors = append(m.cursors, msg.Result.LastSig)
		}
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "n":
			// Next page — only if more pages exist.
			// cursors[m.page] is the `before` cursor for page m.page+1.
			if !m.noMorePages && m.page < len(m.cursors) {
				targetPage := m.page + 1
				before := m.cursors[m.page]
				m.loading = true
				m.spinner = components.NewSpinner(fmt.Sprintf("Loading page %d...", targetPage))
				return m, tea.Batch(m.spinner.Init(), m.loadPage(before, targetPage))
			}
		case "p":
			// Previous page — serve from cache (no RPC call needed).
			if m.page > 1 {
				targetPage := m.page - 1
				if cached, ok := m.pageCache[targetPage]; ok {
					m.page = targetPage
					m.items = cached
					m.cursor = 0
					m.noMorePages = false // we know there's at least the page we came from
				}
			}
		case "s":
			addr := m.walletAddr
			return m, func() tea.Msg { return tui.NavigateMsg{View: "wallet-status", Data: addr} }
		case "w":
			return m, func() tea.Msg { return tui.NavigateMsg{View: "wallet-list"} }
		case "x":
			addr := m.walletAddr
			return m, func() tea.Msg { return tui.NavigateMsg{View: "swap", Data: addr} }
		case "t":
			return m, func() tea.Msg { return tui.NavigateMsg{View: "token-list"} }
		}
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// directionIcon returns the icon for a direction.
func directionIcon(direction, activityType string) string {
	switch activityType {
	case "swap":
		return "⇄"
	}
	switch direction {
	case "sent":
		return "↑"
	case "received":
		return "↓"
	default:
		return "·"
	}
}

// directionStyle returns the styled icon.
func directionStyledIcon(direction, activityType string) string {
	icon := directionIcon(direction, activityType)
	switch {
	case activityType == "swap":
		return tui.StyleTitle.Render(icon) // purple
	case direction == "sent":
		return tui.StyleError.Render(icon) // red
	case direction == "received":
		return tui.StyleSuccess.Render(icon) // green
	default:
		return tui.StyleMuted.Render(icon)
	}
}

// formatActivityLine returns the description for an activity item.
func formatActivityLine(item services.ActivityItem) string {
	switch item.Type {
	case "sol_transfer":
		if item.Direction == "sent" {
			return fmt.Sprintf("Sent %s", item.Amount)
		}
		return fmt.Sprintf("Received %s", item.Amount)
	case "spl_transfer":
		if item.Direction == "sent" {
			return fmt.Sprintf("Sent %s", item.Amount)
		}
		return fmt.Sprintf("Received %s", item.Amount)
	case "swap":
		return fmt.Sprintf("Swapped %s", item.Amount)
	case "program_interaction":
		return "Program interaction"
	default:
		return "Unknown transaction"
	}
}

// View renders the activity history.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("History")
	b.WriteString(title + "\n\n")

	// Initial load (no data yet): show spinner only.
	if m.loading && len(m.items) == 0 {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		// Fall through to show existing items below the error.
	}

	// Page indicator / loading status line.
	if m.loading {
		b.WriteString(m.spinner.View() + "\n\n")
	} else {
		pageInfo := fmt.Sprintf("Page %d", m.page)
		if m.noMorePages && m.page == 1 {
			pageInfo = ""
		}
		if pageInfo != "" {
			b.WriteString(tui.StyleMuted.Render(pageInfo) + "\n\n")
		}
	}

	if len(m.items) == 0 {
		b.WriteString(tui.StyleMuted.Render("No transaction history found.") + "\n")
	} else {
		// Proportional column widths
		w := m.width
		if w <= 0 {
			w = 80
		}
		colDesc := max(15, w*38/100)
		colTime := max(8, w*19/100)
		colStatus := max(8, w*15/100)

		// Cap visible rows to available height.
		// m.height is already the inner content height from the app shell —
		// no need to subtract chrome here. Reserve 3 rows for title + blank + status line.
		maxRows := len(m.items)
		if m.height > 0 {
			visible := m.height - 3
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
		if endIdx > len(m.items) {
			endIdx = len(m.items)
			startIdx = endIdx - maxRows
			if startIdx < 0 {
				startIdx = 0
			}
		}

		for i := startIdx; i < endIdx; i++ {
			item := m.items[i]
			icon := directionStyledIcon(item.Direction, item.Type)
			line := formatActivityLine(item)
			timeStr := FormatRelativeTime(item.Timestamp)

			var counterparty string
			if item.Counterparty != "" {
				if item.Direction == "sent" {
					counterparty = fmt.Sprintf(" → %s", item.Counterparty)
				} else {
					counterparty = fmt.Sprintf(" ← %s", item.Counterparty)
				}
			}

			statusStr := item.Status
			switch item.Status {
			case "confirmed":
				statusStr = tui.StyleSuccess.Render(item.Status)
			case "failed":
				statusStr = tui.StyleError.Render(item.Status)
			}

			rowFmt := fmt.Sprintf("%%s %%-%ds %%-%ds %%-%ds", colDesc, colTime, colStatus)
			row := fmt.Sprintf(rowFmt,
				icon,
				tui.Truncate(line+counterparty, colDesc),
				tui.StyleMuted.Render(timeStr),
				statusStr,
			)

			b.WriteString(tui.RenderRow(row, i == m.cursor) + "\n")
		}

		// Scroll indicator
		if len(m.items) > maxRows {
			hidden := len(m.items) - maxRows
			b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("  ↕ %d more", hidden)) + "\n")
		}
	}

	return b.String()
}

// Footer implements tui.FooterProvider — returns the pinned status bar text.
func (m Model) Footer() string {
	nav := tui.FooterGroup{
		{Key: "w", Action: "wallets"}, {Key: "s", Action: "status"},
		{Key: "x", Action: "swap"}, {Key: "t", Action: "tokens"},
	}
	return tui.RenderFooter(
		nav,
		tui.FooterGroup{{Key: "?", Action: "help"}},
	)
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

// GetCursor returns the current cursor position for testing.
func (m Model) GetCursor() int {
	return m.cursor
}

// GetItems returns the loaded items for testing.
func (m Model) GetItems() []services.ActivityItem {
	return m.items
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
