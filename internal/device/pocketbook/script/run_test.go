package script

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// realReader is a Target that is not Emulated, as a reader under remote control is.
type realReader struct{}

func (realReader) Size() image.Point         { return image.Pt(1404, 1872) }
func (realReader) PointerDown(image.Point)   {}
func (realReader) PointerMove(image.Point)   {}
func (realReader) PointerUp(image.Point)     {}
func (realReader) PressKey(pocketbook.Key)   {}
func (realReader) ReleaseKey(pocketbook.Key) {}
func (realReader) Panel() *image.Gray        { return image.NewGray(image.Rect(0, 0, 1, 1)) }
func (realReader) LastRefresh() time.Time    { return time.Time{} }
func (realReader) Pending() bool             { return false }
func (realReader) RequestExit()              {}

func TestRunRejectsEmulatedCommandsOnAReader(t *testing.T) {
	steps, err := Parse("tap 1 1\nhome\n")
	if err != nil {
		t.Fatal(err)
	}
	idle := func() ([]string, time.Time) { return nil, time.Time{} }
	saved := false
	err = Run(realReader{}, idle, steps, func(string, *image.Gray) error { saved = true; return nil })
	if err == nil || !strings.HasPrefix(err.Error(), "line 2: home") {
		t.Errorf("Run = %v, want an error on line 2", err)
	}
	if saved {
		t.Error("Run ran steps before failing")
	}
}
