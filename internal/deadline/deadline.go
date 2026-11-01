package deadline

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// Pool hands work to its workers and remembers the keys it gave up on. A nil Pool runs the work
// on the caller, with no limit, for tests and tools that have no workers.
type Pool struct {
	jobs    chan *job
	workers int

	mu sync.Mutex // guards the fields below
	// givenUp holds the keys whose wait ran out: they are not run again.
	givenUp map[string]bool
	// abandonedRunning counts the workers busy with a job nobody waits for any more.
	abandonedRunning int
}

// job is one piece of work and what its worker and its caller know of it.
type job struct {
	name, key string
	work      func() any
	// result has room for one value, so the worker never blocks on a caller that left.
	result chan outcome

	// Guarded by the pool's mutex.
	abandoned, finished bool
}

// outcome is a job's result, or the news that it panicked.
type outcome struct {
	value    any
	panicked bool
}

// NewPool returns a pool for workers workers. Start each one with Loop.
func NewPool(workers int) *Pool {
	return &Pool{jobs: make(chan *job), workers: workers, givenUp: make(map[string]bool)}
}

// Loop is the body of one worker: it runs the jobs it is given until ctx is cancelled.
func (pool *Pool) Loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case next := <-pool.jobs:
			pool.do(next)
		}
	}
}

// do runs one job in a span named by its caller, then hands back its result, or, if its caller
// gave up, logs how long the job took after all.
func (pool *Pool) do(next *job) {
	span := timing.Start(context.Background(), next.name)
	result := runRecovered(next.work)
	duration := span.End()

	pool.mu.Lock()
	next.finished = true
	if next.abandoned {
		pool.abandonedRunning--
	}
	abandoned := next.abandoned
	pool.mu.Unlock()

	if abandoned {
		log.Printf("%s: %v for %s, after its caller gave up", next.name, duration, next.key)
	}
	next.result <- result
}

// runRecovered runs work, turning a panic into an outcome, so that one bad element does not
// take a worker, or the app, down with it.
func runRecovered(work func() any) (result outcome) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("deadline: job panicked: %v", recovered)
			result = outcome{panicked: true}
		}
	}()
	return outcome{value: work()}
}

// Run runs work on a worker of pool and waits for its result at most limit. name names the
// work's span in the log and the trace; key names the element, so that once given up it is not
// run again. ok is false when the wait ran out, when the key was given up before, when every
// worker is busy with given-up work, or when the work panicked: the caller then shows the
// element's fallback. A limit of zero or less gives up at once.
func Run[Result any](pool *Pool, name, key string, limit time.Duration, work func() Result) (result Result, ok bool) {
	if pool == nil {
		return work(), true
	}
	if limit <= 0 || !pool.accepts(key) {
		return result, false
	}
	next := &job{
		name:   name,
		key:    key,
		work:   func() any { return work() },
		result: make(chan outcome, 1),
	}
	timer := time.NewTimer(limit)
	defer timer.Stop()

	select {
	case pool.jobs <- next:
	case <-timer.C:
		// No worker became free in time: the job never ran, so only the key is given up.
		pool.giveUp(nil, key)
		return result, false
	}
	select {
	case finished := <-next.result:
		if finished.panicked {
			pool.giveUp(nil, key)
			return result, false
		}
		value, _ := finished.value.(Result)
		return value, true
	case <-timer.C:
		pool.giveUp(next, key)
		return result, false
	}
}

// accepts reports whether a job for key may be queued: its key was never given up, and at
// least one worker is not stuck with work nobody waits for.
func (pool *Pool) accepts(key string) bool {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return !pool.givenUp[key] && pool.abandonedRunning < pool.workers
}

// giveUp remembers key as given up, and counts running, the job left behind if any, among the
// workers busy for nobody until it finishes.
func (pool *Pool) giveUp(running *job, key string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.givenUp[key] = true
	if running != nil && !running.finished {
		running.abandoned = true
		pool.abandonedRunning++
	}
}
