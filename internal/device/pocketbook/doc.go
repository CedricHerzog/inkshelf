// Package pocketbook runs an app on a PocketBook e-reader (firmware 6.x, made for the InkPad 3)
// without PocketBook's SDK. Its C library, libinkview, normally draws the screen, reads the
// buttons and talks to the firmware; this package does all of that itself, in pure Go, so the
// app builds without cgo. What libinkview does was learned by disassembling it and by running
// probe programs on the reader.
//
// # The framebuffer and the canvas
//
// A framebuffer is a block of memory whose bytes are the screen's pixels: here /dev/fb0, one
// byte per pixel (0 is black, 255 white), mapped into the app's memory. Writing to it changes
// nothing on an e-ink panel by itself: the panel only shows new pixels once it is told to
// refresh them (see below).
//
// The app does not draw into the framebuffer directly. It draws into a canvas, an image the
// size of the screen held in memory that the firmware shares with each app (so that its task
// switcher can show the app). Refresh then copies the changed rectangle from the canvas into
// the framebuffer and asks the panel to show it.
//
// The panel is mounted turned a quarter: the framebuffer is landscape (1872x1404 on an
// InkPad 3) while the app works in portrait (1404x1872). The copy therefore rotates every
// pixel: screen pixel (x, y) is framebuffer column y, row 1403 - x. Framebuffer memory is
// slow to write, so the copy moves pixels four at a time in whole 32-bit words
// (framebuffer_copy.go). It runs on every page turn.
//
// # E-ink refreshes and waveforms
//
// An e-ink pixel is a cell of black and white particles that an electric field moves up or
// down. Changing a pixel means driving it with a sequence of voltages over several frames, a
// waveform. Waveforms trade speed for quality:
//
//   - GC16 shows 16 grays and drives every pixel of the rectangle, even unchanged ones. It is
//     the cleanest, and as a "full" refresh it flashes the area black then white.
//   - GL16 also shows 16 grays but only drives pixels that change, and is tuned for text on
//     white: it flickers less.
//   - DU is fast (about half the time) but only black and white; A2 is faster still.
//
// Partial refreshes, which only drive the pixels that change, leave faint traces of what was
// there before: ghosting. A full GC16 refresh now and then (FlashRefresh) clears it. The app
// asks for a refresh with the MXCFB ioctls of the panel's driver, which PocketBook modelled on
// the i.MX6's (eink_refresh.go). Measured on an InkPad 3: a full GC16 refresh takes about
// 600 ms, a partial GL16 one 490 ms, DU 265 ms.
//
// # Talking to the firmware
//
// The firmware (the "pocketbook" process, which runs as root) launches every app as a task
// and knows it by its process group. It talks to apps through three System V or POSIX
// inter-process communication objects (firmware_messages.go, firmware_shared_memory.go):
//
//   - Shared memory holding its global state: which task is in the foreground, whether
//     refreshes are suppressed, the battery's charge.
//   - A POSIX message queue, /hwevent, carrying input: button presses and touches. It is
//     shared by all apps, so only the foreground app may read it.
//   - A System V message queue of task messages: events for a task (go to the background,
//     come back, exit, the network came up...) and requests to the firmware (such as "don't
//     sleep for 30 s"), whose replies come back on the same queue.
//
// The firmware type hides all of this behind a few methods. readerFirmware implements it on
// the reader (the files ending in _linux_arm.go, which build only for the reader); Emulator
// implements it in memory, so that the app runs and is tested on a computer.
//
// # The event loop
//
// Run calls the app from one goroutine only, its event loop (event_loop.go). Every 20 ms, as
// libinkview does, it reads the task messages and the input queue, notices when the app goes
// to the background or comes back, and turns raw input into the app's events: libinkview made
// key repeats, long presses and drags itself, and so does this package (keys.go, touch.go).
// Other goroutines never call the app: they ask the event loop to, with Repaint, Exit and
// PreventSleepFor.
package pocketbook
