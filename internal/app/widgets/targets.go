package widgets

import "image"

// Command is what a button of the reader asks for. Some taps ask for the same: the Back
// button of a header, the page buttons of a list's footer, the sides of a book's page.
type Command int

// The commands; NoCommand is the zero value, a tap that asks for nothing.
const (
	NoCommand Command = iota
	Previous          // the previous page, of a list or of a book
	Next              // the next page
	Back              // back to the previous screen, or out of the app from the first ones
	Menu              // the reader's toolbar
)

// Target is an area of the screen and what a touch on it does. A screen registers its targets
// while it draws, so that they always match what the panel shows.
//
// The closures run after the app's lock is released, as they take it themselves.
type Target struct {
	Rect image.Rectangle
	// Tap runs when a tap starts and ends in Rect; nil does nothing.
	Tap func()
	// Hold runs when a touch in Rect is held; nil does nothing.
	Hold func()
	// Command, when set, makes a tap act as that command instead of running Tap.
	Command Command
	// KeepsOverlays leaves a list's hold menu or A–Z jump open: the tap is what opens them.
	// Any other tap on a list closes them first.
	KeepsOverlays bool
}

// Targets are the targets of the screen shown, the first registered winning where they
// overlap.
type Targets []Target

// Add registers target.
func (targets *Targets) Add(target Target) { *targets = append(*targets, target) }

// Tap registers rect as a target whose tap runs onTap.
func (targets *Targets) Tap(rect image.Rectangle, onTap func()) {
	targets.Add(Target{Rect: rect, Tap: onTap})
}

// Clear forgets every target, as an overlay does: under it, the screen no longer reacts.
func (targets *Targets) Clear() { *targets = (*targets)[:0] }

// Hit returns the first target that a touch from start to end started and ended in.
func (targets Targets) Hit(start, end image.Point) (Target, bool) {
	for _, target := range targets {
		if start.In(target.Rect) && end.In(target.Rect) {
			return target, true
		}
	}
	return Target{}, false
}
