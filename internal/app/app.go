// Package app wires the editor, the day store and the file watcher into a
// Bubble Tea program.
package app

import (
	"fmt"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/praxis-labs-io/zen-notes/internal/editor"
	"github.com/praxis-labs-io/zen-notes/internal/note"
)

// saveInterval is how often a dirty buffer reaches disk. Short enough that a
// second window feels live, long enough not to write on every keystroke.
const saveInterval = 500 * time.Millisecond

type tickMsg struct{}

// statusTicks is how many save ticks a flash message survives, a little over
// three seconds. It expires on its own because whatever set it may be the
// last thing to happen for a while.
const statusTicks = 7

// flashDuration is how long a yank stays lit. Long enough to catch, short
// enough that it never feels like a selection you have to dismiss.
const flashDuration = 110 * time.Millisecond

// fileChangedMsg carries the path a watcher saw change.
type fileChangedMsg string

// yankFlashDoneMsg puts out the highlight over a yank.
type yankFlashDoneMsg struct{}

// reloadDecision is what to do about a note changing underneath us.
type reloadDecision int

const (
	reloadIgnore reloadDecision = iota
	reloadApply
	reloadKeepLocal
)

// decideReload compares the disk copy against what we last wrote. Our own
// save comes back through the watcher and must not reload over the cursor.
func decideReload(disk, ours string, dirty bool) reloadDecision {
	if disk == ours {
		return reloadIgnore
	}
	if dirty {
		return reloadKeepLocal
	}
	return reloadApply
}

// Model is the running app: one day's note, open for editing.
type Model struct {
	store *note.Store
	watch *note.Watcher
	ed    *editor.Editor

	day         note.Day
	followToday bool
	lastWritten string
	status      string
	statusLeft  int
	help        bool

	width, height int
	now           func() note.Day
}

// NewModel opens today's note. The watcher may be nil, in which case the note
// is never reloaded from disk.
func NewModel(s *note.Store, w *note.Watcher) (*Model, error) {
	day := note.Today()
	text, err := s.Load(day)
	if err != nil {
		return nil, err
	}
	return &Model{
		store:       s,
		watch:       w,
		ed:          editor.New(text),
		day:         day,
		followToday: true,
		lastWritten: text,
		width:       80,
		height:      24,
		now:         note.Today,
	}, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(tick(), waitForChange(m.watch), tea.RequestBackgroundColor)
}

