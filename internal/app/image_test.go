package app

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

// writeImage puts a natW x natH image on disk and returns its path.
func writeImage(t *testing.T, dir, name string, natW, natH int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, natW, natH))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if strings.HasSuffix(name, ".jpg") {
		err = jpeg.Encode(f, img, nil)
	} else {
		err = png.Encode(f, img)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("encode %s: %v", name, err)
	}
	return path
}

func TestResolveImagePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	tests := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{"relative", "pic.png", "/notes/pic.png", false},
		{"nested relative", "img/pic.png", "/notes/img/pic.png", false},
		{"dot segments", "./img/../pic.png", "/notes/pic.png", false},
		{"absolute", "/tmp/pic.png", "/tmp/pic.png", false},
		{"home", "~/pic.png", filepath.Join(home, "pic.png"), false},
		{"bare tilde name", "~notpic/pic.png", "/notes/~notpic/pic.png", false},
		{"http", "http://example.com/pic.png", "", true},
		{"https", "https://example.com/pic.png", "", true},
		{"data uri", "data:image/png;base64,AAAA", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveImagePath(tt.target, "/notes")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveImagePath(%q) = %q, want an error", tt.target, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("resolveImagePath(%q) = %q/%v, want %q", tt.target, got, err, tt.want)
			}
		})
	}
}

// A single-letter Windows drive is a path, not a scheme.
func TestResolveImagePathKeepsWindowsDriveLetters(t *testing.T) {
	got, err := resolveImagePath(`C:/pics/pic.png`, "/notes")
	if err != nil {
		t.Fatalf("resolveImagePath = %v, want a path", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(got), "C:/pics/pic.png") {
		t.Fatalf("resolveImagePath = %q, want the drive path kept", got)
	}
}

func TestFitImage(t *testing.T) {
	dir := t.TempDir()
	const cellW, cellH = 10, 20

	tests := []struct {
		name             string
		natW, natH       int
		maxCols, maxRows int
		cols, rows       int
	}{
		// 100x200px is 10x10 cells, and fits as it is.
		{"fits as is", 100, 200, 40, 20, 10, 10},
		// Twice too wide: halved, and the height halves with it.
		{"width bound", 200, 200, 10, 20, 10, 5},
		// Twice too tall: halved, and the width halves with it.
		{"height bound", 200, 400, 20, 10, 10, 10},
		// Never enlarged past its natural size.
		{"small image", 10, 20, 40, 20, 1, 1},
		// A partial cell still takes a whole cell.
		{"rounds up", 105, 205, 40, 20, 11, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeImage(t, dir, strings.ReplaceAll(tt.name, " ", "-")+".png", tt.natW, tt.natH)
			cols, rows, err := fitImage(path, cellW, cellH, tt.maxCols, tt.maxRows)
			if err != nil {
				t.Fatalf("fitImage: %v", err)
			}
			if cols != tt.cols || rows != tt.rows {
				t.Fatalf("fitImage = %dx%d cells, want %dx%d", cols, rows, tt.cols, tt.rows)
			}
		})
	}
}

func TestFitImageNeverExceedsItsBounds(t *testing.T) {
	dir := t.TempDir()
	path := writeImage(t, dir, "wide.png", 4000, 3000)
	cols, rows, err := fitImage(path, 7, 15, 30, 8)
	if err != nil {
		t.Fatalf("fitImage: %v", err)
	}
	if cols > 30 || rows > 8 || cols < 1 || rows < 1 {
		t.Fatalf("fitImage = %dx%d cells, want within 30x8", cols, rows)
	}
}

func TestFitImageRejectsWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	notAnImage := filepath.Join(dir, "note.png")
	if err := os.WriteFile(notAnImage, []byte("not a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fitImage(notAnImage, 10, 20, 40, 20); err == nil {
		t.Fatal("fitImage accepted a file that is not an image")
	}
	if _, _, err := fitImage(filepath.Join(dir, "missing.png"), 10, 20, 40, 20); err == nil {
		t.Fatal("fitImage accepted a missing file")
	}
}

