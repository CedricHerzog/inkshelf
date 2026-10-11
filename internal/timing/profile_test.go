package timing

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCapture(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	stop, err := Capture(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	Start(context.Background(), "work").End()
	stop()
	stop()
	for _, name := range []string{"cpu.pprof", "trace.out", "heap.pprof", "allocs.pprof", "goroutine.pprof", "block.pprof", "mutex.pprof"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Size() == 0 {
			t.Errorf("%s: %v, size %d", name, err, sizeOf(info))
		}
	}
}

func TestCaptureLimit(t *testing.T) {
	dir := t.TempDir()
	if _, err := Capture(dir, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// stop turns mutex sampling off last, once every file is written.
		if runtime.SetMutexProfileFraction(-1) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("the capture did not stop by itself")
}

func sizeOf(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}
