package openbook

import (
	"context"
	"image"
	"log"
	"slices"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// textSizes are the font sizes of book text, in InkPad 3 pixels, smallest first.
var textSizes = []int{32, 36, 40, 44, 50, 56, 64}

// defaultTextSize is the font size of book text until the reader picks one.
const defaultTextSize = 44

// The margins of a page of book text, and its line spacing, in InkPad 3 pixels.
const (
	TextMarginSide   = 90
	TextMarginTop    = 80
	TextMarginBottom = 100
	TextLineSpacing  = 135 // in percent of the font size
)

// textFullEvery is how many page turns of book text go between two refreshes of the whole
// panel: text ghosts less than pictures do.
const textFullEvery = 6

// openTextLocked opens a Reflowable book, laid out at the reader's text size, at its location.
func openTextLocked(app *state.State, book shelf.Book) (*state.TextReader, error) {
	opened, err := reading.Open(app.Store, book, TextSettings(textSizeLocked(app)))
	if err != nil {
		return nil, err
	}
	return &state.TextReader{Book: opened}, nil
}

// textSizeLocked returns the font size of book text the reader picked.
func textSizeLocked(app *state.State) int {
	if slices.Contains(textSizes, app.Preferences.TextSize) {
		return app.Preferences.TextSize
	}
	return defaultTextSize
}

// moveTextLocked moves the open Reflowable book to the next or previous page, and reports
// whether there was one. It updates the book's location, virtual page and completion.
func moveTextLocked(app *state.State, forward bool) bool {
	moved, err := app.Reader.Text.Book.Turn(forward)
	if err != nil {
		app.Reader.PageErr = err
		return true
	}
	if !moved {
		return false
	}
	NoteTextLocationLocked(app)
	return true
}

// NoteTextLocationLocked records the page shown of the open Reflowable book as its progress.
func NoteTextLocationLocked(app *state.State) {
	book := &app.Reader.OpenBook
	if err := app.Reader.Text.Book.UpdateProgress(book); err != nil {
		app.Reader.PageErr = err
		return
	}
	book.Unsynced = true
	book.ReadAt = time.Now()
	app.Reader.Unsaved = true
	app.Reader.PageErr = nil
}

// atLastTextPageLocked reports whether the open Reflowable book shows its last page.
func atLastTextPageLocked(app *state.State) bool {
	return app.Reader.Text.Book.AtLastPage()
}

// CurrentTextPageLocked returns the page shown of the open Reflowable book, drawn now if the
// preloader had not, and how long that took. If it cannot be drawn, the reader's PageErr says
// why.
func CurrentTextPageLocked(app *state.State) (*image.Gray, time.Duration) {
	text := app.Reader.Text.Book
	at := text.At()
	if pageImage := text.Drawn(at); pageImage != nil || app.Reader.PageErr != nil {
		return pageImage, 0
	}
	span := timing.Start(app.Reader.Turn.Context(), "load page")
	pageImage, err := text.Draw(at)
	loadTime := span.End()
	if err != nil {
		app.Reader.PageErr = err
		return nil, loadTime
	}
	text.KeepDrawn(at, pageImage)
	return pageImage, loadTime
}

// PruneTextPagesLocked drops the pages drawn other than the page shown and its neighbours, and
// the layouts of the chapters other than its own and the two around it. It lays nothing out,
// as drawing the page calls it.
func PruneTextPagesLocked(app *state.State) {
	app.Reader.Text.Book.Prune()
}

// preloadText draws the pages around the one shown of the open Reflowable book, the next one
// first. It releases the mutex while it draws, which takes a while: the reader may turn the
// page meanwhile.
func preloadText(app *state.State) {
	app.Lock()
	text := app.Reader.Text
	if text == nil {
		app.Unlock()
		return
	}
	at := text.Book.At()
	app.Unlock()
	for _, neighbour := range text.Book.Neighbours(at) {
		app.Lock()
		wanted := app.Reader.Text == text && text.Book.WantsDrawn(neighbour, at)
		app.Unlock()
		if !wanted {
			return
		}
		span := timing.Start(context.Background(), "preload page")
		pageImage, err := text.Book.Draw(neighbour)
		span.End()
		if err != nil {
			return // the page shows the error if it is turned to
		}
		app.Lock()
		if app.Reader.Text == text && text.Book.At() == at {
			text.Book.KeepDrawn(neighbour, pageImage)
		}
		app.Unlock()
	}
}

// ChangeTextSize makes book text one size larger or smaller, saves the size in the
// preferences, and lays the open book out again at the same place.
func ChangeTextSize(app *state.State, larger bool) {
	var saved state.Preferences
	changed := false
	app.Update(true, func() bool {
		text := app.Reader.Text
		if app.Screen != state.ReaderScreen || text == nil {
			return false
		}
		index := slices.Index(textSizes, textSizeLocked(app))
		if larger {
			index++
		} else {
			index--
		}
		if index < 0 || index >= len(textSizes) {
			return false
		}
		location, err := text.Book.Location(text.Book.At())
		if err != nil {
			app.Reader.PageErr = err
			return true
		}
		app.Preferences.TextSize = textSizes[index]
		saved, changed = app.Preferences, true
		if err := text.Book.Relayout(TextSettings(app.Preferences.TextSize), location); err != nil {
			app.Reader.PageErr = err
		}
		app.Reader.Shown.Valid = false
		return true
	})
	if changed {
		if err := saved.Save(app.ServerDir); err != nil {
			log.Printf("saving filters: %v", err)
		}
	}
}

// JumpTextLocked moves the open Reflowable book to location. The book keeps the page left, for
// the toolbar's Return.
func JumpTextLocked(app *state.State, location shelf.Location) {
	if err := app.Reader.Text.Book.Jump(location); err != nil {
		app.Reader.PageErr = err
		return
	}
	NoteTextLocationLocked(app)
}

// TextSettings returns how book text is laid out on the screen at size, in InkPad 3 pixels.
// The footnotes that a chapter links to are left out, as its links show them in a popup.
func TextSettings(size int) layout.Settings {
	screenSize := pocketbook.ScreenSize()
	return layout.Settings{
		Width: screenSize.X, Height: screenSize.Y,
		Margins:       image.Rect(widgets.Px(TextMarginSide), widgets.Px(TextMarginTop), widgets.Px(TextMarginSide), widgets.Px(TextMarginBottom)),
		FontSize:      widgets.Px(size),
		LineSpacing:   TextLineSpacing,
		Justify:       true,
		HideFootnotes: true,
	}
}
