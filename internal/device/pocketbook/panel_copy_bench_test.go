package pocketbook

import (
	"image"
	"testing"
)

// BenchmarkCopyToPanel copies a full screen, as the emulator and remote control do on every
// full refresh.
func BenchmarkCopyToPanel(b *testing.B) {
	canvas := image.NewGray(image.Rectangle{Max: screenSize})
	for index := range canvas.Pix {
		canvas.Pix[index] = uint8(index)
	}
	panel := image.NewGray(canvas.Rect)
	b.ReportAllocs()
	for b.Loop() {
		copyToPanel(panel, canvas, canvas.Rect, GL16)
	}
}
