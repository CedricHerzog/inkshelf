package calibreweb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Server is a Calibre-Web server as the app sees it: its series, plus one of the books in
// none, and its books, downloaded as EPUB or CBZ files. The reader keeps the read progress.
type Server struct {
	client *Client

	mu      sync.Mutex // guards catalog, used by the app's goroutines at once
	catalog *catalog   // of the last Series, nil before it
}

// Server implements servers.Server.
var _ servers.Server = (*Server)(nil)

// NewServer returns the Server of the server of config, which sends its requests through
// httpClient.
func NewServer(config servers.Config, httpClient *http.Client) *Server {
	return &Server{client: NewClient(config, httpClient)}
}

// Series reads the catalog again and returns its series, sorted by title, the books in no
// series last.
func (server *Server) Series(ctx context.Context) ([]servers.Series, error) {
	loaded, err := server.client.loadCatalog(ctx)
	if err != nil {
		return nil, err
	}
	server.mu.Lock()
	server.catalog = &loaded
	server.mu.Unlock()
	allSeries := make([]servers.Series, len(loaded.series))
	for index, series := range loaded.series {
		allSeries[index] = servers.Series{
			ID:        series.id,
			Title:     series.title,
			SortTitle: servers.SortKey(series.title),
			Direction: servers.LeftToRight,
			Books:     len(series.bookIDs),
		}
		for _, id := range series.bookIDs {
			if updated := loaded.books[id].updated; updated.After(allSeries[index].Modified) {
				allSeries[index].Modified = updated
			}
		}
	}
	return allSeries, nil
}

// Libraries is unsupported: the app shows Calibre-Web's library as one.
func (server *Server) Libraries(ctx context.Context) ([]servers.Library, error) {
	return nil, fmt.Errorf("calibreweb libraries: %w", errors.ErrUnsupported)
}

// Books returns the books of a series, in their order in it, numbered by their series index,
// else (a book in no series, or an entry without it) by their position.
func (server *Server) Books(ctx context.Context, seriesID string) ([]servers.Book, error) {
	known, err := server.knownCatalog(ctx)
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(known.series, func(series catalogSeries) bool { return series.id == seriesID })
	if index < 0 {
		return nil, fmt.Errorf("calibreweb: no series %s", seriesID)
	}
	series := known.series[index]
	books := make([]servers.Book, len(series.bookIDs))
	for position, id := range series.bookIDs {
		book := known.books[id]
		number, ok := series.numbers[id]
		if !ok {
			number = float64(position + 1)
		}
		books[position] = servers.Book{
			ID:          id,
			SeriesID:    series.id,
			SeriesTitle: series.title,
			Title:       book.title,
			Number:      number,
			Format:      book.format,
			Modified:    book.updated,
		}
	}
	return books, nil
}

// InProgress is unsupported: Calibre-Web keeps no reading position over OPDS.
func (server *Server) InProgress(ctx context.Context, count int) ([]servers.Book, error) {
	return nil, fmt.Errorf("calibreweb books in progress: %w", errors.ErrUnsupported)
}

// OpenBook is unsupported: Calibre-Web's books are downloaded as files, through OpenFile.
func (server *Server) OpenBook(ctx context.Context, bookID string, pageCount int) (servers.Pages, error) {
	return nil, fmt.Errorf("calibreweb pages: %w", errors.ErrUnsupported)
}

// OpenFile starts the download of a book's EPUB or CBZ.
func (server *Server) OpenFile(ctx context.Context, bookID string) (io.ReadCloser, int64, error) {
	book, err := server.book(ctx, bookID)
	if err != nil {
		return nil, 0, err
	}
	return server.client.File(ctx, book.file)
}

// FirstPage returns the cover of a book, as Calibre keeps it.
func (server *Server) FirstPage(ctx context.Context, bookID string) ([]byte, error) {
	return server.cover(ctx, bookID)
}

// SeriesThumbnail returns the cover of the series' first book.
func (server *Server) SeriesThumbnail(ctx context.Context, seriesID string) ([]byte, error) {
	known, err := server.knownCatalog(ctx)
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(known.series, func(series catalogSeries) bool { return series.id == seriesID })
	if index < 0 || len(known.series[index].bookIDs) == 0 {
		return nil, fmt.Errorf("calibreweb: no series %s", seriesID)
	}
	return server.cover(ctx, known.series[index].bookIDs[0])
}

// BookThumbnail returns the cover of a book: Calibre-Web's OPDS has no smaller one.
func (server *Server) BookThumbnail(ctx context.Context, bookID string) ([]byte, error) {
	return server.cover(ctx, bookID)
}

// SetProgress is unsupported: Calibre-Web keeps no reading position over OPDS.
func (server *Server) SetProgress(ctx context.Context, bookID string, page int, completed bool) error {
	return fmt.Errorf("calibreweb read progress: %w", errors.ErrUnsupported)
}

// ClearProgress is unsupported: Calibre-Web keeps no reading position over OPDS.
func (server *Server) ClearProgress(ctx context.Context, bookID string) error {
	return fmt.Errorf("calibreweb read progress: %w", errors.ErrUnsupported)
}

// cover returns the cover of a book.
func (server *Server) cover(ctx context.Context, bookID string) ([]byte, error) {
	book, err := server.book(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if book.cover == "" {
		return nil, fmt.Errorf("calibreweb: book %s has no cover", bookID)
	}
	return server.client.getBytes(ctx, book.cover)
}

// book returns a book of the catalog.
func (server *Server) book(ctx context.Context, bookID string) (catalogBook, error) {
	known, err := server.knownCatalog(ctx)
	if err != nil {
		return catalogBook{}, err
	}
	book, ok := known.books[bookID]
	if !ok {
		return catalogBook{}, fmt.Errorf("calibreweb: no book %s", bookID)
	}
	return book, nil
}

// knownCatalog returns the catalog of the last Series, or reads it if there is none yet.
func (server *Server) knownCatalog(ctx context.Context) (*catalog, error) {
	server.mu.Lock()
	known := server.catalog
	server.mu.Unlock()
	if known != nil {
		return known, nil
	}
	loaded, err := server.client.loadCatalog(ctx)
	if err != nil {
		return nil, err
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.catalog == nil {
		server.catalog = &loaded
	}
	return server.catalog, nil
}
