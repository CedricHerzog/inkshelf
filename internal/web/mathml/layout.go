package mathml

import (
	"image"
	"math"
	"strconv"
)

// Variant is how a token's text is drawn: a set of bits.
type Variant uint8

// The variant bits.
const (
	Bold Variant = 1 << iota
	Italic
)

// Fonts are the fonts a formula is laid out and drawn in.
type Fonts interface {
	// Measure returns the width of text in variant at size pixels, and the rectangle its ink
	// covers, drawn from the origin on its baseline (empty if it has none).
	Measure(text string, variant Variant, size int) (width int, ink image.Rectangle)
	// Draw draws text in variant at size pixels into dst, in black, the left end of its baseline
	// at origin.
	Draw(dst *image.Gray, text string, variant Variant, size int, origin image.Point)
}

// Sizes and spaces, in ems of the font size where they are used.
const (
	// scriptScale is how much a script level shrinks the font.
	scriptScale = 0.71
	// minScale is the smallest font of a formula, in its size: two script levels.
	minScale = 0.5
	// maxScale is the largest, against mathsize and negative script levels.
	maxScale = 4.0
	// maxLength bounds a length of an attribute, in ems.
	maxLength = 50
	ruleScale = 0.06 // a fraction bar, a radical's bar
	// The shifts of a fraction's numerator and denominator, at least.
	numeratorShift, displayNumeratorShift     = 0.40, 0.68
	denominatorShift, displayDenominatorShift = 0.35, 0.69
	fractionSide                              = 0.06 // the space on each side of a fraction
	fractionPadding                           = 0.08 // how much wider than its parts its bar is
	// The shifts of scripts: at least, and relative to their base's ink.
	superscriptShift, superscriptDrop = 0.40, 0.30
	subscriptShift, subscriptRise     = 0.20, 0.40
	scriptGap                         = 0.11 // between a superscript's ink and the baseline
	scriptSpace                       = 0.05 // after a script
	italicCorrection                  = 0.07 // after an italic letter, before its superscript
	limitGap, accentGap               = 0.12, 0.05
	tableColumnGap, tableRowGap       = 0.8, 0.4
	frameSpacing                      = 0.4
	encloseSpacing                    = 0.15
)

// The sizes of large operators, in their font size: in display style and in a line.
var largeOperatorScales = map[bool][2]float64{
	false: {1.45, 1.1}, // sums, products…
	true:  {1.9, 1.3},  // integrals
}

// Formula is a formula laid out: its box, and what draws it.
type Formula struct {
	// Width, Ascent and Descent are its size: from its left end, above and below its baseline.
	Width, Ascent, Descent int
	items                  []item
	fonts                  Fonts
}

// Layout lays root out in fonts at size pixels: the size of the text around it.
func Layout(root *Node, fonts Fonts, size int) *Formula {
	size = max(size, 1)
	layout := &layouter{fonts: fonts, minSize: max(int(float64(size)*minScale), 1), maxSize: int(float64(size) * maxScale),
		axes: map[int]int{}}
	laid := layout.node(root, style{size: size, display: root.Display()}, "infix")
	return &Formula{Width: max(laid.width, 0), Ascent: max(laid.ascent, 0), Descent: max(laid.descent, 0), items: laid.items, fonts: fonts}
}

// layouter lays a formula out.
type layouter struct {
	fonts            Fonts
	minSize, maxSize int
	axes             map[int]int // by font size
}

// style is what the elements around a node make of it.
type style struct {
	size        int
	scriptLevel int
	display     bool
	variant     string // mathvariant, "" for the default
}

// em returns ems at the style's size in pixels.
func (current style) em(ems float64) int { return int(math.Round(ems * float64(current.size))) }

const (
	textItem    itemKind = iota
	ruleItem             // a black rectangle
	strokeItem           // a polyline
	stretchItem          // a glyph stretched to a rectangle
)

// axis returns the height of the math axis at a style's size: the middle of a minus sign,
// where fraction bars go.
func (layout *layouter) axis(current style) int {
	if axis, ok := layout.axes[current.size]; ok {
		return axis
	}
	_, ink := layout.fonts.Measure("+", 0, current.size)
	axis := -(ink.Min.Y + ink.Max.Y) / 2
	if ink.Empty() {
		axis = current.em(0.25)
	}
	layout.axes[current.size] = axis
	return axis
}

