package layout

import (
	"fmt"
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// boxedDocument returns paragraphs, those from first to last (counted from 0) in box.
func boxedDocument(paragraphs, first, last int, box document.BorderBox) *document.Document {
	doc := &document.Document{}
	for index := range paragraphs {
		if index == first {
			box.Start = doc.Chars
		}
		block := document.Block{MarginTop: 50, MarginBottom: 50, Runs: []document.Run{{Text: fmt.Sprintf("Paragraph%d is here.", index)}}}
		if index >= first && index <= last {
			block.Left, block.Right = 100, 100
		}
		doc.Add(block)
		if index == last {
			box.End = doc.Chars
		}
	}
	doc.Boxes = append(doc.Boxes, box)
	return doc
}

// solid returns a solid border of width, in gray.
func solid(width int, gray uint8) document.Border {
	return document.Border{Style: document.Solid, Width: width, Gray: gray}
}

// A box goes around its blocks: its border's edges from the text's margins, its padding between
// its border and its text, its background under the text; its borders and background are drawn.
func TestBox(t *testing.T) {
	box := document.BorderBox{
		Top: solid(25, 0), Bottom: solid(25, 0), Left: solid(25, 0x40), Right: document.Border{Style: document.Dashed, Width: 25},
		PaddingTop: 50, PaddingBottom: 50, MarginTop: 100, MarginBottom: 100, LeftEdge: 25, RightEdge: 25,
		Background: 0xD0, Filled: true,
	}
	faces := newTestFaces(t)
	page := paginateOne(t, boxedDocument(4, 1, 2, box), leftSettings, faces, noImages)
	if len(page.Boxes) != 1 {
		t.Fatalf("boxes %+v, want 1", page.Boxes)
	}
	placed, content := page.Boxes[0], leftSettings.content()
	inside, after := wordAt(t, page, "Paragraph1"), wordAt(t, page, "Paragraph3")
	before := wordAt(t, page, "Paragraph0")
	if placed.Rect.Min.X != content.Min.X+6 || placed.Rect.Max.X != content.Max.X-6 {
		t.Errorf("box from x %d to %d, want %d to %d", placed.Rect.Min.X, placed.Rect.Max.X, content.Min.X+6, content.Max.X-6)
	}
	// The top border, 6 pixels, then 12 of padding and the block's margin, 12; the text 24 in,
	// the box 6.
	if inside.Top != placed.Rect.Min.Y+6+12+12 || inside.X != content.Min.X+24 {
		t.Errorf("text at %d,%d in a box at %v", inside.X, inside.Top, placed.Rect)
	}
	if placed.Rect.Min.Y <= before.Top+leftSettings.FontSize || after.Top < placed.Rect.Max.Y+24 {
		t.Errorf("box %v between lines at %d and %d: the margins", placed.Rect, before.Top, after.Top)
	}
	if placed.Top.Width != 6 || placed.Bottom.Width != 6 || placed.Left.Gray != 0x40 || placed.Right.Style != document.Dashed {
		t.Errorf("borders %+v", placed)
	}
	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, noImageLoader); err != nil {
		t.Fatal(err)
	}
	middle := (placed.Rect.Min.Y + placed.Rect.Max.Y) / 2
	for _, pixel := range []struct {
		x, y int
		want uint8
	}{
		{placed.Rect.Min.X + 2, middle, 0x40},              // the left border
		{placed.Rect.Max.X - 20, placed.Rect.Min.Y + 2, 0}, // the top one
		{placed.Rect.Max.X - 20, placed.Rect.Min.Y + 8, 0xD0},
		{placed.Rect.Min.X - 2, middle, 0xFF},
	} {
		if gray := canvas.GrayAt(pixel.x, pixel.y).Y; gray != pixel.want {
			t.Errorf("pixel %d,%d: %#x, want %#x", pixel.x, pixel.y, gray, pixel.want)
		}
	}
}

// noImageLoader loads no image.
func noImageLoader(string, image.Point) (*image.Gray, error) { return nil, fmt.Errorf("no images") }

