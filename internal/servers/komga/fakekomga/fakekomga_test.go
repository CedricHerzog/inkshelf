package fakekomga

import (
	"net/http"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/komga"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// fakeConfig starts a fake Komga, closed at the end of the test, and returns its config and
// HTTP client.
func fakeConfig(t *testing.T) (servers.Config, *http.Client) {
	t.Helper()
	fake := New()
	t.Cleanup(fake.Close)
	return servers.Config{Name: "fake", Server: fake.URL, APIKey: "key"}, fake.Client()
}

// newClient starts a fake Komga and returns a client of it.
func newClient(t *testing.T) *komga.Client {
	t.Helper()
	return komga.NewClient(fakeConfig(t))
}

// newServer starts a fake Komga and returns its servers.Server.
func newServer(t *testing.T) *komga.Server {
	t.Helper()
	return komga.NewServer(fakeConfig(t))
}

// listSeries returns the fake's series, by id.
func listSeries(t *testing.T, client *komga.Client) map[string]komga.Series {
	t.Helper()
	allSeries, err := client.Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSeries) != len(catalog) {
		t.Fatalf("%d series, want %d", len(allSeries), len(catalog))
	}
	byID := map[string]komga.Series{}
	for _, item := range allSeries {
		byID[item.ID] = item
	}
	return byID
}

// alphaBooks returns the books of series alpha.
func alphaBooks(t *testing.T, client *komga.Client) []komga.Book {
	t.Helper()
	books, err := client.Books(t.Context(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 3 {
		t.Fatalf("%d books, want 3", len(books))
	}
	return books
}

func TestSeriesOrder(t *testing.T) {
	allSeries, err := newClient(t).Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first := allSeries[0]; first.Title() != "Alpha Saga" {
		t.Errorf("first series %q, want Alpha Saga", first.Title())
	}
	if last := allSeries[len(allSeries)-1]; last.Title() != "The Zeta" || last.Metadata.TitleSort != "Zeta, The" {
		t.Errorf("last series %q sorted as %q, want The Zeta as Zeta, The", last.Title(), last.Metadata.TitleSort)
	}
}

func TestSeriesFields(t *testing.T) {
	series := listSeries(t, newClient(t))
	if series["beta"].BooksCount != 25 {
		t.Errorf("beta has %d books, want 25", series["beta"].BooksCount)
	}
	if series["delta"].LibraryID != "bd" {
		t.Errorf("delta is in library %q, want bd", series["delta"].LibraryID)
	}
	if series["alpha"].Modified().IsZero() {
		t.Errorf("no lastModified: %q", series["alpha"].LastModified)
	}
}

func TestSeriesStatuses(t *testing.T) {
	series := listSeries(t, newClient(t))
	tests := []struct {
		id   string
		want servers.ReadStatus
	}{
		{id: "alpha", want: servers.Reading},
		{id: "beta", want: servers.Reading},
		{id: "epsilon", want: servers.Read},
		{id: "kappa", want: servers.Unread},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			item := series[test.id]
			if got := servers.SeriesStatus(item.BooksCount, item.BooksReadCount, item.BooksInProgressCount); got != test.want {
				t.Errorf("status %d, want %d", got, test.want)
			}
		})
	}
}

func TestSeriesDirections(t *testing.T) {
	series := listSeries(t, newClient(t))
	tests := []struct {
		id   string
		want servers.Direction
	}{
		{id: "alpha", want: servers.RightToLeft},
		{id: "beta", want: ""},
		{id: "delta", want: servers.LeftToRight},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			if got := series[test.id].Direction(); got != test.want {
				t.Errorf("direction %q, want %q", got, test.want)
			}
		})
	}
}

func TestLibraries(t *testing.T) {
	libraries, err := newClient(t).Libraries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 3 || libraries[0].Name != "BD" {
		t.Errorf("Libraries = %+v, want 3, BD first", libraries)
	}
}

func TestBooks(t *testing.T) {
	books := alphaBooks(t, newClient(t))
	first := books[0]
	if first.LibraryID != "manga" {
		t.Errorf("library %q, want manga", first.LibraryID)
	}
	if first.Media.PagesCount != 10 {
		t.Errorf("%d pages, want 10", first.Media.PagesCount)
	}
	if first.SeriesTitle != "Alpha Saga" {
		t.Errorf("series title %q", first.SeriesTitle)
	}
	if first.Modified().IsZero() {
		t.Errorf("no lastModified: %q", first.LastModified)
	}
}

func TestBooksStartingProgress(t *testing.T) {
	books := alphaBooks(t, newClient(t))
	if books[0].ReadProgress != nil {
		t.Errorf("book 1 progress %+v, want unread", books[0].ReadProgress)
	}
	if books[1].ReadProgress == nil || books[1].ReadProgress.Page != 4 {
		t.Errorf("book 2 progress %+v, want page 4", books[1].ReadProgress)
	}
}

func TestCovers(t *testing.T) {
	fake := newServer(t)
	covers := shelf.CoverFiles{Dir: t.TempDir()}
	tests := []struct {
		name     string
		kind     shelf.CoverKind
		id       string
		wantFail bool
	}{
		{name: "book", kind: shelf.BookCover, id: "alpha-01"},
		{name: "series", kind: shelf.SeriesCover, id: "alpha"},
		{name: "series without covers", kind: shelf.SeriesCover, id: "gamma", wantFail: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := covers.Fetch(t.Context(), fake, test.kind, test.id, 140, 210, time.Time{})
			if test.wantFail && err == nil {
				t.Error("fetched a cover that doesn't exist")
			}
			if !test.wantFail && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestSetProgress(t *testing.T) {
	client := newClient(t)
	if err := client.SetProgress(t.Context(), "alpha-01", 7, false); err != nil {
		t.Fatal(err)
	}
	if books := alphaBooks(t, client); books[0].ReadProgress == nil || books[0].ReadProgress.Page != 7 {
		t.Errorf("progress after SetProgress = %+v, want page 7", books[0].ReadProgress)
	}
}

// The book read last comes first in the books in progress.
func TestInProgress(t *testing.T) {
	client := newClient(t)
	if err := client.SetProgress(t.Context(), "alpha-01", 7, false); err != nil {
		t.Fatal(err)
	}
	reading, err := client.InProgress(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(reading) != 1 || reading[0].ID != "alpha-01" {
		t.Errorf("InProgress = %+v, want alpha-01 only", reading)
	}
}

func TestClearProgress(t *testing.T) {
	client := newClient(t)
	if err := client.ClearProgress(t.Context(), "alpha-02"); err != nil {
		t.Fatal(err)
	}
	if books := alphaBooks(t, client); books[1].ReadProgress != nil {
		t.Errorf("progress after ClearProgress = %+v, want none", books[1].ReadProgress)
	}
}

// A whole book downloads, its WebP page converted.
func TestDownload(t *testing.T) {
	store := shelf.Store{Dir: t.TempDir()}
	book := shelf.Book{ID: "delta-01", SeriesID: "delta", SeriesTitle: "Delta", Title: "Delta, Vol. 1", Pages: 5, Page: 1}
	if err := store.Download(t.Context(), newServer(t), book, 702, 936, func(shelf.DownloadProgress) {}); err != nil {
		t.Fatal(err)
	}
	for number := range upTo(book.Pages) {
		img, err := store.LoadPage(book.ID, number)
		if err != nil {
			t.Fatalf("page %d: %v", number, err)
		}
		if img.Bounds().Dy() != 936 {
			t.Errorf("page %d: size %v, want 936 high", number, img.Bounds())
		}
	}
}
