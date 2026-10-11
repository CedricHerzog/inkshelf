package openbook

import (
	"context"
	"image"
	"log"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// Preloader saves the open book's progress and gets the pages around the one shown ready,
// whenever drawing the page shown wakes it, until the app closes. It runs on a goroutine of its
// own, so that a page turn does not wait for the disk.
func Preloader(app *state.State) {
	for {
		select {
		case <-app.Ctx.Done():
			return
		case <-app.PreloadWake.Woken():
		}
		preload(app)
		app.PreloadWake.Done()
	}
}

// preload saves the open book's progress and gets the pages around the one shown ready.
func preload(app *state.State) {
	app.Lock()
	book, save, isText := app.Reader.OpenBook, app.Reader.Unsaved, app.Reader.Text != nil
	app.Reader.Unsaved = false
	app.Unlock()
	if save {
		if err := app.Store.Save(book); err != nil {
			log.Printf("saving progress: %v", err)
		}
	}
	if isText {
		preloadText(app)
		return
	}
	// The next page first: it is the likely turn.
	for _, pageNumber := range []int{book.Page + 1, book.Page - 1} {
		if pageNumber >= 1 && pageNumber <= book.Pages {
			preloadPage(app, book.ID, pageNumber)
		}
	}
}

// preloadPage decodes page pageNumber of book bookID, unless it is decoded already, or the book
// closed or the reader turned away from it meanwhile.
func preloadPage(app *state.State, bookID string, pageNumber int) {
	app.Lock()
	wanted := app.Reader.Pages != nil && app.Reader.OpenBook.ID == bookID && app.Reader.Pages[pageNumber] == nil
	app.Unlock()
	if !wanted {
		return
	}
	span := timing.Start(context.Background(), "preload page")
	pageImage, err := app.Store.LoadPage(bookID, pageNumber)
	duration := span.End()
	if err != nil {
		return // the page shows the error if it is turned to
	}
	log.Printf("preload page %d: %v", pageNumber, duration)
	app.Lock()
	defer app.Unlock()
	current := app.Reader.OpenBook.Page
	if app.Reader.Pages != nil && app.Reader.OpenBook.ID == bookID && pageNumber >= current-1 && pageNumber <= current+1 {
		app.Reader.Pages[pageNumber] = pageImage
	}
}

// CurrentPageLocked returns the image of the page shown, and how long it took to load it if
// the preloader had not. If it cannot be loaded, the reader's PageErr says why.
func CurrentPageLocked(app *state.State) (*image.Gray, time.Duration) {
	pageNumber := app.Reader.OpenBook.Page
	pageImage := app.Reader.Pages[pageNumber]
	if pageImage != nil || app.Reader.PageErr != nil {
		return pageImage, 0
	}
	// Not preloaded: the book just opened, or the reader turns pages faster than the preloader.
	span := timing.Start(app.Reader.Turn.Context(), "load page")
	pageImage, app.Reader.PageErr = app.Store.LoadPage(app.Reader.OpenBook.ID, pageNumber)
	loadTime := span.End()
	if pageImage != nil {
		app.Reader.Pages[pageNumber] = pageImage
	}
	return pageImage, loadTime
}

// PrunePagesLocked drops the decoded pages other than the one shown and its neighbours.
func PrunePagesLocked(app *state.State) {
	current := app.Reader.OpenBook.Page
	for pageNumber := range app.Reader.Pages {
		if pageNumber < current-1 || pageNumber > current+1 {
			delete(app.Reader.Pages, pageNumber)
		}
	}
}
