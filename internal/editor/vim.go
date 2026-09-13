package editor

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeVisual
	ModeVisualLine
	ModeVisualBlock
	ModeCommand
	ModeSearch
)

func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeVisualLine:
		return "V-LINE"
	case ModeVisualBlock:
		return "V-BLOCK"
	case ModeCommand:
		return "COMMAND"
	case ModeSearch:
		return "SEARCH"
	default:
		return "NORMAL"
	}
}

func (m Mode) Visual() bool {
	return m == ModeVisual || m == ModeVisualLine || m == ModeVisualBlock
}

// Key is one keystroke: a literal rune R, or a named key such as "esc" or "c-d".
type Key struct {
	R    rune
	Name string
}

func Rune(r rune) Key { return Key{R: r} }

func Named(name string) Key { return Key{Name: name} }

type motionKind int

const (
	charExclusive motionKind = iota
	charInclusive
	linewise
)

type motion struct {
	target Pos
	kind   motionKind
}

type register struct {
	text     string
	linewise bool
	block    bool
}

type find struct {
	kind   rune
	target rune
}

type blockPending struct {
	active     bool
	firstLine  int
	lastLine   int
	col        int
	appendEnd  bool
	insertedAt Pos
}

type flashRange struct {
	active   bool
	from, to Pos
	linewise bool
	block    bool
}

type snapshot struct {
	lines  [][]rune
	cursor Pos
}

type pending struct {
	count1 int
	op     rune
	count2 int
	await  rune
	keys   []rune
}

func (p pending) count() int {
	c := 1
	if p.count1 > 0 {
		c *= p.count1
	}
	if p.count2 > 0 {
		c *= p.count2
	}
	return c
}

type Editor struct {
	buf    *Buffer
	cursor Pos
	mode   Mode

	pend        pending
	reg         register
	visualStart Pos
	cmdline     []rune
	message     string
	lastFind    find
	blockInsert blockPending
	search      search

	undo []snapshot
	redo []snapshot

	desiredCol            int
	desiredScreenCol      int
	screenColSet          bool
	applyingDisplayMotion bool
	layoutWidth           int
	top                   int
	rows                  []vrow
	cursorRow             int
	lastVisual            [2]Pos
	height                int
	dirty                 bool
	selection             lipgloss.Style
	flashStyle            lipgloss.Style
	matchStyle            lipgloss.Style
	cursorLine            color.Color
	flash                 flashRange
	quit                  bool
	saveWanted            bool
	clipboard             string
	clipboardWanted       bool
	openLink              string
	openLinkWanted        bool
	images                map[string]ImagePlacement
}

const undoDepth = 200

func New(text string) *Editor {
	return &Editor{
		buf:        NewBuffer(text),
		mode:       ModeNormal,
		selection:  darkSelection,
		flashStyle: darkFlash,
		matchStyle: darkMatch,
		cursorLine: darkCursorLine,
		height:     20,
	}
}

func (e *Editor) Text() string { return e.buf.Text() }

func (e *Editor) LineCount() int { return e.buf.LineCount() }

func (e *Editor) Cursor() Pos { return e.cursor }

// SetCursor moves the caret to p, clamped to the buffer.
func (e *Editor) SetCursor(p Pos) {
	e.cursor = e.buf.Clamp(p)
	e.clampCursor()
	e.desiredCol = e.cursor.Col
	e.screenColSet = false
}

func (e *Editor) Mode() Mode { return e.mode }

// Dirty reports whether the buffer changed since the last MarkSaved or SetText.
func (e *Editor) Dirty() bool { return e.dirty }

func (e *Editor) MarkSaved() { e.dirty = false }

// SetHeight sets the window height that paging and scroll commands use. Non-positive n is ignored.
func (e *Editor) SetHeight(n int) {
	if n > 0 {
		e.height = n
	}
}

func (e *Editor) QuitRequested() bool { return e.quit }

func (e *Editor) TakeSaveRequest() bool {
	want := e.saveWanted
	e.saveWanted = false
	return want
}

func (e *Editor) TakeClipboardRequest() (string, bool) {
	text, wanted := e.clipboard, e.clipboardWanted
	e.clipboard, e.clipboardWanted = "", false
	return text, wanted
}

func (e *Editor) TakeOpenLinkRequest() (string, bool) {
	target, wanted := e.openLink, e.openLinkWanted
	e.openLink, e.openLinkWanted = "", false
	return target, wanted
}

