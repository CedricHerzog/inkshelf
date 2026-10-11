package app

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
)

// Screen is a screen of the app. The app draws the screen shown and passes it the commands of
// the reader's buttons; touches go to the targets it registered while drawing.
type Screen interface {
	// DrawLocked draws the screen into the canvas, whole, and registers its targets. It runs
	// with the state's mutex held; the app then refreshes the panel.
	DrawLocked()
	// Command performs a command of the reader's buttons, or of a tap that acts as one. It runs
	// without the mutex.
	Command(order widgets.Command)
}

// selfRefreshing is a screen that refreshes the panel itself, only where it changed: the
// reader, whose page turns would flash and take longer if the whole panel were refreshed.
type selfRefreshing interface {
	// DrawAndRefreshLocked draws the screen and refreshes what changed of it, or the whole
	// panel if fullRefresh is set. It runs with the mutex held.
	DrawAndRefreshLocked(fullRefresh bool)
}

// swipeable is a screen whose pages turn with a swipe sideways: a swipe to the left is Next.
type swipeable interface {
	TurnsPagesOnSwipe()
}

// touchable is a screen that decides itself what each touch does, instead of hit-testing its
// targets: a tap anywhere on a cover goes back, a tap on the side of a book's page turns it.
type touchable interface {
	// TouchLocked returns what a touch from start to end does; swipe tells whether it was a
	// swipe sideways. It runs with the mutex held.
	TouchLocked(start, end image.Point, swipe bool) widgets.Target
}

// overlaid is a screen that shows overlays over itself, as the lists' hold menu and A–Z jump.
// Any tap or command first closes them, and a command does nothing else.
type overlaid interface {
	// CloseOverlaysLocked closes the overlays and reports whether one was open. It runs with
	// the mutex held.
	CloseOverlaysLocked() bool
}
