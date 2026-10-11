package layout

import (
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// formulaPath returns the image path of a formula's markup.
func formulaPath(t *testing.T, markup string) string {
	t.Helper()
	root, err := mathml.Parse(markup)
	if err != nil {
		t.Fatal(err)
	}
	return mathml.Path(root)
}

// A formula in a line sits on its baseline, going under it, laid out at the text's size; a
// displayed one is centred, larger with the text; both are drawn with the page's fonts.
func TestFormulas(t *testing.T) {
	fraction := formulaPath(t, `<math><mfrac><mi>a</mi><mi>b</mi></mfrac></math>`)
	display := formulaPath(t, `<math display="block"><mfrac><mn>1</mn><mrow><mi>n</mi><mo>+</mo><mn>1</mn></mrow></mfrac></math>`)
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "So "}, {Image: fraction}, {Text: " is a ratio."}}})
	doc.Add(document.Block{Kind: document.Image, Image: display})
	faces := newTestFaces(t)
	layOut := func(settings Settings) Page {
		pages, err := Paginate(doc, settings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 || len(pages[0].Images) != 2 {
			t.Fatalf("pages %+v, want one with two formulas", pages)
		}
		return pages[0]
	}
	page := layOut(testSettings)
	inline, block := page.Images[0], page.Images[1]
	firstLine := page.Lines[0]
	baseline := firstLine.Words[0].Top + mustFont(t, faces, testSettings.FontSize).Ascent()
	if inline.MathSize != testSettings.FontSize || inline.Rect.Min.Y >= baseline-testSettings.FontSize/3 || inline.Rect.Max.Y <= baseline+testSettings.FontSize/5 {
		t.Errorf("inline formula %+v, baseline %d: want it across the baseline", inline, baseline)
	}
	content := testSettings.content()
	if left, right := block.Rect.Min.X-content.Min.X, content.Max.X-block.Rect.Max.X; left-right > 1 || right-left > 1 || block.Rect.Min.Y < page.Lines[0].Words[0].Top {
		t.Errorf("displayed formula at %v, want centred under the text", block.Rect)
	}
	// The line is taller than a line of text alone, for the fraction under its baseline.
	if lineHeight := testSettings.FontSize * testSettings.LineSpacing / 100; block.Rect.Min.Y-content.Min.Y <= lineHeight {
		t.Errorf("formula's line ends at %d, want it taller than %d", block.Rect.Min.Y-content.Min.Y, lineHeight)
	}
	larger := testSettings
	larger.FontSize = 36
	if grown := layOut(larger).Images[1]; grown.MathSize != 36 || grown.Rect.Dy() <= block.Rect.Dy() {
		t.Errorf("formula at 36 pixels %+v, at 24 %+v: want it larger", grown, block)
	}
	canvas := image.NewGray(image.Rect(0, 0, testSettings.Width, testSettings.Height))
	for index := range canvas.Pix {
		canvas.Pix[index] = 255
	}
	if err := page.Draw(canvas, faces, func(string, image.Point) (*image.Gray, error) {
		t.Errorf("a formula drawn as an image")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, placed := range page.Images {
		inked := 0
		for y := placed.Rect.Min.Y; y < placed.Rect.Max.Y; y++ {
			for x := placed.Rect.Min.X; x < placed.Rect.Max.X; x++ {
				if canvas.GrayAt(x, y).Y < 128 {
					inked++
				}
			}
		}
		if inked < 20 {
			t.Errorf("formula at %v: %d pixels inked", placed.Rect, inked)
		}
	}
	// A formula wider than the line is laid out smaller.
	wide := &document.Document{}
	wide.Add(document.Block{Kind: document.Image, Image: formulaPath(t, `<math><mtext>an equation far too wide for the narrow test page at all</mtext></math>`)})
	pages, err := Paginate(wide, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	if placed := pages[0].Images[0]; placed.Rect.Dx() > content.Dx() || placed.MathSize >= testSettings.FontSize {
		t.Errorf("wide formula %+v, want it in %d pixels", placed, content.Dx())
	}
}

// mustFont returns the book font at size.
func mustFont(t *testing.T, faces *Faces, size int) *fonts.Font {
	t.Helper()
	font, err := faces.Font(0, size)
	if err != nil {
		t.Fatal(err)
	}
	return font
}
