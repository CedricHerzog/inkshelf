package pocketbook

import (
	"image"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// App is what Run runs. Its methods are called on Run's goroutine, one at a time, so they
// never run concurrently with each other.
type App interface {
	// Init is called once the screen is open. It should draw the first screen.
	Init() error
	// Draw redraws the screen, after Repaint or when the app comes back to the foreground.
	Draw()
	// Key handles a button press, repeat or release. Its result is not used.
	Key(KeyEvent) bool
	// Pointer handles a touch. Its result is not used.
	Pointer(PointerEvent) bool
	// System handles news from the firmware, such as the network going up.
	System(SystemEvent)
	// Close is called when the app exits. Run returns its error.
	Close() error
}

// These channels carry requests from any goroutine to Run's goroutine, which alone may call
// the app. Each holds one request: a second one, sent before Run reads the first, is dropped,
// since it would do the same thing.
var (
	repaints      = make(chan struct{}, 1)
	exits         = make(chan struct{}, 1)
	sleepRequests = make(chan time.Duration, 1)
)

// Repaint asks Run to call Draw. It is safe to call from any goroutine.
func Repaint() {
	select {
	case repaints <- struct{}{}:
	default:
	}
}

// Exit asks Run to close the app and return. It is safe to call from any goroutine.
func Exit() {
	select {
	case exits <- struct{}{}:
	default:
	}
}

// PreventSleepFor asks the firmware not to suspend the reader for duration, as libinkview's
// BanSleep does. It is for work that must go on without input, such as a download: the reader
// would otherwise fall asleep after a few minutes without a touch. Call it again before
// duration is over to keep the reader awake. It is safe to call from any goroutine.
func PreventSleepFor(duration time.Duration) {
	select {
	case sleepRequests <- duration:
	default:
	}
}

// Run opens the screen and runs app until it exits or the firmware closes it. It returns the
// error of app.Init or app.Close.
func Run(app App) error {
	opened, canvas, err := openFirmware()
	if err != nil {
		return err
	}
	return run(app, opened, canvas)
}

// RunEmulated runs app as Run does, but on emulator instead of a reader.
func RunEmulated(app App, emulator *Emulator) error {
	return run(app, emulator, emulator.canvas)
}

// run is what Run and RunEmulated share: it calls app for every request from other
// goroutines, every signal and, through the event loop, every firmware message, until the app
// exits.
func run(app App, readerOrEmulator firmware, canvas *image.Gray) error {
	current = &device{canvas: canvas, firmware: readerOrEmulator}
	loop := &eventLoop{
		app:        app,
		firmware:   readerOrEmulator,
		screenSize: canvas.Rect.Size(),
		visible:    readerOrEmulator.visible(),
		pid:        os.Getpid(),
	}
	loop.dropStaleEvents()

	// The firmware stops apps with SIGINT (when the task switcher closes an app sent to the
	// background by Home) or with SIGTERM. Either way the app gets to save its state.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	if err := app.Init(); err != nil {
		return err
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-repaints:
			app.Draw()
		case <-exits:
			log.Printf("exit: closing")
			return app.Close()
		case duration := <-sleepRequests:
			seconds := int32(duration.Round(time.Second) / time.Second)
			loop.request(taskBanSuspend, seconds, time.Now())
		case received := <-signals:
			log.Printf("%v: closing", received)
			return app.Close()
		case now := <-ticker.C:
			if loop.poll(now) {
				return app.Close()
			}
		}
	}
}
