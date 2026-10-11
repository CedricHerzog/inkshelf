package pocketbook

// The firmware's inter-process communication objects, opened by the keys libinkview opens
// them with. A System V object (shared memory, message queue) is found by a numeric key that
// every process agrees on; a POSIX queue by its name.
const (
	// globalShmKey is the key of the firmware's shared state ("ivmpc" in libinkview).
	globalShmKey = 0xa12302
	// globalShmSize is the size of the firmware's shared state, in bytes.
	globalShmSize = 0x44d8
	// taskQueueKey is the key of the queue of task messages, to and from every app.
	taskQueueKey = 0xa1230f
	// eventQueue is the POSIX queue of input, which is the foreground app's to read. mq_open
	// takes its name without the leading "/".
	eventQueue = "hwevent"
)

// Where the firmware's shared state holds what the app reads from it. The app only reads it.
const (
	// globalSuppressed is a byte: non-zero while the firmware suppresses refreshes, when the
	// app must leave the panel alone.
	globalSuppressed = 0x02
	// globalForeground is an int32: the process group of the foreground task.
	globalForeground = 0x28
	// globalBattery is an int32: the battery's charge in percent (libinkview's hw_power).
	globalBattery = 0x30
	// globalCharging is an int32 whose bit 0 is set while charging (libinkview's hw_ischarging).
	globalCharging = 0x34
	// globalMultitask is an int32: multitaskMagic when task switching is on.
	globalMultitask = 0x3c
	// multitaskMagic is the value of globalMultitask when task switching is on.
	multitaskMagic = 0x544d
)

// The firmware gives each task a canvas in a shared memory segment of its own: a header, then
// the pixels, one byte each, at canvasHeader plus the int32 at canvasDataOffset. The
// directory taskDirFormat names it (firmware_linux_arm.go).
const (
	canvasHeader     = 0x8c
	canvasWidth      = 0x18 // int32: width in pixels
	canvasHeight     = 0x1c // int32: height in pixels
	canvasDepth      = 0x20 // int32: bits per pixel
	canvasScanline   = 0x30 // int32: bytes per row
	canvasDataOffset = 0x34 // int32: where the pixels start, after the header
)
