package fonts

import (
	"image"
	"image/color"
	"testing"
)

// openCJK returns Noto Sans CJK at size pixels.
func openCJK(t *testing.T, size int) *Font {
	t.Helper()
	file := cjkSans.parsed()
	if file == nil {
		t.Fatal("no CJK font")
	}
	font, err := file.Open(size)
	if err != nil {
		t.Fatal(err)
	}
	return font
}

// Shaped top to bottom, a bracket takes its vertical form, and each character an em of the
// column, centred on it.
func TestShapeVertical(t *testing.T) {
	font := openCJK(t, 40)
	horizontal, vertical := font.Shape("「永」", false, "ja"), font.ShapeVertical("「永」", "ja")
	if len(vertical) != 3 {
		t.Fatalf("%d glyphs, want 3", len(vertical))
	}
	if vertical[0].Index == horizontal[0].Index || vertical[1].Index != horizontal[1].Index {
		t.Errorf("glyphs %v, horizontal %v: want the brackets' vertical forms only", vertical, horizontal)
	}
	for _, shaped := range vertical {
		if shaped.Advance != 40 || shaped.X != -20 || shaped.Y < 30 || shaped.Y > 40 {
			t.Errorf("glyph %+v: want an advance of 40, centred (X -20), its baseline about 35 down", shaped)
		}
	}
	dst := image.NewGray(image.Rect(0, 0, 80, 160))
	for index := range dst.Pix {
		dst.Pix[index] = 0xff
	}
	// Two ideographs fill their ems, one under the other.
	font.DrawGlyphsVertical(dst, image.Pt(40, 10), font.ShapeVertical("永中", "ja"), color.Gray{})
	ink := inkOf(dst)
	if ink.Min.X < 20 || ink.Max.X > 60 || ink.Min.Y < 10 || ink.Max.Y > 90 || ink.Dy() < 70 {
		t.Errorf("ink %v, want within the column x 20-60, from y 10 down 80", ink)
	}
}

// Turned text goes down from its origin, the tops of its letters to the right of the baseline.
func TestDrawTurned(t *testing.T) {
	font := openCJK(t, 40)
	width := font.Width("Lj")
	for _, draw := range []func(dst *image.Gray){
		func(dst *image.Gray) { font.DrawTurned(dst, image.Pt(50, 10), "Lj", color.Gray{}, 0) },
		func(dst *image.Gray) {
			font.DrawGlyphsTurned(dst, image.Pt(50, 10), font.Shape("Lj", false, "en"), color.Gray{})
		},
	} {
		dst := image.NewGray(image.Rect(0, 0, 100, 100))
		for index := range dst.Pix {
			dst.Pix[index] = 0xff
		}
		draw(dst)
		ink := inkOf(dst)
		// The L's upright stroke rises from the baseline (to the right), the j's descender goes
		// under it (to the left).
		if ink.Min.Y < 10 || ink.Max.Y > 10+width+2 || ink.Min.X >= 50 || ink.Max.X <= 70 {
			t.Errorf("ink %v, want from y 10 down %d, either side of x 50, mostly right", ink, width)
		}
	}
}

// inkOf returns the rectangle of dst's pixels that are not white.
func inkOf(dst *image.Gray) image.Rectangle {
	var ink image.Rectangle
	for y := dst.Rect.Min.Y; y < dst.Rect.Max.Y; y++ {
		for x := dst.Rect.Min.X; x < dst.Rect.Max.X; x++ {
			if dst.GrayAt(x, y).Y < 0x80 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return ink
}
