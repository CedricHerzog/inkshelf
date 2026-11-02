package layout

import (
	"image"
	"slices"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
)

// Latin, Greek and Cyrillic in the interface's font draw a character after another; Arabic,
// Hebrew and Japanese, which it lacks or cannot join, need a Label.
func TestPlainLabel(t *testing.T) {
	font, err := fonts.OpenInterfaceFont(40, false)
	if err != nil {
		t.Fatal(err)
	}
	defer font.Close()
	for _, test := range []struct {
		text  string
		plain bool
	}{
		{"Le Trône de Fer - Tome 3", true},
		{"Ἰλιάς, Война и мир", true},
		{"الأجنحة المتكسرة", false},
		{"ספר", false},
		{"草枕", false},
	} {
		if got := PlainLabel(font, test.text); got != test.plain {
			t.Errorf("PlainLabel(%q) = %v, want %v", test.text, got, test.plain)
		}
	}
}

// An Arabic title is one right-to-left run, joined: as wide as its shaped glyphs, narrower than
// its letters apart. In a Latin title, an Arabic word is drawn at its end, where it is read.
func TestLabel(t *testing.T) {
	faces := newTestFaces(t)
	title := "الأجنحة المتكسرة"
	label := faces.Label(title, "sans-serif", 0, 40)
	if len(label.runs) != 1 || label.runs[0].level != 1 || label.runs[0].format.shaping == nil {
		t.Fatalf("%q: runs %+v, want one shaped run at level 1", title, label.runs)
	}
	if want := faces.width(label.runs[0].format, title); label.Width != want {
		t.Errorf("%q is %d wide, want %d", title, label.Width, want)
	}
	font, _ := faces.FamilyFont("sans-serif", 0, 40)
	if apart := font.Width(title); label.Width >= apart {
		t.Errorf("%q is %d wide joined, its letters apart %d", title, label.Width, apart)
	}

	mixed := faces.Label("Inkshelf قارئ", "sans-serif", 0, 40)
	texts := make([]string, len(mixed.runs))
	for index, run := range mixed.runs {
		texts[index] = run.text
	}
	if !slices.Equal(texts, []string{"Inkshelf ", "قارئ"}) {
		t.Errorf("runs from left to right %q, want the Latin first", texts)
	}
	canvas := image.NewGray(image.Rect(0, 0, mixed.Width, 60))
	for index := range canvas.Pix {
		canvas.Pix[index] = 0xff
	}
	if err := faces.DrawLabel(canvas, mixed, image.Point{}, black); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(canvas.Pix, 0) {
		t.Error("nothing drawn")
	}
}
