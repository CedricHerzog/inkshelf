// Package startup holds the screens shown before the series list: the loading screen, which
// says what the session is doing or why it stopped, and the server screen, where the reader
// chooses a server when config.json lists several. On both, Back leaves the app.
package startup

import (
	"fmt"
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// The sizes of the startup screens, in InkPad 3 pixels.
const (
	loadingLogo     = 240 // the logo above the loading screen's title
	serverHeight    = 200 // a server's button
	serverGap       = 40  // between two servers' buttons
	serverLogoInset = 30  // around the logo in a server's button
)

// Loading is the loading screen.
type Loading struct {
	app *state.State
}

// NewLoading returns the loading screen of app.
func NewLoading(app *state.State) *Loading { return &Loading{app: app} }

// DrawLocked draws the logo of the server's kind, its name and the status line, or the error
// that stopped loading and an Exit button.
func (screen *Loading) DrawLocked() {
	app := screen.app
	screenRect := pocketbook.Screen()
	screenWidth, screenHeight := screenRect.Dx(), screenRect.Dy()
	top := screenHeight / 3
	logoBox := image.Rect((screenWidth-widgets.Px(loadingLogo))/2, top-widgets.Px(loadingLogo+60), (screenWidth+widgets.Px(loadingLogo))/2, top-widgets.Px(60))
	app.Logos.Draw(app.Server.Kind, logoBox)
	title := app.KindTitleLocked()
	widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt((screenWidth-app.Fonts.Bold.Width(title))/2, top), title)
	top += widgets.Px(120)
	lines := []string{app.Loading.Status}
	if app.Loading.Err != nil {
		lines = widgets.WrapWords(app.Fonts.Body, "Error: "+widgets.Clean(app.Loading.Err.Error()), screenWidth-2*widgets.Px(widgets.WideMargin))
		lines = append(lines, "", fmt.Sprintf("Check %s/config.json and the Wi-Fi.", app.Dir))
		widgets.ExitButton(app.Fonts, &app.Targets)
	}
	for _, line := range lines {
		widgets.Text(app.Fonts.Body, widgets.Black, image.Pt((screenWidth-app.Fonts.Body.Width(line))/2, top), line)
		top += app.Fonts.Body.LineHeight()
	}
}

// Command leaves the app on Back.
func (screen *Loading) Command(order widgets.Command) { exitOnBack(order) }

// Servers is the server screen.
type Servers struct {
	app *state.State
}

// NewServers returns the server screen of app.
func NewServers(app *state.State) *Servers { return &Servers{app: app} }

// DrawLocked draws a button per server of config.json, the Wi-Fi's status and an Exit button.
func (screen *Servers) DrawLocked() {
	app := screen.app
	screenWidth := pocketbook.Screen().Dx()
	top := widgets.Px(300)
	for _, line := range []struct {
		font  widgets.Font
		label string
	}{{app.Fonts.Bold, widgets.AppName}, {app.Fonts.Body, "Choose a server"}} {
		widgets.Text(line.font, widgets.Black, image.Pt((screenWidth-line.font.Width(line.label))/2, top), line.label)
		top += widgets.Px(120)
	}
	top += widgets.Px(40)
	for index, server := range app.Loading.Servers {
		serverButton := image.Rect(widgets.Px(widgets.WideMargin), top, screenWidth-widgets.Px(widgets.WideMargin), top+widgets.Px(serverHeight))
		screen.drawServerButtonLocked(serverButton, server)
		app.Targets.Tap(serverButton, func() { screen.choose(index) })
		top += widgets.Px(serverHeight + serverGap)
	}
	top += widgets.Px(40)
	widgets.Text(app.Fonts.Body, widgets.DarkGray, image.Pt((screenWidth-app.Fonts.Body.Width(app.Loading.Status))/2, top), app.Loading.Status)
	widgets.ExitButton(app.Fonts, &app.Targets)
}

// Command leaves the app on Back.
func (screen *Servers) Command(order widgets.Command) { exitOnBack(order) }

// drawServerButtonLocked draws the button of a server: the logo of its kind on the left, its
// name, and its address under it.
func (screen *Servers) drawServerButtonLocked(rect image.Rectangle, server servers.Config) {
	app := screen.app
	widgets.Frame(rect, widgets.Px(3), widgets.Black)
	logoBox := image.Rect(rect.Min.X+widgets.Px(serverLogoInset), rect.Min.Y+widgets.Px(serverLogoInset), rect.Min.X+rect.Dy()-widgets.Px(serverLogoInset), rect.Max.Y-widgets.Px(serverLogoInset))
	if app.Logos.Draw(app.KindOf(server), logoBox) {
		// The text is centred in the room right of the logo, kept as clear of the button's
		// right edge as of the logo.
		rect.Min.X = logoBox.Max.X
		rect.Max.X -= logoBox.Dx() + widgets.Px(serverLogoInset)
	}
	gap := widgets.Px(10)
	textTop := rect.Min.Y + (rect.Dy()-app.Fonts.Bold.Size-gap-app.Fonts.Small.Size)/2
	name := widgets.Ellipsize(app.Fonts.Bold, widgets.Clean(server.Name), rect.Dx()-2*widgets.Px(40))
	widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt(rect.Min.X+(rect.Dx()-app.Fonts.Bold.Width(name))/2, textTop), name)
	address := widgets.Ellipsize(app.Fonts.Small, widgets.Clean(server.Server), rect.Dx()-2*widgets.Px(40))
	widgets.Text(app.Fonts.Small, widgets.DarkGray, image.Pt(rect.Min.X+(rect.Dx()-app.Fonts.Small.Width(address))/2, textTop+app.Fonts.Bold.Size+gap), address)
}

// choose gives the session server index of config.json, and shows the loading screen.
func (screen *Servers) choose(index int) {
	app := screen.app
	app.Update(true, func() bool {
		if app.Screen != state.ServersScreen || index >= len(app.Loading.Servers) {
			return false
		}
		// The session waits for the choice. Leaving the screen makes it the only one.
		app.Work.Begin(state.LoadingWork) // until the session has loaded the server chosen
		app.ServerChoice <- index
		app.ShowLocked(state.LoadingScreen)
		return true
	})
}

// exitOnBack leaves the app if order is Back: the startup screens have nothing to go back to.
func exitOnBack(order widgets.Command) {
	if order == widgets.Back {
		pocketbook.Exit()
	}
}
