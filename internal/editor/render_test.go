package editor

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lucasb-eyer/go-colorful"
)

// classLetters gives each token class a letter so a whole line's
// classification reads as a string the same length as the line.
var classLetters = map[tokenClass]rune{
	tokPlain:     '.',
	tokHeading:   'H',
	tokStrong:    'B',
	tokEmphasis:  'I',
	tokCode:      'C',
	tokMarker:    'M',
	tokCheckDone: 'X',
	tokCheckTodo: 'O',
	tokLink:      'L',
}

func classString(classes []tokenClass) string {
	var sb strings.Builder
	for _, c := range classes {
		sb.WriteRune(classLetters[c])
	}
	return sb.String()
}

func TestClassifyLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"plain text", "plain", "....."},
		{"atx heading", "# Title", "HHHHHHH"},
		{"deeper heading", "### Sub", "HHHHHHH"},
		{"hash without space is not a heading", "#nope", "....."},
		{"bold", "**bo**", "BBBBBB"},
		{"italic with stars", "*it*", "IIII"},
		{"italic with underscores", "_it_", "IIII"},
		{"inline code", "`c`", "CCC"},
		{"bold inside text", "a **b** c", "..BBBBB.."},
		{"dash bullet", "- item", "MM...."},
		{"star bullet", "* item", "MM...."},
		{"numbered list", "1. item", "MMM...."},
		{"indented bullet", "  - item", "..MM...."},
		{"blockquote", "> quote", "MM....."},
		{"unchecked box", "- [ ] task", "MMOOO....."},
		{"checked box", "- [x] task", "MMXXX....."},
		{"link", "[text](url)", "LLLLLLLLLLL"},
		{"fence marker", "```go", "MMMMM"},
		{"empty line", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classString(classifyLine([]rune(tt.line), false))
			if got != tt.want {
				t.Errorf("classify(%q)\n got %q\nwant %q", tt.line, got, tt.want)
			}
		})
	}
}