// A box without a top border nor padding takes its first block's top margin, as CSS collapses
// them; one with a border keeps it inside.
func TestBoxMargins(t *testing.T) {
	faces := newTestFaces(t)
	plain := paginateOne(t, boxedDocument(2, 1, 1, document.BorderBox{Filled: true, Background: 0xE0}), leftSettings, faces, noImages)
	unboxed := paginateOne(t, boxedDocument(2, 1, 1, document.BorderBox{}), leftSettings, faces, noImages)
	if got, want := wordAt(t, plain, "Paragraph1").Top, wordAt(t, unboxed, "Paragraph1").Top; got != want {
		t.Errorf("text at %d in a box of a background only, want %d as without", got, want)
	}
	if box := plain.Boxes[0]; box.Rect.Min.Y != wordAt(t, plain, "Paragraph1").Top {
		t.Errorf("background from %d, want the text's top %d", box.Rect.Min.Y, wordAt(t, plain, "Paragraph1").Top)
	}
	bordered := paginateOne(t, boxedDocument(2, 1, 1, document.BorderBox{Top: solid(25, 0)}), leftSettings, faces, noImages)
	if box, text := bordered.Boxes[0], wordAt(t, bordered, "Paragraph1"); text.Top != box.Rect.Min.Y+6+12 {
		t.Errorf("text at %d in a box at %d: its margin inside the border", text.Top, box.Rect.Min.Y)
	}
}

// Boxes inside one another are drawn the outer first, inside it.
func TestNestedBoxes(t *testing.T) {
	doc := boxedDocument(3, 0, 2, document.BorderBox{Left: solid(25, 0), Right: solid(25, 0), Top: solid(25, 0), Bottom: solid(25, 0), PaddingTop: 50, Background: 0xC0, Filled: true})
	doc.Boxes = append(doc.Boxes, document.BorderBox{Start: doc.Blocks[1].Start, End: doc.Blocks[2].Start, LeftEdge: 50, RightEdge: 50, Top: solid(25, 0x60), Bottom: solid(25, 0x60), Background: 0xF0, Filled: true})
	doc.Blocks[1].Left, doc.Blocks[1].Right = 150, 150
	faces := newTestFaces(t)
	page := paginateOne(t, doc, leftSettings, faces, noImages)
	if len(page.Boxes) != 2 {
		t.Fatalf("boxes %+v, want 2", page.Boxes)
	}
	outer, inner := page.Boxes[1], page.Boxes[0] // the inner closes first
	if outer.Depth != 0 || inner.Depth != 1 || !inner.Rect.In(outer.Rect) || inner.Rect.Min.X != outer.Rect.Min.X+12 {
		t.Errorf("outer %+v, inner %+v", outer, inner)
	}
	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, noImageLoader); err != nil {
		t.Fatal(err)
	}
	if gray := canvas.GrayAt(inner.Rect.Max.X-2, inner.Rect.Min.Y+10).Y; gray != 0xF0 {
		t.Errorf("inner background %#x, want it over the outer's", gray)
	}
}

// A box split across pages has its top border on its first page, its bottom one on its last,
// and its sides on each; its text is as it would be without it.
func TestBoxAcrossPages(t *testing.T) {
	box := document.BorderBox{Top: solid(25, 0), Bottom: solid(25, 0), Left: solid(25, 0), Right: solid(25, 0), Background: 0xE0, Filled: true}
	doc := boxedDocument(40, 2, 37, box)
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 3 {
		t.Fatalf("%d pages, want 3 at least", len(pages))
	}
	content := testSettings.content()
	for index, page := range pages {
		if len(page.Boxes) != 1 {
			t.Fatalf("page %d: boxes %+v", index, page.Boxes)
		}
		placed := page.Boxes[0]
		first, last := index == 0, index == len(pages)-1
		if (placed.Top.Width > 0) != first || (placed.Bottom.Width > 0) != last || placed.Left.Width == 0 || placed.Right.Width == 0 {
			t.Errorf("page %d: borders %+v", index, placed)
		}
		if !first && placed.Rect.Min.Y != content.Min.Y || !last && placed.Rect.Max.Y != content.Max.Y {
			t.Errorf("page %d: box %v, want it from the top or to the bottom of %v", index, placed.Rect, content)
		}
		for _, line := range page.Lines {
			if line.Words[0].Top < content.Min.Y || line.Words[0].Top+testSettings.FontSize > content.Max.Y {
				t.Errorf("page %d: a line at %d out of %v", index, line.Words[0].Top, content)
			}
		}
	}
}

