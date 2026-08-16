package editor

import "strings"

// search is the live pattern. Matches are plain substrings rather than regular
// expressions, which is what reads naturally when searching prose.
type search struct {
	pattern  string
	matches  []Pos
	previous string
	origin   Pos
}

// SearchPattern is the pattern currently highlighted, empty when none is.
func (e *Editor) SearchPattern() string { return e.search.pattern }

// startSearch opens the / line, remembering where to put the cursor back if
// the search is abandoned.
func (e *Editor) startSearch() {
	e.mode = ModeSearch
	e.cmdline = nil
	e.search.origin = e.cursor
}

// searchKey handles typing in the / line. Every keystroke re-runs the search,
// so the cursor and the highlight follow along as the pattern is built.
func (e *Editor) searchKey(k Key) {
	switch k.Name {
	case "esc":
		e.mode = ModeNormal
		e.cmdline = nil
		e.cursor = e.buf.Clamp(e.search.origin)
		e.clearSearch()
		return
	case "enter", "cr":
		e.commitSearch(string(e.cmdline))
		return
	case "backspace", "bs":
		if len(e.cmdline) > 0 {
			e.cmdline = e.cmdline[:len(e.cmdline)-1]
		}
		e.preview()
		return
	case "":
	default:
		return
	}
	if k.R != 0 {
		e.cmdline = append(e.cmdline, k.R)
		e.preview()
	}
}

// preview runs the half-typed pattern without reporting anything, since a
// message on every keystroke would be noise.
func (e *Editor) preview() {
	pattern := string(e.cmdline)
	if pattern == "" {
		e.search.pattern, e.search.matches = "", nil
		e.cursor = e.buf.Clamp(e.search.origin)
		return
	}

	e.search.pattern = pattern
	e.search.matches = e.findMatches(pattern)
	if len(e.search.matches) == 0 {
		e.cursor = e.buf.Clamp(e.search.origin)
		return
	}
	e.jumpFrom(e.search.origin, false, true)
}

// commitSearch accepts the pattern. An empty one reuses the last, as in vim.
func (e *Editor) commitSearch(pattern string) {
	e.mode = ModeNormal
	e.cmdline = nil

	if pattern == "" {
		pattern = e.search.previous
	}
	if pattern == "" {
		e.clearSearch()
		return
	}

	e.search.pattern = pattern
	e.search.previous = pattern
	e.search.matches = e.findMatches(pattern)
	if len(e.search.matches) == 0 {
		e.cursor = e.buf.Clamp(e.search.origin)
		e.message = "not found: " + pattern
		return
	}
	e.jumpFrom(e.search.origin, false, false)
}

func (e *Editor) clearSearch() {
	e.search.pattern, e.search.matches = "", nil
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

// jumpToMatch is n and N: step to the match either side of the cursor.
func (e *Editor) jumpToMatch(backward bool) {
	e.jumpFrom(e.cursor, backward, false)
}

// jumpFrom moves to the match either side of from, wrapping around the buffer
// and saying so unless asked to stay quiet.
func (e *Editor) jumpFrom(from Pos, backward, quiet bool) {
	matches := e.search.matches
	if len(matches) == 0 {
		return
	}

	if backward {
		for i := len(matches) - 1; i >= 0; i-- {
			if matches[i].Before(from) {
				e.moveToMatch(matches[i], false, quiet)
				return
			}
		}
		e.moveToMatch(matches[len(matches)-1], true, quiet)
		return
	}

	for _, m := range matches {
		if from.Before(m) {
			e.moveToMatch(m, false, quiet)
			return
		}
	}
	e.moveToMatch(matches[0], true, quiet)
}

func (e *Editor) moveToMatch(p Pos, wrapped, quiet bool) {
	e.cursor = e.buf.Clamp(p)
	e.clampCursor()
	e.desiredCol = e.cursor.Col
	if wrapped && !quiet {
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