// Emphasis must still be found after a bold or code span earlier in the line,
// where a naive left-to-right scan pairs the wrong stars.
func TestClassifyEmphasisAfterOtherSpans(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"after bold", "**b** *i*", "BBBBB.III"},
		{"after code", "`c` *i*", "CCC.III"},
		{"after bold and code", "**b** `c` *i*", "BBBBB.CCC.III"},
		{"between bold spans", "**a** *i* **b**", "BBBBB.III.BBBBB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classString(classifyLine([]rune(tt.line), false))
			if got != tt.want {
				t.Errorf("classify(%q)\n got %q\nwant %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestClassifyInsideFenceIsAllCode(t *testing.T) {
	got := classString(classifyLine([]rune("# not a heading"), true))
	if got != strings.Repeat("C", 15) {
		t.Fatalf("got %q, want all code", got)
	}
}

func TestClassifyBufferTracksFenceState(t *testing.T) {
	b := NewBuffer("# head\n```\n# in code\n```\n# head")
	got := classifyBuffer(b)

	if classString(got[0]) != "HHHHHH" {
		t.Errorf("line 0 = %q, want heading", classString(got[0]))
	}
	if classString(got[2]) != strings.Repeat("C", 9) {
		t.Errorf("line 2 = %q, want all code", classString(got[2]))
	}
	if classString(got[4]) != "HHHHHH" {
		t.Errorf("line 4 = %q, want heading again", classString(got[4]))
	}
}

func TestClassifyUnclosedFenceRunsToTheEnd(t *testing.T) {
	b := NewBuffer("```\nstill code\nmore code")
	got := classifyBuffer(b)
	if classString(got[2]) != strings.Repeat("C", 9) {
		t.Fatalf("line 2 = %q, want all code", classString(got[2]))
	}
}

func TestRowStarts(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  []int
	}{
		{"fits on one row", "hello", 10, []int{0}},
		{"empty line", "", 10, []int{0}},
		{"breaks at a space", "hello world", 5, []int{0, 6}},
		{"breaks mid word when it must", "aaaaaaa", 3, []int{0, 3, 6}},
		{"wraps at each space", "ab cd ef", 3, []int{0, 3, 6}},
		{"keeps a long word whole until the row is full", "hi enormouslylong", 6, []int{0, 3, 9, 15}},
		{"zero width degrades to one row", "hello", 0, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rowStarts([]rune(tt.line), tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("rowStarts = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("rowStarts = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRowStartsAlwaysAdvances(t *testing.T) {
	got := rowStarts([]rune("日本語"), 1)
	if len(got) != 3 {
		t.Fatalf("rowStarts = %v, want one row per wide rune", got)
	}
}

func TestCursorRowAndColumn(t *testing.T) {
	starts := rowStarts([]rune("hello world"), 5)
	tests := []struct {
		col     int
		wantRow int
		wantCol int
	}{
		{0, 0, 0},
		{4, 0, 4},
		{6, 1, 0},
		{10, 1, 4},
		{11, 1, 5},
	}
	for _, tt := range tests {
		row, col := cursorRowCol([]rune("hello world"), starts, tt.col, 0)
		if row != tt.wantRow || col != tt.wantCol {
			t.Errorf("cursorRowCol(%d) = %d, %d; want %d, %d", tt.col, row, col, tt.wantRow, tt.wantCol)
		}
	}
}

func TestCursorColumnCountsDisplayWidth(t *testing.T) {
	runes := []rune("日本x")
	_, col := cursorRowCol(runes, []int{0}, 2, 0)
	if col != 4 {
		t.Fatalf("col = %d, want 4", col)
	}

	runes = []rune("\talpha")
	_, col = cursorRowCol(runes, []int{0}, 2, 0)
	if col != tabWidth+1 {
		t.Fatalf("tabbed col = %d, want %d", col, tabWidth+1)
	}
}

func TestRenderReportsTheCursorPosition(t *testing.T) {
	e := New("hello\nworld")
	feed(t, e, "jll")

	got := e.Render(20, 10)
	wantCol := 2 + GutterWidth(2)
	if got.CursorRow != 1 || got.CursorCol != wantCol {
		t.Fatalf("cursor = %d, %d; want 1, %d", got.CursorRow, got.CursorCol, wantCol)
	}
}

func TestRenderCursorFollowsAWrappedLine(t *testing.T) {
	e := New("hello world")
	feed(t, e, "$")

	got := e.Render(5+GutterWidth(1), 10)
	wantCol := 4 + GutterWidth(1)
	if got.CursorRow != 1 || got.CursorCol != wantCol {
		t.Fatalf("cursor = %d, %d; want 1, %d", got.CursorRow, got.CursorCol, wantCol)
	}
}

func TestRenderCursorIsRelativeToTheScrolledWindow(t *testing.T) {
	e := New(strings.Repeat("x\n", 40))
	feed(t, e, "G")

	got := e.Render(20, 10)
	if got.CursorRow < 0 || got.CursorRow >= 10 {
		t.Fatalf("cursor row = %d, want inside a 10 row window", got.CursorRow)
	}
}

func TestWhitespaceOnlyLinesUseOnlyRequiredRows(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		cursor        Pos
		wantCursorRow int
		wantAfterRow  int
	}{
		{"inactive spaces above", strings.Repeat(" ", 16) + "\nafter", Pos{1, 0}, 1, 1},
		{"inactive spaces below", "bef\n" + strings.Repeat(" ", 16) + "\nafter", Pos{0, 0}, 0, 2},
		{"active spaces at start", strings.Repeat(" ", 12) + "\nafter", Pos{0, 0}, 0, 1},
		{"active spaces at tail", strings.Repeat(" ", 12) + "\nafter", Pos{0, 11}, 3, 4},
		{"inactive tabs above", "\t\t\t\nafter", Pos{1, 0}, 1, 1},
		{"active tabs at tail", "\t\t\t\nafter", Pos{0, 2}, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.text)
			e.SetCursor(tt.cursor)
			gw := GutterWidth(e.buf.LineCount())
			got := e.Render(gw+4, 12)
			if got.CursorRow != tt.wantCursorRow {
				t.Errorf("cursor row = %d, want %d", got.CursorRow, tt.wantCursorRow)
			}
			rows := strings.Split(ansi.Strip(got.Content), "\n")
			if !strings.Contains(rows[tt.wantAfterRow], "afte") {
				t.Errorf("row %d = %q, want the following line", tt.wantAfterRow, rows[tt.wantAfterRow])
			}
			if e.Text() != tt.text {
				t.Errorf("Text = %q, want unchanged %q", e.Text(), tt.text)
			}
			for i, row := range strings.Split(got.Content, "\n") {
				if width := ansi.StringWidth(row); width > gw+4 {
					t.Errorf("row %d is %d cells wide, want at most %d", i, width, gw+4)
				}
			}
		})
	}
}

func TestWhitespaceTailExpandsForEditing(t *testing.T) {
	spaces := strings.Repeat(" ", 12)
	e := New(spaces + "\nafter")
	feed(t, e, "$aX")

	if e.Text() != spaces+"X\nafter" {
		t.Fatalf("Text = %q, want insertion at the real whitespace tail", e.Text())
	}
	if e.Cursor() != (Pos{0, 13}) {
		t.Fatalf("Cursor = %v, want the position after the inserted rune", e.Cursor())
	}
	if got := ansi.Strip(e.Render(GutterWidth(2)+4, 8).Content); !strings.Contains(got, "X") {
		t.Fatalf("inserted tail is not visible: %q", got)
	}
}

func TestWhitespaceCompactionClampsAStaleViewport(t *testing.T) {
	e := New(strings.Repeat(" ", 40) + "\nafter")
	gw := GutterWidth(2)
	e.SetCursor(Pos{0, 39})
	e.Render(gw+4, 3)

	e.SetCursor(Pos{1, 0})
	got := e.Render(gw+4, 3)
	if e.Top() != 0 || got.CursorRow != 1 {
		t.Fatalf("top, cursor row = %d, %d; want 0, 1", e.Top(), got.CursorRow)
	}
	rows := strings.Split(ansi.Strip(got.Content), "\n")
	if !strings.HasPrefix(rows[0], " 1 ") || !strings.HasPrefix(rows[1], " 2 afte") {
		t.Fatalf("hybrid gutters after compaction = %q", rows[:2])
	}
}

func TestWhitespaceCompactionKeepsScreenMotionsOnVisibleLines(t *testing.T) {
	e := New("one\n" + strings.Repeat(" ", 16) + "\ntwo\nthree")
	e.SetHeight(3)
	e.Render(GutterWidth(4)+4, 3)
	feed(t, e, "L")
	if e.Cursor().Line != 2 {
		t.Fatalf("L moved to line %d, want visible line 2", e.Cursor().Line)
	}
}

func TestWhitespaceCompactionKeepsVisualLineSelection(t *testing.T) {
	e := New(strings.Repeat(" ", 16) + "\nafter")
	e.SetBackground(navy)
	feed(t, e, "Vj")
	rows := renderRows(e, GutterWidth(2)+4, 3)
	if !isLit(rows[0]) || !isLit(rows[1]) {
		t.Fatalf("compacted selection is not visible: %q", rows[:2])
	}
	if !strings.Contains(ansi.Strip(rows[1]), "afte") {
		t.Fatalf("following line did not move next to compacted whitespace: %q", ansi.Strip(rows[1]))
	}
}

func TestRenderCursorCountsDisplayWidth(t *testing.T) {
	e := New("日本x")
	feed(t, e, "ll")

	got := e.Render(20, 10)
	wantCol := 4 + GutterWidth(1)
	if got.CursorCol != wantCol {
		t.Fatalf("cursor col = %d, want %d", got.CursorCol, wantCol)
	}
}

func TestDisplayLineMotionsUseRenderedColumns(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		textWidth    int
		beforeRender string
		keys         string
		want         Pos
	}{
		{"wide rune", "a界xabc", 4, "l", "gj", Pos{0, 4}},
		{"tab", "\tabc", tabWidth, "", "gj", Pos{0, 1}},
		{"hanging indent", "- abcdefgh", 6, "", "gjgj", Pos{0, 6}},
		{"omitted separator", "abcd efgh", 4, "lll", "gj", Pos{0, 8}},
		{"compacted whitespace", "abcd\n" + strings.Repeat(" ", 12) + "\nwxyz", 4, "", "2gj", Pos{2, 0}},
		{"stale synthetic row", "abcd\nwxyz", 4, "A", "<esc>gj", Pos{1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.text)
			feed(t, e, tt.beforeRender)
			e.Render(GutterWidth(e.buf.LineCount())+tt.textWidth, 12)
			feed(t, e, tt.keys)
			if got := e.Cursor(); got != tt.want {
				t.Errorf("Cursor = %v, want %v", got, tt.want)
			}
		})
	}
}

// The caret sits one past the last rune in insert mode, on the blank cell.
func TestRenderCursorPastTheLastRune(t *testing.T) {
	e := New("ab")
	feed(t, e, "A")

	got := e.Render(20, 10)
	wantCol := 2 + GutterWidth(1)
	if got.CursorCol != wantCol {
		t.Fatalf("cursor col = %d, want %d", got.CursorCol, wantCol)
	}
}

func TestRenderCursorAtAnExactRightEdge(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		textWidth     int
		wantNormalRow int
		wantNormalCol int
		wantInsertRow int
		wantInsertCol int
	}{
		{"ASCII", "abcd", 4, 0, 3, 1, 0},
		{"wide final rune", "ab界", 4, 0, 2, 1, 0},
		{"tab boundary", "a\t", tabWidth, 0, 1, 1, 0},
		{"omitted trailing separator", "abcd ", 4, 0, 3, 1, 0},
		{"hanging indent", "- abcdefgh", 6, 2, 5, 3, 2},
		{"one-cell text area", "x", 1, 0, 0, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := GutterWidth(1)
			width := gw + tt.textWidth

			normal := run(t, tt.text, "$").Render(width, 10)
			if normal.CursorRow != tt.wantNormalRow || normal.CursorCol != gw+tt.wantNormalCol {
				t.Errorf("normal cursor = %d, %d; want %d, %d", normal.CursorRow, normal.CursorCol, tt.wantNormalRow, gw+tt.wantNormalCol)
			}
			for i, row := range strings.Split(normal.Content, "\n") {
				if got := ansi.StringWidth(row); got > width {
					t.Errorf("normal row %d is %d cells wide, want at most %d", i, got, width)
				}
			}

			insert := run(t, tt.text, "A").Render(width, 10)
			if insert.CursorRow != tt.wantInsertRow || insert.CursorCol != gw+tt.wantInsertCol {
				t.Errorf("insert cursor = %d, %d; want %d, %d", insert.CursorRow, insert.CursorCol, tt.wantInsertRow, gw+tt.wantInsertCol)
			}

			rows := strings.Split(ansi.Strip(insert.Content), "\n")
			if gutter := rows[tt.wantInsertRow][:gw]; strings.TrimSpace(gutter) != "" {
				t.Errorf("synthetic row gutter = %q, want blank", gutter)
			}
			for i, row := range strings.Split(insert.Content, "\n") {
				if got := ansi.StringWidth(row); got > width {
					t.Errorf("row %d is %d cells wide, want at most %d", i, got, width)
				}
			}
		})
	}
}

