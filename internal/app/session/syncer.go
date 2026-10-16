package session

import (
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/progress"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// NewSyncer returns the app's progress.Syncer, which sends the progress of the downloaded books
// to the server in use, once there is one and the reader is connected. Offline, it waits: the
// progress goes once the network is back (System's NetConnected) or at the next page turn. The
// Syncer takes the state's mutex itself, after its own lock: never call it with the mutex held.
func NewSyncer(app *state.State) *progress.Syncer {
	return progress.NewSyncer(app, func() (servers.Server, shelf.Store) {
		if app.Server.Client == nil {
			return nil, shelf.Store{} // no server chosen yet
		}
		if !app.Options.WiFiConnected() {
			return nil, shelf.Store{} // every request would fail at once
		}
		return app.Server.Client, app.Store
	}, func(book shelf.Book) { bookSyncedLocked(app, book) })
}

// bookSyncedLocked records that the server has the progress of book, if it is the open book and
// the reader has not turned its pages since the progress was sent.
func bookSyncedLocked(app *state.State, book shelf.Book) {
	openBook := &app.Reader.OpenBook
	if openBook.ID == book.ID && openBook.Page == book.Page && openBook.Completed == book.Completed {
		openBook.Unsynced = false
	}
}
