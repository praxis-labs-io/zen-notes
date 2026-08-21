package editor

import (
	"strings"
	"unicode"
)

// caseOp maps the key after g onto the operator that runs it, reporting
// false for keys that are not one of gU, gu or g~.
func caseOp(r rune) (rune, bool) {
	switch r {
	case 'U':
		return opUpper, true
	case 'u':
		return opLower, true
	case '~':
		return opToggle, true
	}
	return 0, false
}

// Operators that transform text in place rather than removing it. They live
// outside the ASCII letters so they cannot collide with a real key.
const (
	opUpper  = -1
	opLower  = -2
	opToggle = -3
	opIndent = -4
	opDedent = -5
)

// isCaseOp reports whether an operator rewrites runes rather than cutting them.
func isCaseOp(op rune) bool {
	return op == opUpper || op == opLower || op == opToggle
}

func caseFunc(op rune) func(rune) rune {
	switch op {
	case opUpper:
		return unicode.ToUpper
	case opLower:
		return unicode.ToLower
	default:
		return toggleCase
	}
}

// replaceRunes is r: overwrite count runes under the cursor, staying put.
// It does nothing if the line is too short, as vim does.
func (e *Editor) replaceRunes(with rune, count int) {
	if with == 0 {
		return
	}
	line := e.buf.runes(e.cursor.Line)
	if e.cursor.Col+count > len(line) {
		return
	}

	e.snapshot()
	next := append([]rune(nil), line...)
	for i := range count {
		next[e.cursor.Col+i] = with
	}
	e.buf.ReplaceLines(e.cursor.Line, e.cursor.Line+1, []string{string(next)})
	e.cursor.Col += count - 1
	e.clampCursor()
}

// toggleAt is ~ in normal mode: flip the case under the cursor and step right.
func (e *Editor) toggleAt(count int) {
	line := e.buf.runes(e.cursor.Line)
	if e.cursor.Col >= len(line) {
		return
	}

	e.snapshot()
	next := append([]rune(nil), line...)
	end := min(e.cursor.Col+count, len(next))
	for i := e.cursor.Col; i < end; i++ {
		next[i] = toggleCase(next[i])
	}
	e.buf.ReplaceLines(e.cursor.Line, e.cursor.Line+1, []string{string(next)})
	e.cursor.Col = end
	e.clampCursor()
}

// reselect is gv: put the last visual selection back up.
func (e *Editor) reselect() {
	if e.lastVisual == [2]Pos{} {
		return
	}
	e.mode = ModeVisual
	e.visualStart = e.buf.Clamp(e.lastVisual[0])
	e.cursor = e.buf.Clamp(e.lastVisual[1])
	e.clampCursor()
}

// rememberVisual records the selection so gv can bring it back.
func (e *Editor) rememberVisual() {
	if e.mode.Visual() {
		e.lastVisual = [2]Pos{e.visualStart, e.cursor}
	}
}

// matchBracket is %: the partner of the bracket under the cursor, or of the
// first bracket to its right on the same line.
func matchBracket(b *Buffer, cur Pos) (Pos, bool) {
	line := b.runes(cur.Line)
	for col := cur.Col; col < len(line); col++ {
		open, close, forward, ok := bracketPair(line[col])
		if !ok {
			continue
		}
		at := Pos{cur.Line, col}
		if forward {
			return scanForward(b, at, open, close)
		}
		return scanBack(b, at, open, close)
	}
	return Pos{}, false
}

const (
	openBrackets  = "([{"
	closeBrackets = ")]}"
)

// bracketPair reports the delimiters r belongs to and which way to scan.
func bracketPair(r rune) (open, close rune, forward, ok bool) {
	if i := strings.IndexRune(openBrackets, r); i >= 0 {
		return r, rune(closeBrackets[i]), true, true
	}
	if i := strings.IndexRune(closeBrackets, r); i >= 0 {
		return rune(openBrackets[i]), r, false, true
	}
	return 0, 0, false, false
}
