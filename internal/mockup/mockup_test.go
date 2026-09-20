package mockup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/praxis-labs-io/zen-notes/internal/editor"
	"github.com/praxis-labs-io/zen-notes/internal/note"
)

func seed(t *testing.T) string {
	t.Helper()
	dir, err := Seed()
	if err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestSeedDatesEveryFixtureBackFromToday(t *testing.T) {
	dir := seed(t)

	entries, err := files.ReadDir(notesDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", notesDir, err)
	}
	if len(entries) < 5 {
		t.Fatalf("%d fixture notes, want at least 5 to be worth paging through", len(entries))
	}

	today := note.Today()
	for _, e := range entries {
		age, err := daysBack(e.Name())
		if err != nil {
			t.Fatalf("daysBack(%q) error = %v", e.Name(), err)
		}
		path := filepath.Join(dir, today.Add(-age).String()+".md")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("fixture %s: %v", e.Name(), err)
			continue
		}
		if len(body) == 0 {
			t.Errorf("fixture %s is empty", e.Name())
		}
	}
}

// The app opens today's note, so a mockup without one starts on a blank buffer.
func TestSeedWritesTodaysNote(t *testing.T) {
	dir := seed(t)

	store, err := note.New(dir)
	if err != nil {
		t.Fatalf("note.New() error = %v", err)
	}
	text, err := store.Load(note.Today())
	if err != nil {
		t.Fatalf("Load(today) error = %v", err)
	}
	if text == "" {
		t.Fatal("today has no note")
	}
}

func TestSeedResolvesEveryImageTarget(t *testing.T) {
	dir := seed(t)

	days, err := note.New(dir)
	if err != nil {
		t.Fatalf("note.New() error = %v", err)
	}
	saved, err := days.Days()
	if err != nil {
		t.Fatalf("Days() error = %v", err)
	}

	found := 0
	for _, day := range saved {
		text, err := days.Load(day)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", day, err)
		}
		for _, target := range editor.New(text).ImageTargets() {
			found++
			if _, err := os.Stat(filepath.Join(dir, target)); err != nil {
				t.Errorf("%s draws %q: %v", day, target, err)
			}
		}
	}
	if found == 0 {
		t.Error("no fixture note draws an image")
	}
}

// A fixture set that stops covering a surface makes the demo quietly misleading.
func TestFixturesCoverTheMarkdownSurfaces(t *testing.T) {
	dir := seed(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	var all strings.Builder
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		all.Write(body)
		all.WriteString("\n")
	}
	text := all.String()

	tests := []struct {
		surface string
		want    string
	}{
		{"heading", "\n# "},
		{"subheading", "\n## "},
		{"bullet list", "\n- "},
		{"ordered list", "\n1. "},
		{"nested list", "\n  - "},
		{"done task", "- [x] "},
		{"open task", "- [ ] "},
		{"blockquote", "\n> "},
		{"fenced code", "\n```"},
		{"inline code", "`Render`"},
		{"strong", "**"},
		{"emphasis", "*mid-session*"},
		{"web link", "](https://"},
		{"image", "\n!["},
	}
	for _, tt := range tests {
		if !strings.Contains(text, tt.want) {
			t.Errorf("no fixture note has a %s (%q)", tt.surface, tt.want)
		}
	}
}
