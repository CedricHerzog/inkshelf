package reader

import (
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/downloadqueue"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// drawEndLocked draws the end panel, which shows past a book's last page: the next book of the
// series, with buttons to read or download it, or to go back to the list. It refreshes the
// whole panel, as the page it replaces had nothing in common with it.
func (screen *Screen) drawEndLocked(fullRefresh bool) {
	app := screen.app
	screenRect := pocketbook.Screen()
	app.Reader.Shown = state.Shown{} // back on the page, the page is refreshed whole
	pocketbook.ClearScreen()
	layout := widgets.Panel{Fonts: app.Fonts, Targets: &app.Targets, Area: screenRect, Top: screenRect.Dy() / 4}
	layout.Line(app.Fonts.Body, widgets.DarkGray, "End of")
	layout.Line(app.Fonts.Bold, widgets.Black, app.Reader.OpenBook.Title)
	layout.Gap()
	buttons := screen.drawNextBookLocked(&layout)
	layout.Gap()
	words := app.WordsLocked()
	for _, item := range append(buttons, widgets.PanelButton{Label: "Back to " + words.BookPlural, OnTap: widgets.Target{Command: widgets.Back}}) {
		layout.Button(item)
	}
	hint := "Previous page: back to the " + words.Book
	widgets.Text(app.Fonts.Small, widgets.DarkGray, image.Pt((screenRect.Dx()-app.Fonts.Small.Width(hint))/2, screenRect.Dy()-widgets.Px(150)), hint)
	// Dark or light as the book's pages, so that the whole panel does not change color.
	widgets.Show(openbook.DarkPageLocked(app), fullRefresh, pocketbook.GC16, screenRect)
}

// drawNextBookLocked draws in layout the next book of the series and whether the reader has it,
// and returns the buttons to read or download it.
func (screen *Screen) drawNextBookLocked(layout *widgets.Panel) []widgets.PanelButton {
	app := screen.app
	next, ok := openbook.NextBookLocked(app)
	if !ok {
		if app.Server.Online {
			words := app.WordsLocked()
			layout.Line(app.Fonts.Body, widgets.DarkGray, "The last "+words.Book+" of the "+words.Series)
		}
		return nil
	}
	layout.Line(app.Fonts.Body, widgets.DarkGray, "Next")
	layout.Line(app.Fonts.Bold, widgets.Black, next.Title)
	var buttons []widgets.PanelButton
	status := "Not downloaded"
	switch {
	case next.Downloaded:
		status = "Downloaded"
		buttons = append(buttons, widgets.PanelButton{Label: "Read next", OnTap: widgets.Target{Tap: screen.readNext}})
	case next.ID == app.Queue.Downloading:
		status = downloadqueue.LabelLocked(app)
	case app.Queue.Waits(next.ID):
		status = "Queued"
	case app.Server.Online:
		buttons = append(buttons, widgets.PanelButton{Label: "Download next", OnTap: widgets.Target{Tap: screen.downloadNext}})
	default:
		status = "Not downloaded, offline"
	}
	layout.Line(app.Fonts.Small, widgets.DarkGray, status)
	return buttons
}

// readNext closes the open book and opens the next one, once the delete question, if it is
// asked, is answered.
func (screen *Screen) readNext() {
	app := screen.app
	opened := false
	app.Update(true, func() bool {
		next, ok := openbook.NextBookLocked(app)
		if app.Screen != state.ReaderScreen || !app.Reader.AtEnd || !ok || !next.Downloaded {
			return false
		}
		openbook.CloseLocked(app)
		openbook.AskToDeleteLocked(app, &next)
		opened = true
		return true
	})
	if opened {
		app.Syncer.Schedule(0)
	}
}

// downloadNext queues the download of the book after the open one.
func (screen *Screen) downloadNext() {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.ReaderScreen || !app.Reader.AtEnd {
			return false
		}
		openbook.QueueNextLocked(app)
		return true
	})
}
