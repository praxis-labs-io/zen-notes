package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/praxis-labs-io/zen-notes/internal/editor"
)

const chromeRows = 1

var (
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	dirtyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	helpDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

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

func (m *Model) cursor(r editor.Rendered) *tea.Cursor {
	if cmd := m.ed.CommandLine(); cmd != "" {
		c := tea.NewCursor(ansi.StringWidth(cmd), m.textHeight())
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
			{"h j k l / arrows", "logical row"},
			{"gj gk", "display row"},
			{"w b e", "by word"},
			{"0 ^ $ / home end", "line ends"},
			{"gg G", "buffer ends"},
			{"{ }", "paragraphs"},
			{"f t F T", "find in line"},
			{"; ,", "repeat find"},
			{"/ n N", "search, next, prev"},
			{"%", "matching bracket"},
			{"gx", "open Markdown link"},
			{"H M L", "top, middle, bottom"},
			{"zt zz zb", "scroll line to"},
			{"ctrl+d/u", "half page"},
			{"pgup/pgdn", "page"},
		}},
	},
	{
		{"Edit", [][2]string{
			{"enter backspace", "continue, exit list"},
			{"tab shift+tab", "nest, unnest list"},
			{"d c y / dd cc yy", "motion / line"},
			{"iw aw", "word object"},
			{"i\" i( ip", "quote, paren, para"},
			{"x del D C / r s S", "cut / replace"},
			{"p P J", "paste, join lines"},
			{"cmd+c / paste", "system clipboard"},
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

func helpLines(width, height int) []string {
	if width < 60 {
		return fit(renderHelpColumn(slices.Concat(helpColumns[0], helpColumns[1]), width-2), height)
	}

	colWidth := width / 2
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

func renderHelpColumn(groups []helpGroup, width int) []string {
	keyWidth := 0
	for _, g := range groups {
		for _, k := range g.keys {
			keyWidth = max(keyWidth, ansi.StringWidth(k[0])+2)
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

func fit(rows []string, height int) []string {
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows[:height]
}

func pad(s string, width int) string {
	if gap := width - ansi.StringWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s + " "
}
