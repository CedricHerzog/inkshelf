// Package catalog keeps the lists of series and books that the app shows: what the server
// has, what the reader downloaded, and how far each one is read.
//
// Online, the lists come from the server and the downloaded books are marked in them; offline,
// they come from the books on the reader alone. Either way, a series counts its books read and
// in progress, so that the series list can filter by read status without asking the server
// again: whatever changes a book's progress (reading it, marking it read, deleting it) goes
// through this package, which keeps the counts right.
//
// The book to continue, first in the series list, is the one the server says was read last,
// unless the reader is ahead of it.
package catalog

import (
	"cmp"
	"context"
	"errors"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// JumpLetters are the choices of the series list's A–Z jump; "#" is for the titles that do not
// start with a letter.
const JumpLetters = "#ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// initialOf returns the letter of JumpLetters a series is filed under: the first letter of its
// sort title, without its accent and in upper case, or "#" if it is not one from A to Z. The
// sort title is the server's own, as Komga's, or servers.SortKey's.
func initialOf(sortTitle string) string {
	first, _ := utf8.DecodeRuneInString(strings.ToUpper(servers.Fold(sortTitle)))
	if first >= 'A' && first <= 'Z' {
		return string(first)
	}
	return "#"
}

// FetchSeries reads the series, the libraries and the book to continue from the server, and
// shows them; then it sends the progress read offline. It returns why it cannot.
func FetchSeries(app *state.State, client servers.Server) error {
	ctx, cancel := context.WithTimeout(app.Ctx, state.RequestTimeout)
	defer cancel()
	remoteSeries, err := client.Series(ctx)
	if err != nil {
		return err
	}
	libraries, librariesErr := client.Libraries(ctx)
	if librariesErr != nil {
		log.Printf("libraries: %v", librariesErr) // the saved ones still filter
	}
	reading, readingErr := client.InProgress(ctx, 1)
	if readingErr != nil {
		log.Printf("books in progress: %v", readingErr) // continue the one read last on the reader
	}
	if app.Ctx.Err() != nil {
		return app.Ctx.Err()
	}
	downloadedCounts := map[string]int{}
	for _, book := range app.Store.Books() {
		downloadedCounts[book.SeriesID]++
	}

	log.Printf("%d series, %d libraries", len(remoteSeries), len(libraries))
	var saved state.Preferences
	app.Update(false, func() bool {
		app.Server.Online = true
		app.Server.LocalProgress = errors.Is(readingErr, errors.ErrUnsupported)
		app.Series.Items = seriesItemsFrom(remoteSeries, downloadedCounts)
		if app.Server.LocalProgress { // the server counts no books read: count the reader's
			for _, book := range app.Store.Books() {
				countLocked(app, book.SeriesID, book.Page, book.Completed, 1)
			}
		}
		if libraries != nil {
			app.Preferences.Libraries = libraries
			if app.Preferences.LibraryIndex() < 0 {
				app.Preferences.Library = "" // removed from the server
			}
		}
		saved = app.Preferences
		app.Series.Resume = chooseResumeLocked(app, reading, readingErr)
		if app.Screen == state.LoadingScreen {
			app.ShowLocked(state.SeriesScreen)
			app.Repaint.FullRefresh = true
		}
		return app.Screen == state.SeriesScreen
	})
	if libraries != nil {
		if err := saved.Save(app.ServerDir); err != nil {
			log.Printf("saving filters: %v", err)
		}
	}
	app.Syncer.PushNow() // what was read offline
	return nil
}

// seriesItemsFrom returns the server's series as the list shows them. downloadedCounts are the
// numbers of downloaded books, by series ID.
func seriesItemsFrom(remoteSeries []servers.Series, downloadedCounts map[string]int) []state.SeriesItem {
	var items []state.SeriesItem
	for _, remote := range remoteSeries {
		items = append(items, state.SeriesItem{
			ID:        remote.ID,
			Title:     remote.Title,
			LibraryID: remote.LibraryID,
			Direction: remote.Direction,
			Modified:  remote.Modified,
			Initial:   initialOf(cmp.Or(remote.SortTitle, remote.Title)),
			Books:     remote.Books,
			Local:     downloadedCounts[remote.ID],
			Read:      remote.BooksRead,
			Reading:   remote.BooksInProgress,
		})
	}
	return items
}

// AddDownloadedSeriesLocked fills the series list with the series of the books on the reader,
// for the app offline. local is sorted by series, as the store lists them.
func AddDownloadedSeriesLocked(app *state.State, local []shelf.Book) {
	for _, book := range local {
		count := len(app.Series.Items)
		if count == 0 || app.Series.Items[count-1].ID != book.SeriesID {
			app.Series.Items = append(app.Series.Items, state.SeriesItem{
				ID:        book.SeriesID,
				Title:     book.SeriesTitle,
				LibraryID: book.LibraryID,
				Direction: book.Direction,
				Initial:   initialOf(servers.SortKey(book.SeriesTitle)), // as the store sorts them
			})
		}
		app.Series.Items[len(app.Series.Items)-1].Local++
		countLocked(app, book.SeriesID, book.Page, book.Completed, 1)
	}
}

// chooseResumeLocked returns the book to continue: the one the server says was read last
// (reading, or readingErr if it could not say), unless the reader is ahead.
func chooseResumeLocked(app *state.State, reading []servers.Book, readingErr error) *state.BookItem {
	local := LocalResumeLocked(app)
	switch {
	case local != nil && (local.Unsynced || app.Server.LocalProgress): // the reader is ahead
		return local
	case len(reading) > 0:
		item := BookItemLocked(app, reading[0])
		return &item
	case readingErr == nil: // nothing in progress
		return nil
	default:
		return local
	}
}

// LocalResumeLocked returns the downloaded book in progress that was read last on the reader,
// or nil if there is none.
func LocalResumeLocked(app *state.State) *state.BookItem {
	var resume *state.BookItem
	for _, book := range app.Store.Books() {
		if book.Page > 0 && !book.Completed && (resume == nil || book.ReadAt.After(resume.ReadAt)) {
			resume = &state.BookItem{Book: book, Downloaded: true}
		}
	}
	return resume
}

// Refresh reads the series list again from the server, if the series list shows, on a
// goroutine of its own.
func Refresh(app *state.State) {
	client := beginRefresh(app)
	if client == nil {
		return
	}
	app.Go("refreshing the series", func() {
		endRefresh(app, FetchSeries(app, client))
	})
}

// RefreshNow reads the series list again from the server, if the series list shows, as Refresh
// does, but on the goroutine that calls it, for one that is not the main thread.
func RefreshNow(app *state.State) {
	if client := beginRefresh(app); client != nil {
		endRefresh(app, FetchSeries(app, client))
	}
}

// beginRefresh marks the series list as refreshing and returns the server to read it from, or
// nil if the list doesn't show or is being refreshed already.
func beginRefresh(app *state.State) servers.Server {
	var client servers.Server
	app.Update(false, func() bool {
		if app.Screen != state.SeriesScreen || app.Series.Refreshing || app.Server.Client == nil {
			return false
		}
		app.Series.Refreshing = true
		app.List.Notice = "Refreshing…"
		client = app.Server.Client
		return true
	})
	return client
}

// endRefresh shows how the refresh that beginRefresh started went: err is why it failed.
func endRefresh(app *state.State, err error) {
	if app.Ctx.Err() != nil {
		return
	}
	app.Update(false, func() bool {
		app.Series.Refreshing = false
		app.List.Notice = ""
		if err != nil {
			app.List.Notice = "Cannot refresh: " + err.Error()
		}
		return true
	})
}

// VisibleSeriesLocked returns the indexes in the series list of the series that pass the
// filters.
func VisibleSeriesLocked(app *state.State) []int {
	var visible []int
	for index, item := range app.Series.Items {
		if app.Preferences.Match(item) {
			visible = append(visible, index)
		}
	}
	return visible
}

// ResumeRowsLocked is the number of rows of the series list before its series: 1 for the book
// to continue, if there is one.
func ResumeRowsLocked(app *state.State) int {
	if app.Series.Resume != nil {
		return 1
	}
	return 0
}

// countLocked adds a book's progress to its series' counts of books read and in progress,
// delta being 1 to add it or -1 to take it away.
func countLocked(app *state.State, seriesID string, page int, completed bool, delta int) {
	for index := range app.Series.Items {
		item := &app.Series.Items[index]
		if item.ID != seriesID {
			continue
		}
		switch {
		case completed:
			item.Read += delta
		case page > 0:
			item.Reading += delta
		}
	}
}

// recountLocked counts the books read and in progress of series seriesID again, from the book
// list, which holds the server's full list of its books.
func recountLocked(app *state.State, seriesID string) {
	for index := range app.Series.Items {
		if app.Series.Items[index].ID == seriesID {
			app.Series.Items[index].Read = 0
			app.Series.Items[index].Reading = 0
		}
	}
	for _, book := range app.Books.Items {
		countLocked(app, seriesID, book.Page, book.Completed, 1)
	}
}
