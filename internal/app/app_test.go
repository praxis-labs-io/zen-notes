package app

import (
	"strings"
	"testing"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/drucial/zen-notes/internal/editor"
	"github.com/drucial/zen-notes/internal/note"
)

func TestDecideReload(t *testing.T) {
	tests := []struct {
		name       string
		disk, ours string
		dirty      bool
		want       reloadDecision
	}{
		{"our own write comes back", "abc", "abc", false, reloadIgnore},
		{"foreign write on a clean buffer", "theirs", "ours", false, reloadApply},
		{"foreign write while we have edits", "theirs", "ours", true, reloadKeepLocal},
		{"our own write while typing", "abc", "abc", true, reloadIgnore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideReload(tt.disk, tt.ours, tt.dirty); got != tt.want {
				t.Errorf("decideReload = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTranslateKey(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyPressMsg
		want editor.Key
		ok   bool
	}{
		{"letter", tea.KeyPressMsg{Text: "a", Code: 'a'}, editor.Rune('a'), true},
		// A shifted letter arrives lowercase in Code with the capital in Text.
		{"shifted letter", tea.KeyPressMsg{Text: "A", Code: 'a', Mod: tea.ModShift}, editor.Rune('A'), true},
		{"space", tea.KeyPressMsg{Text: " ", Code: ' '}, editor.Rune(' '), true},
		{"ctrl d", tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}, editor.Named("c-d"), true},
		{"ctrl r", tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}, editor.Named("c-r"), true},
		{"escape", tea.KeyPressMsg{Code: tea.KeyEscape}, editor.Named("esc"), true},
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}, editor.Named("enter"), true},
		{"backspace", tea.KeyPressMsg{Code: tea.KeyBackspace}, editor.Named("backspace"), true},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}, editor.Named("tab"), true},
		{"up", tea.KeyPressMsg{Code: tea.KeyUp}, editor.Named("up"), true},
		{"unknown modifier combo is dropped", tea.KeyPressMsg{Code: 'q', Mod: tea.ModAlt}, editor.Key{}, false},
		{"unknown ctrl combo is dropped", tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl}, editor.Key{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := translateKey(tt.msg)
			if ok != tt.ok || got != tt.want {
				t.Errorf("translateKey = %v, %v; want %v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTypingReachesTheBuffer(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "h", "i")
	if m.ed.Text() != "hi" {
		t.Fatalf("Text = %q, want hi", m.ed.Text())
	}
}

func TestCapitalLettersReachTheBuffer(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "T", "o", "d", "a", "y")
	if m.ed.Text() != "Today" {
		t.Fatalf("Text = %q, want Today", m.ed.Text())
	}
}

func TestShiftedNormalModeCommandsWork(t *testing.T) {
	m := newTestModel(t, "one\ntwo")
	press(m, "A", "!", "<esc>")
	if m.ed.Text() != "one!\ntwo" {
		t.Fatalf("Text = %q, want one!\\ntwo", m.ed.Text())
	}
}

func TestBracketsBrowseDaysOnlyInNormalMode(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "[", "]")
	if m.ed.Text() != "[]" {
		t.Fatalf("Text = %q, want []. Browse keys must be literal in insert mode", m.ed.Text())
	}
}

// { and } are shift+[ and shift+], so they must not trip the browse keys.
func TestBraceMotionIsNotABrowseKey(t *testing.T) {
	m := newTestModel(t, "a\n\nb")
	press(m, "}")
	if m.day != note.Today() {
		t.Fatalf("day = %v, want today. } browsed instead of moving", m.day)
	}
	if m.ed.Cursor().Line != 1 {
		t.Fatalf("cursor line = %d, want 1", m.ed.Cursor().Line)
	}
}

func TestAutosaveWritesTheNote(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "x")

	m.Update(tickMsg{})

	got, err := m.store.Load(m.day)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "x" {
		t.Fatalf("saved %q, want x", got)
	}
	if m.ed.Dirty() {
		t.Fatal("autosave left the editor dirty")
	}
}

func TestAutosaveSkipsACleanBuffer(t *testing.T) {
	m := newTestModel(t, "")
	m.Update(tickMsg{})
	if _, err := m.store.Load(m.day); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.lastWritten != "" {
		t.Fatalf("lastWritten = %q, want empty", m.lastWritten)
	}
}

