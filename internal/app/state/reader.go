package state

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// Reader is the open book. Guarded by the mutex.
type Reader struct {
	OpenBook   shelf.Book
	Unsaved    bool                // OpenBook's progress is not on disk yet
	Pages      map[int]*image.Gray // its decoded pages: the current one and its neighbours
	Text       *TextReader         // a Reflowable book's layout and position; nil for pages
	PageErr    error               // why the current page cannot be shown
	Toolbar    bool                // the toolbar shows over the page
	AtEnd      bool                // past the last page: the end panel shows
	AutoQueued string              // the ID of the open book whose next one was queued
	Turns      int                 // page turns since the book opened, to clear the ghosting
	Shown      Shown               // what the panel shows of the open book
	Turn       timing.Pending      // the trace task of the page turn not drawn yet
}

// Shown is what the panel shows of the open book. A page turn refreshes only what changed: a
// refresh of the whole panel takes longer and flashes.
type Shown struct {
	Valid   bool // false: refresh everything
	Page    int
	Text    reading.PagePosition // of a Reflowable book, whose virtual page spans several
	Toolbar bool
	Rect    image.Rectangle // of the page image
	Popup   *NotePopup      // over a Reflowable book's page, nil if none
	PopupAt int             // the popup's page shown
}

// TextReader is the open Reflowable book as the reader shows it: the reading engine's book,
// and the links and the popup the screen adds over its page. Guarded by the mutex, as Reader.
// The book's renderer has a lock of its own, taken after the mutex and never before it.
type TextReader struct {
	Book  *reading.TextBook
	Links []layout.PlacedLink // of the page shown, which drawing it made targets
	Popup *NotePopup          // over the page shown, nil if none
}

// NotePopup is what a link shows over the page instead of jumping to it: a footnote, a part of
// the book that is no chapter, or a URL. Guarded by the mutex, as Reader.
type NotePopup struct {
	Pages  []layout.Page // its text, laid out
	Page   int           // the page shown, from 0
	Height int           // in pixels, less than the most for a single page of short text
	Drawn  *image.Gray   // the page shown, drawn; nil until it is drawn
}

// Contents is the table of contents of the open Reflowable book. Guarded by the mutex.
type Contents struct {
	Tab   int  // the list shown: the table of contents, the printed pages or the landmarks
	Page  int  // its page shown, from 0
	Focus bool // the next Draw turns to the page of the entry being read
}

// DeleteQuestion is the question whether to delete the book just finished. Guarded by the
// mutex.
type DeleteQuestion struct {
	Book shelf.Book // the book just finished
	Next *BookItem  // the book to open after the answer, if any
	Size int64      // the bytes the book takes on the reader
}
