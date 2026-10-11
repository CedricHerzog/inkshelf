package pocketbook

import "time"

// Key is a button, as numbered by the firmware (IV_KEY_* in inkview.h). An InkPad 3 has Back,
// Forward (the page buttons, KeyPrev and KeyNext), Menu and Home; the other codes are for other
// models.
type Key int

// The buttons.
const (
	KeyPower  Key = 0x01
	KeyDelete Key = 0x08
	KeyOK     Key = 0x0a
	KeyUp     Key = 0x11
	KeyDown   Key = 0x12
	KeyLeft   Key = 0x13
	KeyRight  Key = 0x14
	KeyMinus  Key = 0x15
	KeyPlus   Key = 0x16
	KeyMenu   Key = 0x17
	KeyPrev   Key = 0x18 // the "<" page button
	KeyNext   Key = 0x19 // the ">" page button
	KeyHome   Key = 0x1a // handled by the firmware, never delivered to the app
	KeyBack   Key = 0x1b
	KeyPrev2  Key = 0x1c
	KeyNext2  Key = 0x1d
	KeyMusic  Key = 0x1e
)

// KeyState tells a press from its repeats and its release.
type KeyState int

// The states of a button.
const (
	KeyStatePress KeyState = iota
	KeyStateRepeat
	KeyStateRelease
)

// KeyEvent is a button press, repeat or release.
type KeyEvent struct {
	Key   Key
	State KeyState
}

// A held button repeats after repeatDelay, then every repeatInterval. The firmware sends
// repeats of its own (eventKeyRepeat), but libinkview ignores them and times repeats itself,
// with these delays; so does the event loop.
const (
	repeatDelay    = 600 * time.Millisecond
	repeatInterval = 300 * time.Millisecond
)

// repeatHeldKey sends the app a repeat of the button held down, if it is time to. The event
// loop calls it at each poll.
func (loop *eventLoop) repeatHeldKey(now time.Time) {
	if loop.held && now.Sub(loop.heldSince) >= repeatDelay && now.Sub(loop.lastRepeat) >= repeatInterval {
		loop.lastRepeat = now
		loop.app.Key(KeyEvent{Key: loop.heldKey, State: KeyStateRepeat})
	}
}
