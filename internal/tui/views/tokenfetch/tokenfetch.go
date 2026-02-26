package tokenfetch

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// TokenInfoLoadedMsg is sent when extended token info has been fetched.
type TokenInfoLoadedMsg struct {
	Info models.TokenExtendedInfo
	Err  error
}

// Model is the token detail/fetch view.
type Model struct {
	mintOrSymbol string
	info         models.TokenExtendedInfo
	loading      bool
	spinner      components.SpinnerModel
	tokenInfoSvc *services.TokenInfoService
	db           *database.Database
	width        int
	height       int
	err          error
}

// New creates a new token fetch view.
func New(mintOrSymbol string, tokenInfoSvc *services.TokenInfoService, db *database.Database) Model {
	return Model{
		mintOrSymbol: mintOrSymbol,
		loading:      true,
		spinner:      components.NewSpinner("Fetching token info..."),
		tokenInfoSvc: tokenInfoSvc,
		db:           db,
	}
}

// Init starts fetching token info.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.fetchTokenInfo())
}

func (m Model) fetchTokenInfo() tea.Cmd {
	return func() tea.Msg {
		if m.tokenInfoSvc == nil {
			return TokenInfoLoadedMsg{Err: fmt.Errorf("token info service not available")}
		}
		info, err := m.tokenInfoSvc.FetchExtendedTokenInfo(m.mintOrSymbol, m.db)
		return TokenInfoLoadedMsg{Info: info, Err: err}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TokenInfoLoadedMsg:
		m.loading = false
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.info = msg.Info
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
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

// View renders the token detail view.
func (m Model) View() string {
	if m.loading {
		var b strings.Builder
		b.WriteString(tui.StyleTitle.Render("Token Info") + "\n\n")
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		var b strings.Builder
		b.WriteString(tui.StyleTitle.Render("Token Info") + "\n\n")
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
		return b.String()
	}

	// Build all lines into a slice so we can cap to terminal height.
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	info := m.info

	// Title: name + symbol
	symbolStr := ""
	if len(info.Symbols) > 0 {
		symbolStr = " (" + strings.Join(info.Symbols, "/") + ")"
	}
	add(tui.StyleTitle.Render(info.Name + symbolStr))
	if info.MintAddress != "" {
		add(tui.StyleMuted.Render(info.MintAddress))
	}
	add("")

	// Market data section
	add(tui.StyleBold.Render("Market Data"))
	add(fmt.Sprintf("  Price:       %s", wallet.FormatPrice(info.PriceUSD)))
	add(fmt.Sprintf("  Market Cap:  %s", wallet.FormatLargeNumber(info.MarketCap)))
	add(fmt.Sprintf("  FDV:         %s", wallet.FormatLargeNumber(info.FDV)))
	add(fmt.Sprintf("  Liquidity:   %s", wallet.FormatLargeNumber(info.LiquidityUSD)))
	add("")

	// Trading section
	add(tui.StyleBold.Render("Trading (24h)"))
	add(fmt.Sprintf("  Volume:      %s", wallet.FormatLargeNumber(info.Volume24h)))
	add(fmt.Sprintf("  Txns:        %d", info.Txns24h))
	add(fmt.Sprintf("  Buys/Sells:  %d / %d", info.Buys24h, info.Sells24h))
	add("")

	// Price changes section
	add(tui.StyleBold.Render("Price Changes"))
	if len(info.PriceHistory) > 0 {
		add("  " + tui.StyleBold.Render("Price (1h candles)"))
		add("  " + tui.Sparkline(info.PriceHistory, 24))
	} else if prices := tui.PricePathFromChanges(info.PriceUSD, info.PriceChange.M5, info.PriceChange.H1, info.PriceChange.H6, info.PriceChange.H24); prices != nil {
		add(fmt.Sprintf("  %s", tui.RenderSparkline(prices, 24)))
	}
	add(fmt.Sprintf("  5m:   %s", tui.FormatChange(info.PriceChange.M5)))
	add(fmt.Sprintf("  1h:   %s", tui.FormatChange(info.PriceChange.H1)))
	add(fmt.Sprintf("  6h:   %s", tui.FormatChange(info.PriceChange.H6)))
	add(fmt.Sprintf("  24h:  %s", tui.FormatChange(info.PriceChange.H24)))
	add("")

	// Top holders section
	if len(info.TopHolders) > 0 {
		add(tui.StyleBold.Render("Top Holders"))
		for _, h := range info.TopHolders {
			addr := tui.TruncateAddress(h.Address)
			add(fmt.Sprintf("  %s  %s  %.2f%%",
				addr,
				wallet.FormatBalance(h.Balance),
				h.OwnershipPct,
			))
		}
		add("")
	}

	// Cap to available terminal height (leave 4 lines for border + footer).
	maxLines := len(lines)
	if m.height > 4 {
		avail := m.height - 4
		if avail < maxLines {
			maxLines = avail
		}
	}

	return strings.Join(lines[:maxLines], "\n") + "\n"
}

// Footer returns the pinned footer keybinding line for the App chrome.
func (m Model) Footer() string {
	return tui.RenderFooter(
		tui.FooterGroup{
			{Key: "w", Action: "wallets"}, {Key: "t", Action: "tokens"},
		},
		tui.FooterGroup{{Key: "?", Action: "help"}},
	)
}

// GetInfo returns the loaded token info for testing.
func (m Model) GetInfo() models.TokenExtendedInfo {
	return m.info
}

// IsLoading returns whether the model is in loading state.
func (m Model) IsLoading() bool {
	return m.loading
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