func TestTransmitImageSendsAPNGByPath(t *testing.T) {
	dir := t.TempDir()
	path := writeImage(t, dir, "pic.png", 40, 40)

	seq, err := transmitImage(imageEntry{id: 3, path: path})
	if err != nil {
		t.Fatalf("transmitImage: %v", err)
	}
	if !strings.HasPrefix(seq, "\x1b_G") || !strings.HasSuffix(seq, "\x1b\\") {
		t.Fatalf("transmitImage = %q, want one graphics sequence", seq)
	}
	// t=f is the file transmission, so the pixels never cross the tty.
	if !strings.Contains(seq, "t=f") || !strings.Contains(seq, "i=3") {
		t.Fatalf("transmitImage = %q, want a file transmission for id 3", seq)
	}
	if !strings.Contains(seq, "U=1") {
		t.Fatalf("transmitImage = %q, want a virtual placement", seq)
	}
	// The payload is base64, which x/ansi does not do for us.
	if strings.Contains(seq, path) {
		t.Fatalf("transmitImage sent the path unencoded: %q", seq)
	}
}

func TestTransmitImageReencodesOtherFormats(t *testing.T) {
	dir := t.TempDir()
	path := writeImage(t, dir, "pic.jpg", 40, 40)

	seq, err := transmitImage(imageEntry{id: 4, path: path})
	if err != nil {
		t.Fatalf("transmitImage: %v", err)
	}
	// Direct transmission, because the protocol only compresses PNG.
	if strings.Contains(seq, "t=f") {
		t.Fatalf("transmitImage sent a JPEG by path: %q", seq)
	}
	if !strings.Contains(seq, "f=100") {
		t.Fatalf("transmitImage = %q, want the image re-encoded as PNG", seq)
	}
}

func TestImagesResolveTransmitsOnceUntilSomethingChanges(t *testing.T) {
	dir := t.TempDir()
	writeImage(t, dir, "pic.png", 100, 200)
	i := newImages()
	i.cellW, i.cellH = 10, 20

	first, seq, err := i.resolve("pic.png", dir, 40, 20)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if seq == "" {
		t.Fatal("resolve did not transmit a new image")
	}

	_, seq, err = i.resolve("pic.png", dir, 40, 20)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if seq != "" {
		t.Fatal("resolve re-transmitted an unchanged image")
	}

	// A tighter fit is a different placement, so it has to be sent again.
	second, seq, err := i.resolve("pic.png", dir, 4, 20)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if seq == "" {
		t.Fatal("resolve did not re-transmit after the fit changed")
	}
	if second.id != first.id {
		t.Fatalf("resolve took a new id %d for the same target, want %d", second.id, first.id)
	}
}

func TestImagesResolveRejectsWhatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	i := newImages()
	i.cellW, i.cellH = 10, 20

	for _, target := range []string{"missing.png", "https://example.com/pic.png", ""} {
		if _, _, err := i.resolve(target, dir, 40, 20); err == nil {
			t.Fatalf("resolve(%q) succeeded, want an error", target)
		}
	}
	if len(i.entries) != 0 {
		t.Fatalf("resolve recorded %d entries for targets it rejected", len(i.entries))
	}
}

func TestImagesAreOffUntilTheTerminalReportsACellSize(t *testing.T) {
	i := newImages()
	if i.supported() {
		t.Fatal("images reported supported before the terminal answered")
	}
	i.cellW, i.cellH = 10, 20
	if !i.supported() {
		t.Fatal("images reported unsupported after a cell size arrived")
	}
}

func TestDeleteImageTargetsOneID(t *testing.T) {
	seq := deleteImage(9)
	if !strings.Contains(seq, "a=d") || !strings.Contains(seq, "d=I") || !strings.Contains(seq, "i=9") {
		t.Fatalf("deleteImage = %q, want a delete for id 9", seq)
	}
}

