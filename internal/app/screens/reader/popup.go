package reader

import (
	"fmt"
	"image"
	"log"
	"math"

	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/reading"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// linkTapSize is the least width and height of a link's tap target, centred on the link, in
// InkPad 3 pixels: a note's call may be a single small digit.
const linkTapSize = 72

// The popup over the lower part of the page, in InkPad 3 pixels: its height at most, the line
// above it, and the margins of its text; its page count goes in the bottom one.
const (
	popupHeight       = 780
	popupRule         = 4
	popupMarginTop    = 56
	popupMarginBottom = 96
)

// drawLinksLocked makes the links of the page shown targets that follow them.
func (screen *Screen) drawLinksLocked() {
	app := screen.app
	text := app.Reader.Text
	page, _ := text.Book.Page(text.Book.At()) // laid out already, as it shows
	text.Links = page.Links
	for index, link := range page.Links {
		if rect := linkTapRect(link.Rect); !rect.Empty() {
			app.Targets.Tap(rect, func() { screen.followLink(index) })
		}
	}
}

// drawPopupLocked draws the popup over the lower part of the page: a white panel under a black
// line, a page of its text, and its page count if it has several. A tap in it shows its next
// page; a tap anywhere else closes it.
func (screen *Screen) drawPopupLocked(popup *state.NotePopup) {
	app := screen.app
	rect := popupRect(popup)
	if popup.Drawn == nil {
		drawn, err := app.Reader.Text.Book.DrawPopupPage(popup.Pages[popup.Page], popup.Height)
		if err != nil {
			log.Printf("popup page %d: %v", popup.Page, err)
		}
		popup.Drawn = drawn
	}
	if popup.Drawn != nil {
		pocketbook.DrawImage(popup.Drawn, rect.Min)
	} else {
		pocketbook.Fill(rect, widgets.White)
	}
	pocketbook.Fill(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+widgets.Px(popupRule)), widgets.Black)
	if len(popup.Pages) > 1 {
		label := fmt.Sprintf("%d / %d", popup.Page+1, len(popup.Pages))
		if popup.Page+1 < len(popup.Pages) {
			label = "More › " + label
		}
		font := app.Fonts.Small
		bottom := image.Rect(rect.Min.X, rect.Max.Y-widgets.Px(popupMarginBottom), rect.Max.X-widgets.Px(openbook.TextMarginSide), rect.Max.Y)
		widgets.Text(font, widgets.DarkGray, image.Pt(bottom.Max.X-font.Width(label), font.CenterTop(bottom)), label)
	}
	app.Targets.Tap(rect, func() { screen.turnPopup(true) })
	app.Targets.Tap(pocketbook.Screen(), func() { screen.closePopup() })
}

// followLink follows link linkIndex of the page shown. A footnote, a part of the book that is
// no chapter, or a URL shows in the popup, over the page: reading goes on where it was. Any
// other link jumps to its target; the book keeps the page left for the toolbar's Return.
func (screen *Screen) followLink(linkIndex int) {
	app := screen.app
	jumped := false
	app.Update(false, func() bool {
		text := app.Reader.Text
		if app.Screen != state.ReaderScreen || text == nil || text.Popup != nil || linkIndex < 0 || linkIndex >= len(text.Links) {
			return false
		}
		target := text.Links[linkIndex].Target
		pages, location, err := text.Book.Follow(target, popupPage())
		switch {
		case err != nil:
			log.Printf("link to %s: %v", target, err)
			pages, err = text.Book.MessagePages(fmt.Sprintf("Cannot follow this link: %s", widgets.Clean(err.Error())), popupPage())
			if err != nil {
				app.Reader.PageErr = err
				return true
			}
		case len(pages) == 0:
			log.Printf("link to %s: chapter %d offset %d", target, location.Chapter, location.Offset)
			openbook.JumpTextLocked(app, location)
			app.Repaint.FullRefresh = true
			jumped = true
			return true
		}
		log.Printf("link to %s: a popup of %d pages", target, len(pages))
		under, _ := text.Book.Page(text.Book.At()) // laid out already, as it shows
		text.Popup = newPopup(pages, under)
		return true
	})
	if jumped {
		app.Syncer.Schedule(openbook.SyncDelay)
	}
}

