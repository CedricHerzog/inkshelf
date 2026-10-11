package epub

import (
	"cmp"
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/ebook/epub/container"
	"github.com/CedricHerzog/inkshelf/internal/ebook/fixedlayout"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// ruleMargin is the space above and below an <hr>, in hundredths of an em.
const ruleMargin = 50

// documentName is the name of the element at the bottom of the stack, above the root element:
// it stands for the document itself, which selectors never match.
const documentName = "#document"

// blockProps are the properties shared by the blocks that a block element's text makes.
type blockProps struct {
	kind                          document.BlockKind
	level                         int
	align, alignLast              document.Align
	fontSize, indent, left, right int
	lineHeight, marginBottom      int
	hyphenate                     bool
	direction                     document.Direction
	wordBreak                     document.WordBreak
	lineBreak                     document.LineBreak
}

// element is an element of the chapter while it is open, or once closed while it is the
// previous sibling of an open one. It holds what selectors match against, its computed style,
// and what its text becomes.
type element struct {
	name string
	// attributes are by lowercase local name; an epub:type attribute is "epub:type".
	attributes map[string]string
	classes    []string
	parent     *element
	previous   *element // the previous sibling
	lastChild  *element // the last child closed so far, while the element is open
	style      style
	run        document.Run // the style of the runs of its text, without their text
	hidden     bool
	// block holds the properties of a block element's text; it is nil for an inline element.
	block   *blockProps
	counter int // the list items opened in it so far
	// note is the footnote the element is in, nil if none. noteID is a note's id, or the first
	// id found in it. backLinked is set on a note found by its link back to its call, and
	// flatNote on such a note whose text is its own rather than in child blocks: the blocks
	// after it in its parent continue it.
	note                 *element
	noteID               string
	backLinked, flatNote bool
	// positioned is set while a positioned element of a pre-paginated chapter is open.
	positioned *positionedElement
	// language is the element's language (its lang, else its parent's), as a Run.Language.
	language uint8
	// color is the element's CSS color, and background the gray its text is drawn on. boxDepth
	// is how many border boxes the element is in, and borderBox its own, nil if it has none.
	color      css.Color
	background uint8
	boxDepth   int
	borderBox  *borderBox
	// floating is the float group the element is in, nil if none.
	floating *floatGroup
}

// An element is a css.Element, which selectors are matched against.

func (opened *element) Name() string      { return opened.name }
func (opened *element) ID() string        { return opened.attributes["id"] }
func (opened *element) Classes() []string { return opened.classes }

func (opened *element) Attribute(name string) (string, bool) {
	value, ok := opened.attributes[name]
	return value, ok
}

func (opened *element) Parent() css.Element {
	if opened.parent == nil || opened.parent.name == documentName {
		return nil
	}
	return opened.parent
}

func (opened *element) PreviousSibling() css.Element {
	if opened.previous == nil {
		return nil
	}
	return opened.previous
}

// pending is what the elements opened since the last block ask of the next block.
type pending struct {
	marginTop            int
	pageBreak, borderTop bool
	clear                document.Clear
}

// builder turns a chapter's XHTML into a document. It reads the markup token by token, keeps
// the open elements on a stack, computes each element's style from the chapter's cascade, and
// gathers text into runs until a block ends.
type builder struct {
	book        *Book
	chapterPath string
	cascade     *css.Cascade
	sheetsAdded map[string]bool // by path
	styleText   strings.Builder // the text of the <style> being read
	doc         document.Document
	stack       []*element

	runs         []document.Run // the text of the block being built
	pendingRunes int
	spaceNeeded  bool         // whitespace came last: a space goes before the next character
	spaceRun     document.Run // the run of the element that the whitespace was in
	lastRune     rune         // the block's last rune, 0 at its start, for text-transform: capitalize
	marker       string
	pending      pending
	// continued is set after a <br>: the next block continues the last one's paragraph.
	continued bool
	// forIndex is set when the document is only measured for BuildIndex: its lengths and anchors
	// don't depend on fonts or notes, which are left out.
	forIndex bool
	// page receives the regions of a pre-paginated chapter; it is nil for a reflowable one.
	page *fixedlayout.Page
	// candidate is the element at a call's target until text follows it. lastNote is the last
	// note found by a link back, once closed: the next blocks of its parent continue it.
	candidate *noteCandidate
	lastNote  *element
	// floatGroups is the number of float groups made so far.
	floatGroups int
	// tables are the open table elements, the innermost last; startedTables are those that
	// have their first block.
	tables        []*tableBuilder
	startedTables map[*document.Table]bool
}

// parse reads the XHTML's tokens into the document.
func (parser *builder) parse(data []byte) error {
	decoder := container.NewDecoder(data)
	for {
		offset := decoder.InputOffset()
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Real books have broken markup: what came before the error is kept.
			if len(parser.doc.Blocks) == 0 && parser.pendingRunes == 0 {
				return err
			}
			break
		}
		switch token := token.(type) {
		case xml.StartElement:
			parser.start(token)
			switch parser.top().name {
			case "svg":
				parser.inlineSVG(decoder, data[offset:])
			case "math":
				parser.inlineMath(decoder, data[offset:])
			}
		case xml.EndElement:
			parser.end(strings.ToLower(token.Name.Local))
		case xml.CharData:
			parser.text(string(token))
		}
	}
	parser.flush()
	return nil
}

