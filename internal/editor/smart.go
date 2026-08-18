package editor

import (
	"strconv"
	"strings"
	"unicode"
)

type insertEdit struct {
	deleteTo int
	insert   string
}

type listLine struct {
	indent       int
	contentStart int
	nextPrefix   string
}

// enterEdit describes the context-aware edit for Enter in insert mode.
func enterEdit(line []rune, col int, afterList bool) insertEdit {
	item, isList := parseListLine(line)
	if isList && col >= item.contentStart {
		if onlySpace(line[item.contentStart:]) {
			if item.indent > 0 {
				return insertEdit{deleteTo: min(item.indent, len([]rune(indentUnit)))}
			}
			return exitListEdit(item.contentStart, afterList)
		}
		return insertEdit{insert: "\n" + item.nextPrefix}
	}
	if col == len(line) && isHeading(line) {
		return insertEdit{insert: "\n\n"}
	}
	return insertEdit{insert: "\n"}
}

// backspaceEdit removes an empty list marker at its content boundary.
func backspaceEdit(line []rune, col int, afterList bool) (insertEdit, bool) {
	item, isList := parseListLine(line)
	if !isList || col != item.contentStart || !onlySpace(line[item.contentStart:]) {
		return insertEdit{}, false
	}
	if item.indent > 0 {
		return insertEdit{deleteTo: min(item.indent, len([]rune(indentUnit)))}, true
	}
	return exitListEdit(item.contentStart, afterList), true
}

func exitListEdit(contentStart int, afterList bool) insertEdit {
	edit := insertEdit{deleteTo: contentStart}
	if afterList {
		edit.insert = "\n"
	}
	return edit
}

// needsListSeparator reports whether a newly typed marker begins a list
// immediately after non-list prose.
func needsListSeparator(line, previous []rune) bool {
	item, isList := parseListLine(line)
	if !isList || !onlySpace(line[item.contentStart:]) || onlySpace(previous) {
		return false
	}
	_, previousIsList := parseListLine(previous)
	return !previousIsList
}

func parseListLine(line []rune) (listLine, bool) {
	indentEnd := 0
	for indentEnd < len(line) && (line[indentEnd] == ' ' || line[indentEnd] == '\t') {
		indentEnd++
	}
	if indentEnd >= len(line) {
		return listLine{}, false
	}

	indent := string(line[:indentEnd])
	switch line[indentEnd] {
	case '-', '*', '+':
		return parseBulletList(line, indentEnd, indent)
	default:
		return parseOrderedList(line, indentEnd, indent)
	}
}

func parseBulletList(line []rune, markerAt int, indent string) (listLine, bool) {
	contentStart, ok := skipRequiredSpace(line, markerAt+1)
	if !ok {
		return listLine{}, false
	}

	prefix := indent + string(line[markerAt]) + " "
	if !hasTaskMarker(line, contentStart) {
		return listLine{indent: markerAt, contentStart: contentStart, nextPrefix: prefix}, true
	}

	contentStart, ok = skipRequiredSpace(line, contentStart+3)
	if !ok {
		return listLine{}, false
	}
	return listLine{indent: markerAt, contentStart: contentStart, nextPrefix: prefix + "[ ] "}, true
}

func parseOrderedList(line []rune, numberAt int, indent string) (listLine, bool) {
	delimiterAt := numberAt
	for delimiterAt < len(line) && line[delimiterAt] >= '0' && line[delimiterAt] <= '9' {
		delimiterAt++
	}
	if delimiterAt == numberAt || delimiterAt >= len(line) || (line[delimiterAt] != '.' && line[delimiterAt] != ')') {
		return listLine{}, false
	}

	n, err := strconv.Atoi(string(line[numberAt:delimiterAt]))
	if err != nil {
		return listLine{}, false
	}
	contentStart, ok := skipRequiredSpace(line, delimiterAt+1)
	if !ok {
		return listLine{}, false
	}

	prefix := indent + strconv.Itoa(n+1) + string(line[delimiterAt]) + " "
	return listLine{indent: numberAt, contentStart: contentStart, nextPrefix: prefix}, true
}

func skipRequiredSpace(line []rune, at int) (int, bool) {
	if at >= len(line) || !unicode.IsSpace(line[at]) {
		return 0, false
	}
	for at < len(line) && unicode.IsSpace(line[at]) {
		at++
	}
	return at, true
}

func hasTaskMarker(line []rune, at int) bool {
	return at+2 < len(line) && line[at] == '[' &&
		(line[at+1] == ' ' || line[at+1] == 'x' || line[at+1] == 'X') &&
		line[at+2] == ']'
}

func isHeading(line []rune) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	start := i
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == start || i-start > 6 || i >= len(line) || !unicode.IsSpace(line[i]) {
		return false
	}
	return strings.TrimSpace(string(line[i:])) != ""
}

func onlySpace(runes []rune) bool {
	return strings.TrimSpace(string(runes)) == ""
}

// shiftListItem moves the current list item and its descendants by one level.
// It reports whether the line was a list item, even when the shift is invalid.
func (e *Editor) shiftListItem(dir int) bool {
	item, ok := parseListLine(e.buf.runes(e.cursor.Line))
	if !ok {
		return false
	}
	if dir > 0 && !e.hasPreviousListSibling(item.indent) {
		return true
	}
	if dir < 0 && item.indent == 0 {
		return true
	}

	to := e.listSubtreeEnd(item.indent)
	cursorShift := 0
	changed := false
	for line := e.cursor.Line; line <= to; line++ {
		text := e.buf.Line(line)
		if dir > 0 {
			e.buf.ReplaceLines(line, line+1, []string{indentUnit + text})
			if line == e.cursor.Line {
				cursorShift = len([]rune(indentUnit))
			}
			changed = true
			continue
		}

		trimmed := trimIndent(text)
		if trimmed == text {
			continue
		}
		e.buf.ReplaceLines(line, line+1, []string{trimmed})
		if line == e.cursor.Line {
			cursorShift = -(len([]rune(text)) - len([]rune(trimmed)))
		}
		changed = true
	}
	if changed {
		e.cursor.Col = max(e.cursor.Col+cursorShift, 0)
		e.dirty = true
	}
	return true
}

// hasPreviousListSibling rejects indentation without an item to become the
// parent. Descendants of a previous sibling are skipped while walking back.
func (e *Editor) hasPreviousListSibling(indent int) bool {
	for line := e.cursor.Line - 1; line >= 0; line-- {
		item, ok := parseListLine(e.buf.runes(line))
		if !ok || item.indent < indent {
			return false
		}
		if item.indent == indent {
			return true
		}
	}
	return false
}

func (e *Editor) listSubtreeEnd(indent int) int {
	end := e.cursor.Line
	for line := e.cursor.Line + 1; line < e.buf.LineCount(); line++ {
		item, ok := parseListLine(e.buf.runes(line))
		if ok && item.indent <= indent {
			break
		}
		end = line
	}
	return end
}
