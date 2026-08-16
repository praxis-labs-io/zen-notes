package editor

import (
	"strings"
	"testing"
)

// feed parses vim-style key notation so tests read like what you would type.
// Bare runes are literal; <esc>, <cr>, <bs>, <c-d> and friends are named.
func feed(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for len(keys) > 0 {
		if keys[0] == '<' {
			end := strings.IndexByte(keys, '>')
			if end < 0 {
				t.Fatalf("unterminated key name in %q", keys)
			}
			// <lt> is a literal '<', as in vim, so '<' stays typable.
			if keys[1:end] == "lt" {
				e.Feed(Rune('<'))
			} else {
				e.Feed(Named(keys[1:end]))
			}
			keys = keys[end+1:]
			continue
		}
		r, size := decodeRune(keys)
		e.Feed(Rune(r))
		keys = keys[size:]
	}
}

func decodeRune(s string) (rune, int) {
	for i, r := range s {
		if i == 0 {
			return r, len(string(r))
		}
	}
	return 0, 0
}

func run(t *testing.T, text, keys string) *Editor {
	t.Helper()
	e := New(text)
	feed(t, e, keys)
	return e
}

func TestInsertModeTypesText(t *testing.T) {
	e := run(t, "", "ihello<esc>")
	if e.Text() != "hello" {
		t.Fatalf("Text = %q, want hello", e.Text())
	}
	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal", e.Mode())
	}
}

func TestEscapeStepsCursorBack(t *testing.T) {
	e := run(t, "", "iab<esc>")
	if e.Cursor() != (Pos{0, 1}) {
		t.Fatalf("Cursor = %v, want {0 1}", e.Cursor())
	}
}

func TestNormalModeDoesNotInsertText(t *testing.T) {
	e := run(t, "abc", "zq")
	if e.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", e.Text())
	}
}

func TestInsertEntryKeys(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"i inserts before cursor", "bc", "iaX<esc>", "aXbc"},
		{"a inserts after cursor", "ac", "aX<esc>", "aXc"},
		{"I inserts at first non blank", "  bc", "IX<esc>", "  Xbc"},
		{"A appends at line end", "ab", "AX<esc>", "abX"},
		{"o opens a line below", "ab", "oX<esc>", "ab\nX"},
		{"O opens a line above", "ab", "OX<esc>", "X\nab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnterSplitsLineInInsertMode(t *testing.T) {
	e := run(t, "", "iab<cr>cd<esc>")
	if e.Text() != "ab\ncd" {
		t.Fatalf("Text = %q, want ab\\ncd", e.Text())
	}
}

