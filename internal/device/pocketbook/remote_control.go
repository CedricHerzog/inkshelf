package pocketbook

import (
	"image"
	"sync"
	"time"
)

// RemoteControl lets a driver on another goroutine (the app's remote control server) touch the
// screen and press buttons of a real reader, and see what its panel shows, as an Emulator lets
// one do on a computer. RunRemoteControlled puts it between the event loop and the reader's
// firmware: it passes everything through, gives the event loop the injected input before the
// reader's own, and keeps a copy of the panel from what the app refreshes. The copy only knows
// the app's refreshes: while another app is in front, it still shows the app's last screen.
// Its exported methods are safe from any goroutine.
type RemoteControl struct {
	// firmware is the reader's, set by RunRemoteControlled before the event loop starts and
	// only used on its goroutine.
	firmware firmware

	// mu guards the fields below.
	mu sync.Mutex
	// input is the input not read yet. Its arrived channel also gets the reader's wake-ups
	// (see attach), so that inputWake covers both.
	input injectedInput
	// panel is the app's screen as of its last refreshes, nil until RunRemoteControlled opens
	// the reader.
	panel *image.Gray
	// isVisible is whether the app was in the foreground when the event loop last looked.
	isVisible bool
	// lastRefresh is when the app last refreshed the panel.
	lastRefresh time.Time
}

// RunRemoteControlled runs app as Run does, with remote between the event loop and the
// firmware.
func RunRemoteControlled(app App, remote *RemoteControl) error {
	opened, canvas, err := openFirmware()
	if err != nil {
		return err
	}
	remote.attach(opened, canvas.Rect.Size())
	return run(app, remote, canvas)
}

// attach puts remote in front of reader, whose screen has size. The panel starts white, as the
// app's first draw covers it all.
func (remote *RemoteControl) attach(reader firmware, size image.Point) {
	panel := image.NewGray(image.Rectangle{Max: size})
	for index := range panel.Pix {
		panel.Pix[index] = white.Y
	}
	remote.firmware = reader
	remote.mu.Lock()
	remote.input = newInjectedInput()
	go forwardWakes(reader.inputWake(), remote.input.arrived)
	remote.panel = panel
	remote.isVisible = reader.visible()
	remote.mu.Unlock()
}

// Ready reports whether RunRemoteControlled has opened the reader: before, there is no panel
// and input goes nowhere.
func (remote *RemoteControl) Ready() bool {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	return remote.panel != nil
}

// Size returns the screen's size, or zero before the reader is open.
func (remote *RemoteControl) Size() image.Point {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if remote.panel == nil {
		return image.Point{}
	}
	return remote.panel.Rect.Size()
}

// Panel returns a copy of the app's screen as last refreshed, or an empty image before the
// reader is open.
func (remote *RemoteControl) Panel() *image.Gray {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if remote.panel == nil {
		return image.NewGray(image.Rectangle{})
	}
	panel := image.NewGray(remote.panel.Rect)
	copy(panel.Pix, remote.panel.Pix)
	return panel
}

// LastRefresh returns when the app last refreshed the panel (zero if never).
func (remote *RemoteControl) LastRefresh() time.Time {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	return remote.lastRefresh
}

// Pending reports whether the event loop has injected input it has not handled yet, or a
// repaint it has not started yet. The reader's own input and task messages are not counted:
// they are on the firmware's queues, which only the event loop reads.
func (remote *RemoteControl) Pending() bool {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	return remote.input.pending() || len(repaints) > 0
}

// PointerDown puts the first finger down at point.
func (remote *RemoteControl) PointerDown(point image.Point) { remote.touch(eventPointerDown, point) }

// PointerMove moves the first finger to point.
func (remote *RemoteControl) PointerMove(point image.Point) { remote.touch(eventPointerMove, point) }

// PointerUp lifts the first finger at point.
func (remote *RemoteControl) PointerUp(point image.Point) { remote.touch(eventPointerUp, point) }

