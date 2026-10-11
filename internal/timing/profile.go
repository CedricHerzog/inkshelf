package timing

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"
)

// endProfiles are the profiles written when a capture stops: what is allocated then, all
// that was allocated, the goroutines, and where they waited on channels and locks.
var endProfiles = []string{"heap", "allocs", "goroutine", "block", "mutex"}

// Block and mutex events are sampled only during a capture, where their cost is welcome: one
// sample per 10 µs blocked, one contended lock in 5.
const (
	blockProfileRate     = int(10 * time.Microsecond)
	mutexProfileFraction = 5
)

// Capture profiles the program into dir until stop is called, or until limit has passed if
// it is not 0: the firmware may kill the app without letting it stop, and a trace grows by
// megabytes a minute. It writes cpu.pprof and trace.out as it goes and the other profiles
// (heap.pprof, allocs.pprof, ...) when it stops. Calling stop again does nothing.
func Capture(dir string, limit time.Duration) (stop func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cpuFile, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		return nil, err
	}
	traceFile, err := os.Create(filepath.Join(dir, "trace.out"))
	if err != nil {
		return nil, errors.Join(err, cpuFile.Close())
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		return nil, errors.Join(err, cpuFile.Close(), traceFile.Close())
	}
	if err := trace.Start(traceFile); err != nil {
		pprof.StopCPUProfile()
		return nil, errors.Join(err, cpuFile.Close(), traceFile.Close())
	}
	runtime.SetBlockProfileRate(blockProfileRate)
	runtime.SetMutexProfileFraction(mutexProfileFraction)
	log.Printf("profiling to %s", dir)

	var once sync.Once
	stop = func() {
		once.Do(func() {
			trace.Stop()
			pprof.StopCPUProfile()
			errs := []error{traceFile.Close(), cpuFile.Close()}
			for _, name := range endProfiles {
				errs = append(errs, writeProfile(dir, name))
			}
			runtime.SetBlockProfileRate(0)
			runtime.SetMutexProfileFraction(0)
			if err := errors.Join(errs...); err != nil {
				log.Printf("profiling to %s: %v", dir, err)
				return
			}
			log.Printf("profiling to %s stopped", dir)
		})
	}
	if limit > 0 {
		time.AfterFunc(limit, stop)
	}
	return stop, nil
}

// writeProfile writes the runtime's profile name to dir/name.pprof.
func writeProfile(dir, name string) error {
	file, err := os.Create(filepath.Join(dir, name+".pprof"))
	if err != nil {
		return err
	}
	if name == "heap" {
		runtime.GC() // a heap profile is as of the last collection
	}
	if err := pprof.Lookup(name).WriteTo(file, 0); err != nil {
		return errors.Join(fmt.Errorf("%s profile: %w", name, err), file.Close())
	}
	return file.Close()
}
