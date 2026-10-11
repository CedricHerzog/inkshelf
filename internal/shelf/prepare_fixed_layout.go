package shelf

import (
	"context"
	"fmt"
	"image"
	"os"
	"sync"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// downloadFixedLayout turns a fixed-layout EPUB (opened, at book's EPUBPath) into page images,
// as a comic: each chapter drawn whole by epub.RenderPage, then prepared for the screen. The
// pages already on disk (from an interrupted download) are kept. book.json is saved as a
// ComicArchive's, and the EPUB, which only takes space now, is removed.
func (store Store) downloadFixedLayout(ctx context.Context, opened *epub.Book, book Book, size image.Point, reports *reporter) error {
	span := timing.Start(ctx, "draw fixed-layout pages")
	book.Format, book.Pages = servers.ComicArchive, opened.Chapters()
	if book.Direction == "" {
		// The server says nothing: the EPUB's page progression, else its language's, and not
		// manga's order, which page images default to.
		book.Direction = servers.LeftToRight
		if opened.RightToLeft() {
			book.Direction = servers.RightToLeft
		}
	}
	err := store.renderPages(ctx, opened, book, size, reports)
	duration := span.End()
	if err != nil {
		return err
	}
	store.logf("download %s: fixed-layout EPUB of %d pages drawn in %v", book.ID, book.Pages, duration)
	if err := store.Save(book); err != nil {
		return err
	}
	if err := os.Remove(store.EPUBPath(book.ID)); err != nil {
		store.logf("removing the EPUB of %s: %v", book.ID, err)
	}
	return nil
}

// renderPages draws the pages of a fixed-layout book not on disk yet, prepareWorkers at a
// time, and stops at the first error.
func (store Store) renderPages(ctx context.Context, opened *epub.Book, book Book, size image.Point, reports *reporter) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	numbers := make(chan int, book.Pages)
	for number := range book.Pages {
		numbers <- number + 1
	}
	close(numbers)
	var mu sync.Mutex // guards done, and reports, which takes one caller at a time
	done := 0
	var workers sync.WaitGroup
	for range min(prepareWorkers, book.Pages) {
		workers.Go(func() {
			for number := range numbers {
				if ctx.Err() != nil {
					return
				}
				if err := store.renderPage(opened, book.ID, number, size); err != nil {
					cancel(fmt.Errorf("page %d: %w", number, err))
					return
				}
				mu.Lock()
				done++
				reports.report(DownloadProgress{Done: done, Pages: book.Pages})
				mu.Unlock()
			}
		})
	}
	workers.Wait()
	return context.Cause(ctx)
}

// renderPage draws page number of a fixed-layout book and writes it prepared for a screen of
// size, unless it is on disk already.
func (store Store) renderPage(opened *epub.Book, bookID string, number int, size image.Point) error {
	if store.pageOnDisk(bookID, number) {
		return nil
	}
	drawn, err := opened.RenderPage(number-1, size)
	if err != nil {
		return err
	}
	return writeFileAtomic(store.pagePath(bookID, number, pg4Extension), pageimage.PrepareDecoded(drawn, size.X, size.Y))
}