// top returns the innermost open element.
func (parser *builder) top() *element { return parser.stack[len(parser.stack)-1] }

// blockElement returns the innermost open block element.
func (parser *builder) blockElement() *element {
	for index := len(parser.stack) - 1; ; index-- {
		if parser.stack[index].block != nil {
			return parser.stack[index]
		}
	}
}

// start opens an element: it pushes it on the stack, computes its style, and starts what it
// makes of the document (a block, a table's part, a float, a region of a page).
func (parser *builder) start(token xml.StartElement) {
	parent := parser.top()
	opened := childOf(parent, token)
	parser.stack = append(parser.stack, opened)
	parser.readHeadElement(opened)
	if opened.hidden {
		return
	}
	declared := tableHints(opened, parser.cascade.Compute(opened, ""))
	opened.style = computeStyle(&parent.style, declared, parser.familyOf, parser.page != nil)
	if opened.style.display == "none" {
		opened.hidden = true
		return
	}
	opened.color = colorOf(opened, declared, parent.color)
	if dir, ok := opened.attributes["dir"]; ok && strings.EqualFold(dir, "auto") || !ok && opened.name == "bdi" {
		opened.style.autoDirection = true
	}
	if opened.name == "br" || opened.name == "hr" {
		// A line break and a rule whatever their display, as in browsers: Calibre makes them
		// blocks, which would leave an empty line, or draw borders around the rule.
		opened.style.display = "inline"
	}
	parser.startFloat(opened)
	parser.record(opened)
	if language, ok := opened.attributes["lang"]; ok { // or xml:lang
		opened.language = parser.doc.LanguageIndex(language)
	}
	switch display := opened.style.display; {
	case display == "block" || display == "list-item":
		parser.enterBorderBox(opened, declared)
		parser.openBlock(opened)
		parser.continueNote(opened)
	case tableParts[display]:
		parser.openTablePart(opened, declared)
	}
	if parser.page != nil {
		parser.placeOnPage(opened, declared)
	}
	opened.run = parser.runOf(opened)
	parser.addContent(opened)
}

// childOf returns the element that token opens in parent, with what it inherits from it.
func childOf(parent *element, token xml.StartElement) *element {
	opened := &element{
		name: strings.ToLower(token.Name.Local), attributes: attributesOf(token.Attr),
		parent: parent, previous: parent.lastChild,
		style: style{inherited: parent.style.inherited}, run: parent.run, hidden: parent.hidden, note: parent.note,
		language: parent.language, color: parent.color, background: parent.background, boxDepth: parent.boxDepth,
	}
	opened.classes = strings.Fields(opened.attributes["class"])
	return opened
}

// readHeadElement follows the elements of a chapter's head that matter even though they are
// hidden: a linked stylesheet, and the viewport of a pre-paginated page.
func (parser *builder) readHeadElement(opened *element) {
	if rel := strings.Fields(strings.ToLower(opened.attributes["rel"])); opened.name == "link" && slices.Contains(rel, "stylesheet") && !slices.Contains(rel, "alternate") {
		parser.linkStylesheet(opened.attributes["href"], opened.attributes["media"])
	}
	if parser.page != nil && opened.name == "meta" {
		parser.readViewport(opened)
	}
}

// enterBorderBox gives a block element its border box, if it has one, and sets the background
// that its text is drawn on.
func (parser *builder) enterBorderBox(opened *element, declared map[string]string) {
	box := parser.borderBoxOf(opened, declared)
	if box == nil {
		return
	}
	opened.borderBox = box
	opened.boxDepth++
	if box.drawn.Filled {
		opened.background = box.drawn.Background
	}
}

// record notes what an element's attributes say about the document: its language, an anchor,
// a footnote, the first number of a list.
func (parser *builder) record(opened *element) {
	name, attributes := opened.name, opened.attributes
	if (name == "html" || name == "body") && parser.doc.Language == "" {
		parser.doc.Language = attributes["lang"] // or xml:lang
	}
	id := attributes["id"]
	if name == "a" {
		id = cmp.Or(id, attributes["name"])
	}
	parser.anchor(id)
	switch {
	case slices.ContainsFunc(strings.Fields(attributes["epub:type"]), isNoteType):
		opened.note, opened.noteID = opened, id
	case opened.note != nil && opened.note.noteID == "":
		opened.note.noteID = id
	}
	parser.considerNote(opened, id)
	if name == "ol" && attributes["start"] != "" {
		start, _ := strconv.Atoi(attributes["start"])
		opened.counter = start - 1
	}
}

