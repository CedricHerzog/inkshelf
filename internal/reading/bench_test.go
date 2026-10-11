package reading

import (
	"image"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// benchmarkFontSize is the font size of the benchmarks, the app's default.
const benchmarkFontSize = 44

// screenSettings lays text out at fontSize as the app does on the InkPad 3's screen, its
// margins and line spacing those of package openbook.
func screenSettings(fontSize int) layout.Settings {
	return layout.Settings{
		Width: 1404, Height: 1872, Margins: image.Rect(90, 80, 90, 100),
		FontSize: fontSize, LineSpacing: 135, Justify: true, HideFootnotes: true,
	}
}

// storeBenchmarkBook puts a book of 30 chapters of epubtest in a new store, as a download
// leaves it but without its index, which the first Open builds and saves. It silences the
// log, where the renderer writes a line for each chapter laid out and each page drawn.
func storeBenchmarkBook(b *testing.B) (shelf.Store, shelf.Book) {
	b.Helper()
	logOutput := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(logOutput) })
	store := shelf.Store{Dir: b.TempDir()}
	book := shelf.Book{ID: "novel", Title: "A Test Book", Format: servers.Reflowable, Pages: 1}
	epubPath := store.EPUBPath(book.ID)
	if err := os.MkdirAll(filepath.Dir(epubPath), 0o755); err != nil {
		b.Fatal(err)
	}
	data := epubtest.Zip(b, epubtest.Files(b, epubtest.Options{Chapters: 30}))
	if err := os.WriteFile(epubPath, data, 0o644); err != nil {
		b.Fatal(err)
	}
	return store, book
}

// openBenchmarkBook opens the book of storeBenchmarkBook, closed with the benchmark.
func openBenchmarkBook(b *testing.B) *TextBook {
	b.Helper()
	store, book := storeBenchmarkBook(b)
	text, err := Open(store, book, screenSettings(benchmarkFontSize))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(text.Close)
	return text
}

// BenchmarkOpenTextBook opens and closes a book of 30 chapters, as the reader does when a book
// is tapped: the EPUB opened, its index read from disk (built by the first iteration), the
// first chapter laid out to find the first page. Nothing is drawn.
func BenchmarkOpenTextBook(b *testing.B) {
	store, book := storeBenchmarkBook(b)
	b.ReportAllocs()
	for b.Loop() {
		text, err := Open(store, book, screenSettings(benchmarkFontSize))
		if err != nil {
			b.Fatal(err)
		}
		text.Close()
	}
}

// BenchmarkTurnPage turns a page forward and draws it, through the second chapter (one of
// prose only) and back to its start: the work of a page turn in the engine, the chapter laid
// out once and the page drawn each time, without the refresh of the screen.
func BenchmarkTurnPage(b *testing.B) {
	text := openBenchmarkBook(b)
	start := shelf.Location{Chapter: 1}
	if err := text.GoTo(start); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		next, _, err := text.Next(text.At())
		if err != nil {
			b.Fatal(err)
		}
		if next.Chapter == start.Chapter {
			_, err = text.Turn(true)
		} else {
			err = text.GoTo(start) // the chapter is still laid out: back to its first page
		}
		if err != nil {
			b.Fatal(err)
		}
		if _, err := text.Draw(text.At()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRelayout changes the text size back and forth in the middle of a chapter of prose:
// the layouts dropped and the chapter of the place read laid out again at the new size.
func BenchmarkRelayout(b *testing.B) {
	text := openBenchmarkBook(b)
	location := shelf.Location{Chapter: 1, Offset: 10000}
	if err := text.GoTo(location); err != nil {
		b.Fatal(err)
	}
	fontSizes := []int{benchmarkFontSize + 8, benchmarkFontSize}
	turn := 0
	b.ReportAllocs()
	for b.Loop() {
		if err := text.Relayout(screenSettings(fontSizes[turn%len(fontSizes)]), location); err != nil {
			b.Fatal(err)
		}
		turn++
	}
}
