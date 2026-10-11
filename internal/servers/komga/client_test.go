package komga

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/httpclient"
)

// testServer is a Komga with one series of one book, which accepts the API key "key".
type testServer struct {
	*httptest.Server
	mu       sync.Mutex     // guards progress, set by the server's goroutines
	progress map[string]any // the last progress sent, nil once cleared
}

// sentProgress returns the last progress sent.
func (server *testServer) sentProgress() map[string]any {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.progress
}

// newTestServer starts a testServer, closed at the end of the test.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	server := &testServer{}
	mux := http.NewServeMux()
	handleText := func(pattern, body string) {
		mux.HandleFunc(pattern, func(writer http.ResponseWriter, request *http.Request) {
			if _, err := io.WriteString(writer, body); err != nil {
				t.Errorf("%s: %v", pattern, err)
			}
		})
	}
	handleText("GET /api/v1/series", `{"content":[{"id":"s1","libraryId":"l1","name":"dir","booksCount":1,`+
		`"booksReadCount":0,"booksInProgressCount":1,"lastModified":"2023-03-24T13:44:15Z",`+
		`"metadata":{"title":"One Piece","readingDirection":"RIGHT_TO_LEFT"}}]}`)
	handleText("GET /api/v1/libraries", `[{"id":"l1","name":"Manga","root":"/manga"},{"id":"l2","name":"BD","root":"/bd"}]`)
	handleText("GET /api/v1/series/s1/books", `{"content":[{"id":"b1","seriesId":"s1","libraryId":"l1",`+
		`"seriesTitle":"One Piece","name":"v01","lastModified":"2023-03-25T08:00:00","metadata":{"title":"Vol 1"},`+
		`"media":{"pagesCount":3},"readProgress":{"page":2,"completed":false}}]}`)
	handleText("GET /api/v1/series/s1/thumbnail", "series cover")
	handleText("GET /api/v1/books/b1/thumbnail", "book cover")
	mux.HandleFunc("PATCH /api/v1/books/b1/read-progress", func(writer http.ResponseWriter, request *http.Request) {
		var progress map[string]any
		if err := json.NewDecoder(request.Body).Decode(&progress); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		server.mu.Lock()
		server.progress = progress
		server.mu.Unlock()
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/v1/books/b1/read-progress", func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.progress = nil
		server.mu.Unlock()
		writer.WriteHeader(http.StatusNoContent)
	})
	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-API-Key") != "key" {
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(writer, request)
	}))
	t.Cleanup(server.Close)
	return server
}

// client returns a client of the server with the API key apiKey.
func (server *testServer) client(apiKey string) *Client {
	return NewClient(servers.Config{Name: "test", Server: server.URL, APIKey: apiKey}, server.Client())
}

func TestClientSeries(t *testing.T) {
	series, err := newTestServer(t).client("key").Series(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 {
		t.Fatalf("%d series, want 1", len(series))
	}
	first := series[0]
	if first.Title() != "One Piece" {
		t.Errorf("title %q", first.Title())
	}
	if first.LibraryID != "l1" {
		t.Errorf("library %q", first.LibraryID)
	}
	if first.Direction() != servers.RightToLeft {
		t.Errorf("direction %q, want %q", first.Direction(), servers.RightToLeft)
	}
	if want := time.Date(2023, 3, 24, 13, 44, 15, 0, time.UTC); !first.Modified().Equal(want) {
		t.Errorf("modified %v, want %v", first.Modified(), want)
	}
}

func TestClientLibraries(t *testing.T) {
	libraries, err := newTestServer(t).client("key").Libraries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(libraries) != 2 || libraries[0] != (servers.Library{ID: "l1", Name: "Manga"}) {
		t.Fatalf("Libraries = %+v", libraries)
	}
}

func TestClientBooks(t *testing.T) {
	books, err := newTestServer(t).client("key").Books(t.Context(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("%d books, want 1", len(books))
	}
	book := books[0]
	if book.Title() != "Vol 1" {
		t.Errorf("title %q", book.Title())
	}
	if book.LibraryID != "l1" {
		t.Errorf("library %q", book.LibraryID)
	}
	if book.Media.PagesCount != 3 {
		t.Errorf("%d pages, want 3", book.Media.PagesCount)
	}
	if book.ReadProgress == nil || book.ReadProgress.Page != 2 {
		t.Errorf("progress %+v, want page 2", book.ReadProgress)
	}
	// A date without a time zone is taken as UTC.
	if want := time.Date(2023, 3, 25, 8, 0, 0, 0, time.UTC); !book.Modified().Equal(want) {
		t.Errorf("modified %v, want %v", book.Modified(), want)
	}
}

func TestClientThumbnails(t *testing.T) {
	client := newTestServer(t).client("key")
	if cover, err := client.SeriesThumbnail(t.Context(), "s1"); err != nil || string(cover) != "series cover" {
		t.Errorf("SeriesThumbnail = %q, %v", cover, err)
	}
	if cover, err := client.BookThumbnail(t.Context(), "b1"); err != nil || string(cover) != "book cover" {
		t.Errorf("BookThumbnail = %q, %v", cover, err)
	}
}

func TestClientMissingThumbnail(t *testing.T) {
	_, err := newTestServer(t).client("key").BookThumbnail(t.Context(), "b2")
	var statusError *httpclient.StatusError
	if !errors.As(err, &statusError) || statusError.Code != http.StatusNotFound {
		t.Fatalf("error %v, want a StatusError 404", err)
	}
}

func TestClientSetProgress(t *testing.T) {
	server := newTestServer(t)
	if err := server.client("key").SetProgress(t.Context(), "b1", 3, true); err != nil {
		t.Fatal(err)
	}
	if progress := server.sentProgress(); progress["page"] != 3.0 || progress["completed"] != true {
		t.Fatalf("progress sent %v, want page 3 completed", progress)
	}
}

func TestClientClearProgress(t *testing.T) {
	server := newTestServer(t)
	if err := server.client("key").SetProgress(t.Context(), "b1", 2, false); err != nil {
		t.Fatal(err)
	}
	if err := server.client("key").ClearProgress(t.Context(), "b1"); err != nil {
		t.Fatal(err)
	}
	if progress := server.sentProgress(); progress != nil {
		t.Fatalf("progress %v after ClearProgress", progress)
	}
}

func TestClientWrongKey(t *testing.T) {
	_, err := newTestServer(t).client("wrong").Series(t.Context())
	var statusError *httpclient.StatusError
	if !errors.As(err, &statusError) || statusError.Code != http.StatusUnauthorized {
		t.Fatalf("error %v, want a StatusError 401", err)
	}
	if want := "GET /api/v1/series?unpaged=true&sort=metadata.titleSort,asc: 401 Unauthorized Unauthorized"; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

func TestSeriesDirection(t *testing.T) {
	tests := []struct {
		readingDirection string
		want             servers.Direction
	}{
		{readingDirection: "RIGHT_TO_LEFT", want: servers.RightToLeft},
		{readingDirection: "LEFT_TO_RIGHT", want: servers.LeftToRight},
		{readingDirection: "VERTICAL", want: servers.LeftToRight},
		{readingDirection: "WEBTOON", want: servers.LeftToRight},
		{readingDirection: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.readingDirection, func(t *testing.T) {
			series := Series{Metadata: SeriesMetadata{ReadingDirection: test.readingDirection}}
			if got := series.Direction(); got != test.want {
				t.Errorf("Direction = %q, want %q", got, test.want)
			}
		})
	}
}
