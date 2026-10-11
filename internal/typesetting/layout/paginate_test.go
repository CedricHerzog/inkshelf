package layout

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// testSettings is a small page with Go's fonts (off the reader), justified.
var testSettings = Settings{
	Width: 600, Height: 400, Margins: image.Rect(30, 20, 30, 20),
	FontSize: 24, LineSpacing: 130, Justify: true,
}

// newTestFaces returns serif faces (Go's fonts, off the reader), closed after the test.
func newTestFaces(tb testing.TB) *Faces {
	tb.Helper()
	faces := NewFaces(fonts.Serif)
	tb.Cleanup(faces.Close)
	return faces
}

// noImages is an ImageSizer for documents without images.
func noImages(path string) (image.Point, error) { return image.Point{}, errors.New("no images") }

// testDocument returns a heading, then paragraphs of sentences, with italic, bold, French
// spaces and soft hyphens.
func testDocument(paragraphs int) *document.Document {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Heading, Level: 1, Align: document.AlignCenter, FontScale: 150, MarginBottom: 100, KeepWithNext: true,
		Runs: []document.Run{{Text: "Chapitre premier", Style: document.Bold}}})
	for index := range paragraphs {
		doc.Add(document.Block{Kind: document.Paragraph, Indent: 150, Runs: []document.Run{
			{Text: fmt.Sprintf("Paragraphe %d. Il fallait par\u00adtir avant l'aube, sans bruit ; ", index+1)},
			{Text: "personne ne devait le savoir", Style: document.Italic},
			{Text: ", pas même le meunier du village voisin, qui veillait tard. "},
			{Text: "Soudain", Style: document.Bold},
			{Text: " ! The road wound through the hills — every turn showed a greener valley."},
		}})
	}
	return doc
}

// Pages cover the document's offsets in order, without gaps; no line is wider than the text
// nor any page taller.
func TestPaginateInvariants(t *testing.T) {
	doc := testDocument(12)
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 3 {
		t.Fatalf("%d pages, want several", len(pages))
	}
	content := testSettings.content()
	lineHeight := testSettings.FontSize * testSettings.LineSpacing / 100
	if pages[0].Start != 0 || pages[len(pages)-1].End != doc.Chars {
		t.Errorf("pages cover %d to %d, want 0 to %d", pages[0].Start, pages[len(pages)-1].End, doc.Chars)
	}
	faces := newTestFaces(t)
	for index, page := range pages {
		if index > 0 && page.Start != pages[index-1].End {
			t.Errorf("page %d starts at %d, page %d ends at %d", index, page.Start, index-1, pages[index-1].End)
		}
		if page.Start >= page.End {
			t.Errorf("page %d is empty: %d to %d", index, page.Start, page.End)
		}
		for _, line := range page.Lines {
			last := line.Words[len(line.Words)-1]
			font, err := faces.Font(last.Style, last.Size)
			if err != nil {
				t.Fatal(err)
			}
			if right := last.X + font.Width(last.Text); right > content.Max.X {
				t.Errorf("page %d: line %q ends at x %d, past %d", index, lineText(line), right, content.Max.X)
			}
			if line.Words[0].Top+lineHeight*3/2 > content.Max.Y+lineHeight {
				t.Errorf("page %d: line at y %d, past %d", index, line.Words[0].Top, content.Max.Y)
			}
		}
	}
	if got := PageAt(pages, pages[2].Start+1); got != 2 {
		t.Errorf("PageAt in page 2 = %d", got)
	}
}

// lineText returns the words of line, separated by spaces.
func lineText(line Line) string {
	var words []string
	for _, word := range line.Words {
		words = append(words, word.Text)
	}
	return strings.Join(words, " ")
}

// allLines returns the text of each line of pages.
func allLines(pages []Page) []string {
	var lines []string
	for _, page := range pages {
		for _, line := range page.Lines {
			lines = append(lines, lineText(line))
		}
	}
	return lines
}

