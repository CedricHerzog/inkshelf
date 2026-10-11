// Package servertest is a servers.Server in memory, for the tests of the packages that use one.
package servertest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// ErrNotFound is the error of what the Fake doesn't have.
var ErrNotFound = errors.New("not found")

// Fake is a server whose content is its fields, set before it is used. It records the pages
// asked for and the progress sent.
type Fake struct {
	AllSeries     []servers.Series
	AllLibraries  []servers.Library
	BooksOfSeries map[string][]servers.Book // by series id
	Reading       []servers.Book            // the books in progress, the last read first
	PageImages    map[string][][]byte       // the pages of each book id, page 1 first
	Files         map[string][]byte         // the file of each book id, for OpenFile
	Thumbnails    map[string][]byte         // by series or book id

	mu           sync.Mutex                         // guards the fields below
	pageRequests []string                           // "<book id>:<number>" for each page asked for
	progress     map[string]servers.ReadingProgress // the last progress sent, by book id
}

// Fake implements servers.Server.
var _ servers.Server = (*Fake)(nil)

// PageRequests returns the pages asked for so far, as "<book id>:<number>", sorted.
func (fake *Fake) PageRequests() []string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return slices.Sorted(slices.Values(fake.pageRequests))
}

// SentProgress returns the last progress sent for book bookID, and false if there is none.
func (fake *Fake) SentProgress(bookID string) (servers.ReadingProgress, bool) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	progress, ok := fake.progress[bookID]
	return progress, ok
}

func (fake *Fake) Series(context.Context) ([]servers.Series, error) { return fake.AllSeries, nil }

func (fake *Fake) Libraries(context.Context) ([]servers.Library, error) {
	return fake.AllLibraries, nil
}

func (fake *Fake) Books(_ context.Context, seriesID string) ([]servers.Book, error) {
	books, ok := fake.BooksOfSeries[seriesID]
	if !ok {
		return nil, fmt.Errorf("series %s: %w", seriesID, ErrNotFound)
	}
	return books, nil
}

func (fake *Fake) InProgress(_ context.Context, count int) ([]servers.Book, error) {
	return fake.Reading[:min(count, len(fake.Reading))], nil
}

func (fake *Fake) OpenBook(_ context.Context, bookID string, _ int) (servers.Pages, error) {
	return fakePages{fake: fake, bookID: bookID}, nil
}

func (fake *Fake) OpenFile(_ context.Context, bookID string) (io.ReadCloser, int64, error) {
	data, ok := fake.Files[bookID]
	if !ok {
		return nil, 0, fmt.Errorf("file of %s: %w", bookID, ErrNotFound)
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

func (fake *Fake) FirstPage(_ context.Context, bookID string) ([]byte, error) {
	return fake.page(bookID, 1)
}

func (fake *Fake) SeriesThumbnail(_ context.Context, seriesID string) ([]byte, error) {
	return fake.thumbnail(seriesID)
}

func (fake *Fake) BookThumbnail(_ context.Context, bookID string) ([]byte, error) {
	return fake.thumbnail(bookID)
}

func (fake *Fake) SetProgress(_ context.Context, bookID string, page int, completed bool) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.progress == nil {
		fake.progress = map[string]servers.ReadingProgress{}
	}
	fake.progress[bookID] = servers.ReadingProgress{Page: page, Completed: completed}
	return nil
}

func (fake *Fake) ClearProgress(_ context.Context, bookID string) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	delete(fake.progress, bookID)
	return nil
}

// thumbnail returns the thumbnail of a series or a book.
func (fake *Fake) thumbnail(id string) ([]byte, error) {
	data, ok := fake.Thumbnails[id]
	if !ok {
		return nil, fmt.Errorf("thumbnail of %s: %w", id, ErrNotFound)
	}
	return data, nil
}

// page returns page number (counted from 1) of a book, and records that it was asked for.
func (fake *Fake) page(bookID string, number int) ([]byte, error) {
	fake.mu.Lock()
	fake.pageRequests = append(fake.pageRequests, fmt.Sprintf("%s:%d", bookID, number))
	fake.mu.Unlock()
	pages := fake.PageImages[bookID]
	if number < 1 || number > len(pages) {
		return nil, fmt.Errorf("page %d of %s: %w", number, bookID, ErrNotFound)
	}
	return pages[number-1], nil
}

// fakePages are the pages of a book of a Fake.
type fakePages struct {
	fake   *Fake
	bookID string
}

func (pages fakePages) Page(_ context.Context, number int) ([]byte, error) {
	return pages.fake.page(pages.bookID, number)
}
