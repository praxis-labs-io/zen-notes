package editor

import "github.com/mattn/go-runewidth"

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
	indent = min(max(indent, 0), max(width-1, 0))

	start := 0
	for start < len(runes) {
		room := width
		if len(starts) > 1 {
			room -= indent
		}
		end, w, lastSpace := start, 0, -1
		for end < len(runes) {
			rw := runewidth.RuneWidth(runes[end])
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
	if item, ok := parseListLine(runes); ok {
		contentStart = item.contentStart
	} else if quoteStart, ok := quoteContentStart(runes, contentStart); ok {
		contentStart = quoteStart
	}
	indent := runewidth.StringWidth(string(runes[:contentStart]))
	return min(indent, max(width-1, 0))
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
func cursorRowCol(runes []rune, starts []int, col int) (int, int) {
	row := 0
	for i, s := range starts {
		if col >= s {
			row = i
		}
	}
	width := 0
	for i := starts[row]; i < col && i < len(runes); i++ {
		width += runewidth.RuneWidth(runes[i])
	}
	return row, width
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
