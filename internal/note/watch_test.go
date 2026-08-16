package note

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherReportsForeignWrites(t *testing.T) {
	s := newTestStore(t)
	day := Day{2026, time.August, 16}
	if err := s.Save(day, "original"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	w, err := Watch(s.Dir())
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	defer w.Close()

	other, err := New(s.Dir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := other.Save(day, "from elsewhere"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := awaitChange(t, w); got != day.String()+".md" {
		t.Fatalf("changed file = %q, want %s.md", got, day)
	}
}

func TestWatcherIgnoresOtherFiles(t *testing.T) {
	s := newTestStore(t)
	w, err := Watch(s.Dir())
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	defer w.Close()

	if err := os.WriteFile(filepath.Join(s.Dir(), "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := s.Save(Day{2026, time.August, 16}, "real note"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := awaitChange(t, w); got != "2026-08-16.md" {
		t.Fatalf("changed file = %q, want the note not the txt", got)
	}
}

func TestWatcherClosesItsChannel(t *testing.T) {
	s := newTestStore(t)
	w, err := Watch(s.Dir())
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	w.Close()

	select {
	case _, open := <-w.Changes():
		if open {
			t.Fatal("channel still delivering after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel never closed")
	}
}

func awaitChange(t *testing.T, w *Watcher) string {
	t.Helper()
	select {
	case name := <-w.Changes():
		return name
	case <-time.After(5 * time.Second):
		t.Fatal("no change reported")
		return ""
	}
}
