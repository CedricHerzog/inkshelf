package layout

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// Page is a page laid out: everything on it at its place, in pixels from the top-left corner of
// the screen, ready to be drawn. Its offsets tie it to the document, so that the reader can find
// the page of a reading position again after a new layout.
type Page struct {
	// Start and End are the offsets of the first rune shown and of the rune past the last.
	Start, End int
	Lines      []Line
	Images     []PlacedImage // image blocks and inline images
	Rules      []image.Rectangle
	Strokes    []Stroke // underlines and strike-throughs
	Links      []PlacedLink
	Boxes      []PlacedBorderBox // borders and backgrounds
}

// Stroke is an underline or a strike-through, in the gray of its text.
type Stroke struct {
	image.Rectangle
	Gray uint8
}

// PlacedImage is an image of the document at its place on a page. A formula is an image too,
// whose Path is a mathml.Path: it is drawn with the page's fonts at MathSize pixels.
type PlacedImage struct {
	Path     string
	Rect     image.Rectangle
	MathSize int
}

// PlacedLink is the area of a link's text on a line, which the reader turns into a tap target.
type PlacedLink struct {
	Rect   image.Rectangle
	Target string
}

// PageAt returns the index of the page that shows offset: the last page starting at or before it.
func PageAt(pages []Page, offset int) int {
	index := 0
	for candidate, page := range pages {
		if page.Start > offset {
			break
		}
		index = candidate
	}
	return index
}

// PlacedWord is text placed on a line: X from the left of the page, and Top the top of its font's
// line, which puts its baseline on the line's. Its text is in logical order, as in the document,
// even in right-to-left text: shaping draws it in its direction.
type PlacedWord struct {
	X, Top  int
	Text    string
	Style   document.Style
	Gray    uint8
	Size    int // the font size, before Small shrinks it
	Family  string
	Spacing int // the letter spacing, in pixels
	// Shaping, if not nil, draws the word as its fonts shape it.
	Shaping *Shaping
}

// Shaping is how to draw text that the font shapes (fonts.Font.Shape) rather than character by
// character: Arabic letters join, Indic vowels move. It is drawn in its direction, from the right
// if RightToLeft, and in its language (a BCP 47 tag, "" if unknown), which picks some letter
// forms.
type Shaping struct {
	RightToLeft bool
	Language    string
}

func (word PlacedWord) format() textFormat {
	return textFormat{family: word.Family, style: word.Style, size: word.Size, spacing: word.Spacing, shaping: word.Shaping}
}

// Line is a line of text placed on a page.
type Line struct {
	Words      []PlacedWord
	Start, End int // the offsets of its first rune and past its last
}

// PlacedBorderBox is the part of a border box on a page: its border's outer edges, its
// background, and the sides of its border drawn there, their widths in pixels. A box split across
// pages has its top side on its first page only and its bottom side on its last. Depth is how
// many boxes are around it: those are drawn first, under it.
type PlacedBorderBox struct {
	Rect                     image.Rectangle
	Top, Right, Bottom, Left document.Border
	Background               uint8
	Filled                   bool
	Depth                    int
}
