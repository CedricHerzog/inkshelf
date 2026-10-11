package layout

import (
	"image"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// squareImages is an ImageSizer of square images 100 pixels wide.
func squareImages(path string) (image.Point, error) { return image.Pt(100, 100), nil }

// floatDocument returns a document of a float image of side, FloatWidth width, then
// paragraphs.
func floatDocument(side document.Float, width, paragraphs int) *document.Document {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Image, Image: "picture.png", ImageWidth: 100, Float: side, FloatGroup: 1, FloatWidth: width})
	for range paragraphs {
		doc.Add(testDocument(1).Blocks[1])
	}
	return doc
}

// The lines beside a float go beside it, those under it across the page; the pages still
// cover the document in order.
func TestFloatImage(t *testing.T) {
	faces := newTestFaces(t)
	content := testSettings.content()
	for _, side := range []document.Float{document.FloatLeft, document.FloatRight} {
		doc := floatDocument(side, 30, 3)
		pages, err := Paginate(doc, testSettings, faces, squareImages)
		if err != nil {
			t.Fatal(err)
		}
		checkCoverage(t, pages, doc)
		page := pages[0]
		if len(page.Images) != 1 {
			t.Fatalf("side %d: %d images, want 1", side, len(page.Images))
		}
		picture := page.Images[0].Rect
		if picture.Dx() != content.Dx()*30/100 || picture.Min.Y != content.Min.Y ||
			side == document.FloatLeft && picture.Min.X != content.Min.X || side == document.FloatRight && picture.Max.X != content.Max.X {
			t.Errorf("side %d: image at %v in %v, want 30%% of the text at its side", side, picture, content)
		}
		beside, under := 0, 0
		for _, line := range page.Lines {
			left, right := lineEdges(faces, line)
			top := line.Words[0].Top
			switch {
			case top < picture.Max.Y:
				beside++
				if side == document.FloatLeft && left < picture.Max.X || side == document.FloatRight && right > picture.Min.X {
					t.Errorf("side %d: line %q from %d to %d overlaps the image %v", side, lineText(line), left, right, picture)
				}
			case top > picture.Max.Y+testSettings.FontSize:
				under++
				if right < content.Max.X-testSettings.FontSize && left < content.Min.X+testSettings.FontSize*2 && strings.HasSuffix(lineText(line), ".") {
					continue // a paragraph's last line
				}
				if left > content.Min.X+testSettings.em(150) || right < content.Max.X-2 {
					t.Errorf("side %d: line %q under the image from %d to %d, want across the text", side, lineText(line), left, right)
				}
			}
		}
		if beside < 3 || under < 2 {
			t.Errorf("side %d: %d lines beside the image, %d under it", side, beside, under)
		}
	}
}

// checkCoverage checks that pages cover doc's offsets in order, from 0 to its end.
func checkCoverage(t *testing.T, pages []Page, doc *document.Document) {
	t.Helper()
	if pages[0].Start != 0 || pages[len(pages)-1].End != doc.Chars {
		t.Errorf("pages cover %d to %d, want 0 to %d", pages[0].Start, pages[len(pages)-1].End, doc.Chars)
	}
	for index := 1; index < len(pages); index++ {
		if pages[index].Start != pages[index-1].End || pages[index].Start < pages[index-1].Start {
			t.Errorf("page %d starts at %d, page %d ends at %d", index, pages[index].Start, index-1, pages[index-1].End)
		}
	}
}

// A float too wide stays in the text, as a block; so does one on a page too narrow.
func TestFloatTooWide(t *testing.T) {
	faces := newTestFaces(t)
	narrow := testSettings
	narrow.Width = 260
	for _, test := range []struct {
		width    int
		settings Settings
	}{{80, testSettings}, {40, narrow}} {
		pages, err := Paginate(floatDocument(document.FloatLeft, test.width, 1), test.settings, faces, squareImages)
		if err != nil {
			t.Fatal(err)
		}
		picture := pages[0].Images[0].Rect
		for _, line := range pages[0].Lines {
			if line.Words[0].Top < picture.Max.Y {
				t.Errorf("%d%% on a page %d wide: line %q at %d beside the image %v, want under it",
					test.width, test.settings.Width, lineText(line), line.Words[0].Top, picture)
			}
		}
	}
}

