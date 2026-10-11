package mathml

import (
	"cmp"
	"image"
	"strings"
)

// enclose lays out an <menclose>: its content with a box, sides or strikes around it, or under
// a radical.
func (layout *layouter) enclose(node *Node, current style) *box {
	notations := strings.Fields(cmp.Or(node.Attributes["notation"], "longdiv"))
	content := layout.row(node.Children, current)
	for _, notation := range notations {
		if notation == "radical" {
			content = layout.radical(content, nil, current)
		}
	}
	bar := thickness(current)
	padding := current.em(encloseSpacing)
	laid := &box{width: content.width + 2*padding + 2*bar, ascent: content.ascent + padding + bar, descent: content.descent + padding + bar}
	laid.add(content, padding+bar, 0)
	frame := image.Rect(0, -laid.ascent, laid.width, laid.descent)
	for _, notation := range notations {
		laid.items = append(laid.items, frameItems(frame, bar, notation)...)
	}
	return laid
}

// frameItems returns the items that draw a notation of <menclose> around frame, bar thick.
func frameItems(frame image.Rectangle, bar int, notation string) []item {
	sides := map[string]image.Rectangle{
		"top":    image.Rect(frame.Min.X, frame.Min.Y, frame.Max.X, frame.Min.Y+bar),
		"bottom": image.Rect(frame.Min.X, frame.Max.Y-bar, frame.Max.X, frame.Max.Y),
		"left":   image.Rect(frame.Min.X, frame.Min.Y, frame.Min.X+bar, frame.Max.Y),
		"right":  image.Rect(frame.Max.X-bar, frame.Min.Y, frame.Max.X, frame.Max.Y),
	}
	named := map[string][]string{
		"box": {"top", "bottom", "left", "right"}, "roundedbox": {"top", "bottom", "left", "right"},
		"circle": {"top", "bottom", "left", "right"}, "longdiv": {"top", "left"}, "actuarial": {"top", "right"},
		"madruwb": {"bottom", "right"}, "top": {"top"}, "bottom": {"bottom"}, "left": {"left"}, "right": {"right"},
	}
	var items []item
	for _, side := range named[notation] {
		items = append(items, item{kind: ruleItem, rect: sides[side]})
	}
	left, right := float64(frame.Min.X), float64(frame.Max.X)
	top, bottom := float64(frame.Min.Y), float64(frame.Max.Y)
	middleX, middleY := (left+right)/2, (top+bottom)/2
	strikes := map[string][2]point{
		"updiagonalstrike": {{left, bottom}, {right, top}}, "updiagonalarrow": {{left, bottom}, {right, top}},
		"downdiagonalstrike": {{left, top}, {right, bottom}},
		"verticalstrike":     {{middleX, top}, {middleX, bottom}}, "horizontalstrike": {{left, middleY}, {right, middleY}},
	}
	if strike, ok := strikes[notation]; ok {
		items = append(items, item{kind: strokeItem, points: strike[:], thickness: float64(bar)})
	}
	return items
}
