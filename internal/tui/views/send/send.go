package send

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
)

// Step represents the current step in the send wizard.
type Step int

const (
	StepSelectToken Step = iota // 0 — cursor selection of SOL or SPL token
	StepRecipient               // 1 — text input for recipient address
	StepAmount                  // 2 — text input for amount
	StepReview                  // 3 — summary, enter to confirm
	StepPassword                // 4 — masked password input
	StepSending                 // 5 — spinner + async transfer
	StepResult                  // 6 — success/error display
)

const totalSteps = 6

// StepName returns the display name for a step.
func (s Step) Name() string {
	switch s {
	case StepSelectToken:
		return "Select Token"
	case StepRecipient:
		return "Recipient"
	case StepAmount:
		return "Amount"
	case StepReview:
		return "Review"
	case StepPassword:
		return "Password"
	case StepSending:
		return "Sending"
	case StepResult:
		return "Result"
	default:
		return "Unknown"
	}
}

// Model is the send wizard view.
type Model struct {
	step           Step
	tokens         []models.TokenBalance // from portfolio
	tokenCursor    int
	selectedToken  models.TokenBalance
	recipientInput textinput.Model
	amountInput    textinput.Model
	passwordInput  textinput.Model
	recipient      string
	amount         uint64 // in smallest unit (lamports or token base units)
	amountDisplay  string // human-readable
	isSOL          bool
	createATA      bool // whether recipient ATA needs creation
	estimatedFee   uint64
	signature      string
	spinner        components.SpinnerModel
	err            error

	// Dependencies
	walletAddr  string
	transferSvc *services.TransferService
	rpcClient   *blockchain.RPCClient
	portfolio   models.PortfolioBalance
	width       int
	height      int
}

// New creates a new send wizard.
func New(walletAddr string, transferSvc *services.TransferService, rpcClient *blockchain.RPCClient, portfolio models.PortfolioBalance) Model {
	// Build token list: SOL first, then SPL tokens with balance > 0
	var tokens []models.TokenBalance
	tokens = append(tokens, portfolio.SOLBalance)
	for _, t := range portfolio.TokenBalances {
		if t.Amount > 0 {
			tokens = append(tokens, t)
		}
	}

	// Recipient input
	ri := textinput.New()
	ri.Placeholder = "Enter recipient address"
	ri.CharLimit = 50
	ri.Width = 50

	// Amount input
	ai := textinput.New()
	ai.Placeholder = "Enter amount (or MAX)"
	ai.CharLimit = 30
	ai.Width = 30

	// Password input
	pi := textinput.New()
	pi.Placeholder = "Enter wallet password"
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 128
	pi.Width = 40

	return Model{
		step:           StepSelectToken,
		tokens:         tokens,
		recipientInput: ri,
		amountInput:    ai,
		passwordInput:  pi,
		spinner:        components.NewSpinner("Sending transaction..."),
		walletAddr:     walletAddr,
		transferSvc:    transferSvc,
		rpcClient:      rpcClient,
		portfolio:      portfolio,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tui.TransferSentMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.step = StepResult
			return m, nil
		}
		m.signature = msg.Signature
		m.step = StepResult
		return m, nil

	case tea.KeyMsg:
		// Global escape handling
		if msg.String() == "esc" {
			if m.step == StepSelectToken {
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			}
			if m.step > StepSelectToken && m.step < StepSending {
				m.err = nil
				m.step--
				return m, m.focusCurrentStep()
			}
			return m, nil
		}

		switch m.step {
		case StepSelectToken:
			return m.updateSelectToken(msg)
		case StepRecipient:
			return m.updateRecipient(msg)
		case StepAmount:
			return m.updateAmount(msg)
		case StepReview:
			return m.updateReview(msg)
		case StepPassword:
			return m.updatePassword(msg)
		case StepResult:
			// Any key goes back
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}

	// Update spinner during sending
	if m.step == StepSending {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) focusCurrentStep() tea.Cmd {
	switch m.step {
	case StepRecipient:
		return m.recipientInput.Focus()
	case StepAmount:
		return m.amountInput.Focus()
	case StepPassword:
		return m.passwordInput.Focus()
	default:
		return nil
	}
}

func (m Model) updateSelectToken(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.tokenCursor > 0 {
			m.tokenCursor--
		}
	case "down", "j":
		if m.tokenCursor < len(m.tokens)-1 {
			m.tokenCursor++
		}
	case "enter":
		if len(m.tokens) == 0 {
			m.err = fmt.Errorf("no tokens available to send")
			return m, nil
		}
		m.selectedToken = m.tokens[m.tokenCursor]
		m.isSOL = m.selectedToken.Symbol == "SOL"
		m.err = nil
		m.step = StepRecipient
		m.recipientInput.Focus()
		return m, m.recipientInput.Focus()
	}
	return m, nil
}

