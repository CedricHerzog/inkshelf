package mathml

import "strings"

// row lays nodes out in a row: stretchy fences, laid out last, as tall as the rest.
func (layout *layouter) row(nodes []*Node, current style) *box {
	laidOut := make([]*box, len(nodes))
	ascent, descent := 0, 0
	stretched := false
	for index, node := range nodes {
		if isFence(node) {
			stretched = true
			continue
		}
		laidOut[index] = layout.node(node, current, formAt(index, len(nodes)))
		ascent, descent = max(ascent, laidOut[index].ascent), max(descent, laidOut[index].descent)
	}
	if stretched {
		axis := layout.axis(current)
		for index, node := range nodes {
			if laidOut[index] == nil {
				laidOut[index] = layout.operator(node, layout.restyle(node, current), formAt(index, len(nodes)), ascent-axis, descent+axis)
			}
		}
	}
	laid := &box{}
	for index, child := range laidOut {
		if index > 0 && nodes[index].Name == "mo" && nodes[index].Text == "\u2061" && nodes[index-1].Name == "mi" && runeCount(nodes[index-1].Text) > 1 {
			laid.width += current.size * thinSpace / 18 // sin x
		}
		laid.add(child, laid.width, 0)
		laid.grow(child, 0)
		laid.width += child.width
	}
	if len(laidOut) == 1 {
		laid.italic = laidOut[0].italic
	}
	return laid
}

// isFence reports whether a node is an operator that stretches to the height of its row.
func isFence(node *Node) bool {
	if node.Name != "mo" || !stretchesVertically(node.Text) {
		return false
	}
	return node.Attributes["stretchy"] != "false"
}

// formAt returns the form of an operator at index in a row of count nodes.
func formAt(index, count int) string {
	switch {
	case count > 1 && index == 0:
		return "prefix"
	case count > 1 && index == count-1:
		return "postfix"
	}
	return "infix"
}

// fenced returns the nodes of an <mfenced>'s row: its fences and separators as operators.
func fenced(node *Node) []*Node {
	open, ok := node.Attributes["open"]
	if !ok {
		open = "("
	}
	closing, ok := node.Attributes["close"]
	if !ok {
		closing = ")"
	}
	separators, ok := node.Attributes["separators"]
	if !ok {
		separators = ","
	}
	separatorRunes := []rune(strings.Join(strings.Fields(separators), ""))
	var row []*Node
	if open != "" {
		row = append(row, &Node{Name: "mo", Text: open})
	}
	for index, child := range node.Children {
		if index > 0 && len(separatorRunes) > 0 {
			row = append(row, &Node{Name: "mo", Text: string(separatorRunes[min(index-1, len(separatorRunes)-1)])})
		}
		row = append(row, child)
	}
	if closing != "" {
		row = append(row, &Node{Name: "mo", Text: closing})
	}
	return row
}
