// Package app runs the note editor as a Bubble Tea program.
package app

import (
	"fmt"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/praxis-labs-io/zen-notes/internal/editor"
	"github.com/praxis-labs-io/zen-notes/internal/note"
)

const saveInterval = 500 * time.Millisecond

type tickMsg struct{}

const statusTicks = 7

const flashDuration = 110 * time.Millisecond

type fileChangedMsg string

type yankFlashDoneMsg struct{}

type linkOpenedMsg struct{ request int }

type linkOpenFailedMsg struct {
	request int
	err     error
}

type reloadDecision int

const (
	reloadIgnore reloadDecision = iota
	reloadApply
	reloadKeepLocal
)

// Our own save comes back through the watcher, so a disk copy matching what we last wrote is ignored.
func decideReload(disk, ours string, dirty bool) reloadDecision {
	if disk == ours {
		return reloadIgnore
	}
	if dirty {
		return reloadKeepLocal
	}
	return reloadApply
}

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
	images        *images
	now           func() note.Day
	openLink      func(string) error
	linkRequest   int
}

// NewModel opens today's note. A nil watcher means the note is never reloaded from disk.
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
		images:      newImages(),
		now:         note.Today,
		openLink:    systemOpenLink,
	}, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(tick(), waitForChange(m.watch), tea.RequestBackgroundColor,
		tea.Raw(ansi.WindowOp(16)))
}

func tick() tea.Cmd {
	return tea.Tick(saveInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

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
	model, cmd := m.update(msg)
	return model, tea.Batch(cmd, m.syncImages())
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uv.CellSizeEvent:
		m.images.cellW, m.images.cellH = msg.Width, msg.Height
		return m, nil

	case tea.BackgroundColorMsg:
		m.ed.SetBackground(msg.Color)
		return m, nil

	case tea.FocusMsg:
		return m, tea.RequestBackgroundColor

	case yankFlashDoneMsg:
		m.ed.ClearYankFlash()
		return m, nil

	case linkOpenedMsg:
		if msg.request == m.linkRequest {
			m.setStatus("opened link")
		}
		return m, nil

	case linkOpenFailedMsg:
		if msg.request == m.linkRequest {
			m.setStatus("open link: " + msg.err.Error())
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ed.SetHeight(m.textHeight())
		return m, tea.Raw(ansi.WindowOp(16))

	case tickMsg:
		m.expireStatus()
		m.autosave()
		m.checkRollover()
		return m, tick()

	case fileChangedMsg:
		m.reload(string(msg))
		return m, waitForChange(m.watch)

	case tea.PasteMsg:
		if m.help {
			return m, nil
		}
		m.clearStatus()
		m.ed.Paste(msg.Content)
		return m, m.takeClipboardCmd()

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
	clipboardCmd := m.takeClipboardCmd()
	if target, wanted := m.ed.TakeOpenLinkRequest(); wanted {
		return m.openLinkCmd(target)
	}
	if m.ed.TakeSaveRequest() {
		m.save()
	}
	if m.ed.QuitRequested() {
		return m.quit()
	}
	if m.ed.YankFlash() {
		flashCmd := tea.Tick(flashDuration, func(time.Time) tea.Msg { return yankFlashDoneMsg{} })
		if clipboardCmd != nil {
			return tea.Batch(clipboardCmd, flashCmd)
		}
		return flashCmd
	}
	return clipboardCmd
}

func (m *Model) takeClipboardCmd() tea.Cmd {
	text, wanted := m.ed.TakeClipboardRequest()
	if !wanted {
		return nil
	}
	return tea.SetClipboard(text)
}

func (m *Model) openLinkCmd(target string) tea.Cmd {
	m.linkRequest++
	request := m.linkRequest
	if err := validWebLink(target); err != nil {
		m.setStatus(err.Error())
		return nil
	}
	return func() tea.Msg {
		if err := m.openLink(target); err != nil {
			return linkOpenFailedMsg{request: request, err: err}
		}
		return linkOpenedMsg{request: request}
	}
}

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
		m.open(m.now(), true)
	case '?':
		m.help = true
	default:
		return false
	}
	return true
}

// Today has no file until it is edited, so Store.Next alone strands a browser in the past.
func (m *Model) nextDay(d note.Day) (note.Day, bool, error) {
	day, ok, err := m.store.Next(d)
	if err != nil || ok {
		return day, ok, err
	}
	today := m.now()
	return today, d.Before(today), nil
}

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
	m.open(day, day == m.now())
}

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
	m.ed.Reset()
	m.ed.SetText(text)
	m.ed.SetCursor(editor.Pos{})
	m.clearStatus()
}

func (m *Model) setStatus(s string) {
	m.status = s
	m.statusLeft = statusTicks
}

func (m *Model) clearStatus() {
	m.status = ""
	m.statusLeft = 0
}

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

func (m *Model) checkRollover() {
	if !m.followToday {
		return
	}
	if today := m.now(); today != m.day {
		m.open(today, true)
	}
}

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

// Shift is not rejected as a modifier: a capital arrives with ModShift set and the capital already in Text.
func translateKey(msg tea.KeyPressMsg) (editor.Key, bool) {
	if msg.Mod == tea.ModSuper && msg.Code == 'c' {
		return editor.Named("copy"), true
	}
	if msg.Mod&tea.ModCtrl != 0 {
		switch msg.Code {
		case 'd', 'u', 'r', 'v':
			return editor.Named(fmt.Sprintf("c-%c", msg.Code)), true
		}
		return editor.Key{}, false
	}
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
		if msg.Mod&tea.ModShift != 0 {
			return editor.Named("backtab"), true
		}
		return editor.Named("tab"), true
	case tea.KeyUp:
		return editor.Named("up"), true
	case tea.KeyDown:
		return editor.Named("down"), true
	case tea.KeyLeft:
		return editor.Named("left"), true
	case tea.KeyRight:
		return editor.Named("right"), true
	case tea.KeyHome:
		return editor.Named("home"), true
	case tea.KeyEnd:
		return editor.Named("end"), true
	case tea.KeyPgUp:
		return editor.Named("pgup"), true
	case tea.KeyPgDown:
		return editor.Named("pgdown"), true
	case tea.KeyDelete:
		return editor.Named("delete"), true
	}

	if msg.Text != "" {
		return editor.Rune([]rune(msg.Text)[0]), true
	}
	return editor.Key{}, false
}