func TestForeignWriteReloadsACleanBuffer(t *testing.T) {
	m := newTestModel(t, "original")
	writeFromElsewhere(t, m, "from the other terminal")

	m.Update(fileChangedMsg(m.store.Path(m.day)))

	if m.ed.Text() != "from the other terminal" {
		t.Fatalf("Text = %q, want the disk copy", m.ed.Text())
	}
}

func TestForeignWriteKeepsLocalEdits(t *testing.T) {
	m := newTestModel(t, "original")
	press(m, "i", "!")
	writeFromElsewhere(t, m, "from the other terminal")

	m.Update(fileChangedMsg(m.store.Path(m.day)))

	if m.ed.Text() != "!original" {
		t.Fatalf("Text = %q, want the local edit kept", m.ed.Text())
	}
	if m.status == "" {
		t.Fatal("keeping local edits said nothing in the status bar")
	}
}

func TestOurOwnWriteDoesNotReload(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "x")
	m.Update(tickMsg{})

	press(m, "y")
	m.Update(fileChangedMsg(m.store.Path(m.day)))

	if m.ed.Text() != "xy" {
		t.Fatalf("Text = %q, want xy. Our own save must not reload over us", m.ed.Text())
	}
}

func TestChangeToAnotherDayIsIgnored(t *testing.T) {
	m := newTestModel(t, "today")
	other := note.Day{Year: 2020, Month: time.March, Date: 3}
	if err := m.store.Save(other, "old note"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	m.Update(fileChangedMsg(m.store.Path(other)))

	if m.ed.Text() != "today" {
		t.Fatalf("Text = %q, want today's note untouched", m.ed.Text())
	}
}

func TestDayRolloverSavesAndOpensTheNewDay(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "l", "a", "t", "e")

	tomorrow := m.day.Add(1)
	m.now = func() note.Day { return tomorrow }
	m.Update(tickMsg{})

	if m.day != tomorrow {
		t.Fatalf("day = %v, want %v", m.day, tomorrow)
	}
	if m.ed.Text() != "" {
		t.Fatalf("Text = %q, want the new day to start empty", m.ed.Text())
	}
	yesterday, err := m.store.Load(tomorrow.Add(-1))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if yesterday != "late" {
		t.Fatalf("yesterday's note = %q, want late", yesterday)
	}
}

func TestBrowsingStopsTheDayRollover(t *testing.T) {
	m := newTestModel(t, "today")
	past := m.day.Add(-5)
	if err := m.store.Save(past, "older"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	press(m, "[")
	if m.day != past {
		t.Fatalf("day = %v, want %v", m.day, past)
	}

	tomorrow := note.Today().Add(1)
	m.now = func() note.Day { return tomorrow }
	m.Update(tickMsg{})

	if m.day != past {
		t.Fatalf("day = %v, want to stay on the browsed day", m.day)
	}
}

func TestBrowsePreviousAndNextDay(t *testing.T) {
	m := newTestModel(t, "today")
	past := m.day.Add(-2)
	if err := m.store.Save(past, "older"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	press(m, "[")
	if m.ed.Text() != "older" {
		t.Fatalf("Text = %q, want older", m.ed.Text())
	}
	press(m, "]")
	if m.day != note.Today() {
		t.Fatalf("day = %v, want back to today", m.day)
	}
}

func TestBrowsingSavesTheCurrentDayFirst(t *testing.T) {
	m := newTestModel(t, "")
	past := m.day.Add(-2)
	if err := m.store.Save(past, "older"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	press(m, "i", "z", "<esc>")

	press(m, "[")

	got, err := m.store.Load(note.Today())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "z" {
		t.Fatalf("today's note = %q, want z saved before browsing away", got)
	}
}

func TestBackslashReturnsToToday(t *testing.T) {
	m := newTestModel(t, "today")
	past := m.day.Add(-2)
	if err := m.store.Save(past, "older"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	press(m, "[")
	press(m, "\\")

	if m.day != note.Today() {
		t.Fatalf("day = %v, want today", m.day)
	}
	if m.ed.Text() != "today" {
		t.Fatalf("Text = %q, want today's note", m.ed.Text())
	}
}

func TestBrowsingPastTheOldestNoteSaysSo(t *testing.T) {
	m := newTestModel(t, "today")
	press(m, "[")
	if m.day != note.Today() {
		t.Fatalf("day = %v, want to stay put", m.day)
	}
	if m.status == "" {
		t.Fatal("running out of notes said nothing")
	}
}

func TestQuitSavesFirst(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "b", "y", "e", "<esc>")
	press(m, ":", "q")
	_, cmd := m.Update(keyMsg("enter"))

	if cmd == nil {
		t.Fatal("quit returned no command")
	}
	got, err := m.store.Load(m.day)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "bye" {
		t.Fatalf("saved %q, want bye", got)
	}
}

func TestCtrlCSavesAndQuits(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "h", "i")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("ctrl+c returned no command")
	}
	got, err := m.store.Load(m.day)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "hi" {
		t.Fatalf("saved %q, want hi", got)
	}
}

func TestViewShowsTheDateAndMode(t *testing.T) {
	m := newTestModel(t, "hello")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out := m.View().Content

	if !contains(out, m.day.String()) {
		t.Errorf("view is missing the date")
	}
	if !contains(out, "NORMAL") {
		t.Errorf("view is missing the mode")
	}
	if !contains(out, "hello") {
		t.Errorf("view is missing the note text")
	}
}

func TestCursorShapePerMode(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want tea.CursorShape
	}{
		{"normal is a block", nil, tea.CursorBlock},
		{"insert is a bar", []string{"i"}, tea.CursorBar},
		{"visual is a block", []string{"v"}, tea.CursorBlock},
		{"visual line is a block", []string{"V"}, tea.CursorBlock},
		{"back to normal is a block again", []string{"i", "<esc>"}, tea.CursorBlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t, "hello")
			press(m, tt.keys...)
			c := m.View().Cursor
			if c == nil {
				t.Fatal("no cursor reported")
			}
			if c.Shape != tt.want {
				t.Errorf("shape = %v, want %v", c.Shape, tt.want)
			}
		})
	}
}