// A box that would only show its top border at the bottom of a page starts on the next.
func TestBoxMovesWhole(t *testing.T) {
	faces := newTestFaces(t)
	lineHeight := testSettings.FontSize * testSettings.LineSpacing / 100
	for paragraphs := 1; paragraphs < 20; paragraphs++ {
		doc := boxedDocument(paragraphs+1, paragraphs, paragraphs, document.BorderBox{Top: solid(25, 0), PaddingTop: 150})
		pages, err := Paginate(doc, testSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		for index, page := range pages {
			for _, placed := range page.Boxes {
				if len(page.Lines) == 0 || page.Lines[len(page.Lines)-1].Words[0].Top+lineHeight < placed.Rect.Min.Y {
					t.Errorf("%d paragraphs: page %d has a box at %v under its text", paragraphs, index, placed.Rect)
				}
			}
		}
	}
}

// A box whose blocks are all left out is not drawn; its offsets stay.
func TestBoxOfHiddenNote(t *testing.T) {
	doc := boxedDocument(3, 2, 2, document.BorderBox{Top: solid(25, 0)})
	doc.Blocks[0].Runs = append(doc.Blocks[0].Runs, document.Run{Text: "1", Link: "chapter.xhtml#note"})
	doc.Blocks[2].Footnote = "note"
	settings := testSettings
	settings.HideFootnotes = true
	page := paginateOne(t, doc, settings, newTestFaces(t), noImages)
	if len(page.Boxes) != 0 {
		t.Errorf("boxes %+v of a hidden note", page.Boxes)
	}
}

// Text is drawn in its run's gray, its underline too.
func TestTextGray(t *testing.T) {
	faces := newTestFaces(t)
	page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "black "}, {Text: "grayed", Gray: 0x60, Style: document.Underline}}}), leftSettings, faces, noImages)
	if word := wordAt(t, page, "grayed"); word.Gray != 0x60 || page.Strokes[0].Gray != 0x60 {
		t.Errorf("word %+v, underline %+v", word, page.Strokes[0])
	}
	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, noImageLoader); err != nil {
		t.Fatal(err)
	}
	darkest := uint8(0xFF)
	word := wordAt(t, page, "grayed")
	for y := word.Top; y < word.Top+leftSettings.FontSize; y++ {
		for x := word.X; x < word.X+60; x++ {
			darkest = min(darkest, canvas.GrayAt(x, y).Y)
		}
	}
	if darkest != 0x60 {
		t.Errorf("darkest pixel of gray text %#x, want 0x60", darkest)
	}
}

// Borders of each style draw something in their side, and nothing out of it.
func TestBorderStyles(t *testing.T) {
	for _, style := range []document.BorderStyle{document.Solid, document.Double, document.Dotted, document.Dashed} {
		canvas := whiteCanvas(leftSettings)
		side := image.Rect(10, 10, 300, 19)
		drawSide(canvas, side, document.Border{Style: style, Width: 9}, true)
		inked, outside := 0, 0
		for y := range 40 {
			for x := range 320 {
				if canvas.GrayAt(x, y).Y == 0 {
					if image.Pt(x, y).In(side) {
						inked++
					} else {
						outside++
					}
				}
			}
		}
		if inked == 0 || outside > 0 || style != document.Solid && inked == side.Dx()*side.Dy() {
			t.Errorf("style %d: %d pixels inked in its side, %d out", style, inked, outside)
		}
	}
}

// Plain text lays out as fast with boxes in the code as without.
func BenchmarkPaginateBoxes(b *testing.B) {
	doc := testDocument(60)
	doc.Boxes = []document.BorderBox{{Start: doc.Blocks[3].Start, End: doc.Blocks[40].Start, Left: solid(25, 0), Right: solid(25, 0), Background: 0xE0, Filled: true}}
	faces := newTestFaces(b)
	for b.Loop() {
		if _, err := Paginate(doc, testSettings, faces, noImages); err != nil {
			b.Fatal(err)
		}
	}
}
