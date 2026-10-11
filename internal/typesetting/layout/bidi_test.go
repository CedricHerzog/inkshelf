package layout

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// TestMain makes the fonts of the emulator, the reader's, the first to try: DejaVu Sans is the
// fallback that has Arabic and Hebrew, which Go's fonts lack.
func TestMain(m *testing.M) {
	fonts.AddDir("../../../emulator/fonts")
	os.Exit(m.Run())
}

// wordsLeftToRight returns the text of the words of line, from left to right.
func wordsLeftToRight(line Line) []string {
	words := slices.Clone(line.Words)
	slices.SortStableFunc(words, func(first, second PlacedWord) int { return first.X - second.X })
	texts := make([]string, len(words))
	for index, word := range words {
		texts[index] = word.Text
	}
	return texts
}

// The levels of the bidirectional algorithm: right-to-left letters are odd, digits in them
// even and higher; an isolate is set apart; a block of left-to-right text has none.
func TestBidiLevels(t *testing.T) {
	for _, test := range []struct {
		name  string
		block document.Block
		want  []int8
		rtl   bool
	}{
		{"Hebrew in English", document.Block{Runs: []document.Run{{Text: "ab אב c"}}}, []int8{0, 0, 0, 1, 1, 0, 0}, false},
		{"digits in Hebrew", document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: "אב 12"}}}, []int8{1, 1, 1, 2, 2}, true},
		{"English in Hebrew", document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: "ab"}}}, []int8{2, 2}, true},
		{"auto, Hebrew first", document.Block{Direction: document.DirectionAuto, Runs: []document.Run{{Text: "אב ab"}}}, []int8{1, 1, 1, 2, 2}, true},
		{"auto, Latin first", document.Block{Direction: document.DirectionAuto, Runs: []document.Run{{Text: "ab אב"}}}, []int8{0, 0, 0, 1, 1}, false},
		{"isolate", document.Block{Runs: []document.Run{{Text: "a "}, {Text: "1 ב", Isolate: document.IsolateRightToLeft}, {Text: " c"}}}, []int8{0, 0, 2, 1, 1, 0, 0}, false},
		{"override", document.Block{Runs: []document.Run{{Text: "ab", Isolate: document.OverrideRightToLeft}}}, []int8{1, 1}, false},
	} {
		levels, rightToLeft := bidiLevels(test.block, document.ScanScripts(test.block.Runs).RightToLeft)
		if !slices.Equal(levels, test.want) || rightToLeft != test.rtl {
			t.Errorf("%s: levels %v, right to left %v; want %v, %v", test.name, levels, rightToLeft, test.want, test.rtl)
		}
	}
	if levels, _ := bidiLevels(document.Block{Runs: []document.Run{{Text: "Déjà vu — “quotes”"}}}, false); levels != nil {
		t.Errorf("left-to-right text has levels %v", levels)
	}
}

// Rule L2 reverses the runs of the highest levels first.
func TestReorder(t *testing.T) {
	letters := []rune("abcdefg")
	levels := []int8{0, 1, 1, 2, 2, 1, 0}
	indexes := []int{0, 1, 2, 3, 4, 5, 6}
	reorder(indexes, func(index int) int8 { return levels[index] })
	var got strings.Builder
	for _, index := range indexes {
		got.WriteRune(letters[index])
	}
	if got.String() != "afdecbg" {
		t.Errorf("reordered as %q, want afdecbg", got.String())
	}
}

