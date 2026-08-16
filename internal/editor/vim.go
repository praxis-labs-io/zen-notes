package editor

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Mode is the editing mode the next keystroke will be read in.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeVisual
	ModeVisualLine
	ModeVisualBlock
	ModeCommand
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
	default:
		return "NORMAL"
	}
}

// Visual reports whether the mode is one of the three visual modes.
func (m Mode) Visual() bool {
	return m == ModeVisual || m == ModeVisualLine || m == ModeVisualBlock
}

// Key is one keystroke: either a literal rune or a named key like esc or c-d.
type Key struct {
	R    rune
	Name string
}

// Rune builds a literal keystroke.
func Rune(r rune) Key { return Key{R: r} }

// Named builds a keystroke for a key with no rune, such as esc or c-d.
func Named(name string) Key { return Key{Name: name} }

// motionKind decides how an operator turns a motion target into a range.
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

// register holds the last yank or delete. Linewise content is put on its own
// lines rather than inside the current one.
type register struct {
	text     string
	linewise bool
	block    bool
}

// find remembers the last f, t, F or T so ; and , can walk it.
type find struct {
	kind   rune
	target rune
}

// blockPending tracks a visual-block I or A, so leaving insert mode can
// replicate what was typed down the other lines of the block.
type blockPending struct {
	active     bool
	firstLine  int
	lastLine   int
	col        int
	appendEnd  bool
	insertedAt Pos
}

// flashRange is the span a yank briefly lights up.
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

// pending is the half-typed normal-mode command: counts, an operator waiting
// for its motion, and any key that expects an argument next.
type pending struct {
	count1 int
	op     rune
	count2 int
	await  rune
	keys   []rune
}

func (p pending) empty() bool {
	return p.count1 == 0 && p.op == 0 && p.count2 == 0 && p.await == 0
}

// count multiplies the operator and motion counts, as vim does for d2w.
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

// Editor is a modal buffer: text, cursor, mode, and the undo stack.
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

	undo []snapshot
	redo []snapshot

	desiredCol     int
	top            int
	rows           []vrow
	cursorRow      int
	lastVisual     [2]Pos
	height         int
	dirty          bool
	darkBackground bool
	selection      lipgloss.Style
	flashStyle     lipgloss.Style
	flash          flashRange
	quit           bool
	saveWanted     bool
}

const undoDepth = 200

// New opens text in normal mode with the cursor at the start.
func New(text string) *Editor {
	return &Editor{
		buf:  NewBuffer(text),
		mode: ModeNormal,
		// Assume dark until the terminal answers. A light selection on a dark
		// background is the louder mistake of the two.
		darkBackground: true,
		selection:      darkSelection,
		flashStyle:     darkFlash,
		height:         20,
	}
}

// Text is the current buffer contents.
func (e *Editor) Text() string { return e.buf.Text() }

// Buffer exposes the lines for rendering.
func (e *Editor) Buffer() *Buffer { return e.buf }

// Cursor is where the caret sits.
func (e *Editor) Cursor() Pos { return e.cursor }

// SetCursor moves the caret, clamped to the buffer.
func (e *Editor) SetCursor(p Pos) {
	e.cursor = e.buf.Clamp(p)
	e.clampCursor()
}

// Mode is the current editing mode.
func (e *Editor) Mode() Mode { return e.mode }

// Dirty reports whether the buffer changed since the last MarkSaved.
func (e *Editor) Dirty() bool { return e.dirty }

// MarkSaved records that the current text reached disk.
func (e *Editor) MarkSaved() { e.dirty = false }

// SetHeight tells the editor how many rows a half-page scroll covers.
func (e *Editor) SetHeight(n int) {
	if n > 0 {
		e.height = n
	}
}

// QuitRequested reports whether :q or ZZ asked the app to exit.
func (e *Editor) QuitRequested() bool { return e.quit }

// TakeSaveRequest reports whether :w asked for a save, and clears the request.
func (e *Editor) TakeSaveRequest() bool {
	want := e.saveWanted
	e.saveWanted = false
	return want
}

// Message is the last thing the editor wants to tell the user.
func (e *Editor) Message() string { return e.message }

