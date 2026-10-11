package fonts

import (
	"image"
	"os"
	"slices"
	"testing"
)

// emulatorFonts is the emulator's copy of the firmware's fonts, DejaVu Sans among them.
const emulatorFonts = "../../../emulator/fonts/"

// readDejaVu returns the bytes of the emulator's DejaVu Sans.
func readDejaVu(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile(emulatorFonts + "DejaVuSans.ttf")
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

// openDejaVu opens the emulator's DejaVu Sans at 40 pixels.
func openDejaVu(tb testing.TB) *Font {
	tb.Helper()
	parsed, err := Parse("DejaVuSans.ttf", readDejaVu(tb))
	if err != nil {
		tb.Fatal(err)
	}
	font, err := parsed.Open(40)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { font.Close() })
	return font
}

// indexes returns the glyph indexes of glyphs.
func indexes(glyphs []Glyph) []int {
	all := make([]int, len(glyphs))
	for index, shaped := range glyphs {
		all[index] = int(shaped.Index)
	}
	return all
}

// DejaVu Sans, the fallback font, has Arabic and Hebrew, which Go's fonts lack.
func TestFallbackCoversArabicAndHebrew(t *testing.T) {
	dejaVu := openDejaVu(t)
	goFont := openTestFont(t, false)
	for _, text := range []string{"السلام عليكم", "שָׁלוֹם"} {
		if !dejaVu.Covers(text) || goFont.Covers(text) {
			t.Errorf("%q: DejaVu covers it %v, Go %v", text, dejaVu.Covers(text), goFont.Covers(text))
		}
	}
}

// Arabic letters join: their shaped glyphs are not those of the letters alone, a ligature makes
// one of two. Right-to-left text comes out from left to right: its last letter first.
func TestShape(t *testing.T) {
	font := openDejaVu(t)
	arabic := "سلام"
	shaped := font.Shape(arabic, true, "ar")
	alone := font.unshaped([]rune(arabic), true)
	if len(shaped) != 3 || slices.Equal(indexes(shaped), indexes(alone)) { // lam and alef make one
		t.Errorf("%q shaped as %v, alone %v: want joined forms", arabic, indexes(shaped), indexes(alone))
	}
	hebrew := font.Shape("שלום", true, "he")
	final, _ := font.file.parsed.GlyphIndex(&font.buffer, 'ם')
	if len(hebrew) != 4 || hebrew[0].Index != final {
		t.Errorf("שלום shaped as %v, want the final mem (%d) first", indexes(hebrew), final)
	}
	if got, want := WidthOf(font.Shape("Komga", false, "")), font.Width("Komga"); got < want-1 || got > want+1 {
		t.Errorf("Komga shaped %d wide, measured %d", got, want)
	}
	if again := font.Shape(arabic, true, "ar"); &again[0] != &shaped[0] {
		t.Error("shaped again, not from the cache")
	}
}

// Glyphs drawn by index are those DrawTo draws by character.
func TestDrawGlyphs(t *testing.T) {
	font := openDejaVu(t)
	text := "Komga, page 104"
	got, want := whiteCanvas(image.Pt(400, 60)), whiteCanvas(image.Pt(400, 60))
	font.DrawGlyphs(got, image.Pt(10, 5), font.Shape(text, false, ""), black)
	font.DrawTo(want, image.Pt(10, 5), text, black)
	differing := 0
	for index := range got.Pix {
		if delta := int(got.Pix[index]) - int(want.Pix[index]); delta < -2 || delta > 2 {
			differing++
		}
	}
	if differing > 0 {
		t.Errorf("%d pixels differ from DrawTo's", differing)
	}
	arabic := whiteCanvas(image.Pt(400, 60))
	font.DrawGlyphs(arabic, image.Pt(10, 5), font.Shape("السلام عليكم", true, "ar"), black)
	if slices.Equal(arabic.Pix, whiteCanvas(image.Pt(400, 60)).Pix) {
		t.Error("Arabic not drawn")
	}
}
