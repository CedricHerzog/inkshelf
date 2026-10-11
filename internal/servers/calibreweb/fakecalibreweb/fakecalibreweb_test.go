package fakecalibreweb

import (
	"archive/zip"
	"bytes"
	"errors"
	"image/jpeg"
	"io"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// newServer returns the Server of a new fake Calibre-Web.
func newServer(t *testing.T) *calibreweb.Server {
	t.Helper()
	server := New()
	t.Cleanup(server.Close)
	config := servers.Config{Name: "Books", Type: "calibreweb", Server: server.URL, Username: "me", Password: "secret"}
	return calibreweb.NewServer(config, server.Client())
}

// The series are sorted by title, the books in none last; the PDF is left out.
func TestSeries(t *testing.T) {
	allSeries, err := newServer(t).Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, series := range allSeries {
		got = append(got, series.Title)
		if series.Direction != servers.LeftToRight {
			t.Errorf("%s reads %q, want left to right", series.Title, series.Direction)
		}
	}
	if want := []string{"Les Étoiles", "Ruines", "No series"}; !slices.Equal(got, want) {
		t.Fatalf("series %q, want %q", got, want)
	}
	if allSeries[0].SortTitle != "Etoiles" || allSeries[0].Books != 3 || allSeries[2].Books != 3 {
		t.Errorf("series %+v", allSeries)
	}
	if !allSeries[0].Modified.Equal(day(8, 20)) {
		t.Errorf("Les Étoiles modified %v, want its last book's date", allSeries[0].Modified)
	}
}

// A series' books come in their order in it, with their format and their series index (Ruines
// lacks its second book), the books in no series numbered by position; a new Server reads the
// catalog itself.
func TestBooks(t *testing.T) {
	server := newServer(t)
	tests := []struct {
		seriesID string
		titles   []string
		numbers  []float64
		format   servers.Format
	}{
		{"1", []string{"Les Étoiles, Tome 1", "Les Étoiles, Tome 2", "Les Étoiles, Tome 3"}, []float64{1, 2, 3}, servers.Reflowable},
		{"2", []string{"Ruines, Tome 1", "Ruines, Tome 3"}, []float64{1, 3}, servers.ComicArchive},
		{"no-series", []string{"Contes courts", "Le voyage illustré", "World scripts"}, []float64{1, 2, 3}, servers.Reflowable},
	}
	for _, test := range tests {
		books, err := server.Books(t.Context(), test.seriesID)
		if err != nil {
			t.Fatal(err)
		}
		var titles []string
		var numbers []float64
		for _, book := range books {
			titles = append(titles, book.Title)
			numbers = append(numbers, book.Number)
			if book.Format != test.format || book.SeriesID != test.seriesID || book.Pages != 0 {
				t.Errorf("book %+v", book)
			}
		}
		if !slices.Equal(titles, test.titles) {
			t.Errorf("series %s: books %q, want %q", test.seriesID, titles, test.titles)
		}
		if !slices.Equal(numbers, test.numbers) {
			t.Errorf("series %s: numbers %v, want %v", test.seriesID, numbers, test.numbers)
		}
	}
}

func TestOpenFile(t *testing.T) {
	server := newServer(t)
	for _, test := range []struct {
		bookID string
		file   string // a file the archive must hold
	}{
		{"21", "Ruines, Tome 1/page 8.jpg"},
		{"11", "OEBPS/content.opf"},
	} {
		body, size, err := server.OpenFile(t.Context(), test.bookID)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if size != int64(len(data)) {
			t.Errorf("book %s: size %d, read %d bytes", test.bookID, size, len(data))
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(archive.File, func(file *zip.File) bool { return file.Name == test.file }) {
			t.Errorf("book %s has no %s", test.bookID, test.file)
		}
	}
}

func TestCovers(t *testing.T) {
	server := newServer(t)
	for _, get := range []func() ([]byte, error){
		func() ([]byte, error) { return server.FirstPage(t.Context(), "11") },
		func() ([]byte, error) { return server.BookThumbnail(t.Context(), "22") },
		func() ([]byte, error) { return server.SeriesThumbnail(t.Context(), "no-series") },
	} {
		data, err := get()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			t.Error(err)
		}
	}
}

// Calibre-Web keeps no progress over OPDS: the reader keeps it.
func TestNoProgress(t *testing.T) {
	server := newServer(t)
	if _, err := server.InProgress(t.Context(), 5); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("InProgress: %v, want ErrUnsupported", err)
	}
	if err := server.SetProgress(t.Context(), "11", 3, false); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("SetProgress: %v, want ErrUnsupported", err)
	}
}

func TestNeedsLogin(t *testing.T) {
	server := New()
	defer server.Close()
	config := servers.Config{Name: "Books", Server: server.URL}
	_, err := calibreweb.NewServer(config, server.Client()).Series(t.Context())
	var statusErr *httpclient.StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != 401 {
		t.Fatalf("error %v, want a 401", err)
	}
}

// A CBZ downloads whole: its pages prepared, its page count saved.
func TestDownloadCBZ(t *testing.T) {
	server := newServer(t)
	books, err := server.Books(t.Context(), "2")
	if err != nil {
		t.Fatal(err)
	}
	book := shelf.Book{ID: books[1].ID, SeriesID: "2", Title: books[1].Title, Format: books[1].Format}
	store := shelf.Store{Dir: t.TempDir()}
	if err := store.Download(t.Context(), server, book, 702, 936, func(shelf.DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	saved, ok := store.Book(book.ID)
	if !ok || saved.Pages != 6 {
		t.Fatalf("saved %+v, %v; want 6 pages", saved, ok)
	}
	page, err := store.LoadPage(book.ID, 6)
	if err != nil {
		t.Fatal(err)
	}
	if size := page.Bounds().Size(); size.X != 624 || size.Y != 936 {
		t.Errorf("page 6 is %v, want 624x936", size)
	}
}

// An EPUB downloads with its index and cover, its virtual pages counted.
func TestDownloadEPUB(t *testing.T) {
	server := newServer(t)
	books, err := server.Books(t.Context(), "1")
	if err != nil {
		t.Fatal(err)
	}
	book := shelf.Book{ID: books[0].ID, SeriesID: "1", Title: books[0].Title, Format: books[0].Format}
	store := shelf.Store{Dir: t.TempDir()}
	preparing := false
	if err := store.Download(t.Context(), server, book, 702, 936, func(progress shelf.DownloadProgress) {
		preparing = preparing || progress.Preparing
	}); err != nil {
		t.Fatal(err)
	}
	saved, ok := store.Book(book.ID)
	if !ok || saved.Pages < 2 || saved.Format != servers.Reflowable {
		t.Fatalf("saved %+v, %v; want an EPUB of several virtual pages", saved, ok)
	}
	if !preparing {
		t.Error("the progress never said Preparing")
	}
	opened, err := epub.Open(store.EPUBPath(book.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	index, err := store.LoadIndex(book.ID, opened)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Chapters) != 4 || len(index.TOC) != 7 || len(index.PageList) != 3*printedPagesPerChapter || len(index.Landmarks) != 1 || (index.Total+shelf.CharsPerPage-1)/shelf.CharsPerPage != saved.Pages {
		t.Errorf("index %+v for %d pages", index, saved.Pages)
	}
	cover, err := store.LoadFirstPage(saved)
	if err != nil {
		t.Fatal(err)
	}
	if size := cover.Bounds().Size(); size.Y != 936 {
		t.Errorf("cover %v, want it 936 high", size)
	}
}
