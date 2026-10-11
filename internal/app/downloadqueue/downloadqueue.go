// Package downloadqueue downloads books to the reader, one at a time, in the order they were
// asked for.
//
// One book at a time: each one is ready to read as soon as it can be, instead of all of them
// late, as they would be sharing the reader's network and CPU. Queueing a book while the
// queue is idle starts the downloader, a goroutine that downloads the queue's books until it
// is empty. While it runs, the reader is kept from going to sleep.
//
// The queue lives in state.State, under its mutex, so that the lists can show which books are
// downloading or queued.
package downloadqueue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/app/catalog"
	"github.com/CedricHerzog/inkshelf/internal/app/covers"
	"github.com/CedricHerzog/inkshelf/internal/app/state"
	"github.com/CedricHerzog/inkshelf/internal/app/widgets"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// BooksFromHere is how many books the hold menu's "Download n books from here" queues: the
// book held and the next unread ones.
const BooksFromHere = 3

// progressRepaint is how often the progress of a download is redrawn.
const progressRepaint = 2 * time.Second

// awakeFor is how long the reader is kept from going to sleep at a time while downloading.
const awakeFor = time.Minute

// EnqueueLocked adds book to the queue, and starts the downloader if the queue was idle. Only
// the main thread queues books, as it alone starts goroutines.
func EnqueueLocked(app *state.State, book shelf.Book) {
	book.Unsynced = false
	app.Queue.Waiting = append(app.Queue.Waiting, book)
	if app.Queue.Downloading != "" {
		return // the downloader will get to it
	}
	first, ctx := nextLocked(app)
	app.Go("downloading", func() { downloader(ctx, app, first) })
}

// EnqueueIfMissingLocked queues item unless the reader has it or it is on its way already, or
// the app is offline.
func EnqueueIfMissingLocked(app *state.State, item state.BookItem) {
	if !app.Server.Online || item.Downloaded || app.Queue.OnItsWay(item.ID) {
		return
	}
	EnqueueLocked(app, item.Book)
}

// DequeueLocked takes book id out of the queue, and reports whether it waited there.
func DequeueLocked(app *state.State, id string) bool {
	index := slices.IndexFunc(app.Queue.Waiting, func(book shelf.Book) bool { return book.ID == id })
	if index < 0 {
		return false
	}
	app.Queue.Waiting = slices.Delete(app.Queue.Waiting, index, index+1)
	return true
}

// FromHereLocked returns the indexes in the book list of the books that "Download n books
// from here" queues from book bookIndex: that book and the next unread ones, BooksFromHere in
// all, leaving out those the reader has or that are on their way.
func FromHereLocked(app *state.State, bookIndex int) []int {
	var indexes []int
	for offset, item := range app.Books.Items[bookIndex:] {
		if len(indexes) == BooksFromHere {
			break
		}
		if item.Downloaded || app.Queue.OnItsWay(item.ID) || (offset > 0 && item.Completed) {
			continue
		}
		indexes = append(indexes, bookIndex+offset)
	}
	return indexes
}

// nextLocked takes the queue's first book and makes it the one downloading. It returns a nil
// context when the queue is empty.
func nextLocked(app *state.State) (shelf.Book, context.Context) {
	app.Queue.Downloading = ""
	app.Queue.Progress = shelf.DownloadProgress{}
	app.Queue.Cancel = nil
	if len(app.Queue.Waiting) == 0 {
		return shelf.Book{}, nil
	}
	book := app.Queue.Waiting[0]
	app.Queue.Waiting = app.Queue.Waiting[1:]
	ctx, cancel := context.WithCancel(app.Ctx)
	app.Queue.Downloading = book.ID
	app.Queue.Cancel = cancel
	return book, ctx
}

// downloader downloads book, then the queue's books in turn until it is empty.
func downloader(ctx context.Context, app *state.State, book shelf.Book) {
	defer keepAwake()()
	size := pocketbook.ScreenSize()
	for ctx != nil {
		taskContext, endTask := timing.Task(ctx, "download")
		span := timing.Start(taskContext, "download")
		err := app.Store.Download(taskContext, app.Server.Client, book, size.X, size.Y, func(progress shelf.DownloadProgress) {
			app.Update(false, func() bool {
				return noteProgressLocked(app, progress)
			})
		})
		duration := span.End()
		endTask()
		if app.Ctx.Err() != nil {
			return
		}
		log.Printf("download %s in %v: %v", book.ID, duration.Round(time.Second), err)
		if err == nil {
			covers.SaveForOffline(app, book)
		}
		app.Update(false, func() bool {
			app.Queue.Cancel()
			recordLocked(app, book, err)
			book, ctx = nextLocked(app)
			reading := app.Screen == state.ReaderScreen || app.Screen == state.ContentsScreen
			return !reading || app.Reader.AtEnd
		})
	}
}

// noteProgressLocked records how far the download is, and reports whether to redraw it.
func noteProgressLocked(app *state.State, progress shelf.DownloadProgress) bool {
	app.Queue.Progress = progress
	// Each redraw refreshes the panel and takes the CPU from the download: show the first
	// page, then the progress every progressRepaint, and only on the screens that show it.
	if app.Screen != state.BooksScreen && !app.Reader.AtEnd {
		return false
	}
	if progress.Done != 1 && time.Since(app.Queue.LastRepaint) < progressRepaint {
		return false
	}
	app.Queue.LastRepaint = time.Now()
	return true
}

// recordLocked records the end of book's download, err telling how it went.
func recordLocked(app *state.State, book shelf.Book, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		app.List.Notice = "Cancelled " + book.Title
		return
	case err != nil:
		app.List.Notice = "Download failed: " + err.Error()
		return
	}
	app.List.Notice = "Downloaded " + book.Title
	catalog.NoteDownloadedLocked(app, book)
}

// keepAwake keeps the reader from going to sleep until stop is called: the firmware would
// suspend it in the middle of a download.
func keepAwake() (stop func()) {
	pocketbook.PreventSleepFor(awakeFor)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(awakeFor / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pocketbook.PreventSleepFor(awakeFor)
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// LabelLocked says how far the download of the book downloading is: "Downloading 12 of 80",
// "Downloading 45%".
func LabelLocked(app *state.State) string {
	progress := app.Queue.Progress
	switch {
	case progress.Preparing:
		return "Preparing"
	case progress.Pages > 0:
		return fmt.Sprintf("Downloading %d of %d", progress.Done, progress.Pages)
	case progress.Size > 0:
		return fmt.Sprintf("Downloading %d%%", progress.Fetched*100/progress.Size)
	case progress.Fetched > 0:
		return "Downloading " + widgets.FormatBytes(progress.Fetched)
	}
	return "Downloading"
}
