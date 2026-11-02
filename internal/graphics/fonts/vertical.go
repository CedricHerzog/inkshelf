package fonts

import (
	"image"
	"image/color"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Vertical text, as Chinese and Japanese books set it, goes down in columns. Its characters
// are drawn in one of two ways. Ideographs and kana stay upright, one under the other, each
// centred on the column: ShapeVertical and DrawGlyphsVertical. Latin words and other text
// written sideways are drawn as on a horizontal line turned a quarter turn clockwise, the tops
// of the letters to the right: DrawTurned and DrawGlyphsTurned. The horizontal drawing of
// draw.go is not touched by either.

// ShapeVertical returns the glyphs of text upright from top to bottom, shaped in a language (a
// BCP 47 tag, or ""): shaping top to bottom applies the font's vertical forms (its vert and
// vrt2 features turn brackets and the long vowel mark, and move small kana) and its vertical
// advances. Each glyph's X and Y place it from the dot, which is on the column's centre line at
// the top of the glyph; its Advance is how far the dot moves down.
func (font *Font) ShapeVertical(text, languageTag string) []Glyph {
	key := shapedKey{text: text, language: languageTag}
	if cached, ok := font.shapedVertical[key]; ok {
		return cached
	}
	if font.shapedVertical == nil || len(font.shapedVertical) >= maxShaped {
		font.shapedVertical = map[shapedKey][]Glyph{}
	}
	glyphs := font.shapeVertical(text, languageTag)
	font.shapedVertical[key] = glyphs
	return glyphs
}

// shapeVertical shapes text, as ShapeVertical does, without the cache.
func (font *Font) shapeVertical(text, tag string) []Glyph {
	runes := []rune(text)
	if !font.canShape() {
		return font.unshapedVertical(runes)
	}
	output := font.shaper.Shape(shaping.Input{
		Text: runes, RunEnd: len(runes), Direction: di.DirectionTTB, Face: font.textFace,
		Size: fixed.I(font.size), Script: scriptOf(runes), Language: language.NewLanguage(tag),
	})
	glyphs := make([]Glyph, len(output.Glyphs))
	for index, shaped := range output.Glyphs {
		// The shaper's Y goes up, and its vertical advances are negative.
		glyphs[index] = Glyph{Index: sfnt.GlyphIndex(shaped.GlyphID), Advance: -shaped.Advance.Round(),
			X: shaped.XOffset.Round(), Y: -shaped.YOffset.Round()}
	}
	return glyphs
}

// unshapedVertical returns a glyph per character of runes, each an em high and centred on the
// column.
func (font *Font) unshapedVertical(runes []rune) []Glyph {
	glyphs := make([]Glyph, len(runes))
	for index, character := range runes {
		glyphIndex, _ := font.file.parsed.GlyphIndex(&font.buffer, character)
		glyphs[index] = Glyph{Index: glyphIndex, Advance: font.size, X: -font.Advance(character) / 2, Y: font.ascent}
	}
	return glyphs
}

// DrawGlyphsVertical draws glyphs (from ShapeVertical) into dst in gray, going down from top: a
// point on the column's centre line, at the top of the first glyph.
func (font *Font) DrawGlyphsVertical(dst *image.Gray, top image.Point, glyphs []Glyph, gray color.Gray) {
	dot := top
	for _, shaped := range glyphs {
		rasterized := font.glyphMask(shaped.Index)
		if rasterized.mask != nil {
			blend(dst, rasterized.mask, dot.Add(image.Pt(shaped.X, shaped.Y)).Add(rasterized.offset), gray.Y)
		}
		dot.Y += shaped.Advance
	}
}

// DrawTurned draws text as DrawSpaced does, but turned a quarter turn clockwise: its baseline
// goes down from origin, the dot of its first character, and the tops of its letters face
// right.
func (font *Font) DrawTurned(dst *image.Gray, origin image.Point, text string, gray color.Gray, spacing int) {
	dotY := fixed.I(origin.Y)
	previous := rune(-1)
	for _, character := range text {
		if previous >= 0 {
			dotY += font.kern(previous, character)
		}
		rasterized := font.turnedGlyph(character)
		if rasterized.mask != nil {
			blend(dst, rasterized.mask, image.Pt(origin.X, dotY.Round()).Add(rasterized.offset), gray.Y)
		}
		dotY += rasterized.advance + fixed.I(spacing)
		previous = character
	}
}

// DrawGlyphsTurned draws glyphs (from Shape) as DrawGlyphs does, but turned a quarter turn
// clockwise, as DrawTurned turns text: their baseline goes down from origin.
func (font *Font) DrawGlyphsTurned(dst *image.Gray, origin image.Point, glyphs []Glyph, gray color.Gray) {
	dot := origin
	for _, shaped := range glyphs {
		rasterized := font.turnedMask(shaped.Index)
		if rasterized.mask != nil {
			// The glyph's offset turns with it: along the baseline becomes down, down becomes left.
			blend(dst, rasterized.mask, dot.Add(image.Pt(-shaped.Y, shaped.X)).Add(rasterized.offset), gray.Y)
		}
		dot.Y += shaped.Advance
	}
}

// turnedGlyph returns character rasterized and turned, from the cache.
func (font *Font) turnedGlyph(character rune) glyph {
	if cached, ok := font.turned[character]; ok {
		return cached
	}
	if font.turned == nil {
		font.turned = map[rune]glyph{}
	}
	turned := turn(font.glyph(character))
	font.turned[character] = turned
	return turned
}

// turnedMask returns the glyph of index rasterized and turned, from the cache.
func (font *Font) turnedMask(index sfnt.GlyphIndex) glyph {
	if cached, ok := font.turnedMasks[index]; ok {
		return cached
	}
	if font.turnedMasks == nil {
		font.turnedMasks = map[sfnt.GlyphIndex]glyph{}
	}
	turned := turn(font.glyphMask(index))
	font.turnedMasks[index] = turned
	return turned
}

// turn returns upright turned a quarter turn clockwise about its dot: a point right of the dot
// goes under it, and a point above it to its right.
func turn(upright glyph) glyph {
	turned := glyph{advance: upright.advance}
	if upright.mask == nil {
		return turned
	}
	width, height := upright.mask.Rect.Dx(), upright.mask.Rect.Dy()
	turned.mask = image.NewAlpha(image.Rect(0, 0, height, width))
	turned.offset = image.Pt(-(upright.offset.Y + height), upright.offset.X)
	for row := range width {
		for column := range height {
			turned.mask.Pix[row*turned.mask.Stride+column] = upright.mask.Pix[(height-1-column)*upright.mask.Stride+row]
		}
	}
	return turned
}
