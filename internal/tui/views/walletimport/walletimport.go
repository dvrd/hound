package walletimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/keystore"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/components"
)

// Step represents the current step in the import wizard.
type Step int

const (
	StepChoice          Step = iota // 0 — "Import existing" or "Create new"
	StepSeedPhrase                  // 1 — only for import flow
	StepShowMnemonic                // 2 — only for generate flow
	StepWalletType                  // 3
	StepAccountIndex                // 4
	StepPassword                    // 5
	StepConfirmPassword             // 6
	StepLabel                       // 7
	StepImporting                   // 8
	StepSuccess                     // 9
)

const totalSteps = 8

// StepName returns the display name for a step.
func (s Step) Name() string {
	switch s {
	case StepChoice:
		return "Choose Action"
	case StepSeedPhrase:
		return "Seed Phrase"
	case StepShowMnemonic:
		return "Recovery Phrase"
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

	// Legacy warning confirmation
	legacyWarning bool

	// Choice step
	choiceOptions []string
	choiceCursor  int
	isGenerate    bool

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
		step:           StepChoice,
		seedInput:      ta,
		typeChoices:    []string{"BIP44 Standard (Phantom/Solflare)", "BIP44 Change (Trust Wallet)", "Solana CLI", "Legacy"},
		accountInput:   ai,
		passwordInput:  pi,
		confirmPwInput: cpi,
		labelInput:     li,
		spinner:        components.NewSpinner("Importing wallet..."),
		choiceOptions:  []string{"Import existing wallet", "Create new wallet"},
		db:             db,
		keystoreSvc:    keystoreSvc,
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
			switch m.step {
			case StepChoice:
				// M6: Clear all sensitive state on exit
				m.password = ""
				m.words = nil
				m.passwordInput.Reset()
				m.confirmPwInput.Reset()
				m.seedInput.Reset()
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			case StepSeedPhrase:
				m.err = nil
				m.seedInput.Reset() // M6: Clear seed input
				m.step = StepChoice
				return m, nil
			case StepShowMnemonic:
				m.err = nil
				m.words = nil
				m.isGenerate = false
				m.step = StepChoice
				return m, nil
			default:
				if m.step > StepShowMnemonic && m.step < StepImporting {
					m.err = nil
					// M6: Clear sensitive inputs when navigating back
					if m.step == StepPassword || m.step == StepConfirmPassword {
						m.passwordInput.Reset()
						m.confirmPwInput.Reset()
						m.password = ""
					}
					m.step--
					// Skip StepShowMnemonic if importing, or StepSeedPhrase if generating
					if !m.isGenerate && m.step == StepShowMnemonic {
						m.step = StepSeedPhrase
					}
					if m.isGenerate && m.step == StepSeedPhrase {
						m.step = StepShowMnemonic
					}
					return m, m.focusCurrentStep()
				}
				return m, nil
			}
		}

		switch m.step {
		case StepChoice:
			return m.updateChoice(msg)
		case StepSeedPhrase:
			return m.updateSeedPhrase(msg)
		case StepShowMnemonic:
			return m.updateShowMnemonic(msg)
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

func (m Model) updateChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.choiceCursor > 0 {
			m.choiceCursor--
		}
	case "down", "j":
		if m.choiceCursor < len(m.choiceOptions)-1 {
			m.choiceCursor++
		}
	case "enter":
		if m.choiceCursor == 0 {
			// Import existing wallet
			m.isGenerate = false
			m.step = StepSeedPhrase
			m.seedInput.Focus()
			return m, m.seedInput.Focus()
		}
		// Create new wallet
		m.isGenerate = true
		mnemonic, err := keystore.GenerateMnemonic(128)
		if err != nil {
			m.err = fmt.Errorf("failed to generate mnemonic: %w", err)
			return m, nil
		}
		m.words = strings.Fields(mnemonic)
		m.step = StepShowMnemonic
		return m, nil
	}
	return m, nil
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

func (m Model) updateShowMnemonic(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		m.err = nil
		m.step = StepWalletType
		return m, nil
	}
	return m, nil
}

