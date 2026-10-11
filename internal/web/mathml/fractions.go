package mathml

import "image"

// fraction lays out an <mfrac>: its numerator over its denominator, centred, a bar between
// them on the axis.
func (layout *layouter) fraction(node *Node, current style) *box {
	parts := current
	if parts.display {
		parts.display = false
	} else {
		parts = layout.scriptStyle(current)
	}
	numerator, denominator := layout.part(node, 0, parts), layout.part(node, 1, parts)
	bar := thickness(current)
	if value, ok := node.Attributes["linethickness"]; ok {
		switch value {
		case "thin":
			bar = max(bar/2, 1)
		case "medium":
		case "thick":
			bar *= 2
		default:
			bar = min(max(parseLength(value, current.size, bar), 0), current.em(1))
		}
	}
	axis := layout.axis(current)
	gap := max(bar, 1)
	if current.display {
		gap = 3 * max(bar, 1)
	}
	if bar == 0 {
		gap *= 3
	}
	up, down := current.em(numeratorShift), current.em(denominatorShift)
	if current.display {
		up, down = current.em(displayNumeratorShift), current.em(displayDenominatorShift)
	}
	up = max(up, axis+(bar+1)/2+gap+numerator.descent)
	down = max(down, -axis+(bar+1)/2+gap+denominator.ascent)
	side, padding := current.em(fractionSide), current.em(fractionPadding)
	inner := max(numerator.width, denominator.width) + 2*padding
	laid := &box{width: inner + 2*side}
	laid.add(numerator, side+(inner-numerator.width)/2, -up)
	laid.add(denominator, side+(inner-denominator.width)/2, down)
	laid.grow(numerator, -up)
	laid.grow(denominator, down)
	if bar > 0 {
		top := -axis - bar/2
		laid.rule(image.Rect(side, top, side+inner, top+bar))
	}
	return laid
}

// part lays out a node's child at index in a style, nothing if it has none.
func (layout *layouter) part(node *Node, index int, current style) *box {
	if index >= len(node.Children) {
		return &box{}
	}
	return layout.node(node.Children[index], current, "infix")
}
