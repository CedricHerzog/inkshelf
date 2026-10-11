// Package list draws what the screens of lists share: a header with the Back button, a title
// and the reader's status; a band under it for filters or tabs; a page of rows, each with a
// cover, a title, a subtitle and a progress bar; and a footer that turns the pages.
//
// The series list and a series' book list are pages of rows. The table of contents of a book
// borrows only the header and the footer.
//
// How many rows fit on a page depends on the band's height, which only drawing knows: drawing a
// list therefore sets the number of rows per page and keeps the page shown within the list.
package list

import (
	"fmt"
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/covers"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// Row is a line of a list: a cover, a title, a gray subtitle and an optional Delete button.
type Row struct {
	Cover     state.CoverRequest
	Title     string
	Subtitle  string
	Progress  float64 // how far it is read, from 0 to 1, for a bar under the subtitle; 0: none
	Read      bool    // read to the end: a badge on the cover says so
	Open      func()  // what a tap on the row does
	ShowCover func()  // what a tap on its cover does; nil: the same as on the row
	Hold      func()  // what holding the row or its cover does; nil: nothing
	Delete    func()  // what its Delete button does; nil: the row has none
}

// Page is what a page of a list shows, besides its rows.
type Page struct {
	Header
	Count      string // under the title, unless the list's notice replaces it
	BandHeight int    // the height of the band under the header, which the caller draws
	// OnPageLabelTap, when set, makes the footer's page label a target: the series list's
	// A–Z jump. It shows only when the list has several pages.
	OnPageLabelTap func()
}

// Header is the header of a list.
type Header struct {
	Title     string
	BackLabel string
	Logo      bool   // the logo of the server's kind shows before the title
	Note      string // in gray under the title
	// OnTitleTap, when set, makes the title a target: the series list's refresh.
	OnTitleTap func()
}

// minRowHeight is the least height of a row.
const minRowHeight = 240

// The progress bar of a row: its distance from the subtitle, its height and its widest.
const (
	progressGap    = 18
	progressHeight = 8
	progressWidth  = 600
)

// RowsPerPage is the number of rows on a page of a list with a band bandHeight high under its
// header.
func RowsPerPage(bandHeight int) int {
	return max(1, (pocketbook.Screen().Dy()-2*widgets.Px(widgets.BarHeight)-bandHeight)/widgets.Px(minRowHeight))
}

// TurnLocked turns the list shown, of count rows, one page, and reports whether there was one.
func TurnLocked(app *state.State, forward bool, count int) bool {
	switch {
	case app.List.Rows == 0:
		return false
	case forward && (app.List.Page+1)*app.List.Rows < count:
		app.List.Page++
	case !forward && app.List.Page > 0:
		app.List.Page--
	default:
		return false
	}
	return true
}

// DrawLocked draws the list's page shown of rows between page's header and a footer with the
// page buttons. It leaves the band under the header for the caller to draw, and returns it.
func DrawLocked(app *state.State, page Page, rows []Row) image.Rectangle {
	screenRect := pocketbook.Screen()
	screenWidth, screenHeight := screenRect.Dx(), screenRect.Dy()
	margin, bar := widgets.Px(widgets.ListMargin), widgets.Px(widgets.BarHeight)
	page.Note = page.Count
	if app.List.Notice != "" {
		page.Note = widgets.Clean(app.List.Notice)
	}
	DrawHeaderLocked(app, page.Header)

	band := image.Rect(0, bar, screenWidth, bar+page.BandHeight)
	listArea := image.Rect(0, band.Max.Y, screenWidth, screenHeight-bar)
	pages := settlePageLocked(app, page.BandHeight, len(rows))
	rowHeight := listArea.Dy() / app.List.Rows
	first := app.List.Page * app.List.Rows
	app.Covers.Wanted = app.Covers.Wanted[:0]
	for position, row := range rows[first:min(first+app.List.Rows, len(rows))] {
		top := listArea.Min.Y + position*rowHeight
		if position > 0 {
			pocketbook.Fill(image.Rect(margin, top-widgets.Px(1), screenWidth-margin, top+widgets.Px(1)), widgets.LightGray)
		}
		drawRowLocked(app, row, image.Rect(0, top, screenWidth, top+rowHeight))
	}
	requests := make([]state.CoverRequest, len(rows))
	for index, row := range rows {
		requests[index] = row.Cover
	}
	covers.RequestNextPageLocked(app, requests, first)
	covers.PruneLocked(app, requests, first)
	onPageLabelTap := page.OnPageLabelTap
	if pages < 2 {
		onPageLabelTap = nil
	}
	DrawFooterLocked(app, app.List.Page, pages, onPageLabelTap)
	return band
}

// settlePageLocked sets the number of rows per page, keeps the page shown within the list's
// rowCount rows, and returns the number of pages.
func settlePageLocked(app *state.State, bandHeight, rowCount int) (pages int) {
	app.List.Rows = RowsPerPage(bandHeight)
	pages = max(1, (rowCount+app.List.Rows-1)/app.List.Rows)
	app.List.Page = min(app.List.Page, pages-1)
	return pages
}

// DrawHeaderLocked draws the header of a list: the Back button, the title, its note in gray,
// and the reader's status: the free space, the time and the battery.
func DrawHeaderLocked(app *state.State, header Header) {
	screenWidth := pocketbook.Screen().Dx()
	bar := widgets.Px(widgets.BarHeight)
	backButton := widgets.BackButton(app.Fonts, &app.Targets, header.BackLabel)
	titleLeft := backButton.Max.X + widgets.Px(40)
	if header.Logo {
		logoBox := image.Rect(titleLeft, backButton.Min.Y, titleLeft+backButton.Dy(), backButton.Max.Y)
		if app.Logos.Draw(app.Server.Kind, logoBox) {
			titleLeft = logoBox.Max.X + widgets.Px(24)
		}
	}
	titleTop := (bar - app.Fonts.Bold.Size - widgets.Px(10) - app.Fonts.Small.Size) / 2
	widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt(titleLeft, titleTop), widgets.Ellipsize(app.Fonts.Bold, widgets.Clean(header.Title), screenWidth-widgets.Px(widgets.ListMargin)-titleLeft))
	noteTop := titleTop + app.Fonts.Bold.Size + widgets.Px(10)
	status := deviceStatusLocked(app)
	statusLeft := screenWidth - widgets.Px(widgets.ListMargin) - app.Fonts.Small.Width(status)
	widgets.Text(app.Fonts.Small, widgets.DarkGray, image.Pt(statusLeft, noteTop), status)
	widgets.Text(app.Fonts.Small, widgets.DarkGray, image.Pt(titleLeft, noteTop), widgets.Ellipsize(app.Fonts.Small, header.Note, statusLeft-widgets.Px(32)-titleLeft))
	if header.OnTitleTap != nil {
		app.Targets.Tap(image.Rect(titleLeft, 0, statusLeft, bar), header.OnTitleTap)
	}
	pocketbook.Fill(image.Rect(0, bar-widgets.Px(3), screenWidth, bar), widgets.Black)
}

