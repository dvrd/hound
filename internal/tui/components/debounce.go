package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// DefaultDebounceDelay is the default debounce delay for search inputs.
	DefaultDebounceDelay = 300 * time.Millisecond
)

// DebounceTickMsg is sent after the debounce delay. The Generation field
// must be compared against the current generation to discard stale ticks.
type DebounceTickMsg struct {
	Generation int
}

// Debouncer tracks a generation counter for debounced key-by-key search.
// Each keystroke bumps the generation; only the latest tick fires the search.
// Replaces the duplicated searchGen + scheduleSearch pattern in tokenlist
// and swapview.
type Debouncer struct {
	gen   int
	delay time.Duration
}

// NewDebouncer creates a new Debouncer with the default delay.
func NewDebouncer() Debouncer {
	return Debouncer{delay: DefaultDebounceDelay}
}

// Bump increments the generation counter and returns a tea.Cmd that fires
// a DebounceTickMsg after the delay. The caller should batch this with
// the text input update command.
func (d *Debouncer) Bump() tea.Cmd {
	d.gen++
	gen := d.gen
	return tea.Tick(d.delay, func(_ time.Time) tea.Msg {
		return DebounceTickMsg{Generation: gen}
	})
}

// IsCurrent returns true if the given generation matches the current one.
// Use this to discard stale DebounceTickMsg instances.
func (d Debouncer) IsCurrent(generation int) bool {
	return generation == d.gen
}

// Gen returns the current generation for external comparisons.
func (d Debouncer) Gen() int {
	return d.gen
}

// Reset sets the generation to 0.
func (d *Debouncer) Reset() {
	d.gen = 0
}
