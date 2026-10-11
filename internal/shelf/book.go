package shelf

import (
	"cmp"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/naturalorder"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// Book is a downloaded book and its reading progress, saved as book.json.
type Book struct {
	ID          string `json:"id"`
	SeriesID    string `json:"seriesId"`
	LibraryID   string `json:"libraryId"`
	SeriesTitle string `json:"seriesTitle"`
	Title       string `json:"title"`
	Pages       int    `json:"pages"`
	// Format is how the book was downloaded and is read: page images, or an EPUB to lay out.
	Format servers.Format `json:"format,omitempty"`
	// Number is the book's place in its series (Komga's numberSort).
	Number float64 `json:"number,omitempty"`
	// Direction is the series' reading direction, "" if the server doesn't say.
	Direction servers.Direction `json:"direction,omitempty"`
	// Page is the last page read, counted from 1.
	Page      int  `json:"page"`
	Completed bool `json:"completed"`
	// Unsynced is set while the progress has not been sent to the server yet.
	Unsynced bool `json:"dirty"`
	// PendingRemoval is set on a book to delete once its progress has been sent.
	PendingRemoval bool `json:"remove,omitempty"`
	// ReadAt is when the book was last read on the reader.
	ReadAt time.Time `json:"readAt,omitzero"`
	// Location is where a Reflowable book is read: Page only approximates it, as its pages are
	// laid out while reading. nil at the start.
	Location *Location `json:"location,omitempty"`
}

// Location is a position in a Reflowable book: the first rune of the page shown.
type Location struct {
	Chapter int `json:"chapter"` // counted from 0, in the book's spine
	Offset  int `json:"offset"`  // in runes from the chapter's start
}

// compareBooks orders books by series title as the series list sorts them (series of the same
// title by id, so that each one's books stay together), then by their place in the series:
// their number, or their title for books saved without one.
func compareBooks(left, right Book) int {
	if order := cmp.Or(servers.CompareTitles(left.SeriesTitle, right.SeriesTitle), strings.Compare(left.SeriesID, right.SeriesID)); order != 0 {
		return order
	}
	if left.Number != 0 && right.Number != 0 && left.Number != right.Number {
		return cmp.Compare(left.Number, right.Number)
	}
	return naturalorder.Compare(left.Title, right.Title)
}
