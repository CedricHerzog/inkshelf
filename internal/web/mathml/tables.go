package mathml

import (
	"cmp"
	"image"
	"strings"
)

// tableGrid is the cells of an <mtable>, laid out, by row then column, with how each is
// aligned in its column, and the size of each column and row.
type tableGrid struct {
	rows              [][]*box
	aligns            [][]string
	widths            []int // by column
	ascents, descents []int // by row
}

// tableSpacing is the spacing of an <mtable>, in pixels: between its columns and its rows,
// inside its frame, and the thickness of its lines.
type tableSpacing struct {
	columnGap, rowGap, padding, bar int
}

// table lays out an <mtable>: its cells in rows and columns, centred on the axis, with the
// lines and the frame that its attributes ask for.
func (layout *layouter) table(node *Node, current style) *box {
	grid := layout.tableGrid(node, current)
	spacing := tableSpacing{
		columnGap: layout.length(node, "columnspacing", current, current.em(tableColumnGap)),
		rowGap:    layout.length(node, "rowspacing", current, current.em(tableRowGap)),
		bar:       thickness(current),
	}
	frame := node.Attributes["frame"] == "solid" || node.Attributes["frame"] == "dashed"
	if frame {
		spacing.padding = current.em(frameSpacing)
	}
	width, height := grid.size(spacing)
	top := -(height/2 + layout.axis(current))
	laid := &box{width: width, ascent: -top, descent: height + top}
	grid.placeRows(laid, node.Attributes["rowlines"], top, spacing)
	grid.drawColumnLines(laid, node.Attributes["columnlines"], top, height, spacing)
	if frame {
		laid.items = append(laid.items, frameItems(image.Rect(0, top, width, top+height), spacing.bar, "box")...)
	}
	return laid
}

// tableGrid lays out the cells of an <mtable> and measures its columns and rows. The label of
// an <mlabeledtr> is left out, and a child that is no row is a row of one cell.
func (layout *layouter) tableGrid(node *Node, current style) tableGrid {
	cellStyle := current
	cellStyle.display = false
	var grid tableGrid
	for _, rowNode := range node.Children {
		cells := rowNode.Children
		switch rowNode.Name {
		case "mtr":
		case "mlabeledtr":
			cells = cells[min(1, len(cells)):]
		default:
			cells = []*Node{rowNode}
		}
		var row []*box
		var rowAligns []string
		for column, cell := range cells {
			if cell.Name == "mtd" {
				row = append(row, layout.row(cell.Children, layout.restyle(cell, cellStyle)))
			} else {
				row = append(row, layout.node(cell, cellStyle, "infix"))
			}
			rowAligns = append(rowAligns, cmp.Or(cell.Attributes["columnalign"], listAt(rowNode.Attributes["columnalign"], column), listAt(node.Attributes["columnalign"], column), "center"))
		}
		grid.rows, grid.aligns = append(grid.rows, row), append(grid.aligns, rowAligns)
	}
	grid.ascents, grid.descents = make([]int, len(grid.rows)), make([]int, len(grid.rows))
	for rowIndex, row := range grid.rows {
		for column, cell := range row {
			if column >= len(grid.widths) {
				grid.widths = append(grid.widths, 0)
			}
			grid.widths[column] = max(grid.widths[column], cell.width)
			grid.ascents[rowIndex], grid.descents[rowIndex] = max(grid.ascents[rowIndex], cell.ascent), max(grid.descents[rowIndex], cell.descent)
		}
	}
	return grid
}

// size returns the width and the height of the table, its frame's padding included.
func (grid *tableGrid) size(spacing tableSpacing) (width, height int) {
	height = 2 * spacing.padding
	for rowIndex := range grid.rows {
		height += grid.ascents[rowIndex] + grid.descents[rowIndex]
		if rowIndex > 0 {
			height += spacing.rowGap
		}
	}
	width = 2 * spacing.padding
	for column, columnWidth := range grid.widths {
		width += columnWidth
		if column > 0 {
			width += spacing.columnGap
		}
	}
	return width, height
}

// placeRows places the cells in laid, row by row from top, each aligned in its column, with
// the lines between the rows that rowLines (the rowlines attribute) asks for.
func (grid *tableGrid) placeRows(laid *box, rowLines string, top int, spacing tableSpacing) {
	y := top + spacing.padding
	for rowIndex, row := range grid.rows {
		if rowIndex > 0 {
			if line := listAt(rowLines, rowIndex-1); line == "solid" || line == "dashed" {
				middle := y + spacing.rowGap/2 - spacing.bar/2
				laid.rule(image.Rect(spacing.padding, middle, laid.width-spacing.padding, middle+spacing.bar))
			}
			y += spacing.rowGap
		}
		baseline := y + grid.ascents[rowIndex]
		x := spacing.padding
		for column, columnWidth := range grid.widths {
			if column > 0 {
				x += spacing.columnGap
			}
			if column < len(row) {
				cell := row[column]
				offset := (columnWidth - cell.width) / 2
				switch grid.aligns[rowIndex][column] {
				case "left":
					offset = 0
				case "right":
					offset = columnWidth - cell.width
				}
				laid.add(cell, x+offset, baseline)
			}
			x += columnWidth
		}
		y = baseline + grid.descents[rowIndex]
	}
}

// drawColumnLines draws the lines between the columns that columnLines (the columnlines
// attribute) asks for, from top for height pixels.
func (grid *tableGrid) drawColumnLines(laid *box, columnLines string, top, height int, spacing tableSpacing) {
	x := spacing.padding
	for column, columnWidth := range grid.widths[:max(len(grid.widths)-1, 0)] {
		x += columnWidth
		if line := listAt(columnLines, column); line == "solid" || line == "dashed" {
			middle := x + spacing.columnGap/2 - spacing.bar/2
			laid.rule(image.Rect(middle, top, middle+spacing.bar, top+height))
		}
		x += spacing.columnGap
	}
}

// listAt returns the value at index of a list attribute (values separated by spaces), its last
// value past its end.
func listAt(list string, index int) string {
	values := strings.Fields(list)
	if len(values) == 0 {
		return ""
	}
	return values[min(index, len(values)-1)]
}
