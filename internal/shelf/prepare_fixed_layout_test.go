package shelf

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/epubtest"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
)

// spreadTestSize is the screen of the spread tests, in portrait: small, so that they run fast.
var spreadTestSize = image.Pt(300, 400)

// A fixed-layout book is drawn page by page, and its spreads (pages 2 and 3, 4 and 5: the first
// page is alone) as landscape images, which book.json lists.
func TestDownloadFixedLayoutSpreads(t *testing.T) {
	data := epubtest.Zip(t, epubtest.Files(t, epubtest.Options{Chapters: 5, FixedLayout: true}))
	server := &servertest.Fake{Files: map[string][]byte{"f1": data}}
	store := Store{Dir: t.TempDir()}
	book := Book{ID: "f1", Title: "Pictures", Format: servers.Reflowable}
	if err := store.Download(t.Context(), server, book, spreadTestSize.X, spreadTestSize.Y, func(DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	saved, ok := store.Book(book.ID)
	if !ok || saved.Format != servers.ComicArchive || saved.Pages != 5 || !slices.Equal(saved.Spreads, []int{2, 4}) {
		t.Fatalf("saved %+v, want 5 pages with spreads at 2 and 4", saved)
	}
	for _, first := range saved.Spreads {
		spread, err := store.LoadSpread(book.ID, first)
		if err != nil {
			t.Fatal(err)
		}
		if size := spread.Bounds().Size(); size != image.Pt(spreadTestSize.Y, spreadTestSize.X) {
			t.Errorf("spread %d is %v, want the screen in landscape", first, size)
		}
	}
	if _, err := os.Stat(store.spreadPath(book.ID, 1)); !os.IsNotExist(err) {
		t.Errorf("the first page, alone, has a spread: %v", err)
	}
}

func BenchmarkRenderSpread(b *testing.B) {
	path := epubtest.Write(b, epubtest.Files(b, epubtest.Options{Chapters: 3, FixedLayout: true}))
	opened, err := epub.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer opened.Close()
	store := Store{Dir: b.TempDir()}
	if err := os.MkdirAll(store.bookDir("f1"), 0o755); err != nil {
		b.Fatal(err)
	}
	spread := epub.Spread{First: 1, Left: 1, Right: 2}
	b.ReportAllocs()
	for b.Loop() {
		if err := store.renderSpread(opened, "f1", spread, image.Pt(1404, 1872)); err != nil {
			b.Fatal(err)
		}
	}
}

// A fixed-layout book downloaded before spreads, its book.json without them, reads as it did:
// its pages one by one, and no spread.
func TestFixedLayoutWithoutSpreads(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.bookDir("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"id":"old","seriesId":"","libraryId":"","seriesTitle":"","title":"Old","pages":2,"format":1,"page":1,"completed":false,"dirty":false}`
	if err := os.WriteFile(filepath.Join(store.bookDir("old"), bookFile), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	for number := range 2 {
		if err := os.WriteFile(store.pagePath("old", number+1, pg4Extension), pageimage.PrepareDecoded(gradient(300, 400), 300, 400), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	book, ok := store.Book("old")
	if !ok || book.Spreads != nil {
		t.Fatalf("book %+v, %v; want it without spreads", book, ok)
	}
	for number := range 2 {
		if _, err := store.LoadPage("old", number+1); err != nil {
			t.Error(err)
		}
	}
	if _, err := store.LoadSpread("old", 1); err == nil {
		t.Error("a spread loaded for a book without spreads")
	}
}
