package history_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/history"
)

func newTestModel() history.Model {
	return history.New("7xKXabc123", nil, nil)
}

func loadedModel() history.Model {
	m := newTestModel()
	now := time.Now().Unix()
	items := []services.ActivityItem{
		{
			Signature:    "sig1",
			Type:         "sol_transfer",
			Direction:    "sent",
			Amount:       "1.5 SOL",
			Counterparty: "7xKp...3mFq",
			Fee:          5000,
			Timestamp:    now - 3600, // 1 hour ago
			Status:       "confirmed",
			Slot:         100,
		},
		{
			Signature:    "sig2",
			Type:         "spl_transfer",
			Direction:    "received",
			Amount:       "100 USDC",
			Counterparty: "9aBc...dEfG",
			Fee:          5000,
			Timestamp:    now - 86400*2, // 2 days ago
			Status:       "confirmed",
			Slot:         200,
		},
		{
			Signature:    "sig3",
			Type:         "swap",
			Direction:    "self",
			Amount:       "0.5 SOL",
			Counterparty: "",
			Fee:          5000,
			Timestamp:    now - 86400, // 1 day ago
			Status:       "confirmed",
			Slot:         150,
		},
	}
	updated, _ := m.Update(history.ActivityLoadedMsg{Items: items})
	return updated.(history.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "History") {
		t.Errorf("View should contain 'History', got %q", view)
	}
}

func TestActivityLoadedMsg(t *testing.T) {
	m := loadedModel()
	items := m.GetItems()
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestViewContainsEntries(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "SOL") {
		t.Error("View should contain 'SOL'")
	}
	if !strings.Contains(view, "USDC") {
		t.Error("View should contain 'USDC'")
	}
}

func TestDirectionIcons(t *testing.T) {
	m := loadedModel()
	view := m.View()
	// Check for direction icons (they may be styled, so check the raw characters)
	if !strings.Contains(view, "↑") {
		t.Error("View should contain ↑ for sent")
	}
	if !strings.Contains(view, "↓") {
		t.Error("View should contain ↓ for received")
	}
	if !strings.Contains(view, "⇄") {
		t.Error("View should contain ⇄ for swap")
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "[esc]back") {
		t.Error("View should contain [esc]back in status bar")
	}
}

func TestEscNavigatesBack(t *testing.T) {
	m := loadedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.NavigateBackMsg); !ok {
		t.Errorf("esc should return NavigateBackMsg, got %T", msg)
	}
}

func TestCursorNavigation(t *testing.T) {
	m := loadedModel()

	if m.GetCursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.GetCursor())
	}

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(history.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after j = %d, want 1", model.GetCursor())
	}

	// Move down again
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(history.Model)
	if model.GetCursor() != 2 {
		t.Errorf("cursor after second j = %d, want 2", model.GetCursor())
	}

	// Move down at boundary
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(history.Model)
	if model.GetCursor() != 2 {
		t.Errorf("cursor after third j = %d, want 2 (boundary)", model.GetCursor())
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(history.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after k = %d, want 1", model.GetCursor())
	}
}

func TestActivityLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(history.ActivityLoadedMsg{Err: fmt.Errorf("rpc error")})
	model := updated.(history.Model)
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error when ActivityLoadedMsg has error")
	}
}

func TestEmptyHistory(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(history.ActivityLoadedMsg{Items: nil})
	model := updated.(history.Model)
	view := model.View()
	if !strings.Contains(view, "No transaction history") {
		t.Error("View with no items should show 'No transaction history found'")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(history.Model)
	// Should not panic
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp int64
		contains  string
	}{
		{
			name:      "zero timestamp",
			timestamp: 0,
			contains:  "-",
		},
		{
			name:      "5 minutes ago",
			timestamp: now.Add(-5 * time.Minute).Unix(),
			contains:  "m ago",
		},
		{
			name:      "2 hours ago",
			timestamp: now.Add(-2 * time.Hour).Unix(),
			contains:  "h",
		},
		{
			name:      "3 days ago",
			timestamp: now.Add(-3 * 24 * time.Hour).Unix(),
			contains:  "d",
		},
		{
			name:      "30 days ago",
			timestamp: now.Add(-30 * 24 * time.Hour).Unix(),
			contains:  "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := history.FormatRelativeTime(tt.timestamp)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("FormatRelativeTime(%d) = %q, want to contain %q", tt.timestamp, result, tt.contains)
			}
		})
	}
}

func TestFormatRelativeTime_Ranges(t *testing.T) {
	now := time.Now()

	// Less than 1 hour: "Xm ago"
	result := history.FormatRelativeTime(now.Add(-15 * time.Minute).Unix())
	if !strings.Contains(result, "15m ago") {
		t.Errorf("15 min ago = %q, want '15m ago'", result)
	}

	// Less than 24 hours: "Xh Ym ago"
	result = history.FormatRelativeTime(now.Add(-3*time.Hour - 30*time.Minute).Unix())
	if !strings.Contains(result, "3h") {
		t.Errorf("3h30m ago = %q, want to contain '3h'", result)
	}

	// Less than 7 days: "Xd Yh ago"
	result = history.FormatRelativeTime(now.Add(-2*24*time.Hour - 5*time.Hour).Unix())
	if !strings.Contains(result, "2d") {
		t.Errorf("2d5h ago = %q, want to contain '2d'", result)
	}

	// 7+ days: formatted date
	result = history.FormatRelativeTime(now.Add(-10 * 24 * time.Hour).Unix())
	if strings.Contains(result, "ago") {
		t.Errorf("10 days ago = %q, should not contain 'ago' (should be formatted date)", result)
	}
}

func TestLoadingView(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "History") {
		t.Error("loading view should contain title")
	}
}

func TestSentItemDisplay(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Sent") {
		t.Error("View should contain 'Sent' for sent items")
	}
}

func TestReceivedItemDisplay(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Received") {
		t.Error("View should contain 'Received' for received items")
	}
}

func TestSwapItemDisplay(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Swapped") {
		t.Error("View should contain 'Swapped' for swap items")
	}
}
