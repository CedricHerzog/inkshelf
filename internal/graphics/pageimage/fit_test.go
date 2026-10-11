package pageimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

// fitRef is the straightforward box filter that fit must match, one output pixel at a time.
func fitRef(src *image.Gray, width, height int) *image.Gray {
	srcWidth, srcHeight := src.Rect.Dx(), src.Rect.Dy()
	// The largest size within width×height with src's aspect ratio, at least 1×1.
	dstWidth, dstHeight := width, srcHeight*width/srcWidth
	if dstHeight > height {
		dstWidth, dstHeight = srcWidth*height/srcHeight, height
	}
	dstWidth, dstHeight = max(dstWidth, 1), max(dstHeight, 1)
	dst := image.NewGray(image.Rect(0, 0, dstWidth, dstHeight))
	for row := range dstHeight {
		for column := range dstWidth {
			// The box of source pixels that output pixel (column, row) covers: at least one.
			left, top := column*srcWidth/dstWidth, row*srcHeight/dstHeight
			right := max((column+1)*srcWidth/dstWidth, left+1)
			bottom := max((row+1)*srcHeight/dstHeight, top+1)
			box := image.Rect(left, top, right, bottom)
			// Its average gray, rounded down.
			sum := 0
			for y := range box.Dy() {
				for x := range box.Dx() {
					sum += int(src.GrayAt(box.Min.X+x, box.Min.Y+y).Y)
				}
			}
			dst.SetGray(column, row, color.Gray{Y: uint8(sum / (box.Dx() * box.Dy()))})
		}
	}
	return dst
}

func TestFit(t *testing.T) {
	tests := []struct {
		srcWidth, srcHeight int
		width, height       int
	}{
		{srcWidth: 1000, srcHeight: 1500, width: 140, height: 187},
		{srcWidth: 1999, srcHeight: 2801, width: 140, height: 187},
		{srcWidth: 300, srcHeight: 400, width: 1404, height: 1872},
		{srcWidth: 140, srcHeight: 187, width: 140, height: 187},
		{srcWidth: 5000, srcHeight: 300, width: 140, height: 187},
		{srcWidth: 3, srcHeight: 2, width: 1, height: 1},
		{srcWidth: 800, srcHeight: 600, width: 1404, height: 1872},
		{srcWidth: 4000, srcHeight: 3000, width: 30, height: 30}, // boxes too large for the reciprocal table
	}
	for _, test := range tests {
		name := fmt.Sprintf("%dx%d into %dx%d", test.srcWidth, test.srcHeight, test.width, test.height)
		t.Run(name, func(t *testing.T) {
			src := image.NewGray(image.Rect(0, 0, test.srcWidth, test.srcHeight))
			for index := range src.Pix {
				src.Pix[index] = uint8(rand.IntN(256))
			}
			got, histogram := fit(src, test.width, test.height)
			want := fitRef(src, test.width, test.height)
			if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatal("differs from the reference")
			}
			counted := 0
			for _, count := range histogram {
				counted += count
			}
			if counted != len(want.Pix) {
				t.Fatalf("the histogram counts %d pixels of %d", counted, len(want.Pix))
			}
			if histogram[want.Pix[0]] == 0 {
				t.Fatalf("the histogram has no pixel of gray %d", want.Pix[0])
			}
		})
	}
}