// Floats of a side stack; a block that clears them starts under them.
func TestFloatStackAndClear(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Image, Image: "one.png", ImageWidth: 100, Float: document.FloatLeft, FloatGroup: 1, FloatWidth: 20})
	doc.Add(document.Block{Kind: document.Image, Image: "two.png", ImageWidth: 100, Float: document.FloatLeft, FloatGroup: 2, FloatWidth: 20})
	doc.Add(document.Block{Kind: document.Paragraph, Runs: []document.Run{{Text: "Beside."}}})
	doc.Add(document.Block{Kind: document.Paragraph, Clear: document.ClearLeft, Runs: []document.Run{{Text: "Under."}}})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), squareImages)
	if err != nil {
		t.Fatal(err)
	}
	page := pages[0]
	one, two := page.Images[0].Rect, page.Images[1].Rect
	if two.Min.X <= one.Max.X || two.Min.Y != one.Min.Y {
		t.Errorf("images at %v and %v, want side by side", one, two)
	}
	if len(page.Lines) != 2 || page.Lines[0].Words[0].X <= two.Max.X || page.Lines[1].Words[0].Top < two.Max.Y {
		t.Errorf("lines %+v, want one beside the images, one under them", page.Lines)
	}
}

// A float that does not fit on the rest of the page goes on the next, the text going on.
func TestFloatOnNextPage(t *testing.T) {
	doc := testDocument(3)
	doc.Add(document.Block{Kind: document.Image, Image: "picture.png", ImageWidth: 100, Float: document.FloatRight, FloatGroup: 1, FloatWidth: 40})
	for range 3 {
		doc.Add(testDocument(1).Blocks[1])
	}
	pages, err := Paginate(doc, testSettings, newTestFaces(t), squareImages)
	if err != nil {
		t.Fatal(err)
	}
	checkCoverage(t, pages, doc)
	floatStart := doc.Blocks[4].Start
	for index, page := range pages {
		if len(page.Images) == 0 {
			continue
		}
		if page.Start <= floatStart {
			t.Errorf("the image is on page %d, which starts at %d, before it", index, page.Start)
		}
		if previous := pages[index-1]; previous.End <= floatStart {
			t.Errorf("page %d ends at %d, want the text after the image (%d) on it", index-1, previous.End, floatStart)
		}
		return
	}
	t.Error("no image placed")
}