func (m Model) updateRecipient(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		addr := strings.TrimSpace(m.recipientInput.Value())
		// Validate: not empty
		if addr == "" {
			m.err = fmt.Errorf("recipient address cannot be empty")
			return m, nil
		}
		// Validate: base58 length (32-44 chars)
		if len(addr) < 32 || len(addr) > 44 {
			m.err = fmt.Errorf("invalid address: must be 32-44 characters")
			return m, nil
		}
		// Validate: not self
		if addr == m.walletAddr {
			m.err = fmt.Errorf("cannot send to your own address")
			return m, nil
		}
		m.recipient = addr
		m.err = nil
		m.step = StepAmount
		m.amountInput.Focus()
		return m, m.amountInput.Focus()
	}

	var cmd tea.Cmd
	m.recipientInput, cmd = m.recipientInput.Update(msg)
	return m, cmd
}

func (m Model) updateAmount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		input := strings.TrimSpace(m.amountInput.Value())
		if input == "" {
			m.err = fmt.Errorf("amount cannot be empty")
			return m, nil
		}

		// Estimate fee
		m.estimatedFee = m.estimateFee()

		// Handle MAX
		if strings.EqualFold(input, "MAX") {
			return m.handleMaxAmount()
		}

		// Parse amount
		var amountFloat float64
		_, err := fmt.Sscanf(input, "%f", &amountFloat)
		if err != nil || amountFloat <= 0 {
			m.err = fmt.Errorf("amount must be a positive number")
			return m, nil
		}

		// Convert to base units
		decimals := m.selectedToken.Decimals
		baseUnits := uint64(amountFloat * math.Pow10(decimals))

		// Check balance
		maxBalance := m.maxSendable()
		if baseUnits > maxBalance {
			m.err = fmt.Errorf("insufficient balance (max: %s)", m.formatAmount(maxBalance))
			return m, nil
		}

		m.amount = baseUnits
		m.amountDisplay = input
		m.err = nil
		m.step = StepReview
		return m, nil
	}

	var cmd tea.Cmd
	m.amountInput, cmd = m.amountInput.Update(msg)
	return m, cmd
}

func (m Model) handleMaxAmount() (tea.Model, tea.Cmd) {
	maxBalance := m.maxSendable()
	if maxBalance == 0 {
		m.err = fmt.Errorf("insufficient balance after fees")
		return m, nil
	}
	m.amount = maxBalance
	m.amountDisplay = m.formatAmount(maxBalance)
	m.err = nil
	m.step = StepReview
	return m, nil
}

func (m Model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		m.step = StepPassword
		m.passwordInput.Focus()
		return m, m.passwordInput.Focus()
	}
	return m, nil
}

func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		password := m.passwordInput.Value()
		if password == "" {
			m.err = fmt.Errorf("password cannot be empty")
			return m, nil
		}
		m.err = nil
		m.step = StepSending
		return m, tea.Batch(m.spinner.Init(), m.doTransfer(password))
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) doTransfer(password string) tea.Cmd {
	return func() tea.Msg {
		if m.transferSvc == nil || m.rpcClient == nil {
			return tui.TransferSentMsg{Err: fmt.Errorf("transfer service not available")}
		}

		var sig string
		var err error

		if m.isSOL {
			sig, err = m.transferSvc.SendSOL(m.rpcClient, m.walletAddr, m.recipient, m.amount, password)
		} else {
			sig, err = m.transferSvc.SendSPL(
				m.rpcClient, m.walletAddr, m.recipient,
				m.selectedToken.Mint, m.amount,
				uint8(m.selectedToken.Decimals), password,
			)
		}

		return tui.TransferSentMsg{Signature: sig, Err: err}
	}
}

// maxSendable returns the maximum amount that can be sent in base units.
// For SOL, it subtracts the estimated fee. For SPL tokens, it's the full balance.
func (m Model) maxSendable() uint64 {
	decimals := m.selectedToken.Decimals
	baseBalance := uint64(m.selectedToken.Amount * math.Pow10(decimals))

	if m.isSOL {
		fee := m.estimateFee()
		if baseBalance <= fee {
			return 0
		}
		return baseBalance - fee
	}
	return baseBalance
}

