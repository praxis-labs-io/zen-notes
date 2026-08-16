// Command zen opens today's note. One markdown file per day, edited with vim
// motions, autosaved, and reloaded when another window writes it.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/drucial/zen-notes/internal/app"
	"github.com/drucial/zen-notes/internal/note"
)

func main() {
	dir := flag.String("dir", "", "notes directory (default $ZEN_NOTES_DIR, else ~/.zen-notes)")
	flag.Parse()

	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "zen:", err)
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
	defer watcher.Close()

	model, err := app.NewModel(store, watcher)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model).Run()
	return err
}
