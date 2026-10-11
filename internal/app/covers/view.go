package covers

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// ShowFull shows the cover of series or book id on the whole screen, if the list it was
// tapped on, from, still shows. The screen says it is loading until the cover is there.
func ShowFull(app *state.State, from state.ScreenID, kind shelf.CoverKind, id string) {
	shown, serial := false, 0
	app.Update(true, func() bool {
		if app.Screen != from {
			return false
		}
		app.CoverView.From = app.Screen
		app.CoverView.Image = nil
		app.CoverView.Err = nil
		app.CoverView.Serial++
		serial = app.CoverView.Serial
		app.ShowLocked(state.CoverScreen)
		shown = true
		return true
	})
	if shown {
		app.Go("loading the cover", func() { loadFull(app, serial, kind, id) })
	}
}

// loadFull loads the cover ShowFull asked for, the serial-th, and shows it.
func loadFull(app *state.State, serial int, kind shelf.CoverKind, id string) {
	ctx, cancel := context.WithTimeout(app.Ctx, state.RequestTimeout)
	span := timing.Start(ctx, "cover view")
	cover, err := firstPage(ctx, app, kind, id)
	duration := span.End()
	cancel()
	if app.Ctx.Err() != nil {
		return
	}
	log.Printf("cover view of %s %s in %v: %v", kind, id, duration, err)
	app.Update(false, func() bool {
		if app.Screen != state.CoverScreen || app.CoverView.Serial != serial {
			return false // the reader went back, or asked for another cover
		}
		app.CoverView.Image = cover
		app.CoverView.Err = err
		return true
	})
}

// firstPage returns page 1 of book id, or of the first book of series id, fitted to the
// screen: from the reader if the book is downloaded, else from the server.
func firstPage(ctx context.Context, app *state.State, kind shelf.CoverKind, id string) (*image.Gray, error) {
	app.Lock()
	client, online, words := app.Server.Client, app.Server.Online, app.WordsLocked()
	app.Unlock()
	bookID := id
	if kind == shelf.SeriesCover {
		var err error
		if bookID, err = firstBookOfSeries(ctx, app.Store, client, online, words, id); err != nil {
			return nil, err
		}
	}
	if book, ok := app.Store.Book(bookID); ok {
		return app.Store.LoadFirstPage(book)
	}
	if !online {
		return nil, errors.New("offline, and the book isn't downloaded")
	}
	data, err := client.FirstPage(ctx, bookID)
	if err != nil {
		return nil, err
	}
	size := pocketbook.ScreenSize()
	packed, err := pageimage.Prepare(data, size.X, size.Y, nil)
	if err != nil {
		return nil, err
	}
	return pageimage.Unpack(packed)
}

// firstBookOfSeries returns the ID of the first book of series seriesID: on the server when
// online, else among the downloaded books. Its errors name series and books with words.
func firstBookOfSeries(ctx context.Context, store shelf.Store, client servers.Server, online bool, words servers.Words, seriesID string) (string, error) {
	if online {
		books, err := client.Books(ctx, seriesID)
		if err != nil {
			return "", err
		}
		if len(books) == 0 {
			return "", fmt.Errorf("the %s has no %s", words.Series, words.BookPlural)
		}
		return books[0].ID, nil
	}
	local := store.Books() // sorted by series, then by number
	index := slices.IndexFunc(local, func(book shelf.Book) bool { return book.SeriesID == seriesID })
	if index < 0 {
		return "", fmt.Errorf("offline, and none of its %s is downloaded", words.BookPlural)
	}
	return local[index].ID, nil
}
