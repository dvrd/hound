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
	var b strings.Builder

	if m.loading {
		title := tui.StyleTitle.Render("Token Info")
		b.WriteString(title + "\n\n")
		b.WriteString(m.spinner.View() + "\n")
		return b.String()
	}

	if m.err != nil {
		title := tui.StyleTitle.Render("Token Info")
		b.WriteString(title + "\n\n")
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		b.WriteString("\n")
		return b.String()
	}

	info := m.info

	// Title: name + symbol
	symbolStr := ""
	if len(info.Symbols) > 0 {
		symbolStr = " (" + strings.Join(info.Symbols, "/") + ")"
	}
	title := tui.StyleTitle.Render(info.Name + symbolStr)
	b.WriteString(title + "\n")
	if info.MintAddress != "" {
		b.WriteString(tui.StyleMuted.Render(info.MintAddress) + "\n")
	}
	b.WriteString("\n")

	// Market data section
	b.WriteString(tui.StyleBold.Render("Market Data") + "\n")
	b.WriteString(fmt.Sprintf("  Price:       %s\n", wallet.FormatPrice(info.PriceUSD)))
	b.WriteString(fmt.Sprintf("  Market Cap:  %s\n", wallet.FormatLargeNumber(info.MarketCap)))
	b.WriteString(fmt.Sprintf("  FDV:         %s\n", wallet.FormatLargeNumber(info.FDV)))
	b.WriteString(fmt.Sprintf("  Liquidity:   %s\n", wallet.FormatLargeNumber(info.LiquidityUSD)))
	b.WriteString("\n")

	// Trading section
	b.WriteString(tui.StyleBold.Render("Trading (24h)") + "\n")
	b.WriteString(fmt.Sprintf("  Volume:      %s\n", wallet.FormatLargeNumber(info.Volume24h)))
	b.WriteString(fmt.Sprintf("  Txns:        %d\n", info.Txns24h))
	b.WriteString(fmt.Sprintf("  Buys/Sells:  %d / %d\n", info.Buys24h, info.Sells24h))
	b.WriteString("\n")

	// Price changes section
	b.WriteString(tui.StyleBold.Render("Price Changes") + "\n")
	b.WriteString(fmt.Sprintf("  5m:   %s\n", tui.FormatChange(info.PriceChange.M5)))
	b.WriteString(fmt.Sprintf("  1h:   %s\n", tui.FormatChange(info.PriceChange.H1)))
	b.WriteString(fmt.Sprintf("  6h:   %s\n", tui.FormatChange(info.PriceChange.H6)))
	b.WriteString(fmt.Sprintf("  24h:  %s\n", tui.FormatChange(info.PriceChange.H24)))
	b.WriteString("\n")

	// Top holders section
	if len(info.TopHolders) > 0 {
		b.WriteString(tui.StyleBold.Render("Top Holders") + "\n")
		for _, h := range info.TopHolders {
			addr := truncateAddress(h.Address)
			b.WriteString(fmt.Sprintf("  %s  %s  %.2f%%\n",
				addr,
				wallet.FormatBalance(h.Balance),
				h.OwnershipPct,
			))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Footer returns the pinned footer keybinding line for the App chrome.
func (m Model) Footer() string {
	return "[esc]back"
}

func truncateAddress(addr string) string {
	if len(addr) <= 11 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
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