func (e *Editor) Message() string { return e.message }

func (e *Editor) ClearMessage() { e.message = "" }

// CommandLine returns the ":" or "/" line being typed, prefix included, or "" in other modes.
func (e *Editor) CommandLine() string {
	switch e.mode {
	case ModeCommand:
		return ":" + string(e.cmdline)
	case ModeSearch:
		return "/" + string(e.cmdline)
	}
	return ""
}

func (e *Editor) PendingKeys() string { return string(e.pend.keys) }

// SetText replaces the buffer, clamps the cursor, and clears undo history and the dirty flag.
// In insert mode it seeds one undo point for the insert in progress. Mode is left alone; see Reset.
func (e *Editor) SetText(text string) {
	e.buf = NewBuffer(text)
	e.cursor = e.buf.Clamp(e.cursor)
	e.clampCursor()
	e.undo, e.redo = nil, nil
	if e.mode == ModeInsert {
		e.undo = append(e.undo, snapshot{lines: e.buf.Lines(), cursor: e.cursor})
	}
	e.dirty = false
	e.refreshMatches()
}

// Reset drops the pending command, command line, visual range, find and search, for use with
// SetText when a different note is swapped in. Insert mode is kept.
func (e *Editor) Reset() {
	if e.mode != ModeInsert {
		e.mode = ModeNormal
	}
	e.pend = pending{}
	e.visualStart = Pos{}
	e.lastVisual = [2]Pos{}
	e.blockInsert = blockPending{}
	e.flash = flashRange{}
	e.cmdline = nil
	e.lastFind = find{}
	e.message = ""
	e.clearSearch()
	e.search.previous = ""
	e.search.origin = Pos{}
}

// Selection returns the visual range in buffer order and whether it is linewise. Meaningless outside visual modes.
func (e *Editor) Selection() (Pos, Pos, bool) {
	from, to := e.visualStart, e.cursor
	if to.Before(from) {
		from, to = to, from
	}
	return from, to, e.mode == ModeVisualLine
}

func (e *Editor) Feed(k Key) {
	switch e.mode {
	case ModeInsert:
		e.screenColSet = false
		e.insertKey(k)
	case ModeCommand:
		e.screenColSet = false
		e.commandKey(k)
	case ModeSearch:
		e.screenColSet = false
		e.searchKey(k)
	default:
		e.normalKey(k)
	}
}

// Paste loads text into the register and inserts it for the current mode. A trailing newline makes it linewise.
func (e *Editor) Paste(text string) {
	if text == "" {
		return
	}

	e.screenColSet = false
	reg := clipboardRegister(text)
	switch {
	case e.mode == ModeInsert:
		e.setRegister(reg)
		e.cursor = e.buf.Insert(e.cursor, text)
		e.dirty = true
	case e.mode.Visual():
		e.pasteVisual(reg)
	case e.mode == ModeNormal:
		e.setRegister(reg)
		e.put(true)
	}
}

func clipboardRegister(text string) register {
	if strings.HasSuffix(text, "\n") {
		return register{text: strings.TrimSuffix(text, "\n"), linewise: true}
	}
	return register{text: text}
}

func (e *Editor) setRegister(reg register) {
	e.reg = reg
	e.requestClipboard(reg)
}

func (e *Editor) requestClipboard(reg register) {
	e.clipboard, e.clipboardWanted = "", false
	if reg.text == "" && !reg.linewise {
		return
	}
	e.clipboard = reg.text
	if reg.linewise {
		e.clipboard += "\n"
	}
	e.clipboardWanted = true
}

func (e *Editor) clampCursor() {
	e.cursor = e.buf.Clamp(e.cursor)
	if e.mode == ModeInsert {
		return
	}
	if n := e.buf.LineLen(e.cursor.Line); e.cursor.Col >= n && n > 0 {
		e.cursor.Col = n - 1
	}
}

func (e *Editor) snapshot() {
	e.flash = flashRange{}
	e.undo = append(e.undo, snapshot{lines: e.buf.Lines(), cursor: e.cursor})
	if len(e.undo) > undoDepth {
		e.undo = e.undo[len(e.undo)-undoDepth:]
	}
	e.redo = nil
	e.dirty = true
}

func (e *Editor) restore(from *[]snapshot, to *[]snapshot) {
	if len(*from) == 0 {
		return
	}
	s := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, snapshot{lines: e.buf.Lines(), cursor: e.cursor})
	e.buf.SetLines(s.lines)
	e.cursor = s.cursor
	e.clampCursor()
	e.dirty = true
}

