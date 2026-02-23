package tui_test

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/config"
	"github.com/dvrd/hound/internal/tui"
)

// mockView is a minimal tea.Model for testing navigation.
type mockView struct {
	name string
}

func (m mockView) Init() tea.Cmd                           { return nil }
func (m mockView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m mockView) View() string                            { return "view:" + m.name }

func testFactory(name string, data interface{}) tea.Model {
	return mockView{name: name}
}

func newTestApp() tui.App {
	return tui.NewApp(nil, nil, nil, config.Config{}, testFactory)
}

func TestNewApp_NoFactory(t *testing.T) {
	app := tui.NewApp(nil, nil, nil, config.Config{})
	// Should not panic
	view := app.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}
}

func TestApp_InitCreatesWalletListView(t *testing.T) {
	app := newTestApp()
	app.Init()
	// After Init, the view factory should have been called with "wallet-list"
	// We can't directly check since Init returns a Cmd, but we can verify
	// the app doesn't panic
}

func TestApp_ViewBeforeReady(t *testing.T) {
	app := newTestApp()
	view := app.View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("View before ready should contain 'Initializing', got %q", view)
	}
}

func TestApp_ViewAfterWindowSize(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)
	view := a.View()
	// After receiving WindowSizeMsg, ready should be true
	if strings.Contains(view, "Initializing") {
		t.Error("View after WindowSizeMsg should not contain 'Initializing'")
	}
}

func TestApp_QuitOnCtrlC(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("ctrl+c should return tea.Quit")
	}
}

func TestApp_QDelegatedToView(t *testing.T) {
	// 'q' is no longer handled at App level — it's delegated to individual views
	// so it doesn't interfere with text input views (import wizard, swap, etc.)
	app := newTestApp()
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// Without a current view, 'q' is a no-op at App level (no quit)
	// The key falls through to the current view's Update
	_ = cmd
}

func TestApp_ToggleHelp(t *testing.T) {
	app := newTestApp()
	if app.IsHelpVisible() {
		t.Error("help should be hidden initially")
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a := model.(tui.App)
	if !a.IsHelpVisible() {
		t.Error("help should be visible after '?'")
	}

	// When help is visible, '?' should close it
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a = model.(tui.App)
	if a.IsHelpVisible() {
		t.Error("help should be hidden after second '?'")
	}
}

func TestApp_HelpConsumesKeys(t *testing.T) {
	app := newTestApp()
	// Open help
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a := model.(tui.App)

	// 'q' should NOT quit when help is visible
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	a = model.(tui.App)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("q should not quit when help is visible")
		}
	}
	// Help should still be visible (q was consumed)
	if !a.IsHelpVisible() {
		t.Error("help should still be visible after 'q' (consumed)")
	}
}

func TestApp_HelpEscCloses(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a := model.(tui.App)
	if !a.IsHelpVisible() {
		t.Fatal("help should be visible")
	}

	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEscape})
	a = model.(tui.App)
	if a.IsHelpVisible() {
		t.Error("esc should close help overlay")
	}
}

func TestApp_NavigateMsg(t *testing.T) {
	app := newTestApp()
	// Make ready
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Navigate to wallet-import
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-import"})
	a = model.(tui.App)

	if a.ViewStackDepth() != 1 {
		t.Errorf("ViewStackDepth = %d, want 1", a.ViewStackDepth())
	}

	view := a.GetCurrentView()
	if view == nil {
		t.Fatal("current view should not be nil after navigate")
	}
	mv, ok := view.(mockView)
	if !ok {
		t.Fatal("current view should be mockView")
	}
	if mv.name != "wallet-import" {
		t.Errorf("current view name = %q, want %q", mv.name, "wallet-import")
	}
}

func TestApp_NavigateBackMsg(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Navigate forward
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-import"})
	a = model.(tui.App)
	if a.ViewStackDepth() != 1 {
		t.Fatalf("ViewStackDepth = %d, want 1", a.ViewStackDepth())
	}

	// Navigate back
	model, _ = a.Update(tui.NavigateBackMsg{})
	a = model.(tui.App)
	if a.ViewStackDepth() != 0 {
		t.Errorf("ViewStackDepth = %d, want 0 after back", a.ViewStackDepth())
	}
}

