package widgets

import (
	"image"
	"image/color"
	"log"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// Font is an interface font opened at a size, in pixels.
type Font struct {
	*fonts.Font
	Size int
	bold bool
	// labels sets the text that the font cannot draw a character after another: a title in
	// Arabic, which is joined and right to left, or in Japanese, which the font lacks. Nil in a
	// Font made by hand, which draws all text a character after another.
	labels *layout.Faces
}

// Width returns the width of label in the font, in pixels, set as Label sets it if it needs to be.
func (font Font) Width(label string) int {
	if font.labels == nil || layout.PlainLabel(font.Font, label) {
		return font.Font.Width(label)
	}
	return font.label(label).Width
}

// DrawTo draws label into dst in gray, the top-left corner of its line at topLeft (its baseline
// the font's ascent below), set as Label sets it if it needs to be.
func (font Font) DrawTo(dst *image.Gray, topLeft image.Point, label string, gray color.Gray) {
	if font.labels == nil || layout.PlainLabel(font.Font, label) {
		font.Font.DrawTo(dst, topLeft, label, gray)
		return
	}
	if err := font.labels.DrawLabel(dst, font.label(label), topLeft, gray); err != nil {
		log.Printf("drawing %q: %v", label, err)
	}
}

// label returns label set in the font's family, weight and size.
func (font Font) label(label string) layout.Label {
	style := document.Style(0)
	if font.bold {
		style = document.Bold
	}
	return font.labels.Label(label, "sans-serif", style, font.Size)
}

// LineHeight is the distance between two lines of text in the font.
func (font Font) LineHeight() int { return font.Size * 5 / 4 }

// Fonts are the three fonts of the interface. The app opens them once, when it starts, and
// closes them when it stops.
type Fonts struct {
	Small Font // secondary text and notices
	Body  Font // status lines and page numbers
	Bold  Font // titles and buttons
}

// OpenFonts opens the interface fonts at their sizes on this screen. A font that cannot be
// opened is logged and left nil: the app then draws no text in it rather than stopping.
func OpenFonts() Fonts {
	labels := layout.NewFaces(fonts.SansSerif)
	open := func(size int, bold bool) Font {
		size = Px(size)
		face, err := fonts.OpenInterfaceFont(size, bold)
		if err != nil {
			log.Printf("cannot open font: %v", err)
			return Font{Size: size}
		}
		return Font{Font: face, Size: size, bold: bold, labels: labels}
	}
	return Fonts{
		Small: open(32, false),
		Body:  open(40, false),
		Bold:  open(44, true),
	}
}

// Close closes the fonts.
func (set Fonts) Close() {
	for _, font := range []Font{set.Small, set.Body, set.Bold} {
		// The fonts share their labels' faces: closed once.
		if font.labels != nil {
			font.labels.Close()
			break
		}
	}
	for _, font := range []Font{set.Small, set.Body, set.Bold} {
		if err := font.Close(); err != nil {
			log.Printf("closing font: %v", err)
		}
	}
}
