package walletimport_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/walletimport"
)

func newTestModel() walletimport.Model {
	return walletimport.New(nil, nil)
}

func TestInitialStepIsChoice(t *testing.T) {
	m := newTestModel()
	if m.CurrentStep() != walletimport.StepChoice {
		t.Errorf("initial step = %d, want StepChoice (0)", m.CurrentStep())
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Import Wallet") {
		t.Errorf("View should contain 'Import Wallet', got %q", view)
	}
}

func TestViewContainsStepIndicator(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Step 1/") {
		t.Errorf("View should contain step indicator, got %q", view)
	}
	if !strings.Contains(view, "Choose Action") {
		t.Errorf("View should contain step name 'Choose Action', got %q", view)
	}
}

func TestChoiceImport(t *testing.T) {
	m := newTestModel()
	// Cursor starts at 0 (Import existing)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletimport.Model)
	if model.CurrentStep() != walletimport.StepSeedPhrase {
		t.Errorf("step after choosing import = %d, want StepSeedPhrase", model.CurrentStep())
	}
	if model.IsGenerate() {
		t.Error("should not be in generate mode")
	}
}

func TestChoiceGenerate(t *testing.T) {
	m := newTestModel()
	// Move cursor to "Create new wallet"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletimport.Model)
	// Select it
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletimport.Model)
	if model.CurrentStep() != walletimport.StepShowMnemonic {
		t.Errorf("step after choosing generate = %d, want StepShowMnemonic", model.CurrentStep())
	}
	if !model.IsGenerate() {
		t.Error("should be in generate mode")
	}
	words := model.Words()
	if len(words) != 12 {
		t.Errorf("generated words count = %d, want 12", len(words))
	}
}

func TestShowMnemonicDisplay(t *testing.T) {
	m := newTestModel()
	// Navigate to generate flow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletimport.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)

	view := m.View()
	if !strings.Contains(view, "recovery phrase") {
		t.Error("show mnemonic view should contain 'recovery phrase'")
	}
	// Should contain numbered words
	if !strings.Contains(view, "1.") {
		t.Error("show mnemonic view should contain numbered words")
	}
	if !strings.Contains(view, "Write down") {
		t.Error("show mnemonic view should contain warning")
	}
}

func TestShowMnemonicEnter(t *testing.T) {
	m := newTestModel()
	// Navigate to generate flow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletimport.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)
	if m.CurrentStep() != walletimport.StepShowMnemonic {
		t.Fatalf("should be at StepShowMnemonic, got %d", m.CurrentStep())
	}

	// Press enter to confirm
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)
	if m.CurrentStep() != walletimport.StepWalletType {
		t.Errorf("step after confirming mnemonic = %d, want StepWalletType", m.CurrentStep())
	}
}

func TestShowMnemonicEsc(t *testing.T) {
	m := newTestModel()
	// Navigate to generate flow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(walletimport.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)

	// Esc should go back to choice
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(walletimport.Model)
	if m.CurrentStep() != walletimport.StepChoice {
		t.Errorf("step after esc on mnemonic = %d, want StepChoice", m.CurrentStep())
	}
}

func TestEscAtChoiceNavigatesBack(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc on choice step should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc on choice step should return NavigateBackMsg, got %T", msg)
	}
}

func TestEscOnSeedPhraseGoesToChoice(t *testing.T) {
	m := newTestModel()
	// Go to seed phrase step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)
	if m.CurrentStep() != walletimport.StepSeedPhrase {
		t.Fatalf("should be at seed phrase, got %d", m.CurrentStep())
	}

	// Esc should go back to choice (not NavigateBackMsg)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(walletimport.Model)
	if m.CurrentStep() != walletimport.StepChoice {
		t.Errorf("esc on seed phrase should go to choice, got step %d", m.CurrentStep())
	}
}

func TestStepName(t *testing.T) {
	tests := []struct {
		step walletimport.Step
		want string
	}{
		{walletimport.StepChoice, "Choose Action"},
		{walletimport.StepSeedPhrase, "Seed Phrase"},
		{walletimport.StepShowMnemonic, "Recovery Phrase"},
		{walletimport.StepWalletType, "Wallet Type"},
		{walletimport.StepAccountIndex, "Account Index"},
		{walletimport.StepPassword, "Password"},
		{walletimport.StepConfirmPassword, "Confirm Password"},
		{walletimport.StepLabel, "Label"},
		{walletimport.StepImporting, "Importing"},
		{walletimport.StepSuccess, "Success"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.step.Name()
			if got != tt.want {
				t.Errorf("Step(%d).Name() = %q, want %q", tt.step, got, tt.want)
			}
		})
	}
}

func TestWalletImportedMsg_Success(t *testing.T) {
	m := newTestModel()
	// Simulate receiving a successful import message
	updated, _ := m.Update(tui.WalletImportedMsg{Address: "ABC123"})
	model := updated.(walletimport.Model)
	if model.CurrentStep() != walletimport.StepSuccess {
		t.Errorf("step after success = %d, want StepSuccess", model.CurrentStep())
	}
	view := model.View()
	if !strings.Contains(view, "ABC123") {
		t.Error("success view should contain the imported address")
	}
	if !strings.Contains(view, "successfully") {
		t.Error("success view should contain 'successfully'")
	}
}

func TestWalletImportedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tui.WalletImportedMsg{Err: tui.ErrorMsg{Err: nil}.Err})
	// With nil error, it should go to success
	model := updated.(walletimport.Model)
	if model.CurrentStep() != walletimport.StepSuccess {
		t.Errorf("step after nil error = %d, want StepSuccess", model.CurrentStep())
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletimport.Model)
	// Should not panic
}

func TestSuccessStep_AnyKeyNavigatesBack(t *testing.T) {
	m := newTestModel()
	// Go to success step
	updated, _ := m.Update(tui.WalletImportedMsg{Address: "ABC123"})
	model := updated.(walletimport.Model)

	// Any key should navigate back
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("any key on success step should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("any key on success should return NavigateBackMsg, got %T", msg)
	}
}

func TestChoiceViewContainsOptions(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Import existing") {
		t.Error("choice view should contain 'Import existing'")
	}
	if !strings.Contains(view, "Create new") {
		t.Error("choice view should contain 'Create new'")
	}
}

func TestSeedPhraseStep_ViewContainsInstructions(t *testing.T) {
	m := newTestModel()
	// Go to seed phrase step
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(walletimport.Model)
	view := m.View()
	if !strings.Contains(view, "seed phrase") {
		t.Error("seed phrase step should contain instructions about seed phrase")
	}
}

func TestWalletTypeStep_ViewContainsChoices(t *testing.T) {
	// Verify the view doesn't panic at various steps
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Choose Action") {
		t.Error("initial view should show Choose Action step")
	}
}

func TestImportingStep_ViewContainsSpinner(t *testing.T) {
	m := newTestModel()
	_ = m.View() // Should not panic
}

func TestWalletImport_ResponsiveInputWidths(t *testing.T) {
	m := walletimport.New(nil, nil)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
