package epub

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// The page that stylesheets are laid out for, in CSS pixels: a 6" reader's, as browsers count
// it. Media queries are evaluated against it, and percentages of the width and pixel lengths
// are converted with it, at 16 pixels to the em: the browsers' default font size.
const (
	pageWidth   = 600
	pageHeight  = 800
	pixelsPerEm = 16
)

// screen is the media of the reader.
var screen = css.Media{Type: "screen", Width: pageWidth, Height: pageHeight}

// textWidth is the page's width in hundredths of an em: what 100% of a margin or an indent is.
const textWidth = pageWidth * 100 / pixelsPerEm

// maxMargin is the largest margin or indent an element adds, in hundredths of an em: books
// written for wide screens indent too much for a reader's page.
const maxMargin = 600

// userAgentCSS is the default style of the elements, below the book's stylesheets. Headings'
// margins are in rem, as the app had them before following CSS.
const userAgentCSS = `
html, body, div, p, h1, h2, h3, h4, h5, h6, blockquote, ul, ol, pre, dl, dt, dd, section, article,
header, footer, aside, nav, main, figure, figcaption, center, address, hgroup,
details, summary, form, fieldset, legend { display: block }
li { display: list-item }
table { display: table }
caption { display: table-caption; text-align: center }
thead { display: table-header-group }
tbody { display: table-row-group }
tfoot { display: table-footer-group }
tr { display: table-row }
td, th { display: table-cell }
col { display: table-column }
colgroup { display: table-column-group }
head, script, title, style, link, meta, source, track, param, annotation, annotation-xml, [hidden] { display: none }
p, blockquote, figure { margin-top: 1em; margin-bottom: 1em }
blockquote, dd { margin-left: 1.5rem }
ul, ol { padding-left: 1.5rem }
ul { list-style-type: disc }
ol { list-style-type: decimal }
h1, h2, h3, h4, h5, h6 { font-weight: bold; text-indent: 0 }
h1 { font-size: 1.7em; margin: 1rem 0 }
h2 { font-size: 1.4em; margin: 0.9rem 0 }
h3 { font-size: 1.2em; margin: 0.8rem 0 }
h4 { font-size: 1.05em; margin: 0.7rem 0 }
h5 { font-size: 1em; margin: 0.6rem 0 }
h6 { font-size: 0.9em; margin: 0.6rem 0 }
b, strong, th, dt { font-weight: bold }
i, em, cite, dfn, var, address, math { font-style: italic }
small { font-size: smaller }
sup { vertical-align: super }
sub { vertical-align: sub }
u, ins { text-decoration: underline }
s, strike, del { text-decoration: line-through }
pre, code, tt, kbd, samp { font-family: monospace }
pre { white-space: pre }
center { text-align: center }
[dir=rtl] { direction: rtl }
[dir=ltr] { direction: ltr }
[dir], bdi { unicode-bidi: isolate }
bdo, bdo[dir] { unicode-bidi: isolate-override }
`

// userAgentSheet is userAgentCSS, parsed.
var userAgentSheet = css.Parse(userAgentCSS, screen)

// inherited is the part of an element's style that its children inherit. Lengths are in
// hundredths of an em of the reader's font size, as in package document.
type inherited struct {
	fontSize          int // in percent of the reader's font size
	bold, italic      bool
	smallCaps         bool
	underline, strike bool   // text-decoration: not inherited in CSS, but drawn through the children
	family            string // as document.Run.Family
	transform         string // text-transform
	letterSpacing     int
	align             document.Align
	alignLast         document.Align // text-align-last
	indent            int            // text-indent
	// lineHeight is in percent of the font size, 0 for normal. A length is inherited as a
	// factor, not as a length as CSS has it, so that a larger heading keeps its lines apart.
	lineHeight int
	whiteSpace string
	listStyle  string // list-style-type
	hyphenate  bool
	// underlineUnder is text-underline-position: under.
	underlineUnder bool
	emphasis       document.Emphasis // text-emphasis-style and text-emphasis-position
	direction      document.Direction
	wordBreak      document.WordBreak
	lineBreak      document.LineBreak
}

