package komga

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Server is a Komga server as the app sees it: Client's calls, with Komga's series and books
// turned into the servers package's.
type Server struct {
	client *Client
}

// Server implements servers.Server.
var _ servers.Server = (*Server)(nil)

// NewServer returns the Server of the server of config, which sends its requests through
// httpClient.
func NewServer(config servers.Config, httpClient *http.Client) *Server {
	return &Server{client: NewClient(config, httpClient)}
}

// Series returns all the series, sorted by title.
func (server *Server) Series(ctx context.Context) ([]servers.Series, error) {
	remoteSeries, err := server.client.Series(ctx)
	if err != nil {
		return nil, err
	}
	allSeries := make([]servers.Series, len(remoteSeries))
	for index, remote := range remoteSeries {
		allSeries[index] = servers.Series{
			ID:              remote.ID,
			LibraryID:       remote.LibraryID,
			Title:           remote.Title(),
			SortTitle:       remote.Metadata.TitleSort,
			Direction:       remote.Direction(),
			Modified:        remote.Modified(),
			Books:           remote.BooksCount,
			BooksRead:       remote.BooksReadCount,
			BooksInProgress: remote.BooksInProgressCount,
		}
	}
	return allSeries, nil
}

// Libraries returns the libraries the API key can see.
func (server *Server) Libraries(ctx context.Context) ([]servers.Library, error) {
	return server.client.Libraries(ctx)
}

// Books returns the books of a series, sorted by number.
func (server *Server) Books(ctx context.Context, seriesID string) ([]servers.Book, error) {
	return neutralBooks(server.client.Books(ctx, seriesID))
}

// InProgress returns at most count books started and not finished, the last read first.
func (server *Server) InProgress(ctx context.Context, count int) ([]servers.Book, error) {
	return neutralBooks(server.client.InProgress(ctx, count))
}

// neutralBooks returns Komga's books as servers.Books, or err.
func neutralBooks(remoteBooks []Book, err error) ([]servers.Book, error) {
	if err != nil {
		return nil, err
	}
	books := make([]servers.Book, len(remoteBooks))
	for index, remote := range remoteBooks {
		books[index] = servers.Book{
			ID:          remote.ID,
			SeriesID:    remote.SeriesID,
			LibraryID:   remote.LibraryID,
			SeriesTitle: remote.SeriesTitle,
			Title:       remote.Title(),
			Number:      remote.Metadata.NumberSort,
			Pages:       remote.Media.PagesCount,
			Modified:    remote.Modified(),
		}
		if progress := remote.ReadProgress; progress != nil {
			books[index].Progress = &servers.ReadingProgress{Page: progress.Page, Completed: progress.Completed}
		}
	}
	return books, nil
}

// OpenBook returns the pages of a book, each to be fetched in the format pageConversions
// chooses for it.
func (server *Server) OpenBook(ctx context.Context, bookID string, pageCount int) (servers.Pages, error) {
	return bookPages{
		client:      server.client,
		bookID:      bookID,
		conversions: pageConversions(ctx, server.client, bookID, pageCount),
	}, nil
}

// OpenFile is unsupported: Komga books are downloaded page by page.
func (server *Server) OpenFile(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, fmt.Errorf("komga book files: %w", errors.ErrUnsupported)
}

// FirstPage returns page 1 of a book, as Komga converts it to JPEG.
func (server *Server) FirstPage(ctx context.Context, bookID string) ([]byte, error) {
	return server.client.Page(ctx, bookID, 1, ConvertToJPEG)
}

// SeriesThumbnail returns the cover of a series, as Komga made it (JPEG, about 300 pixels wide).
func (server *Server) SeriesThumbnail(ctx context.Context, seriesID string) ([]byte, error) {
	return server.client.SeriesThumbnail(ctx, seriesID)
}

// BookThumbnail returns the cover of a book, as Komga made it (JPEG, about 300 pixels wide).
func (server *Server) BookThumbnail(ctx context.Context, bookID string) ([]byte, error) {
	return server.client.BookThumbnail(ctx, bookID)
}

// SetProgress marks page as the last one read of a book, and the book as completed or not.
func (server *Server) SetProgress(ctx context.Context, bookID string, page int, completed bool) error {
	return server.client.SetProgress(ctx, bookID, page, completed)
}

// ClearProgress marks a book unread.
func (server *Server) ClearProgress(ctx context.Context, bookID string) error {
	return server.client.ClearProgress(ctx, bookID)
}

// bookPages are the pages of a book being downloaded from Komga.
type bookPages struct {
	client *Client
	bookID string
	// conversions[number] is the format to ask Komga for page number.
	conversions []Conversion
}

// Page fetches page number, converted as conversions says.
func (pages bookPages) Page(ctx context.Context, number int) ([]byte, error) {
	conversion := ConvertToJPEG
	if number < len(pages.conversions) {
		conversion = pages.conversions[number]
	}
	return pages.client.Page(ctx, pages.bookID, number, conversion)
}

// pageConversions returns the format to ask Komga for each page of a book of pageCount pages,
// by page number (index 0 is unused). JPEG, PNG and WebP pages come as they are, much smaller
// than Komga's PNG of them; the other formats as PNG, which loses nothing.
func pageConversions(ctx context.Context, client *Client, bookID string, pageCount int) []Conversion {
	conversions := slices.Repeat([]Conversion{ConvertToJPEG}, pageCount+1)
	pages, err := client.Pages(ctx, bookID)
	if err != nil {
		// Not fatal: Komga can convert any page to JPEG, so ask for that for every page. If
		// Komga cannot be reached, fetching the pages fails anyway.
		return conversions
	}
	for _, info := range pages {
		if info.Number < 1 || info.Number > pageCount {
			continue
		}
		switch info.MediaType {
		case "image/jpeg", "image/png", "image/webp":
			conversions[info.Number] = Original
		default:
			conversions[info.Number] = ConvertToPNG
		}
	}
	return conversions
}