// A nil color leaves the terminal's own cursor color alone, which is the point.
func TestCursorTakesTheTerminalColor(t *testing.T) {
	m := newTestModel(t, "hello")
	if c := m.View().Cursor; c == nil || c.Color != nil {
		t.Fatalf("cursor color = %v, want nil", c.Color)
	}
}

func TestCursorTracksTheCaret(t *testing.T) {
	m := newTestModel(t, "hello\nworld")
	press(m, "j", "l", "l")

	// One column for the border, plus the line-number gutter.
	c := m.View().Cursor
	wantX := 2 + 1 + editor.GutterWidth(2)
	if c == nil || c.X != wantX || c.Y != 2 {
		t.Fatalf("cursor = %v, want X %d Y 2", c, wantX)
	}
}

func TestCursorMovesToTheCommandLine(t *testing.T) {
	m := newTestModel(t, "hello")
	press(m, ":", "w", "q")

	c := m.View().Cursor
	if c == nil {
		t.Fatal("no cursor reported")
	}
	if c.Y != m.textHeight()+2 {
		t.Fatalf("cursor Y = %d, want the status row %d", c.Y, m.textHeight()+2)
	}
	if c.X != 1+len(":wq") {
		t.Fatalf("cursor X = %d, want %d", c.X, 1+len(":wq"))
	}
	if c.Shape != tea.CursorBar {
		t.Fatalf("shape = %v, want a bar on the command line", c.Shape)
	}
}

func TestFrameHasARoundedBorderAndDivider(t *testing.T) {
	m := newTestModel(t, "hello")
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")

	if len(rows) != 24 {
		t.Fatalf("frame is %d rows, want 24", len(rows))
	}
	if !strings.HasPrefix(rows[0], "╭") || !strings.HasSuffix(rows[0], "╮") {
		t.Errorf("top = %q, want a rounded top", rows[0])
	}
	if !strings.HasPrefix(rows[len(rows)-1], "╰") {
		t.Errorf("bottom = %q, want a rounded bottom", rows[len(rows)-1])
	}
	divider := rows[m.textHeight()+1]
	if !strings.HasPrefix(divider, "├") || !strings.HasSuffix(divider, "┤") {
		t.Errorf("divider = %q, want a divider above the status line", divider)
	}
}

func TestEveryFrameRowIsTheFullWidth(t *testing.T) {
	m := newTestModel(t, "hello\nworld")
	for i, row := range strings.Split(m.View().Content, "\n") {
		if got := ansi.StringWidth(row); got != 80 {
			t.Errorf("row %d is %d wide, want 80", i, got)
		}
	}
}

func TestStatusPutsTheFilenameHardRight(t *testing.T) {
	m := newTestModel(t, "hello")
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	status := rows[m.textHeight()+2]

	if !strings.HasSuffix(status, m.day.String()+".md │") {
		t.Fatalf("status = %q, want the filename hard right", status)
	}
	if !strings.Contains(status, "│ NORMAL") {
		t.Fatalf("status = %q, want the mode bottom left", status)
	}
}

