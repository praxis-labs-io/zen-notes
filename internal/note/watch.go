package note

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher reports note files changing on disk. It watches the directory
// rather than a file, because an atomic save replaces the file's inode.
type Watcher struct {
	fs      *fsnotify.Watcher
	changes chan string
	done    chan struct{}
	once    sync.Once
}

// Watch starts reporting changes to the .md notes in dir.
func Watch(dir string) (*Watcher, error) {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	if err := fs.Add(dir); err != nil {
		_ = fs.Close()
		return nil, fmt.Errorf("watch %s: %w", dir, err)
	}

	w := &Watcher{
		fs:      fs,
		changes: make(chan string, 8),
		done:    make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

// Changes delivers the base name of each note that changed. It closes when
// the watcher does.
func (w *Watcher) Changes() <-chan string { return w.changes }

// Close stops watching. It is safe to call more than once.
func (w *Watcher) Close() error {
	w.once.Do(func() { close(w.done) })
	return w.fs.Close()
}

func (w *Watcher) loop() {
	defer close(w.changes)
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if !isNoteWrite(event) {
				continue
			}
			w.send(baseName(event.Name))
		case _, ok := <-w.fs.Errors:
			if !ok {
				return
			}
		}
	}
}

// send never blocks, so a busy consumer cannot stall the watcher. Dropping a
// duplicate is harmless because the app re-reads the whole file.
func (w *Watcher) send(name string) {
	select {
	case w.changes <- name:
	case <-w.done:
	default:
	}
}

// isNoteWrite reports whether an event means a dated note gained new content.
func isNoteWrite(e fsnotify.Event) bool {
	if !e.Has(fsnotify.Write) && !e.Has(fsnotify.Create) && !e.Has(fsnotify.Rename) {
		return false
	}
	name := baseName(e.Name)
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	_, ok := ParseDay(strings.TrimSuffix(name, ".md"))
	return ok
}

func baseName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
