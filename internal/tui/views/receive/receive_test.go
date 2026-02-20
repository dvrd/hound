package receive_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/receive"
)

func newTestModel() receive.Model {
	return receive.New("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU", "Main Wallet")
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU") {
		t.Error("view should contain wallet address")
	}
}

func TestViewContainsAddress(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU") {
		t.Error("view should contain full wallet address")
	}
}

func TestViewContainsLabel(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Main Wallet") {
		t.Error("view should contain wallet label")
	}
}

func TestViewContainsTitle(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Receive") {
		t.Error("view should contain 'Receive'")
	}
}

func TestEscNavigatesBack(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc should return NavigateBackMsg, got %T", msg)
	}
}

func TestCopyKey(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("'c' should return a command")
	}
	// Execute the command to get the clipboard result
	msg := cmd()
	// The clipboard may or may not work in test env, but we should get a clipboardResultMsg
	updated, _ := m.Update(msg)
	model := updated.(receive.Model)
	// Either copied or copyErr should be set
	if !model.IsCopied() && model.GetCopyErr() == "" {
		t.Error("after copy attempt, either copied or copyErr should be set")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(receive.Model)
}

func TestViewContainsInstructions(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "copy") && !strings.Contains(view, "[c]") {
		t.Error("view should contain copy instructions")
	}
}

func TestViewContainsFooter(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "SOL or SPL") {
		t.Error("view should contain footer about sending tokens")
	}
}
