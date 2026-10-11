package layout

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// tableDocument returns a paragraph, a table of rows rows (the first a header) whose cells'
// text is cellText(row, column), and a paragraph.
func tableDocument(rows, columns int, borders document.Edges, cellText func(row, column int) string) (*document.Document, *document.Table) {
	doc := &document.Document{}
	doc.Add(document.Block{Runs: []document.Run{{Text: "Before the table."}}})
	table := &document.Table{Rows: rows, Columns: columns, HeaderRows: 1, MarginTop: 50, MarginBottom: 50}
	for row := range rows {
		for column := range columns {
			cell := &document.TableCell{Table: table, Row: row, Column: column, RowSpan: 1, ColumnSpan: 1, Borders: borders}
			table.Cells = append(table.Cells, cell)
			for index, text := range strings.Split(cellText(row, column), "\n") {
				doc.Add(document.Block{Cell: cell, MarginTop: 100 * min(index, 1), Runs: []document.Run{{Text: text}}})
			}
		}
	}
	doc.Add(document.Block{Runs: []document.Run{{Text: "After the table."}}})
	return doc, table
}

// wordsOf returns the words of pages, by text, with the page they are on.
func wordsOf(pages []Page) map[string][]PlacedWord {
	words := map[string][]PlacedWord{}
	for _, page := range pages {
		for _, line := range page.Lines {
			for _, word := range line.Words {
				words[word.Text] = append(words[word.Text], word)
			}
		}
	}
	return words
}

// checkPages fails if pages don't start in order, or a page's lines (but a header repeated)
// are not between its start and end.
func checkPages(t *testing.T, pages []Page, header int) {
	t.Helper()
	for index, page := range pages {
		if index > 0 && page.Start < pages[index-1].Start {
			t.Errorf("page %d starts at %d, before page %d at %d", index, page.Start, index-1, pages[index-1].Start)
		}
		for _, line := range page.Lines {
			if line.Start >= header && (line.Start < page.Start || line.End > page.End) {
				t.Errorf("page %d (%d to %d) shows the line %d to %d", index, page.Start, page.End, line.Start, line.End)
			}
		}
	}
}

// A table's cells are side by side, in rows, its lines between them; the text around it
// above and below.
func TestTable(t *testing.T) {
	doc, _ := tableDocument(3, 3, document.AllEdges, func(row, column int) string { return fmt.Sprintf("r%dc%d", row, column) })
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	words := wordsOf(pages)
	for row := range 3 {
		for column := range 3 {
			word := words[fmt.Sprintf("r%dc%d", row, column)][0]
			if column > 0 {
				if left := words[fmt.Sprintf("r%dc%d", row, column-1)][0]; left.Top != word.Top || left.X >= word.X {
					t.Errorf("r%dc%d at %+v, after %+v", row, column, word, left)
				}
			}
			if row > 0 {
				if above := words[fmt.Sprintf("r%dc%d", row-1, column)][0]; above.X != word.X || above.Top >= word.Top {
					t.Errorf("r%dc%d at %+v, under %+v", row, column, word, above)
				}
			}
		}
	}
	if before, after, first, last := words["Before"][0], words["After"][0], words["r0c0"][0], words["r2c2"][0]; before.Top >= first.Top || after.Top <= last.Top {
		t.Errorf("the text around at %d and %d, the table from %d to %d", before.Top, after.Top, first.Top, last.Top)
	}
	// 4 edges a cell, inside the text.
	content := testSettings.content()
	if len(pages[0].Rules) != 4*9 {
		t.Errorf("%d rules, want %d", len(pages[0].Rules), 4*9)
	}
	for _, rule := range pages[0].Rules {
		if !rule.In(content) {
			t.Errorf("rule %v out of the text %v", rule, content)
		}
	}
	checkPages(t, pages, 0)
}

// A table longer than a page continues on the next, under its header repeated; a row higher
// than a page is cut across pages, every line shown once.
func TestTableAcrossPages(t *testing.T) {
	long := strings.Repeat("A long cell of text that goes on and on, line after line. ", 12)
	doc, _ := tableDocument(12, 2, document.AllEdges, func(row, column int) string {
		switch {
		case row == 0:
			return fmt.Sprintf("Header%d", column)
		case row == 5 && column == 1:
			return long + "\n" + long
		}
		return fmt.Sprintf("row%d", row)
	})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 3 {
		t.Fatalf("%d pages, want the table across several", len(pages))
	}
	header := doc.Blocks[3].Start // past the header's cells
	checkPages(t, pages, header)
	words := wordsOf(pages)
	if count := len(words["Header0"]); count != len(pages) {
		t.Errorf("the header on %d pages of %d", count, len(pages))
	}
	for row := range 12 {
		if row != 0 && row != 5 && len(words[fmt.Sprintf("row%d", row)]) != 2 {
			t.Errorf("row%d shown %d times, want once in each column", row, len(words[fmt.Sprintf("row%d", row)]))
		}
	}
	// Every line of the long cell once, each page's in its text.
	lines := map[int]int{}
	content := testSettings.content()
	for _, page := range pages {
		for _, line := range page.Lines {
			lines[line.Start]++
			for _, word := range line.Words {
				if word.Top < content.Min.Y || word.Top+testSettings.FontSize > content.Max.Y+testSettings.FontSize/2 {
					t.Errorf("%q at %d, out of the text %v", word.Text, word.Top, content)
				}
			}
		}
	}
	for start, count := range lines {
		if count > 1 && start >= header {
			t.Errorf("the line at %d shown %d times", start, count)
		}
	}
}

