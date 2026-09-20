package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/praxis-labs-io/zen-notes/internal/note"
)

func TestResolveDirRefusesMockupWithDir(t *testing.T) {
	real := t.TempDir()
	if _, _, err := resolveDir(real, true); err == nil {
		t.Fatal("resolveDir(dir, true) succeeded, want an error")
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("%d files written to the given directory, want none", len(entries))
	}
}

func TestResolveDirSeedsAndDiscardsMockup(t *testing.T) {
	t.Setenv("ZEN_NOTES_DIR", filepath.Join(t.TempDir(), "real"))

	dir, cleanup, err := resolveDir("", true)
	if err != nil {
		t.Fatalf("resolveDir(\"\", true) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, note.Today().String()+".md")); err != nil {
		t.Fatalf("today's fixture note: %v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) after cleanup = %v, want not-exist", dir, err)
	}
}

func TestResolveDirWithoutMockup(t *testing.T) {
	env := t.TempDir()
	t.Setenv("ZEN_NOTES_DIR", env)
	given := t.TempDir()

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"flag wins", given, given},
		{"env otherwise", "", env},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup, err := resolveDir(tt.dir, false)
			if err != nil {
				t.Fatalf("resolveDir(%q, false) error = %v", tt.dir, err)
			}
			if dir != tt.want {
				t.Errorf("dir = %q, want %q", dir, tt.want)
			}
			cleanup()
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("cleanup removed a real notes directory: %v", err)
			}
		})
	}
}
