package fonts

import (
	"image"
	"image/color"
	"testing"

	imagefont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// openTestFont opens the default font at 40 pixels: Go's font, off the reader.
func openTestFont(tb testing.TB, bold bool) *Font {
	tb.Helper()
	font, err := OpenInterfaceFont(40, bold)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := font.Close(); err != nil {
			tb.Error(err)
		}
	})
	return font
}

// The grays the tests draw with, those of the app.
var (
	black    = color.Gray{Y: 0x00}
	darkGray = color.Gray{Y: 0x55}
	white    = color.Gray{Y: 0xff}
)

// screenWidth is the InkPad 3's screen width, as wide as a line of the app's text can be.
const screenWidth = 1404

// whiteCanvas returns a white canvas of size.
func whiteCanvas(size image.Point) *image.Gray {
	canvas := image.NewGray(image.Rectangle{Max: size})
	for index := range canvas.Pix {
		canvas.Pix[index] = white.Y
	}
	return canvas
}

func TestFont(t *testing.T) {
	font := openTestFont(t, true)
	if font.Width("Komga") <= font.Width("Kom") {
		t.Errorf("Width is not increasing")
	}
	canvas := whiteCanvas(image.Pt(300, 60))
	font.DrawTo(canvas, image.Pt(10, 10), "Komga", black)
	inked := image.Rectangle{}
	for y := range 60 {
		for x := range 300 {
			if canvas.GrayAt(x, y).Y < 0x80 {
				inked = inked.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	// The glyphs sit under topLeft, within a line and a half (for descenders), as wide as
	// Width says.
	if line := image.Rect(10, 10, 10+font.Width("Komga"), 10+60); !inked.In(line) || inked.Max.Y < 40 {
		t.Errorf("text drawn in %v, want it in %v, reaching the size (y 40)", inked, line)
	}
}

// DrawTo, with its glyph cache, draws what imagefont.Drawer draws, also clipped and in gray.
func TestFontMatchesDrawer(t *testing.T) {
	font := openTestFont(t, false)
	for _, test := range []struct {
		name    string
		topLeft image.Point
		text    string
		gray    uint8
	}{
		{name: "black", topLeft: image.Pt(10, 10), text: "AVAWa Tokyo, LT. 104: Kozuki", gray: black.Y},
		{name: "gray, clipped top left", topLeft: image.Pt(-15, -12), text: "AVAWa", gray: darkGray.Y},
		{name: "again, from the cache", topLeft: image.Pt(-15, -12), text: "AVAWa", gray: darkGray.Y},
		{name: "clipped right", topLeft: image.Pt(250, 30), text: "clipped right…", gray: black.Y},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, want := whiteCanvas(image.Pt(400, 60)), whiteCanvas(image.Pt(400, 60))
			font.DrawTo(got, test.topLeft, test.text, color.Gray{Y: test.gray})
			drawer := imagefont.Drawer{
				Dst:  want,
				Src:  image.NewUniform(color.Gray{Y: test.gray}),
				Face: font.face,
				Dot:  fixed.P(test.topLeft.X, test.topLeft.Y+font.ascent),
			}
			drawer.DrawString(test.text)
			inked := 0
			for index := range got.Pix {
				if delta := int(got.Pix[index]) - int(want.Pix[index]); delta < -1 || delta > 1 {
					t.Fatalf("pixel (%d, %d) = %d, imagefont.Drawer draws %d",
						index%got.Stride, index/got.Stride, got.Pix[index], want.Pix[index])
				}
				if got.Pix[index] != white.Y {
					inked++
				}
			}
			if inked == 0 {
				t.Error("nothing drawn")
			}
		})
	}
}

func BenchmarkDrawString(b *testing.B) {
	font := openTestFont(b, true)
	canvas := image.NewGray(image.Rect(0, 0, screenWidth, 60))
	for b.Loop() {
		font.DrawTo(canvas, image.Pt(10, 5), "One Piece, Vol. 104: Kozuki Momonosuke, Shogun", black)
	}
}

// Width, from the cached advances and kerns, measures what imagefont.MeasureString measures.
func TestWidthMatchesMeasureString(t *testing.T) {
	font := openTestFont(t, false)
	for _, text := range []string{"", "AVAWa Tokyo", "Œufs brouillés, « déjà » — ça ﬁt", "\u00ad\u200b"} {
		for range 2 { // measured, then from the cache
			if got, want := font.Width(text), imagefont.MeasureString(font.face, text).Ceil(); got != want {
				t.Errorf("Width(%q) = %d, imagefont.MeasureString %d", text, got, want)
			}
		}
	}
	if got := font.Advance('A') + font.Kern('A', 'V') + font.Advance('V'); got != font.Width("AV") {
		t.Errorf("Advance and Kern add up to %d, Width(AV) is %d", got, font.Width("AV"))
	}
}

// Off the reader, each style opens Go's font of that style, at the size asked.
func TestOpenFace(t *testing.T) {
	widths := map[int]Style{}
	for _, style := range []Style{Regular, Bold, Italic, BoldItalic} {
		font, err := Open(Serif, style, 40)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { font.Close() })
		if font.Name != "Go" || font.Size() != 40 || font.Ascent() <= font.Descent() {
			t.Errorf("style %d: %s, size %d, ascent %d, descent %d", style, font.Name, font.Size(), font.Ascent(), font.Descent())
		}
		widths[font.Width("Bonjour, le monde")] = style
	}
	if len(widths) < 3 { // italic may be as wide as regular, but bold is wider
		t.Errorf("the styles measure the same: %v", widths)
	}
}

