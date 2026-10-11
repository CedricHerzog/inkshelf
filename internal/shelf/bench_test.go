package shelf

import (
	"bytes"
	"image"
	"image/jpeg"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
)

// The InkPad 3's screen, which the benchmarks prepare pages for.
const (
	screenWidth  = 1404
	screenHeight = 1872
)

// benchmarkComic is a book of 20 pages that benchmarkServer adds to newTestServer's.
var benchmarkComic = Book{ID: "comic", SeriesID: "s1", Title: "Comic", Pages: 20}

// benchmarkServer returns newTestServer's server with benchmarkComic, whose pages are JPEGs of
// 1200x1800 as a comic's scans are, and an EPUB of 30 chapters, "novel".
func benchmarkServer(b *testing.B) *servertest.Fake {
	b.Helper()
	server := newTestServer(b)
	server.PageImages[benchmarkComic.ID] = slices.Repeat([][]byte{scannedJPEG(b, image.Pt(1200, 1800))}, benchmarkComic.Pages)
	server.Files["novel"] = epubtest.Zip(b, epubtest.Files(b, epubtest.Options{Chapters: 30}))
	return server
}

// scannedJPEG returns a JPEG of size in bands of gray crossed by a grid: neither flat nor
// noise, so that it compresses and decodes as a scanned page would.
func scannedJPEG(b *testing.B, size image.Point) []byte {
	b.Helper()
	picture := image.NewGray(image.Rectangle{Max: size})
	for row := range size.Y {
		for column := range size.X {
			shade := uint8(255 - (row*160/size.Y + column*60/size.X))
			if row%50 < 3 || column%50 < 3 {
				shade /= 3
			}
			picture.Pix[row*picture.Stride+column] = shade
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, picture, &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	return encoded.Bytes()
}

// benchmarkDownload downloads book from server into store at each iteration, the book deleted
// out of the timer before the next, so that each iteration downloads it whole.
func benchmarkDownload(b *testing.B, server servers.Server, book Book) {
	store := Store{Dir: b.TempDir()}
	b.ReportAllocs()
	for b.Loop() {
		if err := store.Download(b.Context(), server, book, screenWidth, screenHeight, func(DownloadProgress) {}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := store.Delete(book.ID); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

// BenchmarkDownloadPages downloads a comic of 20 pages from a server in memory: each page
// fetched, decoded, fitted to the screen, dithered to 16 grays and written as a .pg4, by the
// workers of a real download. Its time is the wall-clock time of the download.
func BenchmarkDownloadPages(b *testing.B) {
	benchmarkDownload(b, benchmarkServer(b), benchmarkComic)
}

// BenchmarkDownloadEPUB downloads an EPUB of 30 chapters from a server in memory: the file
// written, the book opened and indexed (every chapter read into a document), its cover
// prepared for the screen and its index saved.
func BenchmarkDownloadEPUB(b *testing.B) {
	benchmarkDownload(b, benchmarkServer(b), Book{ID: "novel", Title: "A Test Book", Format: servers.Reflowable})
}

// BenchmarkLoadPage reads a page of the comic as the reader does to show it: the .pg4 read
// and unpacked to a gray image of the screen's size.
func BenchmarkLoadPage(b *testing.B) {
	store := Store{Dir: b.TempDir()}
	if err := store.Download(b.Context(), benchmarkServer(b), benchmarkComic, screenWidth, screenHeight, func(DownloadProgress) {}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		page, err := store.LoadPage(benchmarkComic.ID, 2)
		if err != nil {
			b.Fatal(err)
		}
		if page.Bounds().Dy() != screenHeight {
			b.Fatalf("page %v, want the screen's height", page.Bounds())
		}
	}
}