// drawRowLocked draws row in rect: its cover, with a badge if it is read, its title, subtitle,
// progress bar and Delete button, and makes them targets.
func drawRowLocked(app *state.State, row Row, rect image.Rectangle) {
	margin := widgets.Px(widgets.ListMargin)
	top, rowHeight := rect.Min.Y, rect.Dy()
	box := image.Rect(margin, top+(rowHeight-widgets.Px(covers.Height))/2, margin+widgets.Px(covers.Width), top+(rowHeight+widgets.Px(covers.Height))/2)
	row.Cover.Shown = true
	covers.DrawLocked(app, box, row.Cover)
	if row.Read {
		badge(app.Fonts, box, "READ")
	}
	if row.ShowCover != nil {
		app.Targets.Add(widgets.Target{Rect: image.Rect(0, top, box.Max.X+widgets.Px(16), top+rowHeight), Tap: row.ShowCover, Hold: row.Hold})
	}
	left := box.Max.X + widgets.Px(32)
	right := rect.Max.X - margin
	if row.Delete != nil {
		deleteButton := image.Rect(rect.Max.X-margin-widgets.Px(200), top+(rowHeight-widgets.Px(90))/2, rect.Max.X-margin, top+(rowHeight+widgets.Px(90))/2)
		widgets.Button(app.Fonts.Small, deleteButton, "Delete", true)
		app.Targets.Tap(deleteButton, row.Delete)
		right = deleteButton.Min.X - widgets.Px(32)
	}
	gap := widgets.Px(14)
	textHeight := app.Fonts.Bold.Size + gap + app.Fonts.Small.Size
	if row.Progress > 0 {
		textHeight += widgets.Px(progressGap + progressHeight)
	}
	textTop := top + (rowHeight-textHeight)/2
	widgets.Text(app.Fonts.Bold, widgets.Black, image.Pt(left, textTop), widgets.Ellipsize(app.Fonts.Bold, widgets.Clean(row.Title), right-left))
	subtitleTop := textTop + app.Fonts.Bold.Size + gap
	widgets.Text(app.Fonts.Small, widgets.DarkGray, image.Pt(left, subtitleTop), widgets.Ellipsize(app.Fonts.Small, row.Subtitle, right-left))
	if row.Progress > 0 {
		drawProgressBar(left, right, subtitleTop+app.Fonts.Small.Size+widgets.Px(progressGap), row.Progress)
	}
	app.Targets.Add(widgets.Target{Rect: rect, Tap: row.Open, Hold: row.Hold})
}

