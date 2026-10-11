package reading

import (
	"image"
	"net/url"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// popupBlockLimit is how many blocks of a part of the book that is no chapter the popup shows,
// from the link's target: a notes file may hold hundreds.
const popupBlockLimit = 30

// noBrowser is what the popup of a link to the web or an email address says.
const noBrowser = "Not opened: no browser."

// PopupPage is the page of the popup that a link opens over the text, instead of jumping: its
// height and its top and bottom margins, in pixels. Its width, side margins and font size are
// the book's, so that a note reads as the text it belongs to.
type PopupPage struct {
	Height, MarginTop, MarginBottom int
}

// settings returns how the popup's text is laid out, from the settings of the book's: the
// same font size and side margins, on the popup's page, notes shown.
func (popup PopupPage) settings(book layout.Settings) layout.Settings {
	book.Height = popup.Height
	book.Margins.Min.Y, book.Margins.Max.Y = popup.MarginTop, popup.MarginBottom
	book.HideFootnotes = false
	return book
}

// Follow returns where a link to target leads. A footnote, a part of the book that is no
// chapter, or a URL is shown in a popup: Follow returns its pages, laid out on popup. Else it
// returns no pages and the location to jump to.
func (text *TextBook) Follow(target string, popup PopupPage) ([]layout.Page, shelf.Location, error) {
	renderer := text.renderer
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	if renderer.book == nil {
		return nil, shelf.Location{}, errClosed
	}
	doc, location, err := renderer.destinationLocked(target)
	if err != nil || doc == nil {
		return nil, location, err
	}
	pages, err := layout.Paginate(doc, popup.settings(renderer.settings), renderer.faces, renderer.book.ImageSize)
	return pages, location, err
}

// destinationLocked returns where a link to target leads: a document for the popup, or a
// location in a chapter. A target is a footnote when blocks at its fragment belong to the
// note of that id.
func (renderer *renderer) destinationLocked(target string) (*document.Document, shelf.Location, error) {
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
		return linkDocument(target), shelf.Location{}, nil // http(s) or mailto: epub keeps no other
	}
	itemPath, fragment, _ := strings.Cut(target, "#")
	chapter := renderer.book.ChapterOfPath(itemPath)
	var doc *document.Document
	var err error
	if chapter >= 0 {
		doc, err = renderer.book.Chapter(chapter)
	} else {
		doc, err = renderer.book.Document(itemPath)
	}
	if err != nil {
		return nil, shelf.Location{}, err
	}
	offset := doc.IDs[fragment] // 0, the start, if unknown
	if notes := noteBlocks(doc.Blocks, fragment); len(notes) > 0 {
		return popupDocument(doc, notes), shelf.Location{}, nil
	}
	if chapter < 0 {
		return popupDocument(doc, blocksFrom(doc.Blocks, offset)), shelf.Location{}, nil
	}
	return nil, shelf.Location{Chapter: chapter, Offset: offset}, nil
}

// noteBlocks returns the blocks of the footnote whose id is fragment, none if it is no note's.
func noteBlocks(blocks []document.Block, fragment string) []document.Block {
	var notes []document.Block
	for _, block := range blocks {
		if fragment != "" && block.Footnote == fragment {
			notes = append(notes, block)
		}
	}
	return notes
}

// blocksFrom returns popupBlockLimit blocks at most from the one that holds offset.
func blocksFrom(blocks []document.Block, offset int) []document.Block {
	first := 0
	for index, block := range blocks {
		if block.Start <= offset {
			first = index
		}
	}
	return blocks[first:min(len(blocks), first+popupBlockLimit)]
}

// popupDocument returns a document of blocks of doc, with its language and fonts, on one page.
func popupDocument(doc *document.Document, blocks []document.Block) *document.Document {
	popup := &document.Document{Language: doc.Language, Fonts: doc.Fonts}
	for _, block := range blocks {
		block.PageBreak = false
		popup.Add(block)
	}
	return popup
}

// linkDocument returns the popup's document for a link out of the book: its URL, which may
// break after its slashes, and why it was not opened.
func linkDocument(target string) *document.Document {
	doc := &document.Document{}
	doc.Add(document.Block{Align: document.AlignLeft, Runs: []document.Run{{Text: strings.ReplaceAll(target, "/", "/\u200b"), Style: document.Bold}}})
	doc.Add(document.Block{Align: document.AlignLeft, MarginTop: 50, Runs: []document.Run{{Text: noBrowser, Style: document.Italic}}})
	return doc
}

// MessagePages returns the pages of a popup that says message, laid out on popup.
func (text *TextBook) MessagePages(message string, popup PopupPage) ([]layout.Page, error) {
	renderer := text.renderer
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	if renderer.book == nil {
		return nil, errClosed // its fonts too
	}
	doc := &document.Document{}
	doc.Add(document.Block{Align: document.AlignLeft, Runs: []document.Run{{Text: message, Style: document.Italic}}})
	return layout.Paginate(doc, popup.settings(renderer.settings), renderer.faces, nil)
}

// DrawPopupPage draws page, a page of a popup, on a white canvas of the book's width, height
// high.
func (text *TextBook) DrawPopupPage(page layout.Page, height int) (*image.Gray, error) {
	renderer := text.renderer
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	if renderer.book == nil {
		return nil, errClosed
	}
	return renderer.drawLocked(page, height)
}
