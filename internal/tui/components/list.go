package components

// ListCursor manages a cursor over a list with bounded scrolling and a
// visible window. Extracted from repeated patterns across tokenlist,
// walletlist, walletstatus, history, and send views.
type ListCursor struct {
	cursor int
	total  int
}

// NewListCursor creates a cursor with the given total item count.
func NewListCursor(total int) ListCursor {
	return ListCursor{total: total}
}

// SetTotal updates the item count and clamps the cursor.
func (lc *ListCursor) SetTotal(total int) {
	lc.total = total
	lc.Clamp()
}

// Up moves the cursor up by one, clamped at 0.
func (lc *ListCursor) Up() {
	if lc.cursor > 0 {
		lc.cursor--
	}
}

// Down moves the cursor down by one, clamped at total-1.
func (lc *ListCursor) Down() {
	if lc.cursor < lc.total-1 {
		lc.cursor++
	}
}

// Clamp ensures the cursor is within bounds.
func (lc *ListCursor) Clamp() {
	if lc.total == 0 {
		lc.cursor = 0
	} else if lc.cursor >= lc.total {
		lc.cursor = lc.total - 1
	}
}

// Pos returns the current cursor position.
func (lc ListCursor) Pos() int {
	return lc.cursor
}

// Reset sets the cursor to 0.
func (lc *ListCursor) Reset() {
	lc.cursor = 0
}

// ViewWindow computes the [startIdx, endIdx) range to display within a
// maxRows constraint, keeping the cursor visible. Replaces the duplicated
// viewWindow/startIdx/endIdx logic in tokenlist, walletlist, walletstatus,
// and history views.
func ViewWindow(cursor, maxRows, total int) (startIdx, endIdx int) {
	startIdx = 0
	if cursor >= maxRows {
		startIdx = cursor - maxRows + 1
	}
	endIdx = startIdx + maxRows
	if endIdx > total {
		endIdx = total
		startIdx = endIdx - maxRows
		if startIdx < 0 {
			startIdx = 0
		}
	}
	return
}
