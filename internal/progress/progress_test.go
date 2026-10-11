package progress

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// The server's progress wins only when it was just read and the reader has nothing unsent.
func TestMerge(t *testing.T) {
	local := shelf.Book{ID: "b1", Page: 4, Pages: 10}
	remote := servers.ReadingProgress{Page: 8, Completed: false}
	for _, test := range []struct {
		name       string
		local      shelf.Book
		remote     servers.ReadingProgress
		fromServer bool
		wantPage   int
		wantChange bool
	}{
		{"server ahead", local, remote, true, 8, true},
		{"offline", local, remote, false, 4, false},
		{"unsent progress", shelf.Book{ID: "b1", Page: 4, Pages: 10, Unsynced: true}, remote, true, 4, false},
		{"same progress", local, servers.ReadingProgress{Page: 4}, true, 4, false},
		{"server behind", local, servers.ReadingProgress{Page: 1}, true, 1, true},
	} {
		merged, changed := Merge(test.local, test.remote, test.fromServer)
		if merged.Page != test.wantPage || changed != test.wantChange {
			t.Errorf("%s: page %d, changed %v; want %d, %v", test.name, merged.Page, changed, test.wantPage, test.wantChange)
		}
	}
	if merged, changed := Merge(local, servers.ReadingProgress{Page: 4, Completed: true}, true); !changed || !merged.Completed {
		t.Errorf("a book read elsewhere: %+v, %v", merged, changed)
	}
}

// Marking a book read sends its last page; unread clears it; a server without progress is no
// error.
func TestSendReadState(t *testing.T) {
	fake := &servertest.Fake{}
	page, err := SendReadState(t.Context(), fake, "b1", 12, true)
	if sent, ok := fake.SentProgress("b1"); err != nil || page != 12 || !ok || sent.Page != 12 || !sent.Completed {
		t.Fatalf("read: page %d, %v, sent %+v", page, err, sent)
	}
	page, err = SendReadState(t.Context(), fake, "b1", 12, false)
	if _, ok := fake.SentProgress("b1"); err != nil || page != 0 || ok {
		t.Fatalf("unread: page %d, %v, still sent: %v", page, err, ok)
	}
	if _, err := SendReadState(t.Context(), noProgress{fake}, "b1", 12, true); err != nil {
		t.Errorf("a server without progress: %v", err)
	}
}

// noProgress is a server that keeps no progress, as Mealie.
type noProgress struct{ *servertest.Fake }

func (noProgress) SetProgress(context.Context, string, int, bool) error {
	return errors.ErrUnsupported
}

func (noProgress) ClearProgress(context.Context, string) error { return errors.ErrUnsupported }

// offline is a server that cannot be reached.
type offline struct{ *servertest.Fake }

func (offline) SetProgress(context.Context, string, int, bool) error {
	return errors.New("no network")
}

// newTestSyncer returns a Syncer pushing to server the books of store, and the books it
// reported synced.
func newTestSyncer(server servers.Server, store shelf.Store) (*Syncer, *[]string) {
	var appLock sync.Mutex
	var synced []string
	syncer := NewSyncer(&appLock, func() (servers.Server, shelf.Store) { return server, store }, func(book shelf.Book) {
		synced = append(synced, book.ID)
	})
	return syncer, &synced
}

// saveBooks saves books in a new store.
func saveBooks(t *testing.T, books ...shelf.Book) shelf.Store {
	t.Helper()
	store := shelf.Store{Dir: t.TempDir()}
	for _, book := range books {
		if err := os.MkdirAll(filepath.Join(store.Dir, book.ID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(book); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

// A push sends the unsynced books, marks them synced, and deletes those to be removed.
func TestSyncerPushNow(t *testing.T) {
	store := saveBooks(t,
		shelf.Book{ID: "read", Page: 3, Pages: 10, Unsynced: true},
		shelf.Book{ID: "synced", Page: 5, Pages: 10},
		shelf.Book{ID: "removed", Page: 10, Pages: 10, Completed: true, Unsynced: true, PendingRemoval: true},
	)
	fake := &servertest.Fake{}
	syncer, synced := newTestSyncer(fake, store)
	syncer.PushNow()
	if sent, ok := fake.SentProgress("read"); !ok || sent.Page != 3 {
		t.Errorf("progress sent %+v, %v", sent, ok)
	}
	if _, ok := fake.SentProgress("synced"); ok {
		t.Error("a synced book was sent again")
	}
	if book, ok := store.Book("read"); !ok || book.Unsynced {
		t.Errorf("after the push %+v, %v", book, ok)
	}
	if _, ok := store.BookEvenIfRemoved("removed"); ok {
		t.Error("the book to remove was kept")
	}
	if len(*synced) != 1 || (*synced)[0] != "read" {
		t.Errorf("synced %v, want [read]", *synced)
	}
}

// Offline, nothing is marked synced; a server without progress counts as synced.
func TestSyncerServers(t *testing.T) {
	book := shelf.Book{ID: "read", Page: 3, Pages: 10, Unsynced: true}
	store := saveBooks(t, book)
	syncer, _ := newTestSyncer(offline{&servertest.Fake{}}, store)
	syncer.PushNow()
	if saved, _ := store.Book("read"); !saved.Unsynced {
		t.Error("marked synced while offline")
	}
	syncer, _ = newTestSyncer(noProgress{&servertest.Fake{}}, store)
	syncer.PushNow()
	if saved, _ := store.Book("read"); saved.Unsynced {
		t.Error("still unsynced on a server that keeps no progress")
	}
	syncer, _ = newTestSyncer(nil, shelf.Store{})
	syncer.PushNow() // no server yet: nothing to do, no store to read
}

// Schedule pushes once after the last call's delay.
func TestSyncerSchedule(t *testing.T) {
	store := saveBooks(t, shelf.Book{ID: "read", Page: 3, Pages: 10, Unsynced: true})
	fake := &servertest.Fake{}
	syncer, _ := newTestSyncer(fake, store)
	syncer.Schedule(time.Hour)
	syncer.Schedule(time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := fake.SentProgress("read"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never pushed")
		}
		time.Sleep(time.Millisecond)
	}
	syncer.Stop()
}
