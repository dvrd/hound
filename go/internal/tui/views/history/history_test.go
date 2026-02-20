package history_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/history"
)

func newTestModel() history.Model {
	return history.New("7xKXabc123", nil)
}

func loadedModel() history.Model {
	m := newTestModel()
	now := time.Now().Unix()
	entries := []models.SwapHistoryEntry{
		{
			ID:           1,
			InputSymbol:  "SOL",
			OutputSymbol: "USDC",
			InputAmount:  1.0,
			OutputAmount: 150.0,
			Status:       "finalized",
			Dex:          "Raydium",
			CreatedAt:    now - 3600, // 1 hour ago
		},
		{
			ID:           2,
			InputSymbol:  "USDC",
			OutputSymbol: "BONK",
			InputAmount:  50.0,
			OutputAmount: 1000000.0,
			Status:       "failed",
			Dex:          "Orca",
			ErrorMessage: "slippage exceeded",
			CreatedAt:    now - 86400*2, // 2 days ago
		},
	}
	updated, _ := m.Update(history.HistoryLoadedMsg{Entries: entries, Total: 2})
	return updated.(history.Model)
}

func TestNew(t *testing.T) {
	m := newTestModel()
	view := m.View()
	if !strings.Contains(view, "Swap History") {
		t.Errorf("View should contain 'Swap History', got %q", view)
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
	if !strings.Contains(view, "BONK") {
		t.Error("View should contain 'BONK'")
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "[n]ext") {
		t.Error("View should contain [n]ext in status bar")
	}
	if !strings.Contains(view, "[p]rev") {
		t.Error("View should contain [p]rev in status bar")
	}
	if !strings.Contains(view, "[esc]back") {
		t.Error("View should contain [esc]back in status bar")
	}
}

func TestViewContainsPagination(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Page 1/") {
		t.Error("View should contain page indicator")
	}
	if !strings.Contains(view, "2 total") {
		t.Error("View should contain total count")
	}
}

func TestViewContainsDex(t *testing.T) {
	m := loadedModel()
	view := m.View()
	if !strings.Contains(view, "Raydium") {
		t.Error("View should contain DEX name 'Raydium'")
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

	// Move down again (boundary)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(history.Model)
	if model.GetCursor() != 1 {
		t.Errorf("cursor after second j = %d, want 1 (boundary)", model.GetCursor())
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(history.Model)
	if model.GetCursor() != 0 {
		t.Errorf("cursor after k = %d, want 0", model.GetCursor())
	}
}

func TestHistoryLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(history.HistoryLoadedMsg{Err: fmt.Errorf("db error")})
	model := updated.(history.Model)
	view := model.View()
	if !strings.Contains(view, "Error") {
		t.Error("View should show error when HistoryLoadedMsg has error")
	}
}

func TestEmptyHistory(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(history.HistoryLoadedMsg{Entries: nil, Total: 0})
	model := updated.(history.Model)
	view := model.View()
	if !strings.Contains(view, "No swap history") {
		t.Error("View with no entries should show 'No swap history found'")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(history.Model)
	// Should not panic
}

func TestPaginationNextPage(t *testing.T) {
	m := newTestModel()
	// Load with more entries than page size
	updated, _ := m.Update(history.HistoryLoadedMsg{
		Entries: make([]models.SwapHistoryEntry, 20),
		Total:   50,
	})
	model := updated.(history.Model)

	if model.GetPage() != 0 {
		t.Errorf("initial page = %d, want 0", model.GetPage())
	}

	// Press 'n' for next page - this triggers a reload
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = updated.(history.Model)
	if model.GetPage() != 1 {
		t.Errorf("page after 'n' = %d, want 1", model.GetPage())
	}
	if cmd == nil {
		t.Error("next page should trigger a reload command")
	}
}

func TestPaginationPrevPage(t *testing.T) {
	m := newTestModel()
	// Simulate being on page 0
	updated, _ := m.Update(history.HistoryLoadedMsg{
		Entries: make([]models.SwapHistoryEntry, 5),
		Total:   5,
	})
	model := updated.(history.Model)

	// Press 'p' on page 0 - should not go negative
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(history.Model)
	if model.GetPage() != 0 {
		t.Errorf("page after 'p' on page 0 = %d, want 0", model.GetPage())
	}
	if cmd != nil {
		t.Error("prev page on page 0 should not trigger a command")
	}
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
	if !strings.Contains(view, "Swap History") {
		t.Error("loading view should contain title")
	}
}