// A right-to-left paragraph starts at the right: its first word is the rightmost, its first
// line's indent on the right; its lines keep their logical offsets.
func TestRightToLeftParagraph(t *testing.T) {
	faces := newTestFaces(t)
	content := leftSettings.content()
	text := strings.Repeat("שלום עולם ומה שלומך היום ", 8)
	doc := oneBlock(document.Block{Direction: document.RightToLeft, Indent: 200, Runs: []document.Run{{Text: text}}})
	pages, err := Paginate(doc, leftSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	lines := pages[0].Lines
	if len(lines) < 2 {
		t.Fatalf("%d lines, want several", len(lines))
	}
	if got := wordsLeftToRight(lines[0]); got[len(got)-1] != "שלום" || got[len(got)-2] != "עולם" {
		t.Errorf("first line from left to right %q, want שלום last", got)
	}
	indent := leftSettings.em(200)
	if _, right := lineEdges(faces, lines[0]); right < content.Max.X-indent-1 || right > content.Max.X-indent+1 {
		t.Errorf("first line ends at %d, want %d: indented from the right", right, content.Max.X-indent)
	}
	for index, line := range lines {
		rightmost := slices.MaxFunc(line.Words, func(first, second PlacedWord) int { return first.X - second.X })
		if right := rightmost.X + faces.width(rightmost.format(), rightmost.Text); index > 0 && (right < content.Max.X-1 || right > content.Max.X) {
			t.Errorf("line %d ends at %d, want the right margin %d", index, right, content.Max.X)
		}
		if index > 0 && line.Start != lines[index-1].End+1 {
			t.Errorf("line %d starts at %d, the one before ends at %d", index, line.Start, lines[index-1].End)
		}
		for _, word := range line.Words {
			if word.Shaping == nil || !word.Shaping.RightToLeft {
				t.Errorf("word %q not shaped right to left", word.Text)
			}
		}
	}
}

// In a left-to-right paragraph, Hebrew words go from right to left between the others; in a
// right-to-left one, a number and Latin words keep their order, numbers apart go its way.
func TestMixedDirections(t *testing.T) {
	faces := newTestFaces(t)
	for _, test := range []struct {
		block document.Block
		want  []string
	}{
		{document.Block{Runs: []document.Run{{Text: "one שתיים שלוש four"}}}, []string{"one", "שלוש", "שתיים", "four"}},
		{document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: "אחת 23 45 שתיים"}}}, []string{"שתיים", "45", "23", "אחת"}}, // numbers apart are in its order
		{document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: "אחת 2.5 שתיים"}}}, []string{"שתיים", "2.5", "אחת"}},
		{document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: "אחת two three שתיים"}}}, []string{"שתיים", "two", "three", "אחת"}},
		{document.Block{Runs: []document.Run{{Text: "a "}, {Text: "b c", Isolate: document.OverrideRightToLeft}, {Text: " d"}}}, []string{"a", "c", "b", "d"}},
	} {
		page := paginateOne(t, oneBlock(test.block), leftSettings, faces, noImages)
		if got := wordsLeftToRight(page.Lines[0]); !slices.Equal(got, test.want) {
			t.Errorf("%q: from left to right %q, want %q", test.block.Runs[0].Text, got, test.want)
		}
	}
}

// Arabic is shaped in the fallback that has it, its letters joined: a word is as wide as its
// shaped glyphs, narrower than its letters alone, and drawn.
func TestArabicShaped(t *testing.T) {
	faces := newTestFaces(t)
	word := "مرحبا"
	page := paginateOne(t, oneBlock(document.Block{Direction: document.RightToLeft, Runs: []document.Run{{Text: word + " بالعالم"}}}), leftSettings, faces, noImages)
	placed := wordAt(t, page, word)
	if placed.Shaping == nil || !placed.Shaping.RightToLeft {
		t.Fatalf("%q not shaped right to left: %+v", word, placed)
	}
	fallbacks, err := faces.fallbacksOf(fonts.Regular, 24)
	if err != nil {
		t.Fatal(err)
	}
	arabic := fallbacks[slices.IndexFunc(fallbacks, func(font *fonts.Font) bool { return font.Covers(word) })]
	shaped := fonts.WidthOf(arabic.Shape(word, true, ""))
	if got := faces.width(placed.format(), word); got != shaped || shaped >= arabic.Width(word) {
		t.Errorf("%q %d wide, shaped %d, its letters alone %d", word, got, shaped, arabic.Width(word))
	}
	canvas := whiteCanvas(leftSettings)
	if err := page.Draw(canvas, faces, nil); err != nil {
		t.Fatal(err)
	}
	if slices.Equal(canvas.Pix, whiteCanvas(leftSettings).Pix) {
		t.Error("nothing drawn")
	}
}

// FuzzBidiLayout lays out any text in any direction: the levels cover its runes, and its
// pages its offsets.
func FuzzBidiLayout(f *testing.F) {
	for _, text := range []string{"ab אב c", "אב 12", "\u2067ab\u2069 c", "مرحبا بالعالم", "\u202eab\u202c", "日本 עברית"} {
		f.Add(text, uint8(0), uint8(0))
	}
	faces := newTestFaces(f)
	f.Fuzz(func(t *testing.T, text string, direction, isolate uint8) {
		block := document.Block{Direction: document.Direction(direction % 3), Runs: []document.Run{{Text: "a "}, {Text: text, Isolate: document.Isolate(isolate % 6)}, {Text: " b"}}}
		levels, _ := bidiLevels(block, document.ScanScripts(block.Runs).RightToLeft)
		if levels != nil && len(levels) != block.Length() {
			t.Fatalf("%d levels for %d runes", len(levels), block.Length())
		}
		pages, err := Paginate(oneBlock(block), leftSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		if pages[0].Start != 0 || pages[len(pages)-1].End != block.Length() {
			t.Errorf("pages from %d to %d, want 0 to %d", pages[0].Start, pages[len(pages)-1].End, block.Length())
		}
	})
}
