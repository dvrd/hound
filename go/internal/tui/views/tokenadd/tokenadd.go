package tokenadd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
)

// AddStep represents the current step in the add token wizard.
type AddStep int

const (
	StepSymbol  AddStep = iota // 0
	StepName                   // 1
	StepAddress                // 2
	StepConfirm                // 3
	StepSaving                 // 4
	StepDone                   // 5
)

const totalSteps = 4

// StepName returns the display name for a step.
func (s AddStep) StepName() string {
	switch s {
	case StepSymbol:
		return "Symbol"
	case StepName:
		return "Name"
	case StepAddress:
		return "Contract Address"
	case StepConfirm:
		return "Confirm"
	case StepSaving:
		return "Saving"
	case StepDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// TokenSavedMsg is sent when a token save completes.
type TokenSavedMsg struct {
	Err error
}

// Model is the add token wizard view.
type Model struct {
	step         AddStep
	symbolInput  textinput.Model
	nameInput    textinput.Model
	addressInput textinput.Model
	db           *database.Database
	err          error
	width        int
	height       int
}

// New creates a new add token wizard.
func New(db *database.Database) Model {
	si := textinput.New()
	si.Placeholder = "e.g. BONK"
	si.CharLimit = 20
	si.Width = 30
	si.Focus()

	ni := textinput.New()
	ni.Placeholder = "e.g. Bonk"
	ni.CharLimit = 64
	ni.Width = 40

	ai := textinput.New()
	ai.Placeholder = "e.g. DezXAZ8z7Pnrn..."
	ai.CharLimit = 64
	ai.Width = 50

	return Model{
		step:         StepSymbol,
		symbolInput:  si,
		nameInput:    ni,
		addressInput: ai,
		db:           db,
	}
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TokenSavedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.step = StepConfirm
			return m, nil
		}
		m.step = StepDone
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global escape handling
		if msg.String() == "esc" {
			if m.step > StepSymbol && m.step < StepSaving {
				m.err = nil
				m.step--
				return m, m.focusCurrentStep()
			}
			if m.step == StepSymbol {
				return m, func() tea.Msg { return tui.NavigateBackMsg{} }
			}
			return m, nil
		}

		switch m.step {
		case StepSymbol:
			return m.updateSymbol(msg)
		case StepName:
			return m.updateName(msg)
		case StepAddress:
			return m.updateAddress(msg)
		case StepConfirm:
			return m.updateConfirm(msg)
		case StepDone:
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}

	return m, nil
}

func (m Model) focusCurrentStep() tea.Cmd {
	switch m.step {
	case StepSymbol:
		return m.symbolInput.Focus()
	case StepName:
		return m.nameInput.Focus()
	case StepAddress:
		return m.addressInput.Focus()
	default:
		return nil
	}
}

func (m Model) updateSymbol(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.symbolInput.Value())
		if val == "" {
			m.err = fmt.Errorf("symbol cannot be empty")
			return m, nil
		}
		m.err = nil
		m.step = StepName
		m.nameInput.Focus()
		return m, m.nameInput.Focus()
	}

	var cmd tea.Cmd
	m.symbolInput, cmd = m.symbolInput.Update(msg)
	return m, cmd
}

func (m Model) updateName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.nameInput.Value())
		if val == "" {
			m.err = fmt.Errorf("name cannot be empty")
			return m, nil
		}
		m.err = nil
		m.step = StepAddress
		m.addressInput.Focus()
		return m, m.addressInput.Focus()
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m Model) updateAddress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.addressInput.Value())
		if len(val) < 32 || len(val) > 44 {
			m.err = fmt.Errorf("contract address must be 32-44 characters (got %d)", len(val))
			return m, nil
		}

		// Check for duplicates in DB
		if m.db != nil {
			_, err := m.db.GetTokenByContractAddress(val)
			if err == nil {
				m.err = fmt.Errorf("token with this address already exists")
				return m, nil
			}
		}

		m.err = nil
		m.step = StepConfirm
		return m, nil
	}

	var cmd tea.Cmd
	m.addressInput, cmd = m.addressInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		m.step = StepSaving
		return m, m.doSave()
	}
	return m, nil
}

func (m Model) doSave() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return TokenSavedMsg{Err: fmt.Errorf("database not available")}
		}
		token := models.Token{
			Symbol:          strings.TrimSpace(m.symbolInput.Value()),
			Name:            strings.TrimSpace(m.nameInput.Value()),
			ContractAddress: strings.TrimSpace(m.addressInput.Value()),
			Chain:           "solana",
		}
		err := m.db.InsertToken(token)
		return TokenSavedMsg{Err: err}
	}
}

// View renders the add token wizard.
func (m Model) View() string {
	var b strings.Builder

	title := tui.StyleTitle.Render("Add Token")
	b.WriteString(title + "\n")

	// Step indicator
	stepNum := int(m.step) + 1
	if stepNum > totalSteps {
		stepNum = totalSteps
	}
	indicator := tui.StyleSubtitle.Render(fmt.Sprintf("Step %d/%d - %s", stepNum, totalSteps, m.step.StepName()))
	b.WriteString(indicator + "\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(tui.StyleError.Render("Error: "+m.err.Error()) + "\n\n")
	}

	switch m.step {
	case StepSymbol:
		b.WriteString("Enter token symbol:\n\n")
		b.WriteString(m.symbolInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue, Esc to cancel"))

	case StepName:
		b.WriteString("Enter token name:\n\n")
		b.WriteString(m.nameInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue, Esc to go back"))

	case StepAddress:
		b.WriteString("Enter contract address (32-44 chars):\n\n")
		b.WriteString(m.addressInput.View() + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to continue, Esc to go back"))

	case StepConfirm:
		b.WriteString(tui.StyleBold.Render("Confirm token details:") + "\n\n")
		b.WriteString(fmt.Sprintf("  Symbol:   %s\n", strings.TrimSpace(m.symbolInput.Value())))
		b.WriteString(fmt.Sprintf("  Name:     %s\n", strings.TrimSpace(m.nameInput.Value())))
		b.WriteString(fmt.Sprintf("  Address:  %s\n", strings.TrimSpace(m.addressInput.Value())))
		b.WriteString(fmt.Sprintf("  Chain:    %s\n", "solana"))
		b.WriteString("\n")
		b.WriteString(tui.StyleMuted.Render("Press Enter to save, Esc to go back"))

	case StepSaving:
		b.WriteString(tui.StyleMuted.Render("Saving token...") + "\n")

	case StepDone:
		b.WriteString(tui.StyleSuccess.Render("Token added successfully!") + "\n\n")
		b.WriteString(tui.StyleMuted.Render("Press any key to continue"))
	}

	return b.String()
}

// CurrentStep returns the current step for testing.
func (m Model) CurrentStep() AddStep {
	return m.step
}

// SetSize updates the view dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}
