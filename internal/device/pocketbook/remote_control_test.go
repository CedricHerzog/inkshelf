package pocketbook

import (
	"image"
	"slices"
	"testing"
)

// newRemoteLoop returns an event loop whose firmware is a RemoteControl in front of a fake.
func newRemoteLoop() (*eventLoop, *RemoteControl, *fakeFirmware, *fakeApp) {
	loop, fake, app := newTestLoop()
	remote := &RemoteControl{}
	remote.attach(fake, screenSize)
	loop.firmware = remote
	return loop, remote, fake, app
}

func TestRemoteControlInput(t *testing.T) {
	loop, remote, fake, app := newRemoteLoop()

	// Injected input comes before the reader's, and Pending tells until the loop has read it all.
	fake.events = [][]byte{encodedEvent(eventKeyPress, 0, int32(KeyMenu), 0)}
	remote.PressKey(KeyNext)
	remote.PointerDown(image.Pt(100, 200))
	remote.PointerUp(image.Pt(100, 200))
	if !remote.Pending() {
		t.Error("Pending = false with input queued")
	}
	loop.poll(testStart)
	want := []KeyEvent{{Key: KeyNext, State: KeyStatePress}, {Key: KeyMenu, State: KeyStatePress}}
	if !slices.Equal(app.keys, want) {
		t.Errorf("keys = %+v, want %+v", app.keys, want)
	}
	if len(app.pointers) != 2 || app.pointers[0].Point != image.Pt(100, 200) {
		t.Errorf("pointers = %+v, want down and up at (100,200)", app.pointers)
	}
	if remote.Pending() {
		t.Error("Pending = true once the loop read everything")
	}

	// In the background, input is dropped rather than left for a loop that does not read it.
	fake.isVisible = false
	loop.poll(testStart)
	remote.PressKey(KeyBack)
	if remote.Pending() {
		t.Error("input queued in the background")
	}
}

func TestRemoteControlPanel(t *testing.T) {
	_, remote, fake, _ := newRemoteLoop()
	canvas := image.NewGray(image.Rectangle{Max: screenSize})
	for index := range canvas.Pix {
		canvas.Pix[index] = 0x40
	}
	rect := image.Rect(10, 20, 30, 40)

	remote.flush(canvas, rect, GC16, false)
	if !slices.Equal(fake.flushes, []image.Rectangle{rect}) {
		t.Errorf("reader flushes = %v, want %v", fake.flushes, rect)
	}
	panel := remote.Panel()
	if got := panel.GrayAt(15, 25).Y; got != quantize(0x40, panelGrays) {
		t.Errorf("refreshed pixel = %#x, want %#x", got, quantize(0x40, panelGrays))
	}
	if got := panel.GrayAt(5, 5).Y; got != white.Y {
		t.Errorf("pixel outside the refresh = %#x, want white", got)
	}
	if remote.LastRefresh().IsZero() {
		t.Error("LastRefresh is zero after a refresh")
	}

	// The reader skips refreshes in the background: the copy keeps the app's last screen.
	fake.isVisible = false
	for index := range canvas.Pix {
		canvas.Pix[index] = 0
	}
	remote.flush(canvas, rect, GC16, false)
	if got := remote.Panel().GrayAt(15, 25).Y; got != quantize(0x40, panelGrays) {
		t.Errorf("pixel after a refresh in the background = %#x, want it unchanged", got)
	}
}
