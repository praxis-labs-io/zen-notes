package editor

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestImageLineTarget(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"bare image", "![alt](pic.png)", "pic.png"},
		{"empty alt", "![](pic.png)", "pic.png"},
		{"surrounding space", "  ![alt](pic.png)  ", "pic.png"},
		{"pointy destination", "![alt](<my pic.png>)", "my pic.png"},
		{"title after target", `![alt](pic.png "a title")`, "pic.png"},
		{"absolute path", "![alt](/tmp/pic.png)", "/tmp/pic.png"},
		{"link, not image", "[alt](pic.png)", ""},
		{"text before", "see ![alt](pic.png)", ""},
		{"text after", "![alt](pic.png) see", ""},
		{"escaped bang", `\!\[alt](pic.png)`, ""},
		{"no target", "![alt]()", ""},
		{"unclosed", "![alt](pic.png", ""},
		{"empty line", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := imageLineTarget([]rune(tt.line))
			if tt.want == "" {
				if ok {
					t.Fatalf("imageLineTarget(%q) = %q, want no image", tt.line, got)
				}
				return
			}
			if !ok || got != tt.want {
				t.Fatalf("imageLineTarget(%q) = %q/%v, want %q", tt.line, got, ok, tt.want)
			}
		})
	}
}

// parseInlineLink starts past the bracket, so a caller that skips the check
// would take "!x](pic.png)" for an image.
func TestImageLineTargetRequiresABracket(t *testing.T) {
	for _, line := range []string{"!x](pic.png)", "!(pic.png)", "!!](pic.png)"} {
		if got, ok := imageLineTarget([]rune(line)); ok {
			t.Fatalf("imageLineTarget(%q) = %q, want no image", line, got)
		}
	}
}

func TestImageGridStopsAtTheAddressableLimit(t *testing.T) {
	e := New("![](p.png)")
	e.SetImages(map[string]ImagePlacement{"p.png": {ID: 7, Cols: 400, Rows: 400}})
	rows := strings.Split(e.Render(GutterWidth(1)+500, 400).Content, "\n")

	// A cell past the diacritic table would repeat the first column's mark,
	// and kitty would draw that column again instead of the rest of the image.
	if got := placeholderCount(rows[1]); got != MaxImageCells {
		t.Fatalf("image row has %d placeholder cells, want %d", got, MaxImageCells)
	}
	imageRows := 0
	for _, row := range rows[1:] {
		if placeholderCount(row) > 0 {
			imageRows++
		}
	}
	if imageRows != MaxImageCells {
		t.Fatalf("image reserved %d rows, want %d", imageRows, MaxImageCells)
	}
}

func TestImageRowsTakeNoCursorLineWash(t *testing.T) {
	e := New("![alt](pic.png)\nbelow")
	e.SetBackground(color.Black)
	e.SetImages(map[string]ImagePlacement{"pic.png": {ID: 7, Cols: 4, Rows: 2}})
	rows := strings.Split(e.Render(GutterWidth(2)+20, 12).Content, "\n")

	// The caret is on the image's line, so its own row is washed. The image
	// rows must not be, or the wash frames the picture in the gutter and the
	// trail while leaving the middle bare.
	if !strings.Contains(rows[0], "\x1b[48") {
		t.Fatalf("image line row = %q, want the cursor line wash", rows[0])
	}
	for i := 1; i < 3; i++ {
		if strings.Contains(rows[i], "\x1b[48") {
			t.Fatalf("image row %d = %q, want no cursor line wash", i, rows[i])
		}
	}
}

func TestImageTargetsListsEveryReference(t *testing.T) {
	e := New("![one](a.png)\nplain\n![two](b.png)\nsee ![three](c.png)")
	got := e.ImageTargets()
	want := []string{"a.png", "b.png"}
	if len(got) != len(want) {
		t.Fatalf("ImageTargets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ImageTargets = %v, want %v", got, want)
		}
	}
}

// withImage renders text where target has been given a cols x rows placement.
func withImage(t *testing.T, text, target string, cols, rows int) (*Editor, []string) {
	t.Helper()
	e := New(text)
	e.SetImages(map[string]ImagePlacement{target: {ID: 7, Cols: cols, Rows: rows}})
	got := e.Render(GutterWidth(e.buf.LineCount())+20, 12)
	return e, strings.Split(got.Content, "\n")
}

func placeholderCount(row string) int {
	return strings.Count(ansi.Strip(row), string(kitty.Placeholder))
}

func TestImageReservesRowsBelowItsLine(t *testing.T) {
	_, rows := withImage(t, "above\n![alt](pic.png)\nbelow", "pic.png", 6, 3)

	for i, want := range []string{"above", "![alt](pic.png)"} {
		if !strings.Contains(ansi.Strip(rows[i]), want) {
			t.Fatalf("row %d = %q, want %q", i, ansi.Strip(rows[i]), want)
		}
	}
	for i := 2; i < 5; i++ {
		if got := placeholderCount(rows[i]); got != 6 {
			t.Fatalf("row %d has %d placeholder cells, want 6", i, got)
		}
	}
	if !strings.Contains(ansi.Strip(rows[5]), "below") {
		t.Fatalf("row 5 = %q, want the line after the image", ansi.Strip(rows[5]))
	}
}

