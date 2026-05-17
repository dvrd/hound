package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dvrd/hound/internal/config"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/wallet"
)

// ViewFactory creates views by name. This allows the App to create views
// without importing the view packages directly (avoiding circular imports).
// The data parameter carries context (e.g., wallet address for "wallet-status").
type ViewFactory func(name string, data interface{}) tea.Model

// FooterProvider is implemented by views that want to render a pinned footer.
// The App extracts the footer and renders it at the bottom of the screen,
// separate from the scrollable content area.
type FooterProvider interface {
	Footer() string
}

// errorDismissMsg is sent when the error bar should auto-dismiss.
type errorDismissMsg struct{}

// notifDismissMsg is sent when the floating notification should auto-dismiss.
type notifDismissMsg struct{}

// viewTitles maps internal view keys to human-readable border titles.
var viewTitles = map[string]string{
	"wallet-list":   "Wallets",
	"wallet-status": "Portfolio",
	"history":       "History",
	"swap":          "Swap",
	"send":          "Send",
	"token-list":    "Tokens",
	"token-fetch":   "Token",
	"wallet-import": "Import",
	"wallet-delete": "Delete",
}

// App is the root Bubble Tea model that manages view navigation.
type App struct {
	currentView     tea.Model
	viewStack       []tea.Model // for back navigation
	viewNameStack   []string    // parallel stack tracking view keys for border title
	db              *database.Database
	walletMgr       *wallet.WalletManager
	keystoreSvc     *services.KeystoreService
	config          config.Config
	width           int
	height          int
	helpVisible     bool
	helpScroll      int // scroll offset for help overlay (lines from top)
	errorMsg        string
	errorShown      bool
	ready           bool
	contentStyle    lipgloss.Style // cached: Height(innerH).Width(innerW)
	appStyle        lipgloss.Style // cached: StyleApp.Width(w).Height(h+1)
	viewFactory     ViewFactory
	currentViewName string // tracks the active view key for border title (D)
	notifMsg        string // floating notification text (E)
	notifShown      bool   // true while notification is visible (E)
}

// NewApp creates a new root TUI application model.
// Pass an optional ViewFactory to enable view creation and navigation.
func NewApp(
	db *database.Database,
	walletMgr *wallet.WalletManager,
	keystoreSvc *services.KeystoreService,
	cfg config.Config,
	opts ...ViewFactory,
) App {
	var factory ViewFactory
	if len(opts) > 0 {
		factory = opts[0]
	}

	app := App{
		db:          db,
		walletMgr:   walletMgr,
		keystoreSvc: keystoreSvc,
		config:      cfg,
		viewFactory: factory,
	}

	// Create initial view eagerly so it's available before Init() is called.
	// Resolution order:
	//   1. Restore last view from app_state (persisted on previous exit).
	//   2. Skip wallet-list when there is exactly one wallet — go straight to wallet-status.
	//   3. Fall back to wallet-list.
	if factory != nil {
		initialView := "wallet-list"
		var initialData interface{}

		if db != nil {
			// Attempt to restore persisted state.
			lastView, _ := db.GetAppState("last_view")
			lastAddr, _ := db.GetAppState("last_addr")

			// Only restore wallet-status — it's the only view that makes sense
			// as a standalone entry point with an empty stack.
			// Everything else (swap, history, send, token-list) needs a parent
			// on the stack to go back to — restoring them strands the user.
			switch lastView {
			case "wallet-status":
				initialView = lastView
				if lastAddr != "" {
					initialData = lastAddr
				}
			default:
				// No persisted state or unrestorable view — use single-wallet shortcut.
				if wallets, err := db.GetAllWallets(); err == nil && len(wallets) == 1 {
					initialView = "wallet-status"
					initialData = wallets[0].Address
				}
			}
		}

		app.currentView = factory(initialView, initialData)
		app.currentViewName = initialView
	}

	return app
}

// Init initializes the app by starting the current view.
func (a App) Init() tea.Cmd {
	if a.currentView != nil {
		return a.currentView.Init()
	}
	return nil
}

