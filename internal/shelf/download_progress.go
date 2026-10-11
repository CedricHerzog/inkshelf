package shelf

import "time"

// DownloadProgress is how far a download is.
type DownloadProgress struct {
	// Fetched is the bytes of the book's file fetched so far, of Size (-1 if unknown). Both are
	// 0 for a book downloaded page by page.
	Fetched, Size int64
	// Done is the number of leading pages on disk, of Pages (0 until the file is fetched).
	Done, Pages int
	// Preparing is set once a Reflowable book's file is fetched, while it is read through.
	Preparing bool
}

// progressReports is how often a file download reports its bytes.
const progressReports = 500 * time.Millisecond

// reporter passes progress to a callback from a goroutine of its own, so that a slow callback
// never holds up the download. A value the callback hasn't taken yet is replaced by the next
// one: only the latest matters, and report never blocks. report is called from one goroutine
// at a time.
type reporter struct {
	latest chan DownloadProgress
	done   chan struct{} // closed once the callback has returned for the last time
}

// startReporter starts passing reported progress to callback.
func startReporter(callback func(DownloadProgress)) *reporter {
	progress := &reporter{latest: make(chan DownloadProgress, 1), done: make(chan struct{})}
	go func() {
		defer close(progress.done)
		for value := range progress.latest {
			callback(value)
		}
	}()
	return progress
}

// report passes value to the callback, replacing the value not taken yet, if any.
func (progress *reporter) report(value DownloadProgress) {
	select {
	case <-progress.latest:
	default:
	}
	progress.latest <- value
}

// stop waits for the callback to take the last value.
func (progress *reporter) stop() {
	close(progress.latest)
	<-progress.done
}
