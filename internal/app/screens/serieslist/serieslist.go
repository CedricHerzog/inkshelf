// Package serieslist is the screen of the series list, the first screen once a server is
// chosen. It shows the book to continue, then the series that pass the filters (the library
// and the read status), a page at a time. The A–Z jump, opened from the page number, turns to
// the first series of a letter. A tap on a series shows its books; a tap on the title reads the
// list again from the server.
package serieslist

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/covers"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/list"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// Screen is the series list.
type Screen struct {
	app *state.State
}

// New returns the series list of app.
func New(app *state.State) *Screen { return &Screen{app: app} }

// Command leaves the app on Back, and turns the list's pages on the page keys.
func (screen *Screen) Command(order widgets.Command) {
	app := screen.app
	switch order {
	case widgets.Back:
		pocketbook.Exit()
	case widgets.Previous, widgets.Next:
		app.Update(false, func() bool { return screen.turnLocked(order == widgets.Next) })
	}
}

// TurnsPagesOnSwipe makes a swipe turn the list's pages.
func (screen *Screen) TurnsPagesOnSwipe() {}

// CloseOverlaysLocked closes the A–Z jump, and reports whether it was open.
func (screen *Screen) CloseOverlaysLocked() bool {
	app := screen.app
	open := app.Series.JumpOpen
	app.Series.JumpOpen = false
	return open
}

// turnLocked turns the list one page, and reports whether there was one.
func (screen *Screen) turnLocked(forward bool) bool {
	app := screen.app
	if app.Screen != state.SeriesScreen {
		return false
	}
	count := len(catalog.VisibleSeriesLocked(app)) + catalog.ResumeRowsLocked(app)
	return list.TurnLocked(app, forward, count)
}

// DrawLocked draws the series list: the book to continue, then the series that pass the
// filters; the filters; and the A–Z jump over them if it is open.
func (screen *Screen) DrawLocked() {
	app := screen.app
	visible := catalog.VisibleSeriesLocked(app)
	words := app.WordsLocked()
	rows := make([]list.Row, len(visible))
	for rowIndex, seriesIndex := range visible {
		rows[rowIndex] = screen.seriesRowLocked(app.Series.Items[seriesIndex], seriesIndex, words)
	}
	if resume := app.Series.Resume; resume != nil {
		rows = append([]list.Row{screen.resumeRowLocked(*resume)}, rows...)
	}
	count := widgets.Counted(len(app.Series.Items), words.Series, words.SeriesPlural)
	if len(visible) < len(app.Series.Items) {
		count = fmt.Sprintf("%d of %s", len(visible), count)
	}
	if app.Server.Client != nil {
		count += " · tap to refresh"
	}
	filterLines := 1
	if len(app.Preferences.Libraries) > 1 {
		filterLines = 2
	}
	title := app.KindTitleLocked()
	if app.Server.Name != "" {
		title = app.Server.Name
	}
	page := list.Page{
		Title: title, BackLabel: "Exit", Logo: true,
		Count:          count,
		BandHeight:     filterLines*widgets.Px(widgets.SegmentHeight+widgets.SegmentGap) + widgets.Px(widgets.SegmentGap),
		OnPageLabelTap: screen.openJump,
	}
	if app.Server.Client != nil {
		page.OnTitleTap = func() { catalog.Refresh(app) }
	}
	band := list.DrawLocked(app, page, rows)
	screen.drawFiltersLocked(band)
	if len(visible) == 0 && len(app.Series.Items) > 0 {
		message := "No " + words.SeriesPlural + " match these filters"
		widgets.Text(app.Fonts.Body, widgets.DarkGray, image.Pt((band.Dx()-app.Fonts.Body.Width(message))/2, band.Max.Y+widgets.Px(150)), message)
	}
	if app.Series.JumpOpen {
		screen.drawJumpLocked()
	}
}

