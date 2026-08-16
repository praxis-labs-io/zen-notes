package editor

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSearchJumpsToTheFirstMatchAhead(t *testing.T) {
	e := run(t, "alpha\nbeta\ngamma beta", "/beta<cr>")
	if e.Cursor() != (Pos{1, 0}) {
		t.Fatalf("Cursor = %v, want {1 0}", e.Cursor())
	}
}

func TestSearchWrapsAround(t *testing.T) {
	e := run(t, "beta\nalpha\ngamma", "G/beta<cr>")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want {0 0}", e.Cursor())
	}
	if !strings.Contains(e.Message(), "wrapped") {
		t.Fatalf("Message = %q, want it to mention wrapping", e.Message())
	}
}

func TestSearchStartsAfterTheCursor(t *testing.T) {
	e := run(t, "beta beta beta", "/beta<cr>")
	if e.Cursor() != (Pos{0, 5}) {
		t.Fatalf("Cursor = %v, want the next match, not the one under the caret", e.Cursor())
	}
}

func TestNextAndPreviousMatch(t *testing.T) {
	e := run(t, "x beta y beta z beta", "/beta<cr>")
	if e.Cursor() != (Pos{0, 2}) {
		t.Fatalf("first match = %v, want {0 2}", e.Cursor())
	}
	feed(t, e, "n")
	if e.Cursor() != (Pos{0, 9}) {
		t.Fatalf("after n = %v, want {0 9}", e.Cursor())
	}
	feed(t, e, "n")
	if e.Cursor() != (Pos{0, 16}) {
		t.Fatalf("after nn = %v, want {0 16}", e.Cursor())
	}
	feed(t, e, "N")
	if e.Cursor() != (Pos{0, 9}) {
		t.Fatalf("after N = %v, want {0 9}", e.Cursor())
	}
}

func TestNextMatchWithoutASearchIsHarmless(t *testing.T) {
	e := run(t, "abc", "n")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want {0 0}", e.Cursor())
	}
}

func TestSearchIsCaseInsensitiveByDefault(t *testing.T) {
	e := run(t, "alpha\nBeta", "/beta<cr>")
	if e.Cursor() != (Pos{1, 0}) {
		t.Fatalf("Cursor = %v, want a lowercase pattern to find Beta", e.Cursor())
	}
}

// An uppercase letter in the pattern means you meant it, as smartcase does.
func TestUppercaseInThePatternMakesItCaseSensitive(t *testing.T) {
	e := run(t, "beta\nBeta", "/Beta<cr>")
	if e.Cursor() != (Pos{1, 0}) {
		t.Fatalf("Cursor = %v, want the capitalised match", e.Cursor())
	}
}

func TestSearchMissSaysSo(t *testing.T) {
	e := run(t, "alpha\nbeta", "/nothing<cr>")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want it left alone", e.Cursor())
	}
	if !strings.Contains(e.Message(), "not found") {
		t.Fatalf("Message = %q, want a miss reported", e.Message())
	}
}

func TestEscapeCancelsTheSearch(t *testing.T) {
	e := run(t, "alpha\nbeta", "/beta<esc>")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want it left alone", e.Cursor())
	}
	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal", e.Mode())
	}
	if e.SearchPattern() != "" {
		t.Fatalf("pattern = %q, want the cancelled search forgotten", e.SearchPattern())
	}
}

func TestSearchLineShowsWhatYouType(t *testing.T) {
	e := run(t, "abc", "/be")
	if e.CommandLine() != "/be" {
		t.Fatalf("CommandLine = %q, want /be", e.CommandLine())
	}
}

func TestBackspaceInTheSearchLine(t *testing.T) {
	e := run(t, "abc", "/bex<bs>")
	if e.CommandLine() != "/be" {
		t.Fatalf("CommandLine = %q, want /be", e.CommandLine())
	}
}

func TestEmptySearchRepeatsTheLast(t *testing.T) {
	e := run(t, "beta x beta", "/beta<cr>0/<cr>")
	if e.Cursor() != (Pos{0, 7}) {
		t.Fatalf("Cursor = %v, want an empty pattern to reuse the last", e.Cursor())
	}
}

