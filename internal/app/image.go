package app

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi/kitty"
	"github.com/praxis-labs-io/zen-notes/internal/editor"
)

// maxImageID is the highest id we hand out. Ids ride in an 8-bit foreground
// colour so a colour profile that downsamples cannot corrupt them.
const maxImageID = 255

// imageEntry is a note reference we resolved, measured and transmitted.
type imageEntry struct {
	id               int
	path             string
	size             int64
	mtime            time.Time
	maxCols, maxRows int
	editor.ImagePlacement
}

// bound reports whether the entry was fitted to the same room it is being
// asked about, so an unchanged file can skip re-reading its header.
func (e imageEntry) bound(maxCols, maxRows int) bool {
	return e.maxCols == maxCols && e.maxRows == maxRows
}

// images holds what the terminal is currently showing, keyed by the target as
// it is written in the note. Cell size stays zero until the terminal answers
// the size query, and images stay off until it does.
type images struct {
	cellW, cellH int
	entries      map[string]imageEntry
	nextID       int
}

func newImages() *images {
	return &images{entries: map[string]imageEntry{}, nextID: 1}
}

// supported reports whether the terminal told us how big a cell is, which is
// the only capability check we get and the only number the fit needs.
func (i *images) supported() bool { return i.cellW > 0 && i.cellH > 0 }

// sync brings the terminal in line with the note: it resolves every reference,
// transmits what is new or changed, forgets what the note no longer mentions,
// and hands the editor the placements to reserve rows for. It returns nil when
// nothing changed, which is the usual case.
func (m *Model) syncImages() tea.Cmd {
	if !m.images.supported() {
		return nil
	}

	maxCols, maxRows := m.imageBounds()
	if maxCols <= 0 || maxRows <= 0 {
		// No room to draw in. Stale placements would reserve rows sized for
		// a window that is gone.
		m.ed.SetImages(nil)
		return nil
	}

	var seqs []string
	placements := map[string]editor.ImagePlacement{}
	tried := map[string]bool{}

	for _, target := range m.ed.ImageTargets() {
		if tried[target] {
			continue
		}
		tried[target] = true

		entry, seq, err := m.images.resolve(target, m.store.Dir(), maxCols, maxRows)
		if err != nil {
			continue
		}
		placements[target] = entry.ImagePlacement
		if seq != "" {
			seqs = append(seqs, seq)
		}
	}

	for target, entry := range m.images.entries {
		if _, ok := placements[target]; ok {
			continue
		}
		delete(m.images.entries, target)
		seqs = append(seqs, deleteImage(entry.id))
	}

	m.ed.SetImages(placements)
	if len(seqs) == 0 {
		return nil
	}
	return tea.Raw(strings.Join(seqs, ""))
}

// imageBounds is the room an image may take: the note's text width, and half
// the window, so an image never buries the note it belongs to.
func (m *Model) imageBounds() (cols, rows int) {
	gutter := editor.GutterWidth(m.ed.Buffer().LineCount())
	return min(m.width-gutter, editor.MaxImageCells), min(m.textHeight()/2, editor.MaxImageCells)
}

// resolve returns the placement for a target, transmitting the image when it
// is new, has changed on disk, or no longer fits the space available.
func (i *images) resolve(target, dir string, maxCols, maxRows int) (imageEntry, string, error) {
	path, err := resolveImagePath(target, dir)
	if err != nil {
		return imageEntry{}, "", err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return imageEntry{}, "", fmt.Errorf("stat image: %w", err)
	}

	was, known := i.entries[target]
	unchanged := known && was.path == path &&
		was.size == stat.Size() && was.mtime.Equal(stat.ModTime()) &&
		was.bound(maxCols, maxRows)
	if unchanged {
		return was, "", nil
	}

	cols, rows, err := fitImage(path, i.cellW, i.cellH, maxCols, maxRows)
	if err != nil {
		return imageEntry{}, "", err
	}
	if known && was.Cols == cols && was.Rows == rows {
		was.maxCols, was.maxRows = maxCols, maxRows
		i.entries[target] = was
		return was, "", nil
	}

	entry := imageEntry{
		id:      i.claimID(target),
		path:    path,
		size:    stat.Size(),
		mtime:   stat.ModTime(),
		maxCols: maxCols,
		maxRows: maxRows,
		ImagePlacement: editor.ImagePlacement{
			Cols: cols,
			Rows: rows,
		},
	}
	entry.ID = entry.id

	seq, err := transmitImage(entry)
	if err != nil {
		return imageEntry{}, "", err
	}
	i.entries[target] = entry
	return entry, seq, nil
}

