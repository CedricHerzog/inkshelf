package state

import (
	"context"
	"slices"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// SeriesList is the series list. Guarded by the mutex.
type SeriesList struct {
	Items      []SeriesItem
	Page       int       // the list's page to go back to from the books of a series
	Resume     *BookItem // the book to continue, first in the list; nil if there is none
	Refreshing bool      // the list is being read again from the server
	JumpOpen   bool      // the A–Z jump shows over the list
}

// SeriesItem is a series of the list, and how far it is read.
type SeriesItem struct {
	ID        string
	Title     string
	LibraryID string
	Direction servers.Direction // "" if the server doesn't say
	Modified  time.Time         // on the server, to know if its cover is stale
	Books     int               // books on the server
	Local     int               // books downloaded on the reader
	Read      int               // books read to the end
	Reading   int               // books started and not finished
	Initial   string            // the letter of the A–Z jump it is filed under
}

// Status returns how far the series is read. Offline, the server's count of books is unknown,
// so it is how far its downloaded books are read.
func (item SeriesItem) Status() servers.ReadStatus {
	books := item.Books
	if books == 0 {
		books = item.Local
	}
	return servers.SeriesStatus(books, item.Read, item.Reading)
}

// BookList is the books of the open series. Guarded by the mutex.
type BookList struct {
	SeriesID  string // of the open series; "" on the series list
	Title     string // of the open series
	Items     []BookItem
	Focus     string // the ID of a book whose page of the list the next Draw turns to
	Browsed   bool   // the list was turned or filtered since the series opened
	MenuOpen  bool   // the hold menu of book MenuIndex shows over the list
	MenuIndex int    // the book of the hold menu, an index in Items
}

// BookItem is a book of a list.
type BookItem struct {
	shelf.Book // with the server's progress, or the reader's while it is ahead
	Downloaded bool
	Modified   time.Time // on the server, to know if its cover is stale
}

// ListView is the list shown, series or books. Guarded by the mutex.
type ListView struct {
	Page   int    // the page shown, from 0
	Rows   int    // rows on a page, which drawing the list sets; 0 before it is first drawn
	Notice string // shown in the list's header in place of the count, when not ""
}

// DownloadQueue is the books to download, one at a time. Guarded by the mutex.
type DownloadQueue struct {
	Downloading string                 // the ID of the book downloading, "" when the queue is idle
	Progress    shelf.DownloadProgress // how far its download is
	LastRepaint time.Time              // when its progress was last redrawn
	Cancel      context.CancelFunc     // stops its download
	Waiting     []shelf.Book           // to download after it, in order
}

// Waits reports whether book id waits in the queue.
func (queue *DownloadQueue) Waits(id string) bool {
	return slices.ContainsFunc(queue.Waiting, func(book shelf.Book) bool { return book.ID == id })
}

// OnItsWay reports whether book id is downloading or waits to.
func (queue *DownloadQueue) OnItsWay(id string) bool {
	return id == queue.Downloading || queue.Waits(id)
}
