package catalog

import (
	"log"

	"github.com/CedricHerzog/inkshelf/internal/app/state"
)

// ChangeFilter changes a filter of the list shown on screen on with choose, which runs with the
// mutex held. If the filters changed, the list shows its first page, and the preferences are
// saved.
func ChangeFilter(app *state.State, on state.ScreenID, choose func()) {
	var saved state.Preferences
	changed := false
	app.Update(false, func() bool {
		if app.Screen != on {
			return false
		}
		before := app.Preferences
		choose()
		changed = app.Preferences.Library != before.Library || app.Preferences.Status != before.Status || app.Preferences.Books != before.Books
		if changed {
			app.List.Page = 0
			app.Books.Focus = ""
			app.Books.Browsed = true
		}
		saved = app.Preferences
		return changed
	})
	if changed {
		if err := saved.Save(app.ServerDir); err != nil {
			log.Printf("saving filters: %v", err)
		}
	}
}