func TestRenderNoLongerPaintsTheCursor(t *testing.T) {
	got := New("hello").Render(20, 10)

	if !strings.Contains(ansi.Strip(got.Content), "hello") {
		t.Fatal("something is painted into the middle of the text")
	}
	if strings.Contains(got.Content, "\x1b[7m") {
		t.Fatal("the cursor cell is inverted; the terminal's own cursor should show through")
	}
}

func TestGutterShowsHybridLineNumbers(t *testing.T) {
	e := New("one\ntwo\nthree\nfour")
	feed(t, e, "jj")

	rows := strings.Split(ansi.Strip(e.Render(30, 10).Content), "\n")
	want := []string{" 2 one", " 1 two", " 3 three", " 1 four"}
	for i, w := range want {
		if !strings.HasPrefix(rows[i], w) {
			t.Errorf("row %d = %q, want prefix %q", i, rows[i], w)
		}
	}
}

// The gutter is sized from the line count, so the absolute number on the
// cursor's line always fits, at every size, without shifting the text.
func TestGutterFitsTheCurrentLineNumberAtAnyLength(t *testing.T) {
	for _, lines := range []int{9, 10, 99, 100, 250, 1000, 4321} {
		e := New(strings.TrimSuffix(strings.Repeat("x\n", lines), "\n"))
		feed(t, e, "G")

		gw := GutterWidth(lines)
		rows := strings.Split(ansi.Strip(e.Render(40, 3).Content), "\n")
		want := strconv.Itoa(lines)

		cell := rows[len(rows)-1][:gw]
		if strings.TrimSpace(cell) != want {
			t.Errorf("%d lines: gutter cell %q, want the number %q", lines, cell, want)
		}
		if !strings.HasSuffix(cell, " ") {
			t.Errorf("%d lines: gutter cell %q has no space before the text", lines, cell)
		}
	}
}

