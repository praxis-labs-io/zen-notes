package editor

import (
	"regexp"
	"unicode/utf8"
)

// Classes rather than styles so the classifier is testable without comparing escape sequences.
type tokenClass int

const (
	tokPlain tokenClass = iota
	tokHeading
	tokStrong
	tokEmphasis
	tokCode
	tokMarker
	tokCheckDone
	tokCheckTodo
	tokLink
)

var (
	headingRe  = regexp.MustCompile(`^\s*#{1,6}\s`)
	quoteRe    = regexp.MustCompile(`^(\s*)(>+)(\s)`)
	fenceRe    = regexp.MustCompile("^\\s*(```|~~~)")
	strongRe   = regexp.MustCompile(`\*\*[^*\n]+\*\*|__[^_\n]+__`)
	emphasisRe = regexp.MustCompile(`\*[^*\n]+\*|_[^_\n]+_`)
	codeRe     = regexp.MustCompile("`[^`\n]*`")
)

// Order matters: earlier patterns claim their runes first.
var inlinePatterns = []struct {
	re    *regexp.Regexp
	class tokenClass
}{
	{codeRe, tokCode},
	{strongRe, tokStrong},
	{emphasisRe, tokEmphasis},
}

func classifyBuffer(b *Buffer) [][]tokenClass {
	out := make([][]tokenClass, b.LineCount())
	inFence := false
	for i := range b.LineCount() {
		runes := b.runes(i)
		fence := fenceRe.MatchString(string(runes))
		out[i] = classifyLine(runes, inFence && !fence)
		if fence {
			inFence = !inFence
		}
	}
	return out
}

func classifyLine(runes []rune, inFence bool) []tokenClass {
	classes := make([]tokenClass, len(runes))
	if len(runes) == 0 {
		return classes
	}
	if inFence {
		fill(classes, 0, len(classes), tokCode)
		return classes
	}

	line := string(runes)
	if fenceRe.MatchString(line) {
		fill(classes, 0, len(classes), tokMarker)
		return classes
	}
	if headingRe.MatchString(line) {
		fill(classes, 0, len(classes), tokHeading)
		return classes
	}

	rest := markLinePrefix(runes, classes)
	markInline(runes, classes, rest)
	return classes
}

func markLinePrefix(runes []rune, classes []tokenClass) int {
	item, isList := parseListLine(runes)
	if isList {
		if item.markerEnd > item.indent {
			fill(classes, item.indent, item.markerEnd, tokMarker)
		}
		if item.task {
			markCheckbox(runes, classes, item.taskAt)
			return item.contentStart
		}
		if hasTaskMarker(runes, item.contentStart) {
			if contentStart, ok := skipRequiredSpace(runes, item.contentStart+3); ok {
				markCheckbox(runes, classes, item.contentStart)
				return contentStart
			}
		}
		return item.markerEnd
	}

	line := string(runes)
	start := 0
	if m := quoteRe.FindStringSubmatchIndex(line); m != nil {
		start = utf8.RuneCountInString(line[:m[1]])
		fill(classes, utf8.RuneCountInString(line[:m[4]]), start, tokMarker)
	}
	if hasTaskMarker(runes, start) {
		if contentStart, ok := skipRequiredSpace(runes, start+3); ok {
			markCheckbox(runes, classes, start)
			return contentStart
		}
	}
	return start
}

func markCheckbox(runes []rune, classes []tokenClass, at int) {
	class := tokCheckTodo
	if runes[at+1] != ' ' {
		class = tokCheckDone
	}
	fill(classes, at, at+3, class)
}

// Matches only still-plain runs so a star left from a bold span cannot pair across it.
func markInline(runes []rune, classes []tokenClass, from int) {
	for _, link := range inlineLinks(runes) {
		start := link.from
		if start > from && runes[start-1] == '!' && !isEscaped(runes, start-1) {
			start--
		}
		if start >= from {
			fill(classes, start, link.to, tokLink)
		}
	}
	for _, p := range inlinePatterns {
		for _, seg := range plainRuns(classes, from) {
			text := string(runes[seg[0]:seg[1]])
			for _, m := range p.re.FindAllStringIndex(text, -1) {
				lo := seg[0] + utf8.RuneCountInString(text[:m[0]])
				fill(classes, lo, seg[0]+utf8.RuneCountInString(text[:m[1]]), p.class)
			}
		}
	}
}

func plainRuns(classes []tokenClass, from int) [][2]int {
	var runs [][2]int
	start := -1
	for i := from; i < len(classes); i++ {
		if classes[i] == tokPlain {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			runs = append(runs, [2]int{start, i})
			start = -1
		}
	}
	if start >= 0 {
		runs = append(runs, [2]int{start, len(classes)})
	}
	return runs
}

func fill(classes []tokenClass, from, to int, c tokenClass) {
	for i := max(from, 0); i < to && i < len(classes); i++ {
		classes[i] = c
	}
}
