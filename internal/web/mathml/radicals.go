package mathml

import "image"

// radical lays out a square root of content, or a root of index: a radical sign, a bar over
// the content.
func (layout *layouter) radical(content, index *box, current style) *box {
	bar := thickness(current)
	gap := bar + bar/4
	if current.display {
		gap = bar + current.em(0.11)
	}
	top := -(content.ascent + gap + bar)
	bottom := max(content.descent, 0) + current.em(0.04)
	height := float64(bottom - top)
	em := float64(current.size)
	// The sign: a tick, a heavy stroke down, a long one up to the bar.
	hook, foot := 0.18*em, 0.38*em
	signWidth := foot + max(0.22*em, height*0.18)
	left := 0.0
	indentation := 0
	if index != nil && index.width > 0 {
		// The index's right end over the tick, raised to 60% of the sign.
		right := int(hook + 0.12*em)
		indentation = max(index.width-right, 0)
		left = float64(indentation)
	}
	bottomY, topY := float64(bottom), float64(top)+float64(bar)/2
	tickY := bottomY - 0.42*em
	laid := &box{}
	weight := float64(bar)
	laid.stroke(weight, point{left, tickY + 0.06*em}, point{left + hook, tickY})
	laid.stroke(weight*2.2, point{left + hook, tickY}, point{left + foot, bottomY})
	laid.stroke(weight, point{left + foot, bottomY}, point{left + signWidth, topY})
	contentX := int(left+signWidth) + current.em(0.05)
	laid.add(content, contentX, 0)
	laid.width = contentX + content.width + current.em(0.05)
	laid.rule(image.Rect(int(left+signWidth)-bar/2, top, laid.width, top+bar))
	laid.ascent, laid.descent = -top, max(bottom+bar, content.descent)
	if index != nil && index.width > 0 {
		raise := int(-(bottomY - height*0.6)) // the index's baseline, up from the formula's
		y := -raise - index.descent
		laid.add(index, int(left)+int(hook+0.12*em)-index.width, y)
		laid.grow(index, y)
	}
	return laid
}
