package editor

import (
	"strconv"
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

// A muted background keeps the syntax colors readable under a selection,
// which reverse video does not.
var selectionStyle = lipgloss.NewStyle().Background(lipgloss.Color("8"))

var (
	numberStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	currentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
)

// GutterWidth is the number column plus padding. Reserved at all times, so
// nothing shifts when the line count crosses a power of ten.
func GutterWidth(lineCount int) int {
	digits := len(strconv.Itoa(lineCount))
	return max(digits, 2) + 2
}

// gutter renders the number cell for one screen row. The cursor's line shows
// its absolute number, every other line its distance, all in one column.
func gutter(line, cursorLine, width int, first bool) string {
	if !first {
		return strings.Repeat(" ", width)
	}
	if line == cursorLine {
		return currentStyle.Render(pad(strconv.Itoa(line+1), width))
	}
	distance := line - cursorLine
	if distance < 0 {
		distance = -distance
	}
	return numberStyle.Render(pad(strconv.Itoa(distance), width))
}

// pad right aligns s in width, leaving a trailing space before the text.
func pad(s string, width int) string {
	room := width - 1
	if len(s) > room {
		s = s[len(s)-room:]
	}
	return strings.Repeat(" ", room-len(s)) + s + " "
}

// Rendered is one frame of the buffer plus where the caret sits in it. The
// caret is reported, not drawn, so the terminal's own cursor shows through.
type Rendered struct {
	Content   string
	CursorRow int
	CursorCol int
}

// vrow is one wrapped screen row: a slice of a logical line.
type vrow struct {
	line       int
	start, end int
}

// Render draws height rows of the buffer, scrolling to keep the caret in
// sight. Width and height are the text area, excluding the status bar.
func (e *Editor) Render(width, height int) Rendered {
	width = max(width, 1)
	height = max(height, 1)

	gw := GutterWidth(e.buf.LineCount())
	textWidth := max(width-gw, 1)

	classes := classifyBuffer(e.buf)
	rows, cursorRow, cursorCol := e.layout(textWidth)
	e.top = scrollTo(e.top, cursorRow, height)

	var out []string
	for i := e.top; i < e.top+height; i++ {
		if i >= len(rows) {
			out = append(out, strings.Repeat(" ", gw))
			continue
		}
		row := rows[i]
		out = append(out, gutter(row.line, e.cursor.Line, gw, row.start == 0)+e.renderRow(row, classes, textWidth))
	}
	return Rendered{
		Content:   strings.Join(out, "\n"),
		CursorRow: cursorRow - e.top,
		CursorCol: min(cursorCol, textWidth-1) + gw,
	}
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

// renderRow styles one screen row, marking any visual selection. It stops at
// width, because the space a line wraps on stays at the end of the row above.
func (e *Editor) renderRow(row vrow, classes [][]tokenClass, width int) string {
	runes := e.buf.runes(row.line)
	lineClasses := classes[row.line]
	selFrom, selTo, selLines := e.Selection()

	var sb strings.Builder
	col := 0
	for i := row.start; i < row.end && i < len(runes); i++ {
		w := runewidth.RuneWidth(runes[i])
		if col+w > width {
			break
		}
		style := classStyles[lineClasses[i]]
		if e.selected(Pos{row.line, i}, selFrom, selTo, selLines) {
			style = selectionStyle
		}
		sb.WriteString(style.Render(string(runes[i])))
		col += w
	}
	return sb.String()
}

// selected reports whether p is highlighted, which depends on which visual
// mode is up. A block selection is a rectangle, not a run of text.
func (e *Editor) selected(p, from, to Pos, linewise bool) bool {
	switch e.mode {
	case ModeVisualBlock:
		lo, hi := blockCols(from, to)
		return p.Line >= from.Line && p.Line <= to.Line && p.Col >= lo && p.Col <= hi
	case ModeVisual, ModeVisualLine:
		return inSelection(p, from, to, linewise)
	default:
		return false
	}
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