// thickness returns the thickness of a fraction bar at a style's size.
func thickness(current style) int { return max(current.em(ruleScale), 1) }

// node lays a node out; form is an <mo>'s place in its row.
func (layout *layouter) node(node *Node, current style, form string) *box {
	current = layout.restyle(node, current)
	children := node.Children
	switch node.Name {
	case "mi", "mn", "mtext", "ms":
		return layout.token(node, current)
	case "mo":
		return layout.operator(node, current, form, 0, 0)
	case "mspace":
		return &box{width: layout.length(node, "width", current, 0), ascent: layout.length(node, "height", current, 0),
			descent: layout.length(node, "depth", current, 0)}
	case "mfrac":
		return layout.fraction(node, current)
	case "msqrt":
		return layout.radical(layout.row(children, current), nil, current)
	case "mroot":
		index := &box{}
		if len(children) > 1 {
			index = layout.node(children[1], current.script(2), "infix")
		}
		return layout.radical(layout.row(first(children), current), index, current)
	case "msub", "msup", "msubsup":
		return layout.scripts(node, current)
	case "munder", "mover", "munderover":
		return layout.limits(node, current)
	case "mmultiscripts":
		return layout.multiscripts(node, current)
	case "mtable":
		return layout.table(node, current)
	case "mfenced":
		return layout.row(fenced(node), current)
	case "mphantom":
		laid := layout.row(children, current)
		laid.items = nil
		return laid
	case "mpadded":
		return layout.padded(node, layout.row(children, current), current)
	case "menclose":
		return layout.enclose(node, current)
	case "semantics":
		return layout.row(first(children), current)
	case "maction":
		selected, _ := strconv.Atoi(node.Attributes["selection"])
		if selected < 1 || selected > len(children) {
			selected = 1
		}
		return layout.row(children[min(selected-1, len(children)):min(selected, len(children))], current)
	case "annotation", "annotation-xml", "none", "mprescripts", "mglyph", "malignmark", "maligngroup":
		return &box{}
	}
	// math, mrow, mstyle, mtd, merror and what MathML doesn't define: a row.
	return layout.row(children, current)
}

// first returns the first of nodes, or none.
func first(nodes []*Node) []*Node { return nodes[:min(len(nodes), 1)] }

// restyle returns the style of a node's content: its displaystyle, scriptlevel, mathsize and
// mathvariant.
func (layout *layouter) restyle(node *Node, current style) style {
	attributes := node.Attributes
	if attributes == nil {
		return current
	}
	switch attributes["displaystyle"] {
	case "true":
		current.display = true
	case "false":
		current.display = false
	}
	if node.Name == "math" && node.Display() {
		current.display = true
	}
	if level := attributes["scriptlevel"]; level != "" {
		if number, err := strconv.Atoi(level); err == nil {
			change := number
			if level[0] != '+' && level[0] != '-' {
				change = number - current.scriptLevel
			}
			display := current.display
			current = current.script(max(min(change, 8), -8))
			current.display = display
			current.size = min(max(current.size, layout.minSize), layout.maxSize)
		}
	}
	if size := attributes["mathsize"]; size != "" {
		switch size {
		case "small":
			current.size = current.size * 80 / 100
		case "big":
			current.size = current.size * 120 / 100
		case "normal":
		default:
			current.size = parseLength(size, current.size, current.size)
		}
		current.size = min(max(current.size, layout.minSize), layout.maxSize)
	}
	if variant := attributes["mathvariant"]; variant != "" {
		current.variant = variant
	}
	return current
}

// script returns the style levels script levels down (up if negative): smaller by scriptScale
// each, not in display style.
func (current style) script(levels int) style {
	current.scriptLevel += levels
	current.display = false
	current.size = max(int(math.Round(float64(current.size)*math.Pow(scriptScale, float64(levels)))), 1)
	return current
}

// scriptStyle returns the style of a node's scripts: a level down, but no smaller than the
// formula's smallest.
func (layout *layouter) scriptStyle(current style) style {
	scripted := current.script(1)
	scripted.size = min(max(scripted.size, layout.minSize), layout.maxSize)
	return scripted
}
