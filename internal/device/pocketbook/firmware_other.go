//go:build !(linux && arm)

package pocketbook

import (
	"errors"
	"image"
)

// Off the reader there is no hardware: Run fails, and the rest of the package builds and is
// tested on the host.

// errNotReader is the error of Run off the reader.
var errNotReader = errors.New("pocketbook: not running on a PocketBook (linux/arm)")

// openFirmware fails: there is no reader to open.
func openFirmware() (firmware, *image.Gray, error) { return nil, nil, errNotReader }
