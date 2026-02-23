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
	if factory != nil {
		app.currentView = factory("wallet-list", nil)
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
		case "?":
			a.helpVisible = true
			return a, nil
		}
		// "q" to quit is handled by individual views (e.g., wallet list)
		// so it doesn't interfere with text input views

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

func (a App) navigate(msg NavigateMsg) (tea.Model, tea.Cmd) {
	if a.viewFactory == nil {
		return a, nil
	}

	newView := a.viewFactory(msg.View, msg.Data)
	if newView == nil {
		return a, nil
	}

	// Push current view to stack
	if a.currentView != nil {
		a.viewStack = append(a.viewStack, a.currentView)
	}
	a.currentView = newView

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

// innerHeight returns the usable content height after subtracting App chrome.
// StyleApp has Padding(1,2) = 2 rows + RoundedBorder = 2 rows = 4 total.
func (a App) innerHeight() int {
	h := a.height - 4
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

	// Overlay error bar at bottom
	if a.errorShown && a.errorMsg != "" {
		errStyle := lipgloss.NewStyle().
			Background(ColorError).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)
		content = content + "\n" + errStyle.Render(a.errorMsg)
	}

	// Overlay help
	if a.helpVisible {
		helpContent := renderHelp()
		content = content + "\n\n" + helpContent
	}

	// Apply app style with constraints
	style := StyleApp
	if a.width > 0 {
		style = style.Width(a.width)
	}
	if a.height > 0 {
		style = style.Height(a.height)
	}

	return style.Render(content)
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
