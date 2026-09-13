package editor

import (
	"strings"
	"unicode"
)

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

// Negative so they never collide with a key rune.
const (
	opUpper  = -1
	opLower  = -2
	opToggle = -3
	opIndent = -4
	opDedent = -5
)

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

// Does nothing when fewer than count runes remain, as in vim.
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

func (e *Editor) reselect() {
	if e.lastVisual == [2]Pos{} {
		return
	}
	e.mode = ModeVisual
	e.visualStart = e.buf.Clamp(e.lastVisual[0])
	e.cursor = e.buf.Clamp(e.lastVisual[1])
	e.clampCursor()
}

func (e *Editor) rememberVisual() {
	if e.mode.Visual() {
		e.lastVisual = [2]Pos{e.visualStart, e.cursor}
	}
}

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

func bracketPair(r rune) (open, close rune, forward, ok bool) {
	if i := strings.IndexRune(openBrackets, r); i >= 0 {
		return r, rune(closeBrackets[i]), true, true
	}
	if i := strings.IndexRune(closeBrackets, r); i >= 0 {
		return rune(openBrackets[i]), r, false, true
	}
	return 0, 0, false, false
}
