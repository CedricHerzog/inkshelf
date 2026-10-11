package pocketbook

import (
	"image"
	"time"
)

// PointerState is the phase of a touch.
type PointerState int

// The phases of a touch. The firmware only sends down, move and up; the event loop makes the
// long presses, holds and drags out of them, as libinkview did.
const (
	PointerDown PointerState = iota
	PointerMove
	PointerUp
	// PointerLong is sent once per touch, when it has been held still for longPressDelay.
	PointerLong
	// PointerHold is sent every holdInterval after PointerLong, while the touch is still held.
	PointerHold
	// PointerDrag is a move once the touch has left its dead zone. It is sent after the
	// PointerMove.
	PointerDrag
)

// PointerEvent is a touch, in screen coordinates.
type PointerEvent struct {
	image.Point
	State PointerState
}

// touchPoint converts a touch's raw coordinates, as the touch panel reports them, to the
// screen's. The touch panel is mounted turned, like the display: its x runs up the portrait
// screen and its y to the right. This was measured on an InkPad 3, which has no tsc.dat
// calibration file for the firmware to apply. A raw value out of range is clamped to the
// screen's edge.
func touchPoint(rawX, rawY int32, screenSize image.Point) image.Point {
	x, y := int(rawY), screenSize.Y-1-int(rawX)
	return image.Pt(min(max(x, 0), screenSize.X-1), min(max(y, 0), screenSize.Y-1))
}

// rawTouch is the inverse of touchPoint: it returns the raw coordinates of a screen point, for
// the emulator to send touches as the reader does.
func rawTouch(point, screenSize image.Point) (rawX, rawY int32) {
	return int32(screenSize.Y - 1 - point.Y), int32(point.X)
}

// A touch held still becomes a long press after longPressDelay, then a hold every
// holdInterval. It counts as still while it stays within dragDeadZone of where it went down,
// as a finger always trembles a little; past it, its moves are drags. dragDeadZone is
// libinkview's TOUCHDRAGDEADZONE: a squared radius, in pixels.
const (
	longPressDelay = 500 * time.Millisecond
	holdInterval   = 500 * time.Millisecond
	dragDeadZone   = 200
)

// touchState follows the first finger on the screen. The app ignores the others.
type touchState struct {
	// active is true while the finger is down.
	active bool
	// start is where the finger went down.
	start image.Point
	// last is where the finger is now.
	last image.Point
	// since is when the finger went down.
	since time.Time
	// dragging is true once the finger has left its dead zone: it gets no long press then.
	dragging bool
	// longSent is true once the long press was sent.
	longSent bool
	// lastHold is when the long press or the last hold was sent.
	lastHold time.Time
}

// handleTouch passes on a touch of the first finger, and tells drags from still touches.
func (loop *eventLoop) handleTouch(kind int32, point image.Point, now time.Time) {
	touch := &loop.touch
	switch kind {
	case eventPointerDown:
		*touch = touchState{active: true, start: point, last: point, since: now}
		loop.app.Pointer(PointerEvent{Point: point, State: PointerDown})
	case eventPointerMove:
		touch.last = point
		loop.app.Pointer(PointerEvent{Point: point, State: PointerMove})
		if !touch.active {
			return
		}
		if !touch.dragging && squaredDistance(point, touch.start) > dragDeadZone {
			touch.dragging = true
		}
		if touch.dragging {
			loop.app.Pointer(PointerEvent{Point: point, State: PointerDrag})
		}
	case eventPointerUp:
		touch.active = false
		loop.app.Pointer(PointerEvent{Point: point, State: PointerUp})
	}
}

// sendLongPress sends a long press once a touch has been held still for longPressDelay, then
// a hold every holdInterval until the finger lifts or moves away. No input arrives while a
// finger is held still, so the event loop calls it at each poll.
func (loop *eventLoop) sendLongPress(now time.Time) {
	touch := &loop.touch
	if !touch.active || touch.dragging || now.Sub(touch.since) < longPressDelay {
		return
	}
	switch {
	case !touch.longSent:
		touch.longSent = true
		touch.lastHold = now
		loop.app.Pointer(PointerEvent{Point: touch.last, State: PointerLong})
	case now.Sub(touch.lastHold) >= holdInterval:
		touch.lastHold = now
		loop.app.Pointer(PointerEvent{Point: touch.last, State: PointerHold})
	}
}

// squaredDistance returns the square of the distance between two points, in pixels.
func squaredDistance(first, second image.Point) int {
	delta := first.Sub(second)
	return delta.X*delta.X + delta.Y*delta.Y
}
