// Package openbook is the book open in the reader: opening it where it was left, turning its
// pages, preloading the pages around the one shown, and closing it.
//
// A book is pages (a comic book archive, or a fixed-layout EPUB prepared at download time) or
// text laid out for the screen (a Reflowable EPUB, through the reading package). Either way the
// book's progress is a page, which the reader saves on its disk and the progress.Syncer sends
// to the server a few seconds after the last page turn.
//
// Turning a page only moves the position and asks for a redraw: the preloader goroutine then
// saves the progress and decodes or draws the next and previous pages, so that the next turn
// finds its page ready.
package openbook

import (
	"cmp"
	"context"
	"image"
	"log"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/downloadqueue"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/progress"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// Every fullEvery page turns, the panel is refreshed whole, with a flash, to clear the
// ghosting that partial refreshes leave.
const fullEvery = 5

// The next book of the series starts downloading once the reader is autoQueueAt/autoQueueOutOf
// of the way through the open one, at 4/5: it is then ready when the open one ends.
const (
	autoQueueAt    = 4
	autoQueueOutOf = 5
)

// SyncDelay is how long after the last page turn the progress is sent to the server, so that
// turning several pages sends it once.
const SyncDelay = 3 * time.Second

// OpenLocked opens downloaded book item where it was left, and shows it. Online, the list
// holds the server's progress, so the book opens where another device stopped reading it.
func OpenLocked(app *state.State, item state.BookItem) {
	span := timing.Start(context.Background(), "open book")
	defer func() { log.Printf("open book %s: %v", item.ID, span.End()) }()
	book, ok := app.Store.Book(item.ID)
	if !ok {
		app.List.Notice = "Cannot open " + item.Title
		return
	}
	remote := servers.ReadingProgress{Page: item.Page, Completed: item.Completed}
	if merged, changed := progress.Merge(book, remote, app.Server.Online && !app.Server.LocalProgress); changed {
		book = merged
		if err := app.Store.Save(book); err != nil {
			log.Printf("saving progress: %v", err)
		}
	}
	// The book was downloaded before libraries were saved, or its series' direction changed on
	// the server.
	if (book.LibraryID == "" && item.LibraryID != "") || (item.Direction != "" && book.Direction != item.Direction) {
		book.LibraryID = cmp.Or(book.LibraryID, item.LibraryID)
		book.Direction = cmp.Or(item.Direction, book.Direction)
		if err := app.Store.Save(book); err != nil {
			log.Printf("saving %s: %v", book.ID, err)
		}
	}
	var text *state.TextReader
	if book.Format == servers.Reflowable {
		var err error
		if text, err = openTextLocked(app, book); err != nil {
			log.Printf("opening %s: %v", book.ID, err)
			app.List.Notice = "Cannot open " + item.Title + ": " + widgets.Clean(err.Error())
			return
		}
	}
	book.Page = min(max(book.Page, 1), book.Pages)
	if text != nil {
		if location, err := text.Book.Location(text.Book.At()); err == nil {
			book.Location = &location
		}
	}
	app.Reader.OpenBook = book
	app.Reader.Text = text
	app.Reader.Unsaved = false
	app.Reader.Pages = map[int]*image.Gray{}
	app.Reader.PageErr = nil
	app.Reader.Toolbar = false
	app.Reader.AtEnd = false
	app.Reader.Turns = 0
	app.ShowLocked(state.ReaderScreen)
}

// TurnPage moves the open book one page and schedules the progress sync; the preloader saves
// it. Past the last page, it shows the end panel.
func TurnPage(app *state.State, forward bool) {
	turned := false
	app.Update(fullRefreshDue(app), func() bool {
		if app.Screen != state.ReaderScreen {
			return false
		}
		// The turn's task ends once its page is drawn; a turn not drawn yet ends here.
		app.Reader.Turn.End()
		app.Reader.Turn = timing.StartPending("page turn")
		if app.Reader.AtEnd || (forward && atLastPageLocked(app)) {
			return showEndPanelLocked(app, forward)
		}
		if !movePageLocked(app, forward) {
			app.Reader.Turn.End()
			return false
		}
		turned = true
		autoQueueLocked(app)
		return true
	})
	if turned {
		app.Syncer.Schedule(SyncDelay)
	}
}

// fullRefreshDue reports whether the next page turn refreshes the panel whole, to clear the
// ghosting: every fullEvery turns, or textFullEvery for text. It reads the reader's turns and
// text without the mutex: only the main thread changes them, and it runs there.
func fullRefreshDue(app *state.State) bool {
	every := fullEvery
	if app.Reader.Text != nil {
		every = textFullEvery
	}
	return app.Reader.Turns%every == every-1
}

// atLastPageLocked reports whether the open book shows its last page.
func atLastPageLocked(app *state.State) bool {
	if app.Reader.Text != nil {
		return atLastTextPageLocked(app)
	}
	return app.Reader.OpenBook.Page == app.Reader.OpenBook.Pages
}

// showEndPanelLocked shows the end panel, or hides it to go back to the last page, and
// reports whether that changed.
func showEndPanelLocked(app *state.State, show bool) bool {
	changed := app.Reader.AtEnd != show
	app.Reader.AtEnd = show
	app.Reader.Toolbar = false
	app.Repaint.FullRefresh = app.Repaint.FullRefresh || changed
	return changed
}

// movePageLocked moves the open book to the next or previous page, and reports whether there
// was one.
func movePageLocked(app *state.State, forward bool) bool {
	if app.Reader.Text != nil {
		if !moveTextLocked(app, forward) {
			return false
		}
		app.Reader.Toolbar = false
		app.Reader.Turns++
		return true
	}
	book := &app.Reader.OpenBook
	pageNumber := book.Page - 1
	if forward {
		pageNumber = book.Page + 1
	}
	if pageNumber < 1 || pageNumber > book.Pages {
		return false
	}
	book.Page = pageNumber
	book.Completed = book.Completed || pageNumber == book.Pages
	book.Unsynced = true
	book.ReadAt = time.Now()
	app.Reader.Unsaved = true
	app.Reader.PageErr = nil
	app.Reader.Toolbar = false
	app.Reader.Turns++
	return true
}

// autoQueueLocked queues the next book once the open one is read far enough, once per book: the
// reader may have taken it out of the queue since.
func autoQueueLocked(app *state.State) {
	book := app.Reader.OpenBook
	if app.Reader.AutoQueued == book.ID || book.Page*autoQueueOutOf < book.Pages*autoQueueAt {
		return
	}
	app.Reader.AutoQueued = book.ID
	QueueNextLocked(app)
}

// CloseLocked closes the open book and shows the book list, with the book's new progress.
func CloseLocked(app *state.State) {
	catalog.NoteProgressLocked(app, app.Reader.OpenBook)
	SaveProgressLocked(app)
	app.Reader.Pages = nil
	if app.Reader.Text != nil {
		app.Reader.Text.Book.Close()
		app.Reader.Text = nil
	}
	app.Reader.Toolbar = false
	app.Reader.AtEnd = false
	app.ShowLocked(state.BooksScreen)
}

// RightToLeftLocked reports whether the open book reads right to left: as chosen on the
// reader, else as the server says, else as manga do. A Reflowable book reads as its EPUB's
// page-progression-direction says, else as its language is written.
func RightToLeftLocked(app *state.State) bool {
	if direction := app.Preferences.Directions[app.Reader.OpenBook.SeriesID]; direction != "" {
		return direction != servers.LeftToRight
	}
	if app.Reader.Text != nil {
		return app.Reader.Text.Book.Index.RightToLeft()
	}
	return app.Reader.OpenBook.Direction != servers.LeftToRight
}

// DarkPageLocked reports whether the open book's pages show white on black, which only dark
// mode does: as chosen on the reader for its series, else for a Reflowable book, whose pages
// are text. A page of pictures (a comic, a manga, a recipe) keeps its colors. The choice is per
// series, as the reading direction is: books without one share the server's catch-all series.
func DarkPageLocked(app *state.State) bool {
	if !pocketbook.DarkMode() {
		return false
	}
	if dark, chosen := app.Preferences.DarkPages[app.Reader.OpenBook.SeriesID]; chosen {
		return dark
	}
	return app.Reader.Text != nil
}

// NextBookLocked returns the book after the open one in its series, if the list has it.
func NextBookLocked(app *state.State) (state.BookItem, bool) {
	for index, book := range app.Books.Items[:max(len(app.Books.Items)-1, 0)] {
		if book.ID == app.Reader.OpenBook.ID {
			return app.Books.Items[index+1], true
		}
	}
	return state.BookItem{}, false
}

// QueueNextLocked queues the download of the book after the open one, unless the reader has
// it or it is on its way already, or the app is offline.
func QueueNextLocked(app *state.State) {
	if next, ok := NextBookLocked(app); ok {
		downloadqueue.EnqueueIfMissingLocked(app, next)
	}
}

// AskToDeleteLocked asks whether to delete the book just closed, if it was read to the end, so
// that finished books do not fill the reader; next, if not nil, opens after the answer.
// Otherwise it opens next at once. It is called right after CloseLocked, which leaves the book
// closed in the reader's state.
func AskToDeleteLocked(app *state.State, next *state.BookItem) {
	finished := app.Reader.OpenBook
	if !finished.Completed {
		if next != nil {
			OpenLocked(app, *next)
		}
		return
	}
	app.AskDelete.Book = finished
	app.AskDelete.Next = next
	app.AskDelete.Size = app.Store.Size(finished.ID)
	app.ShowLocked(state.AskDeleteScreen)
}

// SaveProgressLocked saves the open book's progress on the reader's disk, if it changed.
func SaveProgressLocked(app *state.State) {
	if !app.Reader.Unsaved {
		return
	}
	app.Reader.Unsaved = false
	if err := app.Store.Save(app.Reader.OpenBook); err != nil {
		log.Printf("saving progress: %v", err)
	}
}
