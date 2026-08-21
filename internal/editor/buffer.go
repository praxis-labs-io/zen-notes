// Package editor holds the modal text buffer behind zen-notes: line storage,
// vim motions and operators, markdown highlighting, and the view.
package editor

import (
	"slices"
	"strings"
)

// Pos addresses a rune in the buffer. Col counts runes, never bytes.
type Pos struct {
	Line int
	Col  int
}

// Before reports whether p sorts earlier in the buffer than o.
func (p Pos) Before(o Pos) bool {
	if p.Line != o.Line {
		return p.Line < o.Line
	}
	return p.Col < o.Col
}

// Buffer is the note as logical lines, no terminators stored. NewBuffer and
// Text round trip exactly, so a trailing newline shows as a final empty line.
type Buffer struct {
	lines [][]rune
}

// NewBuffer splits text on newlines. Empty text yields one empty line.
func NewBuffer(text string) *Buffer {
	parts := strings.Split(text, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return &Buffer{lines: lines}
}

// Text joins the lines back into the file contents.
func (b *Buffer) Text() string {
	parts := make([]string, len(b.lines))
	for i, l := range b.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

// LineCount is the number of logical lines, always at least one.
func (b *Buffer) LineCount() int { return len(b.lines) }

// Line returns the logical line at i, or empty if i is out of range.
func (b *Buffer) Line(i int) string {
	if i < 0 || i >= len(b.lines) {
		return ""
	}
	return string(b.lines[i])
}

// LineLen is the rune count of line i.
func (b *Buffer) LineLen(i int) int {
	if i < 0 || i >= len(b.lines) {
		return 0
	}
	return len(b.lines[i])
}

// runes gives internal read access without a conversion. Do not mutate.
func (b *Buffer) runes(i int) []rune {
	if i < 0 || i >= len(b.lines) {
		return nil
	}
	return b.lines[i]
}

// Lines snapshots the buffer for the undo stack.
func (b *Buffer) Lines() [][]rune {
	out := make([][]rune, len(b.lines))
	for i, l := range b.lines {
		out[i] = append([]rune(nil), l...)
	}
	return out
}

// SetLines restores a snapshot taken with Lines.
func (b *Buffer) SetLines(lines [][]rune) {
	if len(lines) == 0 {
		b.lines = [][]rune{{}}
		return
	}
	b.lines = lines
}

// Clamp moves p to the nearest valid position in the buffer.
func (b *Buffer) Clamp(p Pos) Pos {
	if p.Line < 0 {
		p.Line = 0
	}
	if p.Line >= len(b.lines) {
		p.Line = len(b.lines) - 1
	}
	if p.Col < 0 {
		p.Col = 0
	}
	if n := len(b.lines[p.Line]); p.Col > n {
		p.Col = n
	}
	return p
}

// End is the position just past the last rune of the buffer.
func (b *Buffer) End() Pos {
	last := len(b.lines) - 1
	return Pos{last, len(b.lines[last])}
}

// Insert writes text at p, splitting on any newlines, and returns the
// position just past what was inserted.
func (b *Buffer) Insert(p Pos, text string) Pos {
	p = b.Clamp(p)
	if text == "" {
		return p
	}

	line := b.lines[p.Line]
	head := append([]rune(nil), line[:p.Col]...)
	tail := append([]rune(nil), line[p.Col:]...)

	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		b.lines[p.Line] = slices.Concat(head, []rune(parts[0]), tail)
		return Pos{p.Line, p.Col + len([]rune(parts[0]))}
	}

	inserted := make([][]rune, len(parts))
	inserted[0] = append(head, []rune(parts[0])...)
	for i := 1; i < len(parts); i++ {
		inserted[i] = []rune(parts[i])
	}
	last := len(parts) - 1
	end := Pos{p.Line + last, len(inserted[last])}
	inserted[last] = append(inserted[last], tail...)

	b.splice(p.Line, p.Line+1, inserted)
	return end
}

// Delete removes the half-open rune range and returns what it took. The
// positions may be given in either order.
func (b *Buffer) Delete(from, to Pos) string {
	from, to = b.Clamp(from), b.Clamp(to)
	if to.Before(from) {
		from, to = to, from
	}
	if from == to {
		return ""
	}

	if from.Line == to.Line {
		line := b.lines[from.Line]
		removed := string(line[from.Col:to.Col])
		b.lines[from.Line] = slices.Concat(line[:from.Col], line[to.Col:])
		return removed
	}

	var sb strings.Builder
	sb.WriteString(string(b.lines[from.Line][from.Col:]))
	for i := from.Line + 1; i < to.Line; i++ {
		sb.WriteByte('\n')
		sb.WriteString(string(b.lines[i]))
	}
	sb.WriteByte('\n')
	sb.WriteString(string(b.lines[to.Line][:to.Col]))

	joined := slices.Concat(b.lines[from.Line][:from.Col], b.lines[to.Line][to.Col:])
	b.splice(from.Line, to.Line+1, [][]rune{joined})
	return sb.String()
}

// ReplaceLines swaps the half-open line range [from, to) for with.
func (b *Buffer) ReplaceLines(from, to int, with []string) {
	if from < 0 {
		from = 0
	}
	if to > len(b.lines) {
		to = len(b.lines)
	}
	if to < from {
		return
	}
	replacement := make([][]rune, len(with))
	for i, s := range with {
		replacement[i] = []rune(s)
	}
	b.splice(from, to, replacement)
}

// splice replaces lines [from, to), keeping the buffer from ever emptying.
func (b *Buffer) splice(from, to int, with [][]rune) {
	next := make([][]rune, 0, len(b.lines)-(to-from)+len(with))
	next = append(next, b.lines[:from]...)
	next = append(next, with...)
	next = append(next, b.lines[to:]...)
	if len(next) == 0 {
		next = [][]rune{{}}
	}
	b.lines = next
}
