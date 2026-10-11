package epub

import (
	"cmp"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// A floated element (CSS float, or an <img align>) is taken out of the text and placed at a
// side of the page, the text flowing around it. The builder makes the blocks of a floated
// element a float group of the document: the paginator lays them out apart and places them at
// the side where the next block starts. Floats inside a floated element float with it.

// floatGroup is a floated element of a reflowable chapter while it is open.
type floatGroup struct {
	root   *element
	side   document.Float
	number int // its document.Block.FloatGroup
	width  int // in percent of the text's width, 0 for its content's
	// outer is what was pending for the next block when the float opened. It goes on the block
	// after the float, as the float's own margins and page breaks go on its first block.
	outer pending
	// splitText is set if text came before the float in its block: the text after the float
	// continues that paragraph.
	splitText bool
}

// startFloat makes an element of a reflowable chapter a float group if CSS floats it (or, for
// an <img>, its align attribute) and it is in no float already. A floated inline element is
// then a block, as CSS has it. A floated table stays in the text.
func (parser *builder) startFloat(opened *element) {
	opened.floating = opened.parent.floating
	side := floatSide(opened)
	if side == document.NoFloat || opened.floating != nil || parser.page != nil || opened.name == "table" ||
		opened.style.display == "table-cell" {
		return
	}
	parser.floatGroups++
	group := &floatGroup{root: opened, side: side, number: parser.floatGroups, splitText: parser.pendingRunes > 0}
	if opened.style.width > 0 {
		group.width = min(max(opened.style.width*100/textWidth, 1), 100)
	}
	opened.floating = group
	if opened.style.display == "inline" && opened.name != "img" && opened.name != "image" {
		opened.style.display = "block"
	}
}

// enterFloat sets up the block of the element that a float group starts at. Its margins are
// those of the text around it, and what was pending for the next block waits for the block
// after the float. Any block element's clear goes on its first block.
func (parser *builder) enterFloat(opened *element, props, outer *blockProps) {
	clearSide := clearOf(opened.style.clear, opened.style.direction)
	group := opened.floating
	if group == nil || group.root != opened {
		parser.pending.clear |= clearSide
		return
	}
	if group.splitText {
		parser.doc.Blocks[len(parser.doc.Blocks)-1].MarginBottom = 0
	}
	group.outer, parser.pending = parser.pending, pending{clear: clearSide}
	props.left, props.right, props.indent = outer.left, outer.right, max(props.indent, 0)
}

// leaveFloat ends a float group when the element it started at closes. The next block gets
// what was pending before the float, and continues the paragraph that the float split.
func (parser *builder) leaveFloat(closed *element) {
	if group := closed.floating; group != nil && group.root == closed {
		parser.pending, parser.continued = group.outer, group.splitText
	}
}

// floatBlock puts block in the float group of the block element it is made in, if any, and
// gives it the floats it clears.
func (parser *builder) floatBlock(block *document.Block, blockElement *element) {
	block.Clear = parser.pending.clear
	if group := blockElement.floating; group != nil {
		block.Float, block.FloatGroup, block.FloatWidth = group.side, group.number, group.width
	}
}

// floatImage adds the image at imagePath of an <img> that floats by itself, and reports whether
// it does. The float is a single image block, as wide as the image; the text around it in its
// block is split there.
func (parser *builder) floatImage(imagePath string, img *element) bool {
	group := img.floating
	if group == nil || group.root != img {
		return false
	}
	width, _, _ := parser.imageSize(imagePath, img)
	if width > 0 {
		group.width = min(max(width*100/textWidth, 1), 100)
	}
	splitText := parser.pendingRunes > 0
	parser.flush()
	if splitText {
		parser.doc.Blocks[len(parser.doc.Blocks)-1].MarginBottom = 0
	}
	parser.doc.Add(document.Block{
		Kind: document.Image, Image: imagePath, ImageWidth: 100, MarginBottom: imageMargin,
		Float: group.side, FloatGroup: group.number, FloatWidth: group.width,
		Clear: clearOf(img.style.clear, img.style.direction), Footnote: noteIDOf(parser.blockElement()),
	})
	parser.continued = parser.continued || splitText
	return true
}

// clearAfter makes the block after a <br clear> start below the floats it clears.
func (parser *builder) clearAfter(br *element) {
	value := strings.ToLower(br.attributes["clear"])
	if value == "all" {
		value = "both"
	}
	parser.pending.clear |= clearOf(cmp.Or(br.style.clear, value), br.style.direction)
}

// floatSide returns the side an element floats to: its CSS float, else an <img>'s align.
func floatSide(opened *element) document.Float {
	value := opened.style.float
	if value == "" && opened.name == "img" {
		value = strings.ToLower(opened.attributes["align"])
	}
	switch value {
	case "left":
		return document.FloatLeft
	case "right":
		return document.FloatRight
	case "inline-start", "inline-end":
		if value == "inline-start" == (opened.style.direction == document.RightToLeft) {
			return document.FloatRight
		}
		return document.FloatLeft
	}
	return document.NoFloat
}

// clearOf returns the sides a clear value clears in text of direction.
func clearOf(value string, direction document.Direction) document.Clear {
	switch value {
	case "left":
		return document.ClearLeft
	case "right":
		return document.ClearRight
	case "both":
		return document.ClearBoth
	case "inline-start", "inline-end":
		if value == "inline-start" == (direction == document.RightToLeft) {
			return document.ClearRight
		}
		return document.ClearLeft
	}
	return 0
}
