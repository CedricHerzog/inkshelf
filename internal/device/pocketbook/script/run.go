package script

import (
	"fmt"
	"image"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// Target is the reader a script drives: an emulated one, or a real one under remote control.
// Its methods are safe from any goroutine.
type Target interface {
	// Size returns the screen's size.
	Size() image.Point
	// PointerDown, PointerMove and PointerUp touch the screen at a point, with the first finger.
	PointerDown(point image.Point)
	PointerMove(point image.Point)
	PointerUp(point image.Point)
	// PressKey and ReleaseKey press and release a button.
	PressKey(key pocketbook.Key)
	ReleaseKey(key pocketbook.Key)
	// Panel returns a copy of what the panel shows.
	Panel() *image.Gray
	// LastRefresh returns when the panel last changed.
	LastRefresh() time.Time
	// Pending reports whether the event loop has input or a repaint it has not handled yet.
	Pending() bool
	// RequestExit asks the app to exit, as the firmware does.
	RequestExit()
}

// Emulated is a Target whose reader is emulated, so that a script can change what only the
// firmware or the user changes on a real reader.
type Emulated interface {
	Target
	// SetForeground sends the app to the background or brings it back.
	SetForeground(foreground bool)
	// SetBattery sets the battery's charge and whether it is charging.
	SetBattery(percent int, charging bool)
	// SetNetwork turns the network on or off, with the firmware's events.
	SetNetwork(up bool)
	// SetDarkMode switches the dark mode setting.
	SetDarkMode(dark bool)
}

// BackgroundWork returns the app's background work in progress and when some last ended: the
// app's BackgroundWork.
type BackgroundWork func() (names []string, lastEnded time.Time)

// SaveShot saves the panel of a shot step under its name.
type SaveShot func(name string, panel *image.Gray) error

// Run runs steps on target, handing each shot to save. work tells when the app is idle. Steps
// that need an emulated reader fail before anything runs if target is not Emulated.
func Run(target Target, work BackgroundWork, steps []Step, save SaveShot) error {
	emulated, isEmulated := target.(Emulated)
	if !isEmulated {
		for _, current := range steps {
			if emulatedCommands[current.command] {
				return fmt.Errorf("line %d: %s works only in the emulator", current.line, current.command)
			}
		}
	}
	// Input sent before the event loop starts is dropped, as stale.
	if err := waitIdle(target, work, defaultTimeout); err != nil {
		return fmt.Errorf("starting: %w", err)
	}
	screen := image.Rectangle{Max: target.Size()}
	for _, current := range steps {
		log.Printf("script: line %d: %s", current.line, current.command)
		for _, point := range current.points {
			if !point.In(screen) {
				return fmt.Errorf("line %d: %v is off the screen %v", current.line, point, screen.Max)
			}
		}
		if inputCommands[current.command] {
			// Input given while the app still works on the previous one would land on what
			// happens to show then.
			if err := waitSettled(target, work, defaultTimeout); err != nil {
				return fmt.Errorf("line %d: %w", current.line, err)
			}
		}
		if err := runStep(target, emulated, work, current, save); err != nil {
			return fmt.Errorf("line %d: %w", current.line, err)
		}
		// The app exits after an exit step: the steps after it would go nowhere.
		if current.command == "exit" {
			return nil
		}
	}
	return nil
}

// emulatedCommands are the commands that need an Emulated target.
var emulatedCommands = map[string]bool{"home": true, "resume": true, "battery": true, "net": true, "dark": true}

// inputCommands are the commands that touch the screen or press a button.
var inputCommands = map[string]bool{"tap": true, "long": true, "swipe": true, "key": true, "hold": true}

// runStep runs one step on target. emulated is target when it is Emulated, else nil; Run
// checks the steps that need it.
func runStep(target Target, emulated Emulated, work BackgroundWork, current Step, save SaveShot) error {
	switch current.command {
	case "tap":
		touch(target, current.points[0], tapTime)
	case "long":
		touch(target, current.points[0], longTime)
	case "swipe":
		swipe(target, current.points[0], current.points[1])
	case "key":
		press(target, current.key, tapTime)
	case "hold":
		press(target, current.key, current.duration)
	case "home", "resume":
		emulated.SetForeground(current.command == "resume")
	case "battery":
		emulated.SetBattery(current.number, current.flag)
	case "net":
		emulated.SetNetwork(current.flag)
	case "dark":
		emulated.SetDarkMode(current.flag)
	case "wait":
		time.Sleep(current.duration)
	case "idle":
		return waitIdle(target, work, current.duration)
	case "shot":
		if err := waitIdle(target, work, defaultTimeout); err != nil {
			return err
		}
		return save(current.name, target.Panel())
	case "exit":
		target.RequestExit()
	}
	return nil
}

// touch puts a finger on point and lifts it after duration.
func touch(target Target, point image.Point, duration time.Duration) {
	target.PointerDown(point)
	time.Sleep(duration)
	target.PointerUp(point)
}

// swipe puts a finger on from and slides it to to in swipeMoves even moves, over swipeTime.
func swipe(target Target, from, to image.Point) {
	target.PointerDown(from)
	for move := range swipeMoves {
		time.Sleep(swipeTime / swipeMoves)
		target.PointerMove(from.Add(to.Sub(from).Mul(move + 1).Div(swipeMoves)))
	}
	target.PointerUp(to)
}

// press presses a button and releases it after duration.
func press(target Target, key pocketbook.Key, duration time.Duration) {
	target.PressKey(key)
	time.Sleep(duration)
	target.ReleaseKey(key)
}

// waitIdle waits until the app is idle: settled (see waitSettled), with the panel unchanged
// and no work ended for quietTime, counting from now, as the app may not have started on the
// last step yet. The quiet time lets the repaint of the work that ended last reach the panel.
func waitIdle(target Target, work BackgroundWork, timeout time.Duration) error {
	start := time.Now()
	for {
		busy, lastEnded := busyWith(target, work)
		if len(busy) == 0 && time.Since(latest(start, target.LastRefresh(), lastEnded)) >= quietTime {
			return nil
		}
		if time.Since(start) > timeout {
			if len(busy) == 0 {
				busy = []string{"the panel still changes"}
			}
			return fmt.Errorf("not idle after %v: %s", timeout, strings.Join(busy, ", "))
		}
		time.Sleep(pollTime)
	}
}

// waitSettled waits until the app has handled its input and finished its background work.
func waitSettled(target Target, work BackgroundWork, timeout time.Duration) error {
	start := time.Now()
	for {
		busy, _ := busyWith(target, work)
		if len(busy) == 0 {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("still busy after %v: %s", timeout, strings.Join(busy, ", "))
		}
		time.Sleep(pollTime)
	}
}

// busyWith returns what the app is busy with: its background work, and the input and repaints
// the event loop has not handled. lastEnded is when some background work last ended.
func busyWith(target Target, work BackgroundWork) (busy []string, lastEnded time.Time) {
	busy, lastEnded = work()
	if target.Pending() {
		busy = append(busy, "input or a repaint not handled yet")
	}
	return busy, lastEnded
}

// latest returns the latest of times.
func latest(times ...time.Time) time.Time {
	return slices.MaxFunc(times, func(first, second time.Time) int { return first.Compare(second) })
}
