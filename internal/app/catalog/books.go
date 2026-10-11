package catalog

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// BookItemLocked returns a book of the server as the lists show it: with the server's
// progress, or the reader's while the reader is ahead.
func BookItemLocked(app *state.State, book servers.Book) state.BookItem {
	var direction servers.Direction
	if index := slices.IndexFunc(app.Series.Items, func(item state.SeriesItem) bool { return item.ID == book.SeriesID }); index >= 0 {
		direction = app.Series.Items[index].Direction
	}
	item := state.BookItem{
		ID:          book.ID,
		SeriesID:    book.SeriesID,
		LibraryID:   book.LibraryID,
		SeriesTitle: book.SeriesTitle,
		Title:       book.Title,
		Pages:       book.Pages,
		Format:      book.Format,
		Number:      book.Number,
		Direction:   direction,
		Modified:    book.Modified,
	}
	if progress := book.Progress; progress != nil {
		item.Page = progress.Page
		item.Completed = progress.Completed
	}
	if local, ok := app.Store.Book(book.ID); ok {
		item.Downloaded = true
		item.ReadAt = local.ReadAt
		// As it was downloaded: a fixed-layout EPUB is downloaded as pages.
		item.Format = cmp.Or(local.Format, item.Format)
		// The page count of a comic book archive is known once it is downloaded.
		item.Pages = cmp.Or(item.Pages, local.Pages)
		if local.Unsynced || app.Server.LocalProgress { // read offline and not sent yet: the reader is ahead
			item.Page = local.Page
			item.Completed = local.Completed
		}
	}
	return item
}

// FetchBooks replaces the downloaded books of series seriesID, which the book list shows
// first, with the server's list. It runs on a goroutine of its own.
func FetchBooks(app *state.State, seriesID string) {
	ctx, cancel := context.WithTimeout(app.Ctx, state.RequestTimeout)
	remoteBooks, err := app.Server.Client.Books(ctx, seriesID)
	cancel()
	if app.Ctx.Err() != nil {
		return
	}
	app.Update(false, func() bool {
		// Left the series meanwhile? Reading one of its books still counts as in it.
		if app.Books.SeriesID != seriesID {
			return false
		}
		if err != nil {
			app.List.Notice = "Cannot load books: " + err.Error()
			return app.Screen == state.BooksScreen
		}
		app.List.Notice = ""
		app.Books.Items = app.Books.Items[:0]
		for _, book := range remoteBooks {
			app.Books.Items = append(app.Books.Items, BookItemLocked(app, book))
		}
		recountLocked(app, seriesID)
		if !app.Books.Browsed {
			app.Books.Focus = ContinueBookLocked(app)
		}
		return app.Screen == state.BooksScreen || app.Reader.AtEnd
	})
}

// VisibleBooksLocked returns the indexes in the book list of the books that pass its filter:
// those on the reader first (downloaded, downloading or queued), then the others, each group
// in the series' order.
func VisibleBooksLocked(app *state.State) []int {
	var local, remote []int
	for index, book := range app.Books.Items {
		onReader := book.Downloaded || app.Queue.OnItsWay(book.ID)
		switch {
		case app.Preferences.Books == state.BooksDownloaded && !onReader, app.Preferences.Books == state.BooksUnread && book.Completed:
			// filtered out
		case onReader:
			local = append(local, index)
		default:
			remote = append(remote, index)
		}
	}
	return append(local, remote...)
}

// ContinueBookLocked returns the ID of the book of the book list to read next: the first one
// in progress, else the first unread one after the last one read; "" if there is none.
func ContinueBookLocked(app *state.State) string {
	lastRead := -1
	for index, book := range app.Books.Items {
		switch {
		case book.Completed:
			lastRead = index
		case book.Page > 0:
			return book.ID
		}
	}
	for _, book := range app.Books.Items[lastRead+1:] {
		if !book.Completed {
			return book.ID
		}
	}
	return ""
}

