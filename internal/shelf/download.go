package shelf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/ebook/comicarchive"
	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// fetchWorkers is how many pages are fetched at once: more only slows the server (Komga) down.
const fetchWorkers = 3

// fetchedQueue is how many fetched pages may wait for a preparer: enough to keep the
// fetchers busy, few enough not to hold many pages in memory.
const fetchedQueue = 2

// prepareWorkers is how many pages are prepared at once: one per core, at least 2 to keep the
// CPU busy while a page is being written, at most 8 as each holds a decoded page of 20 to 40 MB.
var prepareWorkers = min(max(runtime.GOMAXPROCS(0), 2), 8)

// fetchedPage is a page image as the server sent it.
type fetchedPage struct {
	number int
	data   []byte
}

// Download fetches book from server into its folder, prepared for a width×height screen,
// and saves its book.json last. The pages already on disk, from an interrupted download, are
// kept. A book of page images is fetched page by page, a CBZ whole and then taken apart, and
// an EPUB whole, to be laid out while it is read.
//
// progress receives how far the download is: the bytes of the book's file fetched, then the
// number of leading pages on disk. It is called from a goroutine of its own, so that a slow
// progress never holds up the workers, and never after Download returns.
func (store Store) Download(ctx context.Context, server servers.Server, book Book, width, height int, progress func(DownloadProgress)) error {
	if book.Format == servers.PageImages && book.Pages <= 0 {
		return errors.New("the book has no pages (not analyzed by the server yet?)")
	}
	if err := os.MkdirAll(store.bookDir(book.ID), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reports := startReporter(progress)
	defer reports.stop()

	if book.Format == servers.Reflowable {
		return store.downloadReflowable(ctx, server, book, width, height, reports)
	}
	pages, closePages, err := store.openPages(ctx, server, &book, reports)
	if err != nil {
		return err
	}
	defer closePages()
	if err := store.downloadPages(ctx, cancel, pages, book, width, height, reports); err != nil {
		return err
	}
	if err := store.Save(book); err != nil {
		return err
	}
	if book.Format == servers.ComicArchive {
		// The pages are prepared: the archive only takes space now.
		if err := os.Remove(filepath.Join(store.bookDir(book.ID), archiveFile)); err != nil {
			store.logf("removing the archive of %s: %v", book.ID, err)
		}
	}
	return nil
}

// openPages returns where the pages of book come from: the server, one request a page, or
// for a ComicArchive the book's CBZ, fetched whole first. Then book.Pages is set from it.
// closePages releases them; it is a no-op on an error.
func (store Store) openPages(ctx context.Context, server servers.Server, book *Book, reports *reporter) (pages servers.Pages, closePages func(), err error) {
	if book.Format != servers.ComicArchive {
		pages, err = server.OpenBook(ctx, book.ID, book.Pages)
		return pages, func() {}, err
	}
	archivePath := filepath.Join(store.bookDir(book.ID), archiveFile)
	if err := fetchFile(ctx, server, book.ID, archivePath, reports); err != nil {
		return nil, func() {}, err
	}
	comic, err := comicarchive.Open(archivePath)
	if err != nil {
		return nil, func() {}, err
	}
	if book.Pages = comic.PageCount(); book.Pages == 0 {
		comic.Close()
		return nil, func() {}, errors.New("the archive has no page images")
	}
	return archivePages{comic: comic}, func() { comic.Close() }, nil
}

// downloadPages fetches and prepares every page of book not on disk yet, fetchWorkers and
// prepareWorkers at a time. cancel stops the workers after the first error.
func (store Store) downloadPages(ctx context.Context, cancel context.CancelFunc, pages servers.Pages, book Book, width, height int, reports *reporter) error {
	run := newDownloadRun(ctx, cancel, store, pages, book, width, height, reports)
	var fetchers, preparers sync.WaitGroup
	for range min(fetchWorkers, book.Pages) {
		fetchers.Go(run.fetchLoop)
	}
	for range min(prepareWorkers, book.Pages) {
		preparers.Go(run.prepareLoop)
	}
	fetchers.Wait()
	close(run.fetched)
	preparers.Wait()

	// Every worker has stopped: the fields that mu guards can be read without it.
	if run.firstErr != nil {
		return run.firstErr
	}
	if run.done < book.Pages {
		return ctx.Err() // cancelled between two pages
	}
	if err := ctx.Err(); err != nil {
		return err // cancelled after the last page
	}
	run.logStats()
	return nil
}

// downloadRun is the state of one Download, shared by its workers.
type downloadRun struct {
	ctx    context.Context
	cancel context.CancelFunc // stops the workers after an error
	store  Store
	pages  servers.Pages // fetches the pages from the server
	book   Book
	width  int
	height int

	start    time.Time     // when the download started, for the log
	startCPU time.Duration // the CPU time the process had used then
	times    stageTimes

	pageNumbers chan int         // the page numbers left to fetch, in order
	fetched     chan fetchedPage // from the fetchers to the preparers
	progress    *reporter        // receives the number of pages done

	mu       sync.Mutex // guards the fields below
	firstErr error      // the first error of a worker, which stops the download
	// ready[number] is set once page number is on disk. Index 0 is unused, and the last one,
	// past the last page, is never set: finishPage can always look at ready[done+1].
	ready []bool
	done  int // pages 1 to done are all on disk
}

// newDownloadRun returns the state of a download of book's pages, every page number queued
// for the fetchers.
func newDownloadRun(ctx context.Context, cancel context.CancelFunc, store Store, pages servers.Pages, book Book, width, height int, progress *reporter) *downloadRun {
	run := &downloadRun{
		ctx:         ctx,
		cancel:      cancel,
		store:       store,
		pages:       pages,
		book:        book,
		width:       width,
		height:      height,
		start:       time.Now(),
		startCPU:    cpuTime(),
		pageNumbers: make(chan int, book.Pages),
		fetched:     make(chan fetchedPage, fetchedQueue),
		progress:    progress,
		ready:       make([]bool, book.Pages+2),
	}
	for number := range book.Pages {
		run.pageNumbers <- number + 1
	}
	close(run.pageNumbers)
	return run
}

// fetchLoop is a fetcher: it takes the next page number and fetches that page, unless it is
// already on disk, until no page is left or the download stops.
func (run *downloadRun) fetchLoop() {
	for number := range run.pageNumbers {
		if run.ctx.Err() != nil {
			return
		}
		if run.store.pageOnDisk(run.book.ID, number) {
			run.finishPage(number, nil)
			continue
		}
		data, err := run.fetchPage(number)
		if err != nil {
			run.finishPage(number, err)
			continue
		}
		select {
		case run.fetched <- fetchedPage{number: number, data: data}:
		case <-run.ctx.Done():
		}
	}
}

// prepareLoop is a preparer: it prepares and writes the fetched pages until the fetchers are
// done.
func (run *downloadRun) prepareLoop() {
	for received := range run.fetched {
		if run.ctx.Err() != nil {
			// The download stopped: only drain the channel, so that no fetcher stays blocked.
			continue
		}
		run.finishPage(received.number, run.preparePage(received))
	}
}

// finishPage records the end of page number's work: on disk, or failed with err. The first
// error stops the download.
func (run *downloadRun) finishPage(number int, err error) {
	run.mu.Lock()
	defer run.mu.Unlock()
	if err != nil {
		if run.firstErr == nil {
			run.firstErr = err
			run.cancel() // stop the other workers
		}
		return
	}
	run.ready[number] = true
	if !run.ready[run.done+1] {
		return // an earlier page is still missing: the readable pages didn't change
	}
	for run.ready[run.done+1] {
		run.done++
	}
	run.progress.report(DownloadProgress{Done: run.done, Pages: run.book.Pages})
}

// fetchPage fetches page number from the server.
func (run *downloadRun) fetchPage(number int) ([]byte, error) {
	start := time.Now()
	span := timing.Start(run.ctx, "fetch page")
	data, err := run.pages.Page(run.ctx, number)
	span.End()
	if err != nil {
		return nil, err
	}
	run.times.fetch.Add(int64(time.Since(start)))
	return data, nil
}

// preparePage prepares a fetched page for the screen and writes it.
func (run *downloadRun) preparePage(fetched fetchedPage) error {
	span := timing.Start(run.ctx, "prepare page")
	packed, err := pageimage.Prepare(fetched.data, run.width, run.height, &run.times.prepare)
	span.End()
	if err != nil {
		return fmt.Errorf("page %d: %w", fetched.number, err)
	}
	defer timing.Start(run.ctx, "write page").End()
	return writeFileAtomic(run.store.pagePath(run.book.ID, fetched.number, pg4Extension), packed)
}

// logStats logs the time the download took, in all and per page and stage.
func (run *downloadRun) logStats() {
	pages := run.book.Pages
	fetchPerPage := run.times.fetch.Load() / int64(time.Millisecond) / int64(pages)
	// A CPU time well under cores × the time taken means that something else had the CPU.
	run.store.logf("download %s: %d pages in %v (CPU %v, %d cores); per page: fetch %d ms, %s",
		run.book.ID, pages, time.Since(run.start).Round(time.Millisecond),
		(cpuTime() - run.startCPU).Round(time.Millisecond), runtime.GOMAXPROCS(0),
		fetchPerPage, run.times.prepare.PerPage(pages))
}

// stageTimes adds up, across workers, the time Download spends in each stage.
type stageTimes struct {
	fetch   atomic.Int64 // in nanoseconds
	prepare pageimage.Times
}
