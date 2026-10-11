//go:build !(linux || darwin)

package shelf

import "time"

// FreeSpace cannot tell the free space on this system.
func FreeSpace(dir string) (int64, bool) { return 0, false }

// cpuTime cannot tell the CPU time used on this system.
func cpuTime() time.Duration { return 0 }
