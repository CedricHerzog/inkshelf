package layout

import (
	"image"
	"image/color"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
)

// SVGText returns what draws the text of an SVG with the faces: in the first of its families
// that a document embeds or that is generic, else in the book font, with the fallbacks of a
// page's text. Nil faces return nil, which leaves an SVG's text out.
func (faces *Faces) SVGText() svgimage.TextDrawer {
	if faces == nil {
		return nil
	}
	return svgText{faces: faces}
}

// svgText draws an SVG's text with Faces.
type svgText struct{ faces *Faces }

// format returns the text format of style.
func (text svgText) format(style svgimage.TextStyle) textFormat {
	format := textFormat{size: max(style.Size, 1)}
	for _, family := range style.Families {
		lower := strings.ToLower(family)
		if _, embedded := text.faces.embedded[lower]; embedded || lower == "serif" || lower == "sans-serif" || lower == "monospace" {
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

func (text svgText) Width(style svgimage.TextStyle, content string) int {
	return text.faces.width(text.format(style), content)
}

func (text svgText) Draw(dst *image.Gray, style svgimage.TextStyle, origin image.Point, content string) error {
	format := text.format(style)
	font, err := text.faces.FamilyFont(format.family, format.style, format.size)
	if err != nil {
		return err
	}
	return text.faces.draw(dst, format, image.Pt(origin.X, origin.Y-font.Ascent()), content, color.Gray{Y: style.Gray})
}