// seriesRowLocked returns the row of series item, at seriesIndex in the series list, its books
// counted with words.
func (screen *Screen) seriesRowLocked(item state.SeriesItem, seriesIndex int, words servers.Words) list.Row {
	app := screen.app
	var subtitle []string
	if item.Books > 0 {
		subtitle = append(subtitle, widgets.Counted(item.Books, words.Book, words.BookPlural))
	}
	switch item.Status() {
	case servers.Read:
		subtitle = append(subtitle, "Read")
	case servers.Reading:
		if item.Read > 0 {
			subtitle = append(subtitle, fmt.Sprintf("%d read", item.Read))
		} else {
			subtitle = append(subtitle, "Started")
		}
	}
	if item.Local > 0 {
		subtitle = append(subtitle, fmt.Sprintf("%d downloaded", item.Local))
	}
	listRow := list.Row{
		Cover:     state.CoverRequest{CoverKey: state.CoverKey{Kind: shelf.SeriesCover, ID: item.ID}, Modified: item.Modified},
		Title:     item.Title,
		Subtitle:  strings.Join(subtitle, " · "),
		Open:      func() { screen.open(seriesIndex, "") },
		ShowCover: func() { covers.ShowFull(app, state.SeriesScreen, shelf.SeriesCover, item.ID) },
		Read:      item.Status() == servers.Read,
	}
	if item.Status() == servers.Reading && item.Books > 0 {
		listRow.Progress = float64(item.Read) / float64(item.Books)
	}
	return listRow
}

// resumeRowLocked returns the list row of the book to continue.
func (screen *Screen) resumeRowLocked(resume state.BookItem) list.Row {
	progress := fmt.Sprintf("Page %d of %d", resume.Page, resume.Pages)
	if resume.Format == servers.Reflowable {
		progress = catalog.ProgressLabel(resume.Book)
	}
	subtitle := []string{resume.SeriesTitle, progress}
	if resume.Downloaded {
		subtitle = append(subtitle, "Downloaded")
	}
	return list.Row{
		Cover:    state.CoverRequest{CoverKey: state.CoverKey{Kind: shelf.BookCover, ID: resume.ID}, Modified: resume.Modified},
		Title:    "Continue: " + resume.Title,
		Subtitle: strings.Join(subtitle, " · "),
		Open:     screen.resume,
		Progress: float64(resume.Page) / float64(max(resume.Pages, 1)),
	}
}

// drawFiltersLocked draws the filters in band: the libraries, when there are several, then the
// read status.
func (screen *Screen) drawFiltersLocked(band image.Rectangle) {
	app := screen.app
	margin := widgets.Px(widgets.ListMargin)
	top := band.Min.Y + widgets.Px(widgets.SegmentGap)
	nextLine := func() image.Rectangle {
		rect := image.Rect(margin, top, band.Max.X-margin, top+widgets.Px(widgets.SegmentHeight))
		top += widgets.Px(widgets.SegmentHeight + widgets.SegmentGap)
		return rect
	}
	if len(app.Preferences.Libraries) > 1 {
		labels := []string{"All"}
		for _, library := range app.Preferences.Libraries {
			labels = append(labels, library.Name)
		}
		// The first segment is All, so a segment's library is the one before it.
		widgets.Segments(app.Fonts, &app.Targets, nextLine(), labels, app.Preferences.LibraryIndex()+1, func(index int) {
			screen.chooseLibrary(index - 1)
		})
	}
	var labels []string
	for _, choice := range state.StatusFilters {
		labels = append(labels, choice.Label)
	}
	widgets.Segments(app.Fonts, &app.Targets, nextLine(), labels, app.Preferences.StatusIndex(), screen.chooseStatus)
	widgets.SegmentsRule(band)
}

// drawJumpLocked draws the A–Z jump over the list: a button per letter, gray if no series shown
// starts with it. Its letters are the only targets: a tap anywhere else closes it.
func (screen *Screen) drawJumpLocked() {
	app := screen.app
	screenRect := pocketbook.Screen()
	app.Targets.Clear()
	present := map[string]bool{}
	for _, index := range catalog.VisibleSeriesLocked(app) {
		present[app.Series.Items[index].Initial] = true
	}
	const columns = 6
	lines := (len(catalog.JumpLetters) + columns - 1) / columns
	padding, gap, cellHeight := widgets.Px(40), widgets.Px(16), widgets.Px(130)
	titleHeight := app.Fonts.Bold.LineHeight() + widgets.Px(30)
	height := 2*padding + titleHeight + lines*cellHeight + (lines-1)*gap
	box := image.Rect(widgets.Px(widgets.WideMargin), (screenRect.Dy()-height)/2, screenRect.Dx()-widgets.Px(widgets.WideMargin), (screenRect.Dy()+height)/2)
	pocketbook.Fill(box, widgets.White)
	widgets.Frame(box, widgets.Px(4), widgets.Black)
	title := "Go to"
	widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt(box.Min.X+(box.Dx()-app.Fonts.Bold.Width(title))/2, box.Min.Y+padding), title)
	gridTop := box.Min.Y + padding + titleHeight
	cellWidth := (box.Dx() - 2*padding - (columns-1)*gap) / columns
	for index, letter := range strings.Split(catalog.JumpLetters, "") {
		column, line := index%columns, index/columns
		left := box.Min.X + padding + column*(cellWidth+gap)
		top := gridTop + line*(cellHeight+gap)
		cell := image.Rect(left, top, left+cellWidth, top+cellHeight)
		widgets.Button(app.Fonts.Bold, cell, letter, present[letter])
		if present[letter] {
			app.Targets.Tap(cell, func() { screen.jump(index) })
		}
	}
	app.Targets.Add(widgets.Target{Rect: screenRect}) // closes the jump, as any tap does
}

