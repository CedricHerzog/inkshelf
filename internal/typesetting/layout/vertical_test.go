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

// A box in vertical text has its sides as the lines go: its top border before its first column,
// drawn on the page's right in vertical-rl and on its left in vertical-lr, its left border at the
// top, where the columns start.
func TestVerticalBox(t *testing.T) {
	faces := newTestFaces(t)
	for _, mode := range []document.WritingMode{document.VerticalRL, document.VerticalLR} {
		doc := verticalDocument(mode, 3)
		doc.Boxes = []document.BorderBox{{
			Start: doc.Blocks[1].Start, End: doc.Blocks[2].Start,
			Top: solid(25, 0), Left: solid(50, 0x40), PaddingTop: 100,
		}}
		pages, err := Paginate(doc, testSettings, faces, noImages)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages[0].Boxes) != 1 {
			t.Fatalf("mode %d: boxes %+v, want 1", mode, pages[0].Boxes)
		}
		placed := pages[0].Boxes[0]
		before, other := placed.Right, placed.Left
		if mode == document.VerticalLR {
			before, other = placed.Left, placed.Right
		}
		if before.Style != document.Solid || other.Style != document.NoBorder || placed.Top.Gray != 0x40 || placed.Bottom.Style != document.NoBorder {
			t.Errorf("mode %d: box sides top %+v right %+v bottom %+v left %+v", mode, placed.Top, placed.Right, placed.Bottom, placed.Left)
		}
		// The first column in the box is inside its border and padding, 6 and 24 pixels, by half
		// its width: its words' X is its centre.
		column := 0
		for _, line := range pages[0].Lines {
			if line.Start >= doc.Blocks[1].Start {
				column = line.Words[0].X
				break
			}
		}
		inset := placed.Rect.Max.X - column
		if mode == document.VerticalLR {
			inset = column - placed.Rect.Min.X
		}
		if inset < 6+24 || inset > 6+24+testSettings.FontSize {
			t.Errorf("mode %d: first column at x %d, %d inside the box's edge before it, want %d and half a column", mode, column, inset, 6+24)
		}
	}
}

// A table that stays vertical in a vertical chapter has its rows as columns, from the right:
// a cell's top border, before it, is drawn down its right side, and its left one, at the start
// of its lines, across its top.
func TestVerticalTableBorders(t *testing.T) {
	faces := newTestFaces(t)
	doc := &document.Document{Language: "ja", WritingMode: document.VerticalRL}
	table := &document.Table{Rows: 1, Columns: 1}
	cell := &document.TableCell{Table: table, RowSpan: 1, ColumnSpan: 1, Borders: document.EdgeTop | document.EdgeLeft}
	table.Cells = append(table.Cells, cell)
	doc.Add(document.Block{Cell: cell, WritingMode: document.VerticalRL, Runs: []document.Run{{Text: "縦の表"}}})
	pages, err := Paginate(doc, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	rules := pages[0].Rules
	if len(rules) != 2 {
		t.Fatalf("rules %v, want the cell's two sides", rules)
	}
	column := wordNamed(t, pages, "縦").X
	var right, top bool
	for _, rule := range rules {
		right = right || rule.Dy() > rule.Dx() && rule.Min.X > column
		top = top || rule.Dx() > rule.Dy() && rule.Max.Y <= wordNamed(t, pages, "縦").Top
	}
	if !right || !top {
		t.Errorf("rules %v around a column at x %d: want one down its right, one across its top", rules, column)
	}
}

// MathML lays a formula out across in any writing mode: in vertical text a formula stands
// upright, as wide and high as across, an inline one centred on its column and taking its
// height down it, a displayed one centred down the page between the columns.
func TestVerticalFormulas(t *testing.T) {
	fraction := formulaPath(t, `<math><mfrac><mn>1</mn><mn>6</mn></mfrac></math>`)
	display := formulaPath(t, `<math display="block"><msup><mi>a</mi><mn>2</mn></msup><mo>+</mo><msup><mi>b</mi><mn>2</mn></msup><mo>=</mo><msup><mi>c</mi><mn>2</mn></msup></math>`)
	doc := &document.Document{Language: "ja", WritingMode: document.VerticalRL}
	doc.Add(document.Block{WritingMode: document.VerticalRL, Runs: []document.Run{{Text: "半分は"}, {Image: fraction}, {Text: "である。"}}})
	doc.Add(document.Block{Kind: document.Image, Image: display, WritingMode: document.VerticalRL})
	doc.Add(document.Block{WritingMode: document.VerticalRL, Runs: []document.Run{{Text: "次の段落。"}}})
	faces := newTestFaces(t)
	pages, err := Paginate(doc, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 2 {
		t.Fatalf("pages %+v, want one with two formulas", pages)
	}
	page := pages[0]
	for index, path := range []string{fraction, display} {
		placed := page.Images[index]
		formula, _, err := faces.formula(path, placed.MathSize, image.Pt(1<<20, 1<<20))
		if err != nil {
			t.Fatal(err)
		}
		if placed.Rect.Dx() != formula.Width || placed.Rect.Dy() != formula.Ascent+formula.Descent {
			t.Errorf("formula %d in %v, want it upright, %dx%d", index, placed.Rect, formula.Width, formula.Ascent+formula.Descent)
		}
	}
	inline, block := page.Images[0], page.Images[1]
	before, after := wordNamed(t, pages, "は"), wordNamed(t, pages, "で")
	if centre := (inline.Rect.Min.X + inline.Rect.Max.X) / 2; centre-before.X > 1 || before.X-centre > 1 || inline.Rect.Max.Y > after.Top {
		t.Errorf("inline formula at %v, its column at x %d, the text after from y %d: want it centred, before the text", inline.Rect, before.X, after.Top)
	}
	content := testSettings.content()
	if top, bottom := block.Rect.Min.Y-content.Min.Y, content.Max.Y-block.Rect.Max.Y; top-bottom > 1 || bottom-top > 1 {
		t.Errorf("displayed formula at %v, want it centred down the text %v", block.Rect, content)
	}
	next := wordNamed(t, pages, "次")
	if block.Rect.Min.X >= before.X || block.Rect.Max.X <= next.X {
		t.Errorf("displayed formula from x %d to %d, want it between the columns at %d and %d", block.Rect.Min.X, block.Rect.Max.X, before.X, next.X)
	}
}