// Has and Covers know the characters of the font, before and after they are measured; off the
// reader, the fallbacks are Go's font.
func TestCovers(t *testing.T) {
	font := openTestFont(t, false)
	for range 2 { // asked, then from the cache
		if !font.Has('◊') || font.Has('◆') || !font.Covers("Bonjour ◊") || font.Covers("◆◆◆") {
			t.Errorf("Has ◊ %v, ◆ %v; Covers %v, %v", font.Has('◊'), font.Has('◆'), font.Covers("Bonjour ◊"), font.Covers("◆◆◆"))
		}
	}
	fallbacks, err := OpenFallbacks(Italic, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(fallbacks) != 1 || fallbacks[0].Name != "Go" || fallbacks[0].Size() != 30 {
		t.Errorf("fallbacks %v, want Go's font", fallbacks)
	}
}

// Font data parsed once opens at several sizes, measuring as Open's font does.
func TestFontData(t *testing.T) {
	if _, err := Parse("garbage", []byte("not a font")); err == nil {
		t.Errorf("garbage: no error")
	}
	parsed, err := Parse("embedded.ttf", goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := Open(SansSerif, Regular, 40)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	for _, size := range []int{20, 40} {
		font, err := parsed.Open(size)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { font.Close() })
		if font.Name != "embedded.ttf" || font.Size() != size {
			t.Errorf("%s, size %d, want embedded.ttf, %d", font.Name, font.Size(), size)
		}
		if size == 40 && font.Width("Bonjour") != reference.Width("Bonjour") {
			t.Errorf("width %d, want %d", font.Width("Bonjour"), reference.Width("Bonjour"))
		}
	}
}

// Ink measures the glyphs above and below the baseline.
func TestFontInk(t *testing.T) {
	font := openTestFont(t, false)
	x, h, y := font.Ink("x"), font.Ink("h"), font.Ink("y")
	if x.Min.Y >= 0 || h.Min.Y >= x.Min.Y || x.Max.Y > 1 || y.Max.Y <= 5 {
		t.Errorf("ink of x %v, h %v, y %v: want h taller than x, y below the baseline", x, h, y)
	}
	if minus := font.Ink("−"); minus.Max.Y >= 0 || minus.Min.Y >= 0 {
		t.Errorf("ink of a minus sign %v, want it above the baseline", minus)
	}
	if word := font.Ink("xx"); word.Dx() <= x.Dx() || word.Dx() > font.Width("xx") {
		t.Errorf("ink of xx %v, of x %v", word, x)
	}
	if space := font.Ink(" "); !space.Empty() {
		t.Errorf("ink of a space %v, want none", space)
	}
}
