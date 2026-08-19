package editor

import "github.com/mattn/go-runewidth"

const tabWidth = 8

// rowStarts gives the rune index each wrapped row of a line begins at, always
// starting with 0. It breaks after a space where it can, mid-word where it must.
func rowStarts(runes []rune, width int) []int {
	return indentedRowStarts(runes, width, 0)
}

// indentedRowStarts reserves indent cells on continuation rows.
func indentedRowStarts(runes []rune, width, indent int) []int {
	starts := []int{0}
	if width <= 0 || len(runes) == 0 {
		return starts
	}
	indent = min(max(indent, 0), max(width-2, 0))

	start := 0
	for start < len(runes) {
		room := width
		if len(starts) > 1 {
			room -= indent
		}
		end, w, lastSpace := start, 0, -1
		for end < len(runes) {
			rw := runeWidthAt(runes[end], width-room+w)
			if w+rw > room {
				break
			}
			w += rw
			if runes[end] == ' ' {
				lastSpace = end
			}
			end++
		}
		if end >= len(runes) {
			break
		}

		brk := end
		if runes[end] == ' ' {
			brk = end + 1
		} else if lastSpace > start {
			brk = lastSpace + 1
		}
		if brk <= start {
			brk = start + 1
		}
		if brk >= len(runes) {
			break
		}
		starts = append(starts, brk)
		start = brk
	}
	return starts
}

// continuationIndent is the display column where wrapped content resumes.
func continuationIndent(runes []rune, width int) int {
	contentStart := leadingSpaceEnd(runes)
	for contentStart < len(runes) {
		if quoteStart, ok := quoteContentStart(runes, contentStart); ok {
			contentStart = quoteStart
			continue
		}
		if item, ok := parseListLine(runes[contentStart:]); ok {
			contentStart += item.contentStart
			continue
		}
		break
	}
	indent := displayWidth(runes[:contentStart])
	return min(indent, max(width-2, 0))
}

func displayWidth(runes []rune) int {
	width := 0
	for _, r := range runes {
		width += runeWidthAt(r, width)
	}
	return width
}

func runeWidthAt(r rune, col int) int {
	if r == '\t' {
		return tabWidth - col%tabWidth
	}
	return runewidth.RuneWidth(r)
}

// renderedRowWidth measures the cells renderRow can draw from one visual row.
func renderedRowWidth(runes []rune, start, end, indent, width int) int {
	col := indent
	for i := start; i < end && i < len(runes); i++ {
		w := renderedRuneWidth(runes[i], col, width)
		if w < 0 {
			break
		}
		col += w
	}
	return col
}

func renderedRuneWidth(r rune, col, width int) int {
	w := runeWidthAt(r, col)
	if col+w <= width {
		return w
	}
	if r == '\t' && col < width {
		return width - col
	}
	return -1
}

func leadingSpaceEnd(runes []rune) int {
	end := 0
	for end < len(runes) && (runes[end] == ' ' || runes[end] == '\t') {
		end++
	}
	return end
}

func quoteContentStart(runes []rune, at int) (int, bool) {
	if at >= len(runes) || runes[at] != '>' {
		return 0, false
	}
	for at < len(runes) && runes[at] == '>' {
		at++
		for at < len(runes) && (runes[at] == ' ' || runes[at] == '\t') {
			at++
		}
	}
	return at, true
}

// cursorRowCol maps a rune index in a line to its wrapped row and the display
// column within that row.
func cursorRowCol(runes []rune, starts []int, col, indent int) (int, int) {
	row := 0
	for i, s := range starts {
		if col >= s {
			row = i
		}
	}
	base := 0
	if row > 0 {
		base = indent
	}
	width := base
	for i := starts[row]; i < col && i < len(runes); i++ {
		width += runeWidthAt(runes[i], width)
	}
	return row, width - base
}

// screenColumnToRune maps a display column on a visual row to a real rune.
func screenColumnToRune(runes []rune, row vrow, desired, width int) int {
	if len(runes) == 0 {
		return 0
	}
	if desired < row.indent {
		return min(row.start, len(runes)-1)
	}

	col, last := row.indent, -1
	for i := row.start; i < row.end && i < len(runes); i++ {
		w := renderedRuneWidth(runes[i], col, width)
		if w < 0 {
			break
		}
		if w == 0 {
			continue
		}
		last = i
		if desired < col+w {
			return i
		}
		col += w
	}
	if last >= 0 {
		return last
	}
	return min(row.start, len(runes)-1)
}

// scrollTo returns the top row that keeps the cursor row inside a window of
// the given height.
func scrollTo(top, cursorRow, height int) int {
	if height <= 0 {
		return 0
	}
	if cursorRow < top {
		top = cursorRow
	}
	if cursorRow >= top+height {
		top = cursorRow - height + 1
	}
	return max(top, 0)
}
