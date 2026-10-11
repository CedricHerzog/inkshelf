package mathml

import (
	"math"
	"strconv"
	"strings"
)

// length returns a node's length attribute in pixels at a style's size, value if it has none
// or one that doesn't parse.
func (layout *layouter) length(node *Node, name string, current style, value int) int {
	if attribute, ok := node.Attributes[name]; ok {
		if name == "columnspacing" || name == "rowspacing" {
			attribute = listAt(attribute, 0)
		}
		return parseLength(attribute, current.size, value)
	}
	return value
}

// namedSpaces are MathML's named lengths, in 18ths of an em.
var namedSpaces = map[string]float64{
	"veryverythinmathspace": 1, "verythinmathspace": 2, "thinmathspace": 3, "mediummathspace": 4,
	"thickmathspace": 5, "verythickmathspace": 6, "veryverythickmathspace": 7,
}

// parseLength returns a length attribute in pixels at size: in em, ex, px (CSS's, 16 to an
// em), pt, pc, in, cm, mm, %, a named space, or a number, a multiple of reference;
// reference if it doesn't parse. Lengths are bounded to maxLength ems either way.
func parseLength(attribute string, size, reference int) int {
	attribute = strings.TrimSpace(strings.ToLower(attribute))
	negative := strings.HasPrefix(attribute, "negative")
	if spaces, ok := namedSpaces[strings.TrimPrefix(attribute, "negative")]; ok {
		if negative {
			spaces = -spaces
		}
		return clampLength(spaces*float64(size)/18, size)
	}
	number, unit := splitNumber(attribute)
	if math.IsNaN(number) {
		return reference
	}
	em := float64(size)
	perUnit := map[string]float64{
		"em": em, "ex": em * 0.44, "px": em / 16, "pt": em / 12, "pc": em, "in": em * 6,
		"cm": em * 6 / 2.54, "mm": em * 6 / 25.4, "%": float64(reference) / 100, "": float64(reference),
	}
	scale, ok := perUnit[unit]
	if !ok {
		return reference
	}
	return clampLength(number*scale, size)
}

// splitNumber splits a length into its number and its unit; the number is NaN if there is
// none.
func splitNumber(attribute string) (float64, string) {
	end := strings.IndexFunc(attribute, func(character rune) bool {
		return !(character >= '0' && character <= '9' || character == '.' || character == '-' || character == '+')
	})
	if end < 0 {
		end = len(attribute)
	}
	number, err := strconv.ParseFloat(attribute[:end], 64)
	if err != nil {
		return math.NaN(), ""
	}
	return number, strings.TrimSpace(attribute[end:])
}

// clampLength rounds a length in pixels, bounded to maxLength ems at size.
func clampLength(length float64, size int) int {
	limit := float64(maxLength * size)
	if math.IsNaN(length) {
		return 0
	}
	return int(math.Round(min(max(length, -limit), limit)))
}
