package editor

import "testing"

func TestWordForward(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		from  Pos
		count int
		want  Pos
	}{
		{"to next word", "foo bar", Pos{0, 0}, 1, Pos{0, 4}},
		{"counted", "foo bar baz", Pos{0, 0}, 2, Pos{0, 8}},
		{"punctuation is its own word", "foo.bar", Pos{0, 0}, 1, Pos{0, 3}},
		{"off punctuation", "foo.bar", Pos{0, 3}, 1, Pos{0, 4}},
		{"crosses lines", "foo\nbar", Pos{0, 1}, 1, Pos{1, 0}},
		{"empty line counts as a word", "foo\n\nbar", Pos{0, 0}, 1, Pos{1, 0}},
		{"past empty line", "foo\n\nbar", Pos{1, 0}, 1, Pos{2, 0}},
		{"skips leading blanks", "foo   bar", Pos{0, 0}, 1, Pos{0, 6}},
		{"stops at buffer end", "foo bar", Pos{0, 4}, 1, Pos{0, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.text)
			if got := wordForward(b, tt.from, tt.count, false); got != tt.want {
				t.Errorf("wordForward = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigWordForwardIgnoresPunctuation(t *testing.T) {
	b := NewBuffer("foo.bar baz")
	if got := wordForward(b, Pos{0, 0}, 1, true); got != (Pos{0, 8}) {
		t.Fatalf("wordForward big = %v, want {0 8}", got)
	}
}

func TestWordBack(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		from  Pos
		count int
		want  Pos
	}{
		{"to previous word", "foo bar", Pos{0, 6}, 1, Pos{0, 4}},
		{"to start of line", "foo bar", Pos{0, 4}, 1, Pos{0, 0}},
		{"counted", "foo bar baz", Pos{0, 8}, 2, Pos{0, 0}},
		{"from mid word", "foo bar", Pos{0, 5}, 1, Pos{0, 4}},
		{"crosses lines", "foo\nbar", Pos{1, 0}, 1, Pos{0, 0}},
		{"punctuation is its own word", "foo.bar", Pos{0, 4}, 1, Pos{0, 3}},
		{"stops at buffer start", "foo", Pos{0, 0}, 1, Pos{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.text)
			if got := wordBack(b, tt.from, tt.count, false); got != tt.want {
				t.Errorf("wordBack = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWordEnd(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		from  Pos
		count int
		want  Pos
	}{
		{"end of current word", "foo bar", Pos{0, 0}, 1, Pos{0, 2}},
		{"end of next word", "foo bar", Pos{0, 2}, 1, Pos{0, 6}},
		{"counted", "foo bar baz", Pos{0, 0}, 2, Pos{0, 6}},
		{"crosses lines", "foo\nbar", Pos{0, 2}, 1, Pos{1, 2}},
		{"punctuation", "foo.bar", Pos{0, 0}, 1, Pos{0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.text)
			if got := wordEnd(b, tt.from, tt.count, false); got != tt.want {
				t.Errorf("wordEnd = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstNonBlank(t *testing.T) {
	b := NewBuffer("   foo")
	if got := firstNonBlank(b, 0); got != 3 {
		t.Fatalf("firstNonBlank = %d, want 3", got)
	}
	blank := NewBuffer("   ")
	if got := firstNonBlank(blank, 0); got != 0 {
		t.Fatalf("firstNonBlank of all blanks = %d, want 0", got)
	}
}

func TestFindForward(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		from   Pos
		target rune
		till   bool
		count  int
		want   int
		ok     bool
	}{
		{"f finds char", "hello world", Pos{0, 0}, 'o', false, 1, 4, true},
		{"f counted", "hello world", Pos{0, 0}, 'o', false, 2, 7, true},
		{"f misses", "hello", Pos{0, 0}, 'z', false, 1, 0, false},
		{"t stops before", "hello world", Pos{0, 0}, 'w', true, 1, 5, true},
		{"f does not match cursor", "aXa", Pos{0, 1}, 'X', false, 1, 0, false},
		{"stays on line", "ab\nXc", Pos{0, 0}, 'X', false, 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.text)
			got, ok := findForward(b, tt.from, tt.target, tt.till, tt.count)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("findForward = %d, %v; want %d, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFindBack(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		from   Pos
		target rune
		till   bool
		count  int
		want   int
		ok     bool
	}{
		{"F finds char", "hello world", Pos{0, 10}, 'o', false, 1, 7, true},
		{"F counted", "hello world", Pos{0, 10}, 'o', false, 2, 4, true},
		{"F misses", "hello", Pos{0, 4}, 'z', false, 1, 0, false},
		{"T stops after", "hello world", Pos{0, 10}, 'o', true, 1, 8, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuffer(tt.text)
			got, ok := findBack(b, tt.from, tt.target, tt.till, tt.count)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("findBack = %d, %v; want %d, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParagraphForward(t *testing.T) {
	b := NewBuffer("a\nb\n\nc\nd\n\ne")
	if got := paragraphForward(b, Pos{0, 0}, 1); got.Line != 2 {
		t.Fatalf("paragraphForward line = %d, want 2", got.Line)
	}
	if got := paragraphForward(b, Pos{0, 0}, 2); got.Line != 5 {
		t.Fatalf("paragraphForward count 2 line = %d, want 5", got.Line)
	}
	if got := paragraphForward(b, Pos{5, 0}, 1); got.Line != 6 {
		t.Fatalf("paragraphForward past last blank = %d, want 6", got.Line)
	}
}

func TestParagraphBack(t *testing.T) {
	b := NewBuffer("a\nb\n\nc\nd\n\ne")
	if got := paragraphBack(b, Pos{6, 0}, 1); got.Line != 5 {
		t.Fatalf("paragraphBack line = %d, want 5", got.Line)
	}
	if got := paragraphBack(b, Pos{6, 0}, 2); got.Line != 2 {
		t.Fatalf("paragraphBack count 2 line = %d, want 2", got.Line)
	}
	if got := paragraphBack(b, Pos{1, 0}, 1); got.Line != 0 {
		t.Fatalf("paragraphBack past first blank = %d, want 0", got.Line)
	}
}
