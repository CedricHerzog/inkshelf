package layout

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// mathFamily is the family formulas are drawn in: serif, as in print.
const mathFamily = "serif"

// mathFonts are the fonts of Faces that formulas are laid out and drawn in, with their fallbacks
// for the symbols a font lacks. It is what package mathml needs of fonts, so that mathml knows
// nothing of layout.
type mathFonts struct{ faces *Faces }

// format returns the text format of a mathematical variant (bold, italic) at size.
func (formulaFonts mathFonts) format(variant mathml.Variant, size int) textFormat {
	var style document.Style
	if variant&mathml.Bold != 0 {
		style |= document.Bold
	}
	if variant&mathml.Italic != 0 {
		style |= document.Italic
	}
	return textFormat{family: mathFamily, style: style, size: size}
}

// Measure returns the width of text and the rectangle its ink covers from the origin on its
// baseline.
func (formulaFonts mathFonts) Measure(text string, variant mathml.Variant, size int) (width int, ink image.Rectangle) {
	format := formulaFonts.format(variant, size)
	// The parts of text in fallback fonts share its baseline.
	_ = formulaFonts.faces.forEachFontPart(format, text, func(font *fonts.Font, part string, top int) {
		ink = ink.Union(font.Ink(part).Add(image.Pt(width, 0)))
		width += font.Width(part)
	})
	return width, ink
}

// Draw draws text into dst, its baseline's left end at origin.
func (formulaFonts mathFonts) Draw(dst *image.Gray, text string, variant mathml.Variant, size int, origin image.Point) {
	format := formulaFonts.format(variant, size)
	font, err := formulaFonts.faces.FamilyFont(format.family, format.style, size)
	if err != nil {
		return
	}
	_ = formulaFonts.faces.draw(dst, format, origin.Sub(image.Pt(0, font.Ascent())), text, black)
}

// formula returns the formula of an image path (a mathml.Path) laid out at size pixels, shrunk to
// fit in limit if it is larger, and the size it is laid out at. A formula is laid out again, from
// its MathML, each time it is placed or drawn: nothing ties a page to its layout.
func (faces *Faces) formula(imagePath string, size int, limit image.Point) (*mathml.Formula, int, error) {
	root, err := mathml.FromPath(imagePath)
	if err != nil {
		return nil, 0, err
	}
	formulaFonts := mathFonts{faces: faces}
	formula := mathml.Layout(root, formulaFonts, size)
	// A formula's size is about proportional to its font's: shrink the font in proportion, and
	// again if that was not quite enough.
	for range 3 {
		height := formula.Ascent + formula.Descent
		if formula.Width <= limit.X && height <= limit.Y || size == 1 {
			break
		}
		size = max(min(size*limit.X/max(formula.Width, 1), size*limit.Y/max(height, 1), size-1), 1)
		formula = mathml.Layout(root, formulaFonts, size)
	}
	return formula, size, nil
}
