package fonts

import (
	"image"

	textfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	imagefont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Font is a font at one size, measured and drawn antialiased in gray. It caches what it
// measures and rasterizes, which is why it is not safe for concurrent use: give each
// goroutine its own.
type Font struct {
	// Name is the font file, "Go" for Go's fonts, or the name given to Parse.
	Name string
	face imagefont.Face
	// ascent is the distance from the top of a line to the baseline, in pixels.
	ascent int
	// capHeight is the height of the capitals above the baseline, in pixels.
	capHeight int
	// size is the size the font was opened at, in pixels.
	size int
	// glyphs caches the glyphs rasterized so far by character, advances and kerns the
	// measures taken so far.
	glyphs   map[rune]glyph
	advances map[rune]advance
	kerns    map[[2]rune]fixed.Int26_6
	// file is what the font was opened from. Shaped text needs it, as shaping and drawing
	// shaped glyphs work on the font's tables and outlines directly: textFace and shaper,
	// made on first use, shape text, shaped caches what they shaped, and masks caches the
	// glyphs drawn by glyph index, rasterized through buffer and rasterizer.
	file       *File
	textFace   *textfont.Face
	shaper     *shaping.HarfbuzzShaper
	shaped     map[shapedKey][]Glyph
	masks      map[sfnt.GlyphIndex]glyph
	buffer     sfnt.Buffer
	rasterizer *vector.Rasterizer
}

// advance is how far a character moves the dot, and whether the font has it.
type advance struct {
	width   fixed.Int26_6
	missing bool
}

// CenterTop returns the top of a line of text whose capitals are centered vertically in rect.
func (font *Font) CenterTop(rect image.Rectangle) int {
	return rect.Min.Y + (rect.Dy()+font.capHeight)/2 - font.ascent
}

// Width returns the width of text in pixels, as imagefont.MeasureString measures it, from the
// cached advances and kerns.
func (font *Font) Width(text string) int {
	width, _ := font.Measure(text)
	return width
}

// Measure returns the width of text, as Width does, and whether the font has all its
// characters, as Covers does, in one pass.
func (font *Font) Measure(text string) (width int, covers bool) {
	var total fixed.Int26_6
	covers = true
	previous := rune(-1)
	for _, character := range text {
		if previous >= 0 {
			total += font.kern(previous, character)
		}
		measured := font.advance(character)
		total += measured.width
		covers = covers && !measured.missing
		previous = character
	}
	return total.Ceil(), covers
}

// Size returns the size the font was opened at, in pixels.
func (font *Font) Size() int { return font.size }

// Ascent returns the distance from the top of a line to its baseline, in pixels.
func (font *Font) Ascent() int { return font.ascent }

// Descent returns the distance from the baseline to the bottom of the glyphs' descenders, in
// pixels.
func (font *Font) Descent() int { return font.face.Metrics().Descent.Ceil() }

// Ink returns the rectangle the ink of text covers, drawn from the origin on its baseline, as
// DrawTo places glyphs: empty for text without ink. The glyphs are rasterized, and cached for
// drawing.
func (font *Font) Ink(text string) image.Rectangle {
	var ink image.Rectangle
	var dotX fixed.Int26_6
	previous := rune(-1)
	for _, character := range text {
		if previous >= 0 {
			dotX += font.kern(previous, character)
		}
		rasterized := font.glyph(character)
		if rasterized.mask != nil {
			ink = ink.Union(rasterized.mask.Rect.Add(rasterized.offset).Add(image.Pt(dotX.Round(), 0)))
		}
		dotX += rasterized.advance
		previous = character
	}
	return ink
}

// Advance returns how far character moves the dot, in pixels: a whole number with full
// hinting. A character missing from the font advances as its .notdef glyph.
func (font *Font) Advance(character rune) int { return font.advance(character).width.Round() }

// Kern returns the kerning between previous and next, in pixels.
func (font *Font) Kern(previous, next rune) int { return font.kern(previous, next).Round() }

// Has reports whether the font has a glyph for character.
func (font *Font) Has(character rune) bool { return !font.advance(character).missing }

// Covers reports whether the font has a glyph for every character of text.
func (font *Font) Covers(text string) bool {
	for _, character := range text {
		if font.advance(character).missing {
			return false
		}
	}
	return true
}

// advance returns character's advance, from the cache.
func (font *Font) advance(character rune) advance {
	if cached, ok := font.advances[character]; ok {
		return cached
	}
	// A missing character advances as .notdef, as imagefont.MeasureString counts it.
	width, ok := font.face.GlyphAdvance(character)
	measured := advance{width: width, missing: !ok}
	font.advances[character] = measured
	return measured
}

// kern returns the kerning between previous and next, from the cache.
func (font *Font) kern(previous, next rune) fixed.Int26_6 {
	pair := [2]rune{previous, next}
	if cached, ok := font.kerns[pair]; ok {
		return cached
	}
	kerning := font.face.Kern(previous, next)
	font.kerns[pair] = kerning
	return kerning
}

// Close releases the font. It accepts a nil font.
func (font *Font) Close() error {
	if font == nil {
		return nil
	}
	return font.face.Close()
}
