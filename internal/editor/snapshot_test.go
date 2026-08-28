package editor

import (
	"strings"
	"testing"
)

// snapshotText joins a snapshot the way Buffer.Text would, so a test can say
// what the undo stack still holds.
func snapshotText(lines [][]rune) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// Lines shares the line slices rather than copying them, which is only safe
// because every mutator replaces a line header instead of writing through it.
// One that writes in place would corrupt the undo stack, so each gets a case.
func TestSnapshotSurvivesBufferMutation(t *testing.T) {
	const text = "one two\nthree four\nfive six"

	tests := []struct {
		name   string
		mutate func(*Buffer)
	}{
		{"insert within a line", func(b *Buffer) { b.Insert(Pos{1, 2}, "XY") }},
		{"insert splitting a line", func(b *Buffer) { b.Insert(Pos{1, 5}, "\n") }},
		{"insert multiple lines", func(b *Buffer) { b.Insert(Pos{0, 3}, "a\nb\nc") }},
		{"insert at the end", func(b *Buffer) { b.Insert(b.End(), "tail") }},
		{"delete within a line", func(b *Buffer) { b.Delete(Pos{0, 1}, Pos{0, 4}) }},
		{"delete across lines", func(b *Buffer) { b.Delete(Pos{0, 2}, Pos{2, 3}) }},
		{"delete everything", func(b *Buffer) { b.Delete(Pos{}, b.End()) }},
		{"replace one line", func(b *Buffer) { b.ReplaceLines(1, 2, []string{"gone"}) }},
		{"replace a range with fewer", func(b *Buffer) { b.ReplaceLines(0, 3, []string{"only"}) }},
		{"replace a range with more", func(b *Buffer) { b.ReplaceLines(0, 1, []string{"a", "b", "c"}) }},
		{"replace with nothing", func(b *Buffer) { b.ReplaceLines(0, 3, nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(text)
			snap := b.Lines()
			tt.mutate(b)
			if got := snapshotText(snap); got != text {
				t.Fatalf("snapshot = %q, want %q", got, text)
			}
		})
	}
}

// The same contract from the other side: every editor command that rewrites
// runes has to leave the snapshot it just took alone.
func TestSnapshotSurvivesEditorCommand(t *testing.T) {
	const text = "one two\nthree four\nfive six"

	tests := []struct {
		name string
		keys string
		text string // defaults to the text above
	}{
		{name: "replace under cursor", keys: "rz"},
		{name: "replace several", keys: "3rz"},
		{name: "toggle case", keys: "~"},
		{name: "toggle case with a count", keys: "5~"},
		{name: "upper over a motion", keys: "gUw"},
		{name: "upper over a line", keys: "gUU"},
		{name: "toggle over a visual range", keys: "vjj~"},
		{name: "upper over a visual line", keys: "VU"},
		{name: "upper over a visual block", keys: "<c-v>jjllU"},
		{name: "indent", keys: ">>"},
		{name: "dedent", keys: "<lt><lt>", text: "\tone two\n\tthree four"},
		{name: "delete a word", keys: "dw"},
		{name: "delete a line", keys: "dd"},
		{name: "delete across lines", keys: "d2j"},
		{name: "change a word", keys: "cwzap<esc>"},
		{name: "open a line", keys: "ozap<esc>"},
		{name: "join lines", keys: "J"},
		{name: "insert text", keys: "izap<esc>"},
		{name: "append at the end", keys: "Azap<esc>"},
		{name: "delete a rune", keys: "x"},
		{name: "put a yanked line", keys: "yyp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.text
			if want == "" {
				want = text
			}
			e := New(want)
			e.snapshot()
			snap := e.undo[len(e.undo)-1].lines

			feed(t, e, tt.keys)

			if e.Text() == want {
				t.Fatalf("keys %q changed nothing, so the case proves nothing", tt.keys)
			}
			if got := snapshotText(snap); got != want {
				t.Fatalf("snapshot = %q, want %q", got, want)
			}
		})
	}
}

// SetLines clones so a restored buffer never shares its array with a snapshot
// still on a stack. Undo, edit, redo, undo exercises both stacks in turn.
func TestUndoRedoRoundTripKeepsItsHistory(t *testing.T) {
	e := New("one\ntwo\nthree")

	feed(t, e, "dd")
	if e.Text() != "two\nthree" {
		t.Fatalf("after dd, Text = %q", e.Text())
	}

	feed(t, e, "u")
	if e.Text() != "one\ntwo\nthree" {
		t.Fatalf("after u, Text = %q", e.Text())
	}

	feed(t, e, "x")
	if e.Text() != "ne\ntwo\nthree" {
		t.Fatalf("after x, Text = %q", e.Text())
	}

	feed(t, e, "u")
	if e.Text() != "one\ntwo\nthree" {
		t.Fatalf("after the second u, Text = %q", e.Text())
	}

	feed(t, e, "<c-r>")
	if e.Text() != "ne\ntwo\nthree" {
		t.Fatalf("after c-r, Text = %q", e.Text())
	}

	feed(t, e, "uu")
	if e.Text() != "one\ntwo\nthree" {
		t.Fatalf("after undoing back to the start, Text = %q", e.Text())
	}
}
