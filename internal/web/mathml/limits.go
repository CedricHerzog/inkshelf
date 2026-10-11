package mathml

import (
	"image"
	"strings"
)

// limits lays out an <munder>, <mover> or <munderover>: its scripts centred under and over its
// base. Out of display style, the limits of an operator such as ∑ move after it, as scripts.
func (layout *layouter) limits(node *Node, current style) *box {
	core := coreOperator(node.Children)
	underIndex, overIndex := limitIndexes(node.Name)
	accentUnder, accentOver := accentsOf(node, underIndex, overIndex)
	scriptStyle := layout.scriptStyle(current)
	if core != nil && !current.display && operatorOf(core, "prefix", current.size).movableLimits {
		return layout.limitsAsScripts(node, underIndex, overIndex, current, scriptStyle)
	}
	base := layout.part(node, 0, current)
	styleOf := func(accent bool) style {
		if accent {
			// An accent keeps the size of its base.
			accented := current
			accented.display = false
			return accented
		}
		return scriptStyle
	}
	var under, over *box
	if underIndex > 0 {
		under = layout.overUnder(node, underIndex, styleOf(accentUnder), base.width)
	}
	if overIndex > 0 {
		over = layout.overUnder(node, overIndex, styleOf(accentOver), base.width)
	}
	width := base.width
	for _, script := range []*box{under, over} {
		if script != nil {
			width = max(width, script.width)
		}
	}
	laid := &box{width: width}
	laid.add(base, (width-base.width)/2, 0)
	laid.grow(base, 0)
	if over != nil {
		y := -(base.ascent + limitGapOf(accentOver, current) + over.descent)
		laid.add(over, (width-over.width)/2, y)
		laid.grow(over, y)
	}
	if under != nil {
		y := base.descent + limitGapOf(accentUnder, current) + under.ascent
		laid.add(under, (width-under.width)/2, y)
		laid.grow(under, y)
	}
	return laid
}

// limitIndexes returns the indexes, among the children of an <munder>, <mover> or
// <munderover>, of the script under the base and of the script over it; -1 for none.
func limitIndexes(name string) (under, over int) {
	switch name {
	case "mover":
		return -1, 1
	case "munderover":
		return 1, 2
	}
	return 1, -1
}

// accentsOf reports whether the scripts under and over the base are accents, which sit closer
// to the base: as the accent and accentunder attributes say, else as the operator dictionary
// says of the script.
func accentsOf(node *Node, underIndex, overIndex int) (accentUnder, accentOver bool) {
	accentOver = node.Attributes["accent"] == "true" || overIndex > 0 && overIndex < len(node.Children) && isAccent(node.Children[overIndex])
	accentUnder = node.Attributes["accentunder"] == "true" || underIndex > 0 && underIndex < len(node.Children) && isAccent(node.Children[underIndex])
	if node.Attributes["accent"] == "false" {
		accentOver = false
	}
	if node.Attributes["accentunder"] == "false" {
		accentUnder = false
	}
	return accentUnder, accentOver
}

// limitsAsScripts lays out the limits of an operator out of display style: after it, as a
// subscript and a superscript.
func (layout *layouter) limitsAsScripts(node *Node, underIndex, overIndex int, current, scriptStyle style) *box {
	base := layout.part(node, 0, current)
	var subscript, superscript *box
	if underIndex > 0 {
		subscript = layout.part(node, underIndex, scriptStyle)
	}
	if overIndex > 0 {
		superscript = layout.part(node, overIndex, scriptStyle)
	}
	scripts := scriptSet{subscripts: []*box{subscript}, superscripts: []*box{superscript}}
	return layout.attachScripts(base, true, scripts, current, scriptStyle)
}

// limitGapOf returns the space between a base and a limit, smaller for an accent.
func limitGapOf(accent bool, current style) int {
	if accent {
		return current.em(accentGap)
	}
	return current.em(limitGap)
}

// overUnder lays out the script at index of an <munder>, <mover> or <munderover>, an operator
// that stretches across stretched to width.
func (layout *layouter) overUnder(node *Node, index int, current style, width int) *box {
	if index >= len(node.Children) {
		return &box{}
	}
	script := node.Children[index]
	current = layout.restyle(script, current)
	if script.Name != "mo" || script.Attributes["stretchy"] == "false" || !strings.Contains(horizontalStretchy, script.Text) || runeCount(script.Text) != 1 {
		return layout.node(script, current, "infix")
	}
	variant, text := variantOf(script, current, script.Text)
	laid := layout.text(text, variant, current.size, false)
	if laid.width >= width {
		return laid
	}
	if strings.Contains(bars, text) {
		// A bar: a rule, as thick as a fraction's, where the glyph's ink is.
		bar := thickness(current)
		middle := (laid.descent - laid.ascent) / 2
		laid.items = nil
		laid.rule(image.Rect(0, middle-bar/2, width, middle-bar/2+bar))
		laid.width, laid.ascent, laid.descent = width, -(middle - bar/2), middle-bar/2+bar
		return laid
	}
	laid.items[0] = item{kind: stretchItem, text: text, variant: variant, size: current.size,
		rect: image.Rect(0, -laid.ascent, width, laid.descent)}
	laid.width = width
	return laid
}

// horizontalStretchy are the characters stretched across what they are over or under, bars
// the ones drawn as rules.
const (
	horizontalStretchy = "‾¯_−-=→←↔⇒⇐⇔⟶⟵⟷↦^ˆ~˜ˇ⏞⏟⏜⏝⎴⎵︷︸⃗"
	bars               = "‾¯_−-"
)

// isAccent reports whether a node is an operator the dictionary makes an accent.
func isAccent(node *Node) bool {
	if node.Name != "mo" {
		return false
	}
	if accent := node.Attributes["accent"]; accent != "" {
		return accent == "true"
	}
	return operators[node.Text].accent
}

// coreOperator returns the <mo> that nodes, a base, is or embellishes, nil if none.
func coreOperator(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}
	node := nodes[0]
	switch node.Name {
	case "mo":
		return node
	case "mrow", "mstyle", "mpadded", "semantics", "msub", "msup", "msubsup", "munder", "mover", "munderover", "mfrac":
		if node.Name == "mrow" || node.Name == "mstyle" || node.Name == "mpadded" {
			if len(node.Children) != 1 {
				return nil
			}
		}
		return coreOperator(node.Children)
	}
	return nil
}
