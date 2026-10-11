// Package booklist is the screen of the books of a series. The books on the reader come first,
// then the others, in the series' order, filtered by the book filter; the list opens at the
// book to continue. A tap on a book reads it, or downloads it; holding a book opens a menu to
// download several, mark it read or unread, or delete it.
package booklist

import (
	"fmt"
	"image"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/covers"
	"github.com/CedricHerzog/inkshelf/internal/app/downloadqueue"
	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/list"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/progress"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// Screen is the book list.
type Screen struct {
	app *state.State
}

// New returns the book list of app.
func New(app *state.State) *Screen { return &Screen{app: app} }

// Command goes back to the series list on Back, and turns the list's pages on the page keys.
func (screen *Screen) Command(order widgets.Command) {
	app := screen.app
	switch order {
	case widgets.Back:
		screen.backToSeries()
	case widgets.Previous, widgets.Next:
		app.Update(false, func() bool { return screen.turnLocked(order == widgets.Next) })
	}
}

// TurnsPagesOnSwipe makes a swipe turn the list's pages.
func (screen *Screen) TurnsPagesOnSwipe() {}

// CloseOverlaysLocked closes the hold menu, and reports whether it was open.
func (screen *Screen) CloseOverlaysLocked() bool {
	app := screen.app
	open := app.Books.MenuOpen
	app.Books.MenuOpen = false
	return open
}

// turnLocked turns the list one page, and reports whether there was one. The list then stays
// on the page turned to, rather than going back to the book to continue.
func (screen *Screen) turnLocked(forward bool) bool {
	app := screen.app
	if app.Screen != state.BooksScreen {
		return false
	}
	app.Books.Focus = ""
	app.Books.Browsed = true
	return list.TurnLocked(app, forward, len(catalog.VisibleBooksLocked(app)))
}

// DrawLocked draws the books of the open series that pass the filter, the filter, and the hold
// menu if it is open.
func (screen *Screen) DrawLocked() {
	app := screen.app
	visible := catalog.VisibleBooksLocked(app)
	bandHeight := widgets.Px(widgets.SegmentHeight + 2*widgets.SegmentGap)
	screen.settlePageLocked(visible, bandHeight)
	continueID := catalog.ContinueBookLocked(app)
	rows := make([]list.Row, len(visible))
	for rowIndex, bookIndex := range visible {
		rows[rowIndex] = screen.rowLocked(app.Books.Items[bookIndex], bookIndex, continueID)
	}
	words := app.WordsLocked()
	count := widgets.Counted(len(app.Books.Items), words.Book, words.BookPlural)
	if len(visible) < len(app.Books.Items) {
		count = fmt.Sprintf("%d of %s", len(visible), count)
	}
	band := list.DrawLocked(app, list.Page{
		Title: app.Books.Title, BackLabel: "‹ Back",
		Count:      count,
		BandHeight: bandHeight,
	}, rows)
	margin := widgets.Px(widgets.ListMargin)
	var labels []string
	for _, choice := range state.BookFilters {
		labels = append(labels, choice.Label)
	}
	segmentsRect := image.Rect(margin, band.Min.Y+widgets.Px(widgets.SegmentGap), band.Max.X-margin, band.Min.Y+widgets.Px(widgets.SegmentGap+widgets.SegmentHeight))
	widgets.Segments(app.Fonts, &app.Targets, segmentsRect, labels, app.Preferences.BookIndex(), screen.chooseFilter)
	widgets.SegmentsRule(band)
	if len(rows) == 0 && len(app.Books.Items) > 0 {
		message := "No books match this filter"
		widgets.Text(app.Fonts.Body, widgets.DarkGray, image.Pt((band.Dx()-app.Fonts.Body.Width(message))/2, band.Max.Y+widgets.Px(150)), message)
	}
	if app.Books.MenuOpen && app.Books.MenuIndex < len(app.Books.Items) {
		screen.drawMenuLocked()
	}
}

// settlePageLocked turns the list to the page of the book to focus on, if one is set, and
// clears it. Drawing does it, as only drawing knows how many rows a page holds.
func (screen *Screen) settlePageLocked(visible []int, bandHeight int) {
	app := screen.app
	if app.Books.Focus == "" {
		return
	}
	for position, bookIndex := range visible {
		if app.Books.Items[bookIndex].ID == app.Books.Focus {
			app.List.Page = position / list.RowsPerPage(bandHeight)
		}
	}
	app.Books.Focus = ""
}

// rowLocked returns the row of book, at bookIndex in the book list. continueID is the ID of
// the book to read next.
func (screen *Screen) rowLocked(book state.BookItem, bookIndex int, continueID string) list.Row {
	app := screen.app
	var subtitle []string
	if book.ID == continueID {
		subtitle = append(subtitle, "Continue")
	}
	switch {
	case book.ID == app.Queue.Downloading:
		subtitle = append(subtitle, downloadqueue.LabelLocked(app)+" (tap to cancel)")
	case app.Queue.Waits(book.ID):
		subtitle = append(subtitle, "Queued")
	case book.Downloaded:
		subtitle = append(subtitle, "Downloaded")
	default:
		subtitle = append(subtitle, "Tap to download")
	}
	switch {
	case book.Pages > 0 && book.Format == servers.Reflowable:
		subtitle = append(subtitle, fmt.Sprintf("~%d pages", book.Pages))
	case book.Pages > 0:
		subtitle = append(subtitle, widgets.Counted(book.Pages, "page", "pages"))
	case book.Format == servers.ComicArchive:
		subtitle = append(subtitle, "CBZ")
	case book.Format == servers.Reflowable:
		subtitle = append(subtitle, "EPUB")
	}
	switch {
	case book.Completed:
		subtitle = append(subtitle, "Read")
	case book.Page > 0:
		subtitle = append(subtitle, catalog.ProgressLabel(book.Book))
	}
	bookRow := list.Row{
		Cover:     state.CoverRequest{CoverKey: state.CoverKey{Kind: shelf.BookCover, ID: book.ID}, Modified: book.Modified},
		Title:     book.Title,
		Subtitle:  strings.Join(subtitle, " · "),
		Open:      func() { screen.open(bookIndex) },
		ShowCover: func() { covers.ShowFull(app, state.BooksScreen, shelf.BookCover, book.ID) },
		Hold:      func() { screen.openMenu(bookIndex) },
		Read:      book.Completed,
	}
	if book.Downloaded && book.ID != app.Queue.Downloading {
		bookRow.Delete = func() { screen.delete(bookIndex) }
	}
	if !book.Completed && book.Page > 0 && book.Pages > 0 {
		bookRow.Progress = float64(book.Page) / float64(book.Pages)
	}
	return bookRow
}

// drawMenuLocked draws the hold menu of a book over the list. Its buttons are the only
// targets: a tap anywhere else closes it.
func (screen *Screen) drawMenuLocked() {
	app := screen.app
	screenRect := pocketbook.Screen()
	app.Targets.Clear()
	book := app.Books.Items[app.Books.MenuIndex]
	buttons := screen.menuButtonsLocked(book)

	margin := widgets.Px(widgets.WideMargin)
	titleLines := widgets.WrapWords(app.Fonts.Bold, widgets.Clean(book.Title), screenRect.Dx()-4*margin)
	height := 2*widgets.Px(60) + len(titleLines)*app.Fonts.Bold.LineHeight() + widgets.Px(40) + len(buttons)*widgets.Px(widgets.PanelButtonStep) - widgets.Px(widgets.PanelButtonStep-widgets.PanelButtonHeight)
	box := image.Rect(margin, (screenRect.Dy()-height)/2, screenRect.Dx()-margin, (screenRect.Dy()+height)/2)
	pocketbook.Fill(box, widgets.White)
	widgets.Frame(box, widgets.Px(4), widgets.Black)
	layout := widgets.Panel{Fonts: app.Fonts, Targets: &app.Targets, Area: box, Top: box.Min.Y + widgets.Px(60)}
	layout.Line(app.Fonts.Bold, widgets.Black, book.Title)
	layout.Top += widgets.Px(40)
	for _, item := range buttons {
		layout.Button(item)
	}
	app.Targets.Add(widgets.Target{Rect: screenRect}) // closes the menu, as any tap does
}

// menuButtonsLocked returns the buttons of the hold menu of book, the last one closing it.
func (screen *Screen) menuButtonsLocked(book state.BookItem) []widgets.PanelButton {
	app := screen.app
	index := app.Books.MenuIndex
	var buttons []widgets.PanelButton
	if toQueue := len(downloadqueue.FromHereLocked(app, index)); app.Server.Online && toQueue > 1 {
		buttons = append(buttons, widgets.PanelButton{Label: fmt.Sprintf("Download %d books from here", toQueue), OnTap: widgets.Target{Tap: func() { screen.queueFrom(index) }}})
	}
	if !book.Completed {
		buttons = append(buttons, widgets.PanelButton{Label: "Mark read", OnTap: widgets.Target{Tap: func() { screen.markRead(index, true) }}})
	}
	if book.Completed || book.Page > 0 {
		buttons = append(buttons, widgets.PanelButton{Label: "Mark unread", OnTap: widgets.Target{Tap: func() { screen.markRead(index, false) }}})
	}
	if book.Downloaded && book.ID != app.Queue.Downloading {
		buttons = append(buttons, widgets.PanelButton{Label: "Delete download", OnTap: widgets.Target{Tap: func() { screen.delete(index) }}})
	}
	return append(buttons, widgets.PanelButton{Label: "Cancel"}) // any tap closes the menu
}

// backToSeries goes back to the series list, at the page it was on.
func (screen *Screen) backToSeries() {
	app := screen.app
	app.Update(true, func() bool {
		app.List.Page = app.Series.Page
		app.Books.SeriesID = ""
		app.List.Notice = ""
		app.ShowLocked(state.SeriesScreen)
		return true
	})
}

// open reads book bookIndex if it is on the reader. Otherwise it queues its download, or
// takes it out of the queue, or cancels its download if it is the one downloading.
func (screen *Screen) open(bookIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.BooksScreen || bookIndex >= len(app.Books.Items) {
			return false
		}
		item := app.Books.Items[bookIndex]
		switch {
		case item.Downloaded:
			openbook.OpenLocked(app, item)
			app.Repaint.FullRefresh = true
		case item.ID == app.Queue.Downloading:
			app.Queue.Cancel()
		case downloadqueue.DequeueLocked(app, item.ID):
			app.List.Notice = "Not downloading " + item.Title
		case !app.Server.Online:
			app.List.Notice = "Offline: cannot download"
		default:
			downloadqueue.EnqueueLocked(app, item.Book)
			app.Books.Focus = item.ID // it moves up with the downloaded books
		}
		return true
	})
}

