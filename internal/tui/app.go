package tui

import (
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

// App is the root Bubble Tea model that manages view navigation.
type App struct {
	currentView tea.Model
	viewStack   []tea.Model // for back navigation
	db          *database.Database
	walletMgr   *wallet.WalletManager
	keystoreSvc *services.KeystoreService
	config      config.Config
	width       int
	height      int
	helpVisible bool
	errorMsg    string
	errorShown  bool
	ready       bool
	viewFactory ViewFactory
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

			// Only restore views that make sense as entry points.
			// Transient views (wallet-import, wallet-delete, etc.) fall back to wallet-status.
			switch lastView {
			case "wallet-status", "menu", "swap", "history", "token-list", "send":
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
		// Help overlay consumes keys when visible
		if a.helpVisible {
			if msg.String() == "?" || msg.String() == "esc" {
				a.helpVisible = false
			}
			return a, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			// Global quit — only when at the root view (stack empty).
			// When navigated into a sub-view (send, swap, import, etc.) the user
			// must use esc to go back; q is passed to the view as a normal key.
			if len(a.viewStack) == 0 {
				// Save current view before quitting so next launch restores here.
				if ap, ok := a.currentView.(interface{ WalletAddress() string }); ok {
					a.saveState("wallet-status", ap.WalletAddress())
				}
				return a, tea.Quit
			}
		case "?":
			a.helpVisible = true
			return a, nil
		case "m":
			// Global menu — accessible from any view.
			addr := ""
			if ap, ok := a.currentView.(interface{ WalletAddress() string }); ok {
				addr = ap.WalletAddress()
			}
			return a.navigate(NavigateMsg{View: "menu", Data: addr})
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
	} else if a.currentView != nil {
		a.viewStack = append(a.viewStack, a.currentView)
	}
	a.currentView = newView

	// Persist the new view so next launch restores here.
	addr, _ := msg.Data.(string)
	a.saveState(msg.View, addr)

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
// StyleApp has Padding(1,2) = 4 cols + RoundedBorder = 2 cols = 6 total.
func (a App) innerWidth() int {
	w := a.width - 6
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

	// Overlay help
	if a.helpVisible {
		helpContent := renderHelp()
		content = content + "\n\n" + helpContent
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
		errStyle := lipgloss.NewStyle().
			Background(ColorError).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Width(a.innerWidth())
		footer = errStyle.Render(a.errorMsg)
	}

	// Apply app style with constraints.
	// innerHeight() already reserves 1 row for the footer.
	style := StyleApp
	w := a.innerWidth()
	if a.width > 0 {
		style = style.Width(w)
	}
	if a.height > 0 {
		// +1 because StyleApp wraps content+footer together; footer row is inside the border.
		style = style.Height(a.innerHeight() + 1)
	}

	// Build inner layout: scrollable content area + pinned footer.
	footerStyle := lipgloss.NewStyle().
		Foreground(ColorSubtext).
		Width(w)

	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Height(a.innerHeight()).Width(w).Render(content),
		footerStyle.Render(footer),
	)

	return style.Render(inner)
}

// renderHelp renders a simple help overlay.
func renderHelp() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Width(8)
	descStyle := lipgloss.NewStyle().
		Foreground(ColorText)

	bindings := []struct{ key, desc string }{
		{"?", "Toggle help"},
		{"q", "Quit"},
		{"esc", "Go back"},
	}

	var rows string
	for _, b := range bindings {
		rows += keyStyle.Render(b.key) + descStyle.Render(b.desc) + "\n"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)

	return box.Render(titleStyle.Render("Keyboard Shortcuts") + "\n" + rows)
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
	_ = a.db.SetAppState("last_view", view)
	_ = a.db.SetAppState("last_addr", addr)
}