// claimID reuses a target's id so a resize replaces the image in place rather
// than filling the terminal's store with copies. A wrapped counter would hand
// out an id another entry still holds, and transmitting over it would show
// that entry the wrong picture.
func (i *images) claimID(target string) int {
	if was, ok := i.entries[target]; ok {
		return was.id
	}
	live := make(map[int]bool, len(i.entries))
	for _, e := range i.entries {
		live[e.id] = true
	}
	for range maxImageID {
		id := i.nextID
		i.nextID = id%maxImageID + 1
		if !live[id] {
			return id
		}
	}
	return i.nextID
}

// resolveImagePath turns a note's target into a local file. Nothing that
// names a host is accepted; notes never reach the network.
func resolveImagePath(target, dir string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("empty image target")
	}
	if u, err := url.Parse(target); err == nil && u.Scheme != "" && len(u.Scheme) > 1 {
		return "", fmt.Errorf("not a local image: %s", target)
	}
	if after, ok := strings.CutPrefix(target, "~"); ok && (after == "" || after[0] == '/') {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		target = filepath.Join(home, after)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	return filepath.Clean(target), nil
}

// fitImage scales an image into at most maxCols by maxRows cells, keeping its
// aspect ratio. Only the header is read, never the pixels.
func fitImage(path string, cellW, cellH, maxCols, maxRows int) (cols, rows int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only, and the config is already out

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("decode image header: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("image has no size")
	}

	scale := min(
		1,
		float64(maxCols*cellW)/float64(cfg.Width),
		float64(maxRows*cellH)/float64(cfg.Height),
	)
	cols = ceilDiv(int(float64(cfg.Width)*scale), cellW)
	rows = ceilDiv(int(float64(cfg.Height)*scale), cellH)
	return min(max(cols, 1), maxCols), min(max(rows, 1), maxRows), nil
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// transmitImage builds the sequence that sends the image and creates its
// virtual placement. A PNG is sent as a path, so its bytes never cross the
// tty; anything else is re-encoded, because PNG is the only format the
// protocol compresses.
func transmitImage(e imageEntry) (string, error) {
	o := &kitty.Options{
		Action:           kitty.TransmitAndPut,
		Format:           kitty.PNG,
		ID:               e.id,
		VirtualPlacement: true,
		Columns:          e.Cols,
		Rows:             e.Rows,
		Quiet:            2,
	}

	var img image.Image
	if isPNG(e.path) {
		o.Transmission, o.File = kitty.File, e.path
	} else {
		decoded, err := decodeImage(e.path)
		if err != nil {
			return "", err
		}
		o.Transmission, o.Chunk, img = kitty.Direct, true, decoded
	}

	var sb strings.Builder
	if err := kitty.EncodeGraphics(&sb, img, o); err != nil {
		return "", fmt.Errorf("encode image: %w", err)
	}
	return sb.String(), nil
}

func isPNG(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only probe
	_, err = png.DecodeConfig(f)
	return err == nil
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// deleteImage drops an image from the terminal's store, so ids do not leak
// when a note stops mentioning a file.
func deleteImage(id int) string {
	var sb strings.Builder
	o := &kitty.Options{
		Action: kitty.Delete, Delete: kitty.DeleteID, DeleteResources: true, ID: id, Quiet: 2,
	}
	if err := kitty.EncodeGraphics(&sb, nil, o); err != nil {
		return ""
	}
	return sb.String()
}
