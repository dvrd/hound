package walletimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
)

// Step represents the current step in the import wizard.
type Step int

const (
	StepSeedPhrase      Step = iota // 0
	StepWalletType                  // 1
	StepAccountIndex                // 2
	StepPassword                    // 3
	StepConfirmPassword             // 4
	StepLabel                       // 5
	StepImporting                   // 6
	StepSuccess                     // 7
)

const totalSteps = 7

// StepName returns the display name for a step.
func (s Step) Name() string {
	switch s {
	case StepSeedPhrase:
		return "Seed Phrase"
	case StepWalletType:
		return "Wallet Type"
	case StepAccountIndex:
		return "Account Index"
	case StepPassword:
		return "Password"
	case StepConfirmPassword:
		return "Confirm Password"
	case StepLabel:
		return "Label"
	case StepImporting:
		return "Importing"
	case StepSuccess:
		return "Success"
	default:
		return "Unknown"
	}
}

// Model is the wallet import wizard view.
type Model struct {
	step           Step
	seedInput      textarea.Model
	typeChoices    []string
	typeCursor     int
	accountInput   textinput.Model
	passwordInput  textinput.Model
	confirmPwInput textinput.Model
	labelInput     textinput.Model
	importedAddr   string
	spinner        components.SpinnerModel
	err            error

	// Collected data
	words        []string
	walletType   models.WalletType
	accountIndex int
	password     string
	label        string

	// Dependencies
	db          *database.Database
	keystoreSvc *services.KeystoreService
	width       int
	height      int
}

// New creates a new wallet import wizard.
func New(db *database.Database, keystoreSvc *services.KeystoreService) Model {
	// Seed phrase textarea
	ta := textarea.New()
	ta.Placeholder = "Enter your 12 or 24 word seed phrase..."
	ta.CharLimit = 500
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.Focus()

	// Account index input
	ai := textinput.New()
	ai.Placeholder = "0"
	ai.CharLimit = 5
	ai.Width = 10

	// Password input
	pi := textinput.New()
	pi.Placeholder = "Enter password (12+ chars)"
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 128
	pi.Width = 40

	// Confirm password input
	cpi := textinput.New()
	cpi.Placeholder = "Confirm password"
	cpi.EchoMode = textinput.EchoPassword
	cpi.CharLimit = 128
	cpi.Width = 40

	// Label input
	li := textinput.New()
	li.Placeholder = "e.g. Main Wallet"
	li.CharLimit = 32
	li.Width = 30

	return Model{
		step:           StepSeedPhrase,
		seedInput:      ta,
		typeChoices:    []string{"BIP44 Standard (Phantom/Solflare)", "BIP44 Change (Trust Wallet)", "Solana CLI", "Legacy"},
		accountInput:   ai,
		passwordInput:  pi,
		confirmPwInput: cpi,
		labelInput:     li,
		spinner:        components.NewSpinner("Importing wallet..."),
		db:             db,
		keystoreSvc:    keystoreSvc,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tui.WalletImportedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.step = StepLabel // go back to allow retry
			return m, nil
		}
		m.importedAddr = msg.Address
		m.step = StepSuccess
		return m, nil

	case tea.KeyMsg:
		// Global escape handling
		if msg.String() == "esc" {
			if m.step > StepSeedPhrase {
				m.err = nil
				m.step--
				return m, m.focusCurrentStep()
			}
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}

		switch m.step {
		case StepSeedPhrase:
			return m.updateSeedPhrase(msg)
		case StepWalletType:
			return m.updateWalletType(msg)
		case StepAccountIndex:
			return m.updateAccountIndex(msg)
		case StepPassword:
			return m.updatePassword(msg)
		case StepConfirmPassword:
			return m.updateConfirmPassword(msg)
		case StepLabel:
			return m.updateLabel(msg)
		case StepSuccess:
			// Any key goes back
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}

	// Update spinner during import
	if m.step == StepImporting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) focusCurrentStep() tea.Cmd {
	switch m.step {
	case StepSeedPhrase:
		return m.seedInput.Focus()
	case StepAccountIndex:
		return m.accountInput.Focus()
	case StepPassword:
		return m.passwordInput.Focus()
	case StepConfirmPassword:
		return m.confirmPwInput.Focus()
	case StepLabel:
		return m.labelInput.Focus()
	default:
		return nil
	}
}

func (m Model) updateSeedPhrase(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+d" || msg.String() == "tab" {
		// Submit seed phrase
		text := strings.TrimSpace(m.seedInput.Value())
		words := strings.Fields(text)
		if len(words) != 12 && len(words) != 24 {
			m.err = fmt.Errorf("seed phrase must be 12 or 24 words (got %d)", len(words))
			return m, nil
		}
		m.words = words
		m.err = nil
		m.step = StepWalletType
		return m, nil
	}

	var cmd tea.Cmd
	m.seedInput, cmd = m.seedInput.Update(msg)
	return m, cmd
}