// style is an element's computed style: the CSS properties the app follows.
type style struct {
	inherited
	display string // "block", "list-item", "inline", "none", or a table's part (displayOf)
	// The margins, with the paddings added to them. An element with a border box keeps them
	// apart, as its border is drawn between them.
	marginTop, marginRight, marginBottom, marginLeft int
	borderTop, borderBottom                          bool
	// raise is how much vertical-align raises the text, and script is set for super and sub,
	// which are drawn Small.
	raise  int
	script bool
	// verticalAlign is vertical-align's keyword, which a table's cells follow.
	verticalAlign string
	// pageBreakBefore and pageBreakAfter start a new page; keepWithNext is break-after: avoid.
	pageBreakBefore, pageBreakAfter, keepWithNext bool
	// The size of an image, 0 for auto.
	width, height, maxWidth int
	// unicodeBidi is unicode-bidi. autoDirection is set by dir="auto" (or a <bdi> without dir):
	// the element's direction is the direction of its text.
	unicodeBidi   unicodeBidi
	autoDirection bool
	// float and clear are as declared: inline-start and inline-end depend on the direction.
	float, clear string
}

// unicodeBidi is a value of unicode-bidi.
type unicodeBidi uint8

// The values of unicode-bidi: embed is taken as isolate, and isolate-override as
// bidi-override.
const (
	bidiNormal unicodeBidi = iota
	bidiIsolate
	bidiOverride
	bidiPlaintext
)

// unicodeBidis are the values of unicode-bidi other than normal.
var unicodeBidis = map[string]unicodeBidi{
	"embed": bidiIsolate, "isolate": bidiIsolate, "bidi-override": bidiOverride, "isolate-override": bidiOverride,
	"plaintext": bidiPlaintext,
}

// rootStyle is the style of the document, above the root element.
var rootStyle = style{fontSize: 100, whiteSpace: "normal", listStyle: "disc", display: "block"}

// computeStyle returns the style of an element from its declared values (from
// css.Cascade.Compute) and its parent's style. familyOf resolves a font-family.
//
// The font size is computed first, as the other lengths are relative to it. Font sizes are kept
// between half and three times the reader's, which reads well, unless fixedPage: on a
// pre-paginated page, sizes are those the page was designed with.
func computeStyle(parent *style, declared map[string]string, familyOf func(string) string, fixedPage bool) style {
	computed := style{inherited: parent.inherited, display: "inline"}
	if value, ok := declared["font-size"]; ok {
		computed.fontSize = fontSizeOf(strings.ToLower(value), parent.fontSize)
		if !fixedPage {
			computed.fontSize = min(max(computed.fontSize, 50), 300)
		}
	}
	for property, value := range declared {
		if property[0] == '-' {
			if property = unprefixed(property, declared); property == "" {
				continue
			}
		}
		keyword := strings.ToLower(value)
		if keyword == "inherit" || keyword == "initial" || keyword == "unset" {
			computed.inheritOrReset(property, keyword, parent)
			continue
		}
		// Each property is one of the font, the text or the box: the two other steps ignore it.
		computed.setFontProperty(property, value, keyword, familyOf)
		computed.setTextProperty(property, value, keyword)
		computed.setBoxProperty(property, keyword, parent)
	}
	computed.borderTop = isVisibleBorder(declared["border-top-style"], declared["border-top-width"])
	computed.borderBottom = isVisibleBorder(declared["border-bottom-style"], declared["border-bottom-width"])
	return computed
}

// inheritOrReset follows a property set to inherit, initial or unset. A style starts with its
// parent's inherited properties and the initial values of the others, which is what these
// keywords ask for; only vertical-align can be inherited, as a table's cells do.
func (computed *style) inheritOrReset(property, keyword string, parent *style) {
	if property == "vertical-align" && keyword == "inherit" {
		computed.verticalAlign = parent.verticalAlign
	}
}

// setFontProperty sets a property of the font (but its size). keyword is value in lower case.
func (computed *style) setFontProperty(property, value, keyword string, familyOf func(string) string) {
	switch property {
	case "font-weight":
		computed.bold = isBold(keyword)
	case "font-style":
		computed.italic = isItalic(keyword)
	case "font-variant", "font-variant-caps":
		computed.smallCaps = strings.Contains(keyword, "small-caps")
	case "font-family":
		computed.family = familyOf(value)
	}
}

