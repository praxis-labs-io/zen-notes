package editor

import "testing"

func TestNewBufferAlwaysHasALine(t *testing.T) {
	b := NewBuffer("")
	if b.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", b.LineCount())
	}
	if b.Line(0) != "" {
		t.Fatalf("Line(0) = %q, want empty", b.Line(0))
	}
}

func TestBufferRoundTripsText(t *testing.T) {
	for _, in := range []string{"", "a", "a\nb", "a\nb\n", "\n", "a\n\nb"} {
		if got := NewBuffer(in).Text(); got != in {
			t.Errorf("NewBuffer(%q).Text() = %q", in, got)
		}
	}
}

func TestBufferSplitsLines(t *testing.T) {
	b := NewBuffer("one\ntwo\nthree")
	if b.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3", b.LineCount())
	}
	if b.Line(1) != "two" {
		t.Fatalf("Line(1) = %q, want two", b.Line(1))
	}
}

func TestInsertRuneInMiddle(t *testing.T) {
	b := NewBuffer("ac")
	got := b.Insert(Pos{0, 1}, "b")
	if b.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", b.Text())
	}
	if got != (Pos{0, 2}) {
		t.Fatalf("Insert returned %v, want {0 2}", got)
	}
}

func TestInsertNewlineSplitsLine(t *testing.T) {
	b := NewBuffer("ab")
	got := b.Insert(Pos{0, 1}, "\n")
	if b.Text() != "a\nb" {
		t.Fatalf("Text = %q, want a\\nb", b.Text())
	}
	if got != (Pos{1, 0}) {
		t.Fatalf("Insert returned %v, want {1 0}", got)
	}
}

func TestInsertMultilineText(t *testing.T) {
	b := NewBuffer("ad")
	got := b.Insert(Pos{0, 1}, "b\nc")
	if b.Text() != "ab\ncd" {
		t.Fatalf("Text = %q, want ab\\ncd", b.Text())
	}
	if got != (Pos{1, 1}) {
		t.Fatalf("Insert returned %v, want {1 1}", got)
	}
}

func TestInsertHandlesWideRunes(t *testing.T) {
	b := NewBuffer("日本")
	b.Insert(Pos{0, 1}, "語")
	if b.Text() != "日語本" {
		t.Fatalf("Text = %q, want 日語本", b.Text())
	}
}

func TestDeleteWithinLine(t *testing.T) {
	b := NewBuffer("abcd")
	got := b.Delete(Pos{0, 1}, Pos{0, 3})
	if b.Text() != "ad" {
		t.Fatalf("Text = %q, want ad", b.Text())
	}
	if got != "bc" {
		t.Fatalf("Delete returned %q, want bc", got)
	}
}

func TestDeleteAcrossLinesJoinsThem(t *testing.T) {
	b := NewBuffer("abc\ndef")
	got := b.Delete(Pos{0, 2}, Pos{1, 1})
	if b.Text() != "abef" {
		t.Fatalf("Text = %q, want abef", b.Text())
	}
	if got != "c\nd" {
		t.Fatalf("Delete returned %q, want c\\nd", got)
	}
}

func TestDeleteEmptyRangeChangesNothing(t *testing.T) {
	b := NewBuffer("abc")
	if got := b.Delete(Pos{0, 1}, Pos{0, 1}); got != "" {
		t.Fatalf("Delete returned %q, want empty", got)
	}
	if b.Text() != "abc" {
		t.Fatalf("Text = %q, want abc", b.Text())
	}
}

func TestDeleteReversedRangeIsNormalized(t *testing.T) {
	b := NewBuffer("abcd")
	if got := b.Delete(Pos{0, 3}, Pos{0, 1}); got != "bc" {
		t.Fatalf("Delete returned %q, want bc", got)
	}
	if b.Text() != "ad" {
		t.Fatalf("Text = %q, want ad", b.Text())
	}
}

func TestReplaceLines(t *testing.T) {
	b := NewBuffer("a\nb\nc")
	b.ReplaceLines(1, 2, []string{"x", "y"})
	if b.Text() != "a\nx\ny\nc" {
		t.Fatalf("Text = %q, want a\\nx\\ny\\nc", b.Text())
	}
}

func TestReplaceLinesDeletingAll(t *testing.T) {
	b := NewBuffer("a\nb")
	b.ReplaceLines(0, 2, nil)
	if b.Text() != "" {
		t.Fatalf("Text = %q, want empty", b.Text())
	}
	if b.LineCount() != 1 {
		t.Fatalf("LineCount = %d, want 1", b.LineCount())
	}
}

func TestClampKeepsPositionInsideBuffer(t *testing.T) {
	b := NewBuffer("ab\ncdef")
	tests := []struct {
		in, want Pos
	}{
		{Pos{-1, -1}, Pos{0, 0}},
		{Pos{0, 9}, Pos{0, 2}},
		{Pos{9, 9}, Pos{1, 4}},
		{Pos{1, 2}, Pos{1, 2}},
	}
	for _, tt := range tests {
		if got := b.Clamp(tt.in); got != tt.want {
			t.Errorf("Clamp(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLineLenCountsRunesNotBytes(t *testing.T) {
	b := NewBuffer("日本語")
	if got := b.LineLen(0); got != 3 {
		t.Fatalf("LineLen = %d, want 3", got)
	}
}
