package layout

import (
	"image"
	"log"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// fitColumns measures the cells of grid and sets its columns in width. It returns the font size
// the table is laid out at: the paginator's, or smaller for a table too wide even with every
// column at its narrowest, as a wide table on a small screen often is.
func (layout *paginator) fitColumns(grid *tableGrid, width int) (int, error) {
	size := layout.settings.FontSize
	smallest := max(size*minTableScale/100, 1)
	for attempt := range tableShrinks {
		narrowest, err := layout.measureCells(grid, size, width)
		if err != nil {
			return 0, err
		}
		available := width - grid.thickness
		if narrowest <= available || size <= smallest || attempt == tableShrinks-1 {
			break
		}
		size = max(smallest, min(size-1, size*available/narrowest))
	}
	return size, nil
}

// measureCells measures the cells of grid at size and sets its columns in width. It returns the
// width of the table with every column at its narrowest.
func (layout *paginator) measureCells(grid *tableGrid, size, width int) (int, error) {
	for _, cellLayout := range grid.cells {
		measurer := layout.cellPaginator(cellLayout.cell, size, width, nil)
		minimum, maximum, err := measurer.contentWidths(cellLayout.blocks)
		if err != nil {
			return 0, err
		}
		cell, em := cellLayout.cell, measurer.settings.em
		cellLayout.top, cellLayout.bottom = em(max(cell.PaddingTop, cellPaddingEnd)), em(max(cell.PaddingBottom, cellPaddingEnd))
		cellLayout.left, cellLayout.right = em(max(cell.PaddingLeft, cellPaddingSide)), em(max(cell.PaddingRight, cellPaddingSide))
		sides := grid.thickness + cellLayout.left + cellLayout.right
		cellLayout.minimum, cellLayout.maximum = minimum+sides, max(maximum, minimum)+sides
	}
	minimums, maximums := columnRanges(grid)
	target := 0
	if grid.table.Width > 0 {
		target = width * min(grid.table.Width, 100) / 100
	}
	widths := columnWidths(minimums, maximums, grid.table.ColumnWidths, width-grid.thickness, target-grid.thickness)
	grid.columnX = make([]int, grid.columns+1)
	for index, columnWidth := range widths {
		grid.columnX[index+1] = grid.columnX[index] + columnWidth
	}
	narrowest := 0
	for _, minimum := range minimums {
		narrowest += minimum
	}
	return narrowest, nil
}

// columnRanges returns the narrowest and the widest each column of grid may be, from the widths
// of its cells. A cell spanning columns widens them evenly where they are too narrow for it; the
// cells spanning fewer columns go first, as they say more about each column.
func columnRanges(grid *tableGrid) (minimums, maximums []int) {
	minimums, maximums = make([]int, grid.columns), make([]int, grid.columns)
	spanning := slices.Clone(grid.cells)
	slices.SortStableFunc(spanning, func(first, second *cellLayout) int { return first.columns - second.columns })
	widen := func(widths []int, first, count, wanted int) {
		has := 0
		for _, columnWidth := range widths[first : first+count] {
			has += columnWidth
		}
		for index := range count {
			if wanted > has {
				share := (wanted - has) / count
				if index < (wanted-has)%count {
					share++
				}
				widths[first+index] += share
			}
		}
	}
	for _, cellLayout := range spanning {
		widen(minimums, cellLayout.column, cellLayout.columns, cellLayout.minimum)
		widen(maximums, cellLayout.column, cellLayout.columns, cellLayout.maximum)
	}
	for index := range maximums {
		maximums[index] = max(maximums[index], minimums[index])
	}
	return minimums, maximums
}

// columnWidths returns the widths of columns that may be from minimums to maximums wide, some
// asking (wanted, when not 0) for a percent of the table's width. The table is target wide if
// that is positive; else as wide as its columns want, within available: all of it if some ask
// for a percent, the percents' sum if all do. This is roughly what browsers do for CSS tables.
//
// The columns asked for get their share first, the others share the rest, from their narrowest
// to their widest. Columns too wide for available even at their narrowest shrink in proportion,
// and their text overflows: there is nothing better to do.
func columnWidths(minimums, maximums, wanted []int, available, target int) []int {
	limits := columnLimits{minimums: minimums, maximums: maximums, wanted: wanted}
	tableWidth, sumMinimum := limits.tableWidth(available, target)
	if sumMinimum >= tableWidth {
		return limits.shrunkMinimums(sumMinimum, tableWidth)
	}
	widths := make([]int, len(minimums))
	totals := limits.giveAskedWidths(widths, tableWidth)
	if totals.asked+totals.otherMinimum > tableWidth {
		limits.shrinkAsked(widths, tableWidth, totals)
		return widths
	}
	limits.shareRest(widths, tableWidth-totals.asked, totals.otherMinimum, totals.otherMaximum)
	if totals.asked > 0 && totals.otherMaximum == 0 && totals.asked < tableWidth {
		// Only columns asked for, and empty ones: those asked for share what is left.
		left := tableWidth - totals.asked
		for column := range widths {
			if limits.asked(column) > 0 {
				widths[column] += left * widths[column] / totals.asked
			}
		}
	}
	return widths
}

// columnLimits are the narrowest and the widest each column of a table may be, and the percent of
// the table's width each asks for.
type columnLimits struct {
	minimums, maximums, wanted []int
}

// asked returns the percent of the table's width that column asks for, 0 if none.
func (limits columnLimits) asked(column int) int {
	if column < len(limits.wanted) {
		return min(max(limits.wanted[column], 0), 100)
	}
	return 0
}

// tableWidth returns the width of the table, and the sum of its columns' narrowest widths.
func (limits columnLimits) tableWidth(available, target int) (tableWidth, sumMinimum int) {
	sumMaximum, sumAsked, allAsked := 0, 0, true
	for column := range limits.minimums {
		sumMinimum += limits.minimums[column]
		sumMaximum += limits.maximums[column]
		sumAsked += limits.asked(column)
		allAsked = allAsked && limits.asked(column) > 0
	}
	tableWidth = min(sumMaximum, available)
	switch {
	case target > 0:
		tableWidth = min(target, available)
	case allAsked:
		tableWidth = available * min(sumAsked, 100) / 100
	case sumAsked > 0:
		tableWidth = available
	}
	return max(tableWidth, min(sumMinimum, available)), sumMinimum
}

// shrunkMinimums returns the narrowest widths of the columns, shrunk in proportion to fit in
// tableWidth.
func (limits columnLimits) shrunkMinimums(sumMinimum, tableWidth int) []int {
	widths := make([]int, len(limits.minimums))
	for column, minimum := range limits.minimums {
		widths[column] = minimum
		if sumMinimum > 0 {
			widths[column] = minimum * tableWidth / sumMinimum
		}
	}
	return widths
}

// columnTotals are the sums of a table's columns' widths: of those asked for, at the width they
// get and at their narrowest, and of the others, at their narrowest and at their widest.
type columnTotals struct {
	asked, askedMinimum        int
	otherMinimum, otherMaximum int
}

// giveAskedWidths sets the width of each column asked for to its share of tableWidth, and returns
// the totals of all the columns.
func (limits columnLimits) giveAskedWidths(widths []int, tableWidth int) columnTotals {
	var totals columnTotals
	for column := range widths {
		minimum, maximum := limits.minimums[column], limits.maximums[column]
		percent := limits.asked(column)
		if percent == 0 {
			totals.otherMinimum += minimum
			totals.otherMaximum += maximum
			continue
		}
		widths[column] = max(minimum, tableWidth*percent/100)
		if maximum > widths[column] && maximum <= widths[column]*askedSlack/100 {
			// Asked a little narrower than its text: the book's CSS was written for a smaller
			// font than the reader's, and its short lines would break.
			widths[column] = maximum
		}
		totals.asked += widths[column]
		totals.askedMinimum += minimum
	}
	return totals
}

// shrinkAsked shrinks the columns asked for, when they and the others at their narrowest do not
// fit in tableWidth, and sets the others at their narrowest.
func (limits columnLimits) shrinkAsked(widths []int, tableWidth int, totals columnTotals) {
	room := tableWidth - totals.otherMinimum - totals.askedMinimum
	for column, minimum := range limits.minimums {
		if limits.asked(column) > 0 {
			widths[column] = minimum + (widths[column]-minimum)*room/max(totals.asked-totals.askedMinimum, 1)
		} else {
			widths[column] = minimum
		}
	}
}

// shareRest shares rest, the width the columns asked for leave, among the others, whose
// narrowest and widest add up to otherMinimum and otherMaximum. If they all fit at their widest,
// they share what is left in proportion; else a narrow column gets its widest if the others
// still fit (a date, a name), and the rest are between their narrowest and their widest.
func (limits columnLimits) shareRest(widths []int, rest, otherMinimum, otherMaximum int) {
	minimums, maximums := limits.minimums, limits.maximums
	var flexible []int // the columns not asked for that are not yet as wide as they want
	for column := range widths {
		if limits.asked(column) == 0 {
			flexible = append(flexible, column)
		}
	}
	if otherMaximum <= rest {
		for _, column := range flexible {
			widths[column] = maximums[column]
			if otherMaximum > 0 {
				widths[column] += (rest - otherMaximum) * maximums[column] / otherMaximum
			}
		}
		return
	}
	for narrowed := len(flexible) > 0; narrowed; {
		narrowed = false
		for index, column := range flexible {
			if maximums[column]*len(flexible) <= rest && rest-maximums[column] >= otherMinimum-minimums[column] {
				widths[column] = maximums[column]
				rest, otherMinimum, otherMaximum = rest-maximums[column], otherMinimum-minimums[column], otherMaximum-maximums[column]
				flexible = slices.Delete(flexible, index, index+1)
				narrowed = true
				break
			}
		}
	}
	for _, column := range flexible {
		widths[column] = minimums[column] + (maximums[column]-minimums[column])*(rest-otherMinimum)/max(otherMaximum-otherMinimum, 1)
	}
}

// contentWidths returns the narrowest and the widest that blocks may be laid out: their widest
// word or image, and their longest paragraph on one line.
func (layout *paginator) contentWidths(blocks []document.Block) (minimum, maximum int, err error) {
	for index := 0; index < len(blocks); {
		var blockMinimum, blockMaximum int
		if table := layout.tableOf(blocks[index]); table != nil {
			end := index + 1
			for end < len(blocks) && layout.tableOf(blocks[end]) == table {
				end++
			}
			blockMinimum, blockMaximum, err = layout.tableWidths(table, blocks[index:end])
			index = end
		} else {
			blockMinimum, blockMaximum, err = layout.blockWidths(blocks[index])
			index++
		}
		if err != nil {
			return 0, 0, err
		}
		minimum, maximum = max(minimum, blockMinimum), max(maximum, blockMaximum)
	}
	return minimum, maximum, nil
}

// tableWidths returns the narrowest and the widest that a table in a cell may be.
func (layout *paginator) tableWidths(table *document.Table, blocks []document.Block) (minimum, maximum int, err error) {
	grid := layout.newGrid(table, blocks)
	if grid == nil {
		return layout.contentWidths(nil)
	}
	if _, err := layout.measureCells(grid, layout.settings.FontSize, layout.content.Dx()); err != nil {
		return 0, 0, err
	}
	minimums, maximums := columnRanges(grid)
	margins := layout.settings.em(table.Left) + layout.settings.em(table.Right) + grid.thickness
	for column := range grid.columns {
		minimum += minimums[column]
		maximum += maximums[column]
	}
	return minimum + margins, maximum + margins, nil
}

// blockWidths returns the narrowest and the widest that block may be laid out.
func (layout *paginator) blockWidths(block document.Block) (minimum, maximum int, err error) {
	margins := layout.settings.em(block.Left) + layout.settings.em(block.Right)
	switch block.Kind {
	case document.Rule:
		return 0, 0, nil
	case document.Image:
		if mathml.Is(block.Image) {
			formula, _, err := layout.faces.formula(block.Image, layout.settings.FontSize, image.Pt(layout.content.Dx(), layout.pageHeight))
			if err != nil {
				log.Printf("layout: formula: %v", err)
				return 0, 0, nil
			}
			return formula.Width + margins, formula.Width + margins, nil
		}
		size, err := layout.sizer(block.Image)
		if err != nil || size.X <= 0 || size.Y <= 0 {
			return 0, 0, nil
		}
		width := size.X
		if block.ImageWidth > 0 {
			width = max(layout.content.Dx()*block.ImageWidth/100, 1)
		}
		return min(width, layout.settings.em(minImageCell)) + margins, width + margins, nil
	}
	prepared, err := layout.prepareText(block, layout.blockSize(block))
	if err != nil || len(prepared.words) == 0 {
		return 0, 0, err
	}
	indent := max(layout.settings.em(block.Indent), 0)
	line := indent
	for index, word := range prepared.words {
		if index == 0 {
			minimum = indent + word.width
		}
		minimum = max(minimum, word.width)
		line += word.width
		if index < len(prepared.words)-1 && word.breakAfter() == breakSpace {
			line += word.fragments[len(word.fragments)-1].format.space
		}
	}
	return minimum + margins, line + margins, nil
}
