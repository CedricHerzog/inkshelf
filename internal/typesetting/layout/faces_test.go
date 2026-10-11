package layout

import (
	"image"
	"testing"

	"github.com/tdewolff/font"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// lozenge is a character that Go's fonts have, but not asciiFont.
const lozenge = '◊'

// asciiFont returns Go Regular with only its ASCII characters: an embedded font that lacks ◊, as
// a book's Charter did.
func asciiFont(t *testing.T) *fonts.File {
	t.Helper()
	parsed, err := font.ParseSFNT(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	glyphs := []uint16{0} // .notdef
	for character := ' '; character <= '~'; character++ {
		glyphs = append(glyphs, parsed.GlyphIndex(character))
	}
	subset, err := parsed.Subset(glyphs, font.SubsetOptions{Tables: font.KeepAllTables})
	if err != nil {
		t.Fatal(err)
	}
	data, err := fonts.Parse("ascii", subset.Write())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// asciiDocument returns a paragraph of runs in asciiFont's family, "Ascii".
func asciiDocument(t *testing.T, runs ...document.Run) *document.Document {
	t.Helper()
	doc := oneBlock(document.Block{Runs: runs})
	doc.Fonts = map[string]map[fonts.Style]*fonts.File{"ascii": {fonts.Regular: asciiFont(t)}}
	return doc
}

// A character that a word's font lacks is measured and drawn in the reader's typeface, on the
// same baseline; the rest of the word stays in its font.
func TestGlyphFallback(t *testing.T) {
	faces := newTestFaces(t)
	ascii, err := asciiFont(t).Open(24)
	if err != nil {
		t.Fatal(err)
	}
	fallback, _ := faces.Font(0, 24) // the reader's typeface: Go's off the reader
	if ascii.Has(lozenge) || !fallback.Has(lozenge) {
		t.Fatalf("the test font has ◊: %v, the fallback: %v", ascii.Has(lozenge), fallback.Has(lozenge))
	}
	text := "a" + string(lozenge) + "b"
	page := paginateOne(t, asciiDocument(t, document.Run{Text: text + " ", Family: "Ascii"}, document.Run{Text: "end"}), leftSettings, faces, noImages)
	word := wordAt(t, page, text)
	if got, want := wordAt(t, page, "end").X-word.X, ascii.Width("a")+fallback.Width(string(lozenge))+ascii.Width("b")+ascii.Width(" "); got != want {
		t.Errorf("%q is %d wide with its space, want %d", text, got, want)
	}

	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, nil); err != nil {
		t.Fatal(err)
	}
	// The lozenge as the fallback draws it, at its place.
	left := word.X + ascii.Width("a")
	want := whiteCanvas(leftSettings)
	fallback.DrawTo(want, image.Pt(left, word.Top+ascii.Ascent()-fallback.Ascent()), string(lozenge), black)
	for y := range canvas.Rect.Dy() {
		for x := left; x < left+fallback.Width(string(lozenge)); x++ {
			if got, wanted := canvas.GrayAt(x, y).Y, want.GrayAt(x, y).Y; got != wanted {
				t.Fatalf("pixel (%d, %d) is %d, want the fallback's %d", x, y, got, wanted)
			}
		}
	}
}

// Letter spacing and small capitals measure a fallback's characters as the others.
func TestGlyphFallbackStyles(t *testing.T) {
	faces := newTestFaces(t)
	ascii, _ := asciiFont(t).Open(24)
	small, _ := asciiFont(t).Open(24 * smallCapsScale / 100)
	fallback, _ := faces.Font(0, 24)
	lozengeWidth := fallback.Width(string(lozenge))
	spacing := 50 * 24 / 100
	tests := []struct {
		run  document.Run
		want int // from the word to the next
	}{
		{document.Run{Text: "a◊ ", Family: "Ascii", LetterSpacing: 50}, ascii.Width("a") + lozengeWidth + ascii.Width(" ") + 3*spacing},
		{document.Run{Text: "a◊ ", Family: "Ascii", Style: document.SmallCaps}, small.Width("A") + lozengeWidth + ascii.Width(" ")},
	}
	for _, test := range tests {
		page := paginateOne(t, asciiDocument(t, test.run, document.Run{Text: "end"}), leftSettings, faces, noImages)
		if got := wordAt(t, page, "end").X - wordAt(t, page, "a◊").X; got != test.want {
			t.Errorf("%+v: %d wide with its space, want %d", test.run, got, test.want)
		}
		if err := page.Draw(whiteCanvas(leftSettings), faces, nil); err != nil {
			t.Fatal(err)
		}
	}
}

// Text whose characters its font has opens no fallback, and is drawn in one part.
func TestNoFallbackForASCII(t *testing.T) {
	faces := newTestFaces(t)
	page := paginateOne(t, asciiDocument(t, document.Run{Text: "Plain text, ", Family: "Ascii"}, document.Run{Text: "and the book's."}), leftSettings, faces, noImages)
	if err := page.Draw(whiteCanvas(leftSettings), faces, nil); err != nil {
		t.Fatal(err)
	}
	if len(faces.fallbacks) != 0 {
		t.Errorf("fallbacks opened: %v", faces.fallbacks)
	}
	parts := 0
	err := faces.forEachFontPart(textFormat{family: "Ascii", size: 24}, "Plain", func(*fonts.Font, string, int) { parts++ })
	if err != nil || parts != 1 {
		t.Errorf("%d parts (%v), want 1", parts, err)
	}
}
