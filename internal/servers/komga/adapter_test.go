package komga

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// pagesServer is a Komga with book b1, of 4 pages: a PNG, a WebP, a JPEG and a GIF. It
// answers a page with "<number>:<conversion>".
type pagesServer struct {
	*httptest.Server
	mu       sync.Mutex // guards requests, added to by the server's goroutines
	requests []string   // "<number>:<conversion>" for each page asked for
}

// newPagesServer starts a pagesServer, closed at the end of the test.
func newPagesServer(t *testing.T) *pagesServer {
	t.Helper()
	server := &pagesServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/books/b1/pages", func(writer http.ResponseWriter, request *http.Request) {
		_, err := io.WriteString(writer, `[{"number":1,"mediaType":"image/png"},{"number":2,"mediaType":"image/webp"},`+
			`{"number":3,"mediaType":"image/jpeg"},{"number":4,"mediaType":"image/gif"}]`)
		if err != nil {
			t.Errorf("pages: %v", err)
		}
	})
	mux.HandleFunc("GET /api/v1/books/{id}/pages/{number}", func(writer http.ResponseWriter, request *http.Request) {
		answer := request.PathValue("number") + ":" + request.URL.Query().Get("convert")
		server.mu.Lock()
		server.requests = append(server.requests, answer)
		server.mu.Unlock()
		if _, err := io.WriteString(writer, answer); err != nil {
			t.Errorf("page: %v", err)
		}
	})
	server.Server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// fetchAll opens book bookID of pageCount pages and fetches each of them.
func (server *pagesServer) fetchAll(t *testing.T, bookID string, pageCount int) []string {
	t.Helper()
	komga := NewServer(servers.Config{Name: "test", Server: server.URL, APIKey: "key"}, server.Client())
	pages, err := komga.OpenBook(t.Context(), bookID, pageCount)
	if err != nil {
		t.Fatal(err)
	}
	for index := range pageCount {
		if _, err := pages.Page(t.Context(), index+1); err != nil {
			t.Fatal(err)
		}
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	return slices.Clone(server.requests)
}

// JPEG, PNG and WebP pages are fetched as they are, the others converted to PNG.
func TestOpenBookConversions(t *testing.T) {
	got := newPagesServer(t).fetchAll(t, "b1", 4)
	if want := []string{"1:", "2:", "3:", "4:png"}; !slices.Equal(got, want) {
		t.Fatalf("pages fetched %v, want %v", got, want)
	}
}

// Without the page list, every page is converted to JPEG.
func TestOpenBookWithoutPageList(t *testing.T) {
	got := newPagesServer(t).fetchAll(t, "b2", 2)
	if want := []string{"1:jpeg", "2:jpeg"}; !slices.Equal(got, want) {
		t.Fatalf("pages fetched %v, want %v", got, want)
	}
}

func TestSourceSeriesAndBooks(t *testing.T) {
	server := newTestServer(t)
	komga := NewServer(servers.Config{Name: "test", Server: server.URL, APIKey: "key"}, server.Client())
	allSeries, err := komga.Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(allSeries) != 1 {
		t.Fatalf("%d series, want 1", len(allSeries))
	}
	series := allSeries[0]
	if series.Title != "One Piece" || series.SortTitle != "" || series.Direction != servers.RightToLeft || series.Status() != servers.Reading {
		t.Errorf("series %+v", series)
	}
	books, err := komga.Books(t.Context(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("%d books, want 1", len(books))
	}
	book := books[0]
	if book.Title != "Vol 1" || book.Pages != 3 || book.Progress == nil || book.Progress.Page != 2 {
		t.Errorf("book %+v, progress %+v", book, book.Progress)
	}
}
