// Package mockup seeds a throwaway notes directory with fixture notes.
package mockup

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/praxis-labs-io/zen-notes/internal/note"
)

//go:embed notes/*.md sketch.png
var files embed.FS

// Each fixture note is named for its age in days, so today always has one.
const notesDir = "notes"

const imageName = "sketch.png"

// Seed writes the fixture notes and their image into a new temporary
// directory, dated back from today. The caller owns the directory.
func Seed() (string, error) {
	dir, err := os.MkdirTemp("", "zen-notes-mockup-")
	if err != nil {
		return "", fmt.Errorf("create mockup directory: %w", err)
	}
	if err := fill(dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func fill(dir string) error {
	entries, err := files.ReadDir(notesDir)
	if err != nil {
		return fmt.Errorf("read fixture notes: %w", err)
	}

	today := note.Today()
	for _, e := range entries {
		age, err := daysBack(e.Name())
		if err != nil {
			return err
		}
		body, err := files.ReadFile(path.Join(notesDir, e.Name()))
		if err != nil {
			return fmt.Errorf("read fixture note %s: %w", e.Name(), err)
		}
		day := today.Add(-age)
		if err := os.WriteFile(filepath.Join(dir, day.String()+".md"), body, 0o644); err != nil {
			return fmt.Errorf("write fixture note %s: %w", day, err)
		}
	}

	image, err := files.ReadFile(imageName)
	if err != nil {
		return fmt.Errorf("read fixture image: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, imageName), image, 0o644); err != nil {
		return fmt.Errorf("write fixture image: %w", err)
	}
	return nil
}

func daysBack(name string) (int, error) {
	age, err := strconv.Atoi(strings.TrimSuffix(name, ".md"))
	if err != nil || age < 0 {
		return 0, fmt.Errorf("fixture note %s is not named for a number of days", name)
	}
	return age, nil
}
