package epub

import (
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// Grays of text and boxes on the e-ink panel, from 0 (black) to 255 (white).
const (
	// legibleGray is the lightest text drawn on a light background. Text lighter than
	// invisibleGray there is drawn black instead: the book meant it for another background (a
	// night mode, an image), and the app always drew text black.
	legibleGray   = 0x70
	invisibleGray = 0xC0
	// A background darker than darkBackground under light text stays dark. Any other background
	// is lightened to lightBackground at least, so that dark text stays readable on it. Text and
	// its background are always at least minContrast apart.
	darkBackground  = 0x80
	lightBackground = 0xC0
	minContrast     = 0x80
	// maxBorderGray is the lightest border drawn on a light background.
	maxBorderGray = 0xC0
)

// Limits on the boxes of a chapter, against hostile books: the width of a border, in
// hundredths of an em; how many boxes may be nested; how many boxes a chapter may have.
const (
	maxBorderWidth = 100
	maxBoxDepth    = 8
	maxBoxes       = 4096
)

// borderWidths are the widths of thin, medium and thick in hundredths of an em: 1, 3 and 5 CSS
// pixels.
var borderWidths = map[string]int{"thin": 100 / pixelsPerEm, "medium": 300 / pixelsPerEm, "thick": 500 / pixelsPerEm}

// borderStyles are the values of border-style that are drawn. The 3D styles are drawn solid.
var borderStyles = map[string]document.BorderStyle{
	"solid": document.Solid, "double": document.Double, "dotted": document.Dotted, "dashed": document.Dashed,
	"groove": document.Solid, "ridge": document.Solid, "inset": document.Solid, "outset": document.Solid,
}

// borderBox is a block element with a border or a background, while it is open. The document
// draws the box under the element's text, which is laid out inside its border and padding.
type borderBox struct {
	drawn                     document.BorderBox
	marginLeft, marginRight   int
	paddingLeft, paddingRight int
	// index is where drawn is in Document.Boxes. saved is what was pending when the box opened:
	// an empty box is left out, and what was pending then goes on the next block instead.
	index int
	saved pending
}

// colorOf returns the color of an element from its declared values, given its parent's color:
// its CSS color, else a <font>'s color attribute. A link is black, as the app draws links,
// unless the book colors it.
func colorOf(opened *element, declared map[string]string, parent css.Color) css.Color {
	value, ok := declared["color"]
	switch {
	case ok && strings.EqualFold(value, "initial"):
		return css.Black
	case ok:
		if parsed, valid := css.ParseColor(value, parent); valid {
			return parsed
		}
	case opened.name == "font" && opened.attributes["color"] != "":
		attribute := strings.TrimSpace(opened.attributes["color"])
		if parsed, valid := css.ParseColor(attribute, parent); valid {
			return parsed
		}
		if parsed, valid := css.ParseColor("#"+attribute, parent); valid {
			return parsed
		}
	case opened.name == "a" && opened.attributes["href"] != "":
		return css.Black
	}
	return parent
}

// blend returns the gray of color drawn over background.
func blend(color css.Color, background uint8) uint8 {
	alpha := int(color.Alpha)
	return uint8((int(color.Gray)*alpha + int(background)*(255-alpha) + 127) / 255)
}

// isLight reports whether text of color would be light on white.
func isLight(color css.Color) bool { return blend(color, 255) > invisibleGray }

// textGray returns the gray of text of color on a background. It is the book's color, but on a
// light background it is no lighter than legibleGray (and black if it is lighter than
// invisibleGray), and it stays minContrast apart from the background.
func textGray(color css.Color, background uint8) uint8 {
	gray := int(blend(color, background))
	if background < darkBackground {
		return uint8(min(max(gray, int(background)+minContrast), 255))
	}
	if gray > invisibleGray {
		return 0
	}
	return uint8(min(gray, legibleGray, int(background)-minContrast))
}

// runGray returns the gray of an element's text. On a pre-paginated page, it is the book's
// color over white, as the page draws its own backgrounds. Elsewhere it is made legible on the
// background of the element's box.
func (parser *builder) runGray(opened *element) uint8 {
	if parser.page != nil {
		return blend(opened.color, 255)
	}
	return textGray(opened.color, opened.background)
}

// borderBoxOf returns the border box of a block element of a reflowable chapter, from its
// declared values, or nil if it has no border or background that shows. The background of html
// and body would be the page's, which stays the reader's own.
func (parser *builder) borderBoxOf(opened *element, declared map[string]string) *borderBox {
	if parser.page != nil || opened.name == "html" || opened.name == "body" ||
		opened.boxDepth >= maxBoxDepth || len(parser.doc.Boxes) >= maxBoxes {
		return nil
	}
	fontSize := opened.style.fontSize
	box := document.BorderBox{
		Top:    parser.borderOf(declared, "top", opened),
		Right:  parser.borderOf(declared, "right", opened),
		Bottom: parser.borderOf(declared, "bottom", opened),
		Left:   parser.borderOf(declared, "left", opened),
	}
	if gray, ok := backgroundGray(declared, opened.color, opened.background); ok {
		box.Background, box.Filled = gray, true
	}
	if !box.Filled && box.Top.Style == document.NoBorder && box.Right.Style == document.NoBorder &&
		box.Bottom.Style == document.NoBorder && box.Left.Style == document.NoBorder {
		return nil
	}
	padding := func(side string) int {
		return min(positive(lengthOf(strings.ToLower(declared["padding-"+side]), fontSize, textWidth)), maxMargin)
	}
	margin := func(side string) int {
		length, _ := lengthOf(strings.ToLower(declared["margin-"+side]), fontSize, textWidth)
		return length
	}
	box.PaddingTop, box.PaddingBottom = padding("top"), padding("bottom")
	box.MarginTop, box.MarginBottom = min(max(margin("top"), 0), maxMargin), min(max(margin("bottom"), 0), maxMargin)
	return &borderBox{
		drawn: box, marginLeft: margin("left"), marginRight: margin("right"),
		paddingLeft: padding("left"), paddingRight: padding("right"),
	}
}

// borderOf returns the border of an element on one side ("top", "right"…) from its declared
// values. A width without a style draws nothing, a style without a width is medium, and the
// border's color is the element's unless it is given.
func (parser *builder) borderOf(declared map[string]string, side string, opened *element) document.Border {
	borderStyle := borderStyles[strings.ToLower(strings.TrimSpace(declared["border-"+side+"-style"]))]
	if borderStyle == document.NoBorder {
		return document.Border{}
	}
	value := strings.ToLower(strings.TrimSpace(declared["border-"+side+"-width"]))
	width, ok := borderWidths[value]
	if value == "" {
		width, ok = borderWidths["medium"], true
	}
	if !ok {
		width, _ = lengthOf(value, opened.style.fontSize, 0)
	}
	if width <= 0 {
		return document.Border{}
	}
	color, ok := css.ParseColor(declared["border-"+side+"-color"], opened.color)
	if !ok {
		color = opened.color
	}
	gray := blend(color, opened.background)
	if opened.background >= darkBackground {
		gray = min(gray, maxBorderGray)
	}
	return document.Border{Style: borderStyle, Width: min(width, maxBorderWidth), Gray: gray}
}

// backgroundGray returns the gray of the background that an element declares, over the
// background it is on (under), given the color of its text. A background stays dark under
// light text; any other is lightened to lightBackground. It returns false if the element has
// no background, or one that doesn't differ from the one under it.
func backgroundGray(declared map[string]string, color css.Color, under uint8) (uint8, bool) {
	parsed, ok := css.ParseColor(declared["background-color"], color)
	if !ok {
		if parsed, ok = css.BackgroundColor(declared["background"], color); !ok {
			return 0, false
		}
	}
	if parsed.Alpha == 0 {
		return 0, false
	}
	gray := blend(parsed, under)
	if gray >= darkBackground || !isLight(color) {
		gray = max(gray, lightBackground)
	}
	return gray, gray != under
}

// openBorderBox starts the border box of a block element, whose block is props, inside the
// block of outer. The box's edges are at outer's text, and the element's text goes inside the
// box's border and padding. The margin and page break that were pending go on the box rather
// than on its first block.
func (parser *builder) openBorderBox(opened *element, props, outer *blockProps) {
	open := opened.borderBox
	box := &open.drawn
	box.LeftEdge = max(outer.left+min(open.marginLeft, maxMargin), 0)
	box.RightEdge = max(outer.right+min(open.marginRight, maxMargin), 0)
	props.left = box.LeftEdge + sideWidth(box.Left) + open.paddingLeft
	props.right = box.RightEdge + sideWidth(box.Right) + open.paddingRight
	props.indent = min(max(opened.style.indent, -props.left), maxMargin)
	props.marginBottom = 0
	box.MarginTop = max(box.MarginTop, parser.pending.marginTop)
	box.PageBreak = parser.pending.pageBreak || opened.style.pageBreakBefore
	open.saved = parser.pending
	parser.pending.marginTop, parser.pending.pageBreak = 0, false
	opened.style.borderTop, opened.style.borderBottom = false, false
	box.Start = parser.doc.Chars
	open.index = len(parser.doc.Boxes)
	parser.doc.Boxes = append(parser.doc.Boxes, *box)
}

// sideWidth returns the width of one side of a border, 0 if that side is not drawn.
func sideWidth(border document.Border) int {
	if border.Style == document.NoBorder {
		return 0
	}
	return border.Width
}

// closeBorderBox ends the border box of a block element, once its text is flushed. A box that
// holds nothing is left out of the document: its margins, page break and borders go on the
// next block instead, a border becoming a rule.
func (parser *builder) closeBorderBox(closed *element) {
	open := closed.borderBox
	if open == nil {
		return
	}
	box := &parser.doc.Boxes[open.index]
	if parser.doc.Chars > box.Start {
		box.End = parser.doc.Chars
		return
	}
	parser.doc.Boxes = parser.doc.Boxes[:open.index]
	saved := open.saved
	parser.pending.marginTop = max(parser.pending.marginTop, saved.marginTop, closed.style.marginTop)
	parser.pending.pageBreak = parser.pending.pageBreak || saved.pageBreak || closed.style.pageBreakBefore
	parser.pending.borderTop = parser.pending.borderTop || box.Top.Style != document.NoBorder
	closed.style.borderBottom = box.Bottom.Style != document.NoBorder
	closed.block.marginBottom = closed.style.marginBottom
}
