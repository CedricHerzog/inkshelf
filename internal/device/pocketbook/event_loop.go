package pocketbook

import (
	"image"
	"log"
	"time"
)

// pollInterval is how often the event loop reads the firmware's queues. libinkview polls as
// often: neither queue can wake the app when a message arrives.
const pollInterval = 20 * time.Millisecond

// requestTimeout is how long the event loop waits for the firmware's reply to a request
// before it forgets the request.
const requestTimeout = 30 * time.Second

// eventLoop turns the firmware's messages into calls to the app. Run calls poll every
// pollInterval, on the goroutine that calls the app.
type eventLoop struct {
	app        App
	firmware   firmware
	screenSize image.Point
	// visible is true while the app is in the foreground and may refresh the panel.
	visible bool

	// held is true while a button is down.
	held bool
	// heldKey is the button that is down.
	heldKey Key
	// heldSince is when heldKey went down.
	heldSince time.Time
	// lastRepeat is when heldKey last repeated, or went down.
	lastRepeat time.Time

	// touch follows the first finger on the screen (touch.go).
	touch touchState

	// pid is this process's id, part of every request id.
	pid int
	// serial numbers the requests: it is the serial of the last request sent.
	serial int
	// requests are the requests sent and still waiting for the firmware's reply.
	requests []pendingRequest
}

// dropStaleEvents empties /hwevent of the input that was meant for the app in front before
// this one started.
func (loop *eventLoop) dropStaleEvents() {
	if !loop.visible {
		return
	}
	for {
		if _, ok := loop.firmware.nextHardwareEvent(); !ok {
			return
		}
	}
}

// poll handles the waiting task messages and input, repeats a held button and turns a touch
// held still into a long press. It reports whether the firmware asked the app to exit.
func (loop *eventLoop) poll(now time.Time) (exit bool) {
	if len(loop.requests) > 0 {
		loop.readReplies(now)
	}
	if loop.readTaskMessages() {
		return true
	}
	if !loop.followForeground() {
		return false
	}
	loop.readInput(now)
	loop.repeatHeldKey(now)
	loop.sendLongPress(now)
	return false
}

// readTaskMessages handles every message waiting for this task. It reports whether one of them
// asked the app to exit.
func (loop *eventLoop) readTaskMessages() (exit bool) {
	for {
		message, ok := loop.firmware.nextTaskMessage()
		if !ok {
			return false
		}
		if loop.handleTaskMessage(message) {
			return true
		}
	}
}

// handleTaskMessage replies to a sync event and passes the system events on to the app. It
// reports whether the message asked the app to exit.
func (loop *eventLoop) handleTaskMessage(message []byte) (exit bool) {
	parsed, ok := parseTaskMessage(message)
	if !ok {
		return false
	}
	if parsed.kind == taskSyncEvent {
		// The firmware waits for this reply before it goes on.
		loop.firmware.send(taskReply(parsed.id))
	}
	if parsed.kind != taskAsyncEvent && parsed.kind != taskSyncEvent {
		log.Printf("task message %#x ignored", parsed.kind)
		return false
	}
	if event, ok := systemEvents[parsed.event]; ok {
		log.Printf("task event %d: %v", parsed.event, event)
		loop.app.System(event)
		return false
	}
	switch parsed.event {
	case eventExit:
		log.Printf("the firmware asked to exit")
		return true
	case eventForeground, eventBackground:
		// followForeground watches the firmware's foreground task instead, which also covers
		// refreshes being suppressed.
	default:
		log.Printf("task event %d (%d, %d) ignored", parsed.event, parsed.eventCode, parsed.eventValue)
	}
	return false
}

// followForeground notices when the app moves to the background or comes back, and reports
// whether it is in the foreground now.
func (loop *eventLoop) followForeground() (visible bool) {
	visible = loop.firmware.visible()
	if visible != loop.visible {
		log.Printf("visible %v (%s)", visible, loop.firmware.describe())
	}
	if !visible {
		// Buttons and touches released in the background are never seen: forget them.
		loop.visible = false
		loop.held = false
		loop.touch.active = false
		return false
	}
	if !loop.visible {
		// Back in front: the panel still shows what the previous app drew.
		loop.visible = true
		loop.app.Draw()
	}
	return true
}

// readInput handles every waiting /hwevent message. Call it only in the foreground: the queue
// is shared by all apps.
func (loop *eventLoop) readInput(now time.Time) {
	for {
		message, ok := loop.firmware.nextHardwareEvent()
		if !ok {
			return
		}
		loop.handleHardwareEvent(decodeHardwareEvent(message), now)
	}
}

// handleHardwareEvent passes a button or a touch of the first finger on to the app.
func (loop *eventLoop) handleHardwareEvent(event hardwareEvent, now time.Time) {
	switch event.kind {
	case eventKeyPress:
		loop.held = true
		loop.heldKey = Key(event.keyOrRawX)
		loop.heldSince = now
		loop.lastRepeat = now
		loop.app.Key(KeyEvent{Key: Key(event.keyOrRawX), State: KeyStatePress})
	case eventKeyRelease:
		if loop.held && loop.heldKey == Key(event.keyOrRawX) {
			loop.held = false
		}
		loop.app.Key(KeyEvent{Key: Key(event.keyOrRawX), State: KeyStateRelease})
	case eventPointerDown, eventPointerMove, eventPointerUp:
		if event.touchID != 0 {
			// A second finger: the app only follows the first.
			return
		}
		loop.handleTouch(event.kind, touchPoint(event.keyOrRawX, event.rawY, loop.screenSize), now)
	}
	// The firmware's own key repeats (eventKeyRepeat) and multitouch syncs are ignored, as
	// libinkview ignores them.
}

// pendingRequest is a request to the firmware whose reply hasn't been read yet.
type pendingRequest struct {
	id   int32
	kind int32
	sent time.Time
}

// request sends a request to the firmware, such as taskBanSuspend. poll reads its reply later:
// the event loop never waits for the firmware.
func (loop *eventLoop) request(kind, argument int32, now time.Time) { //nolint:unparam // kind: the firmware takes other requests than taskBanSuspend
	loop.serial++
	id := requestID(loop.pid, loop.serial)
	if !loop.firmware.send(taskRequest(id, kind, argument)) {
		return
	}
	log.Printf("request %#x (%d) sent as %#x", kind, argument, id)
	loop.requests = append(loop.requests, pendingRequest{id: id, kind: kind, sent: now})
}

// readReplies reads the replies to requests. Nothing else would read them, and they would stay
// in the firmware's queue forever.
func (loop *eventLoop) readReplies(now time.Time) {
	waiting := loop.requests[:0]
	for _, pending := range loop.requests {
		if reply, ok := loop.firmware.nextReply(pending.id); ok {
			// A reply too short to parse is logged with zeros: it is only read to empty the queue.
			parsed, _ := parseTaskMessage(reply)
			log.Printf("request %#x (%#x): reply %#x, arg %d", pending.kind, pending.id, parsed.kind, parsed.argument)
			continue
		}
		if now.Sub(pending.sent) > requestTimeout {
			log.Printf("request %#x (%#x): no reply", pending.kind, pending.id)
			continue
		}
		waiting = append(waiting, pending)
	}
	loop.requests = waiting
}