// ClearMessage drops the current message.
func (e *Editor) ClearMessage() { e.message = "" }

// CommandLine is the ":" line being typed, empty outside command mode.
func (e *Editor) CommandLine() string {
	if e.mode != ModeCommand {
		return ""
	}
	return ":" + string(e.cmdline)
}

// PendingKeys is the half-typed command, for the status bar.
func (e *Editor) PendingKeys() string { return string(e.pend.keys) }

// SetText replaces the buffer, keeping the cursor in bounds. Used when
// another instance writes the note.
func (e *Editor) SetText(text string) {
	e.buf = NewBuffer(text)
	e.cursor = e.buf.Clamp(e.cursor)
	e.clampCursor()
	e.undo, e.redo = nil, nil
	e.dirty = false
}

// Selection is the visual range as an ordered pair, and whether it is
// linewise. It is meaningless outside visual modes.
func (e *Editor) Selection() (Pos, Pos, bool) {
	from, to := e.visualStart, e.cursor
	if to.Before(from) {
		from, to = to, from
	}
	return from, to, e.mode == ModeVisualLine
}

// Feed applies one keystroke.
func (e *Editor) Feed(k Key) {
	switch e.mode {
	case ModeInsert:
		e.insertKey(k)
	case ModeCommand:
		e.commandKey(k)
	default:
		e.normalKey(k)
	}
}

// clampCursor keeps the caret on a real rune. Normal and visual modes stop on
// the last rune; insert mode may sit one past it.
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

// ---- insert mode ----

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
		e.cursor = e.buf.Insert(e.cursor, "\n")
		e.dirty = true
		return
	case "backspace", "bs":
		e.backspace()
		return
	case "tab":
		e.cursor = e.buf.Insert(e.cursor, "\t")
		e.dirty = true
		return
	case "up", "down", "left", "right":
		e.arrow(k.Name)
		return
	case "":
	default:
		return
	}
	if k.R == 0 {
		return
	}
	e.cursor = e.buf.Insert(e.cursor, string(k.R))
	e.dirty = true
}

