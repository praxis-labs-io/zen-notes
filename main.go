// Command zen-notes opens today's note. One markdown file per day, edited with
// vim motions, autosaved, and reloaded when another window writes it.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/praxis-labs-io/zen-notes/internal/app"
	"github.com/praxis-labs-io/zen-notes/internal/note"
	"github.com/praxis-labs-io/zen-notes/internal/version"
)

func main() {
	dir := flag.String("dir", "", "notes directory (default $ZEN_NOTES_DIR, else ~/.zen-notes)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	// A released binary is one somebody downloaded rather than built, so it has
	// to be able to say which one it is. A source build reports dev.
	if *showVersion {
		fmt.Println("zen-notes version", version.Version)
		return
	}

	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "zen-notes:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	if dir == "" {
		var err error
		if dir, err = note.DefaultDir(); err != nil {
			return err
		}
	}

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
