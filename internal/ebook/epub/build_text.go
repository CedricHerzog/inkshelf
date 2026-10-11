package epub

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/mathml"
)

// emptyLineMargin is the space that a <br> on an empty line leaves, in hundredths of an em.
const emptyLineMargin = 100

// runOf returns what the runs of an element's text are, but their text: its style, its size
// and baseline relative to its block, its family, letter spacing and link.
func (parser *builder) runOf(opened *element) document.Run {
	computed := &opened.style
	blockSize := parser.blockElement().block.fontSize
	run := document.Run{
		Family: computed.family, Link: opened.parent.run.Link,
		FontScale:     percentOf(computed.fontSize, blockSize),
		LetterSpacing: computed.letterSpacing * 100 / computed.fontSize,
		Emphasis:      computed.emphasis,
	}
	if opened.block == nil {
		run.Shift = opened.parent.run.Shift + computed.raise*100/blockSize
		run.Isolate = opened.parent.run.Isolate
		if isolate := isolateOf(computed); isolate != document.NoIsolate {
			run.Isolate = isolate
		}
	}
	run.Language = opened.language
	run.Gray = parser.runGray(opened)
	for _, bit := range []struct {
		style document.Style
		set   bool
	}{
		{document.Bold, computed.bold}, {document.Italic, computed.italic}, {document.SmallCaps, computed.smallCaps},
		{document.Underline, computed.underline}, {document.Strike, computed.strike},
		{document.UnderlineUnder, computed.underline && computed.underlineUnder},
		// A superscript or a subscript is smaller, unless its CSS makes it so already.
		{document.Small, computed.script && computed.fontSize >= opened.parent.style.fontSize},
	} {
		if bit.set {
			run.Style |= bit.style
		}
	}
	if href := opened.attributes["href"]; opened.name == "a" && href != "" {
		run.Link = linkTarget(parser.chapterPath, href)
	}
	return run
}

// percentOf returns size in percent of base, 0 for 100%.
func percentOf(size, base int) int {
	if size == base {
		return 0
	}
	return size * 100 / base
}

// text adds character data to the block being built.
func (parser *builder) text(data string) {
	current := parser.top()
	if current.name == "style" {
		parser.styleText.WriteString(data)
		return
	}
	if current.hidden {
		return
	}
	switch whiteSpace := current.style.whiteSpace; whiteSpace {
	case "pre", "pre-wrap", "pre-line", "break-spaces":
		for index, line := range strings.Split(data, "\n") {
			if index > 0 {
				parser.lineBreak()
			}
			parser.appendText(line, current, whiteSpace != "pre-line")
		}
	default:
		parser.appendText(data, current, false)
	}
}

// appendText adds text in the current element's style to the block being built, its
// whitespace collapsed unless keepSpaces, its case transformed by its text-transform.
func (parser *builder) appendText(text string, current *element, keepSpaces bool) {
	var added strings.Builder
	for _, character := range text {
		if !keepSpaces && (character == ' ' || character == '\t' || character == '\n' || character == '\r' || character == '\f') {
			parser.spaceNeeded, parser.spaceRun = true, current.run
			continue
		}
		if parser.spaceNeeded && (parser.pendingRunes > 0 || added.Len() > 0) {
			// The space is in the style of the element whose whitespace it was, if that shows:
			// outside an isolate, a link or an underline that the whitespace is outside of.
			if !spaceStyleShows(parser.spaceRun, current.run) {
				added.WriteRune(' ')
			} else {
				space := parser.spaceRun
				space.Text = " "
				parser.appendRun(space)
			}
			parser.lastRune = ' '
		}
		parser.spaceNeeded = false
		if character == symbolBullet {
			character = '•'
		}
		added.WriteRune(transform(character, current.style.transform, parser.lastRune))
		parser.lastRune = character
	}
	if added.Len() > 0 {
		run := current.run
		run.Text = added.String()
		parser.appendRun(run)
	}
}

// symbolBullet is the bullet of Word's Symbol font, a private-use character that books
// converted from Word keep: shown as •, one rune for one, as no font of the reader has it.
const symbolBullet = '\uF0B7'

