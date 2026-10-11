package epub

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
)

// cellText returns the text of doc's blocks in each cell, by cell.
func cellText(doc *document.Document) map[*document.TableCell]string {
	byCell := map[*document.TableCell]string{}
	for _, block := range doc.Blocks {
		if block.Cell == nil {
			continue
		}
		for _, run := range block.Runs {
			byCell[block.Cell] += run.Text
		}
	}
	return byCell
}

// A table's rows and cells make a grid: header rows, spans, a th bold and centred, the
// border attribute as lines, cellpadding where CSS gives none; each cell's blocks are its
// own, with no space between them.
func TestTableCells(t *testing.T) {
	doc := styledChapter(t, `td.padded { padding: 1em 0 } td.ruled { border-bottom: 1px solid black }`, `<p>Before</p>
<table border="1" cellpadding="8" style="margin-top: 2em; width: 50%">
<caption>Caption</caption>
<colgroup><col width="25%"/><col span="2"/></colgroup>
<thead><tr><th>Name</th><th colspan="2">Values</th></tr></thead>
<tbody>
<tr><td rowspan="2">Tall</td><td class="padded">a</td><td valign="bottom">b</td></tr>
<tr><td>c</td><td class="ruled"><p>d</p><p>e</p></td></tr>
</tbody></table><p>After</p>`)
	if got := texts(doc); got[0] != "Before" || got[1] != "Caption" || got[len(got)-1] != "After" {
		t.Fatalf("blocks %q", got)
	}
	if caption := doc.Blocks[1]; caption.Cell != nil || caption.MarginTop != 200 || caption.Align != document.AlignCenter {
		t.Errorf("caption %+v, want a block above the table with its margin", caption)
	}
	var cells []*document.TableCell
	for _, block := range doc.Blocks {
		if block.Cell != nil && (len(cells) == 0 || cells[len(cells)-1] != block.Cell) {
			cells = append(cells, block.Cell)
		}
	}
	if len(cells) != 7 {
		t.Fatalf("%d cells, want 7", len(cells))
	}
	table := cells[0].Table
	if table.Rows != 3 || table.Columns != 3 || table.HeaderRows != 1 || len(table.Cells) != 7 || table.Width != 50 ||
		table.MarginTop != 0 || table.Frame != document.AllEdges || len(table.ColumnWidths) != 3 || table.ColumnWidths[0] != 25 {
		t.Errorf("table %+v", table)
	}
	want := []struct {
		text                             string
		row, column, rowSpan, columnSpan int
	}{
		{"Name", 0, 0, 1, 1}, {"Values", 0, 1, 1, 2}, {"Tall", 1, 0, 2, 1}, {"a", 1, 1, 1, 1}, {"b", 1, 2, 1, 1},
		{"c", 2, 1, 1, 1}, {"de", 2, 2, 1, 1},
	}
	texts := cellText(doc)
	for index, cell := range cells {
		if wanted := want[index]; texts[cell] != wanted.text || cell.Row != wanted.row || cell.Column != wanted.column ||
			cell.RowSpan != wanted.rowSpan || cell.ColumnSpan != wanted.columnSpan || cell.Borders != document.AllEdges {
			t.Errorf("cell %d: %q %+v, want %+v", index, texts[cell], cell, wanted)
		}
	}
	if header := doc.Blocks[2]; header.Align != document.AlignCenter || header.Runs[0].Style != document.Bold {
		t.Errorf("th: %+v, want bold and centred", header)
	}
	if cells[0].PaddingLeft != 50 || cells[3].PaddingTop != 100 || cells[3].PaddingLeft != 0 {
		t.Errorf("paddings: cellpadding %+v, CSS %+v", cells[0], cells[3])
	}
	if cells[4].VerticalAlign != document.VerticalBottom || cells[3].VerticalAlign != document.VerticalMiddle {
		t.Errorf("vertical alignments %d and %d, want valign's and the row's middle", cells[4].VerticalAlign, cells[3].VerticalAlign)
	}
	// The runes of a cell follow the previous cell's: no space between them.
	for index, block := range doc.Blocks[1:] {
		previous := doc.Blocks[index]
		if block.Start != previous.Start+len([]rune(previous.Runs[0].Text)) && len(previous.Runs) == 1 {
			t.Errorf("block %q at %d, after %q at %d", block.Runs[0].Text, block.Start, previous.Runs[0].Text, previous.Start)
		}
	}
}