func TestModeIndicatorChangesColor(t *testing.T) {
	m := newTestModel(t, "hello")
	normal := m.View().Content
	press(m, "i")
	insert := m.View().Content

	if normal == insert {
		t.Fatal("the mode indicator looks the same in normal and insert")
	}
}

func TestHelpModalOpensAndCloses(t *testing.T) {
	m := newTestModel(t, "hello")
	press(m, "?")

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "iw aw") {
		t.Fatal("help did not list the text objects")
	}
	if strings.Contains(out, "hello") {
		t.Fatal("help did not cover the note")
	}
	if m.View().Cursor != nil {
		t.Fatal("the cursor is still showing over the help modal")
	}

	press(m, "j")
	if strings.Contains(ansi.Strip(m.View().Content), "iw aw") {
		t.Fatal("a keypress did not close help")
	}
}

// The binding list is the whole point of the modal, so none of it may be
// clipped at the sizes a note gets written in.
func TestHelpFitsWithoutClipping(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {72, 20}, {100, 30}, {120, 40}} {
		m := newTestModel(t, "")
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		press(m, "?")
		out := ansi.Strip(m.View().Content)

		for _, group := range []string{"Modes", "Move", "Edit", "Notes"} {
			if !strings.Contains(out, group) {
				t.Errorf("%dx%d: help is missing the %s group", size[0], size[1], group)
			}
		}
		for _, key := range []string{"iw aw", "; ,", "ctrl+v", "ZZ", "left down up right", "quote, paren, para"} {
			if !strings.Contains(out, key) {
				t.Errorf("%dx%d: help is missing %q", size[0], size[1], key)
			}
		}
	}
}

func TestHelpStaysInsideTheFrame(t *testing.T) {
	m := newTestModel(t, "")
	m.Update(tea.WindowSizeMsg{Width: 72, Height: 20})
	press(m, "?")
	for i, row := range strings.Split(m.View().Content, "\n") {
		if got := ansi.StringWidth(row); got != 72 {
			t.Errorf("help row %d is %d wide, want 72", i, got)
		}
	}
}

func TestHelpKeyIsLiteralInInsertMode(t *testing.T) {
	m := newTestModel(t, "")
	press(m, "i", "?")
	if m.ed.Text() != "?" {
		t.Fatalf("Text = %q, want a literal question mark", m.ed.Text())
	}
	if m.help {
		t.Fatal("? opened help while inserting")
	}
}

func TestKeyThatClosesHelpDoesNotAlsoEdit(t *testing.T) {
	m := newTestModel(t, "abc")
	press(m, "?", "x")
	if m.ed.Text() != "abc" {
		t.Fatalf("Text = %q, want unchanged. The dismiss key leaked through", m.ed.Text())
	}
}

func TestViewMarksUnsavedEdits(t *testing.T) {
	m := newTestModel(t, "")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	clean := m.View().Content

	press(m, "i", "x")
	dirty := m.View().Content

	if clean == dirty {
		t.Fatal("the view looks the same saved and unsaved")
	}
}

// --- helpers ---

func newTestModel(t *testing.T, text string) *Model {
	t.Helper()
	s, err := note.New(t.TempDir())
	if err != nil {
		t.Fatalf("New store: %v", err)
	}
	day := note.Today()
	if text != "" {
		if err := s.Save(day, text); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	m, err := NewModel(s, nil)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		m.Update(keyMsg(k))
	}
}

// keyMsg builds a keystroke, taking <name> for the named keys.
func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "<esc>":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	r := []rune(k)[0]
	// Match the terminal: a capital arrives lowercase in Code, shifted in Text.
	if unicode.IsUpper(r) {
		return tea.KeyPressMsg{Text: k, Code: unicode.ToLower(r), Mod: tea.ModShift}
	}
	return tea.KeyPressMsg{Text: k, Code: r}
}

func writeFromElsewhere(t *testing.T, m *Model, text string) {
	t.Helper()
	other, err := note.New(m.store.Dir())
	if err != nil {
		t.Fatalf("New store: %v", err)
	}
	if err := other.Save(m.day, text); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// contains ignores styling, so an inverted cursor rune does not split a word.
func contains(haystack, needle string) bool {
	return strings.Contains(ansi.Strip(haystack), needle)
}