func TestGutterWidthGrowsWithTheLineCount(t *testing.T) {
	e := New(strings.Repeat("x\n", 150))
	narrow := New("x\ny")

	wide := GutterWidth(e.buf.LineCount())
	if wide <= GutterWidth(narrow.buf.LineCount()) {
		t.Fatalf("gutter did not grow: %d vs %d", wide, GutterWidth(narrow.buf.LineCount()))
	}
}

func TestWrappedMarkdownPreservesItsContentIndent(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"indented prose", "    alpha beta gamma delta", 4},
		{"unordered list", "  - alpha beta gamma delta", 4},
		{"nested list", "    - alpha beta gamma delta", 6},
		{"ordered list", "10. alpha beta gamma delta", 4},
		{"task list", "- [ ] alpha beta gamma delta", 6},
		{"blockquote", "  > alpha beta gamma delta", 4},
		{"nested blockquote", "  > > alpha beta gamma delta", 6},
		{"blockquote list", "> - alpha beta gamma delta", 4},
		{"list blockquote", "- > alpha beta gamma delta", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.text)
			gw := GutterWidth(1)
			rows := strings.Split(ansi.Strip(e.Render(gw+14, 10).Content), "\n")
			if len(rows) < 2 {
				t.Fatal("line did not wrap")
			}
			text := rows[1][gw:]
			if got := len(text) - len(strings.TrimLeft(text, " ")); got != tt.want {
				t.Fatalf("continuation indent = %d, want %d; row %q", got, tt.want, rows[1])
			}
		})
	}
}