// A drop cap's baseline is that of its paragraph's second or third line, the lines down to it
// beside it, the others across the text.
func TestDropCap(t *testing.T) {
	faces := newTestFaces(t)
	for _, test := range []struct {
		scale, lines int
	}{{250, 2}, {380, 3}} {
		doc := &document.Document{}
		doc.Add(document.Block{Kind: document.Paragraph, Float: document.FloatLeft, FloatGroup: 1, FontScale: test.scale, LineHeight: 100, Runs: []document.Run{{Text: "L"}}})
		paragraph := testDocument(1).Blocks[1]
		paragraph.Runs = append([]document.Run{{Text: "orem ipsum dolor sit amet. "}}, paragraph.Runs...)
		doc.Add(paragraph)
		pages, err := Paginate(doc, testSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		checkCoverage(t, pages, doc)
		lines := pages[0].Lines
		capWord := lines[0].Words[0]
		if capWord.Text != "L" {
			t.Fatalf("first line %q, want the drop cap", lineText(lines[0]))
		}
		baseline := func(word PlacedWord) int {
			font, err := faces.Font(word.Style, word.Size)
			if err != nil {
				t.Fatal(err)
			}
			return word.Top + font.Ascent()
		}
		text := lines[1:]
		if got, want := baseline(capWord), baseline(text[test.lines-1].Words[0]); got != want {
			t.Errorf("%d%%: drop cap's baseline at %d, want %d, its paragraph's line %d's", test.scale, got, want, test.lines)
		}
		capRight := capWord.X + faces.width(capWord.format(), capWord.Text)
		for index, line := range text {
			if beside := index < test.lines; beside != (line.Words[0].X > capRight) {
				t.Errorf("%d%%: line %d %q from x %d, the drop cap ending at %d", test.scale, index, lineText(line), line.Words[0].X, capRight)
			}
		}
	}
}

// A floated letter hardly larger than its paragraph goes back into it, as its first run.
func TestDropCapInline(t *testing.T) {
	doc := &document.Document{}
	doc.Add(document.Block{Kind: document.Paragraph, Float: document.FloatLeft, FloatGroup: 1, FontScale: 160, Runs: []document.Run{{Text: "L"}}})
	doc.Add(document.Block{Kind: document.Paragraph, Runs: []document.Run{{Text: "orem ipsum dolor sit amet."}}})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if lines := pages[0].Lines; len(lines) != 1 || lines[0].Words[0].Text != "L" || lines[0].Words[1].Text != "orem" || lines[0].Start != 0 {
		t.Errorf("lines %+v, want one, the letter first", lines)
	}
}

// Floats of any kind, size and order lay out: the pages cover the document in order, the
// lines stay in the text, and no word overlaps an image.
func FuzzFloats(f *testing.F) {
	f.Add([]byte{1, 30, 0, 2, 0, 1, 0, 0, 3, 9, 2, 50, 1, 0, 0, 5})
	f.Add([]byte{3, 250, 0, 0, 0, 0, 3, 200, 0, 1, 2, 90})
	// Found by fuzzing: a box of two images taller than the page, a letter wider than its box,
	// a letter put back into a paragraph beside floats, drop caps over floats.
	for _, found := range []string{"0800000", "0\xd80200", "1070002\xddX10A", "2\xafX20B00X", "00X0\x01A2\xcbX20A"} {
		f.Add([]byte(found))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc := &document.Document{}
		words := strings.Fields(strings.Repeat("Il fallait partir avant l'aube, sans bruit ; personne ne devait le savoir. ", 3))
		for len(data) >= 3 && len(doc.Blocks) < 40 {
			kind, size, flags := data[0]%4, int(data[1]), data[2]
			data = data[3:]
			block := document.Block{Clear: document.Clear(flags % 4), FloatGroup: int(flags/4%3) + 1}
			if flags&0x10 != 0 {
				block.Float = document.Float(flags/32%2) + document.FloatLeft
			}
			switch kind {
			case 0:
				block.Kind, block.Image, block.ImageWidth, block.FloatWidth = document.Image, "picture.png", size%101, size%71
			case 1:
				block.Kind, block.Runs = document.Paragraph, []document.Run{{Text: strings.Join(words[:size%len(words)+1], " ")}}
			case 2:
				block.Kind, block.FontScale, block.LineHeight, block.Runs = document.Paragraph, 100+size*2, 100, []document.Run{{Text: string(rune('A' + size%26))}}
			default:
				block.Kind, block.Left, block.Right, block.Runs = document.Heading, size, size/2, []document.Run{{Text: "Titre " + strings.Repeat("long ", size%9)}}
			}
			doc.Add(block)
		}
		faces := newTestFaces(t)
		pages, err := Paginate(doc, testSettings, faces, squareImages)
		if err != nil {
			t.Fatal(err)
		}
		checkCoverage(t, pages, doc)
		content := testSettings.content()
		for _, page := range pages {
			for _, line := range page.Lines {
				left, right := lineEdges(faces, line)
				if left < content.Min.X || right > content.Max.X+1 {
					t.Fatalf("line %q from %d to %d, out of %v", lineText(line), left, right, content)
				}
				for _, word := range line.Words {
					// Down to its baseline: a drop cap's descender may hang lower.
					font, err := faces.Font(word.Style, word.Size)
					if err != nil {
						t.Fatal(err)
					}
					wordBox := image.Rect(word.X, word.Top, word.X+faces.width(word.format(), word.Text), word.Top+font.Ascent())
					for _, placed := range page.Images {
						if wordBox.Overlaps(placed.Rect) {
							t.Fatalf("word %q at %v over the image at %v", word.Text, wordBox, placed.Rect)
						}
					}
				}
			}
			for _, placed := range page.Images {
				if !placed.Rect.In(content) {
					t.Fatalf("image at %v, out of %v", placed.Rect, content)
				}
			}
		}
	})
}
