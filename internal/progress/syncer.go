package progress

import (
	"log"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// Syncer sends the progress of the books read since their last sync to their server: after a
// delay, debounced, or at once. It also deletes the books removed from the reader whose
// progress the server didn't have yet.
type Syncer struct {
	appLock sync.Locker
	target  func() (servers.Server, shelf.Store)
	synced  func(book shelf.Book)

	pushing sync.Mutex // makes pushes run one at a time

	timerMu sync.Mutex  // guards timer
	timer   *time.Timer // runs PushNow; nil until the first Schedule
}

// NewSyncer returns a Syncer. appLock is the app's mutex, held while the Syncer reads target
// and marks books synced. target returns the server to send the progress to, nil while there
// is none, and the store of its books. syncedLocked, called with appLock held after book was
// marked synced in the store, lets the app clear the unsynced flag of its copy of the open
// book.
func NewSyncer(appLock sync.Locker, target func() (servers.Server, shelf.Store), syncedLocked func(book shelf.Book)) *Syncer {
	return &Syncer{appLock: appLock, target: target, synced: syncedLocked}
}

// Schedule sends the progress after delay, unless called again before: turning several pages
// sends it once.
func (syncer *Syncer) Schedule(delay time.Duration) {
	syncer.timerMu.Lock()
	defer syncer.timerMu.Unlock()
	if syncer.timer == nil {
		syncer.timer = time.AfterFunc(delay, syncer.PushNow)
	} else {
		syncer.timer.Reset(delay)
	}
}

// Stop cancels the push scheduled, if any.
func (syncer *Syncer) Stop() {
	syncer.timerMu.Lock()
	defer syncer.timerMu.Unlock()
	if syncer.timer != nil {
		syncer.timer.Stop()
	}
}

// PushNow sends the progress of the books read since their last sync, and marks them synced,
// or deletes them if they were to be removed. It stops at the first error: probably offline,
// it is retried on the next page turn or start. It must be called without the app's mutex.
func (syncer *Syncer) PushNow() {
	syncer.pushing.Lock()
	defer syncer.pushing.Unlock()
	syncer.appLock.Lock()
	server, store := syncer.target()
	syncer.appLock.Unlock()
	if server == nil {
		return
	}
	for _, book := range store.AllBooks() {
		if !book.Unsynced && !book.PendingRemoval {
			continue
		}
		if book.Unsynced {
			if err := sendProgress(server, book); err != nil {
				log.Printf("sync %s: %v", book.ID, err)
				return // probably offline; retried on the next page turn or start
			}
			log.Printf("synced %s: page %d/%d", book.ID, book.Page, book.Pages)
		}
		syncer.appLock.Lock()
		syncer.markSyncedLocked(store, book)
		syncer.appLock.Unlock()
	}
}

// markSyncedLocked records that the server has book's progress: it clears its Unsynced flag, or
// deletes it if it was to be removed. Unless the reader turned its pages meanwhile: then the
// server is still behind. The app's mutex is held, so that the app doesn't save the open book
// in between.
func (syncer *Syncer) markSyncedLocked(store shelf.Store, book shelf.Book) {
	now, ok := store.BookEvenIfRemoved(book.ID)
	if !ok || now.Page != book.Page || now.Completed != book.Completed {
		return
	}
	if now.PendingRemoval {
		if err := store.Delete(book.ID); err != nil {
			log.Printf("deleting %s: %v", book.ID, err)
		}
		return
	}
	now.Unsynced = false
	if err := store.Save(now); err != nil {
		log.Printf("saving progress: %v", err)
	}
	syncer.synced(book)
}
