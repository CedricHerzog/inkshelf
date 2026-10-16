package pocketbook

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// Emulator is a reader simulated in memory, for running an app off the reader with
// RunEmulated: drivers (a web page, a script) feed it input and look at its panel. It
// implements the firmware interface as the reader's firmware behaves, down to the bytes of its
// messages, so the event loop runs unchanged. Its methods are safe from any goroutine.
type Emulator struct {
	// canvas is drawn by the app, on Run's goroutine.
	canvas *image.Gray
	// simulateRefresh makes refreshes take as long as on an InkPad 3.
	simulateRefresh bool

	// mu guards the fields below.
	mu sync.Mutex
	// panel is what the e-ink shows: the canvas as of the last refreshes.
	panel *image.Gray
	// isVisible is true while the app is in the foreground.
	isVisible bool
	// hardwareEvents are the /hwevent messages not read yet.
	hardwareEvents [][]byte
	// taskMessages are the task messages not read yet.
	taskMessages [][]byte
	// handlingInput and handlingTasks are true from when the event loop takes a message of
	// hardwareEvents or taskMessages until it asks for the next one and there is none: until
	// then, it may still be handling the message.
	handlingInput, handlingTasks bool
	// replies are the replies to requests not read yet, by request id.
	replies map[int32][]byte
	// messageID is the id of the last task message sent.
	messageID      int32
	batteryPercent int
	charging       bool
	// dark is the emulated dark mode setting (see SetDarkMode).
	dark bool
	// lastRefresh is when the panel last changed.
	lastRefresh time.Time
	// subscribers receive every refresh (see Subscribe).
	subscribers map[chan PanelRefresh]struct{}
}

// PanelRefresh is a refresh of the emulated panel.
type PanelRefresh struct {
	Rect     image.Rectangle
	Waveform Waveform
	Full     bool
	At       time.Time
}

// emulatorPGID is the process group that the emulated firmware sends task events to.
const emulatorPGID = 1

// emulatorBatteryPercent is the battery's charge until SetBattery changes it.
const emulatorBatteryPercent = 80

// subscriptionBuffer is how many refreshes a subscriber may fall behind by before it misses
// some (see Subscribe).
const subscriptionBuffer = 64

// panelGrays is the number of grays the InkPad 3's panel shows with a 16-gray waveform.
// pageimage dithers pages to the same 16 grays, under the same name.
const panelGrays = 16

// How long refreshes take on an InkPad 3, as measured. With simulateRefresh, the emulator
// waits as long.
const (
	fullRefreshTime    = 600 * time.Millisecond // GC16, full
	partialRefreshTime = 490 * time.Millisecond // GL16 or GC16
	fastRefreshTime    = 265 * time.Millisecond // DU
)

// NewEmulator returns an emulated reader with a screen of size, in the foreground, its panel
// white. If simulateRefresh is true, refreshes take as long as on an InkPad 3.
func NewEmulator(size image.Point, simulateRefresh bool) *Emulator {
	emulator := &Emulator{
		canvas:          image.NewGray(image.Rectangle{Max: size}),
		simulateRefresh: simulateRefresh,
		panel:           image.NewGray(image.Rectangle{Max: size}),
		isVisible:       true,
		replies:         map[int32][]byte{},
		batteryPercent:  emulatorBatteryPercent,
		subscribers:     map[chan PanelRefresh]struct{}{},
	}
	for index := range emulator.panel.Pix {
		emulator.panel.Pix[index] = white.Y
	}
	return emulator
}

// Size returns the screen's size.
func (emulator *Emulator) Size() image.Point { return emulator.canvas.Rect.Size() }

