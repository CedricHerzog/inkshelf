package reading

import (
	"fmt"
	"image"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// PagePosition is a page of a text book as laid out now: a chapter (from 0) and a page in it
// (from 0). It changes with the font size; a shelf.Location does not.
type PagePosition struct{ Chapter, Page int }

// TextBook is an open text book: its index, the page shown, and the pages drawn around it.
// Its fields are guarded by the app's mutex; its renderer has a lock of its own, taken after
// the app's and never before it (see the package doc).
type TextBook struct {
	// Index is the book's chapters, table of contents and navigation. Read-only once open.
	Index    epub.Index
	at       PagePosition                 // the page shown
	drawn    map[PagePosition]*image.Gray // the page shown and its neighbours, drawn; nil once closed
	renderer *renderer
	returnTo *shelf.Location // the page left by the last jump, for the toolbar's Return; nil if none
}

// Open opens the text book of the shelf, lays it out with settings and places it at its
// saved location, or at its first chapter with text if it has none.
func Open(store shelf.Store, book shelf.Book, settings layout.Settings) (*TextBook, error) {
	opened, err := epub.Open(store.EPUBPath(book.ID))
	if err != nil {
		return nil, err
	}
	index, err := store.LoadIndex(book.ID, opened)
	if err != nil {
		opened.Close()
		return nil, err
	}
	text := &TextBook{
		Index: index,
		drawn: map[PagePosition]*image.Gray{},
		renderer: &renderer{
			book:     opened,
			settings: settings,
			faces:    layout.NewFaces(fonts.Serif),
			chapters: map[int][]layout.Page{},
		},
	}
	location := shelf.Location{Chapter: text.firstChapter(), Offset: 0}
	if book.Location != nil && book.Page > 0 {
		location = *book.Location
	}
	if err := text.GoTo(location); err != nil {
		text.Close()
		return nil, err
	}
	return text, nil
}

// At returns the page shown.
func (text *TextBook) At() PagePosition { return text.at }

// firstChapter returns the first chapter that has something to show.
func (text *TextBook) firstChapter() int {
	for chapter, info := range text.Index.Chapters {
		if info.Chars > 0 {
			return chapter
		}
	}
	return 0
}

// GoTo shows the page of location.
func (text *TextBook) GoTo(location shelf.Location) error {
	chapter := min(max(location.Chapter, 0), len(text.Index.Chapters)-1)
	pages, err := text.renderer.pages(chapter)
	if err != nil {
		return err
	}
	text.at = PagePosition{Chapter: chapter, Page: layout.PageAt(pages, location.Offset)}
	return nil
}

// Jump shows the page of location, and keeps the page shown until then for Return.
func (text *TextBook) Jump(location shelf.Location) error {
	left, err := text.Location(text.at)
	if err == nil {
		text.returnTo = &left
	}
	return text.GoTo(location)
}

// CanReturn reports whether a jump left a page to go back to.
func (text *TextBook) CanReturn() bool { return text.returnTo != nil }

// Return goes back to the page left by the last jump, if any.
func (text *TextBook) Return() error {
	if text.returnTo == nil {
		return nil
	}
	location := *text.returnTo
	text.returnTo = nil
	return text.GoTo(location)
}

// Close closes the book; a preload drawing a page then finds it closed.
func (text *TextBook) Close() {
	text.renderer.close()
	text.drawn = nil
}

// Next returns the page after at, and false if at is the last page of the book. It lays out
// the next chapter if needed. It may be called without the app's mutex.
func (text *TextBook) Next(at PagePosition) (PagePosition, bool, error) {
	pages, err := text.renderer.pages(at.Chapter)
	if err != nil {
		return PagePosition{}, false, err
	}
	if at.Page+1 < len(pages) {
		return PagePosition{Chapter: at.Chapter, Page: at.Page + 1}, true, nil
	}
	for chapter := at.Chapter + 1; chapter < len(text.Index.Chapters); chapter++ {
		if text.Index.Chapters[chapter].Chars > 0 {
			return PagePosition{Chapter: chapter, Page: 0}, true, nil
		}
	}
	return PagePosition{}, false, nil
}

// Previous returns the page before at, and false if at is the first page of the book. It lays
// out the previous chapter if needed, to find its last page. It may be called without the
// app's mutex.
func (text *TextBook) Previous(at PagePosition) (PagePosition, bool, error) {
	if at.Page > 0 {
		return PagePosition{Chapter: at.Chapter, Page: at.Page - 1}, true, nil
	}
	for chapter := at.Chapter - 1; chapter >= 0; chapter-- {
		if text.Index.Chapters[chapter].Chars == 0 {
			continue
		}
		pages, err := text.renderer.pages(chapter)
		if err != nil {
			return PagePosition{}, false, err
		}
		return PagePosition{Chapter: chapter, Page: len(pages) - 1}, true, nil
	}
	return PagePosition{}, false, nil
}

// Turn shows the next or the previous page, and reports whether there was one.
func (text *TextBook) Turn(forward bool) (bool, error) {
	move := text.Previous
	if forward {
		move = text.Next
	}
	at, ok, err := move(text.at)
	if err != nil || !ok {
		return false, err
	}
	text.at = at
	return true, nil
}

// AtLastPage reports whether the page shown is the last of the book.
func (text *TextBook) AtLastPage() bool {
	_, more, err := text.Next(text.at)
	return err == nil && !more
}

// Location returns the location of page at: the offset of its first rune.
func (text *TextBook) Location(at PagePosition) (shelf.Location, error) {
	pages, err := text.renderer.pages(at.Chapter)
	if err != nil {
		return shelf.Location{}, err
	}
	return shelf.Location{Chapter: at.Chapter, Offset: pages[min(at.Page, len(pages)-1)].Start}, nil
}

// UpdateProgress writes the page shown into book, the book's saved progress: its location,
// its virtual page, and, at the last page, the book read. The caller marks it unsynced.
func (text *TextBook) UpdateProgress(book *shelf.Book) error {
	location, err := text.Location(text.at)
	if err != nil {
		return err
	}
	book.Location = &location
	book.Page = text.VirtualPage(location, book.Pages)
	if _, more, err := text.Next(text.at); err == nil && !more {
		book.Page = book.Pages // the last page: read
		book.Completed = true
	}
	return nil
}

// global returns location's offset from the start of the book, in runes.
func (text *TextBook) global(location shelf.Location) int {
	offset := location.Offset
	for _, chapter := range text.Index.Chapters[:location.Chapter] {
		offset += chapter.Chars
	}
	return offset
}

// VirtualPage returns the virtual page (shelf.CharsPerPage runes each) of location, from 1
// to pages.
func (text *TextBook) VirtualPage(location shelf.Location, pages int) int {
	return min(max(text.global(location)/shelf.CharsPerPage+1, 1), pages)
}

// Percent returns how far into the book location is, in percent.
func (text *TextBook) Percent(location shelf.Location) int {
	if text.Index.Total == 0 {
		return 0
	}
	return text.global(location) * 100 / text.Index.Total
}

// PageLabel returns the label of the page shown, at location: its number in its chapter and
// the chapter's page count, and how far into the book it is.
func (text *TextBook) PageLabel(location shelf.Location) string {
	pages, err := text.renderer.pages(text.at.Chapter)
	if err != nil {
		return fmt.Sprintf("%d%%", text.Percent(location))
	}
	return fmt.Sprintf("%d / %d · %d%%", text.at.Page+1, len(pages), text.Percent(location))
}

// Page returns page at, laid out.
func (text *TextBook) Page(at PagePosition) (layout.Page, error) {
	return text.renderer.page(at)
}

// Drawn returns page at if it was drawn and kept, else nil.
func (text *TextBook) Drawn(at PagePosition) *image.Gray { return text.drawn[at] }

// Draw draws page at on a white canvas of the page's size. It may be called without the app's
// mutex: it keeps nothing, KeepDrawn does.
func (text *TextBook) Draw(at PagePosition) (*image.Gray, error) {
	return text.renderer.draw(at)
}

// KeepDrawn keeps pageImage, page at drawn, unless the book closed.
func (text *TextBook) KeepDrawn(at PagePosition, pageImage *image.Gray) {
	if text.drawn != nil {
		text.drawn[at] = pageImage
	}
}

// WantsDrawn reports whether a preload that started at the page shown at should still draw
// page neighbour: the book is open, the page shown is still at, and neighbour isn't drawn.
func (text *TextBook) WantsDrawn(neighbour, at PagePosition) bool {
	return text.drawn != nil && text.drawn[neighbour] == nil && text.at == at
}

// Neighbours returns the pages around at worth drawing ahead, the next first: it is the
// likely turn. It may be called without the app's mutex.
func (text *TextBook) Neighbours(at PagePosition) []PagePosition {
	var neighbours []PagePosition
	if next, ok, err := text.Next(at); err == nil && ok {
		neighbours = append(neighbours, next)
	}
	if previous, ok, err := text.Previous(at); err == nil && ok {
		neighbours = append(neighbours, previous)
	}
	return neighbours
}

// Prune drops the pages drawn other than the page shown and its neighbours, and the layouts of
// the chapters other than its own and the two around it. It lays nothing out.
func (text *TextBook) Prune() {
	at := text.at
	for drawn := range text.drawn {
		neighbour := (drawn.Chapter == at.Chapter && drawn.Page >= at.Page-1 && drawn.Page <= at.Page+1) ||
			(drawn.Chapter == at.Chapter+1 && drawn.Page == 0) || drawn.Chapter == at.Chapter-1
		if !neighbour {
			delete(text.drawn, drawn)
		}
	}
	text.renderer.keep(at.Chapter-1, at.Chapter, at.Chapter+1)
}

// Relayout lays the book out with settings from now on (a new font size), and shows the page
// of location, usually the one shown until then.
func (text *TextBook) Relayout(settings layout.Settings, location shelf.Location) error {
	text.renderer.resize(settings)
	text.drawn = map[PagePosition]*image.Gray{}
	return text.GoTo(location)
}
