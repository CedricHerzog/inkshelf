// Package widgets holds what every screen of the app draws with: sizes that scale with the
// screen, the grays, the interface fonts, buttons and panels, text that fits, the logos of the
// servers, and the targets that make parts of the screen react to touches.
//
// Sizes are written in pixels of the InkPad 3, whose screen is 1404 pixels wide, and Px
// scales them to the reader the app runs on, so that a screen looks the same on any panel.
//
// Drawing goes into the canvas of pocketbook, which the panel shows only once a screen asks
// for a refresh: widgets draw, they never refresh.
package widgets

// referenceWidth is the shorter side of the InkPad 3's screen, in pixels. The sizes of the app
// are written for it.
const referenceWidth = 1404

// unit is the shorter side of the screen the app runs on. The app sets it once, when it
// starts, before anything is drawn.
var unit = referenceWidth

// SetScreen makes Px scale sizes to a screen whose shorter side is shorterSide pixels.
func SetScreen(shorterSide int) { unit = shorterSide }

// Px scales a size in InkPad 3 pixels to the screen.
func Px(pixels int) int { return pixels * unit / referenceWidth }

// Layout sizes shared by several screens, in InkPad 3 pixels.
const (
	ListMargin   = 48  // the left and right margin of the lists and of the reader's toolbar
	WideMargin   = 96  // the left and right margin of the screens of text and of the overlays
	ButtonHeight = 100 // the height of a header's Back and of a footer's page buttons
	BarHeight    = 168 // the height of a header and of a footer
	// SegmentHeight is the height of a line of segments, the filters and tabs under a header,
	// and SegmentGap the space around it.
	SegmentHeight = 84
	SegmentGap    = 20
)

// SideDivisor divides the screen's width into the sides that turn pages: a tap in the left or
// right screenWidth/SideDivisor of a book's page, or of a list's footer, turns it.
const SideDivisor = 3
