package mathml

import (
	"cmp"
	"image"
	"math"
	"strings"
)

// operator is what an <mo> is, from the operator dictionary: its spaces before and after, in
// 18ths of an em, and how it is drawn.
type operator struct {
	leftSpace, rightSpace int
	// stretchy stretches it to the height of its row (a fence) or the width of what it is over
	// or under (an arrow, a bar).
	stretchy bool
	// largeOp makes it larger in display style; movableLimits puts its limits as scripts out of
	// it.
	largeOp, movableLimits bool
	// accent puts it close over or under what it is on, at the same size.
	accent bool
}

// The spaces of the dictionary, in 18ths of an em.
const (
	thinSpace   = 3
	mediumSpace = 4
	thickSpace  = 5
)

// Kinds of operators of the dictionary.
var (
	fenceOperator    = operator{stretchy: true}
	relationOperator = operator{leftSpace: thickSpace, rightSpace: thickSpace}
	binaryOperator   = operator{leftSpace: mediumSpace, rightSpace: mediumSpace}
	arrowOperator    = operator{leftSpace: thickSpace, rightSpace: thickSpace, stretchy: true}
	separator        = operator{rightSpace: thinSpace}
	sumOperator      = operator{leftSpace: 1, rightSpace: 2, largeOp: true, movableLimits: true}
	integralOperator = operator{rightSpace: 1, largeOp: true}
	limitOperator    = operator{rightSpace: thinSpace, movableLimits: true}
	accentOperator   = operator{stretchy: true, accent: true}
	ordinary         = operator{}
)

// operators is the dictionary, by an <mo>'s text: a small part of MathML's, the operators of
// books. Another operator is a relation's spaced.
var operators = map[string]operator{}

func init() {
	for _, group := range []struct {
		kind       operator
		characters string
	}{
		{fenceOperator, "()[]{}|‖⟨⟩⌈⌉⌊⌋〈〉"},
		{relationOperator, "=≠<>≤≥≦≧≈≡≢∼≃≅∝∈∉∋⊂⊃⊆⊇⊄≪≫⊥∣∥≺≻⪯⪰⊢⊨:≔≐≜"},
		{arrowOperator, "→←↔⇒⇐⇔↦⟶⟵⟷⟹⟸⟺↑↓⇑⇓⇀↽⇌"},
		{binaryOperator, "+−-±∓×÷·⋅∗∘∙∪∩∧∨⊕⊗⊖⊙∖⋆⨯"},
		{separator, ",;"},
		{sumOperator, "∑∏∐⋃⋂⨁⨂⨀⨄⋀⋁"},
		{integralOperator, "∫∬∭∮∯∰"},
		{accentOperator, "^ˆˇ˜~¯‾_˙¨´`\u20d7⏞⏟⏜⏝⎴⎵︷︸"},
		{ordinary, "!′″‴'∂∇¬∀∃∄…⋯⋮⋱°%/.\u2061\u2062\u2063\u2064"},
	} {
		for _, character := range group.characters {
			operators[string(character)] = group.kind
		}
	}
	// In a fraction or a script, -x and 2/3: no spaces.
	operators["/"] = operator{}
	operators[":="] = relationOperator
	operators["::"] = relationOperator
	for _, name := range []string{"lim", "max", "min", "sup", "inf", "limsup", "liminf", "det", "gcd", "Pr"} {
		operators[name] = limitOperator
	}
}

// operatorOf returns what an <mo> of text is at its place in a row (form "prefix", "infix" or
// "postfix"), its attributes overriding the dictionary.
func operatorOf(node *Node, form string, size int) operator {
	found, ok := operators[node.Text]
	if !ok {
		found = relationOperator
		if node.Text == "" || isLetters(node.Text) {
			// A word (a function's name) as an operator: spaced as one.
			found = operator{rightSpace: thinSpace}
		}
	}
	if form = cmp.Or(node.Attributes["form"], form); form != "infix" && (found == binaryOperator || found == relationOperator) {
		// A sign before its operand, a factorial after: no spaces.
		found.leftSpace, found.rightSpace = 0, 0
	}
	for name, flag := range map[string]*bool{"stretchy": &found.stretchy, "largeop": &found.largeOp,
		"movablelimits": &found.movableLimits, "accent": &found.accent} {
		switch node.Attributes[name] {
		case "true":
			*flag = true
		case "false":
			*flag = false
		}
	}
	if value, ok := node.Attributes["lspace"]; ok {
		found.leftSpace = lengthIn18ths(value, size)
	}
	if value, ok := node.Attributes["rspace"]; ok {
		found.rightSpace = lengthIn18ths(value, size)
	}
	return found
}