// flush copies rect of canvas to the panel, rounding each pixel to the grays waveform can
// show, so that a waveform of 2 or 4 grays looks in the emulator as it does on the reader.
func (emulator *Emulator) flush(canvas *image.Gray, rect image.Rectangle, waveform Waveform, full bool) {
	rect = rect.Intersect(canvas.Rect)
	if rect.Empty() {
		return
	}
	defer timing.Start(context.Background(), "refresh").End()
	levels := waveformLevels(waveform)
	emulator.mu.Lock()
	for offset := range rect.Dy() {
		row := rect.Min.Y + offset
		source := canvas.Pix[canvas.PixOffset(rect.Min.X, row):][:rect.Dx()]
		destination := emulator.panel.Pix[emulator.panel.PixOffset(rect.Min.X, row):][:rect.Dx()]
		for column, gray := range source {
			destination[column] = quantize(gray, levels)
		}
	}
	refresh := PanelRefresh{Rect: rect, Waveform: waveform, Full: full, At: time.Now()}
	emulator.lastRefresh = refresh.At
	for subscriber := range emulator.subscribers {
		select {
		case subscriber <- refresh:
		default:
			// A slow subscriber misses refreshes rather than blocking the app.
		}
	}
	emulator.mu.Unlock()

	if emulator.simulateRefresh {
		time.Sleep(refreshTime(waveform, full))
		emulator.mu.Lock()
		// The panel kept changing until now.
		emulator.lastRefresh = time.Now()
		emulator.mu.Unlock()
	}
}

// waveformLevels returns the number of grays waveform shows.
func waveformLevels(waveform Waveform) int {
	switch waveform {
	case DU, A2:
		return 2
	case GC4, DU4:
		return 4
	default:
		return panelGrays
	}
}

// quantize rounds gray to the nearest of levels evenly spaced grays.
func quantize(gray uint8, levels int) uint8 {
	steps := levels - 1
	return uint8((int(gray)*steps + 127) / 255 * 255 / steps)
}

// refreshTime returns how long a refresh takes on an InkPad 3. The emulator waits for every
// refresh to end, while the reader only waits when a refresh overlaps the previous one: the
// emulator shows the worst case.
func refreshTime(waveform Waveform, full bool) time.Duration {
	switch {
	case full:
		return fullRefreshTime
	case waveform == DU || waveform == A2 || waveform == DU4:
		return fastRefreshTime
	default:
		return partialRefreshTime
	}
}

// visible reports whether the app is in the foreground.
func (emulator *Emulator) visible() bool {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	return emulator.isVisible
}

// describe returns the emulator's state, for the log.
func (emulator *Emulator) describe() string {
	return fmt.Sprintf("emulator, visible %v", emulator.visible())
}

// nextHardwareEvent returns the oldest input not read yet, if any.
func (emulator *Emulator) nextHardwareEvent() ([]byte, bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	message, ok := pop(&emulator.hardwareEvents)
	emulator.handlingInput = ok
	return message, ok
}

// nextTaskMessage returns the oldest task message not read yet, if any.
func (emulator *Emulator) nextTaskMessage() ([]byte, bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	message, ok := pop(&emulator.taskMessages)
	emulator.handlingTasks = ok
	return message, ok
}

// pop removes the first message of queue and returns it, if there is one.
func pop(queue *[][]byte) ([]byte, bool) {
	if len(*queue) == 0 {
		return nil, false
	}
	message := (*queue)[0]
	*queue = (*queue)[1:]
	return message, true
}

// nextReply returns the reply to the request with this id, if it came.
func (emulator *Emulator) nextReply(id int32) ([]byte, bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	reply, ok := emulator.replies[id]
	delete(emulator.replies, id)
	return reply, ok
}

// send answers requests to the firmware at once. Replies to sync events go nowhere, since the
// emulator never waits for them.
func (emulator *Emulator) send(message []byte) bool {
	parsed, ok := parseTaskMessage(message)
	if !ok || int32At(message, mtypeOffset) != firmwareMtype {
		return true
	}
	log.Printf("emulator: request %#x (%d)", parsed.kind, parsed.argument)
	emulator.mu.Lock()
	emulator.replies[parsed.id] = taskReply(parsed.id)
	emulator.mu.Unlock()
	return true
}

// battery returns the battery state set with SetBattery.
func (emulator *Emulator) battery() (percent int, charging, ok bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	return emulator.batteryPercent, emulator.charging, true
}

// Panel returns a copy of what the panel shows.
func (emulator *Emulator) Panel() *image.Gray {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	panel := image.NewGray(emulator.panel.Rect)
	copy(panel.Pix, emulator.panel.Pix)
	return panel
}

// LastRefresh returns when the panel was last refreshed, or finished refreshing with
// simulated refresh times (zero if never).
func (emulator *Emulator) LastRefresh() time.Time {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	return emulator.lastRefresh
}

