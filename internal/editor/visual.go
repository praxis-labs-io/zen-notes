package editor

import "strings"

const indentUnit = "  "

// visualCommand runs the keys that only mean something with a selection up,
// reporting whether it took the key.
func (e *Editor) visualCommand(r rune) bool {
	from, to, _ := e.Selection()
	e.rememberVisual()

	switch r {
	case 'o':
		e.visualStart, e.cursor = e.cursor, e.visualStart
		e.clampCursor()
	case '>':
		e.indentLines(from.Line, to.Line, 1)
	case '<':
		e.indentLines(from.Line, to.Line, -1)
	case '~':
		e.mapSelection(toggleCase)
	case 'u':
		e.mapSelection(toLower)
	case 'U':
		e.mapSelection(toUpper)
	case 'J':
		e.joinLines(from.Line, max(to.Line, from.Line+1))
		e.mode = ModeNormal
	case 'I', 'A':
		if e.mode != ModeVisualBlock {
			return false
		}
		e.startBlockInsert(r == 'A')
	default:
		return false
	}
	e.pend = pending{}
	return true
}

// indentLines shifts a run of lines by one indent step in either direction.
func (e *Editor) indentLines(from, to, dir int) {
	e.snapshot()
	for i := from; i <= to && i < e.buf.LineCount(); i++ {
		line := e.buf.Line(i)
		if dir > 0 {
			e.buf.ReplaceLines(i, i+1, []string{indentUnit + line})
			continue
		}
		e.buf.ReplaceLines(i, i+1, []string{trimIndent(line)})
	}
	e.mode = ModeNormal
	e.cursor = Pos{from, 0}
	e.clampCursor()
}

// trimIndent removes up to one indent step of leading blanks.
func trimIndent(line string) string {
	for range len(indentUnit) {
		if strings.HasPrefix(line, " ") {
			line = line[1:]
			continue
		}
		if strings.HasPrefix(line, "\t") {
			return line[1:]
		}
		break
	}
	return line
}

// mapSelection rewrites every rune of the selection through fn.
func (e *Editor) mapSelection(fn func(rune) rune) {
	from, to, linewise := e.Selection()
	e.snapshot()

	if linewise {
		from.Col = 0
		to.Col = max(e.buf.LineLen(to.Line)-1, 0)
	}
	if e.mode == ModeVisualBlock {
		lo, hi := blockCols(from, to)
		for line := from.Line; line <= to.Line; line++ {
			e.mapRange(Pos{line, lo}, Pos{line, min(hi, max(e.buf.LineLen(line)-1, 0))}, fn)
		}
	} else {
		e.mapRange(from, to, fn)
	}

	e.mode = ModeNormal
	e.cursor = from
	e.clampCursor()
}

// mapRange rewrites an inclusive span on one or more lines.
func (e *Editor) mapRange(from, to Pos, fn func(rune) rune) {
	for line := from.Line; line <= to.Line && line < e.buf.LineCount(); line++ {
		runes := append([]rune(nil), e.buf.runes(line)...)
		start, end := 0, len(runes)-1
		if line == from.Line {
			start = from.Col
		}
		if line == to.Line {
			end = min(to.Col, len(runes)-1)
		}
		for i := start; i <= end && i < len(runes); i++ {
			runes[i] = fn(runes[i])
		}
		e.buf.ReplaceLines(line, line+1, []string{string(runes)})
	}
}

// pasteVisual replaces the active selection and keeps the change to one undo.
func (e *Editor) pasteVisual(reg register) {
	from, to, _ := e.Selection()
	switch e.mode {
	case ModeVisualLine:
		e.snapshot()
		reg.linewise, reg.block = true, false
		e.buf.ReplaceLines(from.Line, to.Line+1, strings.Split(reg.text, "\n"))
		e.setRegister(reg)
		e.mode = ModeNormal
		e.cursor = Pos{from.Line, 0}
		e.clampCursor()
	case ModeVisualBlock:
		e.pasteVisualBlock(from, to, reg)
	default:
		e.snapshot()
		reg.linewise, reg.block = false, false
		e.buf.Delete(from, e.forwardOne(to))
		end := e.buf.Insert(from, reg.text)
		e.setRegister(reg)
		e.mode = ModeNormal
		e.cursor = Pos{end.Line, max(end.Col-1, 0)}
		e.clampCursor()
	}
}