// Update handles messages and routes them appropriately.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.contentStyle = lipgloss.NewStyle().Height(a.innerHeight()).Width(a.innerWidth())
		a.appStyle = StyleApp
		if a.width > 0 {
			a.appStyle = a.appStyle.Width(a.innerWidth())
		}
		if a.height > 0 {
			a.appStyle = a.appStyle.Height(a.innerHeight() + 1)
		}
		// Force full repaint so the alternate screen redraws correctly
		// when the terminal grows (e.g. dragging the top edge up).
		cmds = append(cmds, tea.ClearScreen)
		if a.currentView != nil {
			var cmd tea.Cmd
			a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
				Width:  a.innerWidth(),
				Height: a.innerHeight(),
			})
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Help overlay consumes all keys when visible.
		if a.helpVisible {
			switch msg.String() {
			case "?", "esc", "q":
				a.helpVisible = false
				a.helpScroll = 0
			case "j", "down":
				a.helpScroll++
				if max := helpMaxScroll(a.currentViewName, a.height); a.helpScroll > max {
					a.helpScroll = max
				}
			case "k", "up":
				if a.helpScroll > 0 {
					a.helpScroll--
				}
			}
			return a, nil
		}

		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			// Save current view before quitting so next launch restores here.
			if ap, ok := a.currentView.(interface{ WalletAddress() string }); ok {
				a.saveState("wallet-status", ap.WalletAddress())
			}
			return a, tea.Quit
		case "?":
			a.helpVisible = true
			a.helpScroll = 0
			return a, nil
		case "w":
			// Global wallets — accessible from any view.
			return a.navigate(NavigateMsg{View: "wallet-list"})
		}
		// Fall through to current view for all other keys

	case NavigateMsg:
		return a.navigate(msg)

	case NavigateBackMsg:
		return a.navigateBack()

	case ErrorMsg:
		a.errorMsg = models.UserMessage(msg.Err)
		a.errorShown = true
		cmd := tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return errorDismissMsg{}
		})
		return a, cmd

	case errorDismissMsg:
		a.errorShown = false
		a.errorMsg = ""
		return a, nil

	case NotifMsg:
		a.notifMsg = msg.Text
		a.notifShown = true
		cmd := tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return notifDismissMsg{}
		})
		return a, cmd

	case notifDismissMsg:
		a.notifShown = false
		a.notifMsg = ""
		return a, nil
	}

	// Delegate to current view
	if a.currentView != nil {
		var cmd tea.Cmd
		a.currentView, cmd = a.currentView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// rootViews are views that act as navigation roots — pressing q from them quits.
// When navigating to a root view, the stack is cleared rather than pushed.
var rootViews = map[string]bool{
	"wallet-list":   true,
	"wallet-status": true,
}

func (a App) navigate(msg NavigateMsg) (tea.Model, tea.Cmd) {
	if a.viewFactory == nil {
		return a, nil
	}

	newView := a.viewFactory(msg.View, msg.Data)
	if newView == nil {
		return a, nil
	}

	// Root views (wallet-list, wallet-status) clear the stack so that q always
	// quits from them, regardless of how deep the navigation history is.
	// All other views push the current view onto the stack for back navigation.
	if rootViews[msg.View] {
		a.viewStack = nil
		a.viewNameStack = nil
	} else if a.currentView != nil {
		a.viewStack = append(a.viewStack, a.currentView)
		a.viewNameStack = append(a.viewNameStack, a.currentViewName)
	}
	a.currentView = newView
	a.currentViewName = msg.View

	// Persist the new view so next launch restores here.
	// Only save wallet-status — the only view that makes sense as a standalone
	// entry point. history/swap/send/token-list need a parent on the stack.
	addr, _ := msg.Data.(string)
	if msg.View == "wallet-status" {
		a.saveState(msg.View, addr)
	}

	// Pass size to new view
	var cmd tea.Cmd
	a.currentView, cmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.innerWidth(), Height: a.innerHeight(),
	})

	initCmd := a.currentView.Init()
	return a, tea.Batch(cmd, initCmd)
}

