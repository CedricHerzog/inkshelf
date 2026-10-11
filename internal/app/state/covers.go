package state

import (
	"image"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// Covers are the small covers of the lists. Guarded by the mutex.
type Covers struct {
	Loaded map[CoverKey]*image.Gray // those of about the list page shown
	Wanted []CoverRequest           // those the last Draw lacked, the page shown's first
	Failed map[CoverKey]bool        // not on the server: not asked for again
}

// CoverKey names a cover: of a series or of a book, and its ID.
type CoverKey struct {
	Kind shelf.CoverKind
	ID   string
}

// CoverRequest is a cover a list needs.
type CoverRequest struct {
	CoverKey
	Modified time.Time // of the series or book: a cover older than that is stale
	Shown    bool      // on the list page shown, not on the next one
}

// CoverView is a cover shown on the whole screen. Guarded by the mutex.
type CoverView struct {
	Image  *image.Gray // the cover shown, nil while it loads
	Err    error       // why it cannot be shown
	From   ScreenID    // the list to go back to
	Serial int         // counts the covers asked for, so that one loaded too late is dropped
}
