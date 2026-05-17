package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubView is a minimal view for benchmarking App.View() overhead.
type stubView struct{}

func (stubView) Init() tea.Cmd                           { return nil }
func (stubView) Update(tea.Msg) (tea.Model, tea.Cmd)     { return stubView{}, nil }
func (stubView) View() string                            { return "content line 1\ncontent line 2\ncontent line 3\ncontent line 4\ncontent line 5" }
func (stubView) Footer() string                          { return "enter status  S send  x swap  │  ? help" }

func BenchmarkAppView(b *testing.B) {
	app := App{
		currentView:     stubView{},
		currentViewName: "wallet-status",
		width:           120,
		height:          40,
		ready:           true,
	}
	// Simulate WindowSizeMsg to populate cached styles.
	app.contentStyle = StyleApp // placeholder — real one needs lipgloss
	app.appStyle = StyleApp

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.View()
	}
}
