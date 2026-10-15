package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/servertest"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// testTimeout bounds every wait, so that a test that would hang fails instead.
const testTimeout = 10 * time.Second

// countingServer is a servertest.Fake that counts the reads of the series list.
type countingServer struct {
	*servertest.Fake
	seriesReads atomic.Int32
}

func (server *countingServer) Series(ctx context.Context) ([]servers.Series, error) {
	server.seriesReads.Add(1)
	return server.Fake.Series(ctx)
}

// offlineApp returns the state of an app with one server, whose reader is not connected:
// connecting waits for a value on connectResult, which is the error it returns. With
// downloaded, the server's folder holds a downloaded book.
func offlineApp(t *testing.T, downloaded bool) (app *state.State, server *countingServer, connectResult chan error) {
	t.Helper()
	dir := t.TempDir()
	config := `[{"name": "home", "type": "komga", "server": "http://komga.invalid", "apiKey": "key"}]`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if downloaded {
		store := shelf.Store{Dir: filepath.Join(dir, "servers", "home", "books")}
		book := shelf.Book{ID: "local-book", SeriesID: "local-series", SeriesTitle: "Downloaded", Title: "Tome 1", Pages: 3}
		if err := os.MkdirAll(filepath.Join(store.Dir, book.ID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(book); err != nil {
			t.Fatal(err)
		}
	}
	server = &countingServer{Fake: &servertest.Fake{
		AllSeries: []servers.Series{{ID: "remote-series", Title: "From the server", Books: 2}},
	}}
	connectResult = make(chan error)
	options := state.Options{
		WiFiConnected: func() bool { return false },
		ConnectWiFi: func(ctx context.Context) (func(), error) {
			select {
			case err := <-connectResult:
				if err != nil {
					return nil, err
				}
				return func() {}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	open := func(servers.Config) (servers.Server, error) { return server, nil }
	kindOf := func(servers.Config) servers.Kind { return servers.Kind{Name: "Test"} }
	app = state.New(dir, open, kindOf, options)
	var cancel context.CancelFunc
	app.Ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)
	app.Syncer = NewSyncer(app)
	return app, server, connectResult
}

// waitFor waits until condition, called with the state's mutex held, holds.
func waitFor(t *testing.T, app *state.State, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		app.Lock()
		done := condition()
		app.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// loadInBackground runs Load on a goroutine and returns a channel closed when it returns.
func loadInBackground(app *state.State) chan struct{} {
	loaded := make(chan struct{})
	go func() {
		defer close(loaded)
		Load(app)
	}()
	return loaded
}

// waitLoaded waits for Load to return.
func waitLoaded(t *testing.T, loaded chan struct{}) {
	t.Helper()
	select {
	case <-loaded:
	case <-time.After(testTimeout):
		t.Fatal("Load did not return")
	}
}

func TestLoadShowsDownloadedBooksWhileWiFiConnects(t *testing.T) {
	app, server, connectResult := offlineApp(t, true)
	loaded := loadInBackground(app)

	waitFor(t, app, "the downloaded series", func() bool {
		return app.Screen == state.SeriesScreen && len(app.Series.Items) == 1 &&
			app.Series.Items[0].ID == "local-series" && app.List.Notice == "Connecting to Wi-Fi…"
	})
	if reads := server.seriesReads.Load(); reads != 0 {
		t.Errorf("the series list was read %d times before the Wi-Fi connected", reads)
	}

	connectResult <- nil
	waitLoaded(t, loaded)
	app.Lock()
	defer app.Unlock()
	if !app.Server.Online {
		t.Error("the app is offline once the Wi-Fi connected")
	}
	if len(app.Series.Items) != 1 || app.Series.Items[0].ID != "remote-series" {
		t.Errorf("series = %+v, want the server's", app.Series.Items)
	}
	if app.List.Notice != "" || app.Series.Refreshing {
		t.Errorf("notice %q, refreshing %v after the server's list came", app.List.Notice, app.Series.Refreshing)
	}
}

func TestLoadWithoutWiFiKeepsDownloadedBooks(t *testing.T) {
	app, server, connectResult := offlineApp(t, true)
	loaded := loadInBackground(app)
	connectResult <- errors.New("netagent failed")
	waitLoaded(t, loaded)

	app.Lock()
	defer app.Unlock()
	if app.Screen != state.SeriesScreen || len(app.Series.Items) != 1 || app.Series.Items[0].ID != "local-series" {
		t.Errorf("screen %v, series %+v, want the downloaded series", app.Screen, app.Series.Items)
	}
	if want := "Offline: no Wi-Fi connection"; app.List.Notice != want {
		t.Errorf("notice %q, want %q", app.List.Notice, want)
	}
	if reads := server.seriesReads.Load(); reads != 0 {
		t.Errorf("the series list was read %d times without Wi-Fi", reads)
	}
}

func TestLoadWithoutWiFiOrDownloadsShowsWhy(t *testing.T) {
	app, _, connectResult := offlineApp(t, false)
	loaded := loadInBackground(app)
	waitFor(t, app, "the connecting status", func() bool {
		return app.Loading.Status == "Connecting to Wi-Fi…"
	})
	connectResult <- errors.New("netagent failed")
	waitLoaded(t, loaded)

	app.Lock()
	defer app.Unlock()
	if !errors.Is(app.Loading.Err, errNoWiFi) {
		t.Errorf("loading error %v, want %v", app.Loading.Err, errNoWiFi)
	}
}
