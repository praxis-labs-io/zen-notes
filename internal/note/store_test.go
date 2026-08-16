package note

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDayString(t *testing.T) {
	d := Day{2026, time.August, 16}
	if got := d.String(); got != "2026-08-16" {
		t.Fatalf("String() = %q, want 2026-08-16", got)
	}
}

func TestParseDay(t *testing.T) {
	tests := []struct {
		in   string
		want Day
		ok   bool
	}{
		{"2026-08-16", Day{2026, time.August, 16}, true},
		{"2026-1-5", Day{}, false},
		{"not a date", Day{}, false},
		{"2026-13-40", Day{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseDay(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseDay(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDayAddCrossesMonth(t *testing.T) {
	got := Day{2026, time.August, 31}.Add(1)
	want := Day{2026, time.September, 1}
	if got != want {
		t.Fatalf("Add(1) = %v, want %v", got, want)
	}
}

func TestDayBefore(t *testing.T) {
	a := Day{2026, time.August, 16}
	b := Day{2026, time.September, 1}
	if !a.Before(b) {
		t.Error("a.Before(b) = false, want true")
	}
	if b.Before(a) {
		t.Error("b.Before(a) = true, want false")
	}
	if a.Before(a) {
		t.Error("a.Before(a) = true, want false")
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Load(Day{2026, time.August, 16})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "" {
		t.Fatalf("Load of missing note = %q, want empty", got)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s := newTestStore(t)
	day := Day{2026, time.August, 16}
	want := "# Monday\n\n- [ ] ship it\n"

	if err := s.Save(day, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(day)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("Load = %q, want %q", got, want)
	}
}

func TestSaveWritesDatedFilename(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Day{2026, time.August, 16}, "hi"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "2026-08-16.md")); err != nil {
		t.Fatalf("expected 2026-08-16.md: %v", err)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Day{2026, time.August, 16}, "hi"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir holds %v, want only the note", names)
	}
}

func TestSaveOverwrites(t *testing.T) {
	s := newTestStore(t)
	day := Day{2026, time.August, 16}
	if err := s.Save(day, "first"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(day, "second"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(day)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "second" {
		t.Fatalf("Load = %q, want %q", got, "second")
	}
}

func TestDaysListsSavedNotesOldestFirst(t *testing.T) {
	s := newTestStore(t)
	for _, d := range []Day{{2026, time.August, 16}, {2026, time.July, 4}, {2026, time.September, 1}} {
		if err := s.Save(d, "x"); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := s.Days()
	if err != nil {
		t.Fatalf("Days: %v", err)
	}
	want := []Day{{2026, time.July, 4}, {2026, time.August, 16}, {2026, time.September, 1}}
	if len(got) != len(want) {
		t.Fatalf("Days() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Days() = %v, want %v", got, want)
		}
	}
}

func TestPrevSkipsDaysWithoutNotes(t *testing.T) {
	s := newTestStore(t)
	for _, d := range []Day{{2026, time.July, 4}, {2026, time.August, 16}} {
		if err := s.Save(d, "x"); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, ok, err := s.Prev(Day{2026, time.August, 16})
	if err != nil {
		t.Fatalf("Prev: %v", err)
	}
	if !ok || got != (Day{2026, time.July, 4}) {
		t.Fatalf("Prev = %v, %v; want 2026-07-04, true", got, ok)
	}

	if _, ok, err := s.Prev(Day{2026, time.July, 4}); err != nil || ok {
		t.Fatalf("Prev of oldest = %v, want false", ok)
	}
}

func TestNextSkipsDaysWithoutNotes(t *testing.T) {
	s := newTestStore(t)
	for _, d := range []Day{{2026, time.July, 4}, {2026, time.August, 16}} {
		if err := s.Save(d, "x"); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, ok, err := s.Next(Day{2026, time.July, 4})
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !ok || got != (Day{2026, time.August, 16}) {
		t.Fatalf("Next = %v, %v; want 2026-08-16, true", got, ok)
	}

	if _, ok, err := s.Next(Day{2026, time.August, 16}); err != nil || ok {
		t.Fatalf("Next of newest = %v, want false", ok)
	}
}

// Prev and Next work from a day with no note of its own, so today still
// navigates backward before it has been saved.
func TestPrevNextFromUnsavedDay(t *testing.T) {
	s := newTestStore(t)
	for _, d := range []Day{{2026, time.July, 4}, {2026, time.September, 1}} {
		if err := s.Save(d, "x"); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, ok, err := s.Prev(Day{2026, time.August, 16})
	if err != nil || !ok || got != (Day{2026, time.July, 4}) {
		t.Fatalf("Prev = %v, %v, %v; want 2026-07-04, true, nil", got, ok, err)
	}
	got, ok, err = s.Next(Day{2026, time.August, 16})
	if err != nil || !ok || got != (Day{2026, time.September, 1}) {
		t.Fatalf("Next = %v, %v, %v; want 2026-09-01, true, nil", got, ok, err)
	}
}

func TestNewCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "notes")
	if _, err := New(dir); err != nil {
		t.Fatalf("New: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("New did not create a directory")
	}
}

func TestDefaultDirPrefersEnv(t *testing.T) {
	t.Setenv("ZEN_NOTES_DIR", "/tmp/zen-notes-test")
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if got != "/tmp/zen-notes-test" {
		t.Fatalf("DefaultDir = %q, want /tmp/zen-notes-test", got)
	}
}

func TestDefaultDirFallsBackToHome(t *testing.T) {
	t.Setenv("ZEN_NOTES_DIR", "")
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	if got != filepath.Join(home, ".zen-notes") {
		t.Fatalf("DefaultDir = %q, want %q", got, filepath.Join(home, ".zen-notes"))
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}
