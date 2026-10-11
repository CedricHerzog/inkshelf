package app

import (
	"encoding/json"
	"image"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/servers/calibreweb/fakecalibreweb"
	"github.com/CedricHerzog/inkshelf/internal/servers/registry"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// These benchmarks run the whole app on an emulated InkPad 3, its event loop included, against
// the emulator's fake Calibre-Web, which has both a CBZ comic and a reflowable EPUB. They
// measure what a reader waits for: from the button press to the panel refreshed.
//
// Only the event loop may call the app, so the benchmark hands it its input through
// mainThread, which runs it from the loop's next Draw, then reports when the Draw that the
// input asked for is over. The app's background work (the preloader) runs as on the reader,
// and the benchmark waits for it between iterations, with the timer stopped.

// benchmarkScreen is the InkPad 3's screen, which the books are downloaded for.
var benchmarkScreen = image.Pt(1404, 1872)

// benchmarkTextSize is the font size of book text the benchmark reads at: the app's default,
// set explicitly so that the cold benchmark can lay the book out again at the same size.
const benchmarkTextSize = 44

// Series and their first books of the fake Calibre-Web (see its catalog.go).
const (
	comicSeriesID = "2" // Ruines: Tome 1 is a CBZ of 8 pages
	epubSeriesID  = "1" // Les Étoiles: Tome 1 is an EPUB of 3 chapters
)

// benchmarkTimeout bounds every wait, so that a benchmark that would hang fails instead.
const benchmarkTimeout = time.Minute

// addFontsOnce adds the emulator's fonts once per process: fonts.AddDir prepends to a list.
var addFontsOnce sync.Once

// mainThread is the app as the event loop runs it, with a way for the benchmark to run input on
// the loop's goroutine and learn when the Draw that follows it is over.
type mainThread struct {
	App
	// inputs are run by the next Draw, in place of drawing: each must ask for a repaint.
	inputs chan func()
	// drawn receives a value once the Draw after an input is over.
	drawn chan struct{}
	// awaitingDraw is true from an input until the next Draw. Event loop only.
	awaitingDraw bool
}

// Draw runs the input waiting, if any, else draws the screen, and reports the draw that
// follows an input. The event loop calls it on a repaint, which the benchmark asks for to hand
// it an input, and which the input asks for in turn as the app's input does.
func (thread *mainThread) Draw() {
	select {
	case input := <-thread.inputs:
		input()
		thread.awaitingDraw = true
		return
	default:
	}
	thread.App.Draw()
	if thread.awaitingDraw {
		thread.awaitingDraw = false
		thread.drawn <- struct{}{}
	}
}

// harness is the app running on an emulated reader, with the books of the fake server
// downloaded.
type harness struct {
	emulator *pocketbook.Emulator
	thread   *mainThread
	app      *application
	comic    shelf.Book
	epub     shelf.Book
	// firstTextPage is the first page of the open EPUB, where the benchmark goes back to once
	// it has read to the last page. Guarded by the state's mutex.
	firstTextPage reading.PagePosition
	stop          func()
}

// startHarness downloads the fake server's comic and EPUB, as the download queue does, then
// starts the app on an emulated reader and waits until it shows the series list, idle.
func startHarness(b *testing.B) *harness {
	b.Helper()
	addFontsOnce.Do(func() { fonts.AddDir(filepath.Join("..", "..", "emulator", "fonts")) })
	// The app logs every page turn; on the reader its log goes to a file too.
	logFile, err := os.Create(filepath.Join(b.TempDir(), "inkshelf.log"))
	if err != nil {
		b.Fatal(err)
	}
	log.SetOutput(logFile)

	server := httptest.NewServer(fakecalibreweb.NewHandler())
	config := servers.Config{Name: "Fake", Type: "calibreweb", Server: server.URL, Username: "fake", Password: "fake"}
	appDir := b.TempDir()
	writeBenchmarkConfig(b, appDir, config)
	open := registry.Opener(server.Client())
	store := shelf.Store{Dir: filepath.Join(appDir, "servers", config.Name, "books")}
	setup := &harness{
		comic: downloadFirstBook(b, open, config, store, comicSeriesID),
		epub:  downloadFirstBook(b, open, config, store, epubSeriesID),
	}

	setup.emulator = pocketbook.NewEmulator(benchmarkScreen, false)
	setup.app = New(appDir, open, registry.KindOf, Options{}).(*application)
	setup.thread = &mainThread{App: setup.app, inputs: make(chan func(), 1), drawn: make(chan struct{})}
	finished := make(chan error, 1)
	go func() { finished <- pocketbook.RunEmulated(setup.thread, setup.emulator) }()
	setup.stop = func() {
		pocketbook.Exit()
		if err := <-finished; err != nil {
			b.Errorf("app: %v", err)
		}
		server.Close()
		log.SetOutput(os.Stderr)
		if err := logFile.Close(); err != nil {
			b.Error(err)
		}
	}

	setup.waitUntil(b, func() bool { return setup.app.Screen == state.SeriesScreen || setup.app.Loading.Err != nil })
	setup.app.Lock()
	loadErr := setup.app.Loading.Err
	setup.app.Unlock()
	if loadErr != nil {
		setup.stop()
		b.Fatalf("loading: %v", loadErr)
	}
	setup.app.Update(false, func() bool {
		setup.app.Preferences.TextSize = benchmarkTextSize
		return false
	})
	return setup
}

// writeBenchmarkConfig writes the app's config.json, with the one server config.
func writeBenchmarkConfig(b *testing.B, appDir string, config servers.Config) {
	b.Helper()
	data, err := json.Marshal([]servers.Config{config})
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "config.json"), data, 0o644); err != nil {
		b.Fatal(err)
	}
}

