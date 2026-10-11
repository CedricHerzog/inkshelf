package pageimage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestPG4Golden reads a .pg4 page as the app saves it on the reader. Its format must not
// change: the pages of downloaded books are not prepared again when the app is updated. The
// page is 5×3 pixels, their panel grays 0, 1, 2… from the top left, row after row.
func TestPG4Golden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "golden.pg4"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := Unpack(data)
	if err != nil {
		t.Fatal(err)
	}
	if page.Rect.Dx() != 5 || page.Rect.Dy() != 3 {
		t.Fatalf("size %v, want 5×3", page.Rect.Size())
	}
	for row := range 3 {
		for column := range 5 {
			want := uint8((row*5 + column) % panelGrays * grayStep)
			if got := page.GrayAt(column, row).Y; got != want {
				t.Errorf("pixel (%d, %d): %d, want %d", column, row, got, want)
			}
		}
	}

	// Packed again, the page gives the same bytes.
	packed, rowBytes := newPG4(5, 3)
	for row := range 3 {
		for column := range 5 {
			index := page.GrayAt(column, row).Y / grayStep
			if column%2 == 0 {
				packed[pg4Header+row*rowBytes+column/2] |= index << 4
			} else {
				packed[pg4Header+row*rowBytes+column/2] |= index
			}
		}
	}
	if !bytes.Equal(packed, data) {
		t.Errorf("packed again as % x, want % x", packed, data)
	}
}
