package layout

import (
	"image"
	"image/color"
	"log"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// smallScale is the size of Small text, in percent of the text around it.
const smallScale = 80

// smallCapsScale is the size of the small capitals of SmallCaps text, in percent of its size.
const smallCapsScale = 75

// Faces are the fonts of a book, opened when layout or drawing first asks for them: the reader's
// typeface, the generic families and the fonts the book embeds. Opening a font at a size is
// costly, so each stays open until Close. Faces are not safe for concurrent use: one layout or
// one drawing at a time. The elements that workers prepare for a page (the text of its SVGs, its
// formulas) are drawn with fonts of their own, which their lock guards.
type Faces struct {
	typeface fonts.Family
	// embedded are the fonts of the documents laid out so far, by lowercase family name.
	embedded map[string]map[fonts.Style]*fonts.File
	fonts    map[faceKey]*fonts.Font
	// fallbacks are the fonts that draw the characters a font lacks, by style and size (keys have
	// no family), in the order to try them.
	fallbacks map[faceKey][]*fonts.Font
	// elements are the faces of the page's elements. A worker may still be drawing an element
	// that its page gave up waiting for while the page goes on with its words: the two must not
	// share a font, whose caches are not safe for concurrent use.
	elements *elementFaces
}

// elementFaces are faces that several goroutines use, one at a time.
type elementFaces struct {
	mu    sync.Mutex
	faces *Faces
}

// with calls use with the faces, their lock held.
func (elements *elementFaces) with(use func(faces *Faces)) {
	elements.mu.Lock()
	defer elements.mu.Unlock()
	use(elements.faces)
}

// faceKey names a font of Faces: a family's style at a size.
type faceKey struct {
	family string
	style  fonts.Style
	size   int
}

// NewFaces returns the faces of a book whose text is in typeface, none opened yet.
func NewFaces(typeface fonts.Family) *Faces {
	faces := newFaces(typeface)
	faces.elements = &elementFaces{faces: newFaces(typeface)}
	return faces
}

// newFaces returns faces with no element faces.
func newFaces(typeface fonts.Family) *Faces {
	return &Faces{typeface: typeface, embedded: map[string]map[fonts.Style]*fonts.File{},
		fonts: map[faceKey]*fonts.Font{}, fallbacks: map[faceKey][]*fonts.Font{}}
}

// Font returns the book font of style at size pixels (before Small shrinks it), opening it on
// first use.
func (faces *Faces) Font(style document.Style, size int) (*fonts.Font, error) {
	return faces.FamilyFont("", style, size)
}

// FamilyFont returns the font of family (a Run's) in style at size pixels (before Small shrinks
// it), opening it on first use. A family that a document laid out embeds is used in that style,
// else in its regular one; "serif", "sans-serif" and "monospace" are the reader's fonts; any other
// family is the book font, as the reader has no other fonts.
func (faces *Faces) FamilyFont(family string, style document.Style, size int) (*fonts.Font, error) {
	if style&document.Small != 0 {
		size = size * smallScale / 100
	}
	key := faceKey{family: family, style: fontStyle(style), size: max(size, 1)}
	if opened, ok := faces.fonts[key]; ok {
		return opened, nil
	}
	opened, err := faces.open(key)
	if err != nil {
		return nil, err
	}
	faces.fonts[key] = opened
	return opened, nil
}

func (faces *Faces) open(key faceKey) (*fonts.Font, error) {
	family := strings.ToLower(key.family)
	if styles, ok := faces.embedded[family]; ok {
		data := styles[key.style]
		if data == nil {
			data = styles[fonts.Regular]
		}
		if data != nil {
			opened, err := data.Open(key.size)
			if err == nil {
				return opened, nil
			}
			log.Printf("layout: font %s: %v", key.family, err)
		}
	}
	typeface := faces.typeface
	switch family {
	case "serif":
		typeface = fonts.Serif
	case "sans-serif":
		typeface = fonts.SansSerif
	case "monospace":
		typeface = fonts.Monospace
	}
	return fonts.Open(typeface, key.style, key.size)
}

// embed adds the fonts a document embeds, but not those of a family added already: every chapter
// of a book embeds the same fonts, already open.
func (faces *Faces) embed(families map[string]map[fonts.Style]*fonts.File) {
	for family, styles := range families {
		if _, ok := faces.embedded[family]; !ok {
			faces.embedded[family] = styles
		}
	}
	if faces.elements != nil {
		faces.elements.with(func(elementFaces *Faces) { elementFaces.embed(families) })
	}
}

// Close closes the fonts opened. The elements' fonts are left to the garbage collector while a
// worker still draws with them.
func (faces *Faces) Close() {
	if elements := faces.elements; elements != nil && elements.mu.TryLock() {
		elements.faces.Close()
		elements.mu.Unlock()
	}
	for key, opened := range faces.fonts {
		opened.Close()
		delete(faces.fonts, key)
	}
	for key, fallbacks := range faces.fallbacks {
		for _, opened := range fallbacks {
			opened.Close()
		}
		delete(faces.fallbacks, key)
	}
}

// fallbacksOf returns the fonts to try for the characters a font of family (a Run's), style and
// size pixels lacks, opening them on first use: the reader's typeface, then fonts.OpenFallbacks's
// (with a serif CJK font for serif text), but not a font already in the list.
func (faces *Faces) fallbacksOf(family string, style fonts.Style, size int) ([]*fonts.Font, error) {
	serif := faces.isSerif(family)
	key := faceKey{style: style, size: size}
	if serif {
		key.family = "serif"
	}
	if fallbacks, ok := faces.fallbacks[key]; ok {
		return fallbacks, nil
	}
	typeface, err := fonts.Open(faces.typeface, style, size)
	if err != nil {
		return nil, err
	}
	others, err := fonts.OpenFallbacks(style, size, serif)
	if err != nil {
		typeface.Close()
		return nil, err
	}
	fallbacks := []*fonts.Font{typeface}
	for _, opened := range others {
		// Off the reader (in tests and the emulator), the typeface and the fallbacks are all Go's
		// fonts.
		if slices.ContainsFunc(fallbacks, func(tried *fonts.Font) bool { return tried.Name == opened.Name }) {
			opened.Close()
			continue
		}
		fallbacks = append(fallbacks, opened)
	}
	faces.fallbacks[key] = fallbacks
	return fallbacks, nil
}

// isSerif reports whether text in family (a Run's) is serif: the generic serif family, or the
// book font, an embedded one included, when the reader's typeface is serif.
func (faces *Faces) isSerif(family string) bool {
	switch strings.ToLower(family) {
	case "serif":
		return true
	case "sans-serif", "monospace":
		return false
	}
	return faces.typeface == fonts.Serif
}

// canDraw reports whether font, or one of its fallbacks, has character.
func (faces *Faces) canDraw(font *fonts.Font, character rune) bool {
	if font.Has(character) {
		return true
	}
	fallbacks, err := faces.fallbacksOf("", fonts.Regular, font.Size())
	return err == nil && slices.ContainsFunc(fallbacks, func(fallback *fonts.Font) bool { return fallback.Has(character) })
}

// textFormat is how text is drawn: its font, its letter spacing, how it is shaped, if it is,
// and how it is drawn in vertical text.
type textFormat struct {
	family   string
	style    document.Style
	size     int // before Small shrinks it
	spacing  int // pixels added after each character, but not to shaped text
	shaping  *Shaping
	vertical VerticalText
}

// width returns the width of text drawn in format. Text whose font cannot be opened measures 0:
// drawing it reports the error.
func (faces *Faces) width(format textFormat, text string) int {
	if format.vertical == Upright || format.vertical == Combined {
		return faces.verticalWidth(format, text)
	}
	width := 0
	_ = faces.forEachFontPart(format, text, func(font *fonts.Font, part string, top int) {
		if shaping := format.shaping; shaping != nil {
			width += fonts.WidthOf(font.Shape(part, shaping.RightToLeft, shaping.Language))
			return
		}
		width += font.Width(part)
		if format.spacing != 0 {
			width += format.spacing * utf8.RuneCountInString(part)
		}
	})
	return width
}

// shapedPart is a part of shaped text in one font: its glyphs, drawn top pixels lower.
type shapedPart struct {
	font   *fonts.Font
	glyphs []fonts.Glyph
	top    int
}

// draw draws text in format into dst, in gray, the top-left corner of its line at topLeft (for
// vertical text, see drawVertical). Right-to-left text in several fonts has its parts drawn from
// right to left.
func (faces *Faces) draw(dst *image.Gray, format textFormat, topLeft image.Point, text string, gray color.Gray) error {
	if format.vertical != Horizontal {
		return faces.drawVertical(dst, format, topLeft, text, gray)
	}
	if shaping := format.shaping; shaping != nil {
		var parts []shapedPart
		err := faces.forEachFontPart(format, text, func(font *fonts.Font, part string, top int) {
			parts = append(parts, shapedPart{font: font, glyphs: font.Shape(part, shaping.RightToLeft, shaping.Language), top: top})
		})
		if shaping.RightToLeft {
			slices.Reverse(parts)
		}
		x := topLeft.X
		for _, part := range parts {
			part.font.DrawGlyphs(dst, image.Pt(x, topLeft.Y+part.top), part.glyphs, gray)
			x += fonts.WidthOf(part.glyphs)
		}
		return err
	}
	x := topLeft.X
	return faces.forEachFontPart(format, text, func(font *fonts.Font, part string, top int) {
		font.DrawSpaced(dst, image.Pt(x, topLeft.Y+top), part, gray, format.spacing)
		x += font.Width(part) + format.spacing*utf8.RuneCountInString(part)
	})
}

// forEachFontPart calls each with the parts of text in format and their fonts, top pixels lower to
// share the baseline. Text is one part, but small capitals draw lowercase letters as capitals of
// a smaller font, and the characters the font lacks go to a fallback.
func (faces *Faces) forEachFontPart(format textFormat, text string, each func(font *fonts.Font, part string, top int)) error {
	font, err := faces.FamilyFont(format.family, format.style, format.size)
	if err != nil {
		return err
	}
	style := fontStyle(format.style)
	if format.style&document.SmallCaps == 0 || format.shaping != nil {
		return faces.forEachCoveredPart(font, format.family, style, text, 0, each)
	}
	small, err := faces.FamilyFont(format.family, format.style, format.size*smallCapsScale/100)
	if err != nil {
		return err
	}
	for text != "" {
		first, _ := utf8.DecodeRuneInString(text)
		lower := unicode.IsLower(first)
		end := strings.IndexFunc(text, func(character rune) bool { return unicode.IsLower(character) != lower })
		if end < 0 {
			end = len(text)
		}
		if lower {
			err = faces.forEachCoveredPart(small, format.family, style, strings.ToUpper(text[:end]), font.Ascent()-small.Ascent(), each)
		} else {
			err = faces.forEachCoveredPart(font, format.family, style, text[:end], 0, each)
		}
		if err != nil {
			return err
		}
		text = text[end:]
	}
	return nil
}

// forEachCoveredPart calls each with text in font, of family, top pixels lower, but for the
// characters font lacks: those go, a part per font, to the first fallback that has them (else stay
// in font, to be drawn as boxes), lowered to share font's baseline. So do the letters of a joining
// script (Arabic) that font has without the rules to join them, when a fallback has both. Text
// that font covers costs one check.
func (faces *Faces) forEachCoveredPart(font *fonts.Font, family string, style fonts.Style, text string, top int, each func(font *fonts.Font, part string, top int)) error {
	if font.Covers(text) && font.ShapesScriptsOf(text) {
		each(font, text, top)
		return nil
	}
	fallbacks, err := faces.fallbacksOf(family, style, font.Size())
	if err != nil {
		return err
	}
	current, start := font, 0 // the part from start is in current
	for index, character := range text {
		chosen := font
		if !font.Has(character) || !font.ShapesScriptOf(character) {
			found := slices.IndexFunc(fallbacks, func(fallback *fonts.Font) bool {
				return fallback.Has(character) && fallback.ShapesScriptOf(character)
			})
			if found < 0 && !font.Has(character) {
				found = slices.IndexFunc(fallbacks, func(fallback *fonts.Font) bool { return fallback.Has(character) })
			}
			if found >= 0 {
				chosen = fallbacks[found]
			}
		}
		if chosen != current {
			if index > start {
				each(current, text[start:index], top+font.Ascent()-current.Ascent())
			}
			current, start = chosen, index
		}
	}
	each(current, text[start:], top+font.Ascent()-current.Ascent())
	return nil
}

// fontStyle returns the font style that style's bits ask for.
func fontStyle(style document.Style) fonts.Style {
	switch {
	case style&document.Bold != 0 && style&document.Italic != 0:
		return fonts.BoldItalic
	case style&document.Bold != 0:
		return fonts.Bold
	case style&document.Italic != 0:
		return fonts.Italic
	}
	return fonts.Regular
}

// MissingCharacters returns the characters of page's words, each once, that neither their font
// nor any of its fallbacks has: drawn as the font's missing-glyph box. Spaces, controls,
// combining marks and format characters (joiners, direction marks) are not counted: they draw
// nothing of their own, or with the letter they belong to.
func (faces *Faces) MissingCharacters(page Page) []rune {
	seen := map[rune]bool{}
	var missing []rune
	for _, line := range page.Lines {
		for _, word := range line.Words {
			if word.Mark != NoMark {
				continue
			}
			format := word.format()
			font, err := faces.FamilyFont(format.family, format.style, format.size)
			if err != nil {
				continue
			}
			for _, character := range word.Text {
				if seen[character] || unicode.IsSpace(character) || unicode.IsControl(character) ||
					unicode.In(character, unicode.Mn, unicode.Me, unicode.Cf) {
					continue
				}
				seen[character] = true
				if !faces.canDrawIn(font, format, character) {
					missing = append(missing, character)
				}
			}
		}
	}
	return missing
}

// canDrawIn reports whether font, or a fallback of its format's family and style, has character.
func (faces *Faces) canDrawIn(font *fonts.Font, format textFormat, character rune) bool {
	if font.Has(character) {
		return true
	}
	fallbacks, err := faces.fallbacksOf(format.family, fontStyle(format.style), font.Size())
	return err == nil && slices.ContainsFunc(fallbacks, func(fallback *fonts.Font) bool { return fallback.Has(character) })
}
