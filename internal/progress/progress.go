package progress

import (
	"context"
	"errors"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/servers"
	"github.com/CedricHerzog/inkshelf/internal/shelf"
)

// requestTimeout is the longest a request sending the reading progress may take.
const requestTimeout = 10 * time.Second

// Merge returns local, the progress saved on the reader, with the server's progress remote
// when it should win, and reports whether it changed, to be saved. The server's progress
// wins when fromServer (it was just read from a server that keeps progress) and the reader
// has nothing unsent: another device may have read further, or less.
func Merge(local shelf.Book, remote servers.ReadingProgress, fromServer bool) (shelf.Book, bool) {
	if !fromServer || local.Unsynced || (local.Page == remote.Page && local.Completed == remote.Completed) {
		return local, false
	}
	local.Page = remote.Page
	local.Completed = remote.Completed
	return local, true
}

// SendReadState marks book bookID read (at its last page, pages) or unread on the server,
// and returns the page it is now at. A server that keeps no progress is no error.
func SendReadState(parent context.Context, server servers.Server, bookID string, pages int, read bool) (int, error) {
	ctx, cancel := context.WithTimeout(parent, requestTimeout)
	defer cancel()
	var page int
	var err error
	if read {
		page = pages
		err = server.SetProgress(ctx, bookID, pages, true)
	} else {
		err = server.ClearProgress(ctx, bookID)
	}
	if errors.Is(err, errors.ErrUnsupported) { // the server keeps no progress: the reader's is enough
		err = nil
	}
	return page, err
}

// sendProgress sends book's progress to server.
func sendProgress(server servers.Server, book shelf.Book) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	err := server.SetProgress(ctx, book.ID, book.Page, book.Completed)
	if errors.Is(err, errors.ErrUnsupported) { // the server keeps no progress: the reader's is enough
		return nil
	}
	return err
}
