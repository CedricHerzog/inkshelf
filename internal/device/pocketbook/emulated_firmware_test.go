package pocketbook

import (
	"image"
	"slices"
	"testing"
	"time"
)

func TestEmulator(t *testing.T) {
	emulator := NewEmulator(screenSize, false)
	app := &fakeApp{}
	loop := &eventLoop{app: app, firmware: emulator, screenSize: screenSize, visible: true, pid: 100}

	// A tap arrives where it was made.
	emulator.PointerDown(image.Pt(300, 400))
	emulator.PointerUp(image.Pt(300, 400))
	emulator.PressKey(KeyNext)
	emulator.ReleaseKey(KeyNext)
	loop.poll(testStart)
	wantPointers := []PointerEvent{
		{Point: image.Pt(300, 400), State: PointerDown},
		{Point: image.Pt(300, 400), State: PointerUp},
	}
	if !slices.Equal(app.pointers, wantPointers) {
		t.Fatalf("pointers = %+v, want down then up at (300, 400)", app.pointers)
	}
	wantKeys := []KeyEvent{{Key: KeyNext, State: KeyStatePress}, {Key: KeyNext, State: KeyStateRelease}}
	if !slices.Equal(app.keys, wantKeys) {
		t.Fatalf("keys = %+v, want a press and a release of KeyNext", app.keys)
	}

	// Requests are answered, and the replies read.
	loop.request(taskBanSuspend, 60, testStart)
	loop.poll(testStart.Add(time.Second))
	if len(loop.requests) != 0 {
		t.Errorf("waiting = %+v, want the reply read", loop.requests)
	}

	// Home, then back: input is ignored meanwhile, and the app redraws on its return.
	emulator.SetForeground(false)
	loop.poll(testStart.Add(2 * time.Second))
	emulator.PressKey(KeyMenu)
	draws := app.draws
	emulator.SetForeground(true)
	loop.poll(testStart.Add(3 * time.Second))
	if app.draws != draws+1 || len(app.keys) != 2 {
		t.Errorf("back in front: %d draws, keys %+v, want one draw and no key", app.draws-draws, app.keys)
	}

	emulator.SendSystem(NetConnected)
	loop.poll(testStart.Add(4 * time.Second))
	if want := []SystemEvent{NetConnected}; !slices.Equal(app.system, want) {
		t.Errorf("system events = %v, want %v", app.system, want)
	}
	emulator.RequestExit()
	if !loop.poll(testStart.Add(5 * time.Second)) {
		t.Error("RequestExit did not end the loop")
	}
}

func TestEmulatorPanel(t *testing.T) {
	emulator := NewEmulator(image.Pt(4, 2), false)
	copy(emulator.canvas.Pix, []byte{0x00, 0x40, 0x90, 0xff, 0x10, 0x20, 0x30, 0x40})
	refreshes, cancel := emulator.Subscribe()
	t.Cleanup(cancel)

	emulator.flush(emulator.canvas, image.Rect(0, 0, 4, 1), DU, false)
	emulator.flush(emulator.canvas, image.Rect(0, 1, 2, 2), GL16, false)
	// DU shows black and white, GL16 16 grays, and the rest is still white.
	want := []byte{0x00, 0x00, 0xff, 0xff, 0x11, 0x22, 0xff, 0xff}
	if got := emulator.Panel().Pix; !slices.Equal(got, want) {
		t.Errorf("panel = %x, want %x", got, want)
	}
	if refresh := <-refreshes; refresh.Rect != image.Rect(0, 0, 4, 1) || refresh.Waveform != DU {
		t.Errorf("first refresh = %+v", refresh)
	}
	if emulator.LastRefresh().IsZero() {
		t.Error("LastRefresh is zero after a refresh")
	}
}

func TestEmulatorDarkMode(t *testing.T) {
	emulator := NewEmulator(image.Pt(4, 1), false)
	current = &device{canvas: emulator.canvas, firmware: emulator}
	t.Cleanup(func() { current = nil })

	emulator.SetDarkMode(true)
	if DarkMode() {
		t.Error("dark mode before the app read it")
	}
	followDarkMode()
	if !DarkMode() {
		t.Error("no dark mode once read")
	}
}

func TestDarkModeKeptWhenUnknown(t *testing.T) {
	fake := &fakeFirmware{isVisible: true, dark: true}
	current = &device{canvas: image.NewGray(image.Rect(0, 0, 4, 1)), firmware: fake}
	t.Cleanup(func() { current = nil })

	followDarkMode()
	fake.dark, fake.darkUnknown = false, true // the settings file being rewritten
	followDarkMode()
	if !DarkMode() {
		t.Error("an unreadable setting turned dark mode off")
	}
}

func TestEmulatorWaitWakesOnInput(t *testing.T) {
	emulator := NewEmulator(screenSize, false)
	if _, ok := emulator.waitHardwareEvent(0); ok {
		t.Fatal("input before any was sent")
	}
	start := time.Now()
	go func() {
		time.Sleep(20 * time.Millisecond)
		emulator.PressKey(KeyNext)
	}()
	message, ok := emulator.waitHardwareEvent(5 * time.Second)
	if !ok || decodeHardwareEvent(message).keyOrRawX != int32(KeyNext) {
		t.Fatalf("waitHardwareEvent = %x, %v; want the press of KeyNext", message, ok)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Errorf("waited %v for input sent after 20 ms", waited)
	}
	if !emulator.Pending() {
		t.Error("Pending = false while the event loop may still handle the press")
	}
}
