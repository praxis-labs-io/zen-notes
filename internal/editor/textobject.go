package editor

// textRange is the span a text object resolves to, as an inclusive pair.
type textRange struct {
	from, to Pos
	linewise bool
}

// pairs maps the bracket and quote objects to their delimiters. Quotes use the
// same rune on both sides, which is what makes them a separate scan below.
var pairs = map[rune][2]rune{
	'(': {'(', ')'}, ')': {'(', ')'}, 'b': {'(', ')'},
	'[': {'[', ']'}, ']': {'[', ']'},
	'{': {'{', '}'}, '}': {'{', '}'}, 'B': {'{', '}'},
	'<': {'<', '>'}, '>': {'<', '>'},
}

var quotes = map[rune]bool{'"': true, '\'': true, '`': true}

// resolveTextObject finds the span for iw, aw, i", a(, ip and friends,
// reporting false for an object it does not know.
func resolveTextObject(b *Buffer, cur Pos, around bool, object rune) (textRange, bool) {
	switch {
	case object == 'w' || object == 'W':
		return wordObject(b, cur, around, object == 'W')
	case object == 'p':
		return paragraphObject(b, cur, around)
	case quotes[object]:
		return quoteObject(b, cur, around, object)
	}
	if pair, ok := pairs[object]; ok {
		return pairObject(b, cur, around, pair[0], pair[1])
	}
	return textRange{}, false
}

// wordObject is iw and aw. Around takes the trailing run of blanks, or the
// leading one when the word ends the line.
func wordObject(b *Buffer, cur Pos, around, big bool) (textRange, bool) {
	line := b.runes(cur.Line)
	if len(line) == 0 {
		return textRange{}, false
	}
	col := min(cur.Col, len(line)-1)

	class := classOfIn(line, col, big)
	start, end := col, col
	for start > 0 && classOfIn(line, start-1, big) == class {
		start--
	}
	for end+1 < len(line) && classOfIn(line, end+1, big) == class {
		end++
	}

	if !around {
		return textRange{from: Pos{cur.Line, start}, to: Pos{cur.Line, end}}, true
	}

	trailing := end
	for trailing+1 < len(line) && classOfIn(line, trailing+1, big) == classBlank {
		trailing++
	}
	if trailing > end {
		return textRange{from: Pos{cur.Line, start}, to: Pos{cur.Line, trailing}}, true
	}
	for start > 0 && classOfIn(line, start-1, big) == classBlank {
		start--
	}
	return textRange{from: Pos{cur.Line, start}, to: Pos{cur.Line, end}}, true
}

// classOfIn folds punctuation into words for the WORD objects.
func classOfIn(line []rune, i int, big bool) charClass {
	c := classOf(line[i])
	if big && c == classPunct {
		return classWord
	}
	return c
}

// paragraphObject is ip and ap: the run of non-blank lines around the cursor,
// plus the blank lines after it for ap.
func paragraphObject(b *Buffer, cur Pos, around bool) (textRange, bool) {
	blank := isEmptyLine(b, cur.Line)
	start, end := cur.Line, cur.Line
	for start > 0 && isEmptyLine(b, start-1) == blank {
		start--
	}
	for end+1 < b.LineCount() && isEmptyLine(b, end+1) == blank {
		end++
	}
	if around {
		for end+1 < b.LineCount() && isEmptyLine(b, end+1) != blank {
			end++
		}
	}
	return textRange{from: Pos{start, 0}, to: Pos{end, 0}, linewise: true}, true
}

// quoteObject is i" and a", scanning the line's quotes in pairs so the cursor
// can sit anywhere inside or on either delimiter.
func quoteObject(b *Buffer, cur Pos, around bool, q rune) (textRange, bool) {
	line := b.runes(cur.Line)
	var open = -1
	for i := 0; i < len(line); i++ {
		if line[i] != q {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		if cur.Col <= i {
			return quoteRange(cur.Line, open, i, around), true
		}
		open = -1
	}
	return textRange{}, false
}

func quoteRange(line, open, close int, around bool) textRange {
	if around {
		return textRange{from: Pos{line, open}, to: Pos{line, close}}
	}
	return textRange{from: Pos{line, open + 1}, to: Pos{line, close - 1}}
}

// pairObject is i( and a(, matching nesting outward from the cursor. It scans
// the whole buffer so a block spanning lines still resolves.
func pairObject(b *Buffer, cur Pos, around bool, open, close rune) (textRange, bool) {
	start, ok := scanBack(b, cur, open, close)
	if !ok {
		return textRange{}, false
	}
	end, ok := scanForward(b, cur, open, close)
	if !ok {
		return textRange{}, false
	}
	if around {
		return textRange{from: start, to: end}, true
	}
	inner, ok := nextPos(b, start)
	if !ok {
		return textRange{}, false
	}
	last, ok := prevPos(b, end)
	if !ok || last.Before(inner) {
		return textRange{from: inner, to: inner}, true
	}
	return textRange{from: inner, to: last}, true
}

// scanBack walks left for the unmatched opening delimiter.
func scanBack(b *Buffer, cur Pos, open, close rune) (Pos, bool) {
	if runeAt(b, cur) == open {
		return cur, true
	}
	depth := 0
	p := cur
	for {
		prev, ok := prevPos(b, p)
		if !ok {
			return Pos{}, false
		}
		p = prev
		switch runeAt(b, p) {
		case close:
			depth++
		case open:
			if depth == 0 {
				return p, true
			}
			depth--
		}
	}
}

// scanForward walks right for the matching closing delimiter.
func scanForward(b *Buffer, cur Pos, open, close rune) (Pos, bool) {
	if runeAt(b, cur) == close {
		return cur, true
	}
	depth := 0
	p := cur
	for {
		next, ok := nextPos(b, p)
		if !ok {
			return Pos{}, false
		}
		p = next
		switch runeAt(b, p) {
		case open:
			depth++
		case close:
			if depth == 0 {
				return p, true
			}
			depth--
		}
	}
}

func runeAt(b *Buffer, p Pos) rune {
	line := b.runes(p.Line)
	if p.Col < 0 || p.Col >= len(line) {
		return 0
	}
	return line[p.Col]
}
