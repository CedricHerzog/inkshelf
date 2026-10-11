package shelf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/ebook/comicarchive"
	"github.com/CedricHerzog/inkshelf/internal/servers"
)

// archiveFile is the file of a comic book archive, in its book's folder until its pages are
// prepared.
const archiveFile = "book.cbz"

// stallTimeout is how long a file download may receive nothing before it is given up. A large
// file has no overall time limit, but a Wi-Fi connection dropped mid-download would otherwise
// leave it waiting forever.
const stallTimeout = 30 * time.Second

// errStalled is the error of a file download that received nothing for stallTimeout.
var errStalled = errors.New("the download stalled")

// fetchFile downloads the file of book bookID to filePath through filePath.part, renamed once
// complete, unless filePath is there already (from an interrupted download).
func fetchFile(ctx context.Context, server servers.Server, bookID, filePath string, progress *reporter) error {
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	watchdog := time.AfterFunc(stallTimeout, func() { cancel(errStalled) })
	defer watchdog.Stop()

	body, size, err := server.OpenFile(ctx, bookID)
	if err != nil {
		return stalledOr(ctx, err)
	}
	defer body.Close()
	partial := filePath + ".part"
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	fetched, err := copyReporting(ctx, file, body, size, watchdog, progress)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil && size >= 0 && fetched != size {
		err = fmt.Errorf("received %d bytes of %d", fetched, size)
	}
	if err != nil {
		_ = os.Remove(partial) // best effort: the download's error is the one worth returning
		return stalledOr(ctx, err)
	}
	return os.Rename(partial, filePath)
}

// copyReporting copies body to file, resetting watchdog on each read and reporting the bytes
// copied every progressReports; it returns how many bytes it copied.
func copyReporting(ctx context.Context, file io.Writer, body io.Reader, size int64, watchdog *time.Timer, progress *reporter) (int64, error) {
	buffer := make([]byte, 64<<10)
	var fetched int64
	lastReport := time.Time{}
	for {
		count, readErr := body.Read(buffer)
		if count > 0 {
			watchdog.Reset(stallTimeout)
			if _, err := file.Write(buffer[:count]); err != nil {
				return fetched, err
			}
			fetched += int64(count)
			if time.Since(lastReport) >= progressReports {
				progress.report(DownloadProgress{Fetched: fetched, Size: size})
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			progress.report(DownloadProgress{Fetched: fetched, Size: size})
			return fetched, nil
		}
		if readErr != nil {
			return fetched, readErr
		}
		if err := ctx.Err(); err != nil {
			return fetched, err
		}
	}
}

// stalledOr returns errStalled if ctx was cancelled by the watchdog, else err.
func stalledOr(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), errStalled) {
		return errStalled
	}
	return err
}

// archivePages are the pages of a downloaded comic book archive, as a servers.Pages for the
// download's workers.
type archivePages struct{ comic *comicarchive.Archive }

// Page returns page number of the archive, as stored in it.
func (pages archivePages) Page(_ context.Context, number int) ([]byte, error) {
	return pages.comic.Page(number)
}
