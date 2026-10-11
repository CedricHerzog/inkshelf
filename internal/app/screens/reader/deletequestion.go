package reader

import (
	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// DeleteQuestion is the screen that asks, once a book is read to the end and closed, whether to
// delete it from the reader: finished books would otherwise fill its storage.
type DeleteQuestion struct {
	app *state.State
}

// NewDeleteQuestion returns the delete question of app.
func NewDeleteQuestion(app *state.State) *DeleteQuestion { return &DeleteQuestion{app: app} }

// DrawLocked asks whether to delete the book just finished, and says how much room it takes.
func (question *DeleteQuestion) DrawLocked() {
	app := question.app
	screenRect := pocketbook.Screen()
	layout := widgets.Panel{Fonts: app.Fonts, Targets: &app.Targets, Area: screenRect, Top: screenRect.Dy() / 4}
	layout.Line(app.Fonts.Body, widgets.DarkGray, "You finished")
	layout.Line(app.Fonts.Bold, widgets.Black, app.AskDelete.Book.Title)
	layout.Gap()
	layout.Line(app.Fonts.Body, widgets.Black, "Delete it from the reader?")
	space := "It frees " + widgets.FormatBytes(app.AskDelete.Size)
	if free, ok := app.Options.FreeSpace(app.ServerDir); ok {
		space += " (" + widgets.FormatBytes(free) + " free)"
	}
	layout.Line(app.Fonts.Small, widgets.DarkGray, space)
	layout.Gap()
	layout.Button(widgets.PanelButton{Label: "Delete", OnTap: widgets.Target{Tap: func() { question.answer(true) }}})
	layout.Button(widgets.PanelButton{Label: "Keep", OnTap: widgets.Target{Tap: func() { question.answer(false) }}})
}

// Command keeps the book on Back, the safe answer. The page keys do nothing: there is no page
// to turn.
func (question *DeleteQuestion) Command(order widgets.Command) {
	if order == widgets.Back {
		question.answer(false)
	}
}

// answer deletes the book asked about, or keeps it, then opens the next book if the reader
// asked for it, else shows the book list.
func (question *DeleteQuestion) answer(remove bool) {
	app := question.app
	app.Update(true, func() bool {
		if app.Screen != state.AskDeleteScreen {
			return false
		}
		app.ShowLocked(state.BooksScreen)
		if remove {
			catalog.RemoveDownloadLocked(app, app.AskDelete.Book.ID)
		}
		if app.AskDelete.Next != nil {
			openbook.OpenLocked(app, *app.AskDelete.Next)
		}
		app.AskDelete.Next = nil
		return true
	})
	// The Syncer deletes a book marked for removal once it has sent its progress.
	app.Syncer.Schedule(0)
}
