package openbook

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// A book of pages may show two pages at once: a spread, drawn at download as one landscape
// image (shelf.Book.Spreads). Its reading position is then the spread's first page, and a turn
// moves past both pages. A book without spreads (every book downloaded before them) turns one
// page at a time, in portrait.

// spreadAt returns the first page of the spread that shows page of book, and whether a spread
// shows it.
func spreadAt(book shelf.Book, page int) (int, bool) {
	for _, first := range book.Spreads {
		if page == first || page == first+1 {
			return first, true
		}
	}
	return 0, false
}

// shownPage returns the position the screen shows page at: the first page of its spread, or the
// page itself. A position on a spread's second page (the server's, or an older version's) shows
// that spread.
func shownPage(book shelf.Book, page int) int {
	if first, ok := spreadAt(book, page); ok {
		return first
	}
	return page
}

// lastShown returns the last page the screen shows at position page: the spread's second page,
// or the page itself.
func lastShown(book shelf.Book, page int) int {
	if first, ok := spreadAt(book, page); ok {
		return first + 1
	}
	return page
}

// nextPosition and previousPosition return the positions a turn forward or back from page
// leads to, which may be past the book's ends.
func nextPosition(book shelf.Book, page int) int { return lastShown(book, page) + 1 }

func previousPosition(book shelf.Book, page int) int {
	if page <= 1 {
		return 0
	}
	return shownPage(book, page-1)
}

// loadShown loads what the screen shows at position page of book: the spread's image, or the
// page's.
func loadShown(store shelf.Store, book shelf.Book, page int) (*image.Gray, error) {
	if first, ok := spreadAt(book, page); ok && first == page {
		return store.LoadSpread(book.ID, page)
	}
	return store.LoadPage(book.ID, page)
}

// ShowsSpreadLocked reports whether the open book of pages shows a spread: the reader screen
// draws it in landscape.
func ShowsSpreadLocked(app *state.State) bool {
	if app.Reader.Text != nil {
		return app.Reader.Text.Book.AtSpread()
	}
	_, ok := spreadAt(app.Reader.OpenBook, app.Reader.OpenBook.Page)
	return ok
}