func TestImageRowsCarryPositionDiacritics(t *testing.T) {
	_, rows := withImage(t, "![alt](pic.png)", "pic.png", 3, 2)

	for r := range 2 {
		var want strings.Builder
		for c := range 3 {
			want.WriteString(string([]rune{kitty.Placeholder, kitty.Diacritic(r), kitty.Diacritic(c)}))
		}
		if got := ansi.Strip(rows[r+1]); !strings.Contains(got, want.String()) {
			t.Fatalf("image row %d = %q, want the row %d diacritic run", r, got, r)
		}
	}
}

func TestImageRowsAreUnnumbered(t *testing.T) {
	_, rows := withImage(t, "![alt](pic.png)\nbelow", "pic.png", 4, 2)

	// The gutter shows the cursor line's own number, then distances. The image
	// belongs to line 1, so its rows carry no number of their own.
	if got := ansi.Strip(rows[0]); !strings.HasPrefix(got, " 1 ") {
		t.Fatalf("image line gutter = %q, want the line number", got)
	}
	for i := 1; i < 3; i++ {
		if got := ansi.Strip(rows[i])[:3]; strings.TrimSpace(got) != "" {
			t.Fatalf("image row %d gutter = %q, want blank", i, got)
		}
	}
	if got := ansi.Strip(rows[3]); !strings.HasPrefix(got, " 1 ") {
		t.Fatalf("line after image gutter = %q, want a distance of 1", got)
	}
}

func TestImageRowsOnlyAppearForAResolvedPlacement(t *testing.T) {
	e := New("![alt](pic.png)\nbelow")
	rows := strings.Split(e.Render(GutterWidth(2)+20, 12).Content, "\n")
	if got := placeholderCount(rows[1]); got != 0 {
		t.Fatalf("row 1 has %d placeholder cells, want none without a placement", got)
	}
	if !strings.Contains(ansi.Strip(rows[1]), "below") {
		t.Fatalf("row 1 = %q, want the next line", ansi.Strip(rows[1]))
	}

	e.SetImages(map[string]ImagePlacement{"other.png": {ID: 7, Cols: 4, Rows: 2}})
	rows = strings.Split(e.Render(GutterWidth(2)+20, 12).Content, "\n")
	if got := placeholderCount(rows[1]); got != 0 {
		t.Fatalf("row 1 has %d placeholder cells, want none for an unrelated target", got)
	}
}

func TestImageRowsClampToTheTextWidth(t *testing.T) {
	e := New("![](p.png)")
	e.SetImages(map[string]ImagePlacement{"p.png": {ID: 7, Cols: 40, Rows: 1}})
	rows := strings.Split(e.Render(GutterWidth(1)+12, 4).Content, "\n")
	if got := placeholderCount(rows[1]); got != 12 {
		t.Fatalf("image row has %d placeholder cells, want 12 to fit the text width", got)
	}
}

func TestCursorIgnoresImageRows(t *testing.T) {
	e := New("![alt](pic.png)\nbelow")
	e.SetImages(map[string]ImagePlacement{"pic.png": {ID: 7, Cols: 6, Rows: 4}})

	got := e.Render(GutterWidth(2)+20, 12)
	if got.CursorRow != 0 {
		t.Fatalf("CursorRow = %d, want 0 on the image's own line", got.CursorRow)
	}

	feed(t, e, "j")
	got = e.Render(GutterWidth(2)+20, 12)
	if e.Cursor().Line != 1 {
		t.Fatalf("j put the cursor on line %d, want 1", e.Cursor().Line)
	}
	if got.CursorRow != 5 {
		t.Fatalf("CursorRow = %d, want 5, past the four image rows", got.CursorRow)
	}
}

func TestDisplayMotionsStepOverImageRows(t *testing.T) {
	e := New("![alt](pic.png)\nbelow")
	e.SetImages(map[string]ImagePlacement{"pic.png": {ID: 7, Cols: 6, Rows: 4}})
	e.Render(GutterWidth(2)+20, 12)

	feed(t, e, "gj")
	if e.Cursor().Line != 1 {
		t.Fatalf("gj put the cursor on line %d, want 1", e.Cursor().Line)
	}
	feed(t, e, "gk")
	if e.Cursor().Line != 0 {
		t.Fatalf("gk put the cursor on line %d, want 0", e.Cursor().Line)
	}
}

func TestScreenMotionsLandOnLinesNotImageRows(t *testing.T) {
	e := New("![alt](pic.png)\nbelow")
	e.SetImages(map[string]ImagePlacement{"pic.png": {ID: 7, Cols: 6, Rows: 4}})
	e.SetHeight(12)
	e.Render(GutterWidth(2)+20, 12)

	feed(t, e, "L")
	if e.Cursor().Line != 1 {
		t.Fatalf("L put the cursor on line %d, want the last line", e.Cursor().Line)
	}
	feed(t, e, "H")
	if e.Cursor().Line != 0 {
		t.Fatalf("H put the cursor on line %d, want the first line", e.Cursor().Line)
	}
}

func TestImageMarkdownHighlightsAsOneLink(t *testing.T) {
	got := classString(classifyLine([]rune("![alt](pic.png)"), false))
	if want := strings.Repeat("L", 15); got != want {
		t.Fatalf("classifyLine = %q, want %q", got, want)
	}
}