func (a App) navigateBack() (tea.Model, tea.Cmd) {
	if len(a.viewStack) == 0 {
		return a, tea.Quit
	}

	// Pop from stack
	last := len(a.viewStack) - 1
	a.currentView = a.viewStack[last]
	a.viewStack = a.viewStack[:last]

	// Restore view name for border title
	if len(a.viewNameStack) > 0 {
		nameLast := len(a.viewNameStack) - 1
		a.currentViewName = a.viewNameStack[nameLast]
		a.viewNameStack = a.viewNameStack[:nameLast]
	} else {
		a.currentViewName = ""
	}

	// Pass current size
	var sizeCmd tea.Cmd
	a.currentView, sizeCmd = a.currentView.Update(tea.WindowSizeMsg{
		Width: a.innerWidth(), Height: a.innerHeight(),
	})

	// Re-init the view so it refreshes its data
	initCmd := a.currentView.Init()

	return a, tea.Batch(sizeCmd, initCmd)
}

// innerWidth returns the usable content width after subtracting App chrome.
// StyleApp has Padding(1,2) and RoundedBorder. In lipgloss, .Width() sets the
// content width and padding is included inside that budget — only the border
// (1 col left + 1 col right = 2 cols) is added on top. So we subtract 2.
func (a App) innerWidth() int {
	w := a.width - 2
	if w < 20 {
		return 20
	}
	return w
}

// innerHeight returns the usable content height after subtracting App chrome
// and the 1-row pinned footer.
// StyleApp has Padding(1,2) = 2 rows + RoundedBorder = 2 rows = 4 total.
// An additional 1 row is reserved for the footer bar.
func (a App) innerHeight() int {
	h := a.height - 4 - 1 // chrome (4) + footer (1)
	if h < 5 {
		return 5
	}
	return h
}

// helpLineCount returns the total number of content lines the help overlay
// would render for the given view name. This mirrors the allLines construction
// in renderHelp so that Update() can clamp helpScroll without calling renderHelp.
func helpLineCount(viewName string) int {
	global := 4 // ?/q/esc, j/k scroll, ctrl+q, w
	viewExtra := map[string]int{
		"wallet-status": 1 + 13, // section header + 13 bindings
		"wallet-list":   1 + 9,  // section header + 9 bindings
		"history":       1 + 4,  // section header + 4 bindings
		"token-list":    1 + 4,  // section header + 4 bindings
		"token-fetch":   1 + 2,  // section header + 2 bindings
	}
	return global + viewExtra[viewName]
}

// helpMaxScroll returns the maximum valid helpScroll value for the given
// terminal height and view name. Returns 0 when all lines fit on screen.
func helpMaxScroll(viewName string, termH int) int {
	const chrome = 6
	maxModalH := termH - 4
	if maxModalH < chrome+1 {
		maxModalH = chrome + 1
	}
	visibleRows := maxModalH - chrome
	total := helpLineCount(viewName)
	if visibleRows > total {
		visibleRows = total
	}
	max := total - visibleRows
	if max < 0 {
		return 0
	}
	return max
}