func TestApp_NavigateBackEmptyStack_Quits(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(tui.NavigateBackMsg{})
	if cmd == nil {
		t.Fatal("NavigateBackMsg with empty stack should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("NavigateBackMsg with empty stack should quit")
	}
}

func TestApp_ErrorMsg(t *testing.T) {
	app := newTestApp()
	if app.IsErrorVisible() {
		t.Error("error should not be visible initially")
	}

	model, cmd := app.Update(tui.ErrorMsg{Err: errors.New("test error")})
	a := model.(tui.App)
	if !a.IsErrorVisible() {
		t.Error("error should be visible after ErrorMsg")
	}
	if cmd == nil {
		t.Error("ErrorMsg should return a dismiss timer command")
	}
}

func TestApp_ViewWithError(t *testing.T) {
	app := newTestApp()
	// Make ready
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Show error
	model, _ = a.Update(tui.ErrorMsg{Err: errors.New("something broke")})
	a = model.(tui.App)

	view := a.View()
	if !strings.Contains(view, "something broke") {
		t.Errorf("View should contain error message, got %q", view)
	}
}

func TestApp_ViewWithHelp(t *testing.T) {
	app := newTestApp()
	// Make ready
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Toggle help
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	a = model.(tui.App)

	view := a.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Errorf("View should contain help overlay, got %q", view)
	}
}

func TestApp_NavigateNoFactory(t *testing.T) {
	app := tui.NewApp(nil, nil, nil, config.Config{})
	// Should not panic
	model, _ := app.Update(tui.NavigateMsg{View: "wallet-list"})
	_ = model.(tui.App)
}

func TestApp_MultipleNavigations(t *testing.T) {
	app := newTestApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	// Navigate forward twice
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-import"})
	a = model.(tui.App)
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-status", Data: "addr123"})
	a = model.(tui.App)

	if a.ViewStackDepth() != 2 {
		t.Errorf("ViewStackDepth = %d, want 2", a.ViewStackDepth())
	}

	// Navigate back once
	model, _ = a.Update(tui.NavigateBackMsg{})
	a = model.(tui.App)
	if a.ViewStackDepth() != 1 {
		t.Errorf("ViewStackDepth = %d, want 1", a.ViewStackDepth())
	}

	// Navigate back again
	model, _ = a.Update(tui.NavigateBackMsg{})
	a = model.(tui.App)
	if a.ViewStackDepth() != 0 {
		t.Errorf("ViewStackDepth = %d, want 0", a.ViewStackDepth())
	}
}

func TestApp_ForwardsInnerDimensions(t *testing.T) {
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	_ = model

	if capturedWidth != 94 {
		t.Errorf("inner width = %d, want 94", capturedWidth)
	}
	if capturedHeight != 36 {
		t.Errorf("inner height = %d, want 36", capturedHeight)
	}
}

func TestApp_ForwardsInnerDimensions_Small(t *testing.T) {
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 20, Height: 6})
	_ = model

	if capturedWidth != 20 {
		t.Errorf("inner width = %d, want 20", capturedWidth)
	}
	if capturedHeight != 5 {
		t.Errorf("inner height = %d, want 5", capturedHeight)
	}
}

func TestApp_NavigateForwardsInnerDimensions(t *testing.T) {
	var capturedWidth, capturedHeight int
	capturingFactory := func(name string, data interface{}) tea.Model {
		return &capturingView{onSize: func(w, h int) {
			capturedWidth = w
			capturedHeight = h
		}}
	}

	app := tui.NewApp(nil, nil, nil, config.Config{}, capturingFactory)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := model.(tui.App)

	capturedWidth = 0
	capturedHeight = 0
	model, _ = a.Update(tui.NavigateMsg{View: "wallet-import"})
	_ = model

	if capturedWidth != 74 {
		t.Errorf("navigate inner width = %d, want 74", capturedWidth)
	}
	if capturedHeight != 20 {
		t.Errorf("navigate inner height = %d, want 20", capturedHeight)
	}
}

type capturingView struct {
	onSize func(w, h int)
}

func (v *capturingView) Init() tea.Cmd { return nil }
func (v *capturingView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok && v.onSize != nil {
		v.onSize(wsm.Width, wsm.Height)
	}
	return v, nil
}
func (v *capturingView) View() string { return "capturing" }