// lengthIn18ths returns a space attribute in 18ths of an em.
func lengthIn18ths(value string, size int) int {
	return parseLength(value, size, 0) * 18 / max(size, 1)
}

// isLetters reports whether text is letters only.
func isLetters(text string) bool {
	return strings.IndexFunc(text, func(character rune) bool {
		return !('a' <= character && character <= 'z' || 'A' <= character && character <= 'Z')
	}) < 0
}

// verticalFences are the characters a stretchy operator in a row stretches to its height.
const verticalFences = "()[]{}|‖⟨⟩⌈⌉⌊⌋〈〉/\\↑↓⇑⇓"

// stretchesVertically reports whether an operator of text stretches to the height of its row.
func stretchesVertically(text string) bool {
	return text != "" && runeCount(text) == 1 && strings.Contains(verticalFences, text)
}

// isInvisible reports whether text is an invisible operator, drawn as nothing: function
// application, invisible times, separator and plus.
func isInvisible(text string) bool {
	return text == "\u2061" || text == "\u2062" || text == "\u2063" || text == "\u2064"
}

// operator lays out an <mo>, its ink stretched over stretch pixels above the axis and under it
// if it is a stretchy fence and they are larger than its glyph.
func (layout *layouter) operator(node *Node, current style, form string, above, below int) *box {
	if isInvisible(node.Text) {
		return &box{}
	}
	found := operatorOf(node, form, current.size)
	variant, text := variantOf(node, current, node.Text)
	size := current.size
	if found.largeOp {
		scales := largeOperatorScales[strings.ContainsAny(text, "∫∬∭∮∯∰")]
		scale := scales[1]
		if current.display {
			scale = scales[0]
		}
		size = int(math.Round(float64(size) * scale))
	}
	laid := layout.text(text, variant, size, false)
	axis := layout.axis(current)
	switch {
	case found.largeOp:
		// Centred on the axis.
		shift := (laid.ascent-laid.descent)/2 - axis
		laid.items[0].at.Y += shift
		laid.ascent, laid.descent = laid.ascent-shift, laid.descent+shift
	case found.stretchy && stretchesVertically(text) && (above > 0 || below > 0):
		layout.stretchFence(laid, max(above, below), axis)
	}
	if current.scriptLevel > 0 && found.rightSpace != thinSpace {
		// Spaces in scripts are left out, but after a comma.
		found.leftSpace, found.rightSpace = 0, 0
	}
	leftSpace, rightSpace := found.leftSpace*current.size/18, found.rightSpace*current.size/18
	if leftSpace != 0 {
		moved := &box{width: leftSpace}
		moved.add(laid, leftSpace, 0)
		laid.items = moved.items
	}
	laid.width += leftSpace + rightSpace
	return laid
}

// stretchFence stretches a fence's ink to half pixels on each side of the axis, if it is
// shorter: its glyph is drawn larger, up to a quarter, then stretched up and down.
func (layout *layouter) stretchFence(laid *box, half, axis int) {
	text, size := laid.items[0].text, laid.items[0].size
	inkHeight := laid.ascent + laid.descent
	if inkHeight <= 0 || 2*half <= inkHeight*11/10 {
		return
	}
	size = int(float64(size) * min(float64(2*half)/float64(inkHeight), 1.25))
	width, _ := layout.fonts.Measure(text, laid.items[0].variant, size)
	laid.width, laid.ascent, laid.descent = width, half+axis, half-axis
	laid.items[0] = item{kind: stretchItem, text: text, variant: laid.items[0].variant, size: size, vertical: true,
		rect: image.Rect(0, -laid.ascent, width, laid.descent)}
}
