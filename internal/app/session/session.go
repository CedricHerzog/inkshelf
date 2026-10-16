// Package session starts the app's work with a server: it reads config.json, lets the reader
// choose a server when there are several, opens that server's folder on the reader, and loads
// the series list, or falls back to the downloaded books when the server cannot be reached.
//
// Nothing before the series list needs the network. When the reader is not connected yet, the
// downloaded books show at once, while the Wi-Fi turns on: connecting can take a while, or never
// happen, and the books on the reader can be read meanwhile. Once connected, the server's list
// takes their place.
//
// Loading runs on a goroutine of its own, as connecting can take a while: meanwhile the app
// keeps answering the Back button and the firmware.
//
// The session also makes the progress.Syncer, which sends what was read to the server in use.
package session

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// errNoWiFi is why the app is offline when the Wi-Fi does not connect.
var errNoWiFi = errors.New("no Wi-Fi connection")

// Load reads config.json, lets the reader choose a server when there are several, and reads
// the series list, or falls back to the server's downloaded books offline. When the reader is
// not connected, it shows the downloaded books first and connects to the Wi-Fi after. It runs on
// a goroutine of its own.
func Load(app *state.State) {
	configs, err := servers.LoadConfig(app.Dir)
	if err != nil {
		showLoadError(app, err)
		return
	}
	moveLegacyFiles(app, configs[0])
	if len(configs) == 1 {
		app.Update(true, func() bool {
			app.Server.Kind = app.KindOf(configs[0])
			return true
		})
	} else {
		app.Update(true, func() bool {
			app.Loading.Servers = configs
			app.ShowLocked(state.ServersScreen)
			return true
		})
	}
	config, ok := waitForServerChoice(app, configs)
	if !ok {
		return
	}
	if err := openServerDir(app, config, len(configs) > 1); err != nil {
		showLoadError(app, err)
		return
	}

	client, err := app.Open(config)
	if err != nil {
		showLoadError(app, err)
		return
	}
	app.Update(false, func() bool {
		app.Server.Client = client
		return false
	})
	if app.Options.WiFiConnected() {
		fetchSeries(app, client)
		return
	}
	local := app.Store.Books()
	if len(local) == 0 {
		app.Update(false, func() bool {
			app.Loading.Status = "Connecting to Wi-Fi…"
			return app.Screen == state.LoadingScreen
		})
	} else {
		showDownloaded(app, local, "Connecting to Wi-Fi…")
	}
	if !connectNetwork(app) {
		if app.Ctx.Err() != nil {
			return
		}
		if len(local) == 0 {
			showLoadError(app, errNoWiFi)
		} else {
			setSeriesNotice(app, "Offline: "+errNoWiFi.Error())
		}
		return
	}
	if len(local) == 0 {
		app.Update(false, func() bool {
			app.Loading.Status = "Loading series…"
			return app.Screen == state.LoadingScreen
		})
		fetchSeries(app, client)
		return
	}
	// The downloaded books show: the server's list replaces them if the reader is still on it,
	// else the firmware's news of the network, or the next start, brings it.
	catalog.RefreshNow(app)
}

// fetchSeries reads the series list from client and shows it, or the downloaded books if it
// cannot.
func fetchSeries(app *state.State, client servers.Server) {
	err := catalog.FetchSeries(app, client)
	if err == nil || app.Ctx.Err() != nil {
		return
	}
	loadOffline(app, err)
}

// showLoadError shows on the loading screen the error that stopped loading.
func showLoadError(app *state.State, err error) {
	app.Update(true, func() bool {
		app.Loading.Err = err
		return true
	})
}

// connectNetwork turns the Wi-Fi on and keeps it on until the app closes. When it cannot, it
// logs why and returns false, and the app works offline.
func connectNetwork(app *state.State) bool {
	stop, err := app.Options.ConnectWiFi(app.Ctx)
	if err != nil {
		log.Printf("Wi-Fi not connected: %v", err)
		return false
	}
	app.StopNetwork = stop
	return true
}

