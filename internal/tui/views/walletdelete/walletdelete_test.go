package walletdelete_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/walletdelete"
)

var testWallet = models.Wallet{
	Address:    "7xKXabc1234567890abcdef9mPq",
	Label:      "Main",
	IsPrimary:  true,
	WalletType: models.WalletTypeBIP44Standard,
}

func newTestModel(walletCount int) walletdelete.Model {
	return walletdelete.New(testWallet, nil, walletCount)
}

func TestNew(t *testing.T) {
	m := newTestModel(3)
	view := m.View()
	if !strings.Contains(view, "Delete Wallet") {
		t.Errorf("View should contain 'Delete Wallet', got %q", view)
	}
}

func TestViewContainsWalletDetails(t *testing.T) {
	m := newTestModel(3)
	view := m.View()
	if !strings.Contains(view, testWallet.Address) {
		t.Error("View should contain wallet address")
	}
	if !strings.Contains(view, testWallet.Label) {
		t.Error("View should contain wallet label")
	}
	if !strings.Contains(view, "BIP44_Standard") {
		t.Error("View should contain wallet type")
	}
}

func TestViewContainsWarning(t *testing.T) {
	m := newTestModel(3)
	view := m.View()
	if !strings.Contains(view, "cannot be undone") {
		t.Error("View should contain warning about irreversibility")
	}
}

func TestEscNavigatesBack(t *testing.T) {
	m := newTestModel(3)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc should return NavigateBackMsg, got %T", msg)
	}
}

func TestLastWallet_CannotDelete(t *testing.T) {
	m := newTestModel(1)
	view := m.View()
	if !strings.Contains(view, "Cannot delete your only wallet") {
		t.Error("View should show 'Cannot delete your only wallet' when walletCount <= 1")
	}
}

func TestLastWallet_EnterBlocked(t *testing.T) {
	m := newTestModel(1)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletdelete.Model)
	if model.IsConfirmed() {
		t.Error("should not be confirmed when last wallet")
	}
	if cmd != nil {
		t.Error("enter on last wallet should not return a command")
	}
}

func TestEnterWithWrongAddress(t *testing.T) {
	m := newTestModel(3)

	// Type wrong address
	for _, r := range "wrong_address" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(walletdelete.Model)
	}

	// Press enter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletdelete.Model)
	if model.IsConfirmed() {
		t.Error("should not be confirmed with wrong address")
	}
	view := model.View()
	if !strings.Contains(view, "does not match") {
		t.Error("View should show address mismatch error")
	}
}

func TestEnterWithCorrectAddress(t *testing.T) {
	m := newTestModel(3)

	// Type correct address
	for _, r := range testWallet.Address {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(walletdelete.Model)
	}

	// Press enter
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletdelete.Model)
	if !model.IsConfirmed() {
		t.Error("should be confirmed with correct address")
	}
	if !model.IsDeleting() {
		t.Error("should be deleting after confirmation")
	}
	if cmd == nil {
		t.Error("should return a delete command")
	}
}

func TestWalletDeletedMsg_Success(t *testing.T) {
	m := newTestModel(3)
	updated, cmd := m.Update(walletdelete.WalletDeletedMsg{})
	_ = updated.(walletdelete.Model)
	if cmd == nil {
		t.Fatal("successful delete should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("successful delete should navigate back, got %T", msg)
	}
}

func TestWalletDeletedMsg_Error(t *testing.T) {
	m := newTestModel(3)
	updated, _ := m.Update(walletdelete.WalletDeletedMsg{
		Err: models.ErrWalletNotFound,
	})
	model := updated.(walletdelete.Model)
	if model.IsDeleting() {
		t.Error("should not be deleting after error")
	}
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error after failed delete")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel(3)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(walletdelete.Model)
	// Should not panic
}

func TestDeletingView(t *testing.T) {
	m := newTestModel(3)

	// Type correct address
	for _, r := range testWallet.Address {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(walletdelete.Model)
	}

	// Confirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(walletdelete.Model)
	view := model.View()
	if !strings.Contains(view, "Deleting") {
		t.Error("deleting view should contain 'Deleting'")
	}
}

func TestViewContainsConfirmInstructions(t *testing.T) {
	m := newTestModel(3)
	view := m.View()
	if !strings.Contains(view, "Type the full wallet address") {
		t.Error("View should contain confirmation instructions")
	}
}

func TestIsConfirmed_InitiallyFalse(t *testing.T) {
	m := newTestModel(3)
	if m.IsConfirmed() {
		t.Error("should not be confirmed initially")
	}
}

func TestIsDeleting_InitiallyFalse(t *testing.T) {
	m := newTestModel(3)
	if m.IsDeleting() {
		t.Error("should not be deleting initially")
	}
}

func TestWalletDelete_ResponsiveInputWidth(t *testing.T) {
	w := models.Wallet{Address: "abc123", Label: "test"}
	m := walletdelete.New(w, nil, 2)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})

	view := model.(tea.Model).View()
	if view == "" {
		t.Error("View should not be empty at narrow width")
	}
}
