package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// A script drives the emulator, one command per line ("#" starts a comment), coordinates in
// screen pixels. It starts once the app is idle (see idle). A touch or a button waits until
// the app has handled the input before and finished the work it started (app.App's
// BackgroundWork: a download, loading a list, preloading pages), then only as long as a finger
// or a button would. After a step that changes the screen, use idle or shot, which also wait
// for the panel to stop changing.
//
//	tap X Y                 touch and lift
//	long X Y                touch, hold past the long press delay, lift
//	swipe X1 Y1 X2 Y2       touch, move, lift
//	key NAME                press and release a button (see keyNames)
//	hold NAME DURATION      press a button, release it after DURATION
//	home, resume            send the app to the background, bring it back
//	battery PERCENT [charging]
//	net up|down             turn the emulated network on or off, with the firmware's events
//	wait DURATION           sleep
//	idle [TIMEOUT]          wait until the app is idle: its input handled, its work over and
//	                        the panel unchanged for quietTime (default timeout 2m)
//	shot NAME               idle, then save the panel to OUT/NAME.png
//	exit                    ask the app to exit, as the firmware does; the script ends there

// Script timings.
const (
	tapTime        = 50 * time.Millisecond
	longTime       = 800 * time.Millisecond // past pocketbook's long press delay and polling
	swipeTime      = 200 * time.Millisecond
	swipeMoves     = 10 // a swipe is this many moves
	quietTime      = 500 * time.Millisecond
	pollTime       = 10 * time.Millisecond // how often idle and input steps check whether the app is busy
	defaultTimeout = 2 * time.Minute
)

// keyNames are the buttons scripts and the web page can press.
var keyNames = map[string]pocketbook.Key{
	"prev": pocketbook.KeyPrev, "next": pocketbook.KeyNext, // the InkPad 3's page buttons
	"left": pocketbook.KeyLeft, "right": pocketbook.KeyRight, "up": pocketbook.KeyUp, "down": pocketbook.KeyDown,
	"menu": pocketbook.KeyMenu, "ok": pocketbook.KeyOK, "back": pocketbook.KeyBack,
	"plus": pocketbook.KeyPlus, "minus": pocketbook.KeyMinus,
}

// step is a parsed script line.
type step struct {
	line     int
	command  string
	points   []image.Point
	key      pocketbook.Key
	duration time.Duration
	number   int
	flag     bool   // battery: charging; net: up
	name     string // shot
}

// parseScript parses a script, reporting the first bad line.
func parseScript(source string) ([]step, error) {
	var steps []step
	for index, text := range strings.Split(source, "\n") {
		if comment := strings.IndexByte(text, '#'); comment >= 0 {
			text = text[:comment]
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		parsed, err := parseStep(fields)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", index+1, strings.TrimSpace(text), err)
		}
		parsed.line = index + 1
		steps = append(steps, parsed)
	}
	return steps, nil
}

// stepParsers parse the arguments of each command into a step.
var stepParsers = map[string]func(parsed *step, arguments []string) error{
	"tap":     parsePoints(1),
	"long":    parsePoints(1),
	"swipe":   parsePoints(2),
	"key":     parseKey,
	"hold":    parseHold,
	"home":    parseNothing,
	"resume":  parseNothing,
	"exit":    parseNothing,
	"battery": parseBattery,
	"net":     parseNet,
	"wait":    parseWait,
	"idle":    parseIdle,
	"shot":    parseShot,
}

// parseStep parses a script line split into fields, the command first.
func parseStep(fields []string) (step, error) {
	parsed := step{command: fields[0]}
	parse, ok := stepParsers[parsed.command]
	if !ok {
		return parsed, fmt.Errorf("unknown command %q", parsed.command)
	}
	err := parse(&parsed, fields[1:])
	return parsed, err
}

// checkArgumentCount reports an error unless there are between least and most arguments.
func checkArgumentCount(arguments []string, least, most int) error {
	if len(arguments) < least || len(arguments) > most {
		return fmt.Errorf("want %d to %d arguments, got %d", least, most, len(arguments))
	}
	return nil
}

