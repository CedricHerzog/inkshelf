package pocketbook

import "image"

// firmware is what the screen and the event loop need from the reader: its framebuffer
// driver, its message queues and its shared state. readerFirmware (firmware_linux_arm.go)
// talks to the real one; Emulator is one in memory, for running the app on a computer.
//
// Messages go in and out as raw bytes, in the firmware's own layouts (firmware_messages.go),
// so that the event loop decodes the same bytes on the reader and in the emulator.
type firmware interface {
	// flush copies rect of canvas to the panel and refreshes it with waveform. full asks for a
	// flashing refresh, which clears ghosting.
	flush(canvas *image.Gray, rect image.Rectangle, waveform Waveform, full bool)
	// visible reports whether the app is the foreground task and may refresh the panel.
	visible() bool
	// describe returns the firmware state that visible depends on, for the log.
	describe() string
	// nextHardwareEvent returns the next input message from /hwevent, if any. Call it only
	// when visible: the queue is shared by all apps, and the input is the foreground app's.
	nextHardwareEvent() ([]byte, bool)
	// watchInput asks for a value on inputWake when input arrives on /hwevent while it is
	// empty, without reading it: the event loop then reads it only if the app is in front,
	// since the queue's input is the foreground app's. A value ends the watch; ask again. It
	// reports whether the firmware can watch: when not, the event loop polls the queue often.
	watchInput() bool
	// inputWake receives a value when input may have arrived (see watchInput).
	inputWake() <-chan struct{}
	// nextTaskMessage returns the next message for this task, if any.
	nextTaskMessage() ([]byte, bool)
	// nextReply returns the firmware's reply to the request with this id, if it came.
	nextReply(id int32) ([]byte, bool)
	// send sends a task message (with no data) and reports whether it was sent.
	send(message []byte) bool
	// battery returns the battery's charge in percent and whether it is charging. ok is false
	// if the firmware's shared state could not be opened.
	battery() (percent int, charging, ok bool)
	// darkMode reports whether the reader's settings turn dark mode on, which shows its screens
	// white on black. It reads the settings each time, as they change while the app is in the
	// background. ok is false when they cannot be read or do not say.
	darkMode() (dark, ok bool)
}

// device is the reader the app runs on: the canvas it draws on, and the firmware behind it.
type device struct {
	// canvas is the screen's size, in portrait, as the app sees the screen.
	canvas   *image.Gray
	firmware firmware
	// dark is the reader's dark mode setting as last read (see DarkMode).
	dark bool
}

// current is the device that Run opened. It is a package variable so that the app can draw
// with plain functions (Fill, DrawImage, Refresh...), as it did with libinkview. Those
// functions do nothing before Run opens it.
var current *device
