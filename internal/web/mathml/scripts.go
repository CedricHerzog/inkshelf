package mathml

import "slices"

// scriptSet is the scripts of a base, laid out, in pairs: subscripts[i] goes under
// superscripts[i], and a nil one is missing. The prescripts go before the base, the others
// after it.
type scriptSet struct {
	subscripts, superscripts       []*box
	presubscripts, presuperscripts []*box
}

// scripts lays out an <msub>, <msup> or <msubsup>: its scripts after its base, lowered and
// raised.
func (layout *layouter) scripts(node *Node, current style) *box {
	base := layout.part(node, 0, current)
	scriptStyle := layout.scriptStyle(current)
	var subscript, superscript *box
	switch node.Name {
	case "msub":
		subscript = layout.part(node, 1, scriptStyle)
	case "msup":
		superscript = layout.part(node, 1, scriptStyle)
	default:
		subscript, superscript = layout.part(node, 1, scriptStyle), layout.part(node, 2, scriptStyle)
	}
	scripts := scriptSet{subscripts: []*box{subscript}, superscripts: []*box{superscript}}
	return layout.attachScripts(base, isToken(node.Children, current), scripts, current, scriptStyle)
}

// multiscripts lays out an <mmultiscripts>: its base, then pairs of scripts after it, then,
// after <mprescripts/>, pairs of scripts before it.
func (layout *layouter) multiscripts(node *Node, current style) *box {
	base := layout.part(node, 0, current)
	scriptStyle := layout.scriptStyle(current)
	var post, pre [2][]*box
	target := &post
	pairIndex := 0
	for _, child := range node.Children[min(1, len(node.Children)):] {
		if child.Name == "mprescripts" {
			target, pairIndex = &pre, 0
			continue
		}
		var laid *box
		if child.Name != "none" {
			laid = layout.node(child, scriptStyle, "infix")
		}
		target[pairIndex%2] = append(target[pairIndex%2], laid)
		pairIndex++
	}
	scripts := scriptSet{subscripts: post[0], superscripts: post[1], presubscripts: pre[0], presuperscripts: pre[1]}
	return layout.attachScripts(base, isToken(node.Children, current), scripts, current, scriptStyle)
}

// isToken reports whether the first of nodes, a base, is a token. The scripts of a token are
// placed at fixed heights; those of any other base (but a large operator) follow its ink.
func isToken(nodes []*Node, current style) bool {
	if len(nodes) == 0 {
		return false
	}
	switch nodes[0].Name {
	case "mi", "mn", "mtext":
		return true
	case "mo":
		return !operatorOf(nodes[0], "prefix", current.size).largeOp
	}
	return false
}

// attachScripts places scripts around base: the prescripts before it, the others after it,
// each superscript raised and each subscript lowered by the same amount.
func (layout *layouter) attachScripts(base *box, token bool, scripts scriptSet, current, scriptStyle style) *box {
	up, down := scriptShifts(base, token, scripts, current, scriptStyle)
	placer := scriptPlacer{laid: &box{}, up: up, down: down, space: current.em(scriptSpace)}
	placer.place(scripts.presubscripts, scripts.presuperscripts, 0)
	placer.laid.add(base, placer.x, 0)
	placer.laid.grow(base, 0)
	placer.x += base.width
	correction := 0
	if base.italic {
		// An italic letter leans to the right: its superscript leaves room for it.
		correction = current.em(italicCorrection)
	}
	placer.place(scripts.subscripts, scripts.superscripts, correction)
	placer.laid.width = placer.x
	return placer.laid
}

// scriptShifts returns how far the superscripts are raised and the subscripts lowered: at least
// a fixed amount, enough to clear the base's ink if it is no token, and enough to keep a
// subscript and a superscript apart.
func scriptShifts(base *box, token bool, scripts scriptSet, current, scriptStyle style) (up, down int) {
	up, down = current.em(superscriptShift), current.em(subscriptShift)
	hasSuper, hasSub := false, false
	upRaised, downRaised := 0, 0
	for _, superscript := range slices.Concat(scripts.superscripts, scripts.presuperscripts) {
		if superscript == nil {
			continue
		}
		hasSuper = true
		up = max(up, superscript.descent+current.em(scriptGap))
		upRaised = max(upRaised, superscript.descent)
	}
	for _, subscript := range slices.Concat(scripts.subscripts, scripts.presubscripts) {
		if subscript == nil {
			continue
		}
		hasSub = true
		down = max(down, subscript.ascent-current.em(subscriptRise))
		downRaised = max(downRaised, subscript.ascent)
	}
	if !token {
		up = max(up, base.ascent-scriptStyle.em(superscriptDrop*2))
		down = max(down, base.descent+scriptStyle.em(0.05))
	}
	if hasSub && hasSuper {
		if gap := (up - upRaised) - (downRaised - down); gap < 4*thickness(current) {
			down += 4*thickness(current) - gap
		}
	}
	return up, down
}

// scriptPlacer places columns of scripts in laid from x, superscripts up pixels above the
// baseline and subscripts down pixels below it.
type scriptPlacer struct {
	laid        *box
	x, up, down int
	space       int // after each column
}

// place places pairs of subscripts and superscripts in columns, the superscripts moved right by
// after pixels, and moves x past them.
func (placer *scriptPlacer) place(subscripts, superscripts []*box, after int) {
	for index := range max(len(subscripts), len(superscripts)) {
		width := 0
		for _, script := range []struct {
			laid *box
			y    int
		}{{at(subscripts, index), placer.down}, {at(superscripts, index), -placer.up}} {
			if script.laid == nil {
				continue
			}
			shift := 0
			if script.y < 0 {
				shift = after
			}
			placer.laid.add(script.laid, placer.x+shift, script.y)
			placer.laid.grow(script.laid, script.y)
			width = max(width, script.laid.width+shift)
		}
		placer.x += width + placer.space
	}
}

// at returns boxes[index], or nil past their end.
func at(boxes []*box, index int) *box {
	if index < len(boxes) {
		return boxes[index]
	}
	return nil
}
