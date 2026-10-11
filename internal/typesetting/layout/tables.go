package layout

import (
	"image"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// A table is laid out as a grid: each cell's content is laid out apart, on a tall page of its
// own, by a paginator of its own (cellPaginator), first to measure how narrow and how wide its
// column may be, then in the width its column gets. The rows are then placed band by band (a
// band is rows that no cell spans out of), each kept whole on a page when it fits, else cut
// between the lines of its cells.

// Lengths are in hundredths of an em.
const (
	// cellPaddingSide and cellPaddingEnd are the least space between a cell's text and its
	// sides, and its top and bottom: with CSS's usual 1 px, the columns' text would touch.
	cellPaddingSide = 25
	cellPaddingEnd  = 10
	// minImageCell is the width an image in a cell may shrink to before its column must widen.
	minImageCell = 300
)

const (
	// minTableScale is the smallest font a table too wide shrinks to, in percent of the text's:
	// smaller would be too hard to read, so its columns are squeezed instead.
	minTableScale = 60
	// tableShrinks is how many times a table too wide is measured again in a smaller font.
	tableShrinks = 4
	// askedSlack is how much wider than the book asks, in percent, a column may get to keep its
	// text on one line.
	askedSlack = 140
	// headerShare is the share of a page a repeated header may take at most: a quarter.
	headerShare = 4
	// cellHeight is the height of the page a cell is laid out on: endless, for all its content.
	cellHeight = 1 << 30
)

// contentUnit is what a paginator placed in one go on its last page, from top to bottom: a line,
// an image, a rule, or a band of a nested table's rows. A cell records its units so that a row
// too high for a page can be cut between them. start is the offset of the unit's first rune, and
// end the lengths of the page's lists after it, which tell what the unit placed.
type contentUnit struct {
	top, bottom, start int
	end                pageCounts
}

// pageCounts are the lengths of a page's lists.
type pageCounts struct{ lines, images, rules, strokes, links int }

// countsOf returns the lengths of page's lists.
func countsOf(page *Page) pageCounts {
	return pageCounts{len(page.Lines), len(page.Images), len(page.Rules), len(page.Strokes), len(page.Links)}
}

// recordUnit records what was just placed from top down to the current height, starting at
// offset start, if the paginator records its units.
func (layout *paginator) recordUnit(top, start int) {
	if layout.units == nil {
		return
	}
	*layout.units = append(*layout.units, contentUnit{top: top, bottom: layout.y, start: start, end: countsOf(&layout.pages[len(layout.pages)-1])})
}

// tableOf returns the table that block is in at the paginator's level of nesting: a table of the
// document, or one in the cell it lays out. It returns nil for a block of the paginator's own
// text.
func (layout *paginator) tableOf(block document.Block) *document.Table {
	cell := block.Cell
	if cell == layout.cell {
		return nil
	}
	for cell != nil && cell.Table.Parent != layout.cell {
		cell = cell.Table.Parent
	}
	if cell == nil {
		return nil
	}
	return cell.Table
}

// cellLayout is a cell of a table being placed: where it is in the grid, its blocks, how narrow
// and how wide it may be, its paddings in pixels, and its content laid out from the top of a page
// of its own.
type cellLayout struct {
	cell                     *document.TableCell
	row, column              int
	rows, columns            int
	blocks                   []document.Block
	minimum, maximum         int
	top, right, bottom, left int // the paddings
	content                  Page
	units                    []contentUnit
}

// height returns the height of the cell's content from unit from on.
func (cellLayout *cellLayout) height(from int) int {
	if from >= len(cellLayout.units) {
		return 0
	}
	return cellLayout.units[len(cellLayout.units)-1].bottom - cellLayout.units[from].top
}

// bandCell is a cell in a band of rows being placed: its content left to place, from unit from,
// and the rows it spans that are not placed yet.
type bandCell struct {
	layout    *cellLayout
	from      int
	row, rows int
	continued bool // the cell started on the page before
}

// tableGrid is a table being placed: its cells, its size in rows and columns, the thickness of
// its lines and where its columns are.
type tableGrid struct {
	table     *document.Table
	cells     []*cellLayout
	rows      int
	columns   int
	thickness int   // 0 for a table without lines
	columnX   []int // the left of each column on the page, and the right of the last
}

// placeTable places table, whose blocks are blocks, as a grid: it sizes its columns, lays out
// each cell in its columns' width, and places the rows. A table without rows or columns is placed
// as plain blocks.
func (layout *paginator) placeTable(table *document.Table, blocks []document.Block) error {
	grid := layout.newGrid(table, blocks)
	if grid == nil {
		for _, block := range blocks {
			block.Cell = layout.cell
			if err := layout.placeBlock(block, nil); err != nil {
				return err
			}
		}
		return nil
	}
	left, width := layout.tableArea(table)
	size, err := layout.fitColumns(grid, width)
	if err != nil {
		return err
	}
	for _, cellLayout := range grid.cells {
		if err := layout.layOutCell(grid, cellLayout, size); err != nil {
			return err
		}
	}
	// The table is centred in the text.
	offset := left + (width-grid.columnX[grid.columns]-grid.thickness)/2
	for index := range grid.columnX {
		grid.columnX[index] += offset
	}
	return layout.placeRows(grid, blocks)
}

// newGrid returns table's grid, each of blocks in its cell, or nil for a table without rows or
// columns. A block in a nested table goes to the cell that holds that table.
func (layout *paginator) newGrid(table *document.Table, blocks []document.Block) *tableGrid {
	grid := &tableGrid{table: table, rows: table.Rows, columns: min(table.Columns, document.MaxTableColumns)}
	if grid.rows <= 0 || grid.columns <= 0 {
		return nil
	}
	byCell := map[*document.TableCell]*cellLayout{}
	add := func(cell *document.TableCell) *cellLayout {
		row, column := max(cell.Row, 0), min(max(cell.Column, 0), grid.columns-1)
		grid.rows = max(grid.rows, row+1)
		added := &cellLayout{cell: cell, row: row, column: column,
			rows: min(max(cell.RowSpan, 1), document.MaxTableRowSpan), columns: min(max(cell.ColumnSpan, 1), grid.columns-column)}
		byCell[cell] = added
		grid.cells = append(grid.cells, added)
		return added
	}
	for _, cell := range table.Cells {
		add(cell)
	}
	for _, block := range blocks {
		cell := block.Cell
		for cell != nil && cell.Table != table {
			cell = cell.Table.Parent
		}
		if cell == nil {
			continue
		}
		cellLayout := byCell[cell]
		if cellLayout == nil {
			cellLayout = add(cell)
		}
		block.PageBreak = false
		cellLayout.blocks = append(cellLayout.blocks, block)
	}
	for _, cellLayout := range grid.cells {
		cellLayout.rows = min(cellLayout.rows, grid.rows-cellLayout.row)
	}
	// Sorted row after row, the order in which the bands are cut.
	slices.SortStableFunc(grid.cells, func(first, second *cellLayout) int { return first.row - second.row })
	if table.Frame != 0 || slices.ContainsFunc(grid.cells, func(cellLayout *cellLayout) bool { return cellLayout.cell.Borders != 0 }) {
		grid.thickness = layout.ruleThickness()
	}
	return grid
}

// tableArea returns the left and the width of the text that table goes in.
func (layout *paginator) tableArea(table *document.Table) (left, width int) {
	left = layout.content.Min.X + layout.settings.em(table.Left)
	return left, max(layout.content.Max.X-layout.settings.em(table.Right)-left, 1)
}

// cellPaginator returns a paginator that lays out cell's content at size, width wide, on one
// endless page; units, if not nil, receives what it places. Its text is not justified, which
// would spread the few words of a narrow column across it.
func (layout *paginator) cellPaginator(cell *document.TableCell, size, width int, units *[]contentUnit) *paginator {
	settings := layout.settings
	settings.FontSize, settings.Justify = size, false
	return &paginator{
		settings: settings, faces: layout.faces, sizer: layout.sizer, hyphenator: layout.hyphenator, doc: layout.doc,
		content: image.Rect(0, 0, max(width, 1), cellHeight), pageHeight: layout.pageHeight, cell: cell, units: units,
	}
}

// layOutCell lays out the content of a cell of grid at size, in the width of its columns.
func (layout *paginator) layOutCell(grid *tableGrid, cellLayout *cellLayout, size int) error {
	width := grid.columnX[cellLayout.column+cellLayout.columns] - grid.columnX[cellLayout.column] - grid.thickness - cellLayout.left - cellLayout.right
	cellPaginator := layout.cellPaginator(cellLayout.cell, size, width, &cellLayout.units)
	start := 0
	if len(cellLayout.blocks) > 0 {
		start = cellLayout.blocks[0].Start
	}
	cellPaginator.newPage(start)
	if err := cellPaginator.placeBlocks(cellLayout.blocks); err != nil {
		return err
	}
	cellLayout.content = cellPaginator.pages[0]
	return nil
}
