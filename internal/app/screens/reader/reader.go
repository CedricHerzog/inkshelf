package reader

import (
	"fmt"
	"image"
	"log"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// pageWaveform is the e-ink waveform of page turns: GL16 takes about 435 ms, where GC16 takes
// 576 ms for a little less ghosting.
const pageWaveform = pocketbook.GL16

// The widths of the toolbar's buttons, in InkPad 3 pixels: the direction, A− and A+, Contents
// and Return.
const (
	directionWidth      = 220
	textSizeButtonWidth = 130
	contentsButtonWidth = 240
	returnButtonWidth   = 220
)

// toolbarButton is a button of the reader's toolbar for a Reflowable book.
type toolbarButton struct {
	label       string
	buttonWidth int // in InkPad 3 pixels
	onTap       func()
}

// Screen is the reader: the page of the open book, and what shows over it.
type Screen struct {
	app *state.State
}

// New returns the reader of app.
func New(app *state.State) *Screen { return &Screen{app: app} }

// DrawLocked draws the reader and refreshes the whole panel.
func (screen *Screen) DrawLocked() { screen.DrawAndRefreshLocked(true) }

// Command performs the buttons' commands. Back closes the popup, else the book; the page keys
// turn the popup's pages, else the book's; Menu shows or hides the toolbar.
func (screen *Screen) Command(order widgets.Command) {
	switch order {
	case widgets.Back:
		if !screen.closePopup() {
			screen.closeBook()
		}
	case widgets.Previous, widgets.Next:
		if !screen.turnPopup(order == widgets.Next) {
			openbook.TurnPage(screen.app, order == widgets.Next)
		}
	case widgets.Menu:
		screen.toggleToolbar()
	}
}

// DrawAndRefreshLocked draws the page shown and what shows over it, then refreshes what changed
// of the panel, or all of it if fullRefresh is set. Past the last page, it draws the end panel.
func (screen *Screen) DrawAndRefreshLocked(fullRefresh bool) {
	app := screen.app
	defer app.Reader.Turn.End()
	if app.Reader.AtEnd {
		screen.drawEndLocked(fullRefresh)
		return
	}
	span := timing.Start(app.Reader.Turn.Context(), "draw page")
	var pageImage *image.Gray
	var loadTime time.Duration
	if app.Reader.Text != nil {
		pageImage, loadTime = openbook.CurrentTextPageLocked(app)
		openbook.PruneTextPagesLocked(app)
	} else {
		pageImage, loadTime = openbook.CurrentPageLocked(app)
		openbook.PrunePagesLocked(app)
	}
	app.PreloadWake.Wake() // to get the pages around this one ready

	var pageRect image.Rectangle
	if app.Reader.PageErr == nil {
		pageRect = centered(pageImage.Bounds().Size())
	}
	refreshArea := screen.refreshAreaLocked(fullRefresh, pageRect)
	popup, popupAt := screen.shownPopupLocked()
	app.Reader.Shown = state.Shown{Valid: true, Page: app.Reader.OpenBook.Page, Text: screen.shownTextPageLocked(),
		Toolbar: app.Reader.Toolbar, Rect: pageRect, Popup: popup, PopupAt: popupAt}

	// The canvas holds the whole screen, but only refreshArea goes to the panel.
	switch {
	case app.Reader.PageErr != nil:
		screen.drawPageErrorLocked()
	case popup != nil:
		drawPage(pageImage, pageRect)
		screen.drawPopupLocked(popup)
	default:
		drawPage(pageImage, pageRect)
		if app.Reader.Text != nil && !app.Reader.Toolbar {
			screen.drawLinksLocked()
		}
	}
	if app.Reader.Toolbar {
		screen.drawToolbarLocked()
	}
	switch {
	case fullRefresh:
		pocketbook.FlashRefresh()
	case app.Reader.PageErr == nil:
		pocketbook.Refresh(refreshArea, pageWaveform)
	default:
		pocketbook.Refresh(refreshArea, pocketbook.GC16)
	}
	pageName := fmt.Sprintf("page %d", app.Reader.OpenBook.Page)
	if app.Reader.Text != nil {
		at := app.Reader.Text.Book.At()
		pageName = fmt.Sprintf("chapter %d page %d", at.Chapter, at.Page)
	}
	log.Printf("%s: %v (load %v) area %v", pageName, span.End(), timing.Round(loadTime), refreshArea)
}

// TouchLocked returns what a touch does in the reader: what the buttons of the toolbar, the end
// panel or the popup do; else a tap on a link follows it; else the page turns, by the side
// tapped or the swipe's direction; else, in the middle, the toolbar shows.
func (screen *Screen) TouchLocked(start, end image.Point, swipe bool) widgets.Target {
	app := screen.app
	popupOpen := app.Reader.Text != nil && app.Reader.Text.Popup != nil
	if app.Reader.Toolbar || app.Reader.AtEnd || popupOpen {
		if hit, ok := app.Targets.Hit(start, end); ok {
			return hit
		}
		if app.Reader.Toolbar {
			return widgets.Target{Command: widgets.Menu} // a tap outside the toolbar's buttons hides it
		}
	}
	if !swipe {
		if hit, ok := app.Targets.Hit(start, end); ok {
			return hit // the links of a Reflowable book's page, which stay out of its margins
		}
	}
	screenWidth := pocketbook.ScreenSize().X
	// Right to left, as manga: a swipe to the right or a tap on the left side is the next page.
	nextSide := end.X < screenWidth/widgets.SideDivisor
	prevSide := end.X > screenWidth-screenWidth/widgets.SideDivisor
	nextSwipe := end.X-start.X > 0
	if !openbook.RightToLeftLocked(app) {
		nextSide, prevSide, nextSwipe = prevSide, nextSide, !nextSwipe
	}
	switch {
	case swipe && nextSwipe, !swipe && nextSide:
		return widgets.Target{Command: widgets.Next}
	case swipe, prevSide:
		return widgets.Target{Command: widgets.Previous}
	case app.Reader.AtEnd:
		return widgets.Target{}
	}
	return widgets.Target{Command: widgets.Menu}
}

// shownTextPageLocked returns the page shown of the open Reflowable book, or the zero
// PagePosition for a book of pages.
func (screen *Screen) shownTextPageLocked() reading.PagePosition {
	app := screen.app
	if app.Reader.Text == nil {
		return reading.PagePosition{}
	}
	return app.Reader.Text.Book.At()
}

// shownPopupLocked returns the popup over the page of the open Reflowable book and its page
// shown; nil if there is none.
func (screen *Screen) shownPopupLocked() (*state.NotePopup, int) {
	app := screen.app
	if app.Reader.Text == nil || app.Reader.Text.Popup == nil {
		return nil, 0
	}
	return app.Reader.Text.Popup, app.Reader.Text.Popup.Page
}

// refreshAreaLocked returns the part of the panel to refresh to show the page at pageRect: what
// changed since the reader was last drawn, or the whole screen.
func (screen *Screen) refreshAreaLocked(fullRefresh bool, pageRect image.Rectangle) image.Rectangle {
	app := screen.app
	toolbarRect := image.Rect(0, 0, pocketbook.Screen().Dx(), widgets.Px(widgets.BarHeight))
	shownBefore := app.Reader.Shown
	popup, popupAt := screen.shownPopupLocked()
	samePage := shownBefore.Page == app.Reader.OpenBook.Page && shownBefore.Text == screen.shownTextPageLocked()
	switch {
	case fullRefresh || !shownBefore.Valid || app.Reader.PageErr != nil:
		return pocketbook.Screen()
	case samePage && shownBefore.Toolbar == app.Reader.Toolbar && (shownBefore.Popup != popup || shownBefore.PopupAt != popupAt):
		return popupRect(popup).Union(popupRect(shownBefore.Popup)) // only the popup changed: opened, turned or closed
	case samePage && shownBefore.Toolbar != app.Reader.Toolbar:
		return toolbarRect // only the toolbar changed
	case shownBefore.Toolbar || app.Reader.Toolbar:
		return pageRect.Union(shownBefore.Rect).Union(toolbarRect)
	}
	return pageRect.Union(shownBefore.Rect)
}

// drawPageErrorLocked draws why the current page cannot be shown.
func (screen *Screen) drawPageErrorLocked() {
	app := screen.app
	screenRect := pocketbook.Screen()
	pocketbook.ClearScreen()
	message := "Cannot show this page: " + widgets.Clean(app.Reader.PageErr.Error())
	for lineIndex, line := range widgets.WrapWords(app.Fonts.Body, message, screenRect.Dx()-2*widgets.Px(widgets.WideMargin)) {
		widgets.Text(app.Fonts.Body, widgets.Black, image.Pt(widgets.Px(widgets.WideMargin), screenRect.Dy()/3+lineIndex*app.Fonts.Body.LineHeight()), line)
	}
}

// drawToolbarLocked draws the toolbar over the page: Back, the title, the page number, and the
// direction button, or for a Reflowable book the text size, Contents and Return buttons.
func (screen *Screen) drawToolbarLocked() {
	app := screen.app
	screenWidth := pocketbook.Screen().Dx()
	margin, bar := widgets.Px(widgets.ListMargin), widgets.Px(widgets.BarHeight)
	toolbarRect := image.Rect(0, 0, screenWidth, bar)
	pocketbook.Fill(toolbarRect, widgets.White)
	pocketbook.Fill(image.Rect(0, bar-widgets.Px(3), screenWidth, bar), widgets.Black)
	backButton := widgets.BackButton(app.Fonts, &app.Targets, "‹ Back")
	pageLabel := fmt.Sprintf("%d / %d", app.Reader.OpenBook.Page, app.Reader.OpenBook.Pages)
	if app.Reader.Text != nil && app.Reader.OpenBook.Location != nil {
		pageLabel = app.Reader.Text.Book.PageLabel(*app.Reader.OpenBook.Location)
	}
	pageLabelLeft := screenWidth - margin - app.Fonts.Body.Width(pageLabel)
	widgets.Text(app.Fonts.Body, widgets.Black, image.Pt(pageLabelLeft, app.Fonts.Body.CenterTop(toolbarRect)), pageLabel)
	buttonsLeft := pageLabelLeft - widgets.Px(40)
	if app.Reader.Text != nil {
		// A− and A+ (the size of the text), Contents, and Return after a jump.
		buttons := []toolbarButton{
			{"A+", textSizeButtonWidth, func() { openbook.ChangeTextSize(app, true) }},
			{"A−", textSizeButtonWidth, func() { openbook.ChangeTextSize(app, false) }},
			{"Contents", contentsButtonWidth, screen.openContents},
		}
		if app.Reader.Text.Book.CanReturn() {
			buttons = append(buttons, toolbarButton{"‹ Return", returnButtonWidth, screen.returnText})
		}
		for _, each := range buttons {
			rect := image.Rect(buttonsLeft-widgets.Px(each.buttonWidth), backButton.Min.Y, buttonsLeft, backButton.Max.Y)
			widgets.Button(app.Fonts.Bold, rect, each.label, true)
			app.Targets.Tap(rect, each.onTap)
			buttonsLeft = rect.Min.X - widgets.Px(24)
		}
	} else {
		direction := "LTR →"
		if openbook.RightToLeftLocked(app) {
			direction = "← RTL"
		}
		directionButton := image.Rect(buttonsLeft-widgets.Px(directionWidth), backButton.Min.Y, buttonsLeft, backButton.Max.Y)
		widgets.Button(app.Fonts.Body, directionButton, direction, true)
		app.Targets.Tap(directionButton, screen.flipDirection)
		buttonsLeft = directionButton.Min.X - widgets.Px(24)
	}
	titleLeft := backButton.Max.X + widgets.Px(40)
	title := widgets.Ellipsize(app.Fonts.Bold, widgets.Clean(app.Reader.OpenBook.Title), buttonsLeft-widgets.Px(8)-titleLeft)
	if title != "…" { // the Return button may leave no room for it
		widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt(titleLeft, app.Fonts.Bold.CenterTop(toolbarRect)), title)
	}
}

