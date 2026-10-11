package epub

import (
	"slices"
	"strconv"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// A table's elements (or any element whose display is a table's part) make a document.Table: a
// grid of cells, each holding blocks of its own. The builder follows the rows and cells as they
// open, works out which column each cell goes in (rowspan and colspan cover several), and
// gives the blocks of a cell's text that cell. Tables nested too deep, and tables on a
// pre-paginated page, are flat: their cells' text simply follows one another.

// Limits on the tables laid out as grids, against books that would take forever to lay out.
const (
	// maxTableDepth is how deep tables nest in cells: a deeper one is flat.
	maxTableDepth = 3
	// maxTableRows is the most rows of a table: the rows after them make another table.
	maxTableRows = 1000
)

// tableParts are the display values of a table's parts.
var tableParts = map[string]bool{
	"table": true, "table-row": true, "table-row-group": true, "table-header-group": true,
	"table-footer-group": true, "table-cell": true, "table-column": true, "table-column-group": true,
}

// tableBuilder is an open table element: the document.Table its cells go in, and where the next
// cell goes.
type tableBuilder struct {
	element *element
	// flat is set for a table laid out as blocks: nested too deep, or on a pre-paginated page.
	flat bool
	// parentCell is the cell the table is in, nil if none.
	parentCell *document.TableCell
	// table is the table the next cells go in: nil before the first, and after content out of
	// the cells, which starts another.
	table *document.Table
	// row is the element's current row, from 0, -1 before the first; rowBase the rows before
	// table's first.
	row, rowBase int
	rowOpen      bool
	// headerGroup is set in a table-header-group; headerRow on a row of the header, before the
	// first row of the body.
	headerGroup, headerRow, bodyStarted bool
	rowEdges                            document.Edges // the borders of the row, on its cells
	// spans are, by column, the rows a cell still covers, the current one included.
	spans  []int
	column int // the next cell's
	// cell is the open cell, of cellElement; lastCell the row's last.
	cell        *document.TableCell
	cellElement *element
	lastCell    *document.TableCell
	// columnWidths are those of the <col>s, in percent; nextColumn the next <col>'s column.
	columnWidths []int
	nextColumn   int
	// grid draws lines between all the cells: the table's border attribute; cellPadding is its
	// cellpadding attribute, in hundredths of an em, -1 if none.
	grid        bool
	cellPadding int
	frame       document.Edges
	width       int // in percent of the text's, 0 for auto
}

// tableHints adds to declared what the attributes of a table's elements say, where the book's
// CSS says nothing (HTML's presentational hints), and the user agent's defaults that depend on
// it: a cell's vertical-align is its row's, a row's middle; a th is centred.
func tableHints(opened *element, declared map[string]string) map[string]string {
	name := opened.name
	switch name {
	case "table", "tr", "td", "th", "thead", "tbody", "tfoot", "col", "colgroup":
	default:
		return declared
	}
	hint := func(property, value string) {
		if _, ok := declared[property]; ok || value == "" {
			return
		}
		if declared == nil {
			declared = map[string]string{}
		}
		declared[property] = value
	}
	attributes := opened.attributes
	if width := strings.TrimSpace(attributes["width"]); width != "" && name != "tr" {
		if _, err := strconv.Atoi(width); err == nil {
			width += "px"
		}
		hint("width", width)
	}
	if name == "table" || name == "col" || name == "colgroup" {
		return declared
	}
	hint("text-align", strings.ToLower(attributes["align"]))
	hint("vertical-align", strings.ToLower(attributes["valign"]))
	switch name {
	case "td", "th":
		hint("vertical-align", "inherit")
		if name == "th" {
			hint("text-align", "center")
		}
	default:
		hint("vertical-align", "middle")
	}
	return declared
}

// openTablePart opens an element whose display is a table's part, declared its CSS: a table,
// a row, a cell, a column. Out of a table, or in a flat one, a row is a block and a cell's text
// follows the previous one's after a space.
func (parser *builder) openTablePart(opened *element, declared map[string]string) {
	display := opened.style.display
	if display == "table" {
		parser.openTable(opened, declared)
		return
	}
	var current *tableBuilder
	if count := len(parser.tables); count > 0 && !parser.tables[count-1].flat && parser.tables[count-1].cell == nil {
		current = parser.tables[count-1]
	}
	if current == nil {
		switch display {
		case "table-cell":
			parser.spaceNeeded, parser.spaceRun = parser.pendingRunes > 0, opened.parent.run
		case "table-row", "table-row-group", "table-header-group", "table-footer-group":
			parser.openBlock(opened)
		}
		return
	}
	switch display {
	case "table-row-group", "table-header-group", "table-footer-group":
		current.rowOpen, current.headerGroup = false, display == "table-header-group"
	case "table-row":
		parser.startRow(current, declared)
	case "table-cell":
		parser.openCell(current, opened, declared)
	case "table-column":
		parser.addColumns(current, opened)
	}
}

// openTable opens a table element: a block, whose rows and cells make a document.Table.
func (parser *builder) openTable(opened *element, declared map[string]string) {
	parser.openBlock(opened)
	current := &tableBuilder{element: opened, row: -1, cellPadding: -1}
	if count := len(parser.tables); count > 0 {
		outer := parser.tables[count-1]
		current.flat = outer.flat || count >= maxTableDepth
		if outer.cell == nil {
			outer.table = nil // a table out of the cells: the next ones make another
		}
		current.parentCell = outer.cell
		if outer.cell == nil {
			current.parentCell = outer.parentCell
		}
	}
	current.flat = current.flat || parser.page != nil
	parser.tables = append(parser.tables, current)
	if current.flat {
		return
	}
	if border, ok := opened.attributes["border"]; ok {
		width, err := strconv.Atoi(strings.TrimSpace(border))
		current.grid = err != nil || width > 0 // border="" is a border
	}
	if padding, err := strconv.Atoi(strings.TrimSpace(opened.attributes["cellpadding"])); err == nil {
		current.cellPadding = max(padding, 0) * 100 / pixelsPerEm
	}
	current.frame = edgesOf(declared)
	if current.grid {
		current.frame = document.AllEdges
	}
	current.width = min(opened.style.width*100/textWidth, 100)
}

// startRow starts a row of current, whose CSS is declared (nil for a row that cells start).
func (parser *builder) startRow(current *tableBuilder, declared map[string]string) {
	parser.flush()
	current.row++
	current.rowOpen, current.column, current.lastCell = true, 0, nil
	current.rowEdges = edgesOf(declared) & (document.EdgeTop | document.EdgeBottom)
	for column, span := range current.spans {
		current.spans[column] = max(span-1, 0)
	}
	if current.table != nil && current.row-current.rowBase >= maxTableRows {
		current.table = nil
	}
	current.headerRow = current.headerGroup && !current.bodyStarted
	current.bodyStarted = current.bodyStarted || !current.headerGroup
}

// segment returns the table current's next cells go in, started if needed.
func (parser *builder) segment(current *tableBuilder) *document.Table {
	if current.table == nil {
		props := current.element.block
		current.table = &document.Table{
			Width: current.width, ColumnWidths: slices.Clone(current.columnWidths), Left: props.left, Right: props.right,
			Frame: current.frame, Parent: current.parentCell,
		}
		current.rowBase, current.spans = current.row, nil
	}
	return current.table
}

// openCell opens a cell of current, whose CSS is declared: the blocks of its text go in it, at
// the next free column. A cell past the last column the app lays out goes in the row's last
// cell.
func (parser *builder) openCell(current *tableBuilder, opened *element, declared map[string]string) {
	parser.flush()
	if !current.rowOpen {
		parser.startRow(current, nil)
	}
	table := parser.segment(current)
	computed := &opened.style
	opened.block = cellBlockProps(computed)
	current.cellElement = opened
	column := current.nextFreeColumn()
	if column >= document.MaxTableColumns && current.lastCell != nil {
		current.cell = current.lastCell
		return
	}
	column = min(column, document.MaxTableColumns-1)
	columns := min(max(spanOf(opened.attributes["colspan"]), 1), document.MaxTableColumns-column)
	cell := &document.TableCell{
		Table: table, Row: current.row - current.rowBase, Column: column, ColumnSpan: columns,
		RowSpan:       min(max(spanOf(opened.attributes["rowspan"]), 1), document.MaxTableRowSpan),
		VerticalAlign: verticalAlignOf(computed.verticalAlign), Borders: edgesOf(declared) | current.rowEdges,
	}
	current.padCell(cell, computed, declared)
	if current.grid {
		cell.Borders = document.AllEdges
	}
	current.coverColumns(column, columns, cell.RowSpan)
	table.Columns, table.Rows = max(table.Columns, column+columns), max(table.Rows, cell.Row+1)
	if current.headerRow {
		table.HeaderRows = max(table.HeaderRows, cell.Row+1)
	}
	if columns == 1 && computed.width > 0 {
		askColumnWidth(table, column, computed.width)
	}
	table.Cells = append(table.Cells, cell)
	current.cell, current.lastCell = cell, cell
}

// cellBlockProps returns the properties of the blocks of a cell's text. A cell's margins are
// its padding, which the cell itself draws, so its blocks start at its edges.
func cellBlockProps(computed *style) *blockProps {
	return &blockProps{
		kind: document.Paragraph, align: computed.align, alignLast: computed.alignLast, fontSize: computed.fontSize,
		lineHeight: computed.lineHeight, hyphenate: computed.hyphenate, indent: min(max(computed.indent, 0), maxMargin),
		direction: blockDirectionOf(computed), wordBreak: computed.wordBreak, lineBreak: computed.lineBreak,
	}
}

// nextFreeColumn returns the first column from the current one that no cell of the rows above
// still covers.
func (current *tableBuilder) nextFreeColumn() int {
	column := current.column
	for column < len(current.spans) && current.spans[column] > 0 {
		column++
	}
	return column
}

// padCell gives a cell its padding: its CSS padding (computed with the margins, which a cell
// has none of), else the table's cellpadding attribute.
func (current *tableBuilder) padCell(cell *document.TableCell, computed *style, declared map[string]string) {
	for _, side := range []struct {
		padding  *int
		property string
		computed int
	}{
		{&cell.PaddingTop, "padding-top", computed.marginTop}, {&cell.PaddingRight, "padding-right", computed.marginRight},
		{&cell.PaddingBottom, "padding-bottom", computed.marginBottom}, {&cell.PaddingLeft, "padding-left", computed.marginLeft},
	} {
		*side.padding = max(side.computed, 0)
		if _, ok := declared[side.property]; !ok && current.cellPadding >= 0 {
			*side.padding = current.cellPadding
		}
	}
}

// coverColumns records that a cell covers columns from column for rowSpan rows, and moves the
// next cell past it.
func (current *tableBuilder) coverColumns(column, columns, rowSpan int) {
	for len(current.spans) < column+columns {
		current.spans = append(current.spans, 0)
	}
	for index := range columns {
		current.spans[column+index] = rowSpan
	}
	current.column = column + columns
}

// askColumnWidth gives a column of table the width that one of its cells asks for (in
// hundredths of an em), unless an earlier cell or a <col> asked first.
func askColumnWidth(table *document.Table, column, width int) {
	for len(table.ColumnWidths) <= column {
		table.ColumnWidths = append(table.ColumnWidths, 0)
	}
	if table.ColumnWidths[column] == 0 {
		table.ColumnWidths[column] = min(max(width*100/textWidth, 1), 100)
	}
}

// spanOf returns a colspan or rowspan attribute's number, 1 if none.
func spanOf(value string) int {
	span, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 1
	}
	return span
}

