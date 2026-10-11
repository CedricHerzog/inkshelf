// Package contents is the screen of a Reflowable book's navigation, opened from the reader's
// toolbar: its table of contents, and its printed pages and landmarks when the EPUB has them,
// each on a tab. Tapping an entry jumps there; the reader's toolbar then offers to return to
// the page left.
package contents

import (
	"cmp"
	"fmt"
	"image"

	"github.com/CedricHerzog/inkshelf/internal/app/openbook"
	"github.com/CedricHerzog/inkshelf/internal/app/screens/list"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/reading"
)

// Screen is the contents screen.
type Screen struct {
	app *state.State
}

// New returns the contents screen of app.
func New(app *state.State) *Screen { return &Screen{app: app} }

// tab is a list of the contents screen: the table of contents, the printed pages or the
// landmarks.
type tab struct {
	label   string
	entries []epub.TOCEntry
	columns int  // 1: a line per entry, indented by its level; more: a grid of short labels
	marks   bool // the entry being read is marked; the landmarks are no sequence, so they are not
}

// The sizes of the contents screen, in InkPad 3 pixels: a line, the indent of a level, and the
// grid of printed pages.
const (
	rowHeight   = 132
	levelIndent = 60
	gridColumns = 5
	cellGap     = 24
)

// tabsOf returns the lists of the book's navigation: its table of contents, and its printed
// pages and landmarks if it has them.
func tabsOf(text *state.TextReader) []tab {
	index := text.Book.Index
	tabs := []tab{{label: "Contents", entries: text.Book.TableOfContents(), columns: 1, marks: true}}
	if len(index.PageList) > 0 {
		tabs = append(tabs, tab{label: "Pages", entries: index.PageList, columns: gridColumns, marks: true})
	}
	if len(index.Landmarks) > 0 {
		tabs = append(tabs, tab{label: "Landmarks", entries: index.Landmarks, columns: 1})
	}
	return tabs
}

// rowsPerPage is the number of lines on a page of the screen with a band bandHeight high under
// its header.
func rowsPerPage(bandHeight int) int {
	return max(1, (pocketbook.Screen().Dy()-2*widgets.Px(widgets.BarHeight)-bandHeight)/widgets.Px(rowHeight))
}

// tabsHeight returns the height of the band of tabs: none when the book has a single list.
func tabsHeight(tabs []tab) int {
	if len(tabs) < 2 {
		return 0
	}
	return widgets.Px(widgets.SegmentHeight + 2*widgets.SegmentGap)
}

// Command goes back to the page on Back, and turns the list's pages on the page keys.
func (screen *Screen) Command(order widgets.Command) {
	app := screen.app
	switch order {
	case widgets.Back:
		screen.close()
	case widgets.Previous, widgets.Next:
		app.Update(false, func() bool { return screen.turnLocked(order == widgets.Next) })
	}
}

// TurnsPagesOnSwipe makes a swipe turn the list's pages.
func (screen *Screen) TurnsPagesOnSwipe() {}

// close goes back to the page left.
func (screen *Screen) close() {
	app := screen.app
	app.Update(true, func() bool {
		if app.Screen != state.ContentsScreen {
			return false
		}
		app.ShowLocked(state.ReaderScreen)
		return true
	})
}

// showTab shows list tabIndex, at the entry being read.
func (screen *Screen) showTab(tabIndex int) {
	app := screen.app
	app.Update(false, func() bool {
		if app.Screen != state.ContentsScreen || app.Contents.Tab == tabIndex {
			return false
		}
		app.Contents = state.Contents{Tab: tabIndex, Focus: true}
		return true
	})
}

// turnLocked turns the list shown one page, and reports whether there was one.
func (screen *Screen) turnLocked(forward bool) bool {
	app := screen.app
	if app.Screen != state.ContentsScreen {
		return false
	}
	tabs := tabsOf(app.Reader.Text)
	shown := tabs[min(app.Contents.Tab, len(tabs)-1)]
	perPage := rowsPerPage(tabsHeight(tabs)) * shown.columns
	switch {
	case forward && (app.Contents.Page+1)*perPage < len(shown.entries):
		app.Contents.Page++
	case !forward && app.Contents.Page > 0:
		app.Contents.Page--
	default:
		return false
	}
	return true
}

// goTo shows the page of entry entryIndex of the list shown. The book keeps the page left, for
// the toolbar's Return.
func (screen *Screen) goTo(entryIndex int) {
	app := screen.app
	app.Update(true, func() bool {
		text := app.Reader.Text
		if app.Screen != state.ContentsScreen || text == nil {
			return false
		}
		tabs := tabsOf(text)
		entries := tabs[min(app.Contents.Tab, len(tabs)-1)].entries
		if entryIndex < 0 || entryIndex >= len(entries) {
			return false
		}
		openbook.JumpTextLocked(app, reading.EntryLocation(entries[entryIndex]))
		app.Reader.Toolbar = false
		app.ShowLocked(state.ReaderScreen)
		return true
	})
	app.Syncer.Schedule(openbook.SyncDelay)
}