// Justified lines end at the right margin, but the paragraph's last; a centred heading is
// centred.
func TestJustify(t *testing.T) {
	pages, err := Paginate(testDocument(1), testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	faces := newTestFaces(t)
	content := testSettings.content()
	lines := pages[0].Lines
	heading := lines[0].Words[0]
	headingFont, _ := faces.Font(heading.Style, heading.Size)
	headingRight := heading.X + headingFont.Width(lineText(lines[0]))
	if left, right := heading.X-content.Min.X, content.Max.X-headingRight; left-right > 2 || right-left > 2 {
		t.Errorf("heading margins %d and %d, want them equal", left, right)
	}
	for index, line := range lines[1 : len(lines)-1] {
		last := line.Words[len(line.Words)-1]
		font, _ := faces.Font(last.Style, last.Size)
		if right := last.X + font.Width(last.Text); right != content.Max.X {
			t.Errorf("line %d %q ends at %d, want %d", index+1, lineText(line), right, content.Max.X)
		}
	}
	if indent := lines[1].Words[0].X - content.Min.X; indent != testSettings.em(150) {
		t.Errorf("first line indented by %d, want %d", indent, testSettings.em(150))
	}
}

// A soft hyphen breaks a word with a hyphen drawn; a no-break space never breaks.
func TestBreakPoints(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: strings.Repeat("Il fallait par\u00adtir maintenant ; ah\u00a0! ", 12)}}})
	faces := newTestFaces(t)
	hyphenated := false
	for width := 200; width <= 500; width += 7 { // some widths break inside "partir"
		settings := testSettings
		settings.Width = width
		pages, err := Paginate(doc, settings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range allLines(pages) {
			hyphenated = hyphenated || strings.HasSuffix(line, "par-")
			if strings.Contains(line, "\u00ad") {
				t.Errorf("width %d: soft hyphen drawn in %q", width, line)
			}
			if strings.HasPrefix(line, "!") {
				t.Errorf("width %d: broken at a no-break space before %q", width, line)
			}
		}
	}
	if !hyphenated {
		t.Error("no line broke at the soft hyphen")
	}
}

// A word wider than the line is cut between its characters.
func TestLongWord(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "a " + strings.Repeat("anticonstitutionnellement", 4) + " b"}}})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	lines := allLines(pages)
	if len(lines) < 4 || lines[0] != "a" || lines[len(lines)-1] != "b" {
		t.Errorf("lines %q", lines)
	}
	if got := strings.Join(lines, ""); got != "a"+strings.Repeat("anticonstitutionnellement", 4)+"b" {
		t.Errorf("text %q lost characters", got)
	}
}

// A heading near the bottom of a page moves to the next, with the paragraph after it.
func TestKeepWithNext(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: strings.Repeat("Une phrase assez longue pour remplir des lignes. ", 18)}}})
	doc.Add(document.Block{Kind: document.Heading, FontScale: 150, KeepWithNext: true, Runs: []document.Run{{Text: "Titre", Style: document.Bold}}})
	doc.Add(document.Block{Runs: []document.Run{{Text: strings.Repeat("La suite du texte. ", 20)}}})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	for index, page := range pages {
		lines := page.Lines
		if len(lines) > 0 && lineText(lines[len(lines)-1]) == "Titre" {
			t.Errorf("page %d ends with the heading", index)
		}
	}
}

// encodedPNG returns a PNG of size.
func encodedPNG(t *testing.T, size image.Point) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rectangle{Max: size})); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// An image is enlarged at most maxImageScale times, fitted to the page, and drawn.
func TestImages(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "Avant l'image."}}})
	doc.Add(document.Block{Kind: document.Image, Image: "small.png"})
	doc.Add(document.Block{Kind: document.Image, Image: "tall.png"})
	files := map[string][]byte{"small.png": encodedPNG(t, image.Pt(40, 20)), "tall.png": encodedPNG(t, image.Pt(300, 3000))}
	sizer := func(path string) (image.Point, error) { return webimage.Size(files[path]) }
	faces := newTestFaces(t)
	pages, err := Paginate(doc, testSettings, faces, sizer)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || len(pages[0].Images) != 1 || len(pages[1].Images) != 1 {
		t.Fatalf("pages %+v, want the small image on page 1, the tall one on page 2", pages)
	}
	if size := pages[0].Images[0].Rect.Size(); size != image.Pt(120, 60) {
		t.Errorf("small image %v, want 120x60", size)
	}
	content := testSettings.content()
	if rect := pages[1].Images[0].Rect; rect.Dy() != content.Dy() || rect.Min.Y != content.Min.Y {
		t.Errorf("tall image at %v, want the page's height %d", rect, content.Dy())
	}
	if pages[1].Start != 15 || pages[1].End != doc.Chars { // after the 14 runes of text and the small image
		t.Errorf("page 2 covers %d to %d", pages[1].Start, pages[1].End)
	}
	canvas := image.NewGray(image.Rect(0, 0, testSettings.Width, testSettings.Height))
	for index := range canvas.Pix {
		canvas.Pix[index] = 255
	}
	if err := pages[0].Draw(canvas, faces, webimage.NewLoader(readFrom(files), faces.SVGText())); err != nil {
		t.Fatal(err)
	}
	if center := pages[0].Images[0].Rect.Min.Add(image.Pt(60, 30)); canvas.GrayAt(center.X, center.Y).Y > 64 {
		t.Errorf("the black image is not drawn: %d at %v", canvas.GrayAt(center.X, center.Y).Y, center)
	}
}