// addColumns reads a <col> of current: the width it asks for its columns.
func (parser *builder) addColumns(current *tableBuilder, col *element) {
	columns := min(max(spanOf(col.attributes["span"]), 1), document.MaxTableColumns-current.nextColumn)
	percent := 0
	if col.style.width > 0 {
		percent = min(max(col.style.width*100/textWidth, 1), 100)
	}
	for range columns {
		current.columnWidths = append(current.columnWidths, percent)
	}
	current.nextColumn += columns
}

// closeTablePart follows the closing of an element: a table's, a row's or a cell's, reporting
// whether it was one (laid out as a grid).
func (parser *builder) closeTablePart(closed *element) bool {
	count := len(parser.tables)
	if count == 0 {
		return false
	}
	current := parser.tables[count-1]
	switch {
	case closed == current.element:
		parser.tables = parser.tables[:count-1]
		if current.flat {
			return false
		}
		if current.table != nil {
			current.table.MarginBottom = max(current.table.MarginBottom, closed.block.marginBottom)
		}
		return true
	case current.flat:
		return false
	case closed == current.cellElement:
		current.cell, current.cellElement = nil, nil
		return true
	case current.cell != nil:
		return false
	}
	switch closed.style.display {
	case "table-row":
		current.rowOpen = false
		return true
	case "table-row-group", "table-header-group", "table-footer-group":
		current.rowOpen, current.headerGroup = false, false
		return true
	}
	return false
}

