package container

import (
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
)

// benchmarkChapters is the size of the books of the benchmarks: a long novel's chapters.
const benchmarkChapters = 30

// BenchmarkOpen opens and closes a book of 30 chapters: the zip's directory, the container,
// the package document and its manifest and spine, the encryption check.
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

// BenchmarkNavigation reads the table of contents of a book of 30 chapters: its navigation
// document parsed and its links resolved, as the reader does when it opens the contents.
func BenchmarkNavigation(b *testing.B) {
	book, err := Open(epubtest.Write(b, epubtest.Files(b, epubtest.Options{Chapters: benchmarkChapters})))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { book.Close() })
	b.ReportAllocs()
	for b.Loop() {
		navigation, err := book.Navigation()
		if err != nil {
			b.Fatal(err)
		}
		if want := benchmarkChapters + 1; len(navigation.TOC) != want { // and the first chapter's note
			b.Fatalf("%d entries in the table of contents, want %d", len(navigation.TOC), want)
		}
	}
}