// DrawLocked draws the screen: a header, the tabs if the book has several lists, a page of the
// list shown with the entry being read marked, and the page buttons. It first turns to the
// page of the entry being read, when the screen or the tab just opened.
func (screen *Screen) DrawLocked() {
	app := screen.app
	text := app.Reader.Text
	tabs := tabsOf(text)
	app.Contents.Tab = min(app.Contents.Tab, len(tabs)-1)
	shown := tabs[app.Contents.Tab]
	bandHeight := tabsHeight(tabs)
	rows := rowsPerPage(bandHeight)
	perPage := rows * shown.columns
	current := -1
	if location, err := text.Book.Location(text.Book.At()); err == nil && shown.marks {
		current = reading.EntryAt(shown.entries, location)
	}
	pages := max(1, (len(shown.entries)+perPage-1)/perPage)
	if app.Contents.Focus && current >= 0 {
		app.Contents.Page = current / perPage
	}
	app.Contents.Focus = false
	app.Contents.Page = min(app.Contents.Page, pages-1)

	screenRect := pocketbook.Screen()
	screenWidth, screenHeight := screenRect.Dx(), screenRect.Dy()
	margin, bar := widgets.Px(widgets.ListMargin), widgets.Px(widgets.BarHeight)
	list.DrawHeaderLocked(app, list.Header{
		Title:     app.Reader.OpenBook.Title,
		BackLabel: "‹ Back",
		Note:      widgets.Counted(len(shown.entries), "entry", "entries"),
	})
	if bandHeight > 0 {
		band := image.Rect(0, bar, screenWidth, bar+bandHeight)
		labels := make([]string, len(tabs))
		for index, each := range tabs {
			labels[index] = each.label
		}
		segmentsRect := image.Rect(margin, band.Min.Y+widgets.Px(widgets.SegmentGap), band.Max.X-margin, band.Min.Y+widgets.Px(widgets.SegmentGap+widgets.SegmentHeight))
		widgets.Segments(app.Fonts, &app.Targets, segmentsRect, labels, app.Contents.Tab, screen.showTab)
		widgets.SegmentsRule(band)
	}

	listArea := image.Rect(margin, bar+bandHeight, screenWidth-margin, screenHeight-bar)
	lineHeight := listArea.Dy() / rows
	cellWidth := (listArea.Dx() - (shown.columns-1)*widgets.Px(cellGap)) / shown.columns
	first := app.Contents.Page * perPage
	for position := range min(perPage, len(shown.entries)-first) {
		entryIndex := first + position
		top := listArea.Min.Y + position/shown.columns*lineHeight
		left := listArea.Min.X + position%shown.columns*(cellWidth+widgets.Px(cellGap))
		cell := image.Rect(left, top, left+cellWidth, top+lineHeight)
		if shown.columns == 1 {
			entry := shown.entries[entryIndex]
			screen.drawLine(entry, text.Book.Percent(reading.EntryLocation(entry)), cell, position > 0, entryIndex == current)
		} else {
			screen.drawCell(shown.entries[entryIndex], cell.Inset(widgets.Px(cellGap)/2), entryIndex == current)
		}
		app.Targets.Tap(cell, func() { screen.goTo(entryIndex) })
	}
	list.DrawFooterLocked(app, app.Contents.Page, pages, nil)
}

// drawLine draws entry as a line of the table of contents in rect, indented by its level, with
// percent, how far into the book it is. The entry being read is in bold, a bar before it.
func (screen *Screen) drawLine(entry epub.TOCEntry, percent int, rect image.Rectangle, ruled, current bool) {
	fonts := screen.app.Fonts
	if ruled {
		pocketbook.Fill(image.Rect(rect.Min.X, rect.Min.Y-widgets.Px(1), rect.Max.X, rect.Min.Y+widgets.Px(1)), widgets.LightGray)
	}
	font := fonts.Body
	if current {
		font = fonts.Bold
		pocketbook.Fill(image.Rect(rect.Min.X, rect.Min.Y+widgets.Px(24), rect.Min.X+widgets.Px(12), rect.Max.Y-widgets.Px(24)), widgets.Black)
	}
	percentLabel := fmt.Sprintf("%d%%", percent)
	percentLeft := rect.Max.X - fonts.Small.Width(percentLabel)
	left := rect.Min.X + widgets.Px(40) + min(entry.Level, 4)*widgets.Px(levelIndent)
	title := widgets.Ellipsize(font, widgets.Clean(cmp.Or(entry.Title, "—")), percentLeft-widgets.Px(32)-left)
	widgets.Text(font, widgets.Black, image.Pt(left, font.CenterTop(rect)), title)
	widgets.Text(fonts.Small, widgets.DarkGray, image.Pt(percentLeft, fonts.Small.CenterTop(rect)), percentLabel)
}

// drawCell draws entry as a cell of the grid of printed pages, inverted if it is the page
// being read.
func (screen *Screen) drawCell(entry epub.TOCEntry, cell image.Rectangle, current bool) {
	fonts := screen.app.Fonts
	labelColor := widgets.Black
	if current {
		pocketbook.Fill(cell, widgets.Black)
		labelColor = widgets.White
	} else {
		widgets.Frame(cell, widgets.Px(3), widgets.Black)
	}
	label := widgets.Ellipsize(fonts.Body, widgets.Clean(cmp.Or(entry.Title, "—")), cell.Dx()-widgets.Px(24))
	widgets.Text(fonts.Body, labelColor, image.Pt(cell.Min.X+(cell.Dx()-fonts.Body.Width(label))/2, fonts.Body.CenterTop(cell)), label)
}
