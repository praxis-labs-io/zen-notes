// Package app wires the editor, the day store and the file watcher into a
// Bubble Tea program.
package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/drucial/zen-notes/internal/editor"
	"github.com/drucial/zen-notes/internal/note"
)

// saveInterval is how often a dirty buffer reaches disk. Short enough that a
// second window feels live, long enough not to write on every keystroke.
const saveInterval = 500 * time.Millisecond

type tickMsg struct{}

// fileChangedMsg carries the path a watcher saw change.
type fileChangedMsg string

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
	return tea.Batch(tick(), waitForChange(m.watch))
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
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ed.SetHeight(m.textHeight())
		return m, nil

	case tickMsg:
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
	if m.ed.Mode() == editor.ModeNormal && m.browseKey(key) {
		return nil
	}
	m.status = ""
	m.ed.Feed(key)

	if msg := m.ed.Message(); msg != "" {
		m.status = msg
		m.ed.ClearMessage()
	}
	if m.ed.TakeSaveRequest() {
		m.save()
	}
	if m.ed.QuitRequested() {
		return m.quit()
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
		m.step(m.store.Next, "no later note")
	case '\\':
		m.open(note.Today(), true)
	default:
		return false
	}
	return true
}

// step moves to an adjacent day that has a note, saving the current one first.
func (m *Model) step(find func(note.Day) (note.Day, bool, error), missing string) {
	m.save()
	day, ok, err := find(m.day)
	if err != nil {
		m.status = err.Error()
		return
	}
	if !ok {
		m.status = missing
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
		m.status = err.Error()
		return
	}
	m.day = day
	m.followToday = follow
	m.lastWritten = text
	m.ed.SetText(text)
	m.ed.SetCursor(editor.Pos{})
	m.status = ""
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
		m.status = err.Error()
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
		m.status = err.Error()
		return
	}

	switch decideReload(disk, m.lastWritten, m.ed.Dirty()) {
	case reloadIgnore:
		return
	case reloadKeepLocal:
		m.status = "changed elsewhere, keeping your edits"
	case reloadApply:
		cursor := m.ed.Cursor()
		m.ed.SetText(disk)
		m.ed.SetCursor(cursor)
		m.lastWritten = disk
		m.status = "reloaded"
	}
}

func (m *Model) quit() tea.Cmd {
	m.save()
	if m.watch != nil {
		m.watch.Close()
	}
	return tea.Quit
}

// translateKey converts a Bubble Tea keystroke into an editor key, reporting
// false for combinations the editor has no use for.
func translateKey(msg tea.KeyPressMsg) (editor.Key, bool) {
	if msg.Mod&tea.ModCtrl != 0 {
		switch msg.Code {
		case 'd', 'u', 'r':
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

// --- view ---

var (
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	modeStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	dirtyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)

func (m *Model) textHeight() int { return max(m.height-1, 1) }

func (m *Model) View() tea.View {
	body := m.ed.View(m.width, m.textHeight())
	v := tea.NewView(body + "\n" + m.statusBar())
	v.AltScreen = true
	return v
}

func (m *Model) statusBar() string {
	if cmd := m.ed.CommandLine(); cmd != "" {
		return cmd
	}

	left := []string{
		statusStyle.Render(m.day.String()),
		modeStyle.Render(m.ed.Mode().String()),
	}
	if m.ed.Dirty() {
		left = append(left, dirtyStyle.Render("●"))
	}
	if m.status != "" {
		left = append(left, messageStyle.Render(m.status))
	}
	if keys := m.ed.PendingKeys(); keys != "" {
		left = append(left, statusStyle.Render(keys))
	}
	return strings.Join(left, "  ")
}
