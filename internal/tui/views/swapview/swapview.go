package swapview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/swap"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
	"github.com/dvrd/hound/internal/wallet"
)

// SwapPhase represents the current phase of the swap flow.
type SwapPhase int

const (
	PhaseInput     SwapPhase = iota // 0
	PhaseQuoting                    // 1
	PhaseReview                     // 2
	PhasePassword                   // 3
	PhaseExecuting                  // 4
	PhaseResult                     // 5
)

// String returns the display name for a phase.
func (p SwapPhase) String() string {
	switch p {
	case PhaseInput:
		return "Input"
	case PhaseQuoting:
		return "Getting Quote"
	case PhaseReview:
		return "Review Quote"
	case PhasePassword:
		return "Password"
	case PhaseExecuting:
		return "Executing"
	case PhaseResult:
		return "Result"
	default:
		return "Unknown"
	}
}

// QuoteFetchedMsg is sent when a swap quote has been fetched.
type QuoteFetchedMsg struct {
	Quote models.SwapQuote
	Err   error
}

// SwapExecutedMsg is sent when a swap execution completes.
type SwapExecutedMsg struct {
	Result models.SwapTransactionResult
	Err    error
}

// Model is the swap view.
type Model struct {
	phase         SwapPhase
	inputMint     textinput.Model
	outputMint    textinput.Model
	amountInput   textinput.Model
	slippageInput textinput.Model
	passwordInput textinput.Model
	focusIndex    int
	quote         models.SwapQuote
	result        models.SwapTransactionResult
	dryRun        bool
	walletAddr    string
	spinner       components.SpinnerModel
	swapClient    *swap.SwapClient
	swapSvc       *services.SwapService
	width         int
	height        int
	err           error
}

// New creates a new swap view.
func New(walletAddr string, swapClient *swap.SwapClient, swapSvc *services.SwapService, dryRun bool) Model {
	im := textinput.New()
	im.Placeholder = "Input token mint (e.g. SOL mint)"
	im.CharLimit = 64
	im.Width = 50
	im.Focus()

	om := textinput.New()
	om.Placeholder = "Output token mint (e.g. USDC mint)"
	om.CharLimit = 64
	om.Width = 50

	ai := textinput.New()
	ai.Placeholder = "Amount (e.g. 1.5)"
	ai.CharLimit = 20
	ai.Width = 20

	// Default 50 bps = 0.5% slippage. 0 = use Jupiter's adaptive auto-slippage.
	si := textinput.New()
	si.Placeholder = "50"
	si.SetValue("50")
	si.CharLimit = 5
	si.Width = 8

	pi := textinput.New()
	pi.Placeholder = "Enter wallet password"
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 128
	pi.Width = 40

	return Model{
		phase:         PhaseInput,
		inputMint:     im,
		outputMint:    om,
		amountInput:   ai,
		slippageInput: si,
		passwordInput: pi,
		walletAddr:    walletAddr,
		swapClient:    swapClient,
		swapSvc:       swapSvc,
		dryRun:        dryRun,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case QuoteFetchedMsg:
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			m.phase = PhaseInput
			return m, nil
		}
		m.quote = msg.Quote
		m.phase = PhaseReview
		return m, nil

	case SwapExecutedMsg:
		m.spinner.SetDone()
		if msg.Err != nil {
			m.err = msg.Err
			m.result = msg.Result
		} else {
			m.result = msg.Result
		}
		m.phase = PhaseResult
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "esc" {
			switch m.phase {
			case PhaseInput:
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			case PhaseReview:
				m.phase = PhaseInput
				m.err = nil
				return m, nil
			case PhasePassword:
				m.phase = PhaseReview
				return m, nil
			case PhaseResult:
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			default:
				return m, nil
			}
		}

		switch m.phase {
		case PhaseInput:
			return m.updateInput(msg)
		case PhaseReview:
			return m.updateReview(msg)
		case PhasePassword:
			return m.updatePassword(msg)
		case PhaseResult:
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}

	if m.phase == PhaseQuoting || m.phase == PhaseExecuting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.focusIndex = (m.focusIndex + 1) % 4
		} else {
			m.focusIndex = (m.focusIndex + 3) % 4
		}
		m, cmd := m.focusCurrent()
		return m, cmd
	case "enter":
		inVal := strings.TrimSpace(m.inputMint.Value())
		outVal := strings.TrimSpace(m.outputMint.Value())
		amtVal := strings.TrimSpace(m.amountInput.Value())

		if inVal == "" || outVal == "" || amtVal == "" {
			m.err = fmt.Errorf("all fields are required")
			return m, nil
		}

		// Parse slippage — empty or invalid defaults to 0 (Jupiter auto-slippage).
		slippageBps := 0
		if slipStr := strings.TrimSpace(m.slippageInput.Value()); slipStr != "" {
			if v, err := strconv.Atoi(slipStr); err == nil && v >= 0 && v <= 10000 {
				slippageBps = v
			} else if err != nil || v < 0 || v > 10000 {
				m.err = fmt.Errorf("slippage must be 0–10000 bps (0–100%%)")
				return m, nil
			}
		}

		m.err = nil
		m.phase = PhaseQuoting
		m.spinner = components.NewSpinner("Fetching quote...")
		return m, tea.Batch(m.spinner.Init(), m.fetchQuote(inVal, outVal, amtVal, slippageBps))
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.inputMint, cmd = m.inputMint.Update(msg)
	case 1:
		m.outputMint, cmd = m.outputMint.Update(msg)
	case 2:
		m.amountInput, cmd = m.amountInput.Update(msg)
	case 3:
		m.slippageInput, cmd = m.slippageInput.Update(msg)
	}
	return m, cmd
}