func TestNarrowHangingIndentKeepsWideRunes(t *testing.T) {
	e := New("- 日本語")
	got := ansi.Strip(e.Render(GutterWidth(1)+3, 10).Content)
	for _, r := range "日本語" {
		if !strings.ContainsRune(got, r) {
			t.Fatalf("render dropped %q: %q", r, got)
		}
	}
}

func TestWrappedTabIndentUsesTabStops(t *testing.T) {
	e := New("\talpha beta gamma delta")
	gw := GutterWidth(1)
	rows := strings.Split(ansi.Strip(e.Render(gw+14, 10).Content), "\n")
	if len(rows) < 2 {
		t.Fatal("line did not wrap")
	}
	text := rows[1][gw:]
	if got := len(text) - len(strings.TrimLeft(text, " ")); got != tabWidth {
		t.Fatalf("continuation indent = %d, want %d", got, tabWidth)
	}
	if strings.ContainsRune(rows[0], '\t') {
		t.Fatalf("render emitted a literal tab: %q", rows[0])
	}
}

func TestWrappedLinewiseSelectionStylesContinuationIndent(t *testing.T) {
	e := New("- alpha beta gamma delta")
	e.SetBackground(navy)
	feed(t, e, "V")
	rows := renderRows(e, GutterWidth(1)+12, 10)
	if len(rows) < 2 || !regexp.MustCompile(`48;2;[0-9;]+m {2}`).MatchString(rows[1]) {
		t.Fatalf("continuation indent is not selected: %q", rows)
	}

	feed(t, e, "y")
	rows = renderRows(e, GutterWidth(1)+12, 10)
	if !regexp.MustCompile(`48;2;[0-9;]+m {2}`).MatchString(rows[1]) {
		t.Fatalf("continuation indent is not flashed: %q", rows)
	}
}

func TestCursorFollowsAListHangingIndent(t *testing.T) {
	e := New("    - alpha beta gamma")
	feed(t, e, "$")
	gw := GutterWidth(1)
	got := e.Render(gw+14, 10)
	if got.CursorRow != 2 || got.CursorCol != gw+10 {
		t.Fatalf("cursor = %d, %d; want 2, %d", got.CursorRow, got.CursorCol, gw+10)
	}
}

func TestGutterIsBlankOnWrappedRows(t *testing.T) {
	e := New("hello world this wraps")

	rows := strings.Split(ansi.Strip(e.Render(14, 10).Content), "\n")
	if !strings.HasPrefix(rows[0], " 1 ") {
		t.Fatalf("row 0 = %q, want a line number", rows[0])
	}
	if strings.TrimSpace(strings.SplitN(rows[1], " ", 2)[0]) != "" {
		t.Fatalf("row 1 = %q, want a blank gutter on the wrapped row", rows[1])
	}
}

func TestGutterShiftsTheCursor(t *testing.T) {
	e := New("abc")
	got := e.Render(30, 10)
	if got.CursorCol != GutterWidth(1) {
		t.Fatalf("cursor col = %d, want the gutter width %d", got.CursorCol, GutterWidth(1))
	}
}

func TestTextWrapsInsideTheGutter(t *testing.T) {
	e := New("aaaa bbbb")
	// Width 14 with a 4 wide gutter leaves 10 columns, so this fits one row.
	rows := strings.Split(ansi.Strip(e.Render(14, 10).Content), "\n")
	if !strings.Contains(rows[0], "aaaa bbbb") {
		t.Fatalf("row 0 = %q, want the whole line", rows[0])
	}
}