// turnPopup shows the popup's next or previous page, or closes it past its first or last one,
// and reports whether a popup was open.
func (screen *Screen) turnPopup(forward bool) bool {
	app := screen.app
	open := false
	app.Update(false, func() bool {
		text := app.Reader.Text
		if app.Screen != state.ReaderScreen || text == nil || text.Popup == nil {
			return false
		}
		open = true
		popup := text.Popup
		next := popup.Page - 1
		if forward {
			next = popup.Page + 1
		}
		if next < 0 || next >= len(popup.Pages) {
			text.Popup = nil
			return true
		}
		popup.Page, popup.Drawn = next, nil
		return true
	})
	return open
}

// closePopup closes the popup, and reports whether one was open.
func (screen *Screen) closePopup() bool {
	app := screen.app
	closed := false
	app.Update(false, func() bool {
		text := app.Reader.Text
		closed = app.Screen == state.ReaderScreen && text != nil && text.Popup != nil
		if closed {
			text.Popup = nil
		}
		return closed
	})
	return closed
}

// newPopup returns a popup of pages over page under. It is popupHeight high, or, with a single
// page, as high as its text and its top margin; then it is raised over the line of under that
// its top would cut in two.
func newPopup(pages []layout.Page, under layout.Page) *state.NotePopup {
	popup := &state.NotePopup{Pages: pages, Height: widgets.Px(popupHeight)}
	if len(pages) == 1 {
		bottom := 0
		for _, line := range pages[0].Lines {
			_, lineBottom := lineSpan(line)
			bottom = max(bottom, lineBottom)
		}
		for _, placed := range pages[0].Images {
			bottom = max(bottom, placed.Rect.Max.Y)
		}
		popup.Height = min(popup.Height, bottom+widgets.Px(popupMarginTop))
	}
	screenHeight := pocketbook.Screen().Dy()
	for _, line := range under.Lines {
		if lineTop, lineBottom := lineSpan(line); lineTop < screenHeight-popup.Height && screenHeight-popup.Height < lineBottom {
			popup.Height = screenHeight - lineTop + widgets.Px(popupRule)
		}
	}
	return popup
}

// lineSpan returns the top and the bottom of a line of text, from its words.
func lineSpan(line layout.Line) (top, bottom int) {
	top = math.MaxInt
	for _, word := range line.Words {
		top = min(top, word.Top)
		bottom = max(bottom, word.Top+word.Size*openbook.TextLineSpacing/100)
	}
	return top, bottom
}

// popupRect returns where popup shows, across the lower part of the screen; nothing if nil.
func popupRect(popup *state.NotePopup) image.Rectangle {
	if popup == nil {
		return image.Rectangle{}
	}
	screenRect := pocketbook.Screen()
	return image.Rect(0, screenRect.Dy()-popup.Height, screenRect.Dx(), screenRect.Dy())
}

// popupPage returns the page the popup's text is laid out on: popupHeight high, with its
// margins. The reading engine keeps the book's font size and side margins.
func popupPage() reading.PopupPage {
	return reading.PopupPage{Height: widgets.Px(popupHeight), MarginTop: widgets.Px(popupMarginTop), MarginBottom: widgets.Px(popupMarginBottom)}
}

// textArea returns where the text of a page of book text goes. A link's target stays in it, so
// that a tap in the margins always turns the page.
func textArea() image.Rectangle {
	screenRect := pocketbook.Screen()
	return image.Rect(widgets.Px(openbook.TextMarginSide), widgets.Px(openbook.TextMarginTop), screenRect.Dx()-widgets.Px(openbook.TextMarginSide), screenRect.Dy()-widgets.Px(openbook.TextMarginBottom))
}

// linkTapRect returns where a tap follows the link in rect: rect grown to linkTapSize at least
// each way, within the text's area.
func linkTapRect(rect image.Rectangle) image.Rectangle {
	growX, growY := max(widgets.Px(linkTapSize)-rect.Dx(), 0)/2, max(widgets.Px(linkTapSize)-rect.Dy(), 0)/2
	return image.Rect(rect.Min.X-growX, rect.Min.Y-growY, rect.Max.X+growX, rect.Max.Y+growY).Intersect(textArea())
}
