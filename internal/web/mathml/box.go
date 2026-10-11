package mathml

import "image"

// box is a node laid out: its size from its left end and its baseline, and the items that
// draw it, placed there.
type box struct {
	width, ascent, descent int
	items                  []item
	// italic is set on an italic letter, which a superscript leaves room after.
	italic bool
}

// itemKind is what an item draws.
type itemKind int

// item is a part of a formula to draw, from its left end and baseline.
type item struct {
	kind    itemKind
	at      image.Point // where a text's baseline starts
	text    string
	variant Variant
	size    int
	// rect is a rule, or the box a stretched glyph's ink fills.
	rect image.Rectangle
	// points are a stroke's, thickness its width; vertical stretches a glyph up and down, not
	// across.
	points    []point
	thickness float64
	vertical  bool
}

// point is a point of a stroke, from the formula's left end and baseline.
type point struct{ x, y float64 }

// add places child's items in the box, the child's baseline's left end at (x, y).
func (laid *box) add(child *box, x, y int) {
	for _, placed := range child.items {
		placed.at = placed.at.Add(image.Pt(x, y))
		placed.rect = placed.rect.Add(image.Pt(x, y))
		if placed.points != nil {
			moved := make([]point, len(placed.points))
			for index, at := range placed.points {
				moved[index] = point{at.x + float64(x), at.y + float64(y)}
			}
			placed.points = moved
		}
		laid.items = append(laid.items, placed)
	}
}

// grow makes the box tall enough for child at y (its baseline from the box's).
func (laid *box) grow(child *box, y int) {
	laid.ascent = max(laid.ascent, child.ascent-y)
	laid.descent = max(laid.descent, child.descent+y)
}

// rule adds a black rectangle.
func (laid *box) rule(rect image.Rectangle) {
	laid.items = append(laid.items, item{kind: ruleItem, rect: rect})
}

// stroke adds a polyline through points, thickness pixels wide.
func (laid *box) stroke(thickness float64, points ...point) {
	laid.items = append(laid.items, item{kind: strokeItem, points: points, thickness: thickness})
}
