package layout

import (
	"image"
	"image/color"
	"image/draw"
	"slices"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// A border box is the border and background of a block element (a sidebar, a boxed note) drawn
// around its blocks. Boxes nest, and a box that goes on to the next page is drawn as one part per
// page: open on the page it starts, broken at the page's bottom, and opened again at the next
// page's top. Margins collapse through a box without a border or padding, as in CSS.

// borderBoxPart is the part of an open border box on the last page.
type borderBoxPart struct {
	borderBox *document.BorderBox
	top       int // its top on the page, where its border goes
	// first is set on the box's first part, which draws its top border.
	first bool
	// placed is how many things the page held when the part started (placedItemCount): a box
	// whose first part holds nothing moves to the next page whole, rather than leave a lone top
	// border at the bottom of a page.
	placed int
}

// placedItemCount returns how many lines, images and rules the last page holds.
func (layout *paginator) placedItemCount() int {
	page := layout.pages[len(layout.pages)-1]
	return len(page.Lines) + len(page.Images) + len(page.Rules)
}

// borderPixels returns the width of a border in pixels: at least one if it is drawn at all, so
// that a thin border never disappears at a small font size.
func (layout *paginator) borderPixels(border document.Border) int {
	if border.Style == document.NoBorder || border.Width <= 0 {
		return 0
	}
	return max(layout.settings.em(border.Width), 1)
}

// topEdge and bottomEdge return the height of a box's border and padding above and below its
// content, in pixels.
func (layout *paginator) topEdge(borderBox *document.BorderBox) int {
	return layout.borderPixels(borderBox.Top) + layout.settings.em(borderBox.PaddingTop)
}

func (layout *paginator) bottomEdge(borderBox *document.BorderBox) int {
	return layout.borderPixels(borderBox.Bottom) + layout.settings.em(borderBox.PaddingBottom)
}

// enterBorderBoxes closes the open boxes that end before block and opens those that start with
// it. A box whose blocks were all left out (hidden footnotes) is skipped.
func (layout *paginator) enterBorderBoxes(block document.Block) {
	for len(layout.openBorderBoxes) > 0 && layout.openBorderBoxes[len(layout.openBorderBoxes)-1].borderBox.End <= block.Start {
		layout.closeBorderBox()
	}
	boxes := layout.doc.Boxes
	for layout.nextBorderBox < len(boxes) && boxes[layout.nextBorderBox].Start <= block.Start {
		borderBox := &boxes[layout.nextBorderBox]
		layout.nextBorderBox++
		if borderBox.End > block.Start {
			layout.openBorderBox(borderBox, block)
		}
	}
}

// openBorderBox starts a box above first, its first block, under the box's top margin. That
// margin collapses with the last block's bottom one and, when the box has no top border nor
// padding, with first's top margin too.
func (layout *paginator) openBorderBox(borderBox *document.BorderBox, first document.Block) {
	if borderBox.PageBreak && !layout.pageIsEmpty() {
		layout.newPage(first.Start)
	}
	margin, edge := layout.settings.em(borderBox.MarginTop), layout.topEdge(borderBox)
	if edge == 0 {
		margin = max(margin, layout.settings.em(first.MarginTop))
	}
	if !layout.pageIsEmpty() || layout.marginKept {
		layout.y += max(layout.marginAfter, margin)
	}
	layout.marginAfter, layout.marginKept = 0, false
	layout.marginTaken = layout.marginTaken || edge == 0
	layout.openBorderBoxes = append(layout.openBorderBoxes, borderBoxPart{borderBox: borderBox, top: layout.y, first: true, placed: layout.placedItemCount()})
	if edge > 0 {
		layout.y += edge
		// Inside a border, the first block's top margin stays even at the top of the page.
		layout.marginKept, layout.marginTaken = true, false
	}
}

// closeBorderBox ends the innermost open box under what was placed in it: the last block's
// bottom margin, the box's padding and border, then its bottom margin. Without a bottom border
// nor padding, the last block's margin collapses with the box's.
func (layout *paginator) closeBorderBox() {
	part := layout.openBorderBoxes[len(layout.openBorderBoxes)-1]
	layout.openBorderBoxes = layout.openBorderBoxes[:len(layout.openBorderBoxes)-1]
	borderBox := part.borderBox
	border := layout.borderPixels(borderBox.Bottom)
	if edge := layout.bottomEdge(borderBox); edge == 0 {
		layout.marginAfter = max(layout.marginAfter, layout.settings.em(borderBox.MarginBottom))
	} else {
		// At the bottom of a page, the padding is cut so that the border still shows: it may go
		// into the page's bottom margin by its width.
		inside := layout.marginAfter + layout.settings.em(borderBox.PaddingBottom)
		layout.y += max(min(inside, layout.content.Max.Y-border-layout.y), 0) + border
		layout.marginAfter = layout.settings.em(borderBox.MarginBottom)
	}
	layout.placeBorderBoxPart(part, layout.y, true, len(layout.openBorderBoxes))
}

// closeAllBorderBoxes closes the boxes still open at the end of the document.
func (layout *paginator) closeAllBorderBoxes() {
	for len(layout.openBorderBoxes) > 0 {
		layout.closeBorderBox()
	}
}

// breakBorderBoxes ends the parts of the open boxes on the last page before a new page starts:
// they go down to its bottom, without their bottom border. A box that holds nothing yet moves
// to the next page whole, where restartBorderBoxes starts it again.
func (layout *paginator) breakBorderBoxes() {
	placed := layout.placedItemCount()
	for index := range layout.openBorderBoxes {
		part := &layout.openBorderBoxes[index]
		if part.first && part.placed == placed {
			continue
		}
		layout.placeBorderBoxPart(*part, layout.content.Max.Y, false, index)
		part.first = false
	}
}

// restartBorderBoxes starts the parts of the open boxes at the top of the new page; a box that
// moved whole keeps its top border and padding.
func (layout *paginator) restartBorderBoxes() {
	for index := range layout.openBorderBoxes {
		part := &layout.openBorderBoxes[index]
		part.top, part.placed = layout.y, 0
		if part.first {
			layout.y += layout.topEdge(part.borderBox)
			layout.marginKept = true
		}
	}
}

// placeBorderBoxPart adds part of a box, nested in depth boxes, to the last page, down to bottom,
// with its bottom border if last. The part is clipped by the page; one with no width (margins
// wider than the page) or no height is left out.
func (layout *paginator) placeBorderBoxPart(part borderBoxPart, bottom int, last bool, depth int) {
	borderBox := part.borderBox
	left, right := layout.content.Min.X+layout.settings.em(borderBox.LeftEdge), layout.content.Max.X-layout.settings.em(borderBox.RightEdge)
	if left >= right || part.top >= bottom {
		return
	}
	placed := PlacedBorderBox{
		Rect:  image.Rect(left, part.top, right, bottom).Intersect(image.Rect(0, 0, layout.settings.Width, layout.settings.Height)),
		Right: layout.placedBorder(borderBox.Right), Left: layout.placedBorder(borderBox.Left),
		Background: borderBox.Background, Filled: borderBox.Filled, Depth: depth,
	}
	if part.first {
		placed.Top = layout.placedBorder(borderBox.Top)
	}
	if last {
		placed.Bottom = layout.placedBorder(borderBox.Bottom)
	}
	if placed.Rect.Empty() {
		return
	}
	page := &layout.pages[len(layout.pages)-1]
	page.Boxes = append(page.Boxes, placed)
}

// placedBorder returns border with its width in pixels, or no border if it is not drawn.
func (layout *paginator) placedBorder(border document.Border) document.Border {
	border.Width = layout.borderPixels(border)
	if border.Width == 0 {
		return document.Border{}
	}
	return border
}

// drawBorderBoxes draws boxes into dst, the outer ones first so that the inner ones show on top:
// each box's background, then its border.
func drawBorderBoxes(dst *image.Gray, boxes []PlacedBorderBox) {
	if len(boxes) == 0 {
		return
	}
	sorted := slices.Clone(boxes)
	slices.SortStableFunc(sorted, func(first, second PlacedBorderBox) int { return first.Depth - second.Depth })
	for _, borderBox := range sorted {
		rect := borderBox.Rect
		if borderBox.Filled {
			fill(dst, rect, borderBox.Background)
		}
		drawSide(dst, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+borderBox.Top.Width), borderBox.Top, true)
		drawSide(dst, image.Rect(rect.Min.X, rect.Max.Y-borderBox.Bottom.Width, rect.Max.X, rect.Max.Y), borderBox.Bottom, true)
		drawSide(dst, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+borderBox.Left.Width, rect.Max.Y), borderBox.Left, false)
		drawSide(dst, image.Rect(rect.Max.X-borderBox.Right.Width, rect.Min.Y, rect.Max.X, rect.Max.Y), borderBox.Right, false)
	}
}

