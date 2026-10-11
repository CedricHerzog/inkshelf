package fonts

import (
	"fmt"

	imagefont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// File is a TrueType or OpenType font file parsed once, to open at many sizes: a book's
// embedded font, say, which its pages use at several sizes. It is safe for concurrent use; the
// Fonts it opens are not.
type File struct {
	name string
	// data is the file's bytes. The shaper parses them again, its own way (see textFont).
	data   []byte
	parsed *opentype.Font
	// shaping is the file parsed for the shaper, on first use.
	shaping shapingFont
}

// Parse parses the bytes of a TrueType or OpenType font file, named name in errors and in
// Font.Name. The data must not change afterwards: the File keeps it.
func Parse(name string, data []byte) (*File, error) {
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &File{name: name, data: data, parsed: parsed}, nil
}

// Open returns the file's font at size pixels high. Unlike the package's Open, which finds
// one of the reader's own fonts by family and style, it opens this file.
func (file *File) Open(size int) (*Font, error) {
	name := file.name
	// Sizes are in pixels, and at 72 DPI a point is a pixel.
	face, err := opentype.NewFace(file.parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: imagefont.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	capHeight := face.Metrics().CapHeight.Round()
	if bounds, _, ok := face.GlyphBounds('H'); ok {
		// The hinted outline is what is drawn: measure that.
		capHeight = -bounds.Min.Y.Round()
	}
	// The baseline is the font's ascent below the top of the line, as libinkview (through
	// FreeType) placed it.
	return &Font{
		Name:      name,
		face:      face,
		ascent:    face.Metrics().Ascent.Ceil(),
		capHeight: capHeight,
		size:      size,
		glyphs:    map[rune]glyph{},
		advances:  map[rune]advance{},
		kerns:     map[[2]rune]fixed.Int26_6{},
		file:      file,
		shaped:    map[shapedKey][]Glyph{},
		masks:     map[sfnt.GlyphIndex]glyph{},
	}, nil
}
