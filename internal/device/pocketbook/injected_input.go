package pocketbook

import (
	"sync"
	"time"
)

// injectedInput is input that a driver (the emulator's web page, a script) gives the event
// loop instead of the touch panel and the buttons, queued as /hwevent messages. It knows
// whether the event loop may still be handling some of it, which is how a script waits for the
// app to settle. It has no lock: its owner guards it with its own mutex.
type injectedInput struct {
	// events are the /hwevent messages not read yet.
	events [][]byte
	// handling is true from when the event loop takes a message of events until it asks for
	// the next one and there is none: until then, it may still be handling the message.
	handling bool
	// arrived receives a value when input is added, for a wait (see waitFor). Make it with
	// newInjectedInput.
	arrived chan struct{}
}

// newInjectedInput returns an empty injectedInput.
func newInjectedInput() injectedInput {
	return injectedInput{arrived: make(chan struct{}, 1)}
}

// add queues event, and wakes a wait.
func (input *injectedInput) add(event hardwareEvent) {
	input.events = append(input.events, encodeHardwareEvent(event))
	select {
	case input.arrived <- struct{}{}:
	default: // a wake-up is already due
	}
}

// waitFor returns the oldest message not read yet, waiting up to timeout for one. lock guards
// input; it is not held while waiting.
func (input *injectedInput) waitFor(lock sync.Locker, timeout time.Duration) ([]byte, bool) {
	lock.Lock()
	message, ok := input.next()
	lock.Unlock()
	if ok || timeout <= 0 {
		return message, ok
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-input.arrived:
	case <-timer.C:
	}
	lock.Lock()
	defer lock.Unlock()
	return input.next()
}

// next returns the oldest message not read yet, if any.
func (input *injectedInput) next() ([]byte, bool) {
	message, ok := pop(&input.events)
	input.handling = ok
	return message, ok
}

// drop forgets the messages not read yet, as the firmware does with input meant for an app
// that went to the background.
func (input *injectedInput) drop() { input.events = nil }

// pending reports whether some input is not read yet or may still be being handled.
func (input *injectedInput) pending() bool { return len(input.events) > 0 || input.handling }
