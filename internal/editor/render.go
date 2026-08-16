package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// Styles use ANSI base colors so the note takes on the terminal's own theme
// rather than fighting it.
var classStyles = map[tokenClass]lipgloss.Style{
	tokPlain:     lipgloss.NewStyle(),
	tokHeading:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")),
	tokStrong:    lipgloss.NewStyle().Bold(true),
	tokEmphasis:  lipgloss.NewStyle().Italic(true),
	tokCode:      lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	tokMarker:    lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	tokCheckDone: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	tokCheckTodo: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
	tokLink:      lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("4")),
}

var (
	cursorStyle    = lipgloss.NewStyle().Reverse(true)
	selectionStyle = lipgloss.NewStyle().Reverse(true)
)

// vrow is one wrapped screen row: a slice of a logical line.
type vrow struct {
	line       int
	start, end int
}

// View renders height rows of the buffer, scrolling to keep the cursor in
// sight. Width and height are the text area, excluding the status bar.
func (e *Editor) View(width, height int) string {
	width = max(width, 1)
	height = max(height, 1)

	classes := classifyBuffer(e.buf)
	rows, cursorRow, cursorCol := e.layout(width)
	e.top = scrollTo(e.top, cursorRow, height)

	var out []string
	for i := e.top; i < e.top+height; i++ {
		if i >= len(rows) {
			out = append(out, "")
			continue
		}
		showCursor := i == cursorRow
		out = append(out, e.renderRow(rows[i], classes, width, showCursor, cursorCol))
	}
	return strings.Join(out, "\n")
}

// layout wraps every line and reports where the cursor lands, as a row index
// into the returned rows and a display column within that row.
func (e *Editor) layout(width int) ([]vrow, int, int) {
	var rows []vrow
	cursorRow, cursorCol := 0, 0

	for i := range e.buf.LineCount() {
		runes := e.buf.runes(i)
		starts := rowStarts(runes, width)
		for k, s := range starts {
			end := len(runes)
			if k+1 < len(starts) {
				end = starts[k+1]
			}
			if i == e.cursor.Line {
				if r, c := cursorRowCol(runes, starts, e.cursor.Col); r == k {
					cursorRow, cursorCol = len(rows), c
				}
			}
			rows = append(rows, vrow{line: i, start: s, end: end})
		}
	}
	return rows, cursorRow, cursorCol
}

// renderRow styles one screen row, marking any selection and the cursor.
func (e *Editor) renderRow(row vrow, classes [][]tokenClass, width int, showCursor bool, cursorCol int) string {
	runes := e.buf.runes(row.line)
	lineClasses := classes[row.line]
	selFrom, selTo, selLines := e.Selection()
	selecting := e.mode == ModeVisual || e.mode == ModeVisualLine

	var sb strings.Builder
	col := 0
	for i := row.start; i < row.end && i < len(runes); i++ {
		style := classStyles[lineClasses[i]]
		if selecting && inSelection(Pos{row.line, i}, selFrom, selTo, selLines) {
			style = selectionStyle
		}
		if showCursor && col == cursorCol {
			style = cursorStyle
		}
		sb.WriteString(style.Render(string(runes[i])))
		col += runewidth.RuneWidth(runes[i])
	}

	// The caret past the last rune, in insert mode or on an empty line.
	if showCursor && col <= cursorCol && col < width {
		sb.WriteString(cursorStyle.Render(" "))
	}
	return sb.String()
}

// inSelection reports whether p falls inside an ordered visual range.
func inSelection(p, from, to Pos, linewise bool) bool {
	if linewise {
		return p.Line >= from.Line && p.Line <= to.Line
	}
	if p.Line < from.Line || p.Line > to.Line {
		return false
	}
	if p.Line == from.Line && p.Col < from.Col {
		return false
	}
	if p.Line == to.Line && p.Col > to.Col {
		return false
	}
	return true
}
