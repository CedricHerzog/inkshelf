package main

import (
	"log"
	"os"
	"runtime/debug"

	"golang.org/x/sys/unix"
)

// logPath is on the reader's internal storage, readable over USB.
const logPath = "/mnt/ext1/applications/inkshelf.log"

// maxLogSize is the size above which the log is truncated at startup.
const maxLogSize = 1 << 20

// The file descriptors of stdout and stderr, which setupLogging points at the log.
const (
	stdoutFD = 1
	stderrFD = 2
)

// setupLogging sends the log, stdout and stderr (panics) to logPath.
func setupLogging() error {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(logPath); err == nil && info.Size() > maxLogSize {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(logPath, flags, 0o644)
	if err != nil {
		return err
	}
	for _, fd := range []int{stdoutFD, stderrFD} {
		if err := unix.Dup2(int(file.Fd()), fd); err != nil {
			return err
		}
	}
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	// Include every goroutine in crash tracebacks, not just the failing one.
	debug.SetTraceback("all")
	return nil
}
