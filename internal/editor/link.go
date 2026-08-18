package editor

type inlineLink struct {
	from, to int
	target   string
}

// inlineLinks returns the Markdown links on one line with rune-based spans.
func inlineLinks(runes []rune) []inlineLink {
	var links []inlineLink
	for from := 0; from < len(runes); from++ {
		if runes[from] != '[' || isEscaped(runes, from) {
			continue
		}
		link, ok := parseInlineLink(runes, from)
		if !ok {
			continue
		}
		links = append(links, link)
		from = link.to - 1
	}
	return links
}

func parseInlineLink(runes []rune, from int) (inlineLink, bool) {
	labelEnd := closingLabel(runes, from+1)
	if labelEnd < 0 || labelEnd+1 >= len(runes) || runes[labelEnd+1] != '(' {
		return inlineLink{}, false
	}

	pos := labelEnd + 2
	if pos < len(runes) && runes[pos] == '<' {
		return parsePointyDestination(runes, from, pos)
	}
	return parseBareDestination(runes, from, pos)
}

func closingLabel(runes []rune, from int) int {
	depth := 0
	for pos := from; pos < len(runes); pos++ {
		if isMarkdownEscape(runes, pos) {
			pos++
			continue
		}
		switch runes[pos] {
		case '[':
			depth++
		case ']':
			if depth == 0 {
				return pos
			}
			depth--
		}
	}
	return -1
}

func parsePointyDestination(runes []rune, from, open int) (inlineLink, bool) {
	for pos := open + 1; pos < len(runes); pos++ {
		if isMarkdownEscape(runes, pos) {
			pos++
			continue
		}
		if runes[pos] == '<' {
			return inlineLink{}, false
		}
		if runes[pos] == '>' {
			return finishInlineLink(runes, from, pos+1, runes[open+1:pos])
		}
	}
	return inlineLink{}, false
}

func parseBareDestination(runes []rune, from, start int) (inlineLink, bool) {
	depth := 0
	for pos := start; pos < len(runes); pos++ {
		if isMarkdownEscape(runes, pos) {
			pos++
			continue
		}
		switch runes[pos] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return inlineLink{from: from, to: pos + 1, target: markdownText(runes[start:pos])}, true
			}
			depth--
		case ' ', '\t':
			if depth != 0 {
				return inlineLink{}, false
			}
			return finishInlineLink(runes, from, pos, runes[start:pos])
		}
	}
	return inlineLink{}, false
}

func finishInlineLink(runes []rune, from, pos int, target []rune) (inlineLink, bool) {
	if pos < len(runes) && runes[pos] == ')' {
		return inlineLink{from: from, to: pos + 1, target: markdownText(target)}, true
	}
	if pos >= len(runes) || !isLinkSpace(runes[pos]) {
		return inlineLink{}, false
	}
	for pos < len(runes) && isLinkSpace(runes[pos]) {
		pos++
	}
	if pos < len(runes) && runes[pos] == ')' {
		return inlineLink{from: from, to: pos + 1, target: markdownText(target)}, true
	}
	if pos >= len(runes) {
		return inlineLink{}, false
	}

	close := runes[pos]
	switch close {
	case '\'', '"':
	case '(':
		close = ')'
	default:
		return inlineLink{}, false
	}
	for pos++; pos < len(runes); pos++ {
		if isMarkdownEscape(runes, pos) {
			pos++
			continue
		}
		if runes[pos] != close {
			continue
		}
		pos++
		for pos < len(runes) && isLinkSpace(runes[pos]) {
			pos++
		}
		if pos < len(runes) && runes[pos] == ')' {
			return inlineLink{from: from, to: pos + 1, target: markdownText(target)}, true
		}
		return inlineLink{}, false
	}
	return inlineLink{}, false
}

func isLinkSpace(r rune) bool { return r == ' ' || r == '\t' }

func isEscaped(runes []rune, pos int) bool {
	backslashes := 0
	for pos--; pos >= 0 && runes[pos] == '\\'; pos-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func isMarkdownEscape(runes []rune, pos int) bool {
	return pos >= 0 && pos+1 < len(runes) && runes[pos] == '\\' && isASCIIPunctuation(runes[pos+1])
}

func isASCIIPunctuation(r rune) bool {
	return r >= '!' && r <= '/' || r >= ':' && r <= '@' || r >= '[' && r <= '`' || r >= '{' && r <= '~'
}

func markdownText(runes []rune) string {
	text := make([]rune, 0, len(runes))
	for pos := 0; pos < len(runes); pos++ {
		if isMarkdownEscape(runes, pos) {
			pos++
		}
		text = append(text, runes[pos])
	}
	return string(text)
}

// inlineLinkAt finds a link whose complete Markdown span covers col.
func inlineLinkAt(runes []rune, col int) (inlineLink, bool) {
	for _, link := range inlineLinks(runes) {
		if col >= link.from && col < link.to {
			return link, true
		}
	}
	return inlineLink{}, false
}
