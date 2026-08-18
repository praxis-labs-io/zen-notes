package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/praxis-labs-io/zen-notes/internal/editor"
)

// chromeRows is the status line, the only row the note does not get.
const chromeRows = 1

var (
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	dirtyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	helpDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// modeStyles colors the indicator per mode, so the mode is readable at a
// glance without reading the word.
var modeStyles = map[editor.Mode]lipgloss.Style{
	editor.ModeNormal:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")),
	editor.ModeInsert:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")),
	editor.ModeVisual:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")),
	editor.ModeVisualLine:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")),
	editor.ModeVisualBlock: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")),
	editor.ModeCommand:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
	editor.ModeSearch:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
}

func (m *Model) textHeight() int { return max(m.height-chromeRows, 1) }

func (m *Model) View() tea.View {
	width, height := max(m.width, 1), m.textHeight()

	var body []string
	var cursor *tea.Cursor
	if m.help {
		body = helpLines(width, height)
	} else {
		r := m.ed.Render(width, height)
		body = strings.Split(r.Content, "\n")
		cursor = m.cursor(r)
	}

	v := tea.NewView(strings.Join(append(body, m.statusBar(width)), "\n"))
	v.AltScreen = true
	v.Cursor = cursor
	return v
}

// statusBar puts the mode bottom left and the note's filename hard right.
func (m *Model) statusBar(width int) string {
	if cmd := m.ed.CommandLine(); cmd != "" {
		return cmd
	}

	left := []string{modeStyles[m.ed.Mode()].Render(m.ed.Mode().String())}
	if m.ed.Dirty() {
		left = append(left, dirtyStyle.Render("●"))
	}
	if m.status != "" {
		left = append(left, messageStyle.Render(m.status))
	}
	if keys := m.ed.PendingKeys(); keys != "" {
		left = append(left, pendingStyle.Render(keys))
	}

	l := strings.Join(left, "  ")
	r := fileStyle.Render(m.day.String() + ".md")
	gap := width - ansi.StringWidth(l) - ansi.StringWidth(r)
	if gap < 1 {
		return l
	}
	return l + strings.Repeat(" ", gap) + r
}

// cursor places the real terminal cursor. Color stays nil so the terminal's
// own applies, and mode picks the pair: blinking bar to type in, steady block.
func (m *Model) cursor(r editor.Rendered) *tea.Cursor {
	if cmd := m.ed.CommandLine(); cmd != "" {
		c := tea.NewCursor(runewidth.StringWidth(cmd), m.textHeight())
		c.Shape = tea.CursorBar
		return c
	}
	c := tea.NewCursor(r.CursorCol, r.CursorRow)
	if m.ed.Mode() == editor.ModeInsert {
		c.Shape = tea.CursorBar
		return c
	}
	c.Shape = tea.CursorBlock
	c.Blink = false
	return c
}

type helpGroup struct {
	group string
	keys  [][2]string
}

// helpColumns is the binding list, split into the two columns it renders as.
// Labels stay terse so both columns fit a narrow window without clipping.
var helpColumns = [2][]helpGroup{
	{
		{"Modes", [][2]string{
			{"i a", "insert, append"},
			{"I A", "line start, end"},
			{"o O", "open below, above"},
			{"v V ctrl+v", "visual/line/block"},
			{"esc", "normal mode"},
		}},
		{"Move", [][2]string{
			{"h j k l", "left down up right"},
			{"arrows", "same as h j k l"},
			{"w b e", "by word"},
			{"0 ^ $", "line ends"},
			{"gg G", "buffer ends"},
			{"{ }", "paragraphs"},
			{"f t F T", "find in line"},
			{"; ,", "repeat find"},
			{"/ n N", "search, next, prev"},
			{"%", "matching bracket"},
			{"H M L", "top, middle, bottom"},
			{"zt zz zb", "scroll line to"},
			{"ctrl+d/u", "half page"},
		}},
	},
	{
		{"Edit", [][2]string{
			{"enter backspace", "continue, exit list"},
			{"tab shift+tab", "nest, unnest list"},
			{"d c y", "+ a motion"},
			{"dd cc yy", "whole line"},
			{"iw aw", "word object"},
			{"i\" i( ip", "quote, paren, para"},
			{"x D C", "cut, to line end"},
			{"r s S", "replace, substitute"},
			{"p P J", "paste, join lines"},
			{">> << ~", "indent, toggle case"},
			{"gU gu g~", "+ a motion"},
			{"gv", "reselect"},
			{"u ctrl+r", "undo, redo"},
		}},
		{"Notes", [][2]string{
			{"[ ] \\", "prev, next, today"},
			{":w :q :wq", "save, quit"},
			{"ZZ", "save and quit"},
			{"?", "close this"},
		}},
	},
}

// helpLines lays the bindings out in two columns, falling back to one when
// the window is too narrow to split.
func helpLines(width, height int) []string {
	if width < 60 {
		return fit(renderHelpColumn(append(helpColumns[0], helpColumns[1]...), width-2), height)
	}

	colWidth := width / 2
	// Two columns of space between them, so the columns can never touch.
	leftRoom := colWidth - 4
	left := renderHelpColumn(helpColumns[0], leftRoom)
	right := renderHelpColumn(helpColumns[1], width-colWidth-2)

	rows := max(len(left), len(right))
	out := make([]string, rows)
	for i := range rows {
		l := ansi.Truncate(at(left, i), leftRoom, "")
		gap := max(colWidth-2-ansi.StringWidth(l), 0)
		out[i] = "  " + l + strings.Repeat(" ", gap) + at(right, i)
	}
	return fit(out, height)
}

// renderHelpColumn turns one column's groups into styled lines, sizing the
// key column to its longest key so every description starts in one place.
func renderHelpColumn(groups []helpGroup, width int) []string {
	keyWidth := 0
	for _, g := range groups {
		for _, k := range g.keys {
			keyWidth = max(keyWidth, runewidth.StringWidth(k[0])+2)
		}
	}
	keyWidth = min(keyWidth, max(width/2, 8))
	var out []string
	for _, g := range groups {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, helpDimStyle.Render(g.group))
		for _, k := range g.keys {
			out = append(out, helpKeyStyle.Render(pad(k[0], keyWidth))+helpDimStyle.Render(k[1]))
		}
	}
	return out
}

func at(rows []string, i int) string {
	if i < len(rows) {
		return rows[i]
	}
	return ""
}

// fit pads or clips a block to exactly height rows.
func fit(rows []string, height int) []string {
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows[:height]
}

func pad(s string, width int) string {
	if gap := width - runewidth.StringWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s + " "
}
