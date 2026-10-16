package session

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

func TestSyncerWaitsForTheNetwork(t *testing.T) {
	app, server, _ := offlineApp(t, false)
	var connected atomic.Bool
	app.Options.WiFiConnected = connected.Load
	app.Store = shelf.Store{Dir: filepath.Join(t.TempDir(), "books")}
	if err := os.MkdirAll(filepath.Join(app.Store.Dir, "read"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := app.Store.Save(shelf.Book{ID: "read", Page: 4, Pages: 10, Unsynced: true}); err != nil {
		t.Fatal(err)
	}
	app.Server.Client = server

	app.Syncer.PushNow()
	if _, ok := server.SentProgress("read"); ok {
		t.Error("progress sent while offline")
	}
	if book, _ := app.Store.Book("read"); !book.Unsynced {
		t.Error("the book was marked synced while offline")
	}

	connected.Store(true)
	app.Syncer.PushNow()
	if sent, ok := server.SentProgress("read"); !ok || sent.Page != 4 {
		t.Errorf("progress sent once online: %+v, %v; want page 4", sent, ok)
	}
}
