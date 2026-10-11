package layout

import (
	"image"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// band is rows of a table, from first to past last, that no cell spans out of. A band is the
// smallest group of rows that can go on a page on its own.
type band struct{ first, last int }

// bands returns the bands of grid's rows.
func (grid *tableGrid) bands() []band {
	reach := make([]int, grid.rows) // the row past the cells that start at each row
	for row := range grid.rows {
		reach[row] = row + 1
	}
	for _, cellLayout := range grid.cells {
		reach[cellLayout.row] = max(reach[cellLayout.row], cellLayout.row+cellLayout.rows)
	}
	var all []band
	for first := 0; first < grid.rows; {
		last := first + 1
		for row := first; row < last; row++ {
			last = max(last, reach[row])
		}
		all = append(all, band{first, last})
		first = last
	}
	return all
}

// cellsByBand returns the cells of grid in each of bands, none of them placed yet.
func (grid *tableGrid) cellsByBand(bands []band) [][]bandCell {
	cells := make([][]bandCell, len(bands))
	index := 0
	for _, cellLayout := range grid.cells { // row after row
		for index < len(bands)-1 && cellLayout.row >= bands[index].last {
			index++
		}
		cells[index] = append(cells[index], bandCell{layout: cellLayout, row: cellLayout.row, rows: cellLayout.rows})
	}
	return cells
}

// rowHeights returns the heights of rows, each with its top line: as high as its highest cell.
// A cell spanning rows that are too low for it makes the last of them higher.
func (grid *tableGrid) rowHeights(cells []bandCell, rows band) []int {
	heights := make([]int, rows.last-rows.first)
	need := func(cell bandCell) int {
		return grid.thickness + cell.layout.top + cell.layout.height(cell.from) + cell.layout.bottom
	}
	for _, cell := range cells {
		if cell.rows == 1 {
			heights[cell.row-rows.first] = max(heights[cell.row-rows.first], need(cell))
		}
	}
	for _, cell := range cells {
		if cell.rows > 1 {
			has := 0
			for _, height := range heights[cell.row-rows.first : cell.row-rows.first+cell.rows] {
				has += height
			}
			if wanted := need(cell); wanted > has {
				heights[cell.row-rows.first+cell.rows-1] += wanted - has
			}
		}
	}
	return heights
}

// firstOffsetOf returns the offset of the first rune left to place in cells, or fallback if they
// have none.
func firstOffsetOf(cells []bandCell, fallback int) int {
	start := -1
	for _, cell := range cells {
		if cell.from < len(cell.layout.units) {
			if offset := cell.layout.units[cell.from].start; start < 0 || offset < start {
				start = offset
			}
		}
	}
	if start < 0 {
		return fallback
	}
	return start
}

// placeRows places the rows of grid, a table whose blocks are blocks, band after band: each on
// the current page if it fits, else on the next one, under the header repeated. A band higher
// than a page is cut between the lines of its cells.
func (layout *paginator) placeRows(grid *tableGrid, blocks []document.Block) error {
	table, thickness := grid.table, grid.thickness
	if table.PageBreak && !layout.pageIsEmpty() {
		layout.newPage(blocks[0].Start)
	}
	if !layout.pageIsEmpty() {
		layout.y += max(layout.marginAfter, layout.settings.em(table.MarginTop))
	}
	all := grid.bands()
	bandCells := grid.cellsByBand(all)
	starts := bandStarts(blocks, bandCells)
	header := layout.repeatedHeader(grid, all, bandCells)
	frameTop := layout.y // the top of the table's part on the current page
	closeFrame := func() {
		layout.drawFrame(grid, image.Rect(grid.columnX[0], frameTop, grid.columnX[grid.columns], layout.y))
	}
	newPage := func(start int, body bool) {
		closeFrame()
		layout.newPage(start)
		frameTop = layout.y
		if header.cells != nil && body {
			layout.placeBand(grid, bandToPlace{cells: slices.Clone(header.cells), heights: header.heights, rows: header.rows, start: start}, header.height, false)
		}
	}
	for index, rows := range all {
		body := rows.first >= header.rows.last
		cells := bandCells[index]
		atTop := layout.pageIsEmpty() // nothing on the page yet but the repeated header
		for len(cells) > 0 {
			heights := grid.rowHeights(cells, rows)
			total := 0
			for _, height := range heights {
				total += height
			}
			needed := total
			if !body && header.cells != nil && index+1 < len(all) {
				// The header stays with the start of the rows under it, as a heading does.
				needed += min(grid.height(bandCells[index+1], all[index+1]), layout.pageHeight/headerShare)
			}
			start := firstOffsetOf(cells, starts[index])
			toPlace := bandToPlace{cells: cells, heights: heights, rows: rows, start: start}
			if needed+thickness <= layout.roomLeft() {
				layout.placeBand(grid, toPlace, total, false)
				break
			}
			reserved := 0
			if body {
				reserved = header.height
			}
			if !atTop && total+thickness <= layout.pageHeight-reserved {
				newPage(start, body)
				atTop = true
				continue
			}
			left, progressed := layout.placeBand(grid, toPlace, layout.roomLeft()-thickness, atTop)
			switch {
			case !progressed && !atTop:
				newPage(start, body)
				atTop = true
				continue
			case !progressed:
				// A page too small for even one line: the rows go past its bottom.
				layout.placeBand(grid, toPlace, total, false)
				left = nil
			}
			cells, rows = left, band{firstRowOf(left, rows), rows.last}
			if len(cells) > 0 {
				newPage(firstOffsetOf(cells, starts[index]), body)
				atTop = true
			}
		}
	}
	closeFrame()
	layout.y += thickness
	layout.marginAfter = layout.settings.em(table.MarginBottom)
	return nil
}

// bandStarts returns the offset of the first rune of each band of a table whose blocks are
// blocks, and the offset past the table. A band without text starts where the next one does.
func bandStarts(blocks []document.Block, bandCells [][]bandCell) []int {
	last := blocks[len(blocks)-1]
	starts := make([]int, len(bandCells)+1)
	starts[len(bandCells)] = last.Start + last.Length()
	for index := range slices.Backward(bandCells) {
		starts[index] = firstOffsetOf(bandCells[index], starts[index+1])
	}
	return starts
}

// tableHeader is the header rows of a table, repeated at the top of each page the table goes on
// to: their cells (nil if the header is not repeated), the heights of the rows and their total.
// rows ends at the first body row even when the header is not repeated.
type tableHeader struct {
	cells   []bandCell
	heights []int
	height  int
	rows    band
}

// repeatedHeader returns the header of grid's table, whose bands are bands. It is repeated only
// if its rows are bands of their own (no cell spans out of them) and take at most a headerShare
// of a page, which leaves room for the body.
func (layout *paginator) repeatedHeader(grid *tableGrid, bands []band, bandCells [][]bandCell) tableHeader {
	header := tableHeader{rows: band{0, min(max(grid.table.HeaderRows, 0), grid.rows)}}
	headerRows := header.rows.last
	headerBands := slices.IndexFunc(bands, func(rows band) bool { return rows.last == headerRows }) + 1
	if headerRows == 0 || headerRows >= grid.rows || headerBands == 0 {
		return header
	}
	cells := slices.Concat(bandCells[:headerBands]...)
	heights := grid.rowHeights(cells, header.rows)
	height := 0
	for _, rowHeight := range heights {
		height += rowHeight
	}
	if height+grid.thickness > layout.pageHeight/headerShare {
		return header
	}
	header.cells, header.heights, header.height = cells, heights, height
	return header
}

// drawFrame draws the frame of grid's table around area, the table's part on the page.
func (layout *paginator) drawFrame(grid *tableGrid, area image.Rectangle) {
	if grid.table.Frame != 0 && area.Dy() > 0 {
		drawEdges(&layout.pages[len(layout.pages)-1], grid.table.Frame, area, grid.thickness)
	}
}

// firstRowOf returns the first row of cells, or rows' last if there are none.
func firstRowOf(cells []bandCell, rows band) int {
	first := rows.last
	for _, cell := range cells {
		first = min(first, cell.row)
	}
	return first
}

// height returns the height of rows of grid holding cells, none of them placed yet.
func (grid *tableGrid) height(cells []bandCell, rows band) int {
	total := 0
	for _, height := range grid.rowHeights(cells, rows) {
		total += height
	}
	return total
}

// bandToPlace is what is left to place of a band of rows: its cells, the heights of its rows,
// and the offset of its first rune.
type bandToPlace struct {
	cells   []bandCell
	heights []int
	rows    band
	start   int
}

// cellPart is the part of a cell that a band shows on a page: its content's units up to to, in
// its area from top to end, counted from the band's top.
type cellPart struct {
	cell     bandCell
	to       int
	top, end int
}

// placeBand places a band of grid's rows at the current height, cutting it cut pixels below its
// top. It returns the cells left to place, and whether some content was placed. With force, a
// band that would show nothing above cut shows a unit of each cell anyway, so that the table
// moves on.
func (layout *paginator) placeBand(grid *tableGrid, toPlace bandToPlace, cut int, force bool) ([]bandCell, bool) {
	cells, rows := toPlace.cells, toPlace.rows
	tops := make([]int, len(toPlace.heights)+1) // of each row from the band's top, then its bottom
	for index, height := range toPlace.heights {
		tops[index+1] = tops[index] + height
	}
	parts, progressed := cutCells(grid, toPlace, tops, cut, force)
	if !progressed {
		return cells, false
	}
	bottom := partBottom(grid, parts, tops, cut)
	page := &layout.pages[len(layout.pages)-1]
	y := layout.y
	var left []bandCell
	for _, part := range parts {
		layout.drawCellPart(page, grid, part, y, bottom)
		if part.end > bottom {
			// The cell goes on, on the next page, from its first row not placed whole.
			cell := part.cell
			firstRow := rows.first
			for firstRow+1 < rows.last && tops[firstRow+1-rows.first] <= bottom {
				firstRow++
			}
			firstRow = max(firstRow, cell.row)
			left = append(left, bandCell{layout: cell.layout, from: part.to, row: firstRow, rows: cell.row + cell.rows - firstRow, continued: true})
		}
	}
	// The cells of rows below cut have not started.
	for _, cell := range cells {
		if tops[cell.row-rows.first] >= cut {
			left = append(left, cell)
		}
	}
	slices.SortStableFunc(left, func(first, second bandCell) int { return first.row - second.row })
	layout.y += bottom
	layout.recordUnit(y, toPlace.start)
	return left, true
}

// cutCells returns the parts of a band's cells that show above cut, the band's rows starting at
// tops, and whether they show some content (or the band has none to show). With force, a cell
// that would show nothing shows its next unit anyway.
func cutCells(grid *tableGrid, toPlace bandToPlace, tops []int, cut int, force bool) ([]cellPart, bool) {
	rows := toPlace.rows
	parts := make([]cellPart, 0, len(toPlace.cells))
	progressed, content := false, false
	for _, cell := range toPlace.cells {
		cellLayout := cell.layout
		content = content || cell.from < len(cellLayout.units)
		top := tops[cell.row-rows.first]
		if top >= cut {
			continue
		}
		room := cut - top - grid.thickness - cellLayout.top - cellLayout.bottom
		to := cell.from
		for to < len(cellLayout.units) && cellLayout.units[to].bottom-cellLayout.units[cell.from].top <= room {
			to++
		}
		progressed = progressed || to > cell.from
		parts = append(parts, cellPart{cell: cell, to: to, top: top, end: tops[cell.row-rows.first+cell.rows]})
	}
	if !progressed && force {
		for index := range parts {
			if part := &parts[index]; part.to < len(part.cell.layout.units) {
				part.to++
				progressed = true
			}
		}
	}
	return parts, progressed || !content
}

// partBottom returns the bottom of the part of a band that parts show, from the band's top: below
// its rows that fit whole above cut, and below what the cut cells show.
func partBottom(grid *tableGrid, parts []cellPart, tops []int, cut int) int {
	bottom := 0
	for _, rowBottom := range tops[1:] {
		if rowBottom <= cut {
			bottom = rowBottom
		}
	}
	for _, part := range parts {
		if part.end > cut {
			cellLayout := part.cell.layout
			shownHeight := 0
			if part.to > part.cell.from {
				shownHeight = cellLayout.units[part.to-1].bottom - cellLayout.units[part.cell.from].top
			}
			bottom = max(bottom, min(part.top+grid.thickness+cellLayout.top+shownHeight+cellLayout.bottom, cut))
		}
	}
	return bottom
}

// drawCellPart adds part of a cell to page, in a band placed at y whose part on the page ends at
// bottom: its borders, and its content's units, aligned vertically in the cell if they all show.
func (layout *paginator) drawCellPart(page *Page, grid *tableGrid, part cellPart, y, bottom int) {
	cell, cellLayout, thickness := part.cell, part.cell.layout, grid.thickness
	cellArea := image.Rect(grid.columnX[cellLayout.column], y+part.top, grid.columnX[cellLayout.column+cellLayout.columns], y+min(part.end, bottom))
	edges := cellLayout.cell.Borders
	if edges&(document.EdgeTop|document.EdgeBottom) != 0 && (cell.continued || part.end > bottom) {
		edges |= document.EdgeTop | document.EdgeBottom // close the cell where the page cuts it
	}
	drawEdges(page, edges, cellArea, thickness)
	if part.to <= cell.from {
		return
	}
	down := cellArea.Min.Y + thickness + cellLayout.top
	whole := cell.from == 0 && part.to == len(cellLayout.units)
	if room := cellArea.Dy() - thickness - cellLayout.top - cellLayout.bottom - cellLayout.height(0); whole && room > 0 {
		switch cellLayout.cell.VerticalAlign {
		case document.VerticalMiddle:
			down += room / 2
		case document.VerticalBottom:
			down += room
		}
	}
	appendUnits(page, &cellLayout.content, cellLayout.units, cell.from, part.to, image.Pt(cellArea.Min.X+thickness+cellLayout.left, down-cellLayout.units[cell.from].top))
}

// drawEdges draws on page the edges of area, lines thickness thick. The bottom and right ones go
// just past area, where the next cells' top and left lines are, so that shared lines are drawn
// once.
func drawEdges(page *Page, edges document.Edges, area image.Rectangle, thickness int) {
	if thickness == 0 || edges == 0 {
		return
	}
	for _, edge := range []struct {
		side document.Edges
		rect image.Rectangle
	}{
		{document.EdgeTop, image.Rect(area.Min.X, area.Min.Y, area.Max.X+thickness, area.Min.Y+thickness)},
		{document.EdgeBottom, image.Rect(area.Min.X, area.Max.Y, area.Max.X+thickness, area.Max.Y+thickness)},
		{document.EdgeLeft, image.Rect(area.Min.X, area.Min.Y, area.Min.X+thickness, area.Max.Y+thickness)},
		{document.EdgeRight, image.Rect(area.Max.X, area.Min.Y, area.Max.X+thickness, area.Max.Y+thickness)},
	} {
		if edges&edge.side != 0 {
			page.Rules = append(page.Rules, edge.rect)
		}
	}
}

// appendUnits copies onto page the units of content from from to past to, moved by offset.
func appendUnits(page, content *Page, units []contentUnit, from, to int, offset image.Point) {
	var begin pageCounts
	if from > 0 {
		begin = units[from-1].end
	}
	end := units[to-1].end
	for _, line := range content.Lines[begin.lines:end.lines] {
		words := make([]PlacedWord, len(line.Words))
		for index, word := range line.Words {
			word.X += offset.X
			word.Top += offset.Y
			words[index] = word
		}
		line.Words = words
		page.Lines = append(page.Lines, line)
	}
	for _, placed := range content.Images[begin.images:end.images] {
		placed.Rect = placed.Rect.Add(offset)
		page.Images = append(page.Images, placed)
	}
	for _, rule := range content.Rules[begin.rules:end.rules] {
		page.Rules = append(page.Rules, rule.Add(offset))
	}
	for _, stroke := range content.Strokes[begin.strokes:end.strokes] {
		page.Strokes = append(page.Strokes, Stroke{Rectangle: stroke.Add(offset), Gray: stroke.Gray})
	}
	for _, link := range content.Links[begin.links:end.links] {
		link.Rect = link.Rect.Add(offset)
		page.Links = append(page.Links, link)
	}
}