// toggleToolbar shows or hides the toolbar, or closes the popup if one shows.
func (screen *Screen) toggleToolbar() {
	app := screen.app
	if screen.closePopup() {
		return
	}
	app.Update(false, func() bool {
		app.Reader.Toolbar = !app.Reader.Toolbar && app.Screen == state.ReaderScreen && !app.Reader.AtEnd
		return app.Screen == state.ReaderScreen && !app.Reader.AtEnd
	})
}

// closeBook closes the book and goes back to the book list, unless the book was finished: then
// it asks whether to delete it.
func (screen *Screen) closeBook() {
	app := screen.app
	app.Update(true, func() bool {
		openbook.CloseLocked(app)
		openbook.AskToDeleteLocked(app, nil)
		return true
	})
	app.Syncer.Schedule(0)
}

// flipDirection reverses the reading direction of the open book's series, and saves it in the
// preferences.
func (screen *Screen) flipDirection() {
	app := screen.app
	var saved state.Preferences
	changed := false
	app.Update(false, func() bool {
		if app.Screen != state.ReaderScreen {
			return false
		}
		direction := servers.RightToLeft
		if openbook.RightToLeftLocked(app) {
			direction = servers.LeftToRight
		}
		if app.Preferences.Directions == nil {
			app.Preferences.Directions = map[string]servers.Direction{}
		}
		app.Preferences.Directions[app.Reader.OpenBook.SeriesID] = direction
		saved = app.Preferences
		changed = true
		return true
	})
	if changed {
		if err := saved.Save(app.ServerDir); err != nil {
			log.Printf("saving filters: %v", err)
		}
	}
}

