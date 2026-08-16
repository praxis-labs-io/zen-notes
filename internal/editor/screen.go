package editor

// Top is the first visual row on screen, as of the last Render.
func (e *Editor) Top() int { return e.top }

// visibleRows is how many wrapped rows the window shows, capped by the buffer.
func (e *Editor) visibleRows() int {
	return min(e.height, max(len(e.rows)-e.top, 1))
}

// lineAtRow maps a visual row back to the logical line it belongs to, so the
// screen motions land on a line rather than a wrapped fragment.
func (e *Editor) lineAtRow(row int) int {
	if len(e.rows) == 0 {
		return e.cursor.Line
	}
	row = min(max(row, 0), len(e.rows)-1)
	return e.rows[row].line
}

// screenMotion is H, M and L: the top, middle and bottom of what is showing.
// A count offsets H down from the top and L up from the bottom, as in vim.
func (e *Editor) screenMotion(r rune, count int) (motion, bool) {
	visible := e.visibleRows()

	var row int
	switch r {
	case 'H':
		row = e.top + count - 1
	case 'M':
		row = e.top + (visible-1)/2
	case 'L':
		row = e.top + visible - count
	default:
		return motion{}, false
	}
	return motion{target: Pos{e.lineAtRow(row), 0}, kind: linewise}, true
}

// scrollTop repositions the window around the cursor for zt, zz and zb,
// leaving the cursor on its line.
func (e *Editor) scrollTop(where rune) {
	switch where {
	case 't':
		e.top = e.cursorRow
	case 'z':
		e.top = e.cursorRow - (e.height-1)/2
	case 'b':
		e.top = e.cursorRow - e.height + 1
	default:
		return
	}
	e.top = max(e.top, 0)
}
