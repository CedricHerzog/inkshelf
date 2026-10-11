package reading

import (
	"context"
	"errors"
	"image"
	"log"
	"slices"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/webimage"
)

// renderer lays out and draws the chapters of a book. Its methods may be called from the
// app's main thread and from its preloading goroutine: they take its lock.
type renderer struct {
	mu       sync.Mutex
	book     *epub.Book // nil once closed
	settings layout.Settings
	faces    *layout.Faces
	chapters map[int][]layout.Page // the chapters laid out, by index
}

// errClosed is the error of a renderer whose book closed.
var errClosed = errors.New("the book is closed")

// pages returns the pages of chapter, laying it out if it isn't yet.
func (renderer *renderer) pages(chapter int) ([]layout.Page, error) {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	return renderer.pagesLocked(chapter)
}

// pagesLocked is pages with renderer.mu held.
func (renderer *renderer) pagesLocked(chapter int) ([]layout.Page, error) {
	if renderer.book == nil {
		return nil, errClosed
	}
	if pages, ok := renderer.chapters[chapter]; ok {
		return pages, nil
	}
	span := timing.Start(context.Background(), "layout chapter")
	convert := timing.Start(context.Background(), "chapter document")
	doc, err := renderer.book.Chapter(chapter)
	convert.End()
	if err != nil {
		span.End()
		return nil, err
	}
	paginate := timing.Start(context.Background(), "paginate")
	pages, err := layout.Paginate(doc, renderer.settings, renderer.faces, renderer.book.ImageSize)
	paginate.End()
	if err != nil {
		span.End()
		return nil, err
	}
	log.Printf("layout chapter %d: %v, %d pages", chapter, span.End(), len(pages))
	renderer.chapters[chapter] = pages
	return pages, nil
}

// page returns page at, laid out.
func (renderer *renderer) page(at PagePosition) (layout.Page, error) {
	pages, err := renderer.pages(at.Chapter)
	if err != nil {
		return layout.Page{}, err
	}
	return pages[min(at.Page, len(pages)-1)], nil
}

// draw draws page at on a white canvas of the page's size.
func (renderer *renderer) draw(at PagePosition) (*image.Gray, error) {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	pages, err := renderer.pagesLocked(at.Chapter)
	if err != nil {
		return nil, err
	}
	span := timing.Start(context.Background(), "render page")
	canvas, err := renderer.drawLocked(pages[min(at.Page, len(pages)-1)], renderer.settings.Height)
	duration := span.End()
	if err != nil {
		return nil, err
	}
	log.Printf("render chapter %d page %d: %v", at.Chapter, at.Page, duration)
	return canvas, nil
}

// drawLocked draws page on a white canvas of the page's width, height high, with renderer.mu
// held.
func (renderer *renderer) drawLocked(page layout.Page, height int) (*image.Gray, error) {
	canvas := image.NewGray(image.Rect(0, 0, renderer.settings.Width, height))
	for index := range canvas.Pix {
		canvas.Pix[index] = 0xff // white
	}
	if err := page.Draw(canvas, renderer.faces, webimage.NewLoader(renderer.book.Resource, renderer.faces.SVGText())); err != nil {
		return nil, err
	}
	return canvas, nil
}

// keep drops the layouts of the chapters other than chapters.
func (renderer *renderer) keep(chapters ...int) {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	for chapter := range renderer.chapters {
		if !slices.Contains(chapters, chapter) {
			delete(renderer.chapters, chapter)
		}
	}
}

// resize lays out with settings from now on: the layouts are dropped.
func (renderer *renderer) resize(settings layout.Settings) {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	renderer.settings = settings
	renderer.chapters = map[int][]layout.Page{}
}

// close closes the book and its fonts.
func (renderer *renderer) close() {
	renderer.mu.Lock()
	defer renderer.mu.Unlock()
	if renderer.book == nil {
		return
	}
	renderer.faces.Close()
	if err := renderer.book.Close(); err != nil {
		log.Printf("closing the book: %v", err)
	}
	renderer.book = nil
	renderer.chapters = nil
}