// A row must never exceed the width it was given. The wrap point keeps the
// break space on the previous row, which is what used to push it over.
func TestRenderNeverExceedsTheGivenWidth(t *testing.T) {
	texts := []string{
		"A longer line that will wrap around the edge of this narrow window nicely.",
		"aaaa bbbb cccc dddd eeee ffff",
		strings.Repeat("word ", 40),
		"日本語 の テキスト が ここ に あります",
	}
	for _, text := range texts {
		for _, width := range []int{12, 20, 33, 70} {
			e := New(text)
			for i, row := range strings.Split(e.Render(width, 12).Content, "\n") {
				if got := ansi.StringWidth(row); got > width {
					t.Errorf("width %d: row %d is %d wide: %q", width, i, got, ansi.Strip(row))
				}
			}
		}
	}
}

func TestRenderCursorStaysInsideTheWidth(t *testing.T) {
	e := New("aaaa bbbb cccc")
	feed(t, e, "$")
	got := e.Render(14, 10)
	if got.CursorCol >= 14 {
		t.Fatalf("cursor col = %d, want inside a width of 14", got.CursorCol)
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	tests := []struct {
		name             string
		top, cursor, hgt int
		want             int
	}{
		{"already visible", 0, 3, 10, 0},
		{"cursor above the window", 10, 4, 10, 4},
		{"cursor below the window", 0, 12, 10, 3},
		{"never scrolls past the top", 5, 0, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scrollTo(tt.top, tt.cursor, tt.hgt); got != tt.want {
				t.Errorf("scrollTo = %d, want %d", got, tt.want)
			}
		})
	}
}

// The terminal is asked for its background, but the answer can be slow or
// never arrive. Dark is the safer assumption in the meantime.
func TestSelectionAssumesDarkUntilToldOtherwise(t *testing.T) {
	e := New("abcd")
	feed(t, e, "vl")

	dark := e.Render(20, 3).Content
	e.SetDarkBackground(false)
	light := e.Render(20, 3).Content

	if dark == light {
		t.Fatal("the selection ignores the terminal background")
	}
	if !strings.Contains(dark, "237") {
		t.Errorf("default selection = %q, want the dark one", ansi.Strip(dark))
	}
}

// The selection keeps the terminal background's hue, so it reads as part of
// the theme rather than a grey patch laid over it.
func TestSelectionFollowsTheTerminalTheme(t *testing.T) {
	navy := colorful.Color{R: 0.05, G: 0.06, B: 0.14}
	e := New("abcd")
	e.SetBackground(navy)
	feed(t, e, "vl")

	out := e.Render(20, 3).Content
	hex := selectionHex(t, out)
	got, err := colorful.Hex(hex)
	if err != nil {
		t.Fatalf("selection colour %q: %v", hex, err)
	}

	wantH, _, wantL := navy.Hsl()
	gotH, _, gotL := got.Hsl()
	if math.Abs(gotH-wantH) > 1 {
		t.Errorf("hue = %.1f, want the theme's %.1f", gotH, wantH)
	}
	if gotL <= wantL {
		t.Errorf("lightness = %.3f, want lighter than the background %.3f", gotL, wantL)
	}
	if gotL-wantL > 0.3 {
		t.Errorf("lightness jumped %.3f, want a subtle step", gotL-wantL)
	}
}

func TestSelectionLightensDarkThemesAndDarkensLightOnes(t *testing.T) {
	for _, tt := range []struct {
		name    string
		bg      colorful.Color
		lighter bool
	}{
		{"dark theme", colorful.Color{R: 0.05, G: 0.06, B: 0.14}, true},
		{"light theme", colorful.Color{R: 0.97, G: 0.96, B: 0.94}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := New("abcd")
			e.SetBackground(tt.bg)
			feed(t, e, "vl")

			got, err := colorful.Hex(selectionHex(t, e.Render(20, 3).Content))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, _, gotL := got.Hsl()
			_, _, bgL := tt.bg.Hsl()
			if tt.lighter != (gotL > bgL) {
				t.Errorf("selection lightness %.3f against background %.3f", gotL, bgL)
			}
		})
	}
}

// selectionHex pulls the RGB background out of the rendered escape sequence.
func selectionHex(t *testing.T, content string) string {
	t.Helper()
	m := regexp.MustCompile(`\x1b\[48;2;(\d+);(\d+);(\d+)m`).FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no truecolor selection background in %q", content)
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	return fmt.Sprintf("#%02x%02x%02x", n(m[1]), n(m[2]), n(m[3]))
}

// bgHex is the RGB background rendered behind the first occurrence of ch,
// which is how a test names one highlight when several are on screen.
func bgHex(t *testing.T, content, ch string) string {
	t.Helper()
	pattern := `\x1b\[[0-9;]*48;2;(\d+);(\d+);(\d+)m` + regexp.QuoteMeta(ch)
	m := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no background behind %q in %q", ch, content)
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	return fmt.Sprintf("#%02x%02x%02x", n(m[1]), n(m[2]), n(m[3]))
}

