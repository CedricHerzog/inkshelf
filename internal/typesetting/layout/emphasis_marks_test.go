package layout

import (
	"image"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// inkIn reports whether area of canvas has a pixel darker than light gray.
func inkIn(canvas *image.Gray, area image.Rectangle) bool {
	area = area.Intersect(canvas.Bounds())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if canvas.GrayAt(x, y).Y < 160 {
				return true
			}
		}
	}
	return false
}

// Each keyword mark is a shape that draws ink above each emphasized character, whether a font
// has its glyph or not (the test fonts lack the sesame), and none above spaces and punctuation.
// Open marks draw less ink than filled ones.
func TestEmphasisMarkShapes(t *testing.T) {
	faces := newTestFaces(t)
	ink := map[rune]int{}
	for mark, want := range keywordMarks {
		page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "ab, c", Emphasis: document.Emphasis{Mark: mark}}}}), leftSettings, faces, noImages)
		canvas := whiteCanvas(leftSettings)
		if err := page.Draw(canvas, faces, nil); err != nil {
			t.Fatal(err)
		}
		var marks []PlacedWord
		for _, word := range page.Lines[0].Words {
			if word.Mark != NoMark {
				marks = append(marks, word)
			}
		}
		if len(marks) != 3 {
			t.Errorf("mark %q: %d marks, want one over a, b and c", mark, len(marks))
			continue
		}
		for _, placed := range marks {
			if placed.Mark != want.shape || placed.MarkOpen != want.open || placed.Text != "" {
				t.Errorf("mark %q placed as %+v, want %+v", mark, placed, want)
			}
			square := image.Rect(placed.X, placed.Top, placed.X+placed.Size, placed.Top+placed.Size)
			if !inkIn(canvas, square) {
				t.Errorf("mark %q: no ink in %v", mark, square)
			}
		}
		// Nothing over the comma and the space, between b's mark and c's.
		if gap := image.Rect(marks[1].X+marks[1].Size+1, marks[1].Top, marks[2].X-1, marks[1].Top+marks[1].Size); inkIn(canvas, gap) {
			t.Errorf("mark %q: ink over the comma or the space, in %v", mark, gap)
		}
		square := image.Rect(marks[0].X, marks[0].Top, marks[0].X+marks[0].Size, marks[0].Top+marks[0].Size)
		for y := square.Min.Y; y < square.Max.Y; y++ {
			for x := square.Min.X; x < square.Max.X; x++ {
				ink[mark] += 255 - int(canvas.GrayAt(x, y).Y)
			}
		}
	}
	for filled, open := range map[rune]rune{'•': '◦', '●': '○', '◉': '◎', '▲': '△', '﹅': '﹆'} {
		if ink[open] >= ink[filled] {
			t.Errorf("open %q draws %d of ink, filled %q %d: want less", open, ink[open], filled, ink[filled])
		}
	}
}

// A mark that is not a keyword is drawn as its character when a font has it, else as a dot.
func TestEmphasisMarkCharacters(t *testing.T) {
	faces := newTestFaces(t)
	for mark, want := range map[rune]PlacedWord{'*': {Text: "*"}, '\U0001F600': {Mark: DotMark}} {
		page := paginateOne(t, oneBlock(document.Block{Runs: []document.Run{{Text: "a", Emphasis: document.Emphasis{Mark: mark}}}}), leftSettings, faces, noImages)
		if got := page.Lines[0].Words[1]; got.Text != want.Text || got.Mark != want.Mark {
			t.Errorf("mark %q drawn as %q, shape %d; want %q, shape %d", mark, got.Text, got.Mark, want.Text, want.Mark)
		}
	}
}
