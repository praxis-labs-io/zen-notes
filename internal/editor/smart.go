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
	contentStart int
	nextPrefix   string
}

// enterEdit describes the context-aware edit for Enter in insert mode.
func enterEdit(line []rune, col int, afterList bool) insertEdit {
	if item, ok := parseListLine(line); ok && col >= item.contentStart {
		if onlySpace(line[item.contentStart:]) {
			edit := insertEdit{deleteTo: item.contentStart}
			if afterList {
				edit.insert = "\n"
			}
			return edit
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
	item, ok := parseListLine(line)
	if !ok || col != item.contentStart || !onlySpace(line[item.contentStart:]) {
		return insertEdit{}, false
	}
	edit := insertEdit{deleteTo: item.contentStart}
	if afterList {
		edit.insert = "\n"
	}
	return edit, true
}

// needsListSeparator reports whether a newly typed marker begins a list
// immediately after non-list prose.
func needsListSeparator(line, previous []rune) bool {
	item, ok := parseListLine(line)
	if !ok || !onlySpace(line[item.contentStart:]) || onlySpace(previous) {
		return false
	}
	_, previousIsList := parseListLine(previous)
	return !previousIsList
}

func parseListLine(line []rune) (listLine, bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	indent := string(line[:i])
	if i >= len(line) {
		return listLine{}, false
	}

	var prefix string
	switch line[i] {
	case '-', '*', '+':
		marker := line[i]
		i++
		if i >= len(line) || !unicode.IsSpace(line[i]) {
			return listLine{}, false
		}
		for i < len(line) && unicode.IsSpace(line[i]) && line[i] != '\n' {
			i++
		}
		prefix = indent + string(marker) + " "
		if i+2 < len(line) && line[i] == '[' && (line[i+1] == ' ' || line[i+1] == 'x' || line[i+1] == 'X') && line[i+2] == ']' {
			i += 3
			if i >= len(line) || !unicode.IsSpace(line[i]) {
				return listLine{}, false
			}
			for i < len(line) && unicode.IsSpace(line[i]) && line[i] != '\n' {
				i++
			}
			prefix += "[ ] "
		}
	default:
		start := i
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if start == i || i >= len(line) || (line[i] != '.' && line[i] != ')') {
			return listLine{}, false
		}
		delimiter := line[i]
		n, err := strconv.Atoi(string(line[start:i]))
		if err != nil {
			return listLine{}, false
		}
		i++
		if i >= len(line) || !unicode.IsSpace(line[i]) {
			return listLine{}, false
		}
		for i < len(line) && unicode.IsSpace(line[i]) && line[i] != '\n' {
			i++
		}
		prefix = indent + strconv.Itoa(n+1) + string(delimiter) + " "
	}

	return listLine{contentStart: i, nextPrefix: prefix}, true
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
