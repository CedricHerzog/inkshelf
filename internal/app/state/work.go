package state

import (
	"slices"
	"sync"
	"time"
)

// Work counts the app's background work in progress, by name: the goroutines started with Go
// while they run, and the wake-ups sent to a waiting goroutine until it has handled them. The
// app never reads it. The emulator does, so that a script's screenshot waits for the work its
// last tap started (a download, covers, the next pages) instead of guessing how long it takes.
// Its methods are safe from any goroutine; it has a lock of its own, never held for long.
type Work struct {
	mu        sync.Mutex
	running   map[string]int // how many of each work are in progress
	lastEnded time.Time      // when some work last ended
}

// Begin counts one more work named name in progress, until End.
func (work *Work) Begin(name string) {
	work.mu.Lock()
	defer work.mu.Unlock()
	if work.running == nil {
		work.running = map[string]int{}
	}
	work.running[name]++
}

// End records that a work named name that Begin counted is over.
func (work *Work) End(name string) {
	work.mu.Lock()
	defer work.mu.Unlock()
	work.running[name]--
	if work.running[name] <= 0 {
		delete(work.running, name)
	}
	work.lastEnded = time.Now()
}

// InProgress returns the names of the work in progress, sorted, none when the app is idle, and
// when some work last ended: what it changed may still be on its way to the panel.
func (work *Work) InProgress() (names []string, lastEnded time.Time) {
	work.mu.Lock()
	defer work.mu.Unlock()
	for name := range work.running {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, work.lastEnded
}

// LoadingWork is the work of the session's loading, which waits on the server screen for the
// reader's choice without counting as work: the screen counts it again as it sends the choice.
const LoadingWork = "loading"

// Wakeup wakes a goroutine that waits for something to do. A wake-up is work in progress from
// Wake until the goroutine calls Done, once it has done what it was woken for.
type Wakeup struct {
	name    string
	work    *Work
	channel chan struct{}
}

// NewWakeup returns a Wakeup whose wake-ups count in work under name.
func NewWakeup(work *Work, name string) *Wakeup {
	return &Wakeup{name: name, work: work, channel: make(chan struct{}, 1)}
}

// Wake wakes the goroutine, unless it was woken already: it holds one wake-up at most, so a
// busy goroutine runs once more, not once per call.
func (wakeup *Wakeup) Wake() {
	// Counted before it is sent, so that the work never looks over while it waits.
	wakeup.work.Begin(wakeup.name)
	select {
	case wakeup.channel <- struct{}{}:
	default:
		wakeup.work.End(wakeup.name) // the wake-up waiting covers this one
	}
}

// Woken is the channel the goroutine waits on. Call Done after each wake-up taken from it.
func (wakeup *Wakeup) Woken() <-chan struct{} { return wakeup.channel }

// Done records that the goroutine has done what the wake-up taken from Woken asked for.
func (wakeup *Wakeup) Done() { wakeup.work.End(wakeup.name) }