// NoteProgressLocked shows in the lists how far book is read, when the reader leaves it: in
// the book list, in its series' counts, and as the book to continue.
func NoteProgressLocked(app *state.State, book shelf.Book) {
	setProgressLocked(app, book.ID, book.Page, book.Completed)
	switch {
	case !book.Completed && book.Page > 0:
		app.Series.Resume = &state.BookItem{Book: book, Downloaded: true}
	case app.Series.Resume != nil && app.Series.Resume.ID == book.ID:
		app.Series.Resume = nil
	}
}

// setProgressLocked sets the progress of book id in the book list, and counts it again in its
// series.
func setProgressLocked(app *state.State, id string, page int, completed bool) {
	for index := range app.Books.Items {
		if book := &app.Books.Items[index]; book.ID == id {
			countLocked(app, book.SeriesID, book.Page, book.Completed, -1)
			book.Page = page
			book.Completed = completed
			countLocked(app, book.SeriesID, book.Page, book.Completed, 1)
		}
	}
}

// ApplyReadStateLocked shows that book item was marked read or unread on the server, page
// being its new progress: in the lists and in the book on the reader. With err, it shows why
// it could not be.
func ApplyReadStateLocked(app *state.State, item state.BookItem, page int, read bool, err error) {
	if err != nil {
		app.List.Notice = "Cannot change the progress: " + err.Error()
		return
	}
	setProgressLocked(app, item.ID, page, read)
	if local, ok := app.Store.Book(item.ID); ok {
		local.Page = page
		local.Completed = read
		local.Unsynced = false
		if err := app.Store.Save(local); err != nil {
			log.Printf("saving progress: %v", err)
		}
	}
	app.List.Notice = "Marked unread: " + item.Title
	if read {
		app.List.Notice = "Marked read: " + item.Title
	}
}

// NoteDownloadedLocked shows in the lists that book is on the reader now.
func NoteDownloadedLocked(app *state.State, book shelf.Book) {
	// The page count of a comic book archive is known once it is downloaded, and a
	// fixed-layout EPUB is downloaded as pages.
	saved, _ := app.Store.Book(book.ID)
	for index := range app.Books.Items {
		if item := &app.Books.Items[index]; item.ID == book.ID {
			item.Downloaded = true
			item.Pages = cmp.Or(saved.Pages, item.Pages)
			item.Format = cmp.Or(saved.Format, item.Format)
		}
	}
	for index := range app.Series.Items {
		if app.Series.Items[index].ID == book.SeriesID {
			app.Series.Items[index].Local++
		}
	}
}

// RemoveDownloadLocked deletes downloaded book id from the reader, and shows it in the lists.
// A book whose progress is not on the server yet is only marked for removal: the
// progress.Syncer deletes it once it has sent its progress.
func RemoveDownloadLocked(app *state.State, id string) {
	book, ok := app.Store.Book(id)
	if !ok {
		return
	}
	var err error
	if book.Unsynced {
		book.PendingRemoval = true
		err = app.Store.Save(book)
	} else {
		err = app.Store.Delete(id)
	}
	if err != nil {
		app.List.Notice = "Cannot delete: " + err.Error()
		return
	}
	app.List.Notice = "Deleted " + book.Title
	if index := slices.IndexFunc(app.Books.Items, func(item state.BookItem) bool { return item.ID == id }); index >= 0 {
		item := app.Books.Items[index]
		app.Books.Items[index].Downloaded = false
		if !app.Server.Online { // offline, the lists and their counts are of the downloaded books
			countLocked(app, item.SeriesID, item.Page, item.Completed, -1)
			app.Books.Items = slices.Delete(app.Books.Items, index, index+1)
		}
	}
	for index := range app.Series.Items {
		if app.Series.Items[index].ID == book.SeriesID {
			app.Series.Items[index].Local--
		}
	}
}

// ProgressLabel says how far into book the reader is: its page, or its percent for a
// Reflowable book, whose pages are virtual.
func ProgressLabel(book shelf.Book) string {
	if book.Format == servers.Reflowable && book.Pages > 0 {
		return fmt.Sprintf("At %d%%", (book.Page-1)*100/book.Pages)
	}
	return fmt.Sprintf("At page %d", book.Page)
}