// setTextProperty sets a property of how text is drawn, aligned, broken into lines and
// ordered. keyword is value in lower
// case.
func (computed *style) setTextProperty(property, value, keyword string) {
	fontSize := computed.fontSize
	switch property {
	case "text-decoration-line":
		computed.underline = computed.underline || strings.Contains(keyword, "underline")
		computed.strike = computed.strike || strings.Contains(keyword, "line-through")
	case "text-transform":
		computed.transform = keyword
	case "letter-spacing":
		computed.letterSpacing, _ = lengthOf(keyword, fontSize, fontSize)
	case "text-align":
		computed.align = alignOf(keyword, computed.align)
	case "text-align-last":
		computed.alignLast = alignLastOf(keyword, computed.alignLast)
	case "text-underline-position":
		computed.underlineUnder = strings.Contains(keyword, "under")
	case "text-emphasis", "text-emphasis-style":
		if mark, ok := emphasisMarkOf(value); ok || property == "text-emphasis" {
			computed.emphasis.Mark = mark
		}
	case "text-emphasis-position":
		computed.emphasis.Under = strings.Contains(keyword, "under")
	case "direction":
		computed.direction = directions[keyword]
	case "word-break":
		computed.wordBreak = wordBreaks[keyword]
	case "line-break":
		computed.lineBreak = lineBreaks[keyword]
	case "unicode-bidi":
		computed.unicodeBidi = unicodeBidis[keyword]
	case "text-indent":
		computed.indent, _ = lengthOf(keyword, fontSize, textWidth)
	case "line-height":
		computed.lineHeight = lineHeightOf(keyword, fontSize)
	case "white-space":
		computed.whiteSpace = keyword
	case "list-style-type":
		computed.listStyle = keyword
	case "hyphens", "-moz-hyphens", "-ms-hyphens", "adobe-hyphenate":
		computed.hyphenate = keyword == "auto"
	}
}

// setBoxProperty sets a property of the element's box: its display, margins and paddings,
// size, vertical alignment, page breaks and floats.
func (computed *style) setBoxProperty(property, keyword string, parent *style) {
	fontSize := computed.fontSize
	switch property {
	case "display":
		computed.display = displayOf(keyword)
	case "margin-top", "padding-top":
		computed.marginTop += positive(lengthOf(keyword, fontSize, textWidth))
	case "margin-bottom", "padding-bottom":
		computed.marginBottom += positive(lengthOf(keyword, fontSize, textWidth))
	case "margin-left", "padding-left":
		length, _ := lengthOf(keyword, fontSize, textWidth)
		computed.marginLeft += length
	case "margin-right", "padding-right":
		length, _ := lengthOf(keyword, fontSize, textWidth)
		computed.marginRight += length
	case "vertical-align":
		computed.raise, computed.script = raiseOf(keyword, parent.fontSize, fontSize)
		computed.verticalAlign = keyword
	case "page-break-before", "break-before":
		computed.pageBreakBefore = pageBreaks[keyword]
	case "page-break-after", "break-after":
		computed.pageBreakAfter = pageBreaks[keyword]
		computed.keepWithNext = keyword == "avoid" || keyword == "avoid-page"
	case "width":
		computed.width = positive(lengthOf(keyword, fontSize, textWidth))
	case "height":
		computed.height = positive(lengthOf(keyword, fontSize, 0)) // a percentage of an auto height
	case "float":
		computed.float = keyword
	case "clear":
		computed.clear = keyword
	case "max-width":
		computed.maxWidth = positive(lengthOf(keyword, fontSize, textWidth))
	}
}

// unprefixed returns property without its -epub- or -webkit- prefix. EPUB 3 books write some
// properties with those prefixes, often next to the bare property. It returns "" to skip the
// prefixed declaration when the bare property is declared too (the bare one wins), or when a
// -webkit- property is also declared with -epub- (the same property, followed once).
func unprefixed(property string, declared map[string]string) string {
	bare, ok := strings.CutPrefix(property, "-epub-")
	if !ok {
		if bare, ok = strings.CutPrefix(property, "-webkit-"); !ok {
			return property
		}
		if _, epub := declared["-epub-"+bare]; epub {
			return ""
		}
	}
	if _, given := declared[bare]; given {
		return ""
	}
	return bare
}

