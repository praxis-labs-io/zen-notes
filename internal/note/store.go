// Package note stores one markdown file per calendar day.
package note

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const dayLayout = "2006-01-02"

// Day is a calendar date with no time or zone attached.
type Day struct {
	Year  int
	Month time.Month
	Date  int
}

// Today returns the current date in the local zone.
func Today() Day {
	return dayOf(time.Now())
}

// ParseDay reads a YYYY-MM-DD date, reporting whether it was valid.
func ParseDay(s string) (Day, bool) {
	t, err := time.ParseInLocation(dayLayout, s, time.Local)
	if err != nil {
		return Day{}, false
	}
	return dayOf(t), true
}

func dayOf(t time.Time) Day {
	y, m, d := t.Date()
	return Day{y, m, d}
}

func (d Day) String() string {
	return d.time().Format(dayLayout)
}

// Add returns the date n days later, or earlier when n is negative.
func (d Day) Add(n int) Day {
	return dayOf(d.time().AddDate(0, 0, n))
}

// Before reports whether d falls earlier in the calendar than o.
func (d Day) Before(o Day) bool {
	return d.time().Before(o.time())
}

func (d Day) time() time.Time {
	return time.Date(d.Year, d.Month, d.Date, 0, 0, 0, 0, time.Local)
}

// DefaultDir is $ZEN_NOTES_DIR when set, else ~/.zen-notes.
func DefaultDir() (string, error) {
	if dir := os.Getenv("ZEN_NOTES_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".zen-notes"), nil
}

// Store reads and writes the day notes under a single directory.
type Store struct {
	dir string
}

// New creates the directory if it does not exist.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create notes directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir is the directory holding the notes.
func (s *Store) Dir() string { return s.dir }

// Path is where the given day's note lives, whether or not it exists yet.
func (s *Store) Path(d Day) string {
	return filepath.Join(s.dir, d.String()+".md")
}

// Load returns the day's note, or empty if it has never been saved.
func (s *Store) Load(d Day) (string, error) {
	b, err := os.ReadFile(s.Path(d))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read note %s: %w", d, err)
	}
	return string(b), nil
}

// Save replaces the day's note. It writes a temp file and renames it over the
// target so a concurrent reader never sees a partial note.
func (s *Store) Save(d Day, content string) error {
	f, err := os.CreateTemp(s.dir, ".zen-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp note: %w", err)
	}
	tmp := f.Name()
	// Gone already once the rename lands, so a failure here says nothing.
	defer func() { _ = os.Remove(tmp) }()

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp note: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp note: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp note: %w", err)
	}
	if err := os.Rename(tmp, s.Path(d)); err != nil {
		return fmt.Errorf("replace note %s: %w", d, err)
	}
	return nil
}

// Days lists the dates that have a saved note, oldest first. os.ReadDir
// sorts by filename, and YYYY-MM-DD sorts chronologically.
func (s *Store) Days() ([]Day, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read notes directory: %w", err)
	}
	var days []Day
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if d, ok := ParseDay(strings.TrimSuffix(name, ".md")); ok {
			days = append(days, d)
		}
	}
	return days, nil
}

// Prev is the newest saved day older than d, if there is one.
func (s *Store) Prev(d Day) (Day, bool, error) {
	days, err := s.Days()
	if err != nil {
		return Day{}, false, err
	}
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].Before(d) {
			return days[i], true, nil
		}
	}
	return Day{}, false, nil
}

// Next is the oldest saved day newer than d, if there is one.
func (s *Store) Next(d Day) (Day, bool, error) {
	days, err := s.Days()
	if err != nil {
		return Day{}, false, err
	}
	for _, day := range days {
		if d.Before(day) {
			return day, true, nil
		}
	}
	return Day{}, false, nil
}
