package pocketbook

import (
	"slices"
	"testing"
	"time"
)

func TestEventLoop(t *testing.T) {
	loop, fake, app := newTestLoop()

	fake.events = [][]byte{
		encodedEvent(eventPointerDown, 0, 1760, 97),
		encodedEvent(eventPointerDown, 1, 500, 500), // a second finger: ignored
		encodedEvent(eventPointerUp, 0, 1761, 98),
		encodedEvent(eventKeyPress, 0, int32(KeyNext), 0),
	}
	if loop.poll(testStart) {
		t.Fatal("poll asked to exit")
	}
	if want := []PointerState{PointerDown, PointerUp}; !slices.Equal(app.pointerStates(), want) {
		t.Fatalf("pointers = %+v, want down then up", app.pointers)
	}
	if want := []KeyEvent{{Key: KeyNext, State: KeyStatePress}}; !slices.Equal(app.keys, want) {
		t.Fatalf("keys = %+v, want a press of KeyNext", app.keys)
	}

	// Held, the button repeats after repeatDelay, then every repeatInterval, until released.
	loop.poll(testStart.Add(repeatDelay - time.Millisecond))
	loop.poll(testStart.Add(repeatDelay))
	loop.poll(testStart.Add(repeatDelay + repeatInterval/2))
	loop.poll(testStart.Add(repeatDelay + repeatInterval))
	fake.events = [][]byte{encodedEvent(eventKeyRelease, 0, int32(KeyNext), 0)}
	loop.poll(testStart.Add(2 * repeatDelay))
	loop.poll(testStart.Add(3 * repeatDelay))
	want := []KeyState{KeyStatePress, KeyStateRepeat, KeyStateRepeat, KeyStateRelease}
	if got := app.keyStates(); !slices.Equal(got, want) {
		t.Errorf("key states = %v, want %v", got, want)
	}

	// In the background, no input is read (the queue is another app's), and coming back redraws.
	fake.isVisible = false
	fake.events = [][]byte{encodedEvent(eventKeyPress, 0, int32(KeyMenu), 0)}
	loop.poll(testStart.Add(4 * repeatDelay))
	if len(fake.events) != 1 {
		t.Error("read /hwevent in the background")
	}
	draws := app.draws
	fake.isVisible = true
	fake.events = nil
	loop.poll(testStart.Add(5 * repeatDelay))
	if app.draws != draws+1 {
		t.Errorf("coming back to the foreground drew %d times, want once", app.draws-draws)
	}

	// Sync events get a reply, and the exit event ends the loop.
	const unknownEvent = 999
	fake.messages = [][]byte{
		encodedTaskEvent(taskSyncEvent, 0x5005, unknownEvent),
		encodedTaskEvent(taskAsyncEvent, 0x5006, eventExit),
	}
	if !loop.poll(testStart.Add(6 * repeatDelay)) {
		t.Error("the exit event did not end the loop")
	}
	if len(fake.sent) != 1 || int32At(fake.sent[0], mtypeOffset) != 0x5005 {
		t.Errorf("sent = %x, want one reply to 0x5005", fake.sent)
	}
}

func TestSystemEvents(t *testing.T) {
	loop, fake, app := newTestLoop()
	fake.messages = [][]byte{
		encodedTaskEvent(taskAsyncEvent, 1, eventSaveState),
		encodedTaskEvent(taskSyncEvent, 2, eventUSBStoreIn),
		encodedTaskEvent(taskAsyncEvent, 3, eventFSChanged),
		encodedTaskEvent(taskAsyncEvent, 4, eventNetConnected),
		encodedTaskEvent(taskAsyncEvent, 5, 999), // unknown: ignored
	}
	if loop.poll(testStart) {
		t.Fatal("poll asked to exit")
	}
	want := []SystemEvent{SaveState, USBStorageIn, StorageChanged, NetConnected}
	if !slices.Equal(app.system, want) {
		t.Errorf("system events = %v, want %v", app.system, want)
	}
	if len(fake.sent) != 1 || int32At(fake.sent[0], mtypeOffset) != 2 {
		t.Errorf("sent = %x, want one reply, to the sync event", fake.sent)
	}
}

func TestRequests(t *testing.T) {
	loop, fake, _ := newTestLoop()
	loop.request(taskBanSuspend, 60, testStart)
	loop.request(taskBanSuspend, 60, testStart)
	if len(fake.sent) != 2 || len(loop.requests) != 2 {
		t.Fatalf("sent %d requests, %d waiting, want 2 and 2", len(fake.sent), len(loop.requests))
	}
	first, second := loop.requests[0].id, loop.requests[1].id
	if first == second {
		t.Fatalf("both requests have id %#x", first)
	}

	// The reply to the first is read; the second is given up after requestTimeout.
	fake.replies[first] = int32s(first, first, 0, 1, 0)
	loop.poll(testStart.Add(time.Second))
	if len(loop.requests) != 1 || loop.requests[0].id != second || len(fake.replies) != 0 {
		t.Fatalf("waiting = %+v, want only %#x", loop.requests, second)
	}
	loop.poll(testStart.Add(requestTimeout + time.Second))
	if len(loop.requests) != 0 {
		t.Errorf("waiting = %+v after the timeout, want none", loop.requests)
	}
}