// emphasisMarks are the marks of text-emphasis-style's shapes, filled then open.
var emphasisMarks = map[string][2]rune{
	"dot": {'•', '◦'}, "circle": {'●', '○'}, "double-circle": {'◉', '◎'}, "triangle": {'▲', '△'},
	"sesame": {'﹅', '﹆'},
}

// emphasisMarkOf returns the mark of a text-emphasis-style value (or of text-emphasis, whose
// color is ignored): 0 for none. It returns false if value gives no style.
func emphasisMarkOf(value string) (rune, bool) {
	value = strings.TrimSpace(value)
	if value != "" && (value[0] == '"' || value[0] == '\'') {
		mark, _ := utf8.DecodeRuneInString(strings.Trim(value, value[:1]))
		if mark == utf8.RuneError {
			return 0, true
		}
		return mark, true
	}
	shape, open, given := "", false, false
	for word := range strings.FieldsSeq(strings.ToLower(value)) {
		switch _, isShape := emphasisMarks[word]; {
		case word == "none":
			return 0, true
		case word == "filled" || word == "open":
			open, given = word == "open", true
		case isShape:
			shape, given = word, true
		}
	}
	if !given {
		return 0, false
	}
	if shape == "" {
		shape = "circle" // in horizontal text
	}
	if open {
		return emphasisMarks[shape][1], true
	}
	return emphasisMarks[shape][0], true
}

// directions are the values of direction.
var directions = map[string]document.Direction{"ltr": document.LeftToRight, "rtl": document.RightToLeft}

// wordBreaks are the values of word-break. break-word is taken as normal: a word too long for
// a line is cut anyway.
var wordBreaks = map[string]document.WordBreak{"break-all": document.BreakAll, "keep-all": document.KeepAll}

// lineBreaks are the values of line-break.
var lineBreaks = map[string]document.LineBreak{
	"loose": document.LineBreakLoose, "normal": document.LineBreakNormal, "strict": document.LineBreakStrict,
	"anywhere": document.LineBreakAnywhere,
}

// lengthOf returns a CSS length in hundredths of an em of the reader's font size. An em is
// fontSize (in percent of the reader's font size), a percentage is of hundred, and a pixel is
// 1/pixelsPerEm of an em. It returns false if value is no length (auto, normal…).
func lengthOf(value string, fontSize, hundred int) (int, bool) {
	length, ok := css.ParseLength(value)
	if !ok {
		return 0, false
	}
	var hundredths float64
	switch length.Unit {
	case "em":
		hundredths = length.Value * float64(fontSize)
	case "rem":
		hundredths = length.Value * 100
	case "ex", "ch":
		hundredths = length.Value * float64(fontSize) / 2
	case "%":
		hundredths = length.Value * float64(hundred) / 100
	case "vw", "vmin":
		hundredths = length.Value * textWidth / 100
	case "vh", "vmax":
		hundredths = length.Value * pageHeight * 100 / pixelsPerEm / 100
	default:
		pixels, ok := length.Pixels()
		if !ok {
			return 0, false // a number other than 0
		}
		hundredths = pixels * 100 / pixelsPerEm
	}
	return int(math.Round(hundredths)), true
}

// positive returns length if it is positive, else 0. It takes lengthOf's results, for the
// properties that cannot be negative or that the app keeps positive.
func positive(length int, _ bool) int { return max(length, 0) }

// fontSizeKeywords are the absolute sizes of font-size, in percent of the reader's font size.
var fontSizeKeywords = map[string]int{
	"xx-small": 60, "x-small": 75, "small": 89, "medium": 100, "large": 120, "x-large": 150,
	"xx-large": 200, "xxx-large": 300,
}

// fontSizeOf returns the font size of a font-size value given the parent's, both in percent of
// the reader's font size.
func fontSizeOf(value string, parentSize int) int {
	size, ok := fontSizeKeywords[value]
	switch {
	case ok:
	case value == "smaller":
		size = parentSize * 5 / 6
	case value == "larger":
		size = parentSize * 6 / 5
	default:
		if size, ok = lengthOf(value, parentSize, parentSize); !ok || size <= 0 {
			return parentSize
		}
	}
	return size
}