// openMenu opens the hold menu of book bookIndex.
func (screen *Screen) openMenu(bookIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.BooksScreen || bookIndex >= len(app.Books.Items) {
			return false
		}
		app.Books.MenuOpen = true
		app.Books.MenuIndex = bookIndex
		return true
	})
}

// markRead marks book bookIndex read, or unread, on the server and on the reader.
func (screen *Screen) markRead(bookIndex int, read bool) {
	app := screen.app
	var item state.BookItem
	var client servers.Server
	app.Update(false, func() bool {
		if app.Screen != state.BooksScreen || bookIndex >= len(app.Books.Items) {
			return false
		}
		if !app.Server.Online {
			app.List.Notice = "Offline: cannot change the progress"
			return true
		}
		item = app.Books.Items[bookIndex]
		client = app.Server.Client
		return false
	})
	if client == nil {
		return
	}
	app.Go("sending the read state", func() {
		page, err := progress.SendReadState(app.Ctx, client, item.ID, item.Pages, read)
		if app.Ctx.Err() != nil {
			return
		}
		app.Update(false, func() bool {
			catalog.ApplyReadStateLocked(app, item, page, read, err)
			return true
		})
	})
}

// delete removes downloaded book bookIndex from the reader.
func (screen *Screen) delete(bookIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.BooksScreen || bookIndex >= len(app.Books.Items) {
			return false
		}
		book := app.Books.Items[bookIndex]
		if !book.Downloaded || book.ID == app.Queue.Downloading {
			return false
		}
		catalog.RemoveDownloadLocked(app, book.ID)
		return true
	})
}

// queueFrom queues book bookIndex and the next unread books not on the reader, from the hold
// menu: downloadqueue.BooksFromHere in all.
func (screen *Screen) queueFrom(bookIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.BooksScreen || bookIndex >= len(app.Books.Items) {
			return false
		}
		if !app.Server.Online {
			app.List.Notice = "Offline: cannot download"
			return true
		}
		toQueue := downloadqueue.FromHereLocked(app, bookIndex)
		for position, index := range toQueue {
			downloadqueue.EnqueueLocked(app, app.Books.Items[index].Book)
			if position == 0 {
				app.Books.Focus = app.Books.Items[index].ID
			}
		}
		words := app.WordsLocked()
		app.List.Notice = "Queued " + widgets.Counted(len(toQueue), words.Book, words.BookPlural)
		return true
	})
}

// chooseFilter shows the books that pass state.BookFilters[index].
func (screen *Screen) chooseFilter(index int) {
	app := screen.app
	catalog.ChangeFilter(app, state.BooksScreen, func() {
		if index < len(state.BookFilters) {
			app.Preferences.Books = state.BookFilters[index].Name
		}
	})
}
