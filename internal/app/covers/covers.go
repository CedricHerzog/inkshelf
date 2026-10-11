// Package covers shows the covers of series and books: the small ones in the lists, and one on
// the whole screen.
//
// A list never waits for its covers. Drawing a row draws the cover if it is in memory, or a gray
// frame in its place and a request for it. Once the list is drawn, the loader goroutine wakes
// up, reads the covers asked for from the reader's disk, then fetches from the server those
// missing or stale, a few at a time, and asks for a redraw after each batch. The covers of the
// next page are asked for too, so that turning the page shows them at once.
//
// A cover on the whole screen is the first page of the book, sharper than the server's small
// thumbnail.
package covers

import (
	"context"
	"image"
	"log"
	"slices"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// The size of the covers in the lists, in InkPad 3 pixels.
const (
	Width  = 140
	Height = 210
)

// batchSize is how many covers are fetched between two redraws.
const batchSize = 3

// maxInMemory is how many covers are kept in memory, about 30 KB each, before those away from
// the list page shown are dropped.
const maxInMemory = 60

// Loader loads the covers the lists lack whenever drawing a list wakes it, until the app
// closes. It runs on a goroutine of its own.
func Loader(app *state.State) {
	for {
		select {
		case <-app.Ctx.Done():
			return
		case <-app.CoverWake.Woken():
		}
		closing := !loadWanted(app)
		app.CoverWake.Done()
		if closing {
			return
		}
	}
}

// loadWanted loads the covers the last Draw asked for, from the reader or else from the
// server. It returns false if the app is closing.
func loadWanted(app *state.State) bool {
	app.Lock()
	wants := takeWantsLocked(app)
	client, online := app.Server.Client, app.Server.Online
	app.Unlock()

	stale := loadFromDisk(app, wants, online)
	for batch := range slices.Chunk(stale, batchSize) {
		if !fetchBatch(app, client, batch) {
			return false
		}
	}
	return true
}

// takeWantsLocked returns the covers the last Draw asked for, without those the loader got or
// failed to get since.
func takeWantsLocked(app *state.State) []state.CoverRequest {
	return slices.DeleteFunc(slices.Clone(app.Covers.Wanted), func(want state.CoverRequest) bool {
		return app.Covers.Loaded[want.CoverKey] != nil || app.Covers.Failed[want.CoverKey]
	})
}

// loadFromDisk adds the covers of wants that are on the reader, and returns those to fetch from
// the server, missing or stale, when it is online.
func loadFromDisk(app *state.State, wants []state.CoverRequest, online bool) (stale []state.CoverRequest) {
	loaded := map[state.CoverKey]*image.Gray{}
	for _, want := range wants {
		if cover, err := app.CoverFiles.Load(want.Kind, want.ID); err == nil {
			loaded[want.CoverKey] = cover
		}
		if online && app.CoverFiles.Stale(want.Kind, want.ID, want.Modified) {
			stale = append(stale, want)
		}
	}
	add(app, wants, loaded, nil, false)
	return stale
}

// fetchBatch fetches the covers of batch from the server and adds them. It returns false if the
// app is closing.
func fetchBatch(app *state.State, client servers.Server, batch []state.CoverRequest) bool {
	loaded, failed := map[state.CoverKey]*image.Gray{}, map[state.CoverKey]bool{}
	for _, want := range batch {
		ctx, cancel := context.WithTimeout(app.Ctx, state.RequestTimeout)
		err := app.CoverFiles.Fetch(ctx, client, want.Kind, want.ID, widgets.Px(Width), widgets.Px(Height), want.Modified)
		cancel()
		if app.Ctx.Err() != nil {
			return false
		}
		if err != nil {
			log.Printf("cover of %s %s: %v", want.Kind, want.ID, err)
			failed[want.CoverKey] = true
			continue
		}
		if cover, err := app.CoverFiles.Load(want.Kind, want.ID); err == nil {
			loaded[want.CoverKey] = cover
		}
	}
	add(app, batch, loaded, failed, true)
	return true
}

// add adds the loaded covers of wants, replacing those in memory if replace is set, and notes
// the failed ones. It redraws the list when a cover on the page shown changed.
func add(app *state.State, wants []state.CoverRequest, loaded map[state.CoverKey]*image.Gray, failed map[state.CoverKey]bool, replace bool) {
	app.Update(false, func() bool {
		changed := false
		for _, want := range wants {
			if failed[want.CoverKey] {
				app.Covers.Failed[want.CoverKey] = true
			}
			cover := loaded[want.CoverKey]
			if cover == nil || (!replace && app.Covers.Loaded[want.CoverKey] != nil) {
				continue
			}
			app.Covers.Loaded[want.CoverKey] = cover
			changed = changed || want.Shown
		}
		return changed && (app.Screen == state.SeriesScreen || app.Screen == state.BooksScreen)
	})
}

// SaveForOffline fetches the covers of a book just downloaded and of its series, unless the
// reader has them: offline, the lists show the downloaded books with their covers.
func SaveForOffline(app *state.State, book shelf.Book) {
	for _, key := range []state.CoverKey{{Kind: shelf.BookCover, ID: book.ID}, {Kind: shelf.SeriesCover, ID: book.SeriesID}} {
		if !app.CoverFiles.Stale(key.Kind, key.ID, time.Time{}) {
			continue
		}
		ctx, cancel := context.WithTimeout(app.Ctx, state.RequestTimeout)
		if err := app.CoverFiles.Fetch(ctx, app.Server.Client, key.Kind, key.ID, widgets.Px(Width), widgets.Px(Height), time.Time{}); err != nil {
			log.Printf("cover of %s %s: %v", key.Kind, key.ID, err)
		}
		cancel()
	}
}

// DrawLocked draws the cover of request centred in box, or a gray frame in its place while it
// is not in memory, and then asks the loader for it.
func DrawLocked(app *state.State, box image.Rectangle, request state.CoverRequest) {
	cover := app.Covers.Loaded[request.CoverKey]
	if cover == nil {
		widgets.Frame(box, widgets.Px(2), widgets.LightGray)
		if !app.Covers.Failed[request.CoverKey] {
			app.Covers.Wanted = append(app.Covers.Wanted, request)
		}
		return
	}
	size := cover.Bounds().Size()
	pocketbook.DrawImage(cover, box.Min.Add(box.Size().Sub(size).Div(2)))
}

// RequestNextPageLocked asks for the covers of the list page after the one shown, which
// starts at first in requests, so that they show as soon as it is turned to. It wakes the
// loader if any cover is missing.
func RequestNextPageLocked(app *state.State, requests []state.CoverRequest, first int) {
	rowsPerPage := app.List.Rows
	for _, request := range requests[min(first+rowsPerPage, len(requests)):min(first+2*rowsPerPage, len(requests))] {
		if app.Covers.Loaded[request.CoverKey] == nil && !app.Covers.Failed[request.CoverKey] {
			app.Covers.Wanted = append(app.Covers.Wanted, request)
		}
	}
	if len(app.Covers.Wanted) > 0 {
		app.CoverWake.Wake()
	}
}

// PruneLocked drops the covers that are not on the list page shown or the pages around it,
// once more than maxInMemory are in memory. Drawing the list does it, as only it knows the
// pages' rows.
func PruneLocked(app *state.State, requests []state.CoverRequest, first int) {
	if len(app.Covers.Loaded) <= maxInMemory {
		return
	}
	rowsPerPage := app.List.Rows
	keep := map[state.CoverKey]bool{}
	for _, request := range requests[max(first-rowsPerPage, 0):min(first+2*rowsPerPage, len(requests))] {
		keep[request.CoverKey] = true
	}
	for key := range app.Covers.Loaded {
		if !keep[key] {
			delete(app.Covers.Loaded, key)
		}
	}
}
