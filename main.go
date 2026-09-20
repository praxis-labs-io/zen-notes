// Command zen-notes opens today's note for editing.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/praxis-labs-io/zen-notes/internal/app"
	"github.com/praxis-labs-io/zen-notes/internal/mockup"
	"github.com/praxis-labs-io/zen-notes/internal/note"
	"github.com/praxis-labs-io/zen-notes/internal/version"
)

func main() {
	dir := flag.String("dir", "", "notes directory (default $ZEN_NOTES_DIR, else ~/.zen-notes)")
	mock := flag.Bool("mockup", false, "render over fixture notes in a throwaway directory")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("zen-notes version", version.Version)
		return
	}

	if err := run(*dir, *mock); err != nil {
		fmt.Fprintln(os.Stderr, "zen-notes:", err)
		os.Exit(1)
	}
}

func run(dir string, mock bool) error {
	dir, cleanup, err := resolveDir(dir, mock)
	if err != nil {
		return err
	}
	defer cleanup()

	store, err := note.New(dir)
	if err != nil {
		return err
	}
	watcher, err := note.Watch(store.Dir())
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	model, err := app.NewModel(store, watcher)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model).Run()
	return err
}

// resolveDir returns the notes directory to open and the cleanup that discards
// it. --mockup refuses --dir, so fixtures can never land in real notes.
func resolveDir(dir string, mock bool) (string, func(), error) {
	keep := func() {}
	if !mock {
		if dir != "" {
			return dir, keep, nil
		}
		dir, err := note.DefaultDir()
		return dir, keep, err
	}
	if dir != "" {
		return "", keep, errors.New("--mockup brings its own notes, so it cannot be combined with --dir")
	}
	seeded, err := mockup.Seed()
	if err != nil {
		return "", keep, err
	}
	return seeded, func() { _ = os.RemoveAll(seeded) }, nil
}
