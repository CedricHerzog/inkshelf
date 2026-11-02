package layout

import (
	"image"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// japaneseText is the opening of Natsume Sōseki's I Am a Cat (1905), in the public domain.
const japaneseText = "吾輩は猫である。名前はまだ無い。どこで生れたかとんと見当がつかぬ。何でも薄暗いじめじめした所でニャーニャー泣いていた事だけは記憶している。"

// verticalDocument returns paragraphs of Japanese text in mode, with runs (if any) in a last
// paragraph.
func verticalDocument(mode document.WritingMode, paragraphs int, runs ...document.Run) *document.Document {
	doc := &document.Document{Language: "ja", WritingMode: mode}
	for range paragraphs {
		doc.Add(document.Block{Kind: document.Paragraph, Indent: 100, WritingMode: mode, Runs: []document.Run{{Text: japaneseText}}})
	}
	if len(runs) > 0 {
		doc.Add(document.Block{Kind: document.Paragraph, WritingMode: mode, Runs: runs})
	}
	return doc
}

// The columns of a page are its lines: each column's words share its centre line.
func columnsOf(t *testing.T, page Page) []int {
	t.Helper()
	columns := make([]int, len(page.Lines))
	for index, line := range page.Lines {
		columns[index] = line.Words[0].X
		for _, word := range line.Words {
			if word.Mark == NoMark && word.X != columns[index] {
				t.Errorf("line %d: word %q at x %d, its column at %d", index, word.Text, word.X, columns[index])
			}
		}
	}
	return columns
}

// Vertical-rl text goes down in columns, from the right of the page to its left, which they fill
// before the next page starts; vertical-lr goes from the left to the right.
func TestVerticalColumns(t *testing.T) {
	faces := newTestFaces(t)
	content := testSettings.content()
	for _, mode := range []document.WritingMode{document.VerticalRL, document.VerticalLR} {
		doc := verticalDocument(mode, 8)
		pages, err := Paginate(doc, testSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) < 2 {
			t.Fatalf("mode %d: %d pages, want several", mode, len(pages))
		}
		checkCoverage(t, pages, doc)
		lineHeight := testSettings.FontSize * testSettings.LineSpacing / 100
		for index, page := range pages[:len(pages)-1] {
			columns := columnsOf(t, page)
			first, last := columns[0], columns[len(columns)-1]
			for position := 1; position < len(columns); position++ {
				if forward := columns[position] < columns[position-1]; forward != (mode == document.VerticalRL) {
					t.Errorf("mode %d, page %d: column %d at %d after one at %d", mode, index, position, columns[position], columns[position-1])
				}
			}
			if mode == document.VerticalRL && (first < content.Max.X-lineHeight || last > content.Min.X+lineHeight) ||
				mode == document.VerticalLR && (first > content.Min.X+lineHeight || last < content.Max.X-lineHeight) {
				t.Errorf("mode %d, page %d: columns from %d to %d, want across the text, %d to %d", mode, index, first, last, content.Min.X, content.Max.X)
			}
			for _, line := range page.Lines {
				for _, word := range line.Words {
					if word.Vertical != Upright || word.Top < content.Min.Y || word.Top+faces.width(word.format(), word.Text) > content.Max.Y {
						t.Errorf("mode %d, page %d: word %q (%v) from %d down, want upright within %d to %d", mode, index, word.Text, word.Vertical, word.Top, content.Min.Y, content.Max.Y)
					}
				}
			}
		}
	}
}

// wordNamed returns the first word of pages whose text is text.
func wordNamed(t *testing.T, pages []Page, text string) PlacedWord {
	t.Helper()
	if words := wordsOf(pages)[text]; len(words) > 0 {
		return words[0]
	}
	t.Fatalf("no word %q in %v", text, allLines(pages))
	return PlacedWord{}
}

// In mixed orientation, ideographs and kana stay upright and a Latin word is turned; upright
// orientation keeps the Latin word upright too, and sideways turns everything.
func TestVerticalOrientation(t *testing.T) {
	faces := newTestFaces(t)
	for _, test := range []struct {
		orientation    document.TextOrientation
		latin, kanji   VerticalText
		latinSeparated bool
	}{
		{document.OrientationMixed, Sideways, Upright, true},
		{document.OrientationUpright, Upright, Upright, false},
		{document.OrientationSideways, Sideways, Sideways, false},
	} {
		doc := verticalDocument(document.VerticalRL, 0, document.Run{Text: "猫はInkshelfで読む", Orientation: test.orientation})
		pages, err := Paginate(doc, testSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		words := map[string]VerticalText{}
		for _, word := range pages[0].Lines[0].Words {
			words[word.Text] = word.Vertical
		}
		if test.latinSeparated && (words["Inkshelf"] != test.latin || words["猫"] != test.kanji) {
			t.Errorf("orientation %d: words %v, want Inkshelf %v and 猫 %v", test.orientation, words, test.latin, test.kanji)
		}
		for text, form := range words {
			want := test.kanji
			if strings.ContainsAny(text, "Inkshelf") {
				want = test.latin
			}
			if form != want {
				t.Errorf("orientation %d: %q drawn %v, want %v", test.orientation, text, form, want)
			}
		}
	}
}

// text-combine-upright puts a year across one character's cell.
func TestVerticalCombine(t *testing.T) {
	faces := newTestFaces(t)
	doc := verticalDocument(document.VerticalRL, 0,
		document.Run{Text: "西暦"}, document.Run{Text: "2026", Combine: document.CombineAll}, document.Run{Text: "年"})
	pages, err := Paginate(doc, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	year, after := wordNamed(t, pages, "2026"), wordNamed(t, pages, "年")
	if year.Vertical != Combined || after.Top-year.Top != testSettings.FontSize {
		t.Errorf("2026 %v, %d high; want combined in one em, %d", year.Vertical, after.Top-year.Top, testSettings.FontSize)
	}
	canvas := whiteCanvas(testSettings)
	if err := pages[0].Draw(canvas, faces, nil, Elements{}); err != nil {
		t.Fatal(err)
	}
	cell := image.Rect(year.X-testSettings.FontSize/2, year.Top, year.X+testSettings.FontSize/2, after.Top)
	if !inkIn(canvas, cell) || inkIn(canvas, image.Rect(cell.Max.X+2, cell.Min.Y, cell.Max.X+testSettings.FontSize, cell.Max.Y)) {
		t.Errorf("no ink in the year's cell %v, or ink right of it", cell)
	}
}

// Emphasis marks go to the right of vertical text, or to its left if the book says so; and so
// do underlines.
func TestVerticalMarksAndUnderline(t *testing.T) {
	faces := newTestFaces(t)
	for _, mode := range []document.WritingMode{document.VerticalRL, document.VerticalLR} {
		for _, left := range []bool{false, true} {
			style := document.Underline
			if left {
				style |= document.UnderlineLeft
			}
			doc := verticalDocument(mode, 0, document.Run{Text: "猫である", Style: style, Emphasis: document.Emphasis{Mark: '﹅', Left: left}})
			pages, err := Paginate(doc, testSettings, faces, noImages)
			if err != nil {
				t.Fatal(err)
			}
			cat := wordNamed(t, pages, "猫")
			marks, stroke := 0, pages[0].Strokes[0]
			for _, word := range pages[0].Lines[0].Words {
				if word.Mark != SesameMark {
					continue
				}
				marks++
				if right := word.X > cat.X; right == left {
					t.Errorf("mode %d, left %v: mark at x %d, the text's column at %d", mode, left, word.X, cat.X)
				}
			}
			if right := stroke.Min.X > cat.X; marks != 4 || right == left {
				t.Errorf("mode %d, left %v: %d marks, underline %v, the column at %d", mode, left, marks, stroke.Rectangle, cat.X)
			}
		}
	}
}

// A horizontal table in a vertical chapter keeps its rows across, laid out as one box on the
// page, the vertical text going on around it.
func TestVerticalWithHorizontalTable(t *testing.T) {
	faces := newTestFaces(t)
	doc := &document.Document{Language: "ja", WritingMode: document.VerticalRL}
	doc.Add(document.Block{WritingMode: document.VerticalRL, Runs: []document.Run{{Text: "表の前。"}}})
	table := &document.Table{Rows: 2, Columns: 2}
	for row := range 2 {
		for column := range 2 {
			cell := &document.TableCell{Table: table, Row: row, Column: column, RowSpan: 1, ColumnSpan: 1}
			table.Cells = append(table.Cells, cell)
			doc.Add(document.Block{Cell: cell, WritingMode: document.HorizontalTB, Runs: []document.Run{{Text: []string{"Name", "Year", "Inkshelf", "2026"}[row*2+column]}}})
		}
	}
	doc.Add(document.Block{WritingMode: document.VerticalRL, Runs: []document.Run{{Text: "表の後。"}}})
	pages, err := Paginate(doc, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	checkCoverage(t, pages, doc)
	name, year, inkshelf := wordNamed(t, pages, "Name"), wordNamed(t, pages, "Year"), wordNamed(t, pages, "Inkshelf")
	if name.Vertical != Horizontal || year.Vertical != Horizontal || name.Top != year.Top || year.X <= name.X || inkshelf.Top <= name.Top || inkshelf.X != name.X {
		t.Errorf("Name %+v, Year %+v, Inkshelf %+v: want horizontal rows", name, year, inkshelf)
	}
	before, after := wordNamed(t, pages, "表"), wordsOf(pages)["表"][1]
	if before.Vertical != Upright || after.Vertical != Upright {
		t.Errorf("the text around the table %v, %v; want upright", before.Vertical, after.Vertical)
	}
	content := testSettings.content()
	if name.X < content.Min.X || name.Top < content.Min.Y {
		t.Errorf("the table at %d, %d, outside the text %v", name.X, name.Top, content)
	}
}

// The vertical forms of characters are Unicode's.
func TestVerticalOrientationOf(t *testing.T) {
	for character, want := range map[rune]verticalOrientation{
		'永': orientationUpright, 'あ': orientationUpright, '한': orientationUpright, 'A': orientationRotated,
		'1': orientationRotated, 'ー': orientationTransformedRotated, '。': orientationTransformedUpright,
		'「': orientationTransformedRotated, 'ا': orientationRotated, '😀': orientationUpright, 0x10FFFF: orientationRotated,
	} {
		if got := verticalOrientationOf(character); got != want {
			t.Errorf("%U: %d, want %d", character, got, want)
		}
	}
	for _, test := range []struct {
		run  document.Run
		want bool
	}{
		{document.Run{Text: "2026", Combine: document.CombineAll}, true},
		{document.Run{Text: "2026", Combine: 4}, true},
		{document.Run{Text: "2026", Combine: 2}, false},
		{document.Run{Text: "12a", Combine: 4}, false},
		{document.Run{Text: "12"}, false},
	} {
		if got := combines(test.run); got != test.want {
			t.Errorf("combines(%+v) = %v", test.run, got)
		}
	}
}

// A note shown in a popup is laid out across, even from a vertical book.
func TestHorizontalSetting(t *testing.T) {
	settings := testSettings
	settings.Horizontal = true
	pages, err := Paginate(verticalDocument(document.VerticalRL, 1), settings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if word := pages[0].Lines[0].Words[0]; word.Vertical != Horizontal {
		t.Errorf("word %+v, want horizontal", word)
	}
}
