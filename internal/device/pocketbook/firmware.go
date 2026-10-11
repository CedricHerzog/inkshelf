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
	// nextTaskMessage returns the next message for this task, if any.
	nextTaskMessage() ([]byte, bool)
	// nextReply returns the firmware's reply to the request with this id, if it came.
	nextReply(id int32) ([]byte, bool)
	// send sends a task message (with no data) and reports whether it was sent.
	send(message []byte) bool
	// battery returns the battery's charge in percent and whether it is charging. ok is false
	// if the firmware's shared state could not be opened.
	battery() (percent int, charging, ok bool)
}

// device is the reader the app runs on: the canvas it draws on, and the firmware behind it.
type device struct {
	// canvas is the screen's size, in portrait, as the app sees the screen.
	canvas   *image.Gray
	firmware firmware
}

// current is the device that Run opened. It is a package variable so that the app can draw
// with plain functions (Fill, DrawImage, Refresh...), as it did with libinkview. Those
// functions do nothing before Run opens it.
var current *device
