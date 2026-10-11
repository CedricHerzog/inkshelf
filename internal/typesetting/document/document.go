package document

import (
	"slices"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
)

// BlockKind is what a block holds.
type BlockKind int

// The kinds of blocks.
const (
	Paragraph BlockKind = iota
	Heading
	Image // Block.Image, drawn on its own lines
	Rule  // a horizontal line
)

// Align is how a block's lines sit between the margins.
type Align int

// The alignments. AlignDefault justifies when Settings.Justify is set, else aligns at the start.
const (
	AlignDefault Align = iota
	AlignLeft
	AlignCenter
	AlignRight
	// AlignStart and AlignEnd are the sides where the text starts and ends: left and right, the
	// other way round in right-to-left text.
	AlignStart
	AlignEnd
	// AlignJustify justifies whatever Settings.Justify says: text-align-last: justify.
	AlignJustify
)

// Direction is the direction of a block's text.
type Direction uint8

// The directions. DirectionAuto is the direction of the block's first strong character (by
// the Unicode bidirectional algorithm), left to right if none.
const (
	LeftToRight Direction = iota
	RightToLeft
	DirectionAuto
)

// Isolate is how a run's text is set apart for the bidirectional algorithm: CSS unicode-bidi
// with direction, HTML's dir on an inline element, <bdi> and <bdo>. Consecutive runs of the
// same isolate are one.
type Isolate uint8

// The isolates. An override lays out every character in its direction.
const (
	NoIsolate Isolate = iota
	IsolateLeftToRight
	IsolateRightToLeft
	IsolateAuto // in the direction of its first strong character
	OverrideLeftToRight
	OverrideRightToLeft
)

// WordBreak is where the lines of a block may break inside words (CSS word-break).
type WordBreak uint8

// The word breaks.
const (
	WordBreakNormal WordBreak = iota
	BreakAll                  // between any two letters, of any script
	KeepAll                   // not between ideographs: only where Latin text breaks
)

// LineBreak is how strictly the lines of a block break around Chinese and Japanese
// punctuation and small kana (CSS line-break).
type LineBreak uint8

// The line breaks. Auto, loose and normal break before small kana and the prolonged sound
// mark; strict does not; anywhere breaks between any two characters.
const (
	LineBreakAuto LineBreak = iota
	LineBreakLoose
	LineBreakNormal
	LineBreakStrict
	LineBreakAnywhere
)

// Emphasis is the emphasis marks of a run's characters (CSS text-emphasis), drawn above them or
// under them, half their size. Spaces and punctuation get none.
type Emphasis struct {
	Mark  rune // 0 for none
	Under bool
}

// Float is the side of the text that a floated block goes to (CSS float).
type Float uint8

// The floats.
const (
	NoFloat Float = iota
	FloatLeft
	FloatRight
)

// Clear is the sides whose floats a block must start below (CSS clear): a set of bits.
type Clear uint8

// The clears.
const (
	ClearLeft Clear = 1 << iota
	ClearRight
	ClearBoth = ClearLeft | ClearRight
)

// Style is how a run of text is drawn: a set of bits.
type Style uint8

// The style bits.
const (
	Bold      Style = 1 << iota
	Italic          // also oblique
	Small           // smaller: small, sup, sub
	Underline       // a line under the text
	Strike          // a line through the text
	SmallCaps       // lowercase letters drawn as smaller capitals
	// UnderlineUnder draws the underline under the descenders (text-underline-position: under).
	UnderlineUnder
)

// Run is text in one style, or an inline image. Its spaces are already collapsed, as HTML does.
type Run struct {
	Text  string
	Style Style
	// Isolate sets the run's text apart for the bidirectional algorithm.
	Isolate Isolate
	// Language is the run's language, for shaping: an index in Document.Languages from 1, 0
	// for the document's.
	Language uint8
	// Emphasis is the run's emphasis marks.
	Emphasis Emphasis
	// FontScale is the font size, in percent of the block's (0 is 100); Small still shrinks it.
	FontScale Percent
	// Shift raises the baseline, in hundredths of an em of the block's size (negative lowers it).
	Shift Hundredths
	// Family is the font family: "" for the book font, "serif", "sans-serif", "monospace", or the
	// name of a font of Document.Fonts.
	Family string
	// LetterSpacing is added after each character, in hundredths of an em (negative tightens).
	LetterSpacing Hundredths
	// Link is the target of the link the run is in: a path in the book with its fragment
	// ("OEBPS/notes.xhtml#n1"), an absolute URL, or "" if none.
	Link string
	// Gray is the gray the text is drawn in: 0, black, unless the book colors it.
	Gray uint8
	// Image, if set, makes the run an inline image (Text is ""): its path in the book or a data:
	// URI. It counts as one rune. ImageHeight is its height in hundredths of an em, 0 for its own.
	Image       string
	ImageHeight Hundredths
}

