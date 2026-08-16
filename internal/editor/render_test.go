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
		row, col := cursorRowCol([]rune("hello world"), starts, tt.col)
		if row != tt.wantRow || col != tt.wantCol {
			t.Errorf("cursorRowCol(%d) = %d, %d; want %d, %d", tt.col, row, col, tt.wantRow, tt.wantCol)
		}
	}
}

func TestCursorColumnCountsDisplayWidth(t *testing.T) {
	runes := []rune("日本x")
	_, col := cursorRowCol(runes, []int{0}, 2)
	if col != 4 {
		t.Fatalf("col = %d, want 4", col)
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

func TestRenderCursorCountsDisplayWidth(t *testing.T) {
	e := New("日本x")
	feed(t, e, "ll")

	got := e.Render(20, 10)
	wantCol := 4 + GutterWidth(1)
	if got.CursorCol != wantCol {
		t.Fatalf("cursor col = %d, want %d", got.CursorCol, wantCol)
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

func TestRenderNoLongerPaintsTheCursor(t *testing.T) {
	e := New("hello")
	got := e.Render(20, 10)

	if !strings.Contains(got.Content, "hello") {
		t.Fatal("the cursor is still painted into the content, splitting the text")
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
