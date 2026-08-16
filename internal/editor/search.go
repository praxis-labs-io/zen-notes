package editor

import "strings"

// search is the live pattern. Matches are plain substrings rather than regular
// expressions, which is what reads naturally when searching prose.
type search struct {
	pattern string
	matches []Pos
}

// SearchPattern is the pattern currently highlighted, empty when none is.
func (e *Editor) SearchPattern() string { return e.search.pattern }

// startSearch opens the / line.
func (e *Editor) startSearch() {
	e.mode = ModeSearch
	e.cmdline = nil
}

// searchKey handles typing in the / line.
func (e *Editor) searchKey(k Key) {
	switch k.Name {
	case "esc":
		e.mode = ModeNormal
		e.cmdline = nil
		e.clearSearch()
		return
	case "enter", "cr":
		pattern := string(e.cmdline)
		e.mode = ModeNormal
		e.cmdline = nil
		e.runSearch(pattern)
		return
	case "backspace", "bs":
		if len(e.cmdline) > 0 {
			e.cmdline = e.cmdline[:len(e.cmdline)-1]
		}
		return
	case "":
	default:
		return
	}
	if k.R != 0 {
		e.cmdline = append(e.cmdline, k.R)
	}
}

// runSearch accepts a pattern and jumps to the first match after the cursor.
// An empty pattern reuses the last one, as in vim.
func (e *Editor) runSearch(pattern string) {
	if pattern == "" {
		pattern = e.search.pattern
	}
	if pattern == "" {
		return
	}

	e.search.pattern = pattern
	e.search.matches = e.findMatches(pattern)
	if len(e.search.matches) == 0 {
		e.message = "not found: " + pattern
		return
	}
	e.jumpToMatch(false)
}

func (e *Editor) clearSearch() {
	e.search = search{}
}

// findMatches lists the start of every match, top to bottom. The search is
// case insensitive unless the pattern itself carries a capital.
func (e *Editor) findMatches(pattern string) []Pos {
	sensitive := pattern != strings.ToLower(pattern)
	needle := pattern
	if !sensitive {
		needle = strings.ToLower(pattern)
	}

	var out []Pos
	for line := range e.buf.LineCount() {
		hay := e.buf.Line(line)
		if !sensitive {
			hay = strings.ToLower(hay)
		}
		for at := 0; ; {
			i := strings.Index(hay[at:], needle)
			if i < 0 {
				break
			}
			out = append(out, Pos{line, runeLen(hay[:at+i])})
			at += i + len(needle)
		}
	}
	return out
}

// jumpToMatch moves to the match after the cursor, or before it when going
// backward, wrapping around the buffer and saying so when it does.
func (e *Editor) jumpToMatch(backward bool) {
	matches := e.search.matches
	if len(matches) == 0 {
		return
	}

	if backward {
		for i := len(matches) - 1; i >= 0; i-- {
			if matches[i].Before(e.cursor) {
				e.moveToMatch(matches[i], false)
				return
			}
		}
		e.moveToMatch(matches[len(matches)-1], true)
		return
	}

	for _, m := range matches {
		if e.cursor.Before(m) {
			e.moveToMatch(m, false)
			return
		}
	}
	e.moveToMatch(matches[0], true)
}

func (e *Editor) moveToMatch(p Pos, wrapped bool) {
	e.cursor = e.buf.Clamp(p)
	e.clampCursor()
	e.desiredCol = e.cursor.Col
	if wrapped {
		e.message = "wrapped"
	}
}

// refreshMatches recomputes matches against the current text, so editing does
// not leave the highlight pointing at stale positions.
func (e *Editor) refreshMatches() {
	if e.search.pattern != "" {
		e.search.matches = e.findMatches(e.search.pattern)
	}
}

// matchCovers reports whether p sits inside a highlighted match.
func (e *Editor) matchCovers(p Pos) bool {
	n := runeLen(e.search.pattern)
	if n == 0 {
		return false
	}
	for _, m := range e.search.matches {
		if m.Line == p.Line && p.Col >= m.Col && p.Col < m.Col+n {
			return true
		}
	}
	return false
}
