package layout

import (
	"image"
	"image/color"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
)

// SVGText returns what draws the text of an SVG with the faces: in the first of its families
// that a document embeds or that is generic, else in the book font, with the fallbacks of a
// page's text. Nil faces return nil, which leaves an SVG's text out. It draws with the element
// faces, so that an SVG may be drawn on a worker while the page draws its words.
func (faces *Faces) SVGText() svgimage.TextDrawer {
	if faces == nil {
		return nil
	}
	return svgText{elements: faces.elements}
}

// svgText draws an SVG's text with element faces.
type svgText struct{ elements *elementFaces }

// format returns the text format of style in faces.
func (text svgText) format(faces *Faces, style svgimage.TextStyle) textFormat {
	format := textFormat{size: max(style.Size, 1)}
	for _, family := range style.Families {
		lower := strings.ToLower(family)
		if _, embedded := faces.embedded[lower]; embedded || lower == "serif" || lower == "sans-serif" || lower == "monospace" {
			format.family = lower
			break
		}
	}
	if style.Bold {
		format.style |= document.Bold
	}
	if style.Italic {
		format.style |= document.Italic
	}
	return format
}

func (text svgText) Width(style svgimage.TextStyle, content string) (width int) {
	text.elements.with(func(faces *Faces) { width = faces.width(text.format(faces, style), content) })
	return width
}

func (text svgText) Draw(dst *image.Gray, style svgimage.TextStyle, origin image.Point, content string) (err error) {
	text.elements.with(func(faces *Faces) {
		format := text.format(faces, style)
		var font *fonts.Font
		if font, err = faces.FamilyFont(format.family, format.style, format.size); err != nil {
			return
		}
		err = faces.draw(dst, format, image.Pt(origin.X, origin.Y-font.Ascent()), content, color.Gray{Y: style.Gray})
	})
	return err
}