func renderRows(e *Editor, width, height int) []string {
	return strings.Split(e.Render(width, height).Content, "\n")
}

func isLit(row string) bool { return strings.Contains(row, "\x1b[48;") }

var navy = colorful.Color{R: 0.05, G: 0.06, B: 0.14}

func TestCursorLineIsLit(t *testing.T) {
	rows := renderRows(New("one\ntwo"), 20, 3)
	if !isLit(rows[0]) {
		t.Errorf("the cursor's row is not lit: %q", rows[0])
	}
	if isLit(rows[1]) {
		t.Errorf("a row without the cursor is lit: %q", rows[1])
	}
}

func TestCursorLineMovesWithTheCursor(t *testing.T) {
	e := run(t, "one\ntwo\nthree", "j")
	rows := renderRows(e, 20, 3)
	if isLit(rows[0]) || !isLit(rows[1]) || isLit(rows[2]) {
		t.Fatalf("the band did not follow the cursor: %q", rows)
	}
}

func TestCursorLineSpansTheFullWidth(t *testing.T) {
	rows := renderRows(New("hi\nthere"), 20, 3)
	if got := ansi.StringWidth(rows[0]); got != 20 {
		t.Errorf("cursor row is %d wide, want the full 20", got)
	}
	if got := ansi.StringWidth(rows[1]); got == 20 {
		t.Errorf("a row without the cursor was padded to the width")
	}
}

func TestCursorLineCoversTheGutter(t *testing.T) {
	rows := renderRows(New("one\ntwo"), 20, 3)
	number, _, _ := strings.Cut(rows[0], "o")
	if !isLit(number) {
		t.Errorf("the line number sits outside the band: %q", number)
	}
}

func TestCursorLineCoversEveryWrappedRow(t *testing.T) {
	e := New("hello world this wraps over rows")
	rows := renderRows(e, 14, 5)
	for i := range 2 {
		if !isLit(rows[i]) {
			t.Errorf("wrapped row %d is not lit: %q", i, rows[i])
		}
	}
}

func TestCursorLineIsOffInVisualMode(t *testing.T) {
	e := run(t, "one\ntwo", "Vj")
	if isLit(renderRows(e, 20, 3)[2]) {
		t.Error("the band is up under a selection")
	}
}

func TestCursorLineIsOnInInsertMode(t *testing.T) {
	e := run(t, "one\ntwo", "i")
	if !isLit(renderRows(e, 20, 3)[0]) {
		t.Error("the band went out in insert mode")
	}
}

// The band is a wash, not a highlight. Syntax has to read through it.
func TestCursorLineKeepsTheSyntaxColour(t *testing.T) {
	e := New("# Heading\nplain")
	if row := renderRows(e, 20, 3)[0]; !strings.Contains(row, "1;35") {
		t.Errorf("the heading lost its bold magenta under the band: %q", row)
	}
}

func TestSelectionAndFlashWinOverTheCursorLine(t *testing.T) {
	e := New("abcd")
	e.SetBackground(navy)
	line := bgHex(t, e.Render(20, 3).Content, "d")

	feed(t, e, "vl")
	if got := bgHex(t, e.Render(20, 3).Content, "a"); got == line {
		t.Errorf("the selection is the same shade as the band: %s", got)
	}
	feed(t, e, "y")
	if got := bgHex(t, e.Render(20, 3).Content, "a"); got == line {
		t.Errorf("the yank flash is the same shade as the band: %s", got)
	}
}

func TestSearchMatchWinsOverTheCursorLine(t *testing.T) {
	e := New("abcd")
	e.SetBackground(navy)
	line := bgHex(t, e.Render(20, 3).Content, "d")

	feed(t, e, "/bc<cr>")
	if got := bgHex(t, e.Render(20, 3).Content, "b"); got == line {
		t.Errorf("the match is the same shade as the band: %s", got)
	}
}