// openContents shows the contents screen of the open Reflowable book, at the entry being read.
func (screen *Screen) openContents() {
	app := screen.app
	app.Update(true, func() bool {
		if app.Screen != state.ReaderScreen || app.Reader.Text == nil {
			return false
		}
		app.Contents = state.Contents{Focus: true}
		app.ShowLocked(state.ContentsScreen)
		return true
	})
}

// returnText goes back to the page the last jump left.
func (screen *Screen) returnText() {
	app := screen.app
	app.Update(true, func() bool {
		text := app.Reader.Text
		if app.Screen != state.ReaderScreen || text == nil || !text.Book.CanReturn() {
			return false
		}
		if err := text.Book.Return(); err != nil {
			app.Reader.PageErr = err
			return true
		}
		openbook.NoteTextLocationLocked(app)
		app.Reader.Toolbar = false
		return true
	})
	app.Syncer.Schedule(openbook.SyncDelay)
}

// centered returns a rectangle of size centred on the screen.
func centered(size image.Point) image.Rectangle {
	screenRect := pocketbook.Screen()
	topLeft := image.Pt((screenRect.Dx()-size.X)/2, (screenRect.Dy()-size.Y)/2)
	return image.Rectangle{Max: size}.Add(topLeft)
}

// drawPage draws pageImage at pageRect, and white around it.
func drawPage(pageImage *image.Gray, pageRect image.Rectangle) {
	screenRect := pocketbook.Screen()
	screenWidth, screenHeight := screenRect.Dx(), screenRect.Dy()
	for _, marginRect := range []image.Rectangle{
		image.Rect(0, 0, screenWidth, pageRect.Min.Y),
		image.Rect(0, pageRect.Max.Y, screenWidth, screenHeight),
		image.Rect(0, pageRect.Min.Y, pageRect.Min.X, pageRect.Max.Y),
		image.Rect(pageRect.Max.X, pageRect.Min.Y, screenWidth, pageRect.Max.Y),
	} {
		if !marginRect.Empty() {
			pocketbook.Fill(marginRect, widgets.White)
		}
	}
	pocketbook.DrawImage(pageImage, pageRect.Min)
}
