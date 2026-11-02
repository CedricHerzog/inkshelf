package layout

import (
	"image"
	"image/color"

	unicodebidi "golang.org/x/text/unicode/bidi"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// Label is a line of plain text set as a line of a book is, for the interface: a book's title in
// a list, say, which may be Arabic or Japanese. Its characters go to fonts that have them, its
// letters are joined, and its right-to-left runs are in visual order, the paragraph taking the
// direction of its first letter, as dir="auto" does.
type Label struct {
	// Width is the label's width, in pixels.
	Width int
	runs  []labelRun // from left to right
}

// labelRun is a run of a label at one bidirectional level, in its format.
type labelRun struct {
	text   string
	level  int8
	format textFormat
	width  int
}

// firstSetCharacter is the first character that PlainLabel looks at in its font: before it are
// the Latin, Greek and Cyrillic letters, none of which is shaped or right to left.
const firstSetCharacter = 0x0590

// PlainLabel reports whether text draws as it is in font, a character after another, without a
// Label: its characters are left to right, need no shaping, and font has them all.
func PlainLabel(font *fonts.Font, text string) bool {
	for _, character := range text {
		if !font.Has(character) {
			return false
		}
		if character < firstSetCharacter {
			continue
		}
		properties, _ := unicodebidi.LookupRune(character)
		if class := properties.Class(); class == unicodebidi.R || class == unicodebidi.AL || document.NeedsShaping(character) {
			return false
		}
	}
	return true
}

// Label sets text in a font of family, style and size pixels: the sans-serif font of the
// interface, say.
func (faces *Faces) Label(text, family string, style document.Style, size int) Label {
	runes := []rune(text)
	levels := segmentLevels(runes, firstStrongIsRightToLeft(runes))
	var label Label
	for start := 0; start < len(runes); {
		end := start + 1
		for end < len(runes) && levels[end] == levels[start] {
			end++
		}
		run := labelRun{text: string(runes[start:end]), level: levels[start],
			format: textFormat{family: family, style: style, size: size}}
		if rightToLeft := run.level%2 == 1; rightToLeft || containsShaped(run.text) {
			run.format.shaping = &Shaping{RightToLeft: rightToLeft}
		}
		run.width = faces.width(run.format, run.text)
		label.Width += run.width
		label.runs = append(label.runs, run)
		start = end
	}
	reorder(label.runs, func(run labelRun) int8 { return run.level })
	return label
}

// DrawLabel draws label into dst in gray, the top-left corner of its line at topLeft: its
// baseline is its font's ascent below.
func (faces *Faces) DrawLabel(dst *image.Gray, label Label, topLeft image.Point, gray color.Gray) error {
	x := topLeft.X
	for _, run := range label.runs {
		if err := faces.draw(dst, run.format, image.Pt(x, topLeft.Y), run.text, gray); err != nil {
			return err
		}
		x += run.width
	}
	return nil
}
