package pocketbook

import (
	"image"
	"math/rand/v2"
	"testing"
)

// landscapeSize is the InkPad 3's screen in landscape: the framebuffer as it is.
var landscapeSize = image.Pt(screenSize.Y, screenSize.X)

func BenchmarkCopyStraight(b *testing.B) {
	canvas := image.NewGray(image.Rectangle{Max: landscapeSize})
	framebuffer := make([]byte, landscapeSize.X*landscapeSize.Y)
	for b.Loop() {
		copyStraight(framebuffer, landscapeSize.X, canvas, canvas.Rect)
	}
}

func TestCopyStraight(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	canvas := image.NewGray(image.Rect(0, 0, testFramebufferWidth, testFramebufferHeight))
	for index := range canvas.Pix {
		canvas.Pix[index] = byte(random.Uint32())
	}
	for _, rect := range []image.Rectangle{image.Rect(3, 5, 41, 69), canvas.Rect, image.Rect(-10, -10, 5, 5)} {
		framebuffer := make([]byte, testFramebufferStride*testFramebufferHeight)
		copyStraight(framebuffer, testFramebufferStride, canvas, rect)
		inside := rect.Intersect(canvas.Rect)
		for y := range testFramebufferHeight {
			for x := range testFramebufferStride {
				want := byte(0)
				if image.Pt(x, y).In(inside) {
					want = canvas.GrayAt(x, y).Y
				}
				if got := framebuffer[y*testFramebufferStride+x]; got != want {
					t.Fatalf("%v: framebuffer (%d, %d) = %#x, want %#x", rect, x, y, got, want)
				}
			}
		}
	}
}

// In landscape, a touch lands on the framebuffer pixel under it, as copyStraight draws the
// landscape canvas: the point the portrait mapping gives, turned as the framebuffer is.
func TestTouchLandscape(t *testing.T) {
	for _, point := range []image.Point{{0, 0}, {1871, 1403}, {100, 1300}, {936, 702}} {
		rawX, rawY := rawTouch(point, landscapeSize)
		if got := touchPoint(rawX, rawY, landscapeSize); got != point {
			t.Errorf("touchPoint(rawTouch(%v)) = %v", point, got)
		}
		// The same raw touch on the portrait screen, put where the framebuffer has it.
		portrait := touchPoint(rawX, rawY, screenSize)
		framebuffer := image.Pt(portrait.Y, screenSize.X-1-portrait.X)
		if framebuffer != point {
			t.Errorf("%v: the portrait screen's %v is framebuffer pixel %v", point, portrait, framebuffer)
		}
	}
	// Portrait is as it was measured.
	if got := touchPoint(0, 0, screenSize); got != image.Pt(0, screenSize.Y-1) {
		t.Errorf("portrait raw (0, 0) = %v", got)
	}
}

func TestSetOrientation(t *testing.T) {
	fake := &fakeFirmware{isVisible: true}
	portrait := image.NewGray(image.Rectangle{Max: screenSize})
	current = &device{canvas: portrait, portrait: portrait, firmware: fake}
	t.Cleanup(func() { current = nil })

	SetOrientation(Landscape)
	if ScreenSize() != landscapeSize || ScreenOrientation() != Landscape || Canvas() == portrait {
		t.Fatalf("landscape: size %v, orientation %v", ScreenSize(), ScreenOrientation())
	}
	Refresh(image.Rect(0, 0, 10, 10), GL16)
	Refresh(image.Rect(0, 0, 10, 10), GL16)
	if want := []image.Rectangle{{Max: landscapeSize}, image.Rect(0, 0, 10, 10)}; len(fake.flushes) != 2 || fake.flushes[0] != want[0] || fake.flushes[1] != want[1] {
		t.Errorf("flushes %v, want the whole landscape screen first, then the rectangle", fake.flushes)
	}
	landscape := Canvas()

	SetOrientation(Landscape) // does nothing
	Refresh(image.Rect(0, 0, 10, 10), GL16)
	SetOrientation(Portrait)
	if ScreenSize() != screenSize || Canvas() != portrait {
		t.Fatalf("portrait again: size %v", ScreenSize())
	}
	SetOrientation(Landscape)
	if Canvas() != landscape {
		t.Error("the landscape canvas is made again")
	}
	if got := oriented(screenSize); got != landscapeSize {
		t.Errorf("oriented = %v", got)
	}
}