// Text in a table out of its cells, or a table in one, make the next cells another table; a
// table in a cell is in that cell; tables nested too deep are blocks. The table's margins
// come from the elements around it.
func TestTableStructure(t *testing.T) {
	nested := `<table><tr><td>x</td></tr></table>`
	for range maxTableDepth {
		nested = `<table><tr><td>in` + nested + `</td></tr></table>`
	}
	doc := styledChapter(t, `div.around { margin-bottom: 3em }`, `<div class="around"><table><tr><td>one</td></tr>stray<tr><td>two</td></tr></table></div>
<p>Between</p>`+nested)
	byText := map[string]*document.TableCell{}
	for _, block := range doc.Blocks {
		byText[block.Runs[0].Text] = block.Cell
	}
	one, two := byText["one"], byText["two"]
	if one == nil || two == nil || one.Table == two.Table || byText["stray"] != nil || byText["Between"] != nil {
		t.Fatalf("cells %+v %+v, stray in %+v", one, two, byText["stray"])
	}
	if one.Table.MarginBottom != 0 || two.Table.MarginBottom != 300 || two.Row != 0 {
		t.Errorf("tables %+v and %+v: the second ends the div", one.Table, two.Table)
	}
	inner := byText["x"]
	if inner == nil {
		t.Fatalf("no x in %q", texts(doc))
	}
	depth := 0
	for table := inner.Table; table.Parent != nil; table = table.Parent.Table {
		depth++
	}
	if depth != maxTableDepth-1 {
		t.Errorf("x %d tables deep, want %d: the deepest is blocks", depth+1, maxTableDepth)
	}
}

// display: table makes a table of any element; too many columns go into the last; a cell out
// of a table is text after a space, as before tables.
func TestTableDisplay(t *testing.T) {
	var row strings.Builder
	for range document.MaxTableColumns + 3 {
		row.WriteString(`<span class="cell">c</span>`)
	}
	doc := styledChapter(t, `.grid { display: table } .line { display: table-row } .cell { display: table-cell }`,
		`<div class="grid"><div class="line">`+row.String()+`</div></div><p><span class="cell">left</span><span class="cell">right</span></p>`)
	var last *document.TableCell
	for _, block := range doc.Blocks {
		if block.Cell != nil {
			last = block.Cell
		}
	}
	if last == nil || last.Table.Columns != document.MaxTableColumns || len(last.Table.Cells) != document.MaxTableColumns {
		t.Fatalf("last cell %+v", last)
	}
	if text := cellText(doc)[last]; text != "cccc" {
		t.Errorf("the last cell holds %q, want the cells past the last column", text)
	}
	if got := texts(doc); got[len(got)-1] != "left right" {
		t.Errorf("cells out of a table: %q", got)
	}
}

// A table laid out shows its cells side by side, and its links work.
func TestTableLaidOut(t *testing.T) {
	book := openStyled(t, ``, `<table border="1"><tr><td>Left</td><td><a href="#target">Right</a></td></tr></table><p id="target">After</p>`, nil)
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	faces := layout.NewFaces(0)
	defer faces.Close()
	settings := layout.Settings{Width: 600, Height: 800, Margins: image.Rect(20, 20, 20, 20), FontSize: 24, LineSpacing: 130}
	pages, err := layout.Paginate(doc, settings, faces, book.ImageSize)
	if err != nil {
		t.Fatal(err)
	}
	words := map[string]layout.PlacedWord{}
	for _, line := range pages[0].Lines {
		for _, word := range line.Words {
			words[word.Text] = word
		}
	}
	if left, right := words["Left"], words["Right"]; left.Top != right.Top || left.X >= right.X {
		t.Errorf("Left %+v and Right %+v, want side by side", left, right)
	}
	if len(pages[0].Links) != 1 || pages[0].Rules == nil {
		t.Errorf("links %+v, %d rules", pages[0].Links, len(pages[0].Rules))
	}
}

// Whatever the markup, a chapter is laid out, its pages in order.
func FuzzTables(f *testing.F) {
	for _, seed := range []string{
		`<table border="1"><thead><tr><th>a</th></tr></thead><tr><td rowspan="3" colspan="2">b</td><td>c</td></tr></table>`,
		`<table><tr><td><table><tr><td>x<table><tr><td>y</td></tr></table></td></tr></table></td></tr>z<td>w</td></table>`,
		`<div style="display: table"><p style="display: table-cell">a</p><table><td rowspan="0">b</td></table></div>`,
		`<table><caption>c</caption><col span="40" width="10%"/><tr><td colspan="999">d</td><td>e<br/>f</td></tr></table>`,
	} {
		f.Add(seed)
	}
	faces := layout.NewFaces(0)
	defer faces.Close()
	settings := layout.Settings{Width: 300, Height: 200, Margins: image.Rect(10, 10, 10, 10), FontSize: 16, LineSpacing: 130, Justify: true}
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 4000 {
			return
		}
		book := openStyled(t, ``, body, nil)
		started := time.Now()
		doc, err := book.Chapter(0)
		if err != nil {
			return
		}
		pages, err := layout.Paginate(doc, settings, faces, book.ImageSize)
		if err != nil {
			t.Fatal(err)
		}
		if took := time.Since(started); took > 2*time.Second {
			t.Fatalf("laid out in %v", took)
		}
		for index, page := range pages {
			if page.Start > page.End || index > 0 && page.Start < pages[index-1].Start {
				t.Fatalf("page %d: %d to %d, after %d", index, page.Start, page.End, pages[index-1].Start)
			}
		}
	})
}