// readFrom returns a reader of files, path → data.
func readFrom(files map[string][]byte) func(path string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		data, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("%s: not in the book", path)
		}
		return data, nil
	}
}

// An SVG cover wrapper is sized from its viewBox and drawn with the image it links to,
// relative to its own file.
func TestSVGImage(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Image, Image: "text/cover.svg"})
	files := map[string][]byte{
		"text/cover.svg": []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" ` +
			`xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 40 20">` +
			`<image width="40" height="20" xlink:href="../images/black%20page.png"/></svg>`),
		"images/black page.png": encodedPNG(t, image.Pt(40, 20)),
	}
	read := readFrom(files)
	sizer := func(path string) (image.Point, error) {
		data, err := read(path)
		if err != nil {
			return image.Point{}, err
		}
		return webimage.Size(data)
	}
	faces := newTestFaces(t)
	pages, err := Paginate(doc, testSettings, faces, sizer)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 || pages[0].Images[0].Rect.Size() != image.Pt(120, 60) {
		t.Fatalf("pages %+v, want the 40x20 SVG enlarged to 120x60", pages)
	}
	canvas := image.NewGray(image.Rect(0, 0, testSettings.Width, testSettings.Height))
	for index := range canvas.Pix {
		canvas.Pix[index] = 255
	}
	if err := pages[0].Draw(canvas, faces, webimage.NewLoader(read, faces.SVGText())); err != nil {
		t.Fatal(err)
	}
	if center := pages[0].Images[0].Rect.Min.Add(image.Pt(60, 30)); canvas.GrayAt(center.X, center.Y).Y > 64 {
		t.Errorf("the SVG's image is not drawn: %d at %v", canvas.GrayAt(center.X, center.Y).Y, center)
	}
}

func TestEmptyDocument(t *testing.T) {
	pages, err := Paginate(&document.Document{}, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].Start != 0 || pages[0].End != 0 {
		t.Errorf("pages %+v, want one empty page", pages)
	}
}

// benchmarkSettings is the InkPad 3's screen.
var benchmarkSettings = Settings{
	Width: 1404, Height: 1872, Margins: image.Rect(90, 80, 90, 100),
	FontSize: 44, LineSpacing: 135, Justify: true,
}

// A chapter of a novel: about 60 paragraphs, 25,000 characters.
func BenchmarkPaginateChapter(b *testing.B) {
	doc := testDocument(60)
	faces := newTestFaces(b)
	for b.Loop() {
		if _, err := Paginate(doc, benchmarkSettings, faces, noImages); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderPage(b *testing.B) {
	faces := newTestFaces(b)
	pages, err := Paginate(testDocument(60), benchmarkSettings, faces, noImages)
	if err != nil {
		b.Fatal(err)
	}
	canvas := image.NewGray(image.Rect(0, 0, benchmarkSettings.Width, benchmarkSettings.Height))
	for b.Loop() {
		for index := range canvas.Pix {
			canvas.Pix[index] = 255
		}
		if err := pages[1].Draw(canvas, faces, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// An SVG's text is drawn with the faces: dark pixels where it is, none without faces.
func TestSVGText(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100">` +
		`<text x="100" y="70" font-family="sans-serif" font-size="60" text-anchor="middle">Ink</text></svg>`)
	read := func(string) ([]byte, error) { return svg, nil }
	darkPixels := func(faces *Faces) (count, leftmost int) {
		picture, err := webimage.NewLoader(read, faces.SVGText())("text.svg", image.Pt(200, 100))
		if err != nil {
			t.Fatal(err)
		}
		leftmost = picture.Bounds().Dx()
		for row := range picture.Bounds().Dy() {
			for column := range picture.Bounds().Dx() {
				if picture.GrayAt(column, row).Y < 128 {
					count++
					leftmost = min(leftmost, column)
				}
			}
		}
		return count, leftmost
	}
	if count, leftmost := darkPixels(newTestFaces(t)); count < 200 || leftmost < 30 || leftmost > 90 {
		t.Errorf("%d dark pixels from x %d, want the text centred", count, leftmost)
	}
	if count, _ := darkPixels(nil); count != 0 {
		t.Errorf("%d dark pixels without faces, want none", count)
	}
}