// Block is a paragraph, a heading, an image or a rule. Lengths are in hundredths of an em of
// the settings' font size.
type Block struct {
	Kind  BlockKind
	Level int // of a heading, 1 to 6
	Align Align
	// AlignLast is the alignment of the last line, AlignDefault for Align's (but justified
	// text's last line, which aligns at the start).
	AlignLast Align
	// Direction is the direction of the text: where it starts, and how AlignStart and AlignEnd
	// align it.
	Direction Direction
	// WordBreak and LineBreak are where lines may break.
	WordBreak WordBreak
	LineBreak LineBreak
	// Scripts is what the scripts of the block's text need, if scanned: Add scans them once,
	// rather than every layout at every font size.
	Scripts Scripts
	// FontScale is the font size, in percent of the settings' (0 is 100): headings are larger.
	FontScale Percent
	// Indent is the first line's indent, Left the left margin of every line (a quote, a list).
	Indent, Left Hundredths
	// MarginTop and MarginBottom are the space above and below the block.
	MarginTop, MarginBottom Hundredths
	// KeepWithNext keeps the block on the page of the next one (a heading).
	KeepWithNext bool
	// PageBreak starts the block on a new page.
	PageBreak bool
	// Right is the right margin of every line.
	Right Hundredths
	// LineHeight is the height of a line, in percent of the font size (0 is the settings').
	LineHeight Percent
	// Hyphenate breaks words with the hyphenation patterns of Document.Language.
	Hyphenate bool
	// BorderTop and BorderBottom draw a rule across the block above and below it.
	BorderTop, BorderBottom bool
	// Footnote is the id of the footnote the block belongs to, "" if none: the reader may show the
	// note apart.
	Footnote string
	Runs     []Run
	Image    string // the image's path in the book, for an Image block
	// ImageWidth is an Image block's width, in percent of the text's (0 is its own).
	ImageWidth Percent
	// Float takes the block out of the text, to its left or its right, the lines of the next
	// blocks going beside it. The consecutive blocks of one FloatGroup (a number from 1) float
	// together, FloatWidth percent of the text's width (0 for as wide as their content).
	Float      Float
	FloatGroup int
	FloatWidth Percent
	// Clear starts the block below the floats of its sides.
	Clear Clear
	// Start is the offset of the block's first rune in the document; Add sets it.
	Start int
	// Cell is the table cell the block is in, nil if none. The blocks of a table follow each
	// other in the document, cell after cell, row after row.
	Cell *TableCell
}

// Document is the blocks laid out together: a chapter of a book.
type Document struct {
	Blocks []Block
	// Chars is the length of the document, in runes.
	Chars int
	// IDs are the offsets of the document's anchors, by id, for links into it.
	IDs map[string]int
	// Language is the document's language tag, for hyphenation.
	Language string
	// Languages are the language tags of runs that are not in the document's language, which
	// Run.Language indexes from 1.
	Languages []string
	// Fonts are the fonts the document embeds, by lowercase family name and style.
	Fonts map[string]map[fonts.Style]*fonts.File
	// Boxes are the borders and backgrounds of block elements, around their blocks.
	Boxes []BorderBox
}

// Add appends block to the document, at its end, and returns its offset.
func (doc *Document) Add(block Block) int {
	block.Start = doc.Chars
	doc.Chars += block.Length()
	block.Scripts = ScanScripts(block.Runs)
	doc.Blocks = append(doc.Blocks, block)
	return block.Start
}

// Anchor records id at the document's current end, unless it has been recorded already.
func (doc *Document) Anchor(id string) {
	if id == "" {
		return
	}
	if doc.IDs == nil {
		doc.IDs = map[string]int{}
	}
	if _, ok := doc.IDs[id]; !ok {
		doc.IDs[id] = doc.Chars
	}
}

// LanguageIndex returns the Run.Language of tag: 0 for the document's language (or past 255
// languages), else its index in Languages from 1, added if new.
func (doc *Document) LanguageIndex(tag string) uint8 {
	if tag == "" || tag == doc.Language {
		return 0
	}
	if index := slices.Index(doc.Languages, tag); index >= 0 {
		return uint8(index + 1)
	}
	if len(doc.Languages) == 255 {
		return 0
	}
	doc.Languages = append(doc.Languages, tag)
	return uint8(len(doc.Languages))
}

// Length returns the length of the block, in runes: an image and a rule count as one.
func (block Block) Length() int {
	if block.Kind == Image || block.Kind == Rule {
		return 1
	}
	length := 0
	for _, run := range block.Runs {
		if run.Image != "" {
			length++
			continue
		}
		length += utf8.RuneCountInString(run.Text)
	}
	return length
}