// drawProgressBar draws a bar from left, at most to right, filled to progress, from 0 to 1.
func drawProgressBar(left, right, top int, progress float64) {
	track := image.Rect(left, top, left+min(right-left, widgets.Px(progressWidth)), top+widgets.Px(progressHeight))
	pocketbook.Fill(track, widgets.LightGray)
	filled := int(float64(track.Dx()) * min(progress, 1))
	pocketbook.Fill(image.Rect(track.Min.X, track.Min.Y, track.Min.X+filled, track.Max.Y), widgets.DarkGray)
}

// DrawFooterLocked draws the footer of page (from 0) of pages: the Prev and Next buttons and
// the page number. The left and right thirds of the footer turn the page, not only the
// buttons. onPageLabelTap, when set, makes the page number a target too.
func DrawFooterLocked(app *state.State, page, pages int, onPageLabelTap func()) {
	screenRect := pocketbook.Screen()
	screenWidth, screenHeight := screenRect.Dx(), screenRect.Dy()
	margin, bar := widgets.Px(widgets.ListMargin), widgets.Px(widgets.BarHeight)
	sideWidth := screenWidth / widgets.SideDivisor
	footerTop := screenHeight - bar
	pocketbook.Fill(image.Rect(0, footerTop, screenWidth, footerTop+widgets.Px(3)), widgets.Black)
	buttonTop := footerTop + (bar-widgets.Px(widgets.ButtonHeight))/2
	widgets.Button(app.Fonts.Bold, image.Rect(margin, buttonTop, margin+widgets.Px(300), buttonTop+widgets.Px(widgets.ButtonHeight)), "‹ Prev", page > 0)
	widgets.Button(app.Fonts.Bold, image.Rect(screenWidth-margin-widgets.Px(300), buttonTop, screenWidth-margin, buttonTop+widgets.Px(widgets.ButtonHeight)), "Next ›", page < pages-1)
	app.Targets.Add(widgets.Target{Rect: image.Rect(0, footerTop, sideWidth, screenHeight), Command: widgets.Previous})
	app.Targets.Add(widgets.Target{Rect: image.Rect(screenWidth-sideWidth, footerTop, screenWidth, screenHeight), Command: widgets.Next})
	pageLabel := fmt.Sprintf("Page %d of %d", page+1, pages)
	if onPageLabelTap != nil {
		pageLabel += " · A–Z"
		// It opens an overlay, which the tap must not close.
		app.Targets.Add(widgets.Target{Rect: image.Rect(sideWidth, footerTop, screenWidth-sideWidth, screenHeight), Tap: onPageLabelTap, KeepsOverlays: true})
	}
	footer := image.Rect(0, footerTop, screenWidth, screenHeight)
	widgets.Text(app.Fonts.Body, widgets.Black, image.Pt((screenWidth-app.Fonts.Body.Width(pageLabel))/2, app.Fonts.Body.CenterTop(footer)), pageLabel)
}

// badge draws label in white on black in the bottom right corner of box.
func badge(fonts widgets.Fonts, box image.Rectangle, label string) {
	padding := widgets.Px(8)
	tag := image.Rect(box.Max.X-fonts.Small.Width(label)-2*padding, box.Max.Y-fonts.Small.Size-2*padding, box.Max.X, box.Max.Y)
	pocketbook.Fill(tag, widgets.Black)
	widgets.Text(fonts.Small, widgets.White, image.Pt(tag.Min.X+padding, fonts.Small.CenterTop(tag)), label)
}

// deviceStatusLocked is the free space, and the time and the battery as the firmware's status
// bar would show them.
func deviceStatusLocked(app *state.State) string {
	status := app.Options.Now().Format("15:04")
	if percent, charging, ok := pocketbook.Battery(); ok {
		status += fmt.Sprintf(" · %d%%", percent)
		if charging {
			status += "+"
		}
	}
	if free, ok := app.Options.FreeSpace(app.ServerDir); ok && app.ServerDir != "" {
		status = widgets.FormatBytes(free) + " free · " + status
	}
	return status
}