// downloadFirstBook downloads the first book of series seriesID into store, for the screen,
// and returns it as saved.
func downloadFirstBook(b *testing.B, open servers.Opener, config servers.Config, store shelf.Store, seriesID string) shelf.Book {
	b.Helper()
	server, err := open(config)
	if err != nil {
		b.Fatal(err)
	}
	books, err := server.Books(b.Context(), seriesID)
	if err != nil {
		b.Fatal(err)
	}
	first := books[0]
	book := shelf.Book{ID: first.ID, SeriesID: seriesID, Title: first.Title, Format: first.Format}
	if err := store.Download(b.Context(), server, book, benchmarkScreen.X, benchmarkScreen.Y, func(shelf.DownloadProgress) {}); err != nil {
		b.Fatalf("downloading %s: %v", book.Title, err)
	}
	saved, ok := store.Book(book.ID)
	if !ok {
		b.Fatalf("%s is not on the shelf after its download", book.Title)
	}
	return saved
}

// waitUntil waits until done, read with the state's mutex held, is true and the app is idle:
// no background work, no input or repaint waiting.
func (setup *harness) waitUntil(b *testing.B, done func() bool) {
	b.Helper()
	deadline := time.Now().Add(benchmarkTimeout)
	for time.Now().Before(deadline) {
		setup.app.Lock()
		reached := done()
		setup.app.Unlock()
		if names, _ := setup.app.BackgroundWork(); reached && len(names) == 0 && !setup.emulator.Pending() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	b.Fatalf("the app is not idle after %v", benchmarkTimeout)
}

// waitIdle waits until the app has no background work, input or repaint waiting.
func (setup *harness) waitIdle(b *testing.B) {
	b.Helper()
	setup.waitUntil(b, func() bool { return true })
}

// inputAndDraw runs input on the event loop, then waits until the Draw it asked for is over.
func (setup *harness) inputAndDraw(b *testing.B, input func()) {
	b.Helper()
	setup.thread.inputs <- input
	pocketbook.Repaint()
	select {
	case <-setup.thread.drawn:
	case <-time.After(benchmarkTimeout):
		b.Fatalf("no draw after the input in %v", benchmarkTimeout)
	}
}

// openBook opens book on the event loop, as a tap on its row does, and waits until its first
// page is shown and the pages around it preloaded.
func (setup *harness) openBook(b *testing.B, book shelf.Book) {
	b.Helper()
	setup.inputAndDraw(b, func() {
		setup.app.Update(true, func() bool {
			openbook.OpenLocked(setup.app.State, state.BookItem{Book: book, Downloaded: true})
			if setup.app.Reader.Text != nil {
				setup.firstTextPage = setup.app.Reader.Text.Book.At()
			}
			return true
		})
	})
	setup.waitIdle(b)
	setup.app.Lock()
	opened := setup.app.Screen == state.ReaderScreen && setup.app.Reader.OpenBook.ID == book.ID
	setup.app.Unlock()
	if !opened {
		b.Fatalf("%s did not open", book.Title)
	}
}

// closeBook closes the open book on the event loop, as the toolbar's Back does, and waits
// until the book list shows.
func (setup *harness) closeBook(b *testing.B) {
	b.Helper()
	setup.inputAndDraw(b, func() {
		setup.app.Update(true, func() bool {
			openbook.CloseLocked(setup.app.State)
			return true
		})
	})
	setup.waitIdle(b)
}

// turnPage presses and releases the Next button, as the event loop passes them on.
func (setup *harness) turnPage() {
	setup.app.Key(pocketbook.KeyEvent{Key: pocketbook.KeyNext, State: pocketbook.KeyStatePress})
	setup.app.Key(pocketbook.KeyEvent{Key: pocketbook.KeyNext, State: pocketbook.KeyStateRelease})
}

// rewindAtLastPage goes back to the first page of the open book once it shows its last, and
// waits until that page is drawn and its neighbours preloaded, so that the next turn goes
// forward, as reading does. Turning back would measure the preloader going backwards instead.
func (setup *harness) rewindAtLastPage(b *testing.B) {
	b.Helper()
	setup.app.Lock()
	reader := &setup.app.Reader
	atLastPage := reader.OpenBook.Page == reader.OpenBook.Pages
	if reader.Text != nil {
		atLastPage = reader.Text.Book.AtLastPage()
	}
	setup.app.Unlock()
	if !atLastPage {
		return
	}
	setup.inputAndDraw(b, func() {
		setup.app.Update(true, func() bool {
			reader := &setup.app.Reader
			if reader.Text == nil {
				reader.OpenBook.Page = 1
				return true
			}
			location, err := reader.Text.Book.Location(setup.firstTextPage)
			if err != nil {
				reader.PageErr = err
				return true
			}
			openbook.JumpTextLocked(setup.app.State, location)
			return true
		})
	})
	setup.waitIdle(b)
}

// forgetPreloaded drops what the preloader got ready, so that the next turn finds nothing
// ready, as when the reader turns faster than the preloader: for a comic, the decoded pages;
// for an EPUB, the pages drawn and the chapters laid out, the one shown laid out again.
func (setup *harness) forgetPreloaded(b *testing.B) {
	b.Helper()
	var relayoutErr error
	setup.app.Update(false, func() bool {
		reader := &setup.app.Reader
		if reader.Text == nil {
			clear(reader.Pages)
			return false
		}
		location, err := reader.Text.Book.Location(reader.Text.Book.At())
		if err == nil {
			err = reader.Text.Book.Relayout(openbook.TextSettings(benchmarkTextSize), location)
		}
		relayoutErr = err
		return false
	})
	if relayoutErr != nil {
		b.Fatal(relayoutErr)
	}
}

// BenchmarkPageTurn measures a page turn from the page button to the panel refreshed: the
// app's Key, then the Draw that draws the page and refreshes the panel. It reads forward and
// goes back to the first page after the last, untimed. "preloaded" finds the page ready, as
// when the reader takes the time to read it; "cold" finds nothing ready, as when the reader
// turns pages faster than the preloader, which is what a reader feels most.
func BenchmarkPageTurn(b *testing.B) {
	setup := startHarness(b)
	defer setup.stop()
	for _, book := range []struct {
		name string
		book shelf.Book
	}{{"comic", setup.comic}, {"epub", setup.epub}} {
		b.Run(book.name, func(b *testing.B) {
			setup.openBook(b, book.book)
			defer setup.closeBook(b)
			for _, cold := range []bool{false, true} {
				name := "preloaded"
				if cold {
					name = "cold"
				}
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					b.StopTimer()
					for range b.N {
						setup.waitIdle(b)
						setup.rewindAtLastPage(b)
						if cold {
							setup.forgetPreloaded(b)
						}
						b.StartTimer()
						setup.inputAndDraw(b, setup.turnPage)
						b.StopTimer()
					}
				})
			}
		})
	}
}

// BenchmarkOpenBook measures opening a downloaded book from the book list: reading its
// book.json, opening the EPUB and laying out its chapter, then drawing its first page.
func BenchmarkOpenBook(b *testing.B) {
	setup := startHarness(b)
	defer setup.stop()
	for _, book := range []struct {
		name string
		book shelf.Book
	}{{"comic", setup.comic}, {"epub", setup.epub}} {
		b.Run(book.name, func(b *testing.B) {
			b.ReportAllocs()
			b.StopTimer()
			for range b.N {
				b.StartTimer()
				setup.inputAndDraw(b, func() {
					setup.app.Update(true, func() bool {
						openbook.OpenLocked(setup.app.State, state.BookItem{Book: book.book, Downloaded: true})
						return true
					})
				})
				b.StopTimer()
				setup.waitIdle(b)
				setup.closeBook(b)
			}
		})
	}
}
