package editor

import "regexp"

var inlineLinkRe = regexp.MustCompile(`\[([^\]\n]*)\]\(([^)\n]*)\)`)

type inlineLink struct {
	from, to int
	target   string
}

// inlineLinks returns the Markdown links on one line with rune-based spans.
func inlineLinks(runes []rune) []inlineLink {
	line := string(runes)
	matches := inlineLinkRe.FindAllStringSubmatchIndex(line, -1)
	links := make([]inlineLink, 0, len(matches))
	for _, match := range matches {
		links = append(links, inlineLink{
			from:   runeLen(line[:match[0]]),
			to:     runeLen(line[:match[1]]),
			target: line[match[4]:match[5]],
		})
	}
	return links
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