func (m Model) focusCurrent() (Model, tea.Cmd) {
	m.inputMint.Blur()
	m.outputMint.Blur()
	m.amountInput.Blur()
	m.slippageInput.Blur()

	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		cmd = m.inputMint.Focus()
	case 1:
		cmd = m.outputMint.Focus()
	case 2:
		cmd = m.amountInput.Focus()
	case 3:
		cmd = m.slippageInput.Focus()
	}
	return m, cmd
}

func (m Model) fetchQuote(inputMint, outputMint, amount string, slippageBps int) tea.Cmd {
	return func() tea.Msg {
		if m.swapClient == nil {
			return QuoteFetchedMsg{Err: fmt.Errorf("swap client not available")}
		}
		quote, err := m.swapClient.GetQuote(inputMint, outputMint, amount, m.walletAddr, slippageBps)
		return QuoteFetchedMsg{Quote: quote, Err: err}
	}
}

func (m Model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.dryRun {
			m.result = models.SwapTransactionResult{
				Status: "dry-run",
			}
			m.phase = PhaseResult
			return m, nil
		}
		m.phase = PhasePassword
		m.passwordInput.Focus()
		return m, m.passwordInput.Focus()
	}
	return m, nil
}

func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		pw := m.passwordInput.Value()
		m.passwordInput.Reset() // H3: zero password buffer immediately after extraction
		if pw == "" {
			m.err = fmt.Errorf("password is required")
			return m, nil
		}
		m.err = nil
		m.phase = PhaseExecuting
		m.spinner = components.NewSpinner("Executing swap...")
		return m, tea.Batch(m.spinner.Init(), m.executeSwap(pw))
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) executeSwap(password string) tea.Cmd {
	return func() tea.Msg {
		if m.swapSvc == nil {
			return SwapExecutedMsg{Err: fmt.Errorf("swap service not available")}
		}
		result, err := m.swapSvc.ExecuteSwap(m.quote, m.walletAddr, password)
		return SwapExecutedMsg{Result: result, Err: err}
	}
}

// View renders the swap view.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Swap Tokens")
	b.WriteString(title + "\n")
	b.WriteString(tui.StyleSubtitle.Render(m.phase.String()) + "\n\n")

	if m.err != nil && m.phase != PhaseResult {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
	}

	switch m.phase {
	case PhaseInput:
		b.WriteString("Input Token:\n")
		b.WriteString(m.inputMint.View() + "\n\n")
		b.WriteString("Output Token:\n")
		b.WriteString(m.outputMint.View() + "\n\n")
		b.WriteString("Amount:\n")
		b.WriteString(m.amountInput.View() + "\n\n")
		b.WriteString("Slippage (bps, 0=auto):\n")
		b.WriteString(m.slippageInput.View() + "  ")
		if slipStr := strings.TrimSpace(m.slippageInput.Value()); slipStr != "" && slipStr != "0" {
			if v, err := strconv.Atoi(slipStr); err == nil {
				b.WriteString(tui.StyleMuted.Render(fmt.Sprintf("%.2f%%", float64(v)/100.0)))
			}
		} else {
			b.WriteString(tui.StyleMuted.Render("Jupiter auto"))
		}
		b.WriteString("\n")

	case PhaseQuoting:
		b.WriteString(m.spinner.View() + "\n")

	case PhaseReview:
		m.renderQuoteReview(&b)

	case PhasePassword:
		b.WriteString("Enter wallet password to sign transaction:\n\n")
		b.WriteString(m.passwordInput.View() + "\n")

	case PhaseExecuting:
		b.WriteString(m.spinner.View() + "\n")

	case PhaseResult:
		m.renderResult(&b)
	}

	return b.String()
}