// transform returns character after a text-transform, one rune for one so that offsets don't
// change (ß stays ß in uppercase). capitalize changes a word's first letter: one whose
// previous rune is no letter, digit or apostrophe. full-width (EPUB's -epub-fullwidth) draws
// ASCII as its full-width forms, as wide as ideographs; spaces stay, to break lines at.
func transform(character rune, textTransform string, previous rune) rune {
	switch textTransform {
	case "uppercase":
		return unicode.ToUpper(character)
	case "lowercase":
		return unicode.ToLower(character)
	case "capitalize":
		if !unicode.IsLetter(previous) && !unicode.IsDigit(previous) && !unicode.IsMark(previous) && previous != '\'' && previous != '’' {
			return unicode.ToTitle(character)
		}
	case "full-width", "-epub-fullwidth":
		if '!' <= character && character <= '~' {
			return character - '!' + '！'
		}
	}
	return character
}

// appendRun adds run (text or an inline image) to the block being built, after the list
// item's marker if it is its first.
func (parser *builder) appendRun(run document.Run) {
	parser.checkBackLink(run)
	if parser.pendingRunes == 0 && parser.marker != "" {
		marker := run
		marker.Text, marker.Image, marker.ImageHeight = parser.marker, "", 0
		parser.marker = ""
		parser.appendRun(marker)
	}
	if last := len(parser.runs) - 1; last >= 0 && sameStyle(parser.runs[last], run) {
		parser.runs[last].Text += run.Text
	} else {
		parser.runs = append(parser.runs, run)
	}
	if run.Image != "" {
		parser.pendingRunes++
	} else {
		parser.pendingRunes += utf8.RuneCountInString(run.Text)
	}
}

// spaceStyleShows reports whether a space in the style of a run differs from one in that of
// another where it shows: its direction, its link, its underline or strike-through. A space
// in another font or size is not worth a run of its own.
func spaceStyleShows(space, other document.Run) bool {
	const strokes = document.Underline | document.Strike | document.UnderlineUnder
	return space.Isolate != other.Isolate || space.Link != other.Link || space.Style&strokes != other.Style&strokes
}

// sameStyle reports whether two runs of text are drawn alike, and can be one.
func sameStyle(first, second document.Run) bool {
	first.Text, second.Text = "", ""
	return first == second && first.Image == ""
}

// flush adds the text built so far as a block, or as an image block if it is a lone image.
func (parser *builder) flush() {
	if parser.pendingRunes == 0 {
		return
	}
	runs := parser.runs
	parser.runs, parser.pendingRunes, parser.spaceNeeded, parser.lastRune = nil, 0, false, 0
	if len(runs) == 1 && runs[0].Image != "" && !mathml.Is(runs[0].Image) {
		parser.addImageBlock(runs[0].Image, 0)
		return
	}
	blockElement := parser.blockElement()
	props := blockElement.block
	block := document.Block{
		Kind: props.kind, Level: props.level, Align: props.align, AlignLast: props.alignLast, FontScale: percentOf(props.fontSize, 100),
		Indent: props.indent, Left: props.left, Right: props.right, LineHeight: props.lineHeight,
		MarginTop: parser.pending.marginTop, MarginBottom: props.marginBottom,
		KeepWithNext: props.kind == document.Heading, PageBreak: parser.pending.pageBreak,
		BorderTop: parser.pending.borderTop, Hyphenate: props.hyphenate,
		Footnote: noteIDOf(blockElement), Runs: runs,
		Direction: props.direction, WordBreak: props.wordBreak, LineBreak: props.lineBreak,
	}
	if parser.continued {
		block.MarginTop, block.Indent = 0, 0
	}
	parser.floatBlock(&block, blockElement)
	parser.addBlock(block)
	parser.pending, parser.continued = pending{}, false
}

// lineBreak ends the block being built at a <br>: the next one continues it, without margin
// nor indent. A <br> on an empty line leaves a line's space.
func (parser *builder) lineBreak() {
	if parser.pendingRunes == 0 {
		parser.pending.marginTop += emptyLineMargin
		return
	}
	parser.flush()
	if last := len(parser.doc.Blocks) - 1; last >= 0 {
		parser.doc.Blocks[last].MarginBottom = 0
	}
	parser.continued = true
}