// The band sits under every line you read, so it has to be the quietest of
// the computed shades.
func TestCursorLineIsSubtlerThanTheSelection(t *testing.T) {
	e := New("abcd")
	e.SetBackground(navy)
	band, err := colorful.Hex(bgHex(t, e.Render(20, 3).Content, "a"))
	if err != nil {
		t.Fatalf("band colour: %v", err)
	}

	feed(t, e, "vl")
	selected, err := colorful.Hex(bgHex(t, e.Render(20, 3).Content, "a"))
	if err != nil {
		t.Fatalf("selection colour: %v", err)
	}

	_, _, bgL := navy.Hsl()
	_, _, bandL := band.Hsl()
	_, _, selL := selected.Hsl()
	if bandL-bgL >= selL-bgL {
		t.Errorf("band step %.3f, want less than the selection's %.3f", bandL-bgL, selL-bgL)
	}
	if bandL <= bgL {
		t.Errorf("band lightness %.3f, want a step off the background %.3f", bandL, bgL)
	}
}

func TestCursorLineFollowsTheTerminalTheme(t *testing.T) {
	e := New("abcd")
	e.SetBackground(navy)

	got, err := colorful.Hex(bgHex(t, e.Render(20, 3).Content, "a"))
	if err != nil {
		t.Fatalf("band colour: %v", err)
	}
	wantH, _, _ := navy.Hsl()
	if gotH, _, _ := got.Hsl(); math.Abs(gotH-wantH) > 1 {
		t.Errorf("hue = %.1f, want the theme's %.1f", gotH, wantH)
	}
}

func TestCursorLineDarkensLightThemes(t *testing.T) {
	paper := colorful.Color{R: 0.97, G: 0.96, B: 0.94}
	e := New("abcd")
	e.SetBackground(paper)

	got, err := colorful.Hex(bgHex(t, e.Render(20, 3).Content, "a"))
	if err != nil {
		t.Fatalf("band colour: %v", err)
	}
	_, _, bgL := paper.Hsl()
	if _, _, gotL := got.Hsl(); gotL >= bgL {
		t.Errorf("band lightness %.3f, want darker than the background %.3f", gotL, bgL)
	}
}

func TestYankFlashCoversWhatWasTaken(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		keys    string
		covered []Pos
		clear   []Pos
	}{
		{"yiw covers the word", "foo bar", "wyiw",
			[]Pos{{0, 4}, {0, 6}}, []Pos{{0, 0}, {0, 3}}},
		{"yy covers the whole line", "ab\ncd", "yy",
			[]Pos{{0, 0}, {0, 1}}, []Pos{{1, 0}}},
		{"2yy covers both lines", "ab\ncd\nef", "2yy",
			[]Pos{{0, 0}, {1, 1}}, []Pos{{2, 0}}},
		{"a visual range", "abcdef", "vlly",
			[]Pos{{0, 0}, {0, 2}}, []Pos{{0, 3}}},
		{"a block", "abcd\nabcd", "l<c-v>jy",
			[]Pos{{0, 1}, {1, 1}}, []Pos{{0, 0}, {0, 2}}},
		{"yw across a space", "foo bar", "yw",
			[]Pos{{0, 0}, {0, 3}}, []Pos{{0, 4}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, tt.text, tt.keys)
			if !e.YankFlash() {
				t.Fatal("no flash after a yank")
			}
			for _, p := range tt.covered {
				if !e.flashCovers(p) {
					t.Errorf("%v not flashed", p)
				}
			}
			for _, p := range tt.clear {
				if e.flashCovers(p) {
					t.Errorf("%v flashed but was not yanked", p)
				}
			}
		})
	}
}

func TestDeleteDoesNotFlash(t *testing.T) {
	if run(t, "abc", "dw").YankFlash() {
		t.Fatal("delete flashed; the change is already visible")
	}
}

func TestClearYankFlash(t *testing.T) {
	e := run(t, "foo bar", "yiw")
	e.ClearYankFlash()
	if e.YankFlash() {
		t.Fatal("flash survived being cleared")
	}
	if e.flashCovers(Pos{0, 0}) {
		t.Fatal("cleared flash still covers text")
	}
}

// A stale flash would paint over text that has since moved.
func TestEditClearsAPendingFlash(t *testing.T) {
	e := run(t, "foo bar", "yiw")
	feed(t, e, "x")
	if e.YankFlash() {
		t.Fatal("the flash outlived the edit under it")
	}
}

func TestYankFlashRendersDistinctlyFromSelection(t *testing.T) {
	e := New("abcd")
	e.SetBackground(colorful.Color{R: 0.05, G: 0.06, B: 0.14})

	feed(t, e, "vl")
	selected := e.Render(20, 3).Content
	feed(t, e, "y")
	flashed := e.Render(20, 3).Content

	// Named by the rune they sit behind: the cursor line is also lit here.
	if bgHex(t, selected, "a") == bgHex(t, flashed, "a") {
		t.Fatal("the yank flash is the same colour as the selection")
	}
}