// currentCell returns the cell the text being built is in, nil if none. Text in a table out
// of its cells makes the next cells another table.
func (parser *builder) currentCell() *document.TableCell {
	for _, current := range slices.Backward(parser.tables) {
		if current.flat {
			continue
		}
		if current.cell != nil {
			return current.cell
		}
		current.table = nil
		return current.parentCell
	}
	return nil
}

// addBlock adds block to the document, in the cell it is in. The first block of a table gives
// the table its margin and page break.
func (parser *builder) addBlock(block document.Block) {
	if len(parser.tables) > 0 {
		block.Cell = parser.currentCell()
	}
	if block.Cell != nil {
		var first *document.Table // the outermost table it starts
		for cell := block.Cell; cell != nil; cell = cell.Table.Parent {
			if !parser.startedTables[cell.Table] {
				first = cell.Table
			}
		}
		if first != nil {
			if parser.startedTables == nil {
				parser.startedTables = map[*document.Table]bool{}
			}
			for cell := block.Cell; cell != nil; cell = cell.Table.Parent {
				parser.startedTables[cell.Table] = true
			}
			first.MarginTop = max(first.MarginTop, block.MarginTop)
			first.PageBreak = first.PageBreak || block.PageBreak
			block.MarginTop, block.PageBreak, block.BorderTop = 0, false, false
		}
	}
	parser.doc.Add(block)
}