func (e *Editor) backspace() {
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

func (e *Editor) arrow(name string) {
	switch name {
	case "up":
		e.moveVertical(-1)
	case "down":
		e.moveVertical(1)
	case "left":
		if e.cursor.Col > 0 {
			e.cursor.Col--
		}
		e.desiredCol = e.cursor.Col
	case "right":
		e.cursor.Col++
		e.clampCursor()
		e.desiredCol = e.cursor.Col
	}
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

// ---- command mode ----

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

// ---- normal and visual modes ----

func (e *Editor) normalKey(k Key) {
	if k.Name != "" {
		motion, ok := arrowMotion(k.Name)
		if !ok {
			e.namedNormalKey(k.Name)
			return
		}
		// An arrow is never an argument, so it cancels a half-typed f or i.
		if e.pend.await != 0 {
			e.pend = pending{}
			return
		}
		k = Rune(motion)
	}
	r := k.R
	if r == 0 {
		return
	}
	e.pend.keys = append(e.pend.keys, r)

	if e.pend.await != 0 {
		e.resolveAwait(r)
		return
	}
	if e.digit(r) {
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
	switch name {
	case "esc":
		e.pend = pending{}
		if e.mode.Visual() {
			e.rememberVisual()
			e.mode = ModeNormal
		}
	case "c-d":
		e.halfPage(1)
	case "c-u":
		e.halfPage(-1)
	case "c-r":
		e.restore(&e.redo, &e.undo)
	case "c-v":
		e.startVisual(ModeVisualBlock)
		e.pend = pending{}
	}
}

// arrowMotion maps an arrow key onto the hjkl it stands in for, so counts,
// operators and visual mode work the same either way.
func arrowMotion(name string) (rune, bool) {
	switch name {
	case "up":
		return 'k', true
	case "down":
		return 'j', true
	case "left":
		return 'h', true
	case "right":
		return 'l', true
	}
	return 0, false
}

func (e *Editor) halfPage(dir int) {
	e.moveVertical(dir * max(e.height/2, 1))
	e.pend = pending{}
}

// digit accumulates a count. A leading zero is the motion, not a count.
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

// operator handles the keys that take a motion, including the doubled forms
// like dd and >> that act on whole lines.
func (e *Editor) operator(r rune) bool {
	// gUU, guu and g~~ act on the whole line, like dd does for d.
	if op, ok := caseOp(r); ok && e.pend.op == op {
		last := min(e.cursor.Line+e.pend.count()-1, e.buf.LineCount()-1)
		e.applyOperator(op, motion{target: Pos{last, 0}, kind: linewise})
		e.pend = pending{}
		return true
	}
	if r == '>' {
		r = opIndent
	} else if r == '<' {
		r = opDedent
	}
	if r != 'd' && r != 'c' && r != 'y' && r != opIndent && r != opDedent {
		return false
	}
	if e.mode.Visual() {
		e.applyVisual(r)
		return true
	}
	if e.pend.op == r {
		line := e.cursor.Line
		last := min(line+e.pend.count()-1, e.buf.LineCount()-1)
		e.applyOperator(r, motion{target: Pos{last, 0}, kind: linewise})
		e.pend = pending{}
		return true
	}
	if e.pend.op != 0 {
		e.pend = pending{}
		return true
	}
	e.pend.op = r
	return true
}

// resolveMotion turns a motion key into a target, reporting false for keys
// that are not motions. Keys needing an argument park in pend.await.
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
		// cw acts like ce, leaving the space after the word alone.
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
		// Only an operator or a visual selection can take a text object;
		// otherwise these are the insert keys.
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

// applyTextObject resolves an iw/aw/i(/a" style object and hands the span to
// the pending operator, or selects it when in a visual mode.
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

// resolveAwait handles the second key of gg and the target of f, t, F and T.
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
		case 'U', 'u', '~':
			// gU, gu and g~ are operators, so they wait for a motion next.
			e.pend.op, _ = caseOp(r)
		case 'v':
			e.reselect()
			e.pend = pending{}
		default:
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

// applyFind runs one of f, t, F or T and moves or operates with the result.
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

// repeatFind is ; and ,. A t or T repeat starts one rune further along, or it
// would just find the character the cursor is already parked against.
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
	if kind == 't' {
		e.cursor.Col++
	} else if kind == 'T' {
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

// applyMotion either moves the caret or feeds a pending operator.
func (e *Editor) applyMotion(m motion) {
	op := e.pend.op
	e.pend = pending{}

	if op != 0 {
		e.applyOperator(op, m)
		return
	}
	e.cursor = e.buf.Clamp(m.target)
	e.clampCursor()
	if m.kind != linewise {
		e.desiredCol = e.cursor.Col
	}
}

// applyOperator runs d, c or y over the range a motion describes.
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

// forwardOne widens an inclusive motion to cover its target rune.
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
	e.reg = register{text: strings.Join(lines, "\n"), linewise: true}

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
		e.reg = register{text: e.textBetween(from, to)}
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
	e.reg = register{text: e.buf.Delete(from, to)}
	e.cursor = from
	if op == 'c' {
		e.mode = ModeInsert
		e.cursor = e.buf.Clamp(from)
		return
	}
	e.clampCursor()
}

// reportYank says what a yank took, since nothing on screen changes.
func (e *Editor) reportYank(n int, unit string) {
	plural := "s"
	if n == 1 {
		plural = ""
	}
	e.message = fmt.Sprintf("yanked %d %s%s", n, unit, plural)
}

// textBetween reads a range without changing the buffer.
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

// applyVisual runs an operator over the current selection.
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

// command runs the standalone normal-mode keys that take no motion.
func (e *Editor) command(r rune) {
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
		e.reg = register{text: e.buf.Delete(e.cursor, end)}
		e.clampCursor()
	case 'X':
		if e.cursor.Col == 0 {
			return
		}
		e.snapshot()
		start := Pos{e.cursor.Line, max(e.cursor.Col-n, 0)}
		e.reg = register{text: e.buf.Delete(start, e.cursor)}
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

// put inserts the register after the cursor, or before it when after is false.
func (e *Editor) put(after bool) {
	if e.reg.text == "" {
		return
	}
	e.snapshot()

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
		if after {
			at++
		}
		e.buf.ReplaceLines(at, at, strings.Split(e.reg.text, "\n"))
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
