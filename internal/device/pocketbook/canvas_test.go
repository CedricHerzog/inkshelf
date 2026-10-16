package pocketbook

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestDrawing(t *testing.T) {
	fake := &fakeFirmware{isVisible: true}
	canvas := image.NewGray(image.Rect(0, 0, 40, 30))
	canvas.Stride = 44
	canvas.Pix = make([]byte, 44*30)
	current = &device{canvas: canvas, firmware: fake}
	t.Cleanup(func() { current = nil })

	ClearScreen()
	Fill(image.Rect(-5, 10, 10, 12), color.Gray{Y: 0})
	img := image.NewGray(image.Rect(100, 100, 110, 105))
	for index := range img.Pix {
		img.Pix[index] = 0x80
	}
	DrawImage(img, image.Pt(35, 28)) // clipped to 5x2
	for y := range 30 {
		for x := range 40 {
			want := white.Y
			switch {
			case x < 10 && y >= 10 && y < 12:
				want = 0
			case x >= 35 && y >= 28:
				want = 0x80
			}
			if got := canvas.GrayAt(x, y).Y; got != want {
				t.Fatalf("pixel (%d, %d) = %#x, want %#x", x, y, got, want)
			}
		}
	}
	Refresh(image.Rect(30, 20, 100, 100), GL16)
	if len(fake.flushes) != 1 || fake.flushes[0] != image.Rect(30, 20, 40, 30) {
		t.Errorf("flushes = %v, want the update clipped to the screen", fake.flushes)
	}
}

func TestInvertRect(t *testing.T) {
	canvas := image.NewGray(image.Rect(0, 0, 4, 2))
	copy(canvas.Pix, []byte{0, 0, 0, 0, 0, 0, 0x00, 0x30})
	current = &device{canvas: canvas, firmware: &fakeFirmware{isVisible: true}}
	t.Cleanup(func() { current = nil })

	Invert(image.Rect(2, 1, 9, 9)) // clipped to 2x1
	want := []byte{0, 0, 0, 0, 0, 0, 0xff, 0xcf}
	if !bytes.Equal(canvas.Pix, want) {
		t.Errorf("canvas = %x, want %x", canvas.Pix, want)
	}
}