// parsePoints returns a parser of count points, each written X Y.
func parsePoints(count int) func(parsed *step, arguments []string) error {
	return func(parsed *step, arguments []string) error {
		if err := checkArgumentCount(arguments, 2*count, 2*count); err != nil {
			return err
		}
		for pair := range slices.Chunk(arguments, 2) {
			point, err := parsePoint(pair[0], pair[1])
			if err != nil {
				return err
			}
			parsed.points = append(parsed.points, point)
		}
		return nil
	}
}

// parsePoint parses a point written as its two coordinates.
func parsePoint(xText, yText string) (image.Point, error) {
	x, xErr := strconv.Atoi(xText)
	y, yErr := strconv.Atoi(yText)
	if xErr != nil || yErr != nil {
		return image.Point{}, fmt.Errorf("bad point %s %s", xText, yText)
	}
	return image.Pt(x, y), nil
}

// parseKey parses "key NAME".
func parseKey(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	return parseKeyName(parsed, arguments[0])
}

// parseHold parses "hold NAME DURATION".
func parseHold(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 2, 2); err != nil {
		return err
	}
	if err := parseKeyName(parsed, arguments[0]); err != nil {
		return err
	}
	var err error
	parsed.duration, err = time.ParseDuration(arguments[1])
	return err
}

// parseKeyName sets the step's key from its name in keyNames.
func parseKeyName(parsed *step, name string) error {
	key, ok := keyNames[name]
	if !ok {
		return fmt.Errorf("unknown key %q", name)
	}
	parsed.key = key
	return nil
}

// parseNothing parses a command without arguments.
func parseNothing(parsed *step, arguments []string) error {
	return checkArgumentCount(arguments, 0, 0)
}

// parseBattery parses "battery PERCENT [charging]".
func parseBattery(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 2); err != nil {
		return err
	}
	percent, err := strconv.Atoi(arguments[0])
	if err != nil || percent < 0 || percent > 100 {
		return fmt.Errorf("bad percentage %q", arguments[0])
	}
	parsed.number = percent
	if len(arguments) == 2 {
		if arguments[1] != "charging" {
			return fmt.Errorf("want charging, got %q", arguments[1])
		}
		parsed.flag = true
	}
	return nil
}

// parseNet parses "net up" and "net down".
func parseNet(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	if arguments[0] != "up" && arguments[0] != "down" {
		return fmt.Errorf("want up or down, got %q", arguments[0])
	}
	parsed.flag = arguments[0] == "up"
	return nil
}

// parseWait parses "wait DURATION".
func parseWait(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	var err error
	parsed.duration, err = time.ParseDuration(arguments[0])
	return err
}

// parseIdle parses "idle [TIMEOUT]".
func parseIdle(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 0, 1); err != nil {
		return err
	}
	parsed.duration = defaultTimeout
	if len(arguments) == 1 {
		var err error
		parsed.duration, err = time.ParseDuration(arguments[0])
		return err
	}
	return nil
}

