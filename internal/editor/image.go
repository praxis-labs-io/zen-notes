package editor

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

// MaxImageCells is the largest placeholder grid the protocol can address: a
// cell says where it sits with a diacritic, and there are only so many.
const MaxImageCells = 297

// ImagePlacement is where an image has been given room, in cells. The app
// resolves and measures the file; the editor only reserves the rows.
type ImagePlacement struct {
	ID         int
	Cols, Rows int
}

// SetImages records the placements the app resolved, keyed by the target as
// it is written in the note.
func (e *Editor) SetImages(images map[string]ImagePlacement) { e.images = images }

// ImageTargets lists the image references in the buffer, in line order and
// with duplicates left in, so the app can resolve each one.
func (e *Editor) ImageTargets() []string {
	var targets []string
	for i := range e.buf.LineCount() {
		if target, ok := imageLineTarget(e.buf.runes(i)); ok {
			targets = append(targets, target)
		}
	}
	return targets
}

// imagePlacement is the placement to draw under a line, if any.
func (e *Editor) imagePlacement(line int) (ImagePlacement, bool) {
	if len(e.images) == 0 {
		return ImagePlacement{}, false
	}
	target, ok := imageLineTarget(e.buf.runes(line))
	if !ok {
		return ImagePlacement{}, false
	}
	placement, ok := e.images[target]
	return placement, ok
}

// imageLineTarget returns the target of a line holding nothing but an image
// reference. An image draws on rows of its own, so a reference sharing a line
// with other text stays plain markdown.
func imageLineTarget(runes []rune) (string, bool) {
	from := leadingSpaceEnd(runes)
	to := len(runes)
	for to > from && isLinkSpace(runes[to-1]) {
		to--
	}
	if to-from < 2 || runes[from] != '!' || runes[from+1] != '[' {
		return "", false
	}
	link, ok := parseInlineLink(runes[:to], from+1)
	if !ok || link.to != to || link.target == "" {
		return "", false
	}
	return link.target, true
}

// imageRow is one row of an image's placeholder grid. A zero id means the
// row is ordinary text.
type imageRow struct {
	id, row, cols int
}

func (r imageRow) ok() bool { return r.id != 0 }

// render draws one row of Unicode placeholder cells. The terminal composites
// the image over them, reading the image id from the foreground colour and
// the cell's position from the two diacritics. Every cell says where it sits,
// so a partial redraw stays correct.
func (r imageRow) render(width int) (string, int) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(r.id)))
	cols := min(r.cols, width, MaxImageCells)
	var sb strings.Builder
	for col := range cols {
		sb.WriteString(style.Render(string([]rune{
			kitty.Placeholder, kitty.Diacritic(r.row), kitty.Diacritic(col),
		})))
	}
	return sb.String(), cols
}
