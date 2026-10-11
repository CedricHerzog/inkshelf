package layout

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// paragraphsWith returns the paragraphs of testDocument(paragraphs), with between adding blocks
// to the document after each one: images, floats.
func paragraphsWith(paragraphs int, between func(doc *document.Document, index int)) *document.Document {
	doc := &document.Document{}
	for index, block := range testDocument(paragraphs).Blocks[1:] {
		doc.Add(block)
		between(doc, index)
	}
	return doc
}

// benchmarkImages is an ImageSizer of wide and tall illustrations, by the first letter of the
// path.
func benchmarkImages(path string) (image.Point, error) {
	if strings.HasPrefix(path, "wide") {
		return image.Pt(1600, 900), nil
	}
	return image.Pt(800, 2400), nil
}

// A chapter of each kind of content that costs more than text, on the InkPad 3's screen:
// compare each with BenchmarkPaginateChapter, the same paragraphs alone.
func BenchmarkPaginateContent(b *testing.B) {
	withImages := paragraphsWith(60, func(doc *document.Document, index int) {
		if index%6 == 5 {
			path := "tall.png"
			if index%12 == 5 {
				path = "wide.png"
			}
			doc.Add(document.Block{Kind: document.Image, Image: path})
		}
	})
	withFloats := paragraphsWith(60, func(doc *document.Document, index int) {
		if index%4 == 0 {
			side := document.FloatLeft
			if index%8 == 4 {
				side = document.FloatRight
			}
			doc.Add(document.Block{Kind: document.Image, Image: "picture.png", ImageWidth: 100, Float: side, FloatGroup: index + 1, FloatWidth: 30})
		}
	})
	cell := "A cell of a few words, that wraps on narrow columns"
	withTable, _ := tableDocument(40, 4, document.AllEdges, func(row, column int) string {
		if row == 0 {
			return fmt.Sprintf("Column %d", column)
		}
		return fmt.Sprintf("%s, row %d.", cell, row)
	})
	bidi := &document.Document{}
	for index := range 30 {
		bidi.Add(document.Block{Direction: document.RightToLeft, Indent: 150, Runs: []document.Run{
			{Text: fmt.Sprintf("الفقرة %d. كان علينا أن نغادر قبل الفجر، دون ضجيج؛ ", index+1)},
			{Text: "the road wound through the hills", Style: document.Italic},
			{Text: "، ولم يكن أحد يعرف ذلك، ولا حتى طحان القرية المجاورة الذي كان يسهر متأخرا. "},
		}})
		bidi.Add(document.Block{Indent: 150, Runs: []document.Run{
			{Text: "In the margin, a reader had written "},
			{Text: "שלום עולם ומה שלומך היום"},
			{Text: " and the year 1987, then crossed it out."},
		}})
	}
	for _, test := range []struct {
		name  string
		doc   *document.Document
		sizer ImageSizer
	}{
		{"images", withImages, benchmarkImages},
		{"floats", withFloats, squareImages},
		{"table", withTable, noImages},
		{"bidi", bidi, noImages},
	} {
		b.Run(test.name, func(b *testing.B) {
			faces := newTestFaces(b)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Paginate(test.doc, benchmarkSettings, faces, test.sizer); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
