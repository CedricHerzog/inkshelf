//go:build linux || darwin

package shelf

import (
	"syscall"
	"time"
)

// FreeSpace returns the bytes left on the storage that holds dir, and false if it cannot tell.
func FreeSpace(dir string) (int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, false
	}
	return int64(stat.Bavail) * int64(stat.Bsize), true
}

// cpuTime returns the CPU time the process has used, user and system, or 0 if it cannot tell
// (it is only logged).
func cpuTime() time.Duration {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}