// resume opens the book to continue: its series' books, then the book itself if it is
// downloaded.
func (screen *Screen) resume() {
	app := screen.app
	seriesIndex, bookID := -1, ""
	app.Lock()
	if app.Screen == state.SeriesScreen && app.Series.Resume != nil {
		resume := app.Series.Resume
		bookID = resume.ID
		seriesIndex = slices.IndexFunc(app.Series.Items, func(item state.SeriesItem) bool {
			return item.ID == resume.SeriesID
		})
	}
	app.Unlock()
	if seriesIndex >= 0 {
		screen.open(seriesIndex, bookID)
	}
}

// openJump shows the A–Z jump over the list.
func (screen *Screen) openJump() {
	app := screen.app
	app.Update(false, func() bool {
		app.Series.JumpOpen = app.Screen == state.SeriesScreen
		return app.Series.JumpOpen
	})
}

// jump turns the list to the page of the first series filed under letter letterIndex of
// catalog.JumpLetters.
func (screen *Screen) jump(letterIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		return screen.jumpLocked(letterIndex)
	})
}

// jumpLocked turns the list to the page of the first series filed under letter letterIndex of
// catalog.JumpLetters, and reports whether there is one.
func (screen *Screen) jumpLocked(letterIndex int) bool {
	app := screen.app
	if app.Screen != state.SeriesScreen || letterIndex >= len(catalog.JumpLetters) || app.List.Rows == 0 {
		return false
	}
	letter := catalog.JumpLetters[letterIndex : letterIndex+1]
	for position, index := range catalog.VisibleSeriesLocked(app) {
		if app.Series.Items[index].Initial == letter {
			app.List.Page = (position + catalog.ResumeRowsLocked(app)) / app.List.Rows
			return true
		}
	}
	return false
}

// open shows the books of series seriesIndex: those on the reader at once, then the server's
// list once it comes. If book openBookID is downloaded, it opens too.
func (screen *Screen) open(seriesIndex int, openBookID string) {
	app := screen.app
	var seriesID string
	var online bool
	app.Update(true, func() bool {
		if app.Screen != state.SeriesScreen || seriesIndex >= len(app.Series.Items) {
			return false
		}
		item := app.Series.Items[seriesIndex]
		online = app.Server.Online
		app.Series.Page = app.List.Page
		app.List.Page = 0
		app.List.Notice = ""
		app.Books.SeriesID = item.ID
		app.Books.Title = item.Title
		app.Books.Items = nil
		for _, book := range app.Store.Books() {
			if book.SeriesID == item.ID {
				app.Books.Items = append(app.Books.Items, state.BookItem{Book: book, Downloaded: true})
			}
		}
		if online {
			app.List.Notice = "Loading books…"
		}
		app.Books.Focus = catalog.ContinueBookLocked(app)
		app.Books.Browsed = false
		app.ShowLocked(state.BooksScreen)
		for _, book := range app.Books.Items {
			if book.ID == openBookID {
				openbook.OpenLocked(app, book)
			}
		}
		seriesID = item.ID
		return true
	})
	if seriesID == "" || !online {
		return
	}
	app.Go("loading the books", func() { catalog.FetchBooks(app, seriesID) })
}

// chooseStatus shows the series that pass state.StatusFilters[index].
func (screen *Screen) chooseStatus(index int) {
	app := screen.app
	catalog.ChangeFilter(app, state.SeriesScreen, func() {
		if index < len(state.StatusFilters) {
			app.Preferences.Status = state.StatusFilters[index].Name
		}
	})
}

// chooseLibrary shows the series of library index of the preferences' Libraries, or of every
// library if index is -1.
func (screen *Screen) chooseLibrary(index int) {
	app := screen.app
	catalog.ChangeFilter(app, state.SeriesScreen, func() {
		switch {
		case index < 0:
			app.Preferences.Library = ""
		case index < len(app.Preferences.Libraries):
			app.Preferences.Library = app.Preferences.Libraries[index].ID
		}
	})
}
