package widgets

import (
	"log"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
)

// Font is an interface font opened at a size, in pixels.
type Font struct {
	*fonts.Font
	Size int
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
	open := func(size int, bold bool) Font {
		size = Px(size)
		face, err := fonts.OpenInterfaceFont(size, bold)
		if err != nil {
			log.Printf("cannot open font: %v", err)
		}
		return Font{face, size}
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
		if err := font.Close(); err != nil {
			log.Printf("closing font: %v", err)
		}
	}
}
