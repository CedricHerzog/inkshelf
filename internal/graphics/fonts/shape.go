package fonts

import (
	"bytes"
	"sync"

	"github.com/go-text/typesetting/di"
	textfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Glyph is a glyph of shaped text, in pixels: its index in the font (shaped text is made of
// glyphs, not characters: a ligature is one glyph for several characters, and an Arabic
// letter has a glyph per position in its word), how far the dot moves after it, and where it
// is drawn from the dot (Y downward).
type Glyph struct {
	Index   sfnt.GlyphIndex
	Advance int
	X, Y    int
}

// shapedKey is text shaped in a direction and a language.
type shapedKey struct {
	text        string
	rightToLeft bool
	language    string
}

// maxShaped is how many shaped texts a Font keeps. Past that it forgets them all, which is
// simpler than keeping the most used and costs little: a page is shaped again at most.
const maxShaped = 4096

// shapingFont is a font file parsed for go-text's shaper, which reads fonts with a parser of
// its own. It is parsed once per File, on first use, as the File's Fonts share it.
type shapingFont struct {
	once   sync.Once
	parsed *textfont.Font
	err    error
}

// textFont returns the font parsed for the shaper.
func (file *File) textFont() (*textfont.Font, error) {
	file.shaping.once.Do(func() {
		var face *textfont.Face
		face, file.shaping.err = textfont.ParseTTF(bytes.NewReader(file.data))
		if file.shaping.err == nil {
			file.shaping.parsed = face.Font
		}
	})
	return file.shaping.parsed, file.shaping.err
}

// Shape returns the glyphs of text from left to right, as HarfBuzz's rules (go-text's port)
// shape it: joined and positioned as its script wants (Arabic, Hebrew points, Indic scripts),
// right-to-left text reversed and its brackets mirrored. languageTag is a BCP 47 tag, or "". The
// glyphs of a character the font lacks are its .notdef. A font that the shaper cannot read
// gives each character its glyph, unjoined.
func (font *Font) Shape(text string, rightToLeft bool, languageTag string) []Glyph {
	key := shapedKey{text: text, rightToLeft: rightToLeft, language: languageTag}
	if cached, ok := font.shaped[key]; ok {
		return cached
	}
	glyphs := font.shape(text, rightToLeft, languageTag)
	if len(font.shaped) >= maxShaped {
		clear(font.shaped)
	}
	font.shaped[key] = glyphs
	return glyphs
}

// shape shapes text, as Shape does, without the cache.
func (font *Font) shape(text string, rightToLeft bool, tag string) []Glyph {
	runes := []rune(text)
	if font.textFace == nil {
		parsed, err := font.file.textFont()
		if err != nil {
			return font.unshaped(runes, rightToLeft)
		}
		font.textFace = textfont.NewFace(parsed)
		font.shaper = &shaping.HarfbuzzShaper{}
	}
	direction := di.DirectionLTR
	if rightToLeft {
		direction = di.DirectionRTL
	}
	output := font.shaper.Shape(shaping.Input{
		Text: runes, RunEnd: len(runes), Direction: direction, Face: font.textFace,
		Size: fixed.I(font.size), Script: scriptOf(runes), Language: language.NewLanguage(tag),
	})
	glyphs := make([]Glyph, len(output.Glyphs))
	for index, shaped := range output.Glyphs {
		glyphs[index] = Glyph{Index: sfnt.GlyphIndex(shaped.GlyphID), Advance: shaped.Advance.Round(),
			X: shaped.XOffset.Round(), Y: -shaped.YOffset.Round()}
	}
	return glyphs
}

// unshaped returns a glyph per character of runes, from left to right.
func (font *Font) unshaped(runes []rune, rightToLeft bool) []Glyph {
	glyphs := make([]Glyph, len(runes))
	for index, character := range runes {
		glyphIndex, _ := font.file.parsed.GlyphIndex(&font.buffer, character)
		position := index
		if rightToLeft {
			position = len(runes) - 1 - index
		}
		glyphs[position] = Glyph{Index: glyphIndex, Advance: font.Advance(character)}
	}
	return glyphs
}

// scriptOf returns the script of the first character of runes that has one of its own, Latin
// if none has.
func scriptOf(runes []rune) language.Script {
	for _, character := range runes {
		if script := language.LookupScript(character); script.Strong() && script != language.Unknown {
			return script
		}
	}
	return language.Latin
}

// WidthOf returns the width of glyphs, in pixels.
func WidthOf(glyphs []Glyph) int {
	width := 0
	for _, shaped := range glyphs {
		width += shaped.Advance
	}
	return width
}
