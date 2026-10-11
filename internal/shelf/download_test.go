package shelf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
)

// testBook is the book of newTestServer.
var testBook = Book{ID: "b1", SeriesID: "s1", SeriesTitle: "One Piece", Title: "Vol 1", Pages: 3, Page: 1}

// downloadTestBook downloads testBook from a new test server into a new store, fitting its
// pages into 100x100.
func downloadTestBook(t *testing.T) (*servertest.Fake, Store) {
	t.Helper()
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), server, testBook, 100, 100, func(DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	return server, store
}

func TestDownloadReportsAllPages(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	done := 0
	if err := store.Download(t.Context(), server, testBook, 100, 100, func(progress DownloadProgress) { done = progress.Done }); err != nil {
		t.Fatal(err)
	}
	if done != testBook.Pages {
		t.Fatalf("progress ends at %d pages, want %d", done, testBook.Pages)
	}
}

// Each page is fetched once.
func TestDownloadFetchesEachPage(t *testing.T) {
	server, _ := downloadTestBook(t)
	if got, want := server.PageRequests(), []string{"b1:1", "b1:2", "b1:3"}; !slices.Equal(got, want) {
		t.Fatalf("pages fetched %v, want %v", got, want)
	}
}

func TestDownloadSavesBook(t *testing.T) {
	_, store := downloadTestBook(t)
	if got := store.Books(); len(got) != 1 || got[0] != testBook {
		t.Fatalf("List = %+v, want the book downloaded", got)
	}
}

// The pages are fitted to the screen: 200x100 into 100x100 gives 100x50, left half white,
// right half black.
func TestDownloadPreparesPages(t *testing.T) {
	_, store := downloadTestBook(t)
	for _, number := range []int{2, 3} { // the WebP and a PNG
		img, err := store.LoadPage(testBook.ID, number)
		if err != nil {
			t.Fatal(err)
		}
		if bounds := img.Bounds(); bounds.Dx() != 100 || bounds.Dy() != 50 {
			t.Fatalf("page %d size %v, want 100x50", number, bounds)
		}
		if left, right := img.GrayAt(10, 25).Y, img.GrayAt(90, 25).Y; left < 240 || right > 15 {
			t.Fatalf("page %d pixels %d, %d; want white, black", number, left, right)
		}
	}
}

// A download resumes: the pages already on disk are not fetched again.
func TestDownloadSkipsPagesOnDisk(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.bookDir(testBook.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.pagePath(testBook.ID, 1, pngExtension), encodePNG(t, gradient(4, 4)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Download(t.Context(), server, testBook, 100, 100, func(DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	if got, want := server.PageRequests(), []string{"b1:2", "b1:3"}; !slices.Equal(got, want) {
		t.Fatalf("pages fetched %v, want %v", got, want)
	}
}

// The server has no pages for b2: the download fails and leaves no book.
func TestDownloadMissingBook(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), server, Book{ID: "b2", Pages: 10}, 100, 100, func(DownloadProgress) {}); err == nil {
		t.Fatal("the download of a missing book succeeded")
	}
	if _, ok := store.Book("b2"); ok {
		t.Fatal("a failed download saved book.json")
	}
}

func TestDownloadNoPages(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), newTestServer(t), Book{ID: "b1"}, 100, 100, func(DownloadProgress) {}); err == nil {
		t.Fatal("the download of a book without pages succeeded")
	}
}

func TestDelete(t *testing.T) {
	_, store := downloadTestBook(t)
	if err := store.Delete(testBook.ID); err != nil {
		t.Fatal(err)
	}
	if books := store.Books(); len(books) != 0 {
		t.Fatalf("%d books left", len(books))
	}
}

