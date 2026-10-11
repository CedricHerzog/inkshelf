// Package coverview is the screen that shows the cover of a series or a book on the whole
// screen, from a tap on its small cover in a list. The covers service loads it; until it is
// there, the screen says so. Any tap or button goes back to the list.
package coverview

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// Screen is the cover view.
type Screen struct {
	app *state.State
}

// New returns the cover view of app.
func New(app *state.State) *Screen { return &Screen{app: app} }

// DrawLocked draws the cover, or what the screen waits for.
func (screen *Screen) DrawLocked() {
	app := screen.app
	screenRect := pocketbook.Screen()
	if cover := app.CoverView.Image; cover != nil {
		pocketbook.DrawImage(cover, screenRect.Size().Sub(cover.Bounds().Size()).Div(2))
		return
	}
	lines := []string{"Loading the cover…"}
	if app.CoverView.Err != nil {
		lines = widgets.WrapWords(app.Fonts.Body, "Cannot show the cover: "+widgets.Clean(app.CoverView.Err.Error()), screenRect.Dx()-2*widgets.Px(widgets.WideMargin))
	}
	lines = append(lines, "", "Tap to go back")
	top := screenRect.Dy() / 3
	for _, line := range lines {
		widgets.Text(app.Fonts.Body, widgets.DarkGray, image.Pt((screenRect.Dx()-app.Fonts.Body.Width(line))/2, top), line)
		top += app.Fonts.Body.LineHeight()
	}
}

// Command goes back to the list, whatever the button: on the InkPad, the Back button is also
// the previous page key.
func (screen *Screen) Command(widgets.Command) { screen.close() }

// TouchLocked makes any tap go back to the list.
func (screen *Screen) TouchLocked(start, end image.Point, swipe bool) widgets.Target {
	return widgets.Target{Command: widgets.Back}
}

// close goes back from the cover to the list it was shown from.
func (screen *Screen) close() {
	app := screen.app
	app.Update(true, func() bool {
		app.CoverView.Image = nil
		app.CoverView.Err = nil
		app.ShowLocked(app.CoverView.From)
		return true
	})
}