// View renders the app.
func (a App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	var content string
	if a.currentView != nil {
		content = a.currentView.View()
	} else {
		content = "Hound TUI — Coming soon. Press q to quit."
	}

	// Extract footer from view if it implements FooterProvider.
	// The footer is rendered pinned to the bottom of the content area,
	// separate from the scrollable content.
	var footer string
	if fp, ok := a.currentView.(FooterProvider); ok {
		footer = fp.Footer()
	}

	// Error bar overrides the footer when shown.
	if a.errorShown && a.errorMsg != "" {
		footer = errBarStyle.Width(a.innerWidth()).Render(a.errorMsg)
	}

	// App style is pre-computed on WindowSizeMsg.
	style := a.appStyle

	// Build inner layout: scrollable content area + pinned footer.
	// No Width on footerStyle: the footer is already pre-rendered with ANSI
	// color codes by RenderFooter. Applying Width to a pre-styled string causes
	// lipgloss to pad it to the full inner width, which pushes the box beyond
	// the terminal width and clips the right edge of the footer.
	footerStyle := footerBarStyle

	inner := lipgloss.JoinVertical(lipgloss.Left,
		a.contentStyle.Render(content),
		footerStyle.Render(footer),
	)

	rendered := style.Render(inner)

	// D: Splice view name into the top border line.
	if title, ok := viewTitles[a.currentViewName]; ok && title != "" {
		rendered = injectBorderTitle(rendered, " "+title+" ")
	}

	// Help overlay — rendered on top of everything using lipgloss.Place so it
	// doesn't push any content. The modal is capped to terminal dimensions.
	if a.helpVisible {
		helpBox := renderHelp(a.currentViewName, a.width, a.height, a.helpScroll)
		rendered = lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			helpBox,
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	// E: Floating notification — rendered below the box, auto-dismisses after 3s.
	if a.notifShown && a.notifMsg != "" {
		rendered += "\n" + notifBarStyle.Render(a.notifMsg)
	}

	return rendered
}

// injectBorderTitle splices a title string into the top border of a lipgloss
// rounded-border box. It finds the first line (which starts with "╭") and
// replaces the characters immediately after "╭" with the title, preserving
// the rest of the border line.
//
// This is a workaround for lipgloss v1.x not supporting BorderTitle natively.
func injectBorderTitle(rendered, title string) string {
	lines := strings.SplitN(rendered, "\n", 2)
	if len(lines) < 2 {
		return rendered
	}
	topLine := lines[0]
	// Find the opening corner rune (╭ is 3 bytes in UTF-8).
	cornerIdx := strings.Index(topLine, "╭")
	if cornerIdx < 0 {
		return rendered
	}
	afterCorner := cornerIdx + len("╭")
	if afterCorner >= len(topLine) {
		return rendered
	}
	// Count visible runes in title to know how many border chars to skip.
	titleRunes := []rune(title)
	titleLen := len(titleRunes)
	// Walk titleLen runes forward from afterCorner in the border line.
	restRunes := []rune(topLine[afterCorner:])
	if titleLen >= len(restRunes) {
		return rendered // title too long — don't mangle
	}
	newTop := topLine[:afterCorner] + title + string(restRunes[titleLen:])
	return newTop + "\n" + lines[1]
}

// renderHelp renders a view-aware help overlay that is responsive to the
// available terminal dimensions and supports scrolling with j/k.
//
// Layout budget:
//   - modal width  = min(50, termW-4)   — 2 cols margin each side
//   - modal height = min(allLines+4, termH-4) — 2 rows margin top+bottom
//   - border(2) + padding(2) = 4 rows of chrome consumed inside the box
//   - visible content rows = modalHeight - 4
// Reusable styles — hoisted to avoid per-frame allocation.
var (
	helpKeyStyle       = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Width(12)
	helpDescStyle      = lipgloss.NewStyle().Foreground(ColorText)
	helpSectionStyle   = lipgloss.NewStyle().Foreground(ColorSubtext).Bold(true)
	helpIndicatorStyle = lipgloss.NewStyle().Foreground(ColorSubtext)
	helpTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	errBarStyle        = lipgloss.NewStyle().Background(ColorError).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1)
	notifBarStyle      = lipgloss.NewStyle().Foreground(ColorSuccess).PaddingLeft(2)
	footerBarStyle     = lipgloss.NewStyle().Foreground(ColorSubtext)
)