// A table too wide for the page shrinks its font, down to a floor; its lines never go past the
// text.
func TestTableShrinks(t *testing.T) {
	doc, _ := tableDocument(2, 6, document.AllEdges, func(row, column int) string { return fmt.Sprintf("Unbreakable%d", column) })
	faces := newTestFaces(t)
	pages, err := Paginate(doc, testSettings, faces, noImages)
	if err != nil {
		t.Fatal(err)
	}
	content := testSettings.content()
	for _, word := range wordsOf(pages)["Unbreakable5"] {
		if word.Size >= testSettings.FontSize || word.Size < testSettings.FontSize*minTableScale/100 {
			t.Errorf("size %d, want smaller than %d, %d%% at least", word.Size, testSettings.FontSize, minTableScale)
		}
		if right := word.X + faces.width(word.format(), word.Text); right > content.Max.X {
			t.Errorf("%q to %d, past the text's %d", word.Text, right, content.Max.X)
		}
	}
}

// Columns get what they ask for, then the others share the rest from their narrowest; narrow
// columns are as wide as they want; columns too wide shrink.
func TestColumnWidths(t *testing.T) {
	for _, test := range []struct {
		name                       string
		minimums, maximums, wanted []int
		available, target          int
		want                       []int
	}{
		{"fits", []int{10, 20}, []int{30, 40}, nil, 100, 0, []int{30, 40}},
		{"target", []int{10, 20}, []int{30, 30}, nil, 100, 90, []int{45, 45}},
		{"narrow first", []int{50, 10}, []int{500, 20}, nil, 100, 0, []int{80, 20}},
		{"between", []int{10, 10}, []int{200, 200}, nil, 100, 0, []int{50, 50}},
		{"asked", []int{10, 10}, []int{200, 200}, []int{25}, 100, 0, []int{25, 75}},
		{"all asked", []int{5, 5}, []int{10, 10}, []int{20, 30}, 100, 0, []int{20, 30}},
		{"too wide", []int{100, 100}, []int{200, 200}, nil, 100, 0, []int{50, 50}},
	} {
		if got := columnWidths(test.minimums, test.maximums, test.wanted, test.available, test.target); !slices.Equal(got, test.want) {
			t.Errorf("%s: %v, want %v", test.name, got, test.want)
		}
	}
}

// A table in a cell is laid out in it; a cell spanning columns and rows covers them.
func TestNestedTableAndSpans(t *testing.T) {
	doc := &document.Document{}
	outer := &document.Table{Rows: 2, Columns: 2}
	spanning := &document.TableCell{Table: outer, RowSpan: 2, ColumnSpan: 1, Borders: document.AllEdges}
	right := &document.TableCell{Table: outer, Column: 1, RowSpan: 1, ColumnSpan: 1, Borders: document.AllEdges}
	below := &document.TableCell{Table: outer, Row: 1, Column: 1, RowSpan: 1, ColumnSpan: 1, Borders: document.AllEdges}
	outer.Cells = []*document.TableCell{spanning, right, below}
	inner := &document.Table{Rows: 1, Columns: 2, Parent: right}
	innerLeft := &document.TableCell{Table: inner, RowSpan: 1, ColumnSpan: 1}
	innerRight := &document.TableCell{Table: inner, Column: 1, RowSpan: 1, ColumnSpan: 1}
	inner.Cells = []*document.TableCell{innerLeft, innerRight}
	doc.Add(document.Block{Cell: spanning, Runs: []document.Run{{Text: "Tall"}}})
	doc.Add(document.Block{Cell: right, Runs: []document.Run{{Text: "Above"}}})
	doc.Add(document.Block{Cell: innerLeft, Runs: []document.Run{{Text: "InnerLeft"}}})
	doc.Add(document.Block{Cell: innerRight, Runs: []document.Run{{Text: "InnerRight"}}})
	doc.Add(document.Block{Cell: below, Runs: []document.Run{{Text: "Below"}}})
	pages, err := Paginate(doc, testSettings, newTestFaces(t), noImages)
	if err != nil {
		t.Fatal(err)
	}
	words := wordsOf(pages)
	tall, above, left, innerRightWord, under := words["Tall"][0], words["Above"][0], words["InnerLeft"][0], words["InnerRight"][0], words["Below"][0]
	if above.X <= tall.X || left.Top <= above.Top || left.X < above.X || innerRightWord.Top != left.Top || innerRightWord.X <= left.X {
		t.Errorf("Tall %+v, Above %+v, the inner table %+v %+v", tall, above, left, innerRightWord)
	}
	if under.Top <= left.Top || under.X != above.X {
		t.Errorf("Below %+v, want under the inner table %+v", under, left)
	}
	checkPages(t, pages, 0)
}
