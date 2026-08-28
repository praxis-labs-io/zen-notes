package editor

import "testing"

// Home and End resolve to motions, so counts, operators and visual mode come
// with them rather than needing their own cases.
func TestNavigationKeysInNormalMode(t *testing.T) {
	const text = "one two three\nfour five\nsix"

	tests := []struct {
		name   string
		keys   string
		cursor Pos
		want   string
	}{
		{name: "home goes to the first column", keys: "ww<home>", cursor: Pos{0, 0}, want: text},
		{name: "end goes to the last rune", keys: "<end>", cursor: Pos{0, 12}, want: text},
		{name: "end then home returns", keys: "<end><home>", cursor: Pos{0, 0}, want: text},
		{name: "delete takes the rune under the caret", keys: "<del>", cursor: Pos{0, 0}, want: "ne two three\nfour five\nsix"},
		{name: "delete takes a count", keys: "3<del>", cursor: Pos{0, 0}, want: " two three\nfour five\nsix"},
		{name: "delete over an operator", keys: "d<end>", cursor: Pos{0, 0}, want: "\nfour five\nsix"},
		{name: "change to the first column", keys: "wc<home>zap<esc>", cursor: Pos{0, 2}, want: "zaptwo three\nfour five\nsix"},
		{name: "visual to the end", keys: "v<end>d", cursor: Pos{0, 0}, want: "\nfour five\nsix"},
		{name: "visual back to the first column", keys: "wv<home>d", cursor: Pos{0, 0}, want: "wo three\nfour five\nsix"},
		{name: "delete cuts a selection", keys: "vll<del>", cursor: Pos{0, 0}, want: " two three\nfour five\nsix"},
		{name: "delete cuts a linewise selection", keys: "Vj<del>", cursor: Pos{0, 0}, want: "six"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, text, tt.keys)
			if e.Text() != tt.want {
				t.Fatalf("Text = %q, want %q", e.Text(), tt.want)
			}
			if e.Cursor() != tt.cursor {
				t.Fatalf("Cursor = %v, want %v", e.Cursor(), tt.cursor)
			}
		})
	}
}

// A named key is never a count digit, so Home after a stray 2 is still Home
// and does not leave a 20 behind for the next key.
func TestHomeIsNotACountDigit(t *testing.T) {
	e := run(t, "one two three\nfour\nfive\nsix\nseven", "$2<home>j")
	if e.Cursor() != (Pos{1, 0}) {
		t.Fatalf("Cursor = %v, want {1 0}. The 2 and Home made a count of 20", e.Cursor())
	}
}

func TestNavigationKeysInInsertMode(t *testing.T) {
	tests := []struct {
		name string
		keys string
		want string
	}{
		{"home types at the start", "wwiX<esc>", "one two Xthree"},
		{"home rewinds to the start", "wwi<home>X<esc>", "Xone two three"},
		{"end runs to past the last rune", "i<end>X<esc>", "one two threeX"},
		{"delete takes the rune ahead", "i<del><esc>", "ne two three"},
		{"delete pulls the next line up", "A<del><esc>", "one two three"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := run(t, "one two three", tt.keys)
			if e.Text() != tt.want {
				t.Fatalf("Text = %q, want %q", e.Text(), tt.want)
			}
		})
	}
}

func TestForwardDeleteJoinsLines(t *testing.T) {
	e := run(t, "one\ntwo", "A<del><esc>")
	if e.Text() != "onetwo" {
		t.Fatalf("Text = %q, want onetwo", e.Text())
	}
}

func TestForwardDeleteAtTheEndOfTheBufferDoesNothing(t *testing.T) {
	e := run(t, "one\ntwo", "GA<del><del><esc>")
	if e.Text() != "one\ntwo" {
		t.Fatalf("Text = %q, want unchanged", e.Text())
	}
}

// PageUp and PageDown keep two lines on screen across the jump, as vim's C-f
// and C-b do, so the reader has something to land on.
func TestPageKeysOverlapByTwoLines(t *testing.T) {
	lines := make([]byte, 0, 200)
	for i := range 100 {
		lines = append(lines, byte('a'+i%26), '\n')
	}
	text := string(lines[:len(lines)-1])

	e := New(text)
	e.SetHeight(20)

	feed(t, e, "<pgdown>")
	if e.Cursor().Line != 18 {
		t.Fatalf("after pgdown, line = %d, want 18", e.Cursor().Line)
	}
	feed(t, e, "<pgdown>")
	if e.Cursor().Line != 36 {
		t.Fatalf("after a second pgdown, line = %d, want 36", e.Cursor().Line)
	}
	feed(t, e, "<pgup>")
	if e.Cursor().Line != 18 {
		t.Fatalf("after pgup, line = %d, want 18", e.Cursor().Line)
	}
}

func TestPageKeysWorkInInsertMode(t *testing.T) {
	e := New("a\nb\nc\nd\ne\nf\ng\nh\ni\nj")
	e.SetHeight(6)

	feed(t, e, "i<pgdown>")
	if e.Cursor().Line != 4 {
		t.Fatalf("line = %d, want 4", e.Cursor().Line)
	}
	if e.Mode() != ModeInsert {
		t.Fatalf("Mode = %v, want insert", e.Mode())
	}
}