func TestBackspaceInInsertMode(t *testing.T) {
	e := run(t, "", "iabc<bs><esc>")
	if e.Text() != "ab" {
		t.Fatalf("Text = %q, want ab", e.Text())
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	e := run(t, "ab\ncd", "ji<bs><esc>")
	if e.Text() != "abcd" {
		t.Fatalf("Text = %q, want abcd", e.Text())
	}
}

func TestBasicMotions(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want Pos
	}{
		{"l moves right", "abc", "l", Pos{0, 1}},
		{"h moves left", "abc", "llh", Pos{0, 1}},
		{"h stops at line start", "abc", "h", Pos{0, 0}},
		{"l stops at last rune", "abc", "lllll", Pos{0, 2}},
		{"j moves down", "ab\ncd", "j", Pos{1, 0}},
		{"k moves up", "ab\ncd", "jk", Pos{0, 0}},
		{"0 goes to line start", "abc", "ll0", Pos{0, 0}},
		{"caret goes to first non blank", "  abc", "$^", Pos{0, 2}},
		{"dollar goes to last rune", "abc", "$", Pos{0, 2}},
		{"gg goes to first line", "a\nb\nc", "jjgg", Pos{0, 0}},
		{"G goes to last line", "a\nb\nc", "G", Pos{2, 0}},
		{"counted G goes to that line", "a\nb\nc", "2G", Pos{1, 0}},
		{"w moves a word", "foo bar", "w", Pos{0, 4}},
		{"b moves back a word", "foo bar", "$b", Pos{0, 4}},
		{"e moves to word end", "foo bar", "e", Pos{0, 2}},
		{"f finds forward", "hello", "fl", Pos{0, 2}},
		{"t stops before", "hello", "tl", Pos{0, 1}},
		{"F finds backward", "hello", "$Fl", Pos{0, 3}},
		{"T stops after", "hello", "$Tl", Pos{0, 4}},
		{"count repeats motion", "abcdef", "3l", Pos{0, 3}},
		{"brace moves a paragraph", "a\n\nb", "}", Pos{1, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Cursor(); got != tt.want {
				t.Errorf("Cursor = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepeatFind(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want Pos
	}{
		{"semicolon repeats f", "hello world", "fo;", Pos{0, 7}},
		{"comma reverses f", "hello world", "fo;,", Pos{0, 4}},
		{"semicolon repeats F", "hello world", "$Fo;", Pos{0, 4}},
		{"comma reverses F", "hello world", "$Fo;,", Pos{0, 7}},
		{"semicolon repeats t", "a.b.c.d", "t.;", Pos{0, 2}},
		{"counted repeat", "a.b.c.d.e", "t.3;", Pos{0, 6}},
		{"semicolon alone does nothing", "hello", ";", Pos{0, 0}},
		{"repeat past the last match stays put", "hello world", "fo;;", Pos{0, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Cursor(); got != tt.want {
				t.Errorf("Cursor = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepeatFindWithAnOperator(t *testing.T) {
	e := run(t, "hello world", "fod;")
	if e.Text() != "hellrld" {
		t.Fatalf("Text = %q, want hellrld", e.Text())
	}
}

func TestTextObjects(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"diw deletes the word", "foo bar baz", "wdiw", "foo  baz"},
		{"daw takes the trailing space", "foo bar baz", "wdaw", "foo baz"},
		{"ciw from mid word", "foo bar", "lldiw", " bar"},
		{"diw on punctuation", "foo.bar", "lldiw", ".bar"},
		{"diW takes the whole blob", "a foo.bar b", "wdiW", "a  b"},
		{"di\" empties the quotes", `say "hello" now`, `f"di"`, `say "" now`},
		{"da\" takes the quotes too", `say "hello" now`, `f"da"`, "say  now"},
		{"di( empties the parens", "f(a, b)", "lldi(", "f()"},
		{"da( takes the parens", "f(a, b)", "llda(", "f"},
		{"di{ works from inside", "x{a}y", "lldi{", "x{}y"},
		{"di[ works", "x[ab]y", "lldi[", "x[]y"},
		{"cursor on the open paren", "f(a)", "ldi(", "f()"},
		{"nested parens take the inner", "f(g(x))", "fxdi(", "f(g())"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextObjectParagraph(t *testing.T) {
	e := run(t, "a\nb\n\nc\nd", "dip")
	if e.Text() != "\nc\nd" {
		t.Fatalf("Text = %q, want \\nc\\nd", e.Text())
	}
}

func TestChangeTextObjectEntersInsert(t *testing.T) {
	e := run(t, "foo bar", "ciwzap<esc>")
	if e.Text() != "zap bar" {
		t.Fatalf("Text = %q, want zap bar", e.Text())
	}
}

func TestVisualTextObject(t *testing.T) {
	e := run(t, "foo bar baz", "wviwd")
	if e.Text() != "foo  baz" {
		t.Fatalf("Text = %q, want %q", e.Text(), "foo  baz")
	}
}

func TestYankTextObject(t *testing.T) {
	e := run(t, "foo bar", "yiw$p")
	if e.Text() != "foo barfoo" {
		t.Fatalf("Text = %q, want foo barfoo", e.Text())
	}
}

func TestUnknownTextObjectIsHarmless(t *testing.T) {
	e := run(t, "foo bar", "diz")
	if e.Text() != "foo bar" {
		t.Fatalf("Text = %q, want unchanged", e.Text())
	}
}

func TestFailedFindLeavesCursorPut(t *testing.T) {
	e := run(t, "hello", "fz")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want {0 0}", e.Cursor())
	}
}

func TestCursorCannotRestPastLastRuneInNormalMode(t *testing.T) {
	e := run(t, "ab\ncdef", "j$k")
	if e.Cursor() != (Pos{0, 1}) {
		t.Fatalf("Cursor = %v, want {0 1}", e.Cursor())
	}
}

func TestVerticalMotionRemembersColumn(t *testing.T) {
	e := run(t, "abcd\na\nabcd", "$jj")
	if e.Cursor() != (Pos{2, 3}) {
		t.Fatalf("Cursor = %v, want {2 3}", e.Cursor())
	}
}

func TestDeleteOperators(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"dw deletes a word", "foo bar", "dw", "bar"},
		{"d2w deletes two words", "foo bar baz", "d2w", "baz"},
		{"2dw deletes two words", "foo bar baz", "2dw", "baz"},
		{"dd deletes the line", "a\nb\nc", "jdd", "a\nc"},
		{"2dd deletes two lines", "a\nb\nc", "2dd", "c"},
		{"dd on the only line empties it", "abc", "dd", ""},
		{"d$ deletes to line end", "abcdef", "lld$", "ab"},
		{"d0 deletes to line start", "abcdef", "$d0", "f"},
		{"dj deletes both lines", "a\nb\nc", "dj", "c"},
		{"dk deletes both lines", "a\nb\nc", "jjdk", "a"},
		{"df deletes through the target", "hello", "dfl", "lo"},
		{"dt deletes up to the target", "hello", "dtl", "llo"},
		{"x deletes a rune", "abc", "x", "bc"},
		{"3x deletes three runes", "abcde", "3x", "de"},
		{"X deletes the rune before", "abc", "llX", "ac"},
		{"D deletes to line end", "abcdef", "llD", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChangeOperators(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"cw changes a word", "foo bar", "cwbaz<esc>", "baz bar"},
		{"cc clears the line", "a\nb", "ccX<esc>", "X\nb"},
		{"C changes to line end", "abcdef", "llCX<esc>", "abX"},
		{"c$ changes to line end", "abcdef", "llc$X<esc>", "abX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChangeLeavesInsertMode(t *testing.T) {
	e := run(t, "foo bar", "cw")
	if e.Mode() != ModeInsert {
		t.Fatalf("Mode = %v, want insert", e.Mode())
	}
}

func TestYankAndPut(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"yy then p puts the line below", "a\nb", "yyp", "a\na\nb"},
		{"yy then P puts the line above", "a\nb", "jyyP", "a\nb\nb"},
		{"yw then p puts after the cursor", "foo bar", "ywP", "foo foo bar"},
		{"Y yanks the line", "a\nb", "Yjp", "a\nb\na"},
		{"x then p pastes the rune", "ab", "xp", "ba"},
		{"2yy yanks two lines", "a\nb\nc", "2yyGp", "a\nb\nc\na\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeletedTextGoesToTheRegister(t *testing.T) {
	e := run(t, "foo bar", "dw$p")
	if e.Text() != "barfoo " {
		t.Fatalf("Text = %q, want %q", e.Text(), "barfoo ")
	}
}

func TestVisualMode(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"v then d deletes the selection", "abcdef", "vlld", "def"},
		{"v then l is inclusive", "abc", "vld", "c"},
		{"V then d deletes whole lines", "a\nb\nc", "jVd", "a\nc"},
		{"V then j extends by line", "a\nb\nc", "Vjd", "c"},
		{"v then c changes the selection", "abcdef", "vllcX<esc>", "Xdef"},
		{"v then y yanks it", "abc", "vly$p", "abcab"},
		{"v then esc cancels", "abc", "vl<esc>d", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVisualIndent(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"indent one line", "a\nb", "V>", "  a\nb"},
		{"indent two lines", "a\nb", "Vj>", "  a\n  b"},
		{"unindent", "    a\nb", "V<lt>", "  a\nb"},
		{"unindent stops at zero", "a\nb", "V<lt>", "a\nb"},
		{"unindent partial", " a\nb", "V<lt>", "a\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVisualCase(t *testing.T) {
	tests := []struct {
		name string
		text string
		keys string
		want string
	}{
		{"toggle case", "aBc", "vll~", "AbC"},
		{"lowercase", "ABC", "vllu", "abc"},
		{"uppercase", "abc", "vllU", "ABC"},
		{"linewise uppercase", "abc\ndef", "VU", "ABC\ndef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, tt.text, tt.keys).Text(); got != tt.want {
				t.Errorf("Text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVisualJoin(t *testing.T) {
	e := run(t, "a\nb\nc", "VjJ")
	if e.Text() != "a b\nc" {
		t.Fatalf("Text = %q, want %q", e.Text(), "a b\nc")
	}
}

func TestNormalJoin(t *testing.T) {
	e := run(t, "a\nb\nc", "J")
	if e.Text() != "a b\nc" {
		t.Fatalf("Text = %q, want %q", e.Text(), "a b\nc")
	}
}

func TestJoinTrimsLeadingWhitespace(t *testing.T) {
	e := run(t, "a\n    b", "J")
	if e.Text() != "a b" {
		t.Fatalf("Text = %q, want %q", e.Text(), "a b")
	}
}

func TestVisualBlockDelete(t *testing.T) {
	e := run(t, "abcd\nabcd\nabcd", "l<c-v>jjld")
	if e.Text() != "ad\nad\nad" {
		t.Fatalf("Text = %q, want %q", e.Text(), "ad\nad\nad")
	}
}

func TestVisualBlockOnShortLines(t *testing.T) {
	e := run(t, "abcd\nx\nabcd", "l<c-v>jjld")
	if e.Text() != "ad\nx\nad" {
		t.Fatalf("Text = %q, want %q", e.Text(), "ad\nx\nad")
	}
}

func TestVisualBlockInsertReplicates(t *testing.T) {
	e := run(t, "aa\nbb\ncc", "<c-v>jjI>><esc>")
	if e.Text() != ">>aa\n>>bb\n>>cc" {
		t.Fatalf("Text = %q, want %q", e.Text(), ">>aa\n>>bb\n>>cc")
	}
}

func TestVisualBlockAppendReplicates(t *testing.T) {
	e := run(t, "aa\nbb\ncc", "<c-v>jj$A!<esc>")
	if e.Text() != "aa!\nbb!\ncc!" {
		t.Fatalf("Text = %q, want %q", e.Text(), "aa!\nbb!\ncc!")
	}
}

func TestVisualBlockYankAndPut(t *testing.T) {
	e := run(t, "ab\nab", "<c-v>jy$p")
	if e.Text() != "aba\naba" {
		t.Fatalf("Text = %q, want %q", e.Text(), "aba\naba")
	}
}

func TestVisualBlockEscapeCancels(t *testing.T) {
	e := run(t, "abcd\nabcd", "<c-v>jl<esc>d")
	if e.Text() != "abcd\nabcd" {
		t.Fatalf("Text = %q, want unchanged", e.Text())
	}
}

func TestVisualBlockModeName(t *testing.T) {
	e := run(t, "abcd", "<c-v>")
	if e.Mode() != ModeVisualBlock {
		t.Fatalf("Mode = %v, want visual block", e.Mode())
	}
	if e.Mode().String() != "V-BLOCK" {
		t.Fatalf("Mode name = %q, want V-BLOCK", e.Mode().String())
	}
}

func TestVisualSwapEnds(t *testing.T) {
	e := run(t, "abcdef", "llvlo")
	if e.Cursor() != (Pos{0, 2}) {
		t.Fatalf("Cursor = %v, want {0 2}", e.Cursor())
	}
}

func TestVisualSelectionWorksBackward(t *testing.T) {
	e := run(t, "abcdef", "$vhhd")
	if e.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", e.Text())
	}
}

func TestUndoRestoresText(t *testing.T) {
	e := run(t, "abc", "ddu")
	if e.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", e.Text())
	}
}

func TestUndoTreatsOneInsertSessionAsOneStep(t *testing.T) {
	e := run(t, "", "ihello<esc>u")
	if e.Text() != "" {
		t.Fatalf("Text = %q, want empty", e.Text())
	}
}

func TestRedoAfterUndo(t *testing.T) {
	e := run(t, "abc", "ddu<c-r>")
	if e.Text() != "" {
		t.Fatalf("Text = %q, want empty", e.Text())
	}
}

func TestUndoAtTheBottomIsHarmless(t *testing.T) {
	e := run(t, "abc", "uuu")
	if e.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", e.Text())
	}
}

func TestUndoRestoresTheCursor(t *testing.T) {
	e := run(t, "a\nb\nc", "jjddu")
	if e.Cursor() != (Pos{2, 0}) {
		t.Fatalf("Cursor = %v, want {2 0}", e.Cursor())
	}
}

func TestDirtyTracksUnsavedEdits(t *testing.T) {
	e := New("abc")
	if e.Dirty() {
		t.Fatal("a fresh editor is dirty")
	}
	feed(t, e, "x")
	if !e.Dirty() {
		t.Fatal("edit did not mark the editor dirty")
	}
	e.MarkSaved()
	if e.Dirty() {
		t.Fatal("MarkSaved did not clear dirty")
	}
}

func TestMotionAloneDoesNotDirty(t *testing.T) {
	e := New("abc")
	feed(t, e, "llhh")
	if e.Dirty() {
		t.Fatal("motion marked the editor dirty")
	}
}

func TestZZSavesAndQuits(t *testing.T) {
	e := run(t, "abc", "ZZ")
	if !e.QuitRequested() {
		t.Fatal("ZZ did not request quit")
	}
	if !e.TakeSaveRequest() {
		t.Fatal("ZZ did not request save")
	}
}

func TestSingleZDoesNothing(t *testing.T) {
	e := run(t, "abc", "Zx")
	if e.QuitRequested() {
		t.Fatal("Z alone quit the editor")
	}
	if e.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", e.Text())
	}
}

func TestCommandLineQuit(t *testing.T) {
	e := run(t, "abc", ":q<cr>")
	if !e.QuitRequested() {
		t.Fatal("q did not request quit")
	}
}

func TestCommandLineWriteQuit(t *testing.T) {
	e := run(t, "abc", ":wq<cr>")
	if !e.QuitRequested() {
		t.Fatal("wq did not request quit")
	}
	if !e.TakeSaveRequest() {
		t.Fatal("wq did not request save")
	}
}

func TestCommandLineWriteDoesNotQuit(t *testing.T) {
	e := run(t, "abc", ":w<cr>")
	if e.QuitRequested() {
		t.Fatal("w requested quit")
	}
	if !e.TakeSaveRequest() {
		t.Fatal("w did not request save")
	}
}

func TestSaveRequestIsConsumedOnce(t *testing.T) {
	e := run(t, "abc", ":w<cr>")
	e.TakeSaveRequest()
	if e.TakeSaveRequest() {
		t.Fatal("save request survived being taken")
	}
}

func TestUnknownCommandIsReported(t *testing.T) {
	e := run(t, "abc", ":nonsense<cr>")
	if e.QuitRequested() {
		t.Fatal("unknown command quit the editor")
	}
	if e.Message() == "" {
		t.Fatal("unknown command reported nothing")
	}
}

func TestCommandLineEscapeCancels(t *testing.T) {
	e := run(t, "abc", ":q<esc>")
	if e.QuitRequested() {
		t.Fatal("escape still quit")
	}
	if e.Mode() != ModeNormal {
		t.Fatalf("Mode = %v, want normal", e.Mode())
	}
}

func TestCommandLineShowsWhatYouType(t *testing.T) {
	e := run(t, "abc", ":wq")
	if e.CommandLine() != ":wq" {
		t.Fatalf("CommandLine = %q, want :wq", e.CommandLine())
	}
}

func TestPendingKeysShowInProgressCommand(t *testing.T) {
	e := run(t, "abc", "2d")
	if e.PendingKeys() != "2d" {
		t.Fatalf("PendingKeys = %q, want 2d", e.PendingKeys())
	}
}

func TestEscapeClearsAPendingOperator(t *testing.T) {
	e := run(t, "foo bar", "d<esc>w")
	if e.Text() != "foo bar" {
		t.Fatalf("Text = %q, want unchanged", e.Text())
	}
	if e.Cursor() != (Pos{0, 4}) {
		t.Fatalf("Cursor = %v, want {0 4}", e.Cursor())
	}
}

func TestHalfPageScroll(t *testing.T) {
	e := New(strings.Repeat("x\n", 40))
	e.SetHeight(10)
	feed(t, e, "<c-d>")
	if e.Cursor().Line != 5 {
		t.Fatalf("Cursor line = %d, want 5", e.Cursor().Line)
	}
	feed(t, e, "<c-u>")
	if e.Cursor().Line != 0 {
		t.Fatalf("Cursor line = %d, want 0", e.Cursor().Line)
	}
}

func TestSetTextKeepsCursorInBounds(t *testing.T) {
	e := run(t, "aaaa\nbbbb\ncccc", "jj$")
	e.SetText("ab")
	if got := e.Cursor(); got.Line != 0 || got.Col > 1 {
		t.Fatalf("Cursor = %v, want inside a one line buffer", got)
	}
}

func TestSetTextClearsDirty(t *testing.T) {
	e := run(t, "abc", "x")
	e.SetText("reloaded")
	if e.Dirty() {
		t.Fatal("SetText left the editor dirty")
	}
}

func TestArrowKeysMoveInBothModes(t *testing.T) {
	e := run(t, "abc\ndef", "<down><right>")
	if e.Cursor() != (Pos{1, 1}) {
		t.Fatalf("Cursor = %v, want {1 1}", e.Cursor())
	}
	feed(t, e, "i<left><up>")
	if e.Cursor() != (Pos{0, 0}) {
		t.Fatalf("Cursor = %v, want {0 0}", e.Cursor())
	}
}

func TestInsertModeAllowsCursorPastLastRune(t *testing.T) {
	e := run(t, "ab", "A")
	if e.Cursor() != (Pos{0, 2}) {
		t.Fatalf("Cursor = %v, want {0 2}", e.Cursor())
	}
}
