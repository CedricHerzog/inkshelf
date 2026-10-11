package epub

import (
	"cmp"
	"path"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// fontFormats are the formats of @font-face sources that the app reads, by format() or by
// file extension.
var fontFormats = map[string]bool{
	"truetype": true, "opentype": true, "woff": true, "woff2": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
}

// linkStylesheet adds the stylesheet at href (relative to the chapter) to the chapter's
// cascade if media matches the reader.
func (parser *builder) linkStylesheet(href, media string) {
	if screen.Matches(media) {
		parser.addStylesheetAt(resolve(parser.chapterPath, href))
	}
}

// addStylesheetAt adds the stylesheet at sheetPath to the chapter's cascade, once: a sheet
// that imports itself, or that the chapter links twice, is added the first time only.
func (parser *builder) addStylesheetAt(sheetPath string) {
	if parser.sheetsAdded[sheetPath] {
		return
	}
	parser.sheetsAdded[sheetPath] = true
	if sheet := parser.book.Stylesheet(sheetPath, screen); sheet != nil {
		parser.addStylesheet(sheet, sheetPath)
	}
}

// addStylesheet adds sheet to the chapter's cascade, after the sheets it imports, and its
// fonts to the document. Its URLs are relative to sheetPath.
func (parser *builder) addStylesheet(sheet *css.Stylesheet, sheetPath string) {
	for _, imported := range sheet.Imports {
		parser.addStylesheetAt(resolve(sheetPath, imported))
	}
	parser.cascade.Add(sheet)
	for _, face := range sheet.FontFaces {
		parser.addFont(face, sheetPath)
	}
}

// addFont adds the font of an @font-face rule to the document: the first of its sources in a
// format the app reads, by its family in lower case and its style. Measuring, it adds none.
func (parser *builder) addFont(face css.FontFace, sheetPath string) {
	if face.Family == "" || parser.forIndex {
		return
	}
	for _, source := range face.Sources {
		address, _, _ := strings.Cut(source.URL, "?")
		format := cmp.Or(source.Format, path.Ext(address))
		if !fontFormats[strings.ToLower(format)] {
			continue
		}
		data := parser.book.Font(resolve(sheetPath, address))
		if data == nil {
			return
		}
		family := strings.ToLower(face.Family)
		if parser.doc.Fonts == nil {
			parser.doc.Fonts = map[string]map[fonts.Style]*fonts.File{}
		}
		if parser.doc.Fonts[family] == nil {
			parser.doc.Fonts[family] = map[fonts.Style]*fonts.File{}
		}
		parser.doc.Fonts[family][fontStyleOf(face)] = data
		return
	}
}

// fontStyleOf returns the style of the font of an @font-face rule.
func fontStyleOf(face css.FontFace) fonts.Style {
	bold, italic := isBold(strings.ToLower(face.Weight)), isItalic(strings.ToLower(face.Style))
	switch {
	case bold && italic:
		return fonts.BoldItalic
	case bold:
		return fonts.Bold
	case italic:
		return fonts.Italic
	}
	return fonts.Regular
}

// familyOf returns the family of a font-family value, as document.Run.Family: the first that is
// generic (serif, sans-serif or monospace) or that the document embeds, in lower case; "" for
// the reader's font if none is.
func (parser *builder) familyOf(value string) string {
	for _, family := range css.Families(value) {
		family = strings.ToLower(family)
		if _, embedded := parser.doc.Fonts[family]; embedded || family == "serif" || family == "sans-serif" || family == "monospace" {
			return family
		}
	}
	return ""
}
