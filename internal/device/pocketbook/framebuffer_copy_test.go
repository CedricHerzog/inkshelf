package pocketbook

import (
	"image"
	"math/rand/v2"
	"testing"
)

// taskCanvasStride is the stride of the firmware's task canvas on an InkPad 3: its rows are
// padded to whole words.
const taskCanvasStride = 1408

func BenchmarkCopyRotated(b *testing.B) {
	canvas := &image.Gray{
		Pix:    make([]byte, taskCanvasStride*screenSize.Y),
		Stride: taskCanvasStride,
		Rect:   image.Rectangle{Max: screenSize},
	}
	framebuffer := make([]byte, screenSize.Y*screenSize.X)
	for b.Loop() {
		copyRotated(framebuffer, screenSize.Y, screenSize.X, canvas, canvas.Rect)
	}
}

// The framebuffer of TestCopyRotated: the 70x50 canvas turned, with padded rows.
const (
	testFramebufferWidth  = 50
	testFramebufferHeight = 70
	testFramebufferStride = 52
)

func TestCopyRotated(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for _, layout := range []struct {
		name   string
		stride int // the canvas's stride
		start  int // where the canvas's pixels start in their buffer
	}{
		{name: "padded", stride: 72, start: 0}, // like the firmware's task canvas: whole words
		{name: "unaligned", stride: 72, start: 1},
		{name: "odd stride", stride: 71, start: 0}, // copied a byte at a time
	} {
		canvas := randomCanvas(random, layout.stride, layout.start)
		// Parts with edges on and off the 4-pixel blocks, and the whole canvas down to its
		// last row (as FlashRefresh copies it).
		for _, rect := range []image.Rectangle{
			image.Rect(3, 5, 69, 41),
			image.Rect(4, 8, 68, 40),
			image.Rect(1, 2, 6, 5),
			image.Rect(10, 10, 12, 30),
			canvas.Rect,
		} {
			t.Run(layout.name+" "+rect.String(), func(t *testing.T) {
				framebuffer := make([]byte, testFramebufferStride*testFramebufferHeight)
				copyRotated(framebuffer, testFramebufferStride, testFramebufferHeight, canvas, rect)
				want := rotatedPixels(canvas, rect)
				checkFramebuffer(t, framebuffer, want)
				checkFramebufferRect(t, rect, want)
			})
		}
	}
}

// randomCanvas returns a 70x50 canvas of random pixels with this stride, whose pixels start at
// byte start of their buffer.
func randomCanvas(random *rand.Rand, stride, start int) *image.Gray {
	canvas := image.NewGray(image.Rect(0, 0, testFramebufferHeight, testFramebufferWidth))
	canvas.Stride = stride
	canvas.Pix = make([]byte, start+stride*testFramebufferWidth)[start:]
	for index := range canvas.Pix {
		canvas.Pix[index] = byte(random.Uint32())
	}
	return canvas
}

// rotatedPixels returns the framebuffer that copyRotated should write, pixel by pixel: canvas
// pixel (x, y) of rect goes to framebuffer row testFramebufferHeight-1-x, column y. The rest
// stays 0.
func rotatedPixels(canvas *image.Gray, rect image.Rectangle) []byte {
	want := make([]byte, testFramebufferStride*testFramebufferHeight)
	for y := range canvas.Rect.Dy() {
		for x := range canvas.Rect.Dx() {
			if image.Pt(x, y).In(rect) {
				row, column := testFramebufferHeight-1-x, y
				want[row*testFramebufferStride+column] = canvas.GrayAt(x, y).Y
			}
		}
	}
	return want
}

// checkFramebuffer fails at the first byte of framebuffer that differs from want.
func checkFramebuffer(t *testing.T, framebuffer, want []byte) {
	t.Helper()
	for index := range framebuffer {
		if framebuffer[index] != want[index] {
			t.Fatalf("framebuffer byte %d (row %d, column %d) = %d, want %d",
				index, index/testFramebufferStride, index%testFramebufferStride, framebuffer[index], want[index])
		}
	}
}

// checkFramebufferRect checks that the rectangle sent to the driver covers exactly the bytes
// written, want.
func checkFramebufferRect(t *testing.T, rect image.Rectangle, want []byte) {
	t.Helper()
	panelRect := framebufferRect(rect, testFramebufferHeight)
	for row := range testFramebufferHeight {
		for column := range testFramebufferWidth {
			inside := image.Pt(column, row).In(panelRect)
			if written := want[row*testFramebufferStride+column] != 0; written && !inside {
				t.Fatalf("framebuffer (%d, %d) written outside %v", column, row, panelRect)
			}
		}
	}
	if panelRect.Dx() != rect.Dy() || panelRect.Dy() != rect.Dx() {
		t.Errorf("framebufferRect(%v) = %v, not the same size turned", rect, panelRect)
	}
}