func TestMatchesAreHighlighted(t *testing.T) {
	e := run(t, "beta and beta", "/beta<cr>")
	for _, p := range []Pos{{0, 0}, {0, 3}, {0, 9}, {0, 12}} {
		if !e.matchCovers(p) {
			t.Errorf("%v not highlighted", p)
		}
	}
	for _, p := range []Pos{{0, 4}, {0, 8}} {
		if e.matchCovers(p) {
			t.Errorf("%v highlighted but is not a match", p)
		}
	}
}

func TestHighlightSurvivesUntilDismissed(t *testing.T) {
	e := run(t, "beta and beta", "/beta<cr>")
	feed(t, e, "j")
	if !e.matchCovers(Pos{0, 0}) {
		t.Fatal("moving dropped the highlight")
	}

	feed(t, e, "<esc>")
	if e.matchCovers(Pos{0, 0}) {
		t.Fatal("escape did not dismiss the highlight")
	}
	if e.SearchPattern() != "" {
		t.Fatalf("pattern = %q, want it cleared", e.SearchPattern())
	}
}

func TestHighlightIsVisibleInTheRender(t *testing.T) {
	plain := New("beta and beta").Render(30, 3).Content
	e := run(t, "beta and beta", "/beta<cr>")

	got := e.Render(30, 3).Content
	if got == plain {
		t.Fatal("matches are not styled")
	}
	if !strings.Contains(ansi.Strip(got), "beta and beta") {
		t.Fatal("highlighting mangled the text")
	}
}

func TestSearchFindsAcrossLines(t *testing.T) {
	e := run(t, "one\ntwo\nthree\nfour", "/three<cr>")
	if e.Cursor() != (Pos{2, 0}) {
		t.Fatalf("Cursor = %v, want {2 0}", e.Cursor())
	}
}

func TestSearchModeName(t *testing.T) {
	e := run(t, "abc", "/x")
	if e.Mode() != ModeSearch {
		t.Fatalf("Mode = %v, want search", e.Mode())
	}
}

func TestSearchIsIncremental(t *testing.T) {
	e := New("alpha\nbeta\ngamma")
	feed(t, e, "/bet")

	if e.Cursor() != (Pos{1, 0}) {
		t.Fatalf("Cursor = %v, want it to follow along as you type", e.Cursor())
	}
	if !e.matchCovers(Pos{1, 0}) {
		t.Fatal("no live highlight while typing")
	}
}

func TestIncrementalSearchFollowsEachKeystroke(t *testing.T) {
	e := New("aaa\nbbb\nccc")
	feed(t, e, "/c")
	if e.Cursor().Line != 2 {
		t.Fatalf("line = %d, want 2", e.Cursor().Line)
	}
	feed(t, e, "<bs>b")
	if e.Cursor().Line != 1 {
		t.Fatalf("line = %d, want 1 after retyping", e.Cursor().Line)
	}
}

func TestBackspacingToEmptyReturnsToTheStart(t *testing.T) {
	e := New("alpha\nbeta")
	feed(t, e, "/beta")
	if e.Cursor().Line != 1 {
		t.Fatalf("line = %d, want 1", e.Cursor().Line)
	}
	feed(t, e, "<bs><bs><bs><bs>")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want back where the search started", e.Cursor())
	}
}

func TestCancellingPutsTheCursorBack(t *testing.T) {
	e := run(t, "alpha\nbeta\ngamma", "jj")
	start := e.Cursor()
	feed(t, e, "/alpha")
	if e.Cursor() == start {
		t.Fatal("the preview never moved")
	}

	feed(t, e, "<esc>")
	if e.Cursor() != start {
		t.Fatalf("Cursor = %v, want %v restored", e.Cursor(), start)
	}
}

func TestIncrementalSearchDoesNotFlashWrapped(t *testing.T) {
	e := New("beta\nalpha")
	feed(t, e, "G/beta")
	if e.Message() != "" {
		t.Fatalf("Message = %q, want the preview to stay quiet", e.Message())
	}
	feed(t, e, "<cr>")
	if !strings.Contains(e.Message(), "wrapped") {
		t.Fatalf("Message = %q, want wrapping reported on commit", e.Message())
	}
}

func TestMatchesAreNotBold(t *testing.T) {
	e := run(t, "beta", "/beta<cr>")
	if strings.Contains(e.Render(20, 3).Content, "\x1b[1m") {
		t.Fatal("matches are bold; the heavier glyph misaligns with its cell")
	}
}
