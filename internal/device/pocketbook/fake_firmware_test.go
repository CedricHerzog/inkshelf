package pocketbook

import (
	"image"
	"time"
)

// screenSize is the InkPad 3's screen.
var screenSize = image.Pt(1404, 1872)

// testPGID is the process group that the tests' task messages are for.
const testPGID = 1

// testStart is the time the tests' event loops start at.
var testStart = time.Unix(1000, 0)

// fakeFirmware records what the event loop and the screen do.
type fakeFirmware struct {
	isVisible bool
	// events are the /hwevent messages not read yet.
	events [][]byte
	// givenBack are the messages given back to /hwevent.
	givenBack [][]byte
	// messages are the task messages not read yet.
	messages [][]byte
	// sent are the task messages sent.
	sent [][]byte
	// replies are the replies to requests not read yet, by request id.
	replies map[int32][]byte
	// flushes are the rectangles refreshed.
	flushes []image.Rectangle
	// global is the firmware's shared state.
	global []byte
	// dark is the dark mode setting, and darkUnknown makes it unreadable.
	dark, darkUnknown bool
}

func (fake *fakeFirmware) flush(canvas *image.Gray, rect image.Rectangle, waveform Waveform, full bool) {
	fake.flushes = append(fake.flushes, rect)
}

func (fake *fakeFirmware) darkMode() (bool, bool) { return fake.dark, !fake.darkUnknown }

func (fake *fakeFirmware) visible() bool    { return fake.isVisible }
func (fake *fakeFirmware) describe() string { return "fake" }

func (fake *fakeFirmware) waitHardwareEvent(time.Duration) ([]byte, bool) { return pop(&fake.events) }
func (fake *fakeFirmware) giveBack(message []byte)                        { fake.givenBack = append(fake.givenBack, message) }
func (fake *fakeFirmware) nextTaskMessage() ([]byte, bool)                { return pop(&fake.messages) }

func (fake *fakeFirmware) nextReply(id int32) ([]byte, bool) {
	reply, ok := fake.replies[id]
	delete(fake.replies, id)
	return reply, ok
}

func (fake *fakeFirmware) send(message []byte) bool {
	fake.sent = append(fake.sent, message)
	return true
}

func (fake *fakeFirmware) battery() (int, bool, bool) {
	percent, charging := batteryState(fake.global)
	return percent, charging, true
}

// fakeApp records the calls the event loop makes.
type fakeApp struct {
	draws    int
	keys     []KeyEvent
	pointers []PointerEvent
	system   []SystemEvent
}

func (app *fakeApp) Init() error { return nil }
func (app *fakeApp) Draw()       { app.draws++ }

func (app *fakeApp) Key(event KeyEvent) bool {
	app.keys = append(app.keys, event)
	return true
}

func (app *fakeApp) Pointer(event PointerEvent) bool {
	app.pointers = append(app.pointers, event)
	return true
}

func (app *fakeApp) System(event SystemEvent) { app.system = append(app.system, event) }
func (app *fakeApp) Close() error             { return nil }

// pointerStates returns the states of the touches app received.
func (app *fakeApp) pointerStates() []PointerState {
	var states []PointerState
	for _, event := range app.pointers {
		states = append(states, event.State)
	}
	return states
}

// keyStates returns the states of the buttons app received.
func (app *fakeApp) keyStates() []KeyState {
	var states []KeyState
	for _, event := range app.keys {
		states = append(states, event.State)
	}
	return states
}

// newTestLoop returns an event loop in the foreground, on a fake reader with an InkPad 3's
// screen.
func newTestLoop() (*eventLoop, *fakeFirmware, *fakeApp) {
	fake := &fakeFirmware{isVisible: true, replies: map[int32][]byte{}}
	app := &fakeApp{}
	loop := &eventLoop{app: app, firmware: fake, screenSize: screenSize, visible: true, pid: 100}
	return loop, fake, app
}

// int32s encodes values as consecutive little-endian int32s.
func int32s(values ...int32) []byte {
	buffer := make([]byte, 4*len(values))
	putInt32s(buffer, values...)
	return buffer
}

// encodedEvent returns an /hwevent message, as the firmware sends it.
func encodedEvent(kind, touchID, keyOrRawX, rawY int32) []byte {
	return int32s(kind, touchID, keyOrRawX, rawY, fullPressure, fingerTool, 0, 0)
}

// encodedTaskEvent returns a task event for testPGID, with this message id.
func encodedTaskEvent(kind, id, event int32) []byte {
	return int32s(taskEventMtype|testPGID, id, kind, 0, eventDataSize, event, 0, 0, 0)
}
