package editor

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

// MaxImageCells caps an image's rows and columns: kitty placeholders address each with a fixed diacritic table.
const MaxImageCells = 297

type ImagePlacement struct {
	ID         int
	Cols, Rows int
}

// SetImages sets the placements to draw, keyed by image target as written in the note.
func (e *Editor) SetImages(images map[string]ImagePlacement) { e.images = images }

// ImageTargets returns the target of every image-only line, in line order with duplicates kept.
func (e *Editor) ImageTargets() []string {
	var targets []string
	for i := range e.buf.LineCount() {
		if target, ok := imageLineTarget(e.buf.runes(i)); ok {
			targets = append(targets, target)
		}
	}
	return targets
}

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

type imageRow struct {
	id, row, cols int
}

func (r imageRow) ok() bool { return r.id != 0 }

// Kitty reads the image id from the foreground colour and the cell's row and column from the two diacritics.
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