func tick() tea.Cmd {
	return tea.Tick(saveInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// waitForChange blocks on the watcher until a note changes.
func waitForChange(w *note.Watcher) tea.Cmd {
	if w == nil {
		return nil
	}
	return func() tea.Msg {
		name, ok := <-w.Changes()
		if !ok {
			return nil
		}
		return fileChangedMsg(name)
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.ed.SetBackground(msg.Color)
		return m, nil

	case tea.FocusMsg:
		// The theme may have changed while we were in the background.
		return m, tea.RequestBackgroundColor

	case yankFlashDoneMsg:
		m.ed.ClearYankFlash()
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ed.SetHeight(m.textHeight())
		return m, nil

	case tickMsg:
		m.expireStatus()
		m.autosave()
		m.checkRollover()
		return m, tick()

	case fileChangedMsg:
		m.reload(string(msg))
		return m, waitForChange(m.watch)

	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.Mod == tea.ModCtrl && msg.Code == 'c' {
		return m.quit()
	}
	key, ok := translateKey(msg)
	if !ok {
		return nil
	}
	if m.help {
		m.help = false
		return nil
	}
	if m.ed.Mode() == editor.ModeNormal && m.browseKey(key) {
		return nil
	}
	m.clearStatus()
	m.ed.Feed(key)

	if msg := m.ed.Message(); msg != "" {
		m.setStatus(msg)
		m.ed.ClearMessage()
	}
	if m.ed.TakeSaveRequest() {
		m.save()
	}
	if m.ed.QuitRequested() {
		return m.quit()
	}
	if m.ed.YankFlash() {
		return tea.Tick(flashDuration, func(time.Time) tea.Msg { return yankFlashDoneMsg{} })
	}
	return nil
}

// browseKey handles the day navigation keys, reporting whether it took the
// keystroke. These are only live in normal mode so they stay typable.
func (m *Model) browseKey(key editor.Key) bool {
	if key.Name != "" {
		return false
	}
	switch key.R {
	case '[':
		m.step(m.store.Prev, "no earlier note")
	case ']':
		m.step(m.nextDay, "no later note")
	case '\\':
		m.open(note.Today(), true)
	case '?':
		m.help = true
	default:
		return false
	}
	return true
}

// nextDay is the next saved day, falling back to today. Today has no file
// until it is edited, so Store.Next alone strands a browser in the past.
func (m *Model) nextDay(d note.Day) (note.Day, bool, error) {
	day, ok, err := m.store.Next(d)
	if err != nil || ok {
		return day, ok, err
	}
	today := note.Today()
	return today, d.Before(today), nil
}

// step moves to an adjacent day that has a note, saving the current one first.
func (m *Model) step(find func(note.Day) (note.Day, bool, error), missing string) {
	m.save()
	day, ok, err := find(m.day)
	if err != nil {
		m.setStatus(err.Error())
		return
	}
	if !ok {
		m.setStatus(missing)
		return
	}
	m.open(day, day == note.Today())
}

// open switches to another day. follow marks whether the app should still
// roll over at midnight.
func (m *Model) open(day note.Day, follow bool) {
	m.save()
	text, err := m.store.Load(day)
	if err != nil {
		m.setStatus(err.Error())
		return
	}
	m.day = day
	m.followToday = follow
	m.lastWritten = text
	m.ed.SetText(text)
	m.ed.SetCursor(editor.Pos{})
	m.clearStatus()
}

// setStatus shows a flash message and starts its countdown.
func (m *Model) setStatus(s string) {
	m.status = s
	m.statusLeft = statusTicks
}

func (m *Model) clearStatus() {
	m.status = ""
	m.statusLeft = 0
}

// expireStatus counts a flash down and drops it when it runs out.
func (m *Model) expireStatus() {
	if m.statusLeft == 0 {
		return
	}
	m.statusLeft--
	if m.statusLeft == 0 {
		m.status = ""
	}
}

func (m *Model) autosave() {
	if m.ed.Dirty() {
		m.save()
	}
}

func (m *Model) save() {
	if !m.ed.Dirty() {
		return
	}
	text := m.ed.Text()
	if err := m.store.Save(m.day, text); err != nil {
		m.setStatus(err.Error())
		return
	}
	m.lastWritten = text
	m.ed.MarkSaved()
}

// checkRollover moves to the new day once midnight passes, unless the user
// has browsed away from today.
func (m *Model) checkRollover() {
	if !m.followToday {
		return
	}
	if today := m.now(); today != m.day {
		m.open(today, true)
	}
}

// reload takes the disk copy when another instance wrote the note we have open.
func (m *Model) reload(path string) {
	if filepath.Base(path) != filepath.Base(m.store.Path(m.day)) {
		return
	}
	disk, err := m.store.Load(m.day)
	if err != nil {
		m.setStatus(err.Error())
		return
	}

	switch decideReload(disk, m.lastWritten, m.ed.Dirty()) {
	case reloadIgnore:
		return
	case reloadKeepLocal:
		m.setStatus("changed elsewhere, keeping your edits")
	case reloadApply:
		cursor := m.ed.Cursor()
		m.ed.SetText(disk)
		m.ed.SetCursor(cursor)
		m.lastWritten = disk
		m.setStatus("reloaded")
	}
}

func (m *Model) quit() tea.Cmd {
	m.save()
	if m.watch != nil {
		_ = m.watch.Close()
	}
	return tea.Quit
}

// translateKey converts a Bubble Tea keystroke into an editor key, reporting
// false for combinations the editor has no use for.
func translateKey(msg tea.KeyPressMsg) (editor.Key, bool) {
	if msg.Mod&tea.ModCtrl != 0 {
		switch msg.Code {
		case 'd', 'u', 'r', 'v':
			return editor.Named(fmt.Sprintf("c-%c", msg.Code)), true
		}
		return editor.Key{}, false
	}
	// Shift is not a modifier here: the capital is already in Text.
	if msg.Mod&^tea.ModShift != 0 {
		return editor.Key{}, false
	}

	switch msg.Code {
	case tea.KeyEscape:
		return editor.Named("esc"), true
	case tea.KeyEnter:
		return editor.Named("enter"), true
	case tea.KeyBackspace:
		return editor.Named("backspace"), true
	case tea.KeyTab:
		return editor.Named("tab"), true
	case tea.KeyUp:
		return editor.Named("up"), true
	case tea.KeyDown:
		return editor.Named("down"), true
	case tea.KeyLeft:
		return editor.Named("left"), true
	case tea.KeyRight:
		return editor.Named("right"), true
	}

	if msg.Text != "" {
		return editor.Rune([]rune(msg.Text)[0]), true
	}
	return editor.Key{}, false
}