func fill(dst *image.Gray, area image.Rectangle, gray uint8) {
	draw.Draw(dst, area, image.NewUniform(color.Gray{Y: gray}), image.Point{}, draw.Src)
}

// drawSide draws a side of a border in area, horizontally or vertically: a double border as two
// lines a third of its width each, a dotted one as squares, a dashed one as dashes three times as
// long as they are wide.
func drawSide(dst *image.Gray, area image.Rectangle, border document.Border, horizontal bool) {
	if border.Style == document.NoBorder || area.Empty() {
		return
	}
	width := border.Width
	switch border.Style {
	case document.Double:
		if width < 3 {
			break
		}
		line := (width + 1) / 3
		if horizontal {
			fill(dst, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+line), border.Gray)
			fill(dst, image.Rect(area.Min.X, area.Max.Y-line, area.Max.X, area.Max.Y), border.Gray)
		} else {
			fill(dst, image.Rect(area.Min.X, area.Min.Y, area.Min.X+line, area.Max.Y), border.Gray)
			fill(dst, image.Rect(area.Max.X-line, area.Min.Y, area.Max.X, area.Max.Y), border.Gray)
		}
		return
	case document.Dotted, document.Dashed:
		dash, gap := max(width, 2), max(width, 2)
		if border.Style == document.Dashed {
			dash, gap = 3*dash, 2*gap
		}
		start, end := area.Min.Y, area.Max.Y
		if horizontal {
			start, end = area.Min.X, area.Max.X
		}
		for position := start; position < end; position += dash + gap {
			if horizontal {
				fill(dst, image.Rect(position, area.Min.Y, min(position+dash, end), area.Max.Y), border.Gray)
			} else {
				fill(dst, image.Rect(area.Min.X, position, area.Max.X, min(position+dash, end)), border.Gray)
			}
		}
		return
	}
	fill(dst, area, border.Gray)
}