// lineHeightOf returns the line height of a line-height value in percent of fontSize, 0 for
// normal or a value the app doesn't follow.
func lineHeightOf(value string, fontSize int) int {
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return max(int(math.Round(number*100)), 0)
	}
	length, _ := lengthOf(value, fontSize, fontSize)
	return max(length*100/fontSize, 0)
}

// raiseOf returns how much vertical-align raises an element's baseline, given its parent's
// font size and its own, and whether it is a superscript or a subscript.
func raiseOf(value string, parentSize, fontSize int) (raise int, script bool) {
	switch value {
	case "super":
		return parentSize / 3, true
	case "sub":
		return -parentSize / 5, true
	}
	length, _ := lengthOf(value, fontSize, fontSize*120/100) // a percentage of the line height
	return length, false
}

// pageBreaks are the values of page-break-before/after and break-before/after that start a
// new page.
var pageBreaks = map[string]bool{"always": true, "page": true, "left": true, "right": true, "recto": true, "verso": true}

// displayOf returns what a display value is to the app: "block" (a table's caption, flex and
// grid containers too), "list-item", "none", a table's part (tableParts), or "inline"
// (inline-block too).
func displayOf(value string) string {
	outer, _, _ := strings.Cut(value, " ")
	if tableParts[outer] {
		return outer
	}
	switch outer {
	case "none", "list-item":
		return outer
	case "block", "table-caption", "flex", "grid", "flow-root":
		return "block"
	}
	return "inline"
}

// isBold reports whether a font-weight is bold: bold, bolder, or at least 600. A range (of an
// @font-face) is read by its first weight.
func isBold(weight string) bool {
	first, _, _ := strings.Cut(strings.TrimSpace(weight), " ")
	number, err := strconv.ParseFloat(first, 64)
	return first == "bold" || first == "bolder" || err == nil && number >= 600
}

// isItalic reports whether a font-style is italic or oblique.
func isItalic(fontStyle string) bool {
	return strings.HasPrefix(fontStyle, "italic") || strings.HasPrefix(fontStyle, "oblique")
}

// alignOf returns the alignment of a text-align value, current if unknown.
func alignOf(value string, current document.Align) document.Align {
	switch value {
	case "center":
		return document.AlignCenter
	case "right":
		return document.AlignRight
	case "left":
		return document.AlignLeft
	case "start":
		return document.AlignStart
	case "end":
		return document.AlignEnd
	case "justify":
		return document.AlignDefault
	}
	return current
}

// isolateOf returns how an inline element sets its text apart for the bidirectional
// algorithm, document.NoIsolate if it does not.
func isolateOf(computed *style) document.Isolate {
	rightToLeft := computed.direction == document.RightToLeft
	switch computed.unicodeBidi {
	case bidiIsolate:
		switch {
		case computed.autoDirection:
			return document.IsolateAuto
		case rightToLeft:
			return document.IsolateRightToLeft
		}
		return document.IsolateLeftToRight
	case bidiOverride:
		if rightToLeft {
			return document.OverrideRightToLeft
		}
		return document.OverrideLeftToRight
	case bidiPlaintext:
		return document.IsolateAuto
	}
	return document.NoIsolate
}

// blockDirectionOf returns the direction of a block element's text.
func blockDirectionOf(computed *style) document.Direction {
	if computed.autoDirection || computed.unicodeBidi == bidiPlaintext {
		return document.DirectionAuto
	}
	return computed.direction
}

// alignLastOf returns the alignment of a text-align-last value, current if unknown.
func alignLastOf(value string, current document.Align) document.Align {
	switch value {
	case "auto":
		return document.AlignDefault
	case "justify":
		return document.AlignJustify
	}
	return alignOf(value, current)
}

// isVisibleBorder reports whether a border of a style and width is drawn.
func isVisibleBorder(borderStyle, width string) bool {
	borderStyle = strings.ToLower(borderStyle)
	if borderStyle == "" || borderStyle == "none" || borderStyle == "hidden" {
		return false
	}
	length, ok := lengthOf(width, 100, 0)
	return !ok || length > 0 // thin, medium and thick are no lengths
}
