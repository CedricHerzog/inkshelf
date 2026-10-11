package container

import (
	"cmp"
	"log"
	"path"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/webfont"
)

// Stylesheet returns the stylesheet at sheetPath, parsed once for every chapter that links
// it, its @media rules kept if they match media (the same for every call); nil if the book
// has none there.
func (book *Book) Stylesheet(sheetPath string, media css.Media) *css.Stylesheet {
	book.cacheMu.Lock()
	defer book.cacheMu.Unlock()
	if sheet, ok := book.sheets[sheetPath]; ok {
		return sheet
	}
	var sheet *css.Stylesheet
	if data, err := book.ReadFile(sheetPath); err == nil {
		sheet = css.Parse(string(data), media)
	}
	if book.sheets == nil {
		book.sheets = map[string]*css.Stylesheet{}
	}
	book.sheets[sheetPath] = sheet
	return sheet
}

// Font returns the font at fontPath, loaded once: de-obfuscated, converted from WOFF or WOFF2
// and parsed. It returns nil for a font that cannot be loaded, which is logged.
func (book *Book) Font(fontPath string) *fonts.File {
	book.cacheMu.Lock()
	defer book.cacheMu.Unlock()
	if data, ok := book.fonts[fontPath]; ok {
		return data
	}
	data, err := book.loadFont(fontPath)
	if err != nil {
		log.Printf("epub: font %s: %v", fontPath, err)
	}
	if book.fonts == nil {
		book.fonts = map[string]*fonts.File{}
	}
	book.fonts[fontPath] = data
	return data
}

// loadFont reads the font at fontPath. An obfuscated font is tried with each identifier of
// the book as its key, until one gives a font; an Adobe one, then with the key its header gives.
func (book *Book) loadFont(fontPath string) (*fonts.File, error) {
	data, err := book.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	algorithm, obfuscated := book.obfuscated[fontPath]
	if !obfuscated {
		return parseFont(fontPath, data)
	}
	var firstErr error
	for _, identifier := range book.identifiers {
		plain, err := webfont.Deobfuscate(data, algorithm, identifier)
		if err == nil {
			var font *fonts.File
			if font, err = parseFont(fontPath, plain); err == nil {
				return font, nil
			}
		}
		firstErr = cmp.Or(firstErr, err)
	}
	if algorithm == webfont.AdobeAlgorithm {
		// Books converted by Calibre often lose the UUID the font was obfuscated with.
		if plain, err := webfont.RecoverAdobe(data); err == nil {
			if font, err := parseFont(fontPath, plain); err == nil {
				log.Printf("epub: font %s: Adobe key recovered from the font", fontPath)
				return font, nil
			}
		}
	}
	return nil, firstErr
}

// parseFont parses the data of the font at fontPath: TrueType, OpenType, WOFF or WOFF2.
func parseFont(fontPath string, data []byte) (*fonts.File, error) {
	sfnt, err := webfont.ToSFNT(data)
	if err != nil {
		return nil, err
	}
	return fonts.Parse(path.Base(fontPath), sfnt)
}
