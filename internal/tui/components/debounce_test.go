package components_test

import (
	"testing"

	"github.com/dvrd/hound/internal/tui/components"
)

func TestDebouncer_BumpInvalidatesOld(t *testing.T) {
	d := components.NewDebouncer()

	// Initial gen
	initialGen := d.Gen()

	// Bump returns a cmd and increments gen
	_ = d.Bump()
	if d.Gen() == initialGen {
		t.Error("Bump should increment generation")
	}

	// Old generation is stale
	if d.IsCurrent(initialGen) {
		t.Error("old generation should not be current after Bump")
	}

	// Current generation is current
	if !d.IsCurrent(d.Gen()) {
		t.Error("current generation should be current")
	}
}

func TestDebouncer_Reset(t *testing.T) {
	d := components.NewDebouncer()
	_ = d.Bump()
	_ = d.Bump()

	d.Reset()
	if d.Gen() != 0 {
		t.Errorf("after Reset, gen = %d, want 0", d.Gen())
	}
}

func TestDebouncer_MultipleBumps(t *testing.T) {
	d := components.NewDebouncer()

	gen1 := d.Gen()
	_ = d.Bump()
	gen2 := d.Gen()
	_ = d.Bump()
	gen3 := d.Gen()

	if gen1 == gen2 || gen2 == gen3 {
		t.Error("each Bump should produce a unique generation")
	}

	if d.IsCurrent(gen1) || d.IsCurrent(gen2) {
		t.Error("only the latest generation should be current")
	}
	if !d.IsCurrent(gen3) {
		t.Error("latest generation should be current")
	}
}