// Pending reports whether the event loop has input or task messages it has not handled yet,
// or a repaint it has not started yet: the app has something left to do that may change the
// panel. A repaint being drawn is not pending: the panel changes once it is drawn.
func (emulator *Emulator) Pending() bool {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	return len(emulator.hardwareEvents) > 0 || len(emulator.taskMessages) > 0 ||
		emulator.handlingInput || emulator.handlingTasks || len(repaints) > 0
}

// Subscribe returns a channel that receives the panel's refreshes, and a function that ends
// the subscription. Refreshes are dropped while the channel is full.
func (emulator *Emulator) Subscribe() (refreshes <-chan PanelRefresh, cancel func()) {
	channel := make(chan PanelRefresh, subscriptionBuffer)
	emulator.mu.Lock()
	emulator.subscribers[channel] = struct{}{}
	emulator.mu.Unlock()
	return channel, func() {
		emulator.mu.Lock()
		delete(emulator.subscribers, channel)
		emulator.mu.Unlock()
	}
}

// PointerDown, PointerMove and PointerUp touch the screen at point, with the first finger.
// Like the reader's touch panel, the screen ignores them while the app is in the background.

// PointerDown puts the first finger down at point.
func (emulator *Emulator) PointerDown(point image.Point) { emulator.touch(eventPointerDown, point) }

// PointerMove moves the first finger to point.
func (emulator *Emulator) PointerMove(point image.Point) { emulator.touch(eventPointerMove, point) }

// PointerUp lifts the first finger at point.
func (emulator *Emulator) PointerUp(point image.Point) { emulator.touch(eventPointerUp, point) }

// touch queues a touch of the first finger, in the touch panel's raw coordinates.
func (emulator *Emulator) touch(kind int32, point image.Point) {
	rawX, rawY := rawTouch(point, emulator.Size())
	emulator.input(hardwareEvent{kind: kind, keyOrRawX: rawX, rawY: rawY})
}

// PressKey presses a button.
func (emulator *Emulator) PressKey(key Key) {
	emulator.input(hardwareEvent{kind: eventKeyPress, keyOrRawX: int32(key)})
}

// ReleaseKey releases a button.
func (emulator *Emulator) ReleaseKey(key Key) {
	emulator.input(hardwareEvent{kind: eventKeyRelease, keyOrRawX: int32(key)})
}

// input queues an /hwevent message, unless the app is in the background: input goes to the
// foreground app.
func (emulator *Emulator) input(event hardwareEvent) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	if emulator.isVisible {
		emulator.hardwareEvents = append(emulator.hardwareEvents, encodeHardwareEvent(event))
	}
}

// SetForeground moves the app to the background (as pressing Home does) or back to the
// foreground (as the task switcher does).
func (emulator *Emulator) SetForeground(foreground bool) {
	event := int32(eventBackground)
	if foreground {
		event = eventForeground
	}
	emulator.mu.Lock()
	emulator.isVisible = foreground
	emulator.hardwareEvents = nil
	emulator.mu.Unlock()
	emulator.sendTaskEvent(event)
}

// SendSystem sends the app a firmware event that App.System receives.
func (emulator *Emulator) SendSystem(event SystemEvent) {
	for number, systemEvent := range systemEvents {
		if systemEvent == event {
			emulator.sendTaskEvent(number)
			return
		}
	}
}

// RequestExit asks the app to exit, as the firmware does.
func (emulator *Emulator) RequestExit() { emulator.sendTaskEvent(eventExit) }

// sendTaskEvent queues a task event for the app. Its code is the process group, which is what
// the firmware sends with the foreground event.
func (emulator *Emulator) sendTaskEvent(event int32) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	emulator.messageID++
	message := taskEvent(emulatorPGID, emulator.messageID, event, emulatorPGID, 0)
	emulator.taskMessages = append(emulator.taskMessages, message)
}

// darkMode returns the dark mode set with SetDarkMode.
func (emulator *Emulator) darkMode() (dark, ok bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	return emulator.dark, true
}

// SetDarkMode turns the emulated reader's dark mode setting on or off. As on the reader, the app
// reads it when it comes back to the foreground.
func (emulator *Emulator) SetDarkMode(dark bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	emulator.dark = dark
}

// SetBattery sets the battery's charge in percent, and whether it is charging.
func (emulator *Emulator) SetBattery(percent int, charging bool) {
	emulator.mu.Lock()
	defer emulator.mu.Unlock()
	emulator.batteryPercent = percent
	emulator.charging = charging
}
