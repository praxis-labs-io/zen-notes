package editor

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
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

// Fallbacks for when the terminal will not say what its background is,
// which happens under some multiplexers.
var (
	darkSelection   = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	lightSelection  = lipgloss.NewStyle().Background(lipgloss.Color("253"))
	darkFlash       = lipgloss.NewStyle().Background(lipgloss.Color("242"))
	lightFlash      = lipgloss.NewStyle().Background(lipgloss.Color("248"))
	darkMatch       = lipgloss.NewStyle().Background(lipgloss.Color("240"))
	lightMatch      = lipgloss.NewStyle().Background(lipgloss.Color("250"))
	darkCursorLine  = lipgloss.Color("236")
	lightCursorLine = lipgloss.Color("254")
)

// How far each shade sits off the background. A step near white reads
// stronger than near black, so light themes take a smaller one.
const (
	darkSelectionStep   = 0.14
	lightSelectionStep  = 0.07
	darkFlashStep       = 0.30
	lightFlashStep      = 0.16
	darkMatchStep       = 0.22
	lightMatchStep      = 0.11
	darkCursorLineStep  = 0.05
	lightCursorLineStep = 0.025
)

var (
	numberStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	currentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)

// SetDarkBackground picks fallbacks when only the light or dark bias is
// known, not the colour itself.
func (e *Editor) SetDarkBackground(dark bool) {
	e.darkBackground = dark
	e.selection, e.flashStyle, e.matchStyle = darkSelection, darkFlash, darkMatch
	e.cursorLine = darkCursorLine
	if !dark {
		e.selection, e.flashStyle, e.matchStyle = lightSelection, lightFlash, lightMatch
		e.cursorLine = lightCursorLine
	}
}

// SetBackground tunes the selection and the yank flash to the terminal's own
// background, keeping its hue so both belong to the theme rather than greying
// it out. Call it again whenever the terminal may have changed theme.
func (e *Editor) SetBackground(c color.Color) {
	col, ok := colorful.MakeColor(c)
	if !ok {
		return
	}
	h, s, l := col.Hsl()
	e.darkBackground = l < 0.5

	selStep, flashStep := darkSelectionStep, darkFlashStep
	matchStep, lineStep := darkMatchStep, darkCursorLineStep
	if !e.darkBackground {
		selStep, flashStep = -lightSelectionStep, -lightFlashStep
		matchStep, lineStep = -lightMatchStep, -lightCursorLineStep
	}
	e.selection = shifted(h, s, l, selStep)
	e.flashStyle = shifted(h, s, l, flashStep)
	e.matchStyle = shifted(h, s, l, matchStep)
	e.cursorLine = shade(h, s, l, lineStep)
}

// shade is a colour the given lightness step away from the theme's.
func shade(h, s, l, step float64) color.Color {
	l = min(max(l+step, 0), 1)
	return lipgloss.Color(colorful.Hsl(h, s, l).Hex())
}

func shifted(h, s, l, step float64) lipgloss.Style {
	return lipgloss.NewStyle().Background(shade(h, s, l, step))
}

func (e *Editor) selectionStyle() lipgloss.Style { return e.selection }

// YankFlash reports whether a yank is still lit up.
func (e *Editor) YankFlash() bool { return e.flash.active }

// ClearYankFlash puts the flash out.
func (e *Editor) ClearYankFlash() { e.flash = flashRange{} }

// flashYank lights up what a yank just took, so it is visible that it worked.
func (e *Editor) flashYank(from, to Pos, linewise, block bool) {
	e.flash = flashRange{active: true, from: from, to: to, linewise: linewise, block: block}
}

func (e *Editor) flashCovers(p Pos) bool {
	if !e.flash.active {
		return false
	}
	return inRange(p, e.flash.from, e.flash.to, e.flash.linewise, e.flash.block)
}

// GutterWidth is the widest line number plus the space before the text.
// Reserved at all times, so nothing shifts as the line count grows.
func GutterWidth(lineCount int) int {
	digits := len(strconv.Itoa(lineCount))
	return max(digits, 2) + 1
}

// gutter renders the number cell for one screen row. The cursor's line shows
// its absolute number, every other line its distance, all in one column.
func gutter(line, cursorLine, width int, first bool, bg color.Color) string {
	if !first {
		return washed(lipgloss.NewStyle(), bg).Render(strings.Repeat(" ", width))
	}
	if line == cursorLine {
		return washed(currentStyle, bg).Render(pad(strconv.Itoa(line+1), width))
	}
	distance := line - cursorLine
	if distance < 0 {
		distance = -distance
	}
	return washed(numberStyle, bg).Render(pad(strconv.Itoa(distance), width))
}

