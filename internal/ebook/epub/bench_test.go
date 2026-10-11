package epub

import (
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
)

// benchmarkChapters is the size of the books of the benchmarks: a long novel's chapters.
const benchmarkChapters = 30

// screenSize is the InkPad 3's screen, which fixed-layout pages are drawn for.
var screenSize = image.Pt(1404, 1872)

// BenchmarkOpen opens and closes a book of 30 chapters, as the reader does each time it opens
// a book: the container, nothing of the chapters yet.
func BenchmarkOpen(b *testing.B) {
	bookPath := epubtest.Write(b, epubtest.Files(b, epubtest.Options{Chapters: benchmarkChapters}))
	b.ReportAllocs()
	for b.Loop() {
		book, err := Open(bookPath)
		if err != nil {
			b.Fatal(err)
		}
		if err := book.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBuildIndex indexes a book of 30 chapters, as a download does: every chapter read
// into a document and measured, the calls of notes found, the table of contents placed. Each
// iteration opens the book afresh, out of the timer, as the calls of notes are read once per
// open book.
func BenchmarkBuildIndex(b *testing.B) {
	bookPath := epubtest.Write(b, epubtest.Files(b, epubtest.Options{Chapters: benchmarkChapters}))
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		book, err := Open(bookPath)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		index, err := BuildIndex(book)
		if err != nil {
			b.Fatal(err)
		}
		if len(index.Chapters) != benchmarkChapters {
			b.Fatalf("%d chapters indexed, want %d", len(index.Chapters), benchmarkChapters)
		}
		b.StopTimer()
		if err := book.Close(); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

// BenchmarkRenderPage draws a fixed-layout page for the screen, as a download of a
// fixed-layout book does for each page: its XHTML read into regions, a full-page picture
// decoded and fitted, an SVG rasterized and a caption laid out over them.
func BenchmarkRenderPage(b *testing.B) {
	book, err := Open(epubtest.Write(b, epubtest.Files(b, epubtest.Options{Chapters: 3, FixedLayout: true})))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { book.Close() })
	if !book.FixedLayout() {
		b.Fatal("the book is not fixed-layout")
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := book.RenderPage(1, screenSize); err != nil {
			b.Fatal(err)
		}
	}
}
