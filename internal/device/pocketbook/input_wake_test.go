package pocketbook

import (
	"testing"
	"time"
)

func TestNextPoll(t *testing.T) {
	loop, _, _ := newTestLoop()
	if got := loop.nextPoll(); got != idlePollInterval {
		t.Errorf("idle: nextPoll = %v, want %v", got, idlePollInterval)
	}
	loop.touch.active = true
	if got := loop.nextPoll(); got != pollInterval {
		t.Errorf("finger down: nextPoll = %v, want %v", got, pollInterval)
	}
	loop.touch.active, loop.held = false, true
	if got := loop.nextPoll(); got != pollInterval {
		t.Errorf("button down: nextPoll = %v, want %v", got, pollInterval)
	}
	loop.held, loop.pollsInput = false, true
	if got := loop.nextPoll(); got != pollInterval {
		t.Errorf("input can't wake the loop: nextPoll = %v, want %v", got, pollInterval)
	}
}

func TestInputWoke(t *testing.T) {
	loop, fake, app := newTestLoop()

	// In front, the input that woke the loop is handled.
	fake.events = [][]byte{encodedEvent(eventKeyPress, 0, int32(KeyNext), 0)}
	loop.inputWoke(testStart)
	if len(app.keys) != 1 || len(fake.events) != 0 {
		t.Errorf("in front: keys %+v, %d messages left; want the press handled", app.keys, len(fake.events))
	}

	// In the background, it is another app's: left in the queue.
	loop.visible = false
	fake.events = [][]byte{encodedEvent(eventKeyPress, 0, int32(KeyMenu), 0)}
	loop.inputWoke(testStart)
	if len(app.keys) != 1 || len(fake.events) != 1 {
		t.Errorf("in the background: keys %+v, %d messages left; want the queue untouched", app.keys, len(fake.events))
	}
}

func TestPollingWhenWakesFail(t *testing.T) {
	// A firmware that can't watch input: the loop polls at once.
	loop, fake, _ := newTestLoop()
	fake.cannotWatch = true
	loop.watchInput()
	if !loop.pollsInput {
		t.Error("pollsInput = false although input can't be watched")
	}

	// Input that polls keep finding, with no wake-up: the loop stops waiting for wake-ups.
	loop, fake, _ = newTestLoop()
	for turn := range missedWakesLimit {
		if loop.pollsInput {
			t.Fatalf("pollsInput after %d missed wake-ups, want it after %d", turn, missedWakesLimit)
		}
		fake.events = [][]byte{encodedEvent(eventKeyPress, 0, int32(KeyNext), 0)}
		loop.poll(testStart.Add(time.Duration(turn) * time.Second))
	}
	if !loop.pollsInput {
		t.Errorf("pollsInput = false after %d polls found input", missedWakesLimit)
	}
}

func TestEmulatorWakesOnInput(t *testing.T) {
	emulator := NewEmulator(screenSize, false)
	emulator.PressKey(KeyNext)
	select {
	case <-emulator.inputWake():
	case <-time.After(time.Second):
		t.Fatal("no wake-up for a key press")
	}
}