// touch queues a touch of the first finger, in the touch panel's raw coordinates.
func (remote *RemoteControl) touch(kind int32, point image.Point) {
	rawX, rawY := rawTouch(point, remote.Size())
	remote.inject(hardwareEvent{kind: kind, keyOrRawX: rawX, rawY: rawY})
}

// PressKey presses a button.
func (remote *RemoteControl) PressKey(key Key) {
	remote.inject(hardwareEvent{kind: eventKeyPress, keyOrRawX: int32(key)})
}

// ReleaseKey releases a button.
func (remote *RemoteControl) ReleaseKey(key Key) {
	remote.inject(hardwareEvent{kind: eventKeyRelease, keyOrRawX: int32(key)})
}

// Battery returns the battery's charge and whether it is charging, as Battery does. ok is
// false before the reader is open.
func (remote *RemoteControl) Battery() (percent int, charging, ok bool) {
	// The firmware is set before the panel, under mu: once Ready, it can be read.
	if !remote.Ready() {
		return 0, false, false
	}
	return remote.firmware.battery()
}

// RequestExit asks the app to exit, as Exit does.
func (remote *RemoteControl) RequestExit() { Exit() }

// inject queues an /hwevent message, unless the app is in the background or the reader is not
// open yet: input goes to the foreground app, and a script would otherwise wait for input the
// event loop never reads.
func (remote *RemoteControl) inject(event hardwareEvent) {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if remote.panel != nil && remote.isVisible {
		remote.input.add(event)
	}
}

// flush refreshes the panel, then copies what it shows now. A refresh in the background is
// skipped by the reader, so it is not copied either.
func (remote *RemoteControl) flush(canvas *image.Gray, rect image.Rectangle, waveform Waveform, full bool) {
	visible := remote.firmware.visible()
	remote.firmware.flush(canvas, rect, waveform, full)
	rect = rect.Intersect(canvas.Rect)
	if !visible || rect.Empty() {
		return
	}
	remote.mu.Lock()
	copyToPanel(remote.panel, canvas, rect, waveform)
	remote.lastRefresh = time.Now()
	remote.mu.Unlock()
}

// visible asks the reader, and remembers the answer for inject. Input queued for an app that
// has gone to the background is forgotten, as the firmware does.
func (remote *RemoteControl) visible() bool {
	visible := remote.firmware.visible()
	remote.mu.Lock()
	remote.isVisible = visible
	if !visible {
		remote.input.drop()
	}
	remote.mu.Unlock()
	return visible
}

// nextHardwareEvent returns the injected input first, then the reader's.
func (remote *RemoteControl) nextHardwareEvent() ([]byte, bool) {
	remote.mu.Lock()
	message, ok := remote.input.next()
	remote.mu.Unlock()
	if ok {
		return message, true
	}
	return remote.firmware.nextHardwareEvent()
}

// forwardWakes passes on each value of from to to, dropping it if to already has one.
func forwardWakes(from <-chan struct{}, to chan<- struct{}) {
	for range from {
		select {
		case to <- struct{}{}:
		default:
		}
	}
}

func (remote *RemoteControl) watchInput() bool { return remote.firmware.watchInput() }

// inputWake receives a value when injected input or the reader's arrives.
func (remote *RemoteControl) inputWake() <-chan struct{} { return remote.input.arrived }

func (remote *RemoteControl) describe() string { return remote.firmware.describe() }

func (remote *RemoteControl) nextTaskMessage() ([]byte, bool) {
	return remote.firmware.nextTaskMessage()
}

func (remote *RemoteControl) nextReply(id int32) ([]byte, bool) { return remote.firmware.nextReply(id) }

func (remote *RemoteControl) send(message []byte) bool { return remote.firmware.send(message) }

func (remote *RemoteControl) battery() (percent int, charging, ok bool) {
	return remote.firmware.battery()
}

func (remote *RemoteControl) darkMode() (dark, ok bool) { return remote.firmware.darkMode() }
