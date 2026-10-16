package pocketbook

import (
	"context"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// In dark mode the firmware shows its screens white on black. The app draws black on white as
// always, then inverts what must show dark (Invert) before the refresh, so the canvas holds
// what the panel shows and a screen that stays light costs nothing.

// DarkMode reports whether the reader's dark mode is on, as it was when the app started or last
// came back to the foreground: the setting is changed in the firmware's settings app, which
// sends this one to the background. It is false when the app is not running.
func DarkMode() bool { return current != nil && current.dark }

// followDarkMode reads the dark mode setting, and keeps the last one if it cannot be read. Call
// it before the app redraws the whole screen.
func followDarkMode() {
	if current == nil {
		return
	}
	defer timing.Start(context.Background(), "read dark mode").End()
	if dark, ok := current.firmware.darkMode(); ok {
		current.dark = dark
	}
}
