package components_test

import (
	"testing"

	"github.com/dvrd/hound/internal/tui/components"
)

func TestListCursor_UpDown(t *testing.T) {
	lc := components.NewListCursor(5)
	if lc.Pos() != 0 {
		t.Errorf("initial pos = %d, want 0", lc.Pos())
	}

	lc.Down()
	if lc.Pos() != 1 {
		t.Errorf("after down pos = %d, want 1", lc.Pos())
	}

	lc.Up()
	if lc.Pos() != 0 {
		t.Errorf("after up pos = %d, want 0", lc.Pos())
	}

	// Should not go below 0
	lc.Up()
	if lc.Pos() != 0 {
		t.Errorf("after double up pos = %d, want 0", lc.Pos())
	}
}

func TestListCursor_BoundaryDown(t *testing.T) {
	lc := components.NewListCursor(3)
	lc.Down()
	lc.Down()
	if lc.Pos() != 2 {
		t.Errorf("pos = %d, want 2", lc.Pos())
	}

	// Should not exceed total-1
	lc.Down()
	if lc.Pos() != 2 {
		t.Errorf("after boundary down pos = %d, want 2", lc.Pos())
	}
}

func TestListCursor_SetTotal(t *testing.T) {
	lc := components.NewListCursor(10)
	lc.Down()
	lc.Down()
	lc.Down()
	lc.Down()
	lc.Down() // pos=5

	// Shrink total: cursor should clamp
	lc.SetTotal(3)
	if lc.Pos() != 2 {
		t.Errorf("after shrink pos = %d, want 2", lc.Pos())
	}
}

func TestListCursor_EmptyList(t *testing.T) {
	lc := components.NewListCursor(0)
	lc.Down()
	if lc.Pos() != 0 {
		t.Errorf("down on empty list pos = %d, want 0", lc.Pos())
	}
}

func TestViewWindow(t *testing.T) {
	tests := []struct {
		name      string
		cursor    int
		maxRows   int
		total     int
		wantStart int
		wantEnd   int
	}{
		{"cursor at top", 0, 5, 10, 0, 5},
		{"cursor in middle", 3, 5, 10, 0, 5},
		{"cursor near bottom", 7, 5, 10, 3, 8},
		{"cursor at last", 9, 5, 10, 5, 10},
		{"total less than maxRows", 2, 10, 5, 0, 5},
		{"single item", 0, 5, 1, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := components.ViewWindow(tt.cursor, tt.maxRows, tt.total)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("ViewWindow(%d,%d,%d) = (%d,%d), want (%d,%d)",
					tt.cursor, tt.maxRows, tt.total, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
