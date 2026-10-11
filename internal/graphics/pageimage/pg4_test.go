package pageimage

import (
	"fmt"
	"image"
	"math/rand/v2"
	"testing"
)

// identityTable is a tone table that changes nothing.
func identityTable() *[256]uint8 {
	var identity [256]uint8
	for level := range identity {
		identity[level] = uint8(level)
	}
	return &identity
}

// A page of the panel's grays, through an identity table, has nothing to dither: it reads
// back exactly.
func TestGray4RoundTrip(t *testing.T) {
	for _, width := range []int{1, 2, 7, 64} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			src := image.NewGray(image.Rect(0, 0, width, 3))
			for index := range src.Pix {
				src.Pix[index] = uint8(rand.IntN(panelGrays) * grayStep)
			}
			gray, err := Unpack(ditherAndPack(src, identityTable()))
			if err != nil {
				t.Fatal(err)
			}
			if gray.Bounds() != src.Bounds() {
				t.Fatalf("size %v, want %v", gray.Bounds(), src.Bounds())
			}
			for index, value := range src.Pix {
				if gray.Pix[index] != value {
					t.Fatalf("pixel %d is %d, want %d", index, gray.Pix[index], value)
				}
			}
		})
	}
}

func TestUnpackTruncated(t *testing.T) {
	packed := ditherAndPack(image.NewGray(image.Rect(0, 0, 5, 5)), identityTable())
	if _, err := Unpack(packed[:20]); err == nil {
		t.Fatal("a truncated page decoded")
	}
}

func TestUnpackNotAPage(t *testing.T) {
	if _, err := Unpack([]byte("\x89PNG\r\n\x1a\n")); err == nil {
		t.Fatal("a PNG decoded as a page")
	}
}