func (m Model) updateWalletType(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.typeCursor > 0 {
			m.typeCursor--
		}
	case "down", "j":
		if m.typeCursor < len(m.typeChoices)-1 {
			m.typeCursor++
		}
	case "enter":
		switch m.typeCursor {
		case 0:
			m.walletType = models.WalletTypeBIP44Standard
		case 1:
			m.walletType = models.WalletTypeBIP44Change
		case 2:
			m.walletType = models.WalletTypeSolanaCLI
		case 3:
			m.walletType = models.WalletTypeLegacy
		}
		m.step = StepAccountIndex
		m.accountInput.Focus()
		return m, m.accountInput.Focus()
	}
	return m, nil
}

func (m Model) updateAccountIndex(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.accountInput.Value())
		if val == "" {
			m.accountIndex = 0
		} else {
			idx, err := strconv.Atoi(val)
			if err != nil || idx < 0 {
				m.err = fmt.Errorf("account index must be a non-negative number")
				return m, nil
			}
			m.accountIndex = idx
		}
		m.err = nil
		m.step = StepPassword
		m.passwordInput.Focus()
		return m, m.passwordInput.Focus()
	}

	var cmd tea.Cmd
	m.accountInput, cmd = m.accountInput.Update(msg)
	return m, cmd
}

func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		pw := m.passwordInput.Value()
		if len(pw) < 12 {
			m.err = fmt.Errorf("password must be at least 12 characters")
			return m, nil
		}
		m.password = pw
		m.err = nil
		m.step = StepConfirmPassword
		m.confirmPwInput.Focus()
		return m, m.confirmPwInput.Focus()
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmPassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		if m.confirmPwInput.Value() != m.password {
			m.err = fmt.Errorf("passwords do not match")
			return m, nil
		}
		m.err = nil
		m.step = StepLabel
		m.labelInput.Focus()
		return m, m.labelInput.Focus()
	}

	var cmd tea.Cmd
	m.confirmPwInput, cmd = m.confirmPwInput.Update(msg)
	return m, cmd
}

func (m Model) updateLabel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		label := strings.TrimSpace(m.labelInput.Value())
		if label == "" {
			m.err = fmt.Errorf("label cannot be empty")
			return m, nil
		}
		m.label = label
		m.err = nil
		m.step = StepImporting
		return m, tea.Batch(m.spinner.Init(), m.doImport())
	}

	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

func (m Model) doImport() tea.Cmd {
	return func() tea.Msg {
		if m.keystoreSvc == nil || m.db == nil {
			return tui.WalletImportedMsg{Err: fmt.Errorf("import service not available")}
		}
		addr, err := m.keystoreSvc.ImportKeypair(
			m.db, m.words, m.password, m.label, true,
			m.walletType, m.accountIndex,
		)
		return tui.WalletImportedMsg{Address: addr, Err: err}
	}
}

// View renders the import wizard.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Import Wallet")
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
	case StepSeedPhrase:
		b.WriteString("Enter your seed phrase (12 or 24 words):\n\n")
		b.WriteString(m.seedInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Ctrl+D or Tab to continue, Esc to cancel"))

	case StepWalletType:
		b.WriteString("Select wallet type:\n\n")
		for i, choice := range m.typeChoices {
			cursor := "  "
			if i == m.typeCursor {
				cursor = tui.StylePrimaryBadge.Render("> ")
			}
			b.WriteString(cursor + choice + "\n")
		}
		b.WriteString("\n" + tui.StyleMuted.Render("Use arrow keys to select, Enter to confirm"))

	case StepAccountIndex:
		b.WriteString("Account index (default 0):\n\n")
		b.WriteString(m.accountInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue"))

	case StepPassword:
		b.WriteString("Set encryption password (12+ characters):\n\n")
		b.WriteString(m.passwordInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue"))

	case StepConfirmPassword:
		b.WriteString("Confirm password:\n\n")
		b.WriteString(m.confirmPwInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue"))

	case StepLabel:
		b.WriteString("Enter a label for this wallet:\n\n")
		b.WriteString(m.labelInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to import"))

	case StepImporting:
		b.WriteString(m.spinner.View() + "\n")

	case StepSuccess:
		b.WriteString(tui.StyleSuccess.Render("Wallet imported successfully!") + "\n\n")
		b.WriteString(fmt.Sprintf("Address: %s\n", m.importedAddr))
		b.WriteString("\n" + tui.StyleMuted.Render("Press any key to continue"))
	}

	return b.String()
}

// CurrentStep returns the current step for testing.
func (m Model) CurrentStep() Step {
	return m.step
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