// waitForServerChoice returns the server to use: the only one of config.json, or the one
// chosen on the server screen. It returns false if the app closes meanwhile.
func waitForServerChoice(app *state.State, configs []servers.Config) (servers.Config, bool) {
	if len(configs) == 1 {
		return configs[0], true
	}
	app.Update(false, func() bool {
		app.Loading.Status = "" // the choice is all that is left
		return app.Screen == state.ServersScreen
	})
	// The choice is the reader's: the app is idle until it comes. The server screen counts the
	// loading as work again when it sends the choice, and Go ends it when loading is over.
	app.Work.End(state.LoadingWork)
	select {
	case index := <-app.ServerChoice:
		return configs[index], true
	case <-app.Ctx.Done():
		app.Work.Begin(state.LoadingWork) // for Go to end
		return servers.Config{}, false
	}
}

// openServerDir makes the folder of the server of config, and opens its downloaded books,
// covers and preferences. named tells whether the series list's header shows the server's
// name, as it does when there are several servers.
//
// The folder is under the app's own, in applications/, so that the reader's library does not
// index the pages of the downloaded books as pictures.
func openServerDir(app *state.State, config servers.Config, named bool) error {
	log.Printf("server %s: %s", config.Name, config.Server)
	serverDir := filepath.Join(app.Dir, "servers", config.Name)
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		return err
	}
	app.Update(false, func() bool {
		app.ServerDir = serverDir
		app.Store = shelf.Store{Dir: filepath.Join(serverDir, "books"), Logf: log.Printf}
		app.CoverFiles = shelf.CoverFiles{Dir: filepath.Join(serverDir, "covers")}
		app.Server.Kind = app.KindOf(config)
		app.Preferences = state.LoadPreferences(serverDir)
		app.Loading.Status = "Loading series…"
		if named {
			app.Server.Name = config.Name
		}
		return true
	})
	return nil
}

// loadOffline shows the series of the downloaded books, after reading the series from the
// server failed with err. With no book downloaded, it shows err.
func loadOffline(app *state.State, err error) {
	log.Printf("offline: %v", err)
	local := app.Store.Books()
	if len(local) == 0 {
		showLoadError(app, err)
		return
	}
	showDownloaded(app, local, "Offline: "+err.Error())
}

// showDownloaded shows the series of local, the downloaded books, with notice in the list's
// header.
func showDownloaded(app *state.State, local []shelf.Book, notice string) {
	app.Update(true, func() bool {
		app.List.Notice = notice
		catalog.AddDownloadedSeriesLocked(app, local)
		app.Series.Resume = catalog.LocalResumeLocked(app)
		app.ShowLocked(state.SeriesScreen)
		return true
	})
}

// setSeriesNotice shows notice in the header of the series list, if the list still shows and no
// refresh has a notice of its own there.
func setSeriesNotice(app *state.State, notice string) {
	app.Update(false, func() bool {
		if app.Screen != state.SeriesScreen || app.Series.Refreshing {
			return false
		}
		app.List.Notice = notice
		return true
	})
}

// moveLegacyFiles moves the books and filters of the versions that knew a single server into
// the folder of first, the server they came from.
func moveLegacyFiles(app *state.State, first servers.Config) {
	serverDir := filepath.Join(app.Dir, "servers", first.Name)
	if _, err := os.Stat(serverDir); err == nil {
		return
	}
	for _, name := range []string{"books", "filters.json"} {
		if _, err := os.Stat(filepath.Join(app.Dir, name)); err != nil {
			continue
		}
		if err := os.MkdirAll(serverDir, 0o755); err != nil {
			log.Printf("moving %s: %v", name, err)
			return
		}
		if err := os.Rename(filepath.Join(app.Dir, name), filepath.Join(serverDir, name)); err != nil {
			log.Printf("moving %s: %v", name, err)
		} else {
			log.Printf("moved %s to %s", name, serverDir)
		}
	}
}