// Progress only grows and ends at the book's page count, even with a slow receiver.
func TestDownloadProgress(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	book := Book{ID: "alpha-01", Pages: 10}
	var reported []int
	err := store.Download(t.Context(), server, book, 140, 187, func(progress DownloadProgress) {
		time.Sleep(20 * time.Millisecond) // as long as a refresh of the panel
		reported = append(reported, progress.Done)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reported) == 0 || reported[len(reported)-1] != book.Pages {
		t.Fatalf("progress %v, want it to end at %d", reported, book.Pages)
	}
	for index := range len(reported) - 1 {
		if reported[index+1] <= reported[index] {
			t.Fatalf("progress %v does not grow", reported)
		}
	}
}

// A cancelled download leaves no book.
func TestDownloadCancelled(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	book := Book{ID: "alpha-01", Pages: 10}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := store.Download(ctx, server, book, 140, 187, func(DownloadProgress) { cancel() }); err == nil {
		t.Fatal("a cancelled download succeeded")
	}
	if _, ok := store.Book(book.ID); ok {
		t.Fatal("a cancelled download saved book.json")
	}
}

// cbzBook is a book of newTestServer downloaded as a comic book archive.
var cbzBook = Book{ID: "c1", SeriesID: "s1", Title: "Comic", Format: servers.ComicArchive}

// A comic book archive is fetched, its pages prepared, its page count saved and the archive
// removed.
func TestDownloadArchive(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	var last DownloadProgress
	if err := store.Download(t.Context(), server, cbzBook, 100, 100, func(progress DownloadProgress) { last = progress }); err != nil {
		t.Fatal(err)
	}
	saved, ok := store.Book(cbzBook.ID)
	if !ok || saved.Pages != 3 || saved.Format != servers.ComicArchive {
		t.Fatalf("saved %+v, %v; want 3 pages of a comic book archive", saved, ok)
	}
	if last != (DownloadProgress{Done: 3, Pages: 3}) {
		t.Errorf("last progress %+v, want 3 of 3 pages", last)
	}
	for number := range 3 {
		if _, err := store.LoadPage(cbzBook.ID, number+1); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.bookDir(cbzBook.ID), archiveFile)); !os.IsNotExist(err) {
		t.Errorf("the archive is still there: %v", err)
	}
}

// An archive fetched by an interrupted download is not fetched again.
func TestDownloadArchiveResumes(t *testing.T) {
	server := newTestServer(t)
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.bookDir(cbzBook.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.bookDir(cbzBook.ID), archiveFile), server.Files[cbzBook.ID], 0o644); err != nil {
		t.Fatal(err)
	}
	delete(server.Files, cbzBook.ID)
	if err := store.Download(t.Context(), server, cbzBook, 100, 100, func(DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Book(cbzBook.ID); !ok {
		t.Fatal("the book was not saved")
	}
}

// A file that is no zip fails the download, and leaves no book.
func TestDownloadArchiveBroken(t *testing.T) {
	server := newTestServer(t)
	server.Files[cbzBook.ID] = []byte("not a zip")
	store := Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), server, cbzBook, 100, 100, func(DownloadProgress) {}); err == nil {
		t.Fatal("the download of a broken archive succeeded")
	}
	if _, ok := store.Book(cbzBook.ID); ok {
		t.Fatal("a failed download saved book.json")
	}
}

// An EPUB protected by DRM fails its download, and leaves neither book nor file.
func TestDownloadEPUBWithDRM(t *testing.T) {
	server := newTestServer(t)
	server.Files["e1"] = zipFiles(t, map[string][]byte{
		"META-INF/container.xml":  []byte(`<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`),
		"META-INF/encryption.xml": []byte(`<encryption><EncryptedData><EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/></EncryptedData></encryption>`),
	})
	store := Store{Dir: t.TempDir()}
	book := Book{ID: "e1", Title: "Protected", Format: servers.Reflowable}
	if err := store.Download(t.Context(), server, book, 100, 100, func(DownloadProgress) {}); !errors.Is(err, epub.ErrDRM) {
		t.Fatalf("Download = %v, want ErrDRM", err)
	}
	if _, ok := store.Book(book.ID); ok {
		t.Error("a failed download saved book.json")
	}
	if _, err := os.Stat(store.EPUBPath(book.ID)); !os.IsNotExist(err) {
		t.Errorf("the protected EPUB was kept: %v", err)
	}
}
