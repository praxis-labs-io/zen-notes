package editor

import "unicode"

// charClass splits runes the way vim does: blanks, word characters, and
// everything else. A word boundary is any change between the latter two.
type charClass int

const (
	classBlank charClass = iota
	classWord
	classPunct
)

func classOf(r rune) charClass {
	switch {
	case r == ' ' || r == '\t':
		return classBlank
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return classWord
	default:
		return classPunct
	}
}

// classAt reports the class at p, treating the position past a line's last
// rune as blank so line ends separate words.
func classAt(b *Buffer, p Pos) charClass {
	line := b.runes(p.Line)
	if p.Col >= len(line) {
		return classBlank
	}
	return classOf(line[p.Col])
}

// isEmptyLine reports whether the line has no runes, which vim counts as a
// word of its own for w and b.
func isEmptyLine(b *Buffer, line int) bool {
	return b.LineLen(line) == 0
}

// nextPos steps one rune forward, moving past a line end onto the next line.
func nextPos(b *Buffer, p Pos) (Pos, bool) {
	if p.Col < b.LineLen(p.Line) {
		return Pos{p.Line, p.Col + 1}, true
	}
	if p.Line+1 < b.LineCount() {
		return Pos{p.Line + 1, 0}, true
	}
	return p, false
}

// prevPos steps one rune backward, moving past a line start onto the line above.
func prevPos(b *Buffer, p Pos) (Pos, bool) {
	if p.Col > 0 {
		return Pos{p.Line, p.Col - 1}, true
	}
	if p.Line > 0 {
		return Pos{p.Line - 1, b.LineLen(p.Line - 1)}, true
	}
	return p, false
}

// wordForward is w and W: the start of the next word, or the buffer end.
func wordForward(b *Buffer, p Pos, count int, big bool) Pos {
	for range count {
		p = wordForwardOnce(b, p, big)
	}
	return p
}

func wordForwardOnce(b *Buffer, p Pos, big bool) Pos {
	start := classAt(b, p)
	cur := p

	if start != classBlank {
		for {
			next, ok := nextPos(b, cur)
			if !ok {
				return b.End()
			}
			cur = next
			if next.Col == 0 && isEmptyLine(b, next.Line) {
				return next
			}
			if c := classAt(b, cur); c == classBlank || (!big && c != start) {
				break
			}
		}
	}

	for classAt(b, cur) == classBlank {
		if cur.Col == 0 && isEmptyLine(b, cur.Line) && cur != p {
			return cur
		}
		next, ok := nextPos(b, cur)
		if !ok {
			return b.End()
		}
		cur = next
	}
	return cur
}

// wordBack is b and B: the start of the word before the cursor.
func wordBack(b *Buffer, p Pos, count int, big bool) Pos {
	for range count {
		p = wordBackOnce(b, p, big)
	}
	return p
}

func wordBackOnce(b *Buffer, p Pos, big bool) Pos {
	cur, ok := prevPos(b, p)
	if !ok {
		return p
	}

	for classAt(b, cur) == classBlank {
		if cur.Col == 0 && isEmptyLine(b, cur.Line) {
			return cur
		}
		prev, ok := prevPos(b, cur)
		if !ok {
			return cur
		}
		cur = prev
	}

	class := classAt(b, cur)
	for {
		prev, ok := prevPos(b, cur)
		if !ok {
			return cur
		}
		if prev.Col == 0 && isEmptyLine(b, prev.Line) {
			return cur
		}
		c := classAt(b, prev)
		if c == classBlank || (!big && c != class) {
			return cur
		}
		cur = prev
	}
}

// wordEnd is e and E: the last rune of the current or next word.
func wordEnd(b *Buffer, p Pos, count int, big bool) Pos {
	for range count {
		p = wordEndOnce(b, p, big)
	}
	return p
}

func wordEndOnce(b *Buffer, p Pos, big bool) Pos {
	cur, ok := nextPos(b, p)
	if !ok {
		return b.End()
	}

	for classAt(b, cur) == classBlank {
		next, ok := nextPos(b, cur)
		if !ok {
			return cur
		}
		cur = next
	}

	class := classAt(b, cur)
	for {
		next, ok := nextPos(b, cur)
		if !ok {
			return cur
		}
		c := classAt(b, next)
		if c == classBlank || (!big && c != class) {
			return cur
		}
		cur = next
	}
}

// firstNonBlank is the ^ column, or 0 on a line that is all blanks.
func firstNonBlank(b *Buffer, line int) int {
	runes := b.runes(line)
	for i, r := range runes {
		if classOf(r) != classBlank {
			return i
		}
	}
	return 0
}

// findForward is f and t: the column of the count'th target right of the
// cursor on this line, one short of it when till is set.
func findForward(b *Buffer, p Pos, target rune, till bool, count int) (int, bool) {
	line := b.runes(p.Line)
	col := p.Col
	for range count {
		found := -1
		for i := col + 1; i < len(line); i++ {
			if line[i] == target {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		col = found
	}
	if till {
		return col - 1, true
	}
	return col, true
}

// findBack is F and T: the column of the count'th target left of the cursor
// on this line, one past it when till is set.
func findBack(b *Buffer, p Pos, target rune, till bool, count int) (int, bool) {
	line := b.runes(p.Line)
	col := p.Col
	for range count {
		found := -1
		for i := col - 1; i >= 0; i-- {
			if i < len(line) && line[i] == target {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		col = found
	}
	if till {
		return col + 1, true
	}
	return col, true
}

// paragraphForward is }: the next blank line, or the last line.
func paragraphForward(b *Buffer, p Pos, count int) Pos {
	line := p.Line
	for range count {
		line = paragraphForwardOnce(b, line)
	}
	return Pos{line, 0}
}

func paragraphForwardOnce(b *Buffer, line int) int {
	for i := line + 1; i < b.LineCount(); i++ {
		if isEmptyLine(b, i) && !isEmptyLine(b, i-1) {
			return i
		}
	}
	return b.LineCount() - 1
}

// paragraphBack is {: the previous blank line, or the first line.
func paragraphBack(b *Buffer, p Pos, count int) Pos {
	line := p.Line
	for range count {
		line = paragraphBackOnce(b, line)
	}
	return Pos{line, 0}
}

func paragraphBackOnce(b *Buffer, line int) int {
	for i := line - 1; i > 0; i-- {
		if isEmptyLine(b, i) && !isEmptyLine(b, i-1) {
			return i
		}
	}
	return 0
}
