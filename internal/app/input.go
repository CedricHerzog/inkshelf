package app

import (
	"context"
	"image"
	"log"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// swipeDivisor makes a touch a swipe when it moves at least screenWidth/swipeDivisor sideways,
// and twice as far sideways as up or down.
const swipeDivisor = 5

// keys are the commands of the reader's buttons. The InkPad has a few, and other PocketBooks
// others, so several keys share a command.
var keys = map[pocketbook.Key]widgets.Command{
	pocketbook.KeyNext:  widgets.Next,
	pocketbook.KeyNext2: widgets.Next,
	pocketbook.KeyRight: widgets.Next,
	pocketbook.KeyDown:  widgets.Next,
	pocketbook.KeyPlus:  widgets.Next,
	pocketbook.KeyPrev:  widgets.Previous,
	pocketbook.KeyPrev2: widgets.Previous,
	pocketbook.KeyLeft:  widgets.Previous,
	pocketbook.KeyUp:    widgets.Previous,
	pocketbook.KeyMinus: widgets.Previous,
	pocketbook.KeyBack:  widgets.Back,
	pocketbook.KeyMenu:  widgets.Menu,
	pocketbook.KeyOK:    widgets.Menu,
}

// Key performs the command of a button: on a press, or on a repeat of a page key held down.
func (app *application) Key(event pocketbook.KeyEvent) bool {
	order, ok := keys[event.Key]
	if !ok {
		if event.State == pocketbook.KeyStatePress {
			log.Printf("unhandled key %#x", int(event.Key))
		}
		return false
	}
	isPageKey := order == widgets.Previous || order == widgets.Next
	if event.State != pocketbook.KeyStatePress && !(event.State == pocketbook.KeyStateRepeat && isPageKey) {
		return true
	}
	// Only the reader has a menu, its toolbar: elsewhere, the Menu button goes back.
	if order == widgets.Menu && app.currentScreen() != state.ReaderScreen {
		order = widgets.Back
	}
	defer timing.Start(context.Background(), "key").End()
	app.command(order)
	return true
}

// Pointer performs what a tap, a swipe or a long press does.
func (app *application) Pointer(event pocketbook.PointerEvent) bool {
	switch event.State {
	case pocketbook.PointerDown:
		app.touchStart = event.Point
		app.longPress = false
	case pocketbook.PointerLong:
		defer timing.Start(context.Background(), "long press").End()
		app.longPress = true
		app.Lock()
		held := app.touchTargetLocked(app.touchStart, app.touchStart)
		app.Unlock()
		if held.Hold != nil {
			held.Hold()
		}
	case pocketbook.PointerUp:
		if app.longPress {
			return true // the hold did what the touch does
		}
		defer timing.Start(context.Background(), "tap").End()
		app.Lock()
		tapped := app.touchTargetLocked(app.touchStart, event.Point)
		app.Unlock()
		app.run(tapped)
	}
	return true
}

// System saves the progress when the firmware asks, or when the reader's storage is about to
// be shared over USB, and sends what was read offline once the network is back.
func (app *application) System(event pocketbook.SystemEvent) {
	switch event {
	case pocketbook.SaveState, pocketbook.USBStorageIn:
		app.Lock()
		openbook.SaveProgressLocked(app.State)
		app.Unlock()
		app.Syncer.Schedule(0)
	case pocketbook.NetConnected:
		app.Syncer.Schedule(0)
		app.Lock()
		offline := !app.Server.Online
		app.Unlock()
		if offline { // the app started without the server: try it again
			catalog.Refresh(app.State)
		}
	}
}

// touchTargetLocked returns what a touch from start to end does on the screen shown: what the
// screen decides, if it decides itself; else a swipe turns its pages, if they turn so; else a
// tap does what the target under it does.
func (app *application) touchTargetLocked(start, end image.Point) widgets.Target {
	screenWidth := pocketbook.ScreenSize().X
	delta := end.Sub(start)
	swipe := abs(delta.X) > screenWidth/swipeDivisor && abs(delta.X) > 2*abs(delta.Y)
	screen := app.screens[app.Screen]
	if touched, ok := screen.(touchable); ok {
		return touched.TouchLocked(start, end, swipe)
	}
	if _, ok := screen.(swipeable); ok && swipe {
		if delta.X < 0 {
			return widgets.Target{Command: widgets.Next}
		}
		return widgets.Target{Command: widgets.Previous}
	}
	hit, _ := app.Targets.Hit(start, end)
	return hit
}

// run does what a tap on tapped does. A tap first closes the overlays of the screen shown,
// unless the target is what opens them.
func (app *application) run(tapped widgets.Target) {
	if tapped.Command != widgets.NoCommand {
		app.command(tapped.Command)
		return
	}
	if overlays, ok := app.screens[app.currentScreen()].(overlaid); ok && !tapped.KeepsOverlays {
		app.closeOverlays(overlays)
	}
	if tapped.Tap != nil {
		tapped.Tap()
	}
}

// command passes order to the screen shown. Over a list's overlay, the command only closes it:
// Back closes a menu rather than leaving the list under it.
func (app *application) command(order widgets.Command) {
	screen := app.screens[app.currentScreen()]
	if overlays, ok := screen.(overlaid); ok && app.closeOverlays(overlays) {
		return
	}
	screen.Command(order)
}

// closeOverlays closes the overlays of a screen, and reports whether one was open.
func (app *application) closeOverlays(overlays overlaid) bool {
	closed := false
	app.Update(false, func() bool {
		closed = overlays.CloseOverlaysLocked()
		return closed
	})
	return closed
}

// currentScreen returns the ID of the screen shown.
func (app *application) currentScreen() state.ScreenID {
	app.Lock()
	defer app.Unlock()
	return app.Screen
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
