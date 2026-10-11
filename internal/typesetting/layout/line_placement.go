package layout

import (
	"image"
	"unicode"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// laidOutLine is a line laid out across the page but not yet down it: its words, inline images,
// underlines and links are placed from the top of the line, and move down with it when the
// paginator puts the line on a page.
type laidOutLine struct {
	Line
	height  int
	images  []PlacedImage
	strokes []Stroke
	links   []PlacedLink
}

// layOutLines lays block's text out into lines at size: it prepares the text, breaks it into
// lines, and places each line's words across the page.
func (layout *paginator) layOutLines(block document.Block, size int) ([]laidOutLine, error) {
	prepared, err := layout.prepareText(block, size)
	if err != nil || len(prepared.words) == 0 {
		return nil, err
	}
	block, left, width := prepared.block, prepared.left, prepared.width
	measure := func(text string, format *runFormat) int { return format.width(layout.faces, text) }
	spacer := lineSpacer{measure: measure}
	shared := blockLines{spacer: spacer, measure: measure,
		plain: plainLine{size: size, height: prepared.lineHeight, ascent: prepared.baseFont.Ascent()}}
	if prepared.text.levels != nil {
		text := prepared.text
		shared.levels = text.level
	}
	indent := min(layout.settings.em(block.Indent), width/2)
	breaker := lineBreaker{firstWidth: width - indent, width: width}
	if block.Hyphenate {
		breaker.hyphenator = layout.hyphenator
	}
	broken := breaker.breakLines(prepared.words, spacer)
	align, lastAlign := layout.lineAlignments(block)
	lines := make([]laidOutLine, len(broken))
	for index, words := range broken {
		lineLeft, lineWidth := left, width
		if index == 0 {
			lineWidth = width - indent
			if !prepared.rightToLeft {
				lineLeft = left + indent
			}
		}
		last := index == len(broken)-1
		lineAlign := align
		if last {
			lineAlign = lastAlign
		}
		hyphenated := !last && words[len(words)-1].breakAfter() == breakSoft
		lines[index] = placeWords(words, &shared, fitLine(words, spacer, lineLeft, lineWidth, lineAlign, hyphenated))
	}
	return lines, nil
}

// fitLine returns how a line of words, from left and width wide, is aligned by align: where its
// first word starts, and the pixels that justification spreads over its gaps. A line wider than
// width (a word that could not be cut) starts at left whatever its alignment.
func fitLine(words []word, spacer lineSpacer, left, width int, align document.Align, hyphenated bool) lineFit {
	natural, gaps := 0, 0
	for position, word := range words {
		natural += word.width
		if position < len(words)-1 {
			natural += spacer.gapAfter(word)
			if stretches(word, words[position+1]) {
				gaps++
			}
		}
	}
	if hyphenated {
		natural += spacer.hyphen(words[len(words)-1])
	}
	fit := lineFit{x: left, gaps: gaps, hyphenated: hyphenated}
	switch align {
	case document.AlignCenter:
		fit.x += max(width-natural, 0) / 2
	case document.AlignRight:
		fit.x += max(width-natural, 0)
	case document.AlignJustify:
		if gaps > 0 {
			fit.extra = max(width-natural, 0)
		}
	}
	return fit
}

// lineAlignments returns how block's lines align, and how its last line does: AlignLeft,
// AlignCenter, AlignRight or AlignJustify. The last line of a justified block is not stretched
// but aligned at the start, as in print, unless AlignLast says otherwise.
func (layout *paginator) lineAlignments(block document.Block) (lines, last document.Align) {
	lines = layout.resolveAlign(block.Align, block)
	if block.AlignLast != document.AlignDefault {
		return lines, layout.resolveAlign(block.AlignLast, block)
	}
	if lines == document.AlignJustify {
		return lines, layout.resolveAlign(document.AlignStart, block)
	}
	return lines, lines
}

// resolveAlign turns align into AlignLeft, AlignCenter, AlignRight or AlignJustify: the start of
// a line is its right in right-to-left text.
func (layout *paginator) resolveAlign(align document.Align, block document.Block) document.Align {
	switch align {
	case document.AlignDefault:
		if layout.settings.Justify && block.Kind == document.Paragraph {
			return document.AlignJustify
		}
		return layout.resolveAlign(document.AlignStart, block)
	case document.AlignStart:
		if block.Direction == document.RightToLeft {
			return document.AlignRight
		}
		return document.AlignLeft
	case document.AlignEnd:
		if block.Direction == document.RightToLeft {
			return document.AlignLeft
		}
		return document.AlignRight
	}
	return align
}

// plainLine is a line of a block's text in the block's own font: its size, height and ascent. A
// line is never lower than this, so that lines of plain text are evenly spaced.
type plainLine struct{ size, height, ascent int }

// lineItem is one thing that a line places from left to right: a fragment, or the gap after a
// word.
type lineItem struct {
	fragment *fragment // nil for a gap
	// hyphen is set on the fragment that ends a hyphenated line: a hyphen goes after it.
	hyphen bool
	// gap is a gap's width; stretch is set if justification widens it.
	gap     int
	stretch bool
	level   int8 // the bidirectional level, which orders the items
}

// blockLines is what the lines of a block share when their words are placed.
type blockLines struct {
	spacer  lineSpacer
	measure measurer
	plain   plainLine
	// levels returns the bidirectional level of the rune at an offset. It is nil for a block of
	// left-to-right text only, whose lines need no reordering.
	levels func(offset int) int8
}

// lineFit is how a line's words fill its width: where the first one starts, and how many pixels
// justification spreads over the gaps that stretch.
type lineFit struct {
	x, extra, gaps int
	// hyphenated is set when the line ends inside a word, which then gets a hyphen.
	hyphenated bool
}

// measureLine returns the baseline of a line of words, from its top, its height and how many
// fragments it has. All the words of a line share its baseline. The line is as high as plain,
// made higher by a larger font (in proportion, so that the leading grows with it), by a fragment
// that rises above plain's ascent (by the difference), by emphasis marks, or by a formula that
// goes under the baseline.
func measureLine(words []word, plain plainLine) (baseline, height, count int) {
	baseline, largest, below := plain.ascent, plain.size, 0
	formulaBelow := 0 // how far formulas go under the baseline
	for _, word := range words {
		count += len(word.fragments)
		for _, fragment := range word.fragments {
			baseline = max(baseline, fragment.ascent())
			if fragment.format.image == "" {
				largest = max(largest, fragment.format.font.Size())
			}
			if fragment.format.marks != nil {
				baseline = max(baseline, fragment.ascent()+fragment.marksAbove())
				below = max(below, fragment.marksBelow())
			}
			formulaBelow = max(formulaBelow, fragment.format.imageDescent-fragment.format.shift)
		}
	}
	// The leading is the space under the plain text's descenders, which go about a quarter of its
	// size below the baseline. A formula that goes lower keeps it under itself.
	leading := max(plain.height-plain.ascent-plain.size/4, 0)
	height = max(plain.height*largest/plain.size, baseline+plain.height-plain.ascent, baseline+below, baseline+formulaBelow+leading)
	return baseline, height, count
}

// placeWords places a line's words as fit says and returns the line. Justification spreads
// fit.extra pixels over the gaps that stretch, a pixel more for the first ones when it does not
// divide evenly. When the block has bidirectional levels, the fragments and gaps are placed in
// visual order rather than in the order of the text.
func placeWords(words []word, lines *blockLines, fit lineFit) laidOutLine {
	spacer, measure, plain, levels := lines.spacer, lines.measure, lines.plain, lines.levels
	x, extra, gaps, hyphenated := fit.x, fit.extra, fit.gaps, fit.hyphenated
	baseline, height, count := measureLine(words, plain)
	lastWord := words[len(words)-1]
	lastFragment := lastWord.fragments[len(lastWord.fragments)-1]
	line := laidOutLine{
		Words: make([]PlacedWord, 0, count), Start: words[0].fragments[0].start, End: lastFragment.start + lastFragment.runes,
		height: height,
	}
	gapIndex := 0
	var previous *runFormat // the format of the fragment before, whose underline may go on
	place := func(placed lineItem) {
		if placed.fragment == nil {
			x += placed.gap
			if placed.stretch {
				if gaps > 0 {
					x += extra / gaps
					if gapIndex < extra%gaps {
						x++
					}
				}
				gapIndex++
			}
			return
		}
		fragment := placed.fragment
		format := fragment.format
		itemBaseline := baseline - format.shift
		if format.image != "" {
			if size := format.imageSize; size.X > 0 && size.Y > 0 {
				bottom := itemBaseline + format.imageDescent
				line.images = append(line.images, PlacedImage{Path: format.image, Rect: image.Rect(x, bottom-size.Y, x+size.X, bottom), MathSize: format.mathSize})
			}
		} else {
			text := fragment.text
			if placed.hyphen {
				text += "-"
			}
			line.Words = append(line.Words, PlacedWord{X: x, Top: itemBaseline - format.font.Ascent(), Text: text,
				Style: format.style, Size: format.size, Family: format.family, Spacing: format.spacing, Gray: format.gray, Shaping: format.shaping})
			if format.marks != nil {
				line.addMarks(*fragment, x, itemBaseline, measure)
			}
			line.addStrokes(format, x, x+fragment.width, baseline, plain.size, format == previous)
		}
		if format.link != "" {
			line.addLink(format.link, x, x+fragment.width)
		}
		previous = format
		x += fragment.width
	}
	if levels == nil {
		for position, word := range words {
			for fragmentIndex := range word.fragments {
				place(lineItem{fragment: &word.fragments[fragmentIndex], hyphen: hyphenated && position == len(words)-1 && fragmentIndex == len(word.fragments)-1})
			}
			if position < len(words)-1 {
				place(lineItem{gap: spacer.gapAfter(word), stretch: stretches(word, words[position+1])})
			}
		}
		return line
	}
	for _, placed := range visualOrder(words, levels, hyphenated, spacer) {
		place(placed)
	}
	return line
}

// visualOrder returns the fragments of a line's words and the gaps between them in the order
// they appear from left to right, which their bidirectional levels give.
func visualOrder(words []word, levels func(offset int) int8, hyphenated bool, spacer lineSpacer) []lineItem {
	var items []lineItem
	for position, word := range words {
		for fragmentIndex := range word.fragments {
			fragment := &word.fragments[fragmentIndex]
			items = append(items, lineItem{fragment: fragment, level: levels(fragment.start),
				hyphen: hyphenated && position == len(words)-1 && fragmentIndex == len(word.fragments)-1})
		}
		if position < len(words)-1 {
			last := word.fragments[len(word.fragments)-1]
			// A gap takes the level of the space after the word, or of the word if it has none.
			level := levels(last.start)
			if word.breakAfter() == breakSpace {
				level = levels(last.start + last.runes)
			}
			items = append(items, lineItem{gap: spacer.gapAfter(word), stretch: stretches(word, words[position+1]), level: level})
		}
	}
	reorder(items, func(item lineItem) int8 { return item.level })
	return items
}

// ascent returns how high the fragment rises above its own baseline: its font's ascent, or its
// image's height (a formula's height above its baseline).
func (fragment fragment) ascent() int {
	if fragment.format.image != "" {
		return fragment.format.imageSize.Y - fragment.format.imageDescent
	}
	return fragment.format.font.Ascent()
}

// marksAbove returns how far emphasis marks above the fragment rise over its font's ascent.
func (fragment *fragment) marksAbove() int {
	if fragment.format.marks == nil || fragment.format.marks.Under {
		return 0
	}
	return fragment.format.marks.format.font.Ascent()
}

// marksBelow returns how far below the baseline emphasis marks under the fragment go, 0 if it
// has none there.
func (fragment *fragment) marksBelow() int {
	if fragment.format.marks == nil || !fragment.format.marks.Under {
		return 0
	}
	marks := fragment.format.marks.format.font
	return fragment.format.font.Descent() + marks.Ascent() + marks.Descent()
}

// addMarks adds the emphasis marks of fragment, placed from x on baseline, as words of their own:
// one centred above or under each of its characters but spaces and punctuation.
func (line *laidOutLine) addMarks(fragment fragment, x, baseline int, measure measurer) {
	format, marks := fragment.format, fragment.format.marks.format
	mark := string(fragment.format.marks.Mark)
	markWidth := measure(mark, marks)
	rightToLeft := format.shaping != nil && format.shaping.RightToLeft
	if rightToLeft {
		x += fragment.width // its first character is at the right
	}
	top := baseline - format.font.Ascent() - marks.font.Ascent()
	if fragment.format.marks.Under {
		top = baseline + format.font.Descent()
	}
	for index, character := range fragment.text {
		if !emphasized(character) {
			continue
		}
		left := measure(fragment.text[:index], format)
		characterWidth := measure(fragment.text[:index+utf8.RuneLen(character)], format) - left
		characterX := x + left
		if rightToLeft {
			characterX = x - left - characterWidth
		}
		line.Words = append(line.Words, PlacedWord{X: characterX + (characterWidth-markWidth)/2, Top: top, Text: mark, Size: marks.size, Gray: format.gray})
	}
}

// emphasized reports whether character gets an emphasis mark: spaces, punctuation and controls
// do not.
func emphasized(character rune) bool {
	return !unicode.IsSpace(character) && !unicode.IsPunct(character) && !unicode.IsControl(character)
}

// strokeThickness returns the thickness of the underline and strike-through of text of size.
func strokeThickness(size int) int { return max(size/16, 1) }

// addStrokes adds the underline and the strike-through of text in format from left to right, if
// it has them, on a line whose baseline is lineBaseline and whose plain text is plainSize. With
// continued, the text is the same run as the fragment before, and its strokes extend that
// fragment's across the space between them rather than leave a gap. A raised run (a footnote
// call) is underlined where the line's text is: under its own baseline the line would touch it.
func (line *laidOutLine) addStrokes(format *runFormat, left, right, lineBaseline, plainSize int, continued bool) {
	size, baseline := format.font.Size(), lineBaseline-format.shift
	var strokes []Stroke
	if format.style&document.Underline != 0 {
		underlineSize, underlineBaseline := size, baseline
		if format.shift > 0 {
			underlineSize, underlineBaseline = plainSize, lineBaseline
		}
		top := underlineBaseline + underlineSize/10
		if format.style&document.UnderlineUnder != 0 {
			top = underlineBaseline + format.font.Descent()
		}
		strokes = append(strokes, Stroke{image.Rect(left, top, right, top+strokeThickness(underlineSize)), format.gray})
	}
	if format.style&document.Strike != 0 {
		top := baseline - size*3/10
		strokes = append(strokes, Stroke{image.Rect(left, top, right, top+strokeThickness(size)), format.gray})
	}
	for index, stroke := range strokes {
		if continued {
			line.strokes[len(line.strokes)-len(strokes)+index].Max.X = right
		} else {
			line.strokes = append(line.strokes, stroke)
		}
	}
}

// addLink adds the area of a link's text from left to right, as high as the line so that it is
// easy to tap; it extends the previous area when that is the same link's.
func (line *laidOutLine) addLink(target string, left, right int) {
	if last := len(line.links) - 1; last >= 0 && line.links[last].Target == target {
		line.links[last].Rect.Max.X = right
		return
	}
	line.links = append(line.links, PlacedLink{Rect: image.Rect(left, 0, right, line.height), Target: target})
}