// countingFS records how often the header of an image is read, which is the
// expensive half of a resolve.
func TestResolveReadsTheHeaderOnlyWhenSomethingMoved(t *testing.T) {
	dir := t.TempDir()
	path := writeImage(t, dir, "pic.png", 100, 200)
	i := newImages()
	i.cellW, i.cellH = 10, 20

	if _, _, err := i.resolve("pic.png", dir, 40, 20); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Make the file unreadable as an image. A resolve that still re-reads the
	// header would now fail; one that trusts the cache returns what it had.
	if err := os.WriteFile(path, []byte("no longer a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Restore the recorded stat so the entry still looks unchanged.
	entry := i.entries["pic.png"]
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry.size, entry.mtime = stat.Size(), stat.ModTime()
	i.entries["pic.png"] = entry

	if _, _, err := i.resolve("pic.png", dir, 40, 20); err != nil {
		t.Fatalf("resolve re-read the header of an unchanged image: %v", err)
	}

	// Different bounds mean a different fit, so the header has to be read,
	// and now it cannot be.
	if _, _, err := i.resolve("pic.png", dir, 4, 20); err == nil {
		t.Fatal("resolve skipped the fit after the bounds changed")
	}
}

func TestClaimIDNeverTakesAnIDInUse(t *testing.T) {
	i := newImages()
	for n := range maxImageID {
		target := fmt.Sprintf("pic%d.png", n)
		i.entries[target] = imageEntry{id: i.claimID(target)}
	}

	live := map[int]bool{}
	for target, e := range i.entries {
		if live[e.id] {
			t.Fatalf("id %d handed out twice, latest to %s", e.id, target)
		}
		live[e.id] = true
	}
	if len(live) != maxImageID {
		t.Fatalf("claimID produced %d distinct ids, want %d", len(live), maxImageID)
	}

	// Freeing one id makes exactly that id the one available again. A counter
	// that only wraps would hand out whatever it landed on next.
	freed := i.entries["pic7.png"].id
	delete(i.entries, "pic7.png")
	if got := i.claimID("fresh.png"); got != freed {
		t.Fatalf("claimID = %d, want the freed id %d", got, freed)
	}
}

func TestClaimIDReusesATargetsOwnID(t *testing.T) {
	i := newImages()
	first := i.claimID("pic.png")
	i.entries["pic.png"] = imageEntry{id: first}
	if again := i.claimID("pic.png"); again != first {
		t.Fatalf("claimID = %d for a known target, want %d", again, first)
	}
}

// A target that resolved once and then stops must be released, or its entry,
// its terminal-side image and its id are held for the rest of the session.
func TestSyncImagesReleasesATargetThatStopsResolving(t *testing.T) {
	m := newTestModel(t, "![](pic.png)")
	path := writeImage(t, m.store.Dir(), "pic.png", 100, 200)
	m.images.cellW, m.images.cellH = 10, 20

	m.syncImages()
	if len(m.images.entries) != 1 {
		t.Fatalf("entries = %d after a good resolve, want 1", len(m.images.entries))
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	m.syncImages()

	if len(m.images.entries) != 0 {
		t.Fatalf("entries = %d after the file went away, want 0", len(m.images.entries))
	}
	if got := m.ed.Render(80, 24); strings.Contains(got.Content, string(kitty.Placeholder)) {
		t.Fatal("image rows survived the file going away")
	}
}

func TestSyncImagesDropsPlacementsWhenThereIsNoRoom(t *testing.T) {
	m := newTestModel(t, "![](pic.png)")
	writeImage(t, m.store.Dir(), "pic.png", 100, 200)
	m.images.cellW, m.images.cellH = 10, 20

	m.syncImages()
	if got := m.ed.Render(80, 24); !strings.Contains(got.Content, string(kitty.Placeholder)) {
		t.Fatal("no image placed at a normal window size")
	}

	// textHeight floors at 1, so half of it is zero and nothing can be drawn.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 2})
	if got := m.ed.Render(80, 2); strings.Contains(got.Content, string(kitty.Placeholder)) {
		t.Fatal("image rows survived a window with no room for them")
	}
}