func (m Model) updateWalletType(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If showing legacy warning, handle confirmation
	if m.legacyWarning {
		switch msg.String() {
		case "y", "Y":
			m.walletType = models.WalletTypeLegacy
			m.legacyWarning = false
			m.step = StepAccountIndex
			m.accountInput.Focus()
			return m, m.accountInput.Focus()
		case "n", "N", "esc":
			m.legacyWarning = false
			m.err = nil
			return m, nil
		}
		return m, nil
	}

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
			// Show legacy warning instead of proceeding
			m.legacyWarning = true
			return m, nil
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
		if err := keystore.ValidatePasswordStrength(pw); err != nil {
			m.err = err
			return m, nil
		}
		m.password = pw
		// M6: Clear password from input buffer after extraction
		m.passwordInput.Reset()
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
		// M6: Clear confirm password buffer after validation
		m.confirmPwInput.Reset()
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
		cmd := m.doImport()
		// M6: Clear sensitive state after capturing in closure
		m.password = ""
		m.words = nil
		m.seedInput.Reset()
		return m, tea.Batch(m.spinner.Init(), cmd)
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

	// Step indicator — adjust step number for display
	stepNum := m.displayStepNum()
	indicator := tui.StyleSubtitle.Render(fmt.Sprintf("Step %d/%d - %s", stepNum, totalSteps, m.step.Name()))
	b.WriteString(indicator + "\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
	}

	switch m.step {
	case StepChoice:
		b.WriteString("What would you like to do?\n\n")
		for i, option := range m.choiceOptions {
			cursor := "  "
			if i == m.choiceCursor {
				cursor = tui.StylePrimaryBadge.Render("> ")
			}
			b.WriteString(cursor + option + "\n")
		}
		b.WriteString("\n" + tui.StyleMuted.Render("Use arrow keys to select, Enter to confirm, Esc to cancel"))

	case StepSeedPhrase:
		b.WriteString("Enter your seed phrase (12 or 24 words):\n\n")
		b.WriteString(m.seedInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Ctrl+D or Tab to continue, Esc to go back"))

	case StepShowMnemonic:
		b.WriteString(tui.StyleWarning.Render("Write down these words and store them safely!") + "\n")
		b.WriteString(tui.StyleWarning.Render("You will NOT be able to see them again.") + "\n\n")
		b.WriteString("Your recovery phrase:\n\n")
		// Display words in a 3-column grid
		for i, word := range m.words {
			col := i % 3
			num := fmt.Sprintf("%2d. %-12s", i+1, word)
			b.WriteString(tui.StyleBold.Render(num))
			if col == 2 || i == len(m.words)-1 {
				b.WriteString("\n")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n" + tui.StyleMuted.Render("Press Enter to confirm you have saved your recovery phrase"))

	case StepWalletType:
		if m.legacyWarning {
			b.WriteString(tui.StyleWarning.Render("⚠ Legacy wallets cannot be recovered in other wallets") + "\n")
			b.WriteString(tui.StyleWarning.Render("  (Phantom, Solflare, etc.). Use BIP44 Standard instead.") + "\n\n")
			b.WriteString("Continue with Legacy derivation? (y/n)\n")
			return b.String()
		}
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
		b.WriteString("Set encryption password:\n\n")
		b.WriteString(m.passwordInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("12+ chars, uppercase, lowercase, digit, special char") + "\n")
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

// displayStepNum returns the human-friendly step number (1-based),
// skipping steps not relevant to the current flow.
func (m Model) displayStepNum() int {
	// For import flow: Choice, SeedPhrase, WalletType, AccountIndex, Password, ConfirmPassword, Label, Importing/Success
	// For generate flow: Choice, ShowMnemonic, WalletType, AccountIndex, Password, ConfirmPassword, Label, Importing/Success
	switch m.step {
	case StepChoice:
		return 1
	case StepSeedPhrase, StepShowMnemonic:
		return 2
	case StepWalletType:
		return 3
	case StepAccountIndex:
		return 4
	case StepPassword:
		return 5
	case StepConfirmPassword:
		return 6
	case StepLabel:
		return 7
	case StepImporting, StepSuccess:
		return 8
	default:
		return int(m.step) + 1
	}
}

// CurrentStep returns the current step for testing.
func (m Model) CurrentStep() Step {
	return m.step
}

// IsGenerate returns whether the generate flow was chosen for testing.
func (m Model) IsGenerate() bool {
	return m.isGenerate
}

// Words returns the collected words for testing.
func (m Model) Words() []string {
	return m.words
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