func (e *Editor) insertKey(k Key) {
	switch k.Name {
	case "esc":
		e.finishBlockInsert()
		e.mode = ModeNormal
		if e.cursor.Col > 0 {
			e.cursor.Col--
		}
		e.clampCursor()
		return
	case "enter", "cr":
		e.smartEnter()
		return
	case "backspace", "bs":
		e.backspace()
		return
	case "tab":
		if !e.shiftListItem(1) {
			e.cursor = e.buf.Insert(e.cursor, "\t")
			e.dirty = true
		}
		return
	case "backtab":
		e.shiftListItem(-1)
		return
	case "up", "down", "left", "right", "home", "end", "pgup", "pgdown":
		e.insertMove(k.Name)
		return
	case "delete", "del":
		e.forwardDelete()
		return
	case "":
	default:
		return
	}
	if k.R == 0 {
		return
	}
	e.cursor = e.buf.Insert(e.cursor, string(k.R))
	e.separateNewList()
	e.dirty = true
}

func (e *Editor) smartEnter() {
	edit := enterEdit(e.buf.runes(e.cursor.Line), e.cursor.Col, e.previousLineIsList())
	e.applyInsertEdit(edit)
	e.dirty = true
}

func (e *Editor) applyInsertEdit(edit insertEdit) {
	if edit.deleteTo > 0 {
		e.buf.Delete(Pos{e.cursor.Line, 0}, Pos{e.cursor.Line, edit.deleteTo})
		e.cursor.Col = max(e.cursor.Col-edit.deleteTo, 0)
	}
	if edit.insert != "" {
		e.cursor = e.buf.Insert(e.cursor, edit.insert)
	}
}

func (e *Editor) previousLineIsList() bool {
	if e.cursor.Line == 0 {
		return false
	}
	line := e.buf.runes(e.cursor.Line - 1)
	item, ok := parseListLine(line)
	return ok && !onlySpace(line[item.contentStart:])
}

func (e *Editor) separateNewList() {
	if e.cursor.Line == 0 || !needsListSeparator(e.buf.runes(e.cursor.Line), e.buf.runes(e.cursor.Line-1)) {
		return
	}
	col := e.cursor.Col
	e.buf.Insert(Pos{e.cursor.Line, 0}, "\n")
	e.cursor = Pos{e.cursor.Line + 1, col}
}

func (e *Editor) backspace() {
	if edit, ok := backspaceEdit(e.buf.runes(e.cursor.Line), e.cursor.Col, e.previousLineIsList()); ok {
		e.applyInsertEdit(edit)
		e.dirty = true
		return
	}
	if e.cursor.Col > 0 {
		e.buf.Delete(Pos{e.cursor.Line, e.cursor.Col - 1}, e.cursor)
		e.cursor.Col--
		e.dirty = true
		return
	}
	if e.cursor.Line == 0 {
		return
	}
	prevLen := e.buf.LineLen(e.cursor.Line - 1)
	e.buf.Delete(Pos{e.cursor.Line - 1, prevLen}, e.cursor)
	e.cursor = Pos{e.cursor.Line - 1, prevLen}
	e.dirty = true
}

func (e *Editor) insertMove(name string) {
	switch name {
	case "up":
		e.moveVertical(-1)
	case "down":
		e.moveVertical(1)
	case "pgup":
		e.moveVertical(-e.pageSize())
	case "pgdown":
		e.moveVertical(e.pageSize())
	case "left":
		if e.cursor.Col > 0 {
			e.cursor.Col--
		}
		e.desiredCol = e.cursor.Col
	case "right":
		e.cursor.Col++
		e.clampCursor()
		e.desiredCol = e.cursor.Col
	case "home":
		e.cursor.Col = 0
		e.desiredCol = 0
	case "end":
		e.cursor.Col = e.buf.LineLen(e.cursor.Line)
		e.desiredCol = e.cursor.Col
	}
}

func (e *Editor) forwardDelete() {
	if e.cursor == e.buf.End() {
		return
	}
	to := e.forwardOne(e.cursor)
	if to == e.cursor {
		to = Pos{e.cursor.Line + 1, 0}
	}
	e.buf.Delete(e.cursor, to)
	e.dirty = true
}