// addContent adds what an element stands for besides its text: a line break, an image, a rule,
// an opening quotation mark. When an element is shown as an image, its fallback content is
// hidden.
func (parser *builder) addContent(opened *element) {
	attributes := opened.attributes
	switch name := opened.name; {
	case name == "br":
		parser.lineBreak()
		parser.clearAfter(opened)
	case name == "img" || name == "image":
		parser.addImage(cmp.Or(attributes["src"], attributes["href"]), opened)
	case name == "object":
		// An <object> shows its image if the book has one to show, else its content.
		if _, ok := parser.book.SupportedImage(resolve(parser.chapterPath, attributes["data"])); ok {
			parser.addImage(attributes["data"], opened)
			opened.hidden = true
		}
	case name == "video" && attributes["poster"] != "":
		parser.addImage(attributes["poster"], opened)
		opened.hidden = true
	case name == "hr":
		parser.flush()
		parser.addBlock(document.Block{Kind: document.Rule, MarginTop: ruleMargin, MarginBottom: ruleMargin})
	case name == "q":
		parser.appendText("“", opened, false) // and ” when it closes
	}
}

// attributesOf returns the attributes of an element by lowercase local name. A type attribute
// in a namespace is epub:type, which is named "epub:type" to keep it apart from HTML's type.
func attributesOf(attributes []xml.Attr) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	byName := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		name := strings.ToLower(attribute.Name.Local)
		if name == "type" && attribute.Name.Space != "" {
			name = "epub:type"
		}
		byName[name] = attribute.Value
	}
	return byName
}

// openBlock starts the block of a block element. The text before it ends there, and the
// margins and page breaks it starts with go on its first block.
func (parser *builder) openBlock(opened *element) {
	parser.flush()
	computed := &opened.style
	outer := parser.blockElement().block
	props := blockProps{
		kind: document.Paragraph, align: computed.align, alignLast: computed.alignLast, fontSize: computed.fontSize,
		left:       max(outer.left+min(computed.marginLeft, maxMargin), 0),
		right:      max(outer.right+min(computed.marginRight, maxMargin), 0),
		lineHeight: computed.lineHeight, marginBottom: computed.marginBottom, hyphenate: computed.hyphenate,
		direction: blockDirectionOf(computed), wordBreak: computed.wordBreak, lineBreak: computed.lineBreak,
	}
	// A negative indent hangs the first line into the margin, but no further.
	props.indent = min(max(computed.indent, -props.left), maxMargin)
	parser.enterFloat(opened, &props, outer)
	if len(opened.name) == 2 && opened.name[0] == 'h' && opened.name[1] >= '1' && opened.name[1] <= '6' {
		props.kind, props.level = document.Heading, int(opened.name[1]-'0')
	}
	opened.block = &props
	if opened.borderBox != nil {
		parser.openBorderBox(opened, &props, outer)
	} else {
		parser.pending.marginTop = max(parser.pending.marginTop, computed.marginTop)
		parser.pending.pageBreak = parser.pending.pageBreak || computed.pageBreakBefore
	}
	parser.pending.borderTop = parser.pending.borderTop || computed.borderTop
	if computed.display == "list-item" {
		parser.marker = listMarker(opened)
	}
}

// end closes the innermost open element named name, and the elements left open inside it.
func (parser *builder) end(name string) {
	for index := len(parser.stack) - 1; index > 0; index-- {
		if parser.stack[index].name != name {
			continue
		}
		for len(parser.stack) > index {
			parser.close(parser.top())
			parser.stack = parser.stack[:len(parser.stack)-1]
		}
		return
	}
}

// close ends the innermost open element: a <style> adds its stylesheet, a <q> its closing
// quotation mark, and a block element gives its last block its bottom margin, border and page
// break. The element becomes its parent's last child.
func (parser *builder) close(closed *element) {
	closed.parent.lastChild, closed.lastChild = closed, nil // its children are no longer matched
	parser.closeNote(closed)
	switch {
	case closed.name == "style":
		if screen.Matches(closed.attributes["media"]) {
			parser.addStylesheet(css.Parse(parser.styleText.String(), screen), parser.chapterPath)
		}
		parser.styleText.Reset()
	case closed.name == "q" && !closed.hidden:
		parser.appendText("”", closed, false)
	}
	if closed.block == nil {
		parser.closeTablePart(closed) // a row, a row group
		return
	}
	parser.flush()
	parser.closeBorderBox(closed)
	tablePart, last := parser.closeTablePart(closed), len(parser.doc.Blocks)-1
	var table *document.Table // the closed table that the last block ends, if any
	if last >= 0 {
		table = parser.closedTableOf(parser.doc.Blocks[last])
	}
	switch {
	case tablePart || last < 0:
	case table != nil:
		table.MarginBottom = max(table.MarginBottom, closed.block.marginBottom)
	default:
		block := &parser.doc.Blocks[last]
		block.MarginBottom = max(block.MarginBottom, closed.block.marginBottom)
		block.BorderBottom = block.BorderBottom || closed.style.borderBottom
		block.KeepWithNext = block.KeepWithNext || closed.style.keepWithNext
	}
	parser.pending.pageBreak = parser.pending.pageBreak || closed.style.pageBreakAfter
	parser.leaveFloat(closed)
	if closed.positioned != nil {
		parser.closePositioned(closed)
	}
}
