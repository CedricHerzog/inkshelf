package mathml

import "strings"

// padded lays out an <mpadded>: its content in a box of another size, perhaps moved.
func (layout *layouter) padded(node *Node, content *box, current style) *box {
	attributes := node.Attributes
	laid := &box{width: content.width, ascent: content.ascent, descent: content.descent}
	pseudo := map[string]int{"width": content.width, "height": content.ascent, "depth": content.descent}
	dimension := func(name string, value int) int {
		if attributes[name] == "" {
			return value
		}
		return max(paddedLength(attributes[name], value, pseudo, current.size), 0)
	}
	laid.width = dimension("width", laid.width)
	laid.ascent = dimension("height", laid.ascent)
	laid.descent = dimension("depth", laid.descent)
	left := 0
	if attributes["lspace"] != "" {
		left = paddedLength(attributes["lspace"], 0, pseudo, current.size)
	}
	raise := 0
	if attributes["voffset"] != "" {
		raise = paddedLength(attributes["voffset"], 0, pseudo, current.size)
	}
	laid.add(content, left, -raise)
	return laid
}

// paddedLength returns an <mpadded>'s length attribute against value: a length, or a number of
// a pseudo-unit (width, height, depth) or a percentage, of which a signed one is added to value.
func paddedLength(attribute string, value int, pseudo map[string]int, size int) int {
	sign := 0
	switch {
	case strings.HasPrefix(attribute, "+"):
		sign, attribute = 1, attribute[1:]
	case strings.HasPrefix(attribute, "-"):
		sign, attribute = -1, attribute[1:]
	}
	attribute = strings.TrimSpace(attribute)
	length := 0
	number, unit := splitNumber(attribute)
	if reference, ok := pseudo[unit]; ok {
		length = clampLength(number*float64(reference), size)
	} else if unit == "%" {
		length = clampLength(number*float64(value)/100, size)
	} else {
		length = parseLength(attribute, size, 0)
	}
	if sign == 0 {
		return length
	}
	return value + sign*length
}
