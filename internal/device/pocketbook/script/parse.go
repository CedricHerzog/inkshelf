package script

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

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

// KeyNames are the buttons scripts and the web page can press.
var KeyNames = map[string]pocketbook.Key{
	"prev": pocketbook.KeyPrev, "next": pocketbook.KeyNext, // the InkPad 3's page buttons
	"left": pocketbook.KeyLeft, "right": pocketbook.KeyRight, "up": pocketbook.KeyUp, "down": pocketbook.KeyDown,
	"menu": pocketbook.KeyMenu, "ok": pocketbook.KeyOK, "back": pocketbook.KeyBack,
	"plus": pocketbook.KeyPlus, "minus": pocketbook.KeyMinus,
}

// Step is a parsed script line.
type Step struct {
	line     int
	command  string
	points   []image.Point
	key      pocketbook.Key
	duration time.Duration
	number   int
	flag     bool   // battery: charging; net: up; dark: on
	name     string // shot
}

// Parse parses a script, reporting the first bad line.
func Parse(source string) ([]Step, error) {
	var steps []Step
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
var stepParsers = map[string]func(parsed *Step, arguments []string) error{
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
	"dark":    parseDark,
	"wait":    parseWait,
	"idle":    parseIdle,
	"shot":    parseShot,
}

// parseStep parses a script line split into fields, the command first.
func parseStep(fields []string) (Step, error) {
	parsed := Step{command: fields[0]}
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
func parsePoints(count int) func(parsed *Step, arguments []string) error {
	return func(parsed *Step, arguments []string) error {
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
func parseKey(parsed *Step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	return parseKeyName(parsed, arguments[0])
}

// parseHold parses "hold NAME DURATION".
func parseHold(parsed *Step, arguments []string) error {
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

// parseKeyName sets the step's key from its name in KeyNames.
func parseKeyName(parsed *Step, name string) error {
	key, ok := KeyNames[name]
	if !ok {
		return fmt.Errorf("unknown key %q", name)
	}
	parsed.key = key
	return nil
}

// parseNothing parses a command without arguments.
func parseNothing(parsed *Step, arguments []string) error {
	return checkArgumentCount(arguments, 0, 0)
}

// parseBattery parses "battery PERCENT [charging]".
func parseBattery(parsed *Step, arguments []string) error {
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
func parseNet(parsed *Step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	if arguments[0] != "up" && arguments[0] != "down" {
		return fmt.Errorf("want up or down, got %q", arguments[0])
	}
	parsed.flag = arguments[0] == "up"
	return nil
}

// parseDark parses "dark on" and "dark off".
func parseDark(parsed *Step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	if arguments[0] != "on" && arguments[0] != "off" {
		return fmt.Errorf("want on or off, got %q", arguments[0])
	}
	parsed.flag = arguments[0] == "on"
	return nil
}

// parseWait parses "wait DURATION".
func parseWait(parsed *Step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	var err error
	parsed.duration, err = time.ParseDuration(arguments[0])
	return err
}

// parseIdle parses "idle [TIMEOUT]".
func parseIdle(parsed *Step, arguments []string) error {
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
func parseShot(parsed *Step, arguments []string) error {
	if err := checkArgumentCount(arguments, 1, 1); err != nil {
		return err
	}
	parsed.name = arguments[0]
	if strings.ContainsAny(parsed.name, `/\`) {
		return fmt.Errorf("shot names cannot hold a path")
	}
	return nil
}
