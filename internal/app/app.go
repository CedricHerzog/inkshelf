package app

import (
	"context"
	"image"
	"log"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/covers"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/booklist"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/contents"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/coverview"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/reader"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/serieslist"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/startup"
	"github.com/CedricHerzog/inkshelf/internal/app/session"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// application is the pocketbook.App: it draws the screen shown and passes it the reader's
// buttons and touches.
type application struct {
	*state.State
	screens map[state.ScreenID]Screen // every screen, by its ID; read-only once made

	// Main thread only.
	cancel     context.CancelFunc // cancels Ctx, to stop the goroutines
	touchStart image.Point        // where the touch going on started
	longPress  bool               // the touch going on was held: its end is no tap
}

// Options replace what the app reads from the system to show it. See state.Options.
type Options = state.Options

// App is the app as pocketbook runs it, and the background work in progress, for the emulator.
type App interface {
	pocketbook.App
	// BackgroundWork returns the names of the work in progress off the main thread (none when
	// the app is idle), and when some last ended. See state.Work.
	BackgroundWork() (names []string, lastEnded time.Time)
}

// New returns the app, which keeps its files in dir and connects to the servers of its
// config.json with open. kindOf tells how to show each kind of server.
func New(dir string, open servers.Opener, kindOf servers.KindOf, options Options) App {
	appState := state.New(dir, open, kindOf, options)
	return &application{
		State: appState,
		screens: map[state.ScreenID]Screen{
			state.LoadingScreen:   startup.NewLoading(appState),
			state.ServersScreen:   startup.NewServers(appState),
			state.SeriesScreen:    serieslist.New(appState),
			state.BooksScreen:     booklist.New(appState),
			state.ReaderScreen:    reader.New(appState),
			state.CoverScreen:     coverview.New(appState),
			state.AskDeleteScreen: reader.NewDeleteQuestion(appState),
			state.ContentsScreen:  contents.New(appState),
		},
	}
}

// Init draws the loading screen and starts the goroutines: the session's loading, the
// preloader of pages and the loader of covers.
func (app *application) Init() error {
	size := pocketbook.ScreenSize()
	widgets.SetScreen(min(size.X, size.Y))
	app.Fonts = widgets.OpenFonts()
	log.Printf("screen %v, font %s", size, app.Fonts.Body.Name)
	// On the reader, the firmware's fonts are those book text may use.
	if names, err := fonts.FirmwareFonts(); err == nil {
		log.Printf("firmware fonts: %v", names)
	}
	app.PreloadWake = state.NewWakeup(&app.Work, "preloading pages")
	app.CoverWake = state.NewWakeup(&app.Work, "loading covers")
	app.Covers.Loaded = map[state.CoverKey]*image.Gray{}
	app.Covers.Failed = map[state.CoverKey]bool{}
	app.ServerChoice = make(chan int, 1)
	app.Syncer = session.NewSyncer(app.State)
	app.Loading.Status = "Loading series…"
	app.Draw()

	app.Ctx, app.cancel = context.WithCancel(context.Background())
	app.Go(state.LoadingWork, func() { session.Load(app.State) })
	app.GoLoop(func() { openbook.Preloader(app.State) })
	app.GoLoop(func() { covers.Loader(app.State) })
	return nil
}

// BackgroundWork returns the work in progress off the main thread. See App.
func (app *application) BackgroundWork() (names []string, lastEnded time.Time) {
	return app.Work.InProgress()
}

// Close stops the goroutines, saves the open book's progress and makes a last attempt to send
// it to the server. It returns the error that stopped loading, if any.
func (app *application) Close() error {
	app.Lock()
	app.Repaint.Closing = true
	app.Unlock()
	app.cancel()
	app.Wait()
	app.Syncer.Stop()
	app.Lock()
	openbook.SaveProgressLocked(app.State)
	app.Unlock()
	if app.Server.Online {
		app.Syncer.PushNow()
	}
	if app.StopNetwork != nil {
		app.StopNetwork()
	}
	app.Fonts.Close()
	return app.Loading.Err
}

// Draw draws the screen shown and refreshes the panel. The refresh is a full one, which
// flashes, when the screen changed or the app came back to the foreground; else it is a
// partial one.
func (app *application) Draw() {
	defer timing.Start(context.Background(), "draw").End()
	app.Lock()
	defer app.Unlock()
	// A Draw that Update did not ask for comes from the start or from the app coming back to the
	// foreground: the panel may show anything, so refresh it whole.
	fullRefresh := app.Repaint.FullRefresh || !app.Repaint.Queued
	app.Repaint.FullRefresh = false
	app.Repaint.Queued = false
	app.Targets.Clear()

	screen := app.screens[app.Screen]
	if self, ok := screen.(selfRefreshing); ok {
		self.DrawAndRefreshLocked(fullRefresh)
		return
	}
	// The panel no longer shows the book's page: the reader refreshes it whole when it comes
	// back.
	app.Reader.Shown = state.Shown{}
	pocketbook.ClearScreen()
	screen.DrawLocked()
	if fullRefresh {
		pocketbook.FlashRefresh()
	} else {
		pocketbook.Refresh(pocketbook.Screen(), pocketbook.GC16)
	}
}