// estimateFee returns the estimated fee in lamports.
func (m Model) estimateFee() uint64 {
	if m.transferSvc != nil {
		return m.transferSvc.EstimateFee(m.createATA)
	}
	// Fallback: base fee
	fee := uint64(5000)
	if m.createATA {
		fee += 2_039_280
	}
	return fee
}

// formatAmount converts base units to a human-readable string.
func (m Model) formatAmount(baseUnits uint64) string {
	decimals := m.selectedToken.Decimals
	amount := float64(baseUnits) / math.Pow10(decimals)
	return fmt.Sprintf("%g %s", amount, m.selectedToken.Symbol)
}

// truncateAddr shows first 4 and last 4 characters of an address.
func truncateAddr(addr string) string {
	if len(addr) <= 8 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}

// View renders the send wizard.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Send")
	b.WriteString(title + "\n")

	// Step indicator
	stepNum := int(m.step) + 1
	if stepNum > totalSteps {
		stepNum = totalSteps
	}
	indicator := tui.StyleSubtitle.Render(fmt.Sprintf("Step %d/%d - %s", stepNum, totalSteps, m.step.Name()))
	b.WriteString(indicator + "\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
	}

	switch m.step {
	case StepSelectToken:
		b.WriteString("Select token to send:\n\n")
		for i, token := range m.tokens {
			cursor := "  "
			style := tui.StyleTableRow
			if i == m.tokenCursor {
				cursor = tui.StylePrimaryBadge.Render("> ")
				style = tui.StyleTableRowSelected
			}
			line := fmt.Sprintf("%s  %g", token.Symbol, token.Amount)
			if token.USDValue > 0 {
				line += fmt.Sprintf("  ($%.2f)", token.USDValue)
			}
			b.WriteString(cursor + style.Render(line) + "\n")
		}
		b.WriteString("\n" + tui.StyleMuted.Render("Use j/k to navigate, Enter to select, Esc to cancel"))

	case StepRecipient:
		b.WriteString(fmt.Sprintf("Sending: %s\n\n", tui.StyleBold.Render(m.selectedToken.Symbol)))
		b.WriteString("Enter recipient address:\n\n")
		b.WriteString(m.recipientInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue, Esc to go back"))

	case StepAmount:
		b.WriteString(fmt.Sprintf("Sending %s to %s\n\n", tui.StyleBold.Render(m.selectedToken.Symbol), truncateAddr(m.recipient)))
		b.WriteString(fmt.Sprintf("Available: %g %s\n\n", m.selectedToken.Amount, m.selectedToken.Symbol))
		b.WriteString("Enter amount (or MAX):\n\n")
		b.WriteString(m.amountInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue, Esc to go back"))

	case StepReview:
		b.WriteString("Review Transaction\n\n")
		b.WriteString(fmt.Sprintf("  Token:     %s\n", tui.StyleBold.Render(m.selectedToken.Symbol)))
		b.WriteString(fmt.Sprintf("  Amount:    %s\n", tui.StyleBold.Render(m.amountDisplay+" "+m.selectedToken.Symbol)))
		b.WriteString(fmt.Sprintf("  To:        %s\n", tui.StyleBold.Render(truncateAddr(m.recipient))))
		b.WriteString(fmt.Sprintf("  Fee:       %s\n", tui.StyleMuted.Render(fmt.Sprintf("~%g SOL", float64(m.estimatedFee)/1e9))))
		b.WriteString("\n" + tui.StyleMuted.Render("Press Enter to confirm, Esc to go back"))

	case StepPassword:
		b.WriteString("Enter wallet password to sign transaction:\n\n")
		b.WriteString(m.passwordInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to send, Esc to go back"))

	case StepSending:
		b.WriteString(m.spinner.View() + "\n")

	case StepResult:
		if m.err != nil {
			b.WriteString(tui.StyleError.Render("Transaction Failed") + "\n\n")
			b.WriteString(m.err.Error() + "\n")
		} else {
			b.WriteString(tui.StyleSuccess.Render("Transaction Sent!") + "\n\n")
			b.WriteString(fmt.Sprintf("Signature: %s\n", m.signature))
			b.WriteString(fmt.Sprintf("Explorer:  https://solscan.io/tx/%s\n", m.signature))
		}
		b.WriteString("\n" + tui.StyleMuted.Render("Press any key to continue"))
	}

	return b.String()
}

// CurrentStep returns the current step for testing.
func (m Model) CurrentStep() Step {
	return m.step
}

// TokenCursor returns the current token cursor position for testing.
func (m Model) TokenCursor() int {
	return m.tokenCursor
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
