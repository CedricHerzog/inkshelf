package deadline

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// startWorkers starts workers workers on pool, as the app does with state.GoLoop, and returns
// a function that stops them and waits for them to return.
func startWorkers(t *testing.T, pool *Pool, workers int) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var group sync.WaitGroup
	for range workers {
		group.Go(func() { pool.Loop(ctx) })
	}
	return func() {
		cancel()
		group.Wait()
	}
}

func TestRunReturnsResultWithinLimit(t *testing.T) {
	pool := NewPool(1)
	defer startWorkers(t, pool, 1)()

	result, ok := Run(pool, "test job", "image", time.Second, func() int { return 42 })
	if !ok || result != 42 {
		t.Fatalf("Run = %d, %v; want 42, true", result, ok)
	}
}

func TestRunGivesUpPastLimitAndDropsLateResult(t *testing.T) {
	pool := NewPool(1)
	defer startWorkers(t, pool, 1)()
	release := make(chan struct{})

	start := time.Now()
	result, ok := Run(pool, "slow job", "slow", 20*time.Millisecond, func() int {
		<-release
		return 7
	})
	waited := time.Since(start)
	close(release)

	if ok || result != 0 {
		t.Fatalf("Run = %d, %v; want the zero value and false", result, ok)
	}
	if waited > 500*time.Millisecond {
		t.Fatalf("Run waited %v past a 20ms limit", waited)
	}
}

func TestRunRefusesGivenUpKeyAtOnce(t *testing.T) {
	pool := NewPool(2)
	defer startWorkers(t, pool, 2)()
	release := make(chan struct{})
	defer close(release)

	Run(pool, "slow job", "same", 10*time.Millisecond, func() int { <-release; return 1 })

	ran := false
	start := time.Now()
	_, ok := Run(pool, "slow job", "same", time.Second, func() int { ran = true; return 1 })
	if ok || ran {
		t.Fatalf("a given-up key ran again (ok %v, ran %v)", ok, ran)
	}
	if waited := time.Since(start); waited > 100*time.Millisecond {
		t.Fatalf("a given-up key waited %v instead of failing at once", waited)
	}
}

func TestRunRefusesWhenEveryWorkerIsBusyForNobody(t *testing.T) {
	pool := NewPool(2)
	defer startWorkers(t, pool, 2)()
	release := make(chan struct{})
	defer close(release)

	for _, key := range []string{"first", "second"} {
		if _, ok := Run(pool, "stuck job", key, 10*time.Millisecond, func() int { <-release; return 1 }); ok {
			t.Fatalf("job %s finished; want it given up", key)
		}
	}

	start := time.Now()
	_, ok := Run(pool, "next job", "third", time.Second, func() int { return 1 })
	if ok {
		t.Fatal("a job was queued behind two workers busy with given-up work")
	}
	if waited := time.Since(start); waited > 100*time.Millisecond {
		t.Fatalf("Run waited %v instead of refusing at once", waited)
	}
}

func TestRunAcceptsAgainOnceAbandonedWorkFinishes(t *testing.T) {
	pool := NewPool(1)
	defer startWorkers(t, pool, 1)()
	release := make(chan struct{})

	Run(pool, "stuck job", "stuck", 10*time.Millisecond, func() int { <-release; return 1 })
	close(release)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if result, ok := Run(pool, "next job", "next", 100*time.Millisecond, func() int { return 3 }); ok {
			if result != 3 {
				t.Fatalf("Run = %d; want 3", result)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the worker stayed counted as busy after its abandoned job finished")
}

func TestRunWithNoTimeLeftGivesUpAtOnce(t *testing.T) {
	pool := NewPool(1)
	defer startWorkers(t, pool, 1)()

	ran := false
	if _, ok := Run(pool, "job", "late", 0, func() int { ran = true; return 1 }); ok || ran {
		t.Fatalf("Run with no time left: ok %v, ran %v; want neither", ok, ran)
	}
}

func TestRunTurnsPanicIntoFallback(t *testing.T) {
	pool := NewPool(1)
	defer startWorkers(t, pool, 1)()

	if _, ok := Run(pool, "bad job", "bad", time.Second, func() int { panic("broken image") }); ok {
		t.Fatal("a job that panicked reported success")
	}
	if result, ok := Run(pool, "good job", "good", time.Second, func() int { return 5 }); !ok || result != 5 {
		t.Fatalf("the worker did not survive a panic: Run = %d, %v", result, ok)
	}
}

func TestNilPoolRunsInline(t *testing.T) {
	var pool *Pool
	result, ok := Run(pool, "job", "inline", time.Nanosecond, func() string {
		time.Sleep(5 * time.Millisecond)
		return "done"
	})
	if !ok || result != "done" {
		t.Fatalf("Run on a nil pool = %q, %v; want done, true with no limit", result, ok)
	}
}

func TestWorkersStopWithoutLeaking(t *testing.T) {
	before := runtime.NumGoroutine()
	pool := NewPool(2)
	stop := startWorkers(t, pool, 2)
	Run(pool, "job", "one", time.Second, func() int { return 1 })
	stop()

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("%d goroutines before, %d after the workers stopped", before, after)
	}
}