// parseShot parses "shot NAME".
func parseShot(parsed *step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	parsed.name = arguments[0]
	if strings.ContainsAny(parsed.name, `/\`) {
		return fmt.Errorf("shot names cannot hold a path")
	}
	return nil
}

// backgroundWork returns the app's background work in progress and when some last ended: the
// app's BackgroundWork.
type backgroundWork func() (names []string, lastEnded time.Time)

// runScript runs steps on emulator, saving shots into outDir. work tells when the app is idle.
func runScript(emulator *pocketbook.Emulator, work backgroundWork, steps []step, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	// Input sent before the event loop starts is dropped, as stale.
	if err := waitIdle(emulator, work, defaultTimeout); err != nil {
		return fmt.Errorf("starting: %w", err)
	}
	screen := image.Rectangle{Max: emulator.Size()}
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
			if err := waitSettled(emulator, work, defaultTimeout); err != nil {
				return fmt.Errorf("line %d: %w", current.line, err)
			}
		}
		if err := runStep(emulator, work, current, outDir); err != nil {
			return fmt.Errorf("line %d: %w", current.line, err)
		}
		// The app exits after an exit step: the steps after it would go nowhere.
		if current.command == "exit" {
			return nil
		}
	}
	return nil
}

// inputCommands are the commands that touch the screen or press a button.
var inputCommands = map[string]bool{"tap": true, "long": true, "swipe": true, "key": true, "hold": true}

// runStep runs one step on emulator. A shot is saved into outDir.
func runStep(emulator *pocketbook.Emulator, work backgroundWork, current step, outDir string) error {
	switch current.command {
	case "tap":
		touch(emulator, current.points[0], tapTime)
	case "long":
		touch(emulator, current.points[0], longTime)
	case "swipe":
		swipe(emulator, current.points[0], current.points[1])
	case "key":
		press(emulator, current.key, tapTime)
	case "hold":
		press(emulator, current.key, current.duration)
	case "home", "resume":
		emulator.SetForeground(current.command == "resume")
	case "battery":
		emulator.SetBattery(current.number, current.flag)
	case "net":
		setNetwork(emulator, current.flag)
	case "wait":
		time.Sleep(current.duration)
	case "idle":
		return waitIdle(emulator, work, current.duration)
	case "shot":
		if err := waitIdle(emulator, work, defaultTimeout); err != nil {
			return err
		}
		return savePNG(filepath.Join(outDir, current.name+".png"), emulator.Panel())
	case "exit":
		emulator.RequestExit()
	}
	return nil
}

// touch puts a finger on point and lifts it after duration.
func touch(emulator *pocketbook.Emulator, point image.Point, duration time.Duration) {
	emulator.PointerDown(point)
	time.Sleep(duration)
	emulator.PointerUp(point)
}

// swipe puts a finger on from and slides it to to in swipeMoves even moves, over swipeTime.
func swipe(emulator *pocketbook.Emulator, from, to image.Point) {
	emulator.PointerDown(from)
	for move := range swipeMoves {
		time.Sleep(swipeTime / swipeMoves)
		emulator.PointerMove(from.Add(to.Sub(from).Mul(move + 1).Div(swipeMoves)))
	}
	emulator.PointerUp(to)
}

// press presses a button and releases it after duration.
func press(emulator *pocketbook.Emulator, key pocketbook.Key, duration time.Duration) {
	emulator.PressKey(key)
	time.Sleep(duration)
	emulator.ReleaseKey(key)
}

// waitIdle waits until the app is idle: settled (see waitSettled), with the panel unchanged
// and no work ended for quietTime, counting from now, as the app may not have started on the
// last step yet. The quiet time lets the repaint of the work that ended last reach the panel.
func waitIdle(emulator *pocketbook.Emulator, work backgroundWork, timeout time.Duration) error {
	start := time.Now()
	for {
		busy, lastEnded := busyWith(emulator, work)
		if len(busy) == 0 && time.Since(latest(start, emulator.LastRefresh(), lastEnded)) >= quietTime {
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
func waitSettled(emulator *pocketbook.Emulator, work backgroundWork, timeout time.Duration) error {
	start := time.Now()
	for {
		busy, _ := busyWith(emulator, work)
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
func busyWith(emulator *pocketbook.Emulator, work backgroundWork) (busy []string, lastEnded time.Time) {
	busy, lastEnded = work()
	if emulator.Pending() {
		busy = append(busy, "input or a repaint not handled yet")
	}
	return busy, lastEnded
}

// latest returns the latest of times.
func latest(times ...time.Time) time.Time {
	return slices.MaxFunc(times, func(first, second time.Time) int { return first.Compare(second) })
}

// savePNG writes img to path as a PNG file.
func savePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		// The encoding error is the one to report: the file is incomplete anyway.
		_ = file.Close()
		return err
	}
	log.Printf("script: saved %s", path)
	return file.Close()
}
