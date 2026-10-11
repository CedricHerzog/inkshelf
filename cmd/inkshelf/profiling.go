package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// The reader is profiled when profileMarker exists in the app's directory. The profiles go to
// profileDir, next to the log, to be read over USB (docs/reference/profiling.md).
const (
	profileMarker = "profile"
	profileDir    = "/mnt/ext1/applications/inkshelf-profile"
)

// defaultProfileLimit is how long a capture lasts when the marker says nothing: the firmware
// may kill the app without letting it stop the capture, and a trace grows by megabytes a
// minute.
const defaultProfileLimit = 5 * time.Minute

// startProfiling starts profiling if the marker file asks for it, for as long as the file
// says ("90s", "10m"), and returns what stops it. Without the marker, stop does nothing.
func startProfiling() (stop func()) {
	contents, err := os.ReadFile(filepath.Join(appDir, profileMarker))
	if err != nil {
		return func() {}
	}
	limit := defaultProfileLimit
	if text := strings.TrimSpace(string(contents)); text != "" {
		if limit, err = time.ParseDuration(text); err != nil || limit <= 0 {
			log.Printf("profile: %q is no duration, profiling for %v", text, defaultProfileLimit)
			limit = defaultProfileLimit
		}
	}
	stop, err = timing.Capture(profileDir, limit)
	if err != nil {
		log.Printf("profile: %v", err)
		return func() {}
	}
	log.Printf("profiling for %v at most", limit)
	return stop
}