// closedTableOf returns the outermost table closed that block is in, nil if none: the margins
// of the elements closed after it go to that table.
func (parser *builder) closedTableOf(block document.Block) *document.Table {
	var table *document.Table
	for cell := block.Cell; cell != nil && !parser.cellOpen(cell); cell = cell.Table.Parent {
		table = cell.Table
	}
	return table
}

// cellOpen reports whether cell's element is open.
func (parser *builder) cellOpen(cell *document.TableCell) bool {
	return slices.ContainsFunc(parser.tables, func(current *tableBuilder) bool { return current.cell == cell })
}

// edgesOf returns the sides that declared draws a border on.
func edgesOf(declared map[string]string) document.Edges {
	var edges document.Edges
	for _, side := range []struct {
		edge document.Edges
		name string
	}{{document.EdgeTop, "top"}, {document.EdgeRight, "right"}, {document.EdgeBottom, "bottom"}, {document.EdgeLeft, "left"}} {
		if isVisibleBorder(declared["border-"+side.name+"-style"], declared["border-"+side.name+"-width"]) {
			edges |= side.edge
		}
	}
	return edges
}

// verticalAlignOf returns the vertical alignment of a cell's vertical-align keyword: top for
// baseline and others.
func verticalAlignOf(keyword string) document.VerticalAlign {
	switch keyword {
	case "middle":
		return document.VerticalMiddle
	case "bottom":
		return document.VerticalBottom
	}
	return document.VerticalTop
}
