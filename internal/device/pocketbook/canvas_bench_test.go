package pocketbook

import (
	"image"
	"image/color"
	"testing"
)

// benchmarkCanvas makes the device draw into a canvas laid out as the firmware's task canvas on
// an InkPad 3, until the benchmark ends.
func benchmarkCanvas(b *testing.B) {
	b.Helper()
	canvas := &image.Gray{
		Pix:    make([]byte, taskCanvasStride*screenSize.Y),
		Stride: taskCanvasStride,
		Rect:   image.Rectangle{Max: screenSize},
	}
	current = &device{canvas: canvas, firmware: &fakeFirmware{isVisible: true}}
	b.Cleanup(func() { current = nil })
}

// BenchmarkFill fills the whole screen, as clearing it before a page is drawn does, and a
// button, as the app's widgets do many times a screen.
func BenchmarkFill(b *testing.B) {
	benchmarkCanvas(b)
	gray := color.Gray{Y: 0x55}
	for _, area := range []struct {
		name string
		rect image.Rectangle
	}{
		{name: "screen", rect: image.Rectangle{Max: screenSize}},
		{name: "button", rect: image.Rect(100, 1700, 400, 1800)},
	} {
		b.Run(area.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Fill(area.rect, gray)
			}
		})
	}
}

// BenchmarkDrawImage draws a decoded page the size of the screen, what a page turn costs
// before the refresh.
func BenchmarkDrawImage(b *testing.B) {
	benchmarkCanvas(b)
	page := image.NewGray(image.Rectangle{Max: screenSize})
	for index := range page.Pix {
		page.Pix[index] = uint8(index)
	}
	b.ReportAllocs()
	for b.Loop() {
		DrawImage(page, image.Point{})
	}
}

// BenchmarkInvert inverts the whole screen, what a page turn costs in dark mode before the
// refresh.
func BenchmarkInvert(b *testing.B) {
	benchmarkCanvas(b)
	b.ReportAllocs()
	for b.Loop() {
		Invert(Screen())
	}
}
