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
	Items []services.ActivityItem
	Err   error
}

// Model is the activity history view.
type Model struct {
	items         []services.ActivityItem
	cursor        int
	walletAddr    string
	activitySvc   *services.ActivityService
	rpcClient     *blockchain.RPCClient
	lastSignature string // pagination cursor
	noMorePages   bool
	loading       bool
	spinner       components.SpinnerModel
	width         int
	height        int
	err           error
}

// New creates a new activity history view.
func New(walletAddr string, activitySvc *services.ActivityService, rpcClient *blockchain.RPCClient) Model {
	return Model{
		walletAddr:  walletAddr,
		activitySvc: activitySvc,
		rpcClient:   rpcClient,
		loading:     true,
		spinner:     components.NewSpinner("Loading history..."),
	}
}

const pageSize = 20

// Init starts loading history.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.loadActivity())
}

func (m Model) loadActivity() tea.Cmd {
	return func() tea.Msg {
		if m.activitySvc == nil || m.rpcClient == nil {
			return ActivityLoadedMsg{Err: fmt.Errorf("activity service not available")}
		}

		items, err := m.activitySvc.GetActivity(context.Background(), m.rpcClient, m.walletAddr, pageSize, m.lastSignature)
		if err != nil {
			return ActivityLoadedMsg{Err: err}
		}

		return ActivityLoadedMsg{Items: items}
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
			return m, nil
		}
		if len(msg.Items) == 0 {
			m.noMorePages = true
			return m, nil
		}
		isFirstLoad := len(m.items) == 0
		m.items = append(m.items, msg.Items...)
		if isFirstLoad {
			m.cursor = 0
		}
		m.lastSignature = m.items[len(m.items)-1].Signature
		if len(msg.Items) < pageSize {
			m.noMorePages = true
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
			// Load more (next page)
			if !m.noMorePages && m.lastSignature != "" {
				m.loading = true
				m.spinner = components.NewSpinner("Loading more...")
				return m, tea.Batch(m.spinner.Init(), m.loadActivity())
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

	if m.loading {
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
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
		{Key: "w", Action: "wallets"}, {Key: "s", Action: "send"},
		{Key: "x", Action: "swap"}, {Key: "t", Action: "tokens"},
	}
	if !m.noMorePages {
		nav = append(nav, tui.FooterBinding{Key: "n", Action: "next page"})
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