// washed lays the cursor line's background under a style, leaving it alone
// when there is none.
func washed(s lipgloss.Style, bg color.Color) lipgloss.Style {
	if bg == nil {
		return s
	}
	return s.Background(bg)
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
	indent     int
}

// Render draws height rows of the buffer, scrolling to keep the caret in
// sight. Width and height are the text area, excluding the status bar.
func (e *Editor) Render(width, height int) Rendered {
	width = max(width, 1)
	height = max(height, 1)

	gw := GutterWidth(e.buf.LineCount())
	textWidth := max(width-gw, 1)

	e.refreshMatches()
	classes := classifyBuffer(e.buf)
	rows, cursorRow, cursorCol := e.layout(textWidth)
	e.rows, e.cursorRow = rows, cursorRow
	e.top = scrollTo(e.top, cursorRow, height)

	var out []string
	for i := e.top; i < e.top+height; i++ {
		if i >= len(rows) {
			out = append(out, strings.Repeat(" ", gw))
			continue
		}
		row := rows[i]
		bg := e.cursorLineBG(row.line)
		text, used := e.renderRow(row, classes, textWidth, bg)
		out = append(out, gutter(row.line, e.cursor.Line, gw, row.start == 0, bg)+text+trail(bg, textWidth-used))
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
		indent := continuationIndent(runes, width)
		starts := indentedRowStarts(runes, width, indent)
		for k, s := range starts {
			end := len(runes)
			if k+1 < len(starts) {
				end = starts[k+1]
			}
			rowIndent := 0
			if k > 0 {
				rowIndent = indent
			}
			if i == e.cursor.Line {
				if r, c := cursorRowCol(runes, starts, e.cursor.Col); r == k {
					cursorRow, cursorCol = len(rows), c+rowIndent
				}
			}
			rows = append(rows, vrow{line: i, start: s, end: end, indent: rowIndent})
		}
	}
	return rows, cursorRow, cursorCol
}

// cursorLineBG is the wash to lay under line, nil for any other line and for
// visual modes, where the selection is already the thing to look at.
func (e *Editor) cursorLineBG(line int) color.Color {
	if line != e.cursor.Line || e.mode.Visual() {
		return nil
	}
	return e.cursorLine
}

// trail extends the cursor line's wash to the edge of the window.
func trail(bg color.Color, width int) string {
	if bg == nil || width <= 0 {
		return ""
	}
	return washed(lipgloss.NewStyle(), bg).Render(strings.Repeat(" ", width))
}

// renderRow styles one screen row and reports how wide it came out. It stops
// at width, because the space a line wraps on stays on the row above.
func (e *Editor) renderRow(row vrow, classes [][]tokenClass, width int, bg color.Color) (string, int) {
	runes := e.buf.runes(row.line)
	lineClasses := classes[row.line]
	selFrom, selTo, selLines := e.Selection()

	var sb strings.Builder
	col := row.indent
	if row.indent > 0 {
		sb.WriteString(washed(classStyles[tokPlain], bg).Render(strings.Repeat(" ", row.indent)))
	}
	for i := row.start; i < row.end && i < len(runes); i++ {
		w := runewidth.RuneWidth(runes[i])
		if col+w > width {
			break
		}
		style := washed(classStyles[lineClasses[i]], bg)
		switch {
		case e.flashCovers(Pos{row.line, i}):
			style = e.flashStyle
		case e.selected(Pos{row.line, i}, selFrom, selTo, selLines):
			style = e.selectionStyle()
		case e.matchCovers(Pos{row.line, i}):
			style = e.matchStyle
		}
		sb.WriteString(style.Render(string(runes[i])))
		col += w
	}
	return sb.String(), col
}

// selected reports whether p is highlighted, which depends on which visual
// mode is up. A block selection is a rectangle, not a run of text.
func (e *Editor) selected(p, from, to Pos, linewise bool) bool {
	switch e.mode {
	case ModeVisualBlock:
		return inRange(p, from, to, false, true)
	case ModeVisual, ModeVisualLine:
		return inRange(p, from, to, linewise, false)
	default:
		return false
	}
}

// inRange covers the three shapes a highlight can take: a rectangle, whole
// lines, or a run of characters.
func inRange(p, from, to Pos, linewise, block bool) bool {
	if block {
		lo, hi := blockCols(from, to)
		return p.Line >= from.Line && p.Line <= to.Line && p.Col >= lo && p.Col <= hi
	}
	return inSelection(p, from, to, linewise)
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
