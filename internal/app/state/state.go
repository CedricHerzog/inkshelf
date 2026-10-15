// Package state holds everything the app knows while it runs, and the one lock that guards it.
//
// The app has a single State. Its screens draw from it, and its background goroutines (loading
// the lists, downloading books, loading covers, preloading pages) change it. They meet through
// the State's mutex:
//
//   - The event loop calls the app on one goroutine, the main thread. Drawing happens there,
//     with the mutex held, so a screen always draws a consistent state.
//   - A goroutine changes the state only inside Update, which holds the mutex and asks the
//     event loop for a repaint when the change shows. It never draws.
//
// Functions whose name ends in Locked must be called with the mutex held, and never take it.
// A few fields are not guarded by the mutex because nobody changes them once the goroutines
// run, or because only the main thread touches them; their comments say so.
package state

import (
	"context"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/progress"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// ScreenID names a screen of the app. Only one shows at a time.
type ScreenID int

// The screens, each with what it shows.
const (
	LoadingScreen   ScreenID = iota // a status line, or the error that stopped loading
	ServersScreen                   // the servers of config.json, to choose one
	SeriesScreen                    // a page of series
	BooksScreen                     // a page of the books of the open series
	ReaderScreen                    // a page of the open book
	CoverScreen                     // the cover of a series or a book, on the whole screen
	AskDeleteScreen                 // whether to delete the book just finished
	ContentsScreen                  // the table of contents of the open Reflowable book
)

// RequestTimeout bounds every request to the server: the lists, the covers and the pages.
const RequestTimeout = 30 * time.Second

// State is the app's state. See the package documentation for who may read and change it.
type State struct {
	// mu guards every field of this first group, down to Repaint.
	mu          sync.Mutex
	Screen      ScreenID
	Targets     widgets.Targets // the targets of the screen shown, registered when it was drawn
	Loading     Loading         // for the loading and server screens
	Server      Server          // the server in use
	Preferences Preferences     // the lists' filters and the reader's settings
	Series      SeriesList      // the series list
	Books       BookList        // the books of the open series
	List        ListView        // the list shown: series or books
	Queue       DownloadQueue   // the books to download
	Reader      Reader          // the open book
	Covers      Covers          // the small covers of the lists
	CoverView   CoverView       // the cover shown on the whole screen
	Contents    Contents        // the table of contents of the open book
	AskDelete   DeleteQuestion  // whether to delete the book just finished
	Repaint     Repaint         // the redraws asked for

	// Set before the goroutines start, then read-only.
	Dir          string           // holds config.json and the servers' folders
	Open         servers.Opener   // connects to the server chosen
	KindOf       servers.KindOf   // the name, logo and words of each kind of server
	Options      Options          // the clock, free space and Wi-Fi, real unless replaced
	Ctx          context.Context  // cancelled when the app closes, to stop the goroutines
	PreloadWake  *Wakeup          // wakes the goroutine that preloads pages
	CoverWake    *Wakeup          // wakes the goroutine that loads covers
	ServerChoice chan int         // the server chosen on the server screen, by index
	Syncer       *progress.Syncer // sends the reading progress; its lock is taken before mu

	// The background work in progress, for the emulator. It has a lock of its own.
	Work Work

	// Set while loading, before the series list shows, then read-only.
	ServerDir  string // the folder of the server in use: its books, covers and filters.json
	Store      shelf.Store
	CoverFiles shelf.CoverFiles

	// Set while loading, read when the app closes, once the goroutines have stopped.
	StopNetwork func() // lets the Wi-Fi go; nil if it never connected

	// Main thread only.
	goroutines sync.WaitGroup // every goroutine started with Go or GoLoop
	Fonts      widgets.Fonts
	Logos      widgets.Logos
}

// Repaint is what the next Draw must do. Guarded by the mutex.
type Repaint struct {
	FullRefresh bool // the next Draw refreshes the whole panel, with a flash
	Queued      bool // a repaint is queued: the next Draw comes from Update
	Closing     bool // the app is closing: no more repaints
}

// Options replace what the app reads from the system to show it, so that the emulator's shots
// come out the same on every run, and the reader's Wi-Fi, so that the emulator can start
// without it. Their zero value is the real clock, free space and Wi-Fi.
type Options struct {
	// Now is the time shown in the lists' header.
	Now func() time.Time
	// FreeSpace is the space left on the disk that holds dir, and whether it is known.
	FreeSpace func(dir string) (int64, bool)
	// WiFiConnected reports whether the reader is connected to a network already.
	WiFiConnected func() bool
	// ConnectWiFi connects to a known network, as pocketbook.KeepWiFiOn does.
	ConnectWiFi func(ctx context.Context) (stop func(), err error)
}

// New returns the state of an app that keeps its files in dir and connects to the servers of
// its config.json with open. kindOf tells how to show each kind of server.
func New(dir string, open servers.Opener, kindOf servers.KindOf, options Options) *State {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.FreeSpace == nil {
		options.FreeSpace = shelf.FreeSpace
	}
	if options.WiFiConnected == nil {
		options.WiFiConnected = pocketbook.WiFiConnected
	}
	if options.ConnectWiFi == nil {
		options.ConnectWiFi = pocketbook.KeepWiFiOn
	}
	return &State{Dir: dir, Open: open, KindOf: kindOf, Options: options}
}

// Lock takes the mutex. With Unlock, it makes the State a sync.Locker.
func (app *State) Lock() { app.mu.Lock() }

// Unlock releases the mutex.
func (app *State) Unlock() { app.mu.Unlock() }

// Update runs change with the mutex held and, if change reports that something visible
// changed, asks for a redraw: a full one if fullRefresh is set. It is how a goroutine, or a
// tap, changes the state.
func (app *State) Update(fullRefresh bool, change func() bool) {
	app.mu.Lock()
	repaint := false
	if change() {
		app.Repaint.FullRefresh = app.Repaint.FullRefresh || fullRefresh
		// One queued repaint draws every change made before it runs.
		repaint = !app.Repaint.Queued && !app.Repaint.Closing
		app.Repaint.Queued = true
	}
	app.mu.Unlock()
	if repaint {
		pocketbook.Repaint()
	}
}

// ShowLocked switches to screen next. The targets of the screen left go, and so do the
// overlays of the lists: coming back to a list shows it plain.
func (app *State) ShowLocked(next ScreenID) {
	app.Screen = next
	app.Targets = nil
	app.Books.MenuOpen = false
	app.Series.JumpOpen = false
}

// Go runs task on a goroutine of its own, which the app waits for when it closes, and counts
// it as work named name while it runs (see Work). Only the main thread starts goroutines, so
// that none starts while the app waits for them.
func (app *State) Go(name string, task func()) {
	app.Work.Begin(name)
	app.GoLoop(func() {
		defer app.Work.End(name)
		task()
	})
}

// GoLoop runs task, a goroutine that waits for wake-ups until the app closes, as Go does but
// without counting it as work: only the wake-ups it handles count (see Wakeup).
func (app *State) GoLoop(task func()) {
	app.goroutines.Go(func() {
		task()
	})
}

// Wait waits for the goroutines started with Go or GoLoop to return. Cancelling Ctx asks them to.
func (app *State) Wait() { app.goroutines.Wait() }

// KindTitleLocked returns the name of the server's kind, or the app's before it is known.
func (app *State) KindTitleLocked() string { return widgets.KindTitle(app.Server.Kind) }

// WordsLocked returns how the server calls its series and books.
func (app *State) WordsLocked() servers.Words { return widgets.Words(app.Server.Kind) }