func renderHelp(viewName string, termW, termH, scroll int) string {
	keyStyle := helpKeyStyle
	descStyle := helpDescStyle
	sectionStyle := helpSectionStyle

	// Global bindings always shown.
	global := []struct{ key, desc string }{
		{"?/q/esc", "Close help"},
		{"j/k", "Scroll help"},
		{"ctrl+q", "Quit"},
		{"w", "Go to wallets"},
	}

	// Per-view bindings.
	viewBindings := map[string][]struct{ key, desc string }{
		"wallet-status": {
			{"j/k ↑/↓", "Navigate tokens"},
			{"enter", "Token detail"},
			{"s", "Send"},
			{"x", "Swap"},
			{"h", "History"},
			{"t", "Token list"},
			{"r", "Refresh"},
			{"R", "Rename wallet"},
			{"c", "Copy address"},
			{"a", "Toggle dust filter"},
			{"1/2/3", "Sort by value/symbol/balance"},
			{"<", "Hide token"},
			{">", "Unhide token"},
		},
		"wallet-list": {
			{"j/k ↑/↓", "Navigate wallets"},
			{"s/enter", "Open wallet"},
			{"S", "Send"},
			{"x", "Swap"},
			{"h", "History"},
			{"t", "Token list"},
			{"i", "Import wallet"},
			{"d", "Delete wallet"},
			{"r", "Refresh all"},
		},
		"history": {
			{"j/k ↑/↓", "Navigate"},
			{"n", "Next page"},
			{"p", "Previous page"},
			{"esc", "Back"},
		},
		"token-list": {
			{"↑/↓ ctrl+n/p", "Navigate results"},
			{"enter", "Open token"},
			{"esc", "Clear search / back"},
			{"type anything", "Search tokens"},
		},
		"token-fetch": {
			{"esc", "Back"},
			{"t", "Token list"},
		},
	}

	// Build all content lines (unstyled for slicing, styled when rendering).
	type line struct {
		key, desc string
		isSection bool
	}
	var allLines []line

	for _, b := range global {
		allLines = append(allLines, line{key: b.key, desc: b.desc})
	}
	if vb, ok := viewBindings[viewName]; ok {
		allLines = append(allLines, line{isSection: true, desc: "This view"})
		for _, b := range vb {
			allLines = append(allLines, line{key: b.key, desc: b.desc})
		}
	}

	// Determine modal dimensions.
	modalW := termW - 4
	if modalW > 52 {
		modalW = 52
	}
	if modalW < 30 {
		modalW = 30
	}

	// chrome: border top+bottom (2) + padding top+bottom (2) + title row (1) + blank (1) = 6
	const chrome = 6
	maxModalH := termH - 4
	if maxModalH < chrome+1 {
		maxModalH = chrome + 1
	}
	visibleRows := maxModalH - chrome
	if visibleRows > len(allLines) {
		visibleRows = len(allLines)
	}

	// Clamp scroll so we never go past the last page.
	maxScroll := len(allLines) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	// Slice the visible window.
	end := scroll + visibleRows
	if end > len(allLines) {
		end = len(allLines)
	}
	visible := allLines[scroll:end]

	// Render visible lines.
	var rowsBuilder strings.Builder
	for _, l := range visible {
		if l.isSection {
			rowsBuilder.WriteString(sectionStyle.Render(l.desc))
			rowsBuilder.WriteByte('\n')
		} else {
			rowsBuilder.WriteString(keyStyle.Render(l.key))
			rowsBuilder.WriteString(descStyle.Render(l.desc))
			rowsBuilder.WriteByte('\n')
		}
	}
	rows := rowsBuilder.String()

	// Scroll indicators.
	indicatorStyle := helpIndicatorStyle
	scrollBar := ""
	if scroll > 0 && scroll < maxScroll {
		scrollBar = indicatorStyle.Render("↑↓ more")
	} else if scroll > 0 {
		scrollBar = indicatorStyle.Render("↑ more above")
	} else if scroll < maxScroll {
		scrollBar = indicatorStyle.Render("↓ more below")
	}
	if scrollBar != "" {
		rows += scrollBar + "\n"
	}

	titleStyle := helpTitleStyle

	contentW := modalW - 6 // border(2) + padding(4)
	if contentW < 10 {
		contentW = 10
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Width(contentW)

	title := titleStyle.Render("Keyboard Shortcuts")
	return box.Render(title + "\n\n" + rows)
}

// ViewStackDepth returns the number of views on the navigation stack.
func (a App) ViewStackDepth() int {
	return len(a.viewStack)
}

// IsHelpVisible returns whether the help overlay is shown.
func (a App) IsHelpVisible() bool {
	return a.helpVisible
}

// IsErrorVisible returns whether the error bar is shown.
func (a App) IsErrorVisible() bool {
	return a.errorShown
}

// GetCurrentView returns the current view model for testing.
func (a App) GetCurrentView() tea.Model {
	return a.currentView
}

// saveState persists the current view name and wallet address to the DB.
// Errors are silently ignored — state persistence is best-effort.
func (a App) saveState(view, addr string) {
	if a.db == nil {
		return
	}
	_ = a.db.SetAppState("last_view", view)  // best-effort: state persistence is non-critical
	_ = a.db.SetAppState("last_addr", addr)   // best-effort: state persistence is non-critical
}
