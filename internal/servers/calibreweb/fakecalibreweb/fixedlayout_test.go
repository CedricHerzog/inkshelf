package fakecalibreweb

import (
	"os"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// A fixed-layout EPUB downloads as page images, one per spine item, fitted in the screen:
// 1200x1800 into 702x936 gives 624x936. The EPUB is not kept.
func TestDownloadFixedLayout(t *testing.T) {
	server := newServer(t)
	books, err := server.Books(t.Context(), "no-series")
	if err != nil {
		t.Fatal(err)
	}
	book := shelf.Book{ID: books[1].ID, SeriesID: "no-series", Title: books[1].Title, Format: books[1].Format}
	store := shelf.Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), server, book, 702, 936, func(shelf.DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	saved, ok := store.Book(book.ID)
	if !ok || saved.Pages != 3 || saved.Format != servers.ComicArchive || saved.Direction != servers.LeftToRight {
		t.Fatalf("saved %+v, %v; want 3 pages of a comic, left to right", saved, ok)
	}
	if _, err := os.Stat(store.EPUBPath(book.ID)); !os.IsNotExist(err) {
		t.Errorf("the EPUB was kept: %v", err)
	}
	for index := range 3 {
		page, err := store.LoadPage(book.ID, index+1)
		if err != nil {
			t.Fatal(err)
		}
		if size := page.Bounds().Size(); size.X != 624 || size.Y != 936 {
			t.Errorf("page %d is %v, want 624x936", index+1, size)
		}
	}
	// Page 3, the SVG: its dark circle, centred at (600, 700) of 1200x1800.
	page, err := store.LoadPage(book.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if center := page.GrayAt(312, 364).Y; center > 128 {
		t.Errorf("the centre of the SVG's circle is %d, want it dark", center)
	}
}