func (m Model) renderQuoteReview(b *strings.Builder) {
	q := m.quote

	b.WriteString(tui.StyleBold.Render("Swap Quote") + "\n\n")

	inputLabel := q.InputMint
	if q.InputSymbol != "" {
		inputLabel = q.InputSymbol
	}
	outputLabel := q.OutputMint
	if q.OutputSymbol != "" {
		outputLabel = q.OutputSymbol
	}

	b.WriteString(fmt.Sprintf("  %s -> %s\n", inputLabel, outputLabel))
	b.WriteString(fmt.Sprintf("  In:           %s\n", q.InAmount))
	b.WriteString(fmt.Sprintf("  Out:          %s\n", q.OutAmount))

	if q.Rate > 0 {
		b.WriteString(fmt.Sprintf("  Rate:         %.6f\n", q.Rate))
	}

	if q.SlippageBps > 0 {
		b.WriteString(fmt.Sprintf("  Slippage:     %.2f%%\n", float64(q.SlippageBps)/100.0))
	}

	if q.MinReceived > 0 {
		b.WriteString(fmt.Sprintf("  Min Received: %s %s\n", wallet.FormatBalance(q.MinReceived), outputLabel))
	}

	// Price impact warnings
	if q.PriceImpactPct > 5 {
		b.WriteString(tui.StyleError.Render(fmt.Sprintf("  Price Impact: %.2f%% HIGH IMPACT", q.PriceImpactPct)) + "\n")
	} else if q.PriceImpactPct > 1 {
		b.WriteString(tui.StyleWarning.Render(fmt.Sprintf("  Price Impact: %.2f%%", q.PriceImpactPct)) + "\n")
	} else {
		b.WriteString(fmt.Sprintf("  Price Impact: %.2f%%\n", q.PriceImpactPct))
	}

	if q.NetworkFee > 0 {
		b.WriteString(fmt.Sprintf("  Network Fee:  %s SOL\n", wallet.FormatBalance(q.NetworkFee)))
	}

	b.WriteString(fmt.Sprintf("  Route:        %s\n", q.RouteLabel(inputLabel, outputLabel)))

	b.WriteString("\n")
	if m.dryRun {
		b.WriteString(tui.StyleWarning.Render("DRY RUN - no transaction will be executed") + "\n")
	}
}

func (m Model) renderResult(b *strings.Builder) {
	r := m.result

	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Swap Failed") + "\n\n")
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n")
		if r.ErrorMessage != "" {
			b.WriteString(tui.StyleMuted.Render("Details: "+r.ErrorMessage) + "\n")
		}
	} else if r.Status == "dry-run" {
		b.WriteString(tui.StyleWarning.Render("Dry Run Complete") + "\n\n")
		b.WriteString("Quote was valid. No transaction was executed.\n")
	} else {
		b.WriteString(tui.StyleSuccess.Render("Swap Successful!") + "\n\n")
		if r.Signature != "" {
			b.WriteString(fmt.Sprintf("  Signature: %s\n", r.Signature))
		}
		b.WriteString(fmt.Sprintf("  Status:    %s\n", r.Status))
		if r.Dex != "" {
			b.WriteString(fmt.Sprintf("  DEX:       %s\n", r.Dex))
		}
		inputLabel := m.quote.InputMint
		if m.quote.InputSymbol != "" {
			inputLabel = m.quote.InputSymbol
		}
		outputLabel := m.quote.OutputMint
		if m.quote.OutputSymbol != "" {
			outputLabel = m.quote.OutputSymbol
		}
		b.WriteString(fmt.Sprintf("  Route:     %s\n", m.quote.RouteLabel(inputLabel, outputLabel)))
	}

}

// Footer implements tui.FooterProvider — returns the pinned status bar text.
func (m Model) Footer() string {
	switch m.phase {
	case PhaseInput:
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "tab", Action: "next field"}, {Key: "enter", Action: "get quote"}},
			tui.FooterGroup{{Key: "esc", Action: "back"}, {Key: "ctrl+q", Action: "quit"}},
		)
	case PhaseReview:
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "enter", Action: "confirm"}},
			tui.FooterGroup{{Key: "esc", Action: "back"}, {Key: "ctrl+q", Action: "quit"}},
		)
	case PhasePassword:
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "enter", Action: "execute"}},
			tui.FooterGroup{{Key: "esc", Action: "back"}, {Key: "ctrl+q", Action: "quit"}},
		)
	case PhaseResult:
		return tui.RenderFooter(
			tui.FooterGroup{{Key: "any key", Action: "back"}},
		)
	default:
		return ""
	}
}

// GetPhase returns the current phase for testing.
func (m Model) GetPhase() SwapPhase {
	return m.phase
}

// GetFocusIndex returns the currently focused field index for testing.
func (m Model) GetFocusIndex() int {
	return m.focusIndex
}

// GetQuote returns the current quote for testing.
func (m Model) GetQuote() models.SwapQuote {
	return m.quote
}

// GetResult returns the swap result for testing.
func (m Model) GetResult() models.SwapTransactionResult {
	return m.result
}

// IsDryRun returns whether the swap is in dry-run mode.
func (m Model) IsDryRun() bool {
	return m.dryRun
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetSlippageValue sets the slippage input value directly (used in tests).
func (m *Model) SetSlippageValue(v string) {
	m.slippageInput.SetValue(v)
}

// resizeInputs adjusts text input widths to fit the available width.
func (m *Model) resizeInputs() {
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 10
	}
	m.inputMint.Width = min(50, maxW)
	m.outputMint.Width = min(50, maxW)
	m.amountInput.Width = min(20, maxW)
	m.slippageInput.Width = min(8, maxW)
	m.passwordInput.Width = min(40, maxW)
}