func (e *Editor) pasteVisualBlock(from, to Pos, reg register) {
	reg.linewise, reg.block = false, true
	parts := strings.Split(reg.text, "\n")
	height := to.Line - from.Line + 1
	block := make([]string, height)
	if len(parts) == 1 {
		for i := range block {
			block[i] = reg.text
		}
	} else {
		copy(block, parts)
	}
	reg.text = strings.Join(block, "\n")

	undoLen := len(e.undo)
	e.applyVisual('d')
	e.setRegister(reg)
	e.put(false)
	if len(e.undo) > undoLen+1 {
		e.undo = e.undo[:len(e.undo)-1]
	}
}

func toggleCase(r rune) rune {
	if lower := toLower(r); lower != r {
		return lower
	}
	return toUpper(r)
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

// joinLines pulls the lines after from up onto it, separated by one space and
// with each joined line's leading blanks dropped.
func (e *Editor) joinLines(from, to int) {
	if from >= e.buf.LineCount()-1 {
		return
	}
	e.snapshot()
	to = min(to, e.buf.LineCount()-1)

	joined := e.buf.Line(from)
	for i := from + 1; i <= to; i++ {
		next := strings.TrimLeft(e.buf.Line(i), " \t")
		if joined != "" && next != "" {
			joined += " "
		}
		joined += next
	}
	e.buf.ReplaceLines(from, to+1, []string{joined})
	e.cursor = Pos{from, 0}
	e.clampCursor()
}

// blockCols gives the left and right columns of a block selection.
func blockCols(from, to Pos) (int, int) {
	return min(from.Col, to.Col), max(from.Col, to.Col)
}

// applyBlock runs d, c or y over a rectangle, one line at a time.
func (e *Editor) applyBlock(op rune) {
	from, to, _ := e.Selection()
	lo, hi := blockCols(from, to)

	var parts []string
	for line := from.Line; line <= to.Line; line++ {
		runes := e.buf.runes(line)
		start := min(lo, len(runes))
		end := min(hi+1, len(runes))
		parts = append(parts, string(runes[start:end]))
	}
	e.setRegister(register{text: strings.Join(parts, "\n"), block: true})

	if op == 'y' {
		e.flashYank(from, to, false, true)
		e.reportYank(to.Line-from.Line+1, "line")
		e.mode = ModeNormal
		e.cursor = Pos{from.Line, lo}
		e.clampCursor()
		return
	}

	e.snapshot()
	for line := to.Line; line >= from.Line; line-- {
		runes := e.buf.runes(line)
		start := min(lo, len(runes))
		end := min(hi+1, len(runes))
		if start >= end {
			continue
		}
		e.buf.Delete(Pos{line, start}, Pos{line, end})
	}

	e.cursor = Pos{from.Line, lo}
	if op == 'c' {
		e.mode = ModeInsert
		e.blockInsert = blockPending{
			active:    true,
			firstLine: from.Line,
			lastLine:  to.Line,
			col:       lo,
		}
		return
	}
	e.mode = ModeNormal
	e.clampCursor()
}

// startBlockInsert begins a block I or A. What gets typed on the first line is
// replicated down the block when insert mode ends.
func (e *Editor) startBlockInsert(atEnd bool) {
	from, to, _ := e.Selection()
	lo, hi := blockCols(from, to)

	col := lo
	if atEnd {
		col = hi + 1
	}
	e.snapshot()
	e.mode = ModeInsert
	e.cursor = e.buf.Clamp(Pos{from.Line, min(col, e.buf.LineLen(from.Line))})
	e.blockInsert = blockPending{
		active:     true,
		firstLine:  from.Line,
		lastLine:   to.Line,
		col:        col,
		appendEnd:  atEnd,
		insertedAt: e.cursor,
	}
}

// finishBlockInsert copies whatever was typed on the first line of a block
// insert down the remaining lines.
func (e *Editor) finishBlockInsert() {
	b := e.blockInsert
	e.blockInsert = blockPending{}
	if !b.active || e.cursor.Line != b.firstLine {
		return
	}

	typed := e.buf.Line(b.firstLine)
	start, end := b.insertedAt.Col, e.cursor.Col
	if end <= start || start > len(typed) {
		return
	}
	inserted := string([]rune(typed)[start:min(end, len([]rune(typed)))])
	if inserted == "" {
		return
	}

	for line := b.firstLine + 1; line <= b.lastLine && line < e.buf.LineCount(); line++ {
		col := b.col
		if b.appendEnd {
			col = min(col, e.buf.LineLen(line))
		}
		if col > e.buf.LineLen(line) {
			continue
		}
		e.buf.Insert(Pos{line, col}, inserted)
	}
}