func (e *Editor) moveVertical(delta int) {
	line := e.cursor.Line + delta
	if line < 0 {
		line = 0
	}
	if line >= e.buf.LineCount() {
		line = e.buf.LineCount() - 1
	}
	e.cursor.Line = line
	e.cursor.Col = e.desiredCol
	e.clampCursor()
}

func (e *Editor) commandKey(k Key) {
	switch k.Name {
	case "esc":
		e.mode = ModeNormal
		e.cmdline = nil
		return
	case "enter", "cr":
		e.runCommand(string(e.cmdline))
		e.mode = ModeNormal
		e.cmdline = nil
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

func (e *Editor) runCommand(cmd string) {
	switch strings.TrimSpace(cmd) {
	case "q", "q!":
		e.quit = true
	case "w":
		e.saveWanted = true
	case "wq", "x":
		e.saveWanted = true
		e.quit = true
	case "":
	default:
		e.message = "not a command: " + cmd
	}
}

func (e *Editor) normalKey(k Key) {
	if k.Name != "" {
		stands, ok := namedKeyCommand(k.Name, e.mode)
		if !ok {
			e.namedNormalKey(k.Name)
			return
		}
		if e.pend.await != 0 {
			e.pend = pending{}
			return
		}
		e.runNormal(stands, false)
		return
	}
	if k.R != 0 {
		e.runNormal(k.R, true)
	}
}

// countable is false for named keys, so Home standing in for 0 never reads as a count.
func (e *Editor) runNormal(r rune, countable bool) {
	e.pend.keys = append(e.pend.keys, r)

	if e.pend.await != 0 {
		e.resolveAwait(r)
		return
	}
	if countable && e.digit(r) {
		return
	}
	if e.operator(r) {
		return
	}
	if e.mode.Visual() && e.visualCommand(r) {
		return
	}
	if m, ok := e.resolveMotion(r); ok {
		e.applyMotion(m)
		return
	}
	if e.pend.await != 0 {
		return
	}
	e.command(r)
	if e.pend.await == 0 {
		e.pend = pending{}
	}
}

func (e *Editor) namedNormalKey(name string) {
	if name != "c-v" {
		e.screenColSet = false
	}
	switch name {
	case "esc":
		e.pend = pending{}
		if e.mode.Visual() {
			e.rememberVisual()
			e.mode = ModeNormal
			return
		}
		e.clearSearch()
	case "c-d":
		e.halfPage(1)
	case "c-u":
		e.halfPage(-1)
	case "pgup":
		e.page(-1)
	case "pgdown":
		e.page(1)
	case "c-r":
		e.restore(&e.redo, &e.undo)
	case "c-v":
		e.startVisual(ModeVisualBlock)
		e.pend = pending{}
	case "copy":
		if e.mode.Visual() {
			e.applyVisual('y')
		}
	}
}

// Delete maps to d in visual mode because x only ever takes the rune under the caret.
func namedKeyCommand(name string, mode Mode) (rune, bool) {
	switch name {
	case "up":
		return 'k', true
	case "down":
		return 'j', true
	case "left":
		return 'h', true
	case "right":
		return 'l', true
	case "home":
		return '0', true
	case "end":
		return '$', true
	case "delete", "del":
		if mode.Visual() {
			return 'd', true
		}
		return 'x', true
	}
	return 0, false
}

func (e *Editor) halfPage(dir int) {
	e.moveVertical(dir * max(e.height/2, 1))
	e.pend = pending{}
}

// Two logical lines short of a screen, as vim's C-f and C-b; wrapped lines break the overlap.
func (e *Editor) pageSize() int { return max(e.height-2, 1) }

func (e *Editor) page(dir int) {
	e.moveVertical(dir * e.pageSize())
	e.pend = pending{}
}

func (e *Editor) digit(r rune) bool {
	if r < '0' || r > '9' {
		return false
	}
	if r == '0' && e.activeCount() == 0 {
		return false
	}
	d := int(r - '0')
	if e.pend.op == 0 {
		e.pend.count1 = e.pend.count1*10 + d
	} else {
		e.pend.count2 = e.pend.count2*10 + d
	}
	return true
}

func (e *Editor) activeCount() int {
	if e.pend.op == 0 {
		return e.pend.count1
	}
	return e.pend.count2
}

func (e *Editor) operator(r rune) bool {
	if op, ok := caseOp(r); ok && e.pend.op == op {
		last := min(e.cursor.Line+e.pend.count()-1, e.buf.LineCount()-1)
		e.applyOperator(op, motion{target: Pos{last, 0}, kind: linewise})
		e.screenColSet = false
		e.pend = pending{}
		return true
	}
	switch r {
	case '>':
		r = opIndent
	case '<':
		r = opDedent
	}
	if r != 'd' && r != 'c' && r != 'y' && r != opIndent && r != opDedent {
		return false
	}
	if e.mode.Visual() {
		e.applyVisual(r)
		e.screenColSet = false
		return true
	}
	if e.pend.op == r {
		line := e.cursor.Line
		last := min(line+e.pend.count()-1, e.buf.LineCount()-1)
		e.applyOperator(r, motion{target: Pos{last, 0}, kind: linewise})
		e.screenColSet = false
		e.pend = pending{}
		return true
	}
	if e.pend.op != 0 {
		e.screenColSet = false
		e.pend = pending{}
		return true
	}
	e.pend.op = r
	return true
}

// cw resolves as ce, as in vim, leaving the blank after the word.
func (e *Editor) resolveMotion(r rune) (motion, bool) {
	n := e.pend.count()
	cur := e.cursor

	switch r {
	case 'h':
		return motion{Pos{cur.Line, max(cur.Col-n, 0)}, charExclusive}, true
	case 'l':
		return motion{Pos{cur.Line, min(cur.Col+n, e.buf.LineLen(cur.Line))}, charExclusive}, true
	case 'j':
		return motion{Pos{min(cur.Line+n, e.buf.LineCount()-1), e.desiredCol}, linewise}, true
	case 'k':
		return motion{Pos{max(cur.Line-n, 0), e.desiredCol}, linewise}, true
	case '0':
		return motion{Pos{cur.Line, 0}, charExclusive}, true
	case '^':
		return motion{Pos{cur.Line, firstNonBlank(e.buf, cur.Line)}, charExclusive}, true
	case '$':
		return motion{Pos{cur.Line, max(e.buf.LineLen(cur.Line)-1, 0)}, charInclusive}, true
	case 'w', 'W':
		big := r == 'W'
		if e.pend.op == 'c' && classAt(e.buf, cur) != classBlank {
			return motion{wordEnd(e.buf, cur, n, big), charInclusive}, true
		}
		return motion{wordForward(e.buf, cur, n, big), charExclusive}, true
	case 'b':
		return motion{wordBack(e.buf, cur, n, false), charExclusive}, true
	case 'B':
		return motion{wordBack(e.buf, cur, n, true), charExclusive}, true
	case 'e':
		return motion{wordEnd(e.buf, cur, n, false), charInclusive}, true
	case 'E':
		return motion{wordEnd(e.buf, cur, n, true), charInclusive}, true
	case '{':
		return motion{paragraphBack(e.buf, cur, n), charExclusive}, true
	case '}':
		return motion{paragraphForward(e.buf, cur, n), charExclusive}, true
	case 'G':
		line := e.buf.LineCount() - 1
		if e.pend.count1 > 0 {
			line = min(e.pend.count1-1, line)
		}
		return motion{Pos{line, 0}, linewise}, true
	case 'H', 'M', 'L':
		return e.screenMotion(r, n)
	case '%':
		if target, ok := matchBracket(e.buf, cur); ok {
			return motion{target, charInclusive}, true
		}
		e.pend = pending{}
		return motion{}, false
	case 'g', 'f', 'F', 't', 'T', 'r', 'z':
		e.pend.await = r
		return motion{}, false
	case 'i', 'a':
		if e.pend.op != 0 || e.mode.Visual() {
			e.pend.await = r
			return motion{}, false
		}
	case ';':
		e.repeatFind(false)
		return motion{}, false
	case ',':
		e.repeatFind(true)
		return motion{}, false
	}
	return motion{}, false
}

func (e *Editor) applyTextObject(around bool, object rune) {
	span, ok := resolveTextObject(e.buf, e.cursor, around, object)
	if !ok {
		e.pend = pending{}
		return
	}

	if e.mode.Visual() {
		e.visualStart = span.from
		e.cursor = e.buf.Clamp(span.to)
		e.pend = pending{}
		return
	}

	op := e.pend.op
	e.pend = pending{}
	if op == 0 {
		return
	}
	e.cursor = span.from
	if span.linewise {
		e.operateLines(op, span.from.Line, span.to.Line)
		return
	}
	e.operateChars(op, span.from, e.forwardOne(span.to))
}

func (e *Editor) resolveAwait(r rune) {
	await := e.pend.await
	e.pend.await = 0

	if await == 'Z' {
		if r == 'Z' {
			e.saveWanted = true
			e.quit = true
		}
		e.pend = pending{}
		return
	}

	if await == 'g' {
		switch r {
		case 'g':
			line := 0
			if e.pend.count1 > 0 {
				line = min(e.pend.count1-1, e.buf.LineCount()-1)
			}
			e.applyMotion(motion{Pos{line, 0}, linewise})
		case 'j':
			e.applyDisplayMotion(1, e.pend.count())
		case 'k':
			e.applyDisplayMotion(-1, e.pend.count())
		case 'x':
			e.screenColSet = false
			e.requestLinkUnderCursor()
			e.pend = pending{}
		case 'U', 'u', '~':
			e.pend.op, _ = caseOp(r)
		case 'v':
			e.screenColSet = false
			e.reselect()
			e.pend = pending{}
		default:
			e.screenColSet = false
			e.pend = pending{}
		}
		return
	}

	if await == 'i' || await == 'a' {
		e.applyTextObject(await == 'a', r)
		return
	}

	if await == 'r' {
		e.replaceRunes(r, e.pend.count())
		e.pend = pending{}
		return
	}

	if await == 'z' {
		e.scrollTop(r)
		e.pend = pending{}
		return
	}

	e.lastFind = find{kind: await, target: r}
	e.applyFind(await, r, e.pend.count())
}

func (e *Editor) requestLinkUnderCursor() {
	if e.mode != ModeNormal || e.pend.op != 0 {
		return
	}
	link, ok := inlineLinkAt(e.buf.runes(e.cursor.Line), e.cursor.Col)
	if !ok {
		e.message = "no link under cursor"
		return
	}
	e.openLink, e.openLinkWanted = link.target, true
}

func (e *Editor) applyFind(kind, target rune, n int) {
	var col int
	var ok bool
	switch kind {
	case 'f':
		col, ok = findForward(e.buf, e.cursor, target, false, n)
	case 't':
		col, ok = findForward(e.buf, e.cursor, target, true, n)
	case 'F':
		col, ok = findBack(e.buf, e.cursor, target, false, n)
	case 'T':
		col, ok = findBack(e.buf, e.cursor, target, true, n)
	}
	if !ok {
		e.pend = pending{}
		return
	}

	motionKind := charInclusive
	if kind == 'F' || kind == 'T' {
		motionKind = charExclusive
	}
	e.applyMotion(motion{Pos{e.cursor.Line, col}, motionKind})
}

// A t or T repeat starts one rune along, or it would find the rune the cursor already rests against.
func (e *Editor) repeatFind(reverse bool) {
	if e.lastFind.kind == 0 {
		e.pend = pending{}
		return
	}
	kind := e.lastFind.kind
	if reverse {
		kind = flipFind(kind)
	}

	n := e.pend.count()
	saved := e.cursor
	switch kind {
	case 't':
		e.cursor.Col++
	case 'T':
		e.cursor.Col--
	}

	op := e.pend.op
	e.applyFind(kind, e.lastFind.target, n)
	if e.cursor == e.buf.Clamp(saved) && op == 0 {
		e.cursor = saved
	}
}

func flipFind(kind rune) rune {
	switch kind {
	case 'f':
		return 'F'
	case 'F':
		return 'f'
	case 't':
		return 'T'
	default:
		return 't'
	}
}

func (e *Editor) applyMotion(m motion) {
	op := e.pend.op
	e.pend = pending{}

	if op != 0 {
		e.applyOperator(op, m)
		e.screenColSet = false
		return
	}
	e.cursor = e.buf.Clamp(m.target)
	e.clampCursor()
	if e.applyingDisplayMotion {
		return
	}
	e.screenColSet = false
	if m.kind != linewise {
		e.desiredCol = e.cursor.Col
	}
}

func (e *Editor) applyOperator(op rune, m motion) {
	from, to := e.cursor, m.target
	if to.Before(from) {
		from, to = to, from
	}

	if m.kind == linewise {
		e.operateLines(op, from.Line, to.Line)
		return
	}
	if m.kind == charInclusive {
		to = e.forwardOne(to)
	}
	e.operateChars(op, from, to)
}

func (e *Editor) forwardOne(p Pos) Pos {
	if p.Col < e.buf.LineLen(p.Line) {
		return Pos{p.Line, p.Col + 1}
	}
	return p
}

func (e *Editor) operateLines(op rune, from, to int) {
	switch {
	case op == opIndent:
		e.indentLines(from, to, 1)
		return
	case op == opDedent:
		e.indentLines(from, to, -1)
		return
	case isCaseOp(op):
		e.snapshot()
		e.mapRange(Pos{from, 0}, Pos{to, max(e.buf.LineLen(to)-1, 0)}, caseFunc(op))
		e.mode = ModeNormal
		e.cursor = Pos{from, e.cursor.Col}
		e.clampCursor()
		return
	}

	var lines []string
	for i := from; i <= to; i++ {
		lines = append(lines, e.buf.Line(i))
	}
	e.setRegister(register{text: strings.Join(lines, "\n"), linewise: true})

	if op == 'y' {
		e.flashYank(Pos{from, 0}, Pos{to, 0}, true, false)
		e.reportYank(to-from+1, "line")
		e.cursor = Pos{from, e.cursor.Col}
		e.clampCursor()
		return
	}

	e.snapshot()
	if op == 'c' {
		e.buf.ReplaceLines(from, to+1, []string{""})
		e.cursor = Pos{from, 0}
		e.mode = ModeInsert
		return
	}
	e.buf.ReplaceLines(from, to+1, nil)
	e.cursor = Pos{min(from, e.buf.LineCount()-1), 0}
	e.clampCursor()
}

func (e *Editor) operateChars(op rune, from, to Pos) {
	if isCaseOp(op) {
		e.snapshot()
		last := to
		if last.Col > 0 {
			last.Col--
		}
		e.mapRange(from, last, caseFunc(op))
		e.mode = ModeNormal
		e.cursor = from
		e.clampCursor()
		return
	}
	if op == opIndent || op == opDedent {
		dir := 1
		if op == opDedent {
			dir = -1
		}
		e.indentLines(from.Line, to.Line, dir)
		return
	}
	if op == 'y' {
		e.setRegister(register{text: e.textBetween(from, to)})
		last := to
		if last.Col > 0 {
			last.Col--
		}
		e.flashYank(from, last, false, false)
		e.reportYank(len([]rune(e.reg.text)), "char")
		e.cursor = from
		e.clampCursor()
		return
	}

	e.snapshot()
	e.setRegister(register{text: e.buf.Delete(from, to)})
	e.cursor = from
	if op == 'c' {
		e.mode = ModeInsert
		e.cursor = e.buf.Clamp(from)
		return
	}
	e.clampCursor()
}

func (e *Editor) reportYank(n int, unit string) {
	plural := "s"
	if n == 1 {
		plural = ""
	}
	e.message = fmt.Sprintf("yanked %d %s%s", n, unit, plural)
}

func (e *Editor) textBetween(from, to Pos) string {
	if from.Line == to.Line {
		line := e.buf.runes(from.Line)
		return string(line[min(from.Col, len(line)):min(to.Col, len(line))])
	}
	var sb strings.Builder
	sb.WriteString(string(e.buf.runes(from.Line)[from.Col:]))
	for i := from.Line + 1; i < to.Line; i++ {
		sb.WriteByte('\n')
		sb.WriteString(e.buf.Line(i))
	}
	sb.WriteByte('\n')
	sb.WriteString(string(e.buf.runes(to.Line)[:to.Col]))
	return sb.String()
}

func (e *Editor) applyVisual(op rune) {
	e.rememberVisual()
	if e.mode == ModeVisualBlock {
		e.pend = pending{}
		e.applyBlock(op)
		return
	}
	from, to, lines := e.Selection()
	e.mode = ModeNormal
	e.pend = pending{}

	if lines {
		e.operateLines(op, from.Line, to.Line)
		return
	}
	e.cursor = from
	e.operateChars(op, from, e.forwardOne(to))
}

func (e *Editor) command(r rune) {
	if r != 'v' && r != 'V' {
		e.screenColSet = false
	}
	n := e.pend.count()

	switch r {
	case 'i':
		e.enterInsert(e.cursor)
	case 'a':
		e.enterInsert(Pos{e.cursor.Line, min(e.cursor.Col+1, e.buf.LineLen(e.cursor.Line))})
	case 'I':
		e.enterInsert(Pos{e.cursor.Line, firstNonBlank(e.buf, e.cursor.Line)})
	case 'A':
		e.enterInsert(Pos{e.cursor.Line, e.buf.LineLen(e.cursor.Line)})
	case 'o':
		e.snapshot()
		e.mode = ModeInsert
		e.cursor = e.buf.Insert(Pos{e.cursor.Line, e.buf.LineLen(e.cursor.Line)}, "\n")
	case 'O':
		e.snapshot()
		e.mode = ModeInsert
		e.buf.Insert(Pos{e.cursor.Line, 0}, "\n")
		e.cursor = Pos{e.cursor.Line, 0}
	case 'x':
		e.snapshot()
		end := Pos{e.cursor.Line, min(e.cursor.Col+n, e.buf.LineLen(e.cursor.Line))}
		e.setRegister(register{text: e.buf.Delete(e.cursor, end)})
		e.clampCursor()
	case 'X':
		if e.cursor.Col == 0 {
			return
		}
		e.snapshot()
		start := Pos{e.cursor.Line, max(e.cursor.Col-n, 0)}
		e.setRegister(register{text: e.buf.Delete(start, e.cursor)})
		e.cursor = start
		e.clampCursor()
	case 'D':
		e.applyOperator('d', motion{Pos{e.cursor.Line, e.buf.LineLen(e.cursor.Line)}, charExclusive})
	case 'C':
		e.applyOperator('c', motion{Pos{e.cursor.Line, e.buf.LineLen(e.cursor.Line)}, charExclusive})
	case 'Y':
		e.operateLines('y', e.cursor.Line, min(e.cursor.Line+n-1, e.buf.LineCount()-1))
	case 'p':
		e.put(true)
	case 'P':
		e.put(false)
	case 's':
		end := Pos{e.cursor.Line, min(e.cursor.Col+n, e.buf.LineLen(e.cursor.Line))}
		e.operateChars('c', e.cursor, end)
	case 'S':
		e.operateLines('c', e.cursor.Line, min(e.cursor.Line+n-1, e.buf.LineCount()-1))
	case '~':
		e.toggleAt(n)
	case 'u':
		e.restore(&e.undo, &e.redo)
	case 'J':
		e.joinLines(e.cursor.Line, e.cursor.Line+max(n-1, 1))
	case 'v':
		e.startVisual(ModeVisual)
	case 'V':
		e.startVisual(ModeVisualLine)
	case ':':
		e.mode = ModeCommand
		e.cmdline = nil
	case '/':
		e.startSearch()
	case 'n':
		e.jumpToMatch(false)
	case 'N':
		e.jumpToMatch(true)
	case 'Z':
		e.pend.await = 'Z'
	}
}

func (e *Editor) enterInsert(at Pos) {
	e.snapshot()
	e.mode = ModeInsert
	e.cursor = e.buf.Clamp(at)
}

func (e *Editor) startVisual(m Mode) {
	if e.mode == m {
		e.mode = ModeNormal
		return
	}
	e.mode = m
	e.visualStart = e.cursor
}

func (e *Editor) put(after bool) {
	e.putRegister(after, true)
}

func (e *Editor) putRegister(after, takeSnapshot bool) {
	if e.reg.text == "" && !e.reg.linewise {
		return
	}
	if takeSnapshot {
		e.snapshot()
	}

	if e.reg.block {
		col := e.cursor.Col
		if after && e.buf.LineLen(e.cursor.Line) > 0 {
			col++
		}
		for i, part := range strings.Split(e.reg.text, "\n") {
			line := e.cursor.Line + i
			if line >= e.buf.LineCount() {
				break
			}
			e.buf.Insert(Pos{line, min(col, e.buf.LineLen(line))}, part)
		}
		e.cursor = e.buf.Clamp(Pos{e.cursor.Line, col})
		e.clampCursor()
		return
	}

	if e.reg.linewise {
		at := e.cursor.Line
		to := at
		if e.buf.LineCount() == 1 && e.buf.Line(0) == "" {
			to++
		} else if after {
			at++
			to = at
		}
		e.buf.ReplaceLines(at, to, strings.Split(e.reg.text, "\n"))
		e.cursor = Pos{at, 0}
		e.clampCursor()
		return
	}

	at := e.cursor
	if after && e.buf.LineLen(at.Line) > 0 {
		at.Col++
	}
	end := e.buf.Insert(at, e.reg.text)
	e.cursor = Pos{end.Line, max(end.Col-1, 0)}
	e.clampCursor()
}
