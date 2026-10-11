package layout

import (
	"slices"
	"unicode/utf8"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/hyphen"
)

// lineSpacer measures what goes between words on a line, and the hyphen after the last one.
type lineSpacer struct{ measure measurer }

// space returns the width of the space after word, in the format of its last fragment.
func (spacer lineSpacer) space(word word) int {
	return word.fragments[len(word.fragments)-1].format.space
}

// hyphen returns the width of a hyphen after word.
func (spacer lineSpacer) hyphen(word word) int {
	return spacer.measure("-", word.fragments[len(word.fragments)-1].format)
}

// gapAfter returns the gap between word and the next word on its line: a space, or nothing
// after a dash or between ideographs.
func (spacer lineSpacer) gapAfter(word word) int {
	if word.breakAfter() == breakSpace {
		return spacer.space(word)
	}
	return 0
}

// stretches reports whether justification widens the gap between word and next. Spaces
// stretch; so do the breaks next to an ideograph, as Chinese and Japanese have no spaces to
// stretch.
func stretches(word, next word) bool {
	switch word.breakAfter() {
	case breakSpace:
		return true
	case breakDirect:
		return document.IsWide(lastRune(word)) || document.IsWide(firstRune(next))
	}
	return false
}

// lineBreaker breaks words into lines greedily: each line takes all the words that fit on it,
// and the next word starts the next line. TeX weighs every way of breaking a paragraph at once
// (Knuth and Plass) for even spacing; greedy breaking is what browsers and e-readers do, and is
// fast enough to lay out a chapter at every font size change.
//
// A word that does not fit is first hyphenated, if the block asks for it; a word wider than a
// whole line is cut between two characters, wherever the line ends.
type lineBreaker struct {
	firstWidth, width int // the first line's width, and the others'
	// hyphenator is nil when the block is not hyphenated or its language has no patterns.
	hyphenator *hyphen.Hyphenator
}

// hyphenate splits a word at its last hyphenation point that leaves a start which, with its
// hyphen, fits in room: the line gets as much of the word as it can. A word that cannot be split
// so is returned whole.
func (breaker *lineBreaker) hyphenate(candidate word, room int, spacer lineSpacer) []word {
	for _, point := range slices.Backward(breaker.hyphenator.Points(candidate.text())) {
		if parts := splitWord(candidate, point, breakSoft, spacer.measure); parts[0].width+spacer.hyphen(parts[0]) <= room {
			return parts
		}
	}
	return []word{candidate}
}

// breakLines breaks words into lines, measuring what goes between them with spacer.
func (breaker *lineBreaker) breakLines(words []word, spacer lineSpacer) [][]word {
	filling := lineFiller{firstWidth: breaker.firstWidth, otherWidth: breaker.width}
	queue := wordQueue{words: words}
	for !queue.empty() {
		candidate := queue.next()
		gap := 0
		if len(filling.line) > 0 {
			gap = spacer.gapAfter(filling.line[len(filling.line)-1])
		}
		available := filling.available()
		switch {
		case filling.width+gap+candidate.width <= available:
			filling.line = append(filling.line, candidate)
			filling.width += gap + candidate.width
			queue.pop()
		case candidate.whole && breaker.hyphenator != nil:
			queue.pop()
			candidate.whole = false // so that a word that cannot be hyphenated is tried only once
			queue.pushFront(breaker.hyphenate(candidate, available-filling.width-gap, spacer))
		case len(filling.line) > 0:
			filling.finishLine(spacer)
		default: // alone on its line and still too wide: cut it
			queue.pop()
			cutParts := cutWord(candidate, available, spacer.measure)
			if len(cutParts) == 1 { // cannot be cut further: put it on its own line anyway
				filling.line, filling.width = cutParts, candidate.width
				filling.finishLine(spacer)
				continue
			}
			queue.pushFront(cutParts)
		}
	}
	if len(filling.line) > 0 {
		filling.lines = append(filling.lines, filling.line)
	}
	return filling.lines
}

// lineFiller holds the lines broken so far and the line being filled.
type lineFiller struct {
	firstWidth, otherWidth int // the first line's width, and the others'
	lines                  [][]word
	line                   []word
	width                  int // of the line being filled, its gaps included
}

// available returns the width the line being filled may take: the first line is shorter by
// its indent.
func (filling *lineFiller) available() int {
	if len(filling.lines) == 0 {
		return filling.firstWidth
	}
	return filling.otherWidth
}

// finishLine ends the line being filled. A line that ends at a soft hyphen draws a hyphen,
// which breaking did not count: if it does not fit, the line's last word moves to the next line.
func (filling *lineFiller) finishLine(spacer lineSpacer) {
	line := filling.line
	last := line[len(line)-1]
	if len(line) > 1 && last.breakAfter() == breakSoft && filling.width+spacer.hyphen(last) > filling.available() {
		filling.lines = append(filling.lines, line[:len(line)-1])
		filling.line, filling.width = []word{last}, last.width
		return
	}
	filling.lines = append(filling.lines, line)
	filling.line, filling.width = nil, 0
}

// wordQueue is the words left to put on lines. When a word is split, by hyphenation or a cut,
// its parts go back to the front of the queue, before the words after it.
type wordQueue struct {
	words, parts []word
}

func (queue *wordQueue) empty() bool { return len(queue.parts) == 0 && len(queue.words) == 0 }

// next returns the word that comes next, without taking it from the queue.
func (queue *wordQueue) next() word {
	if len(queue.parts) > 0 {
		return queue.parts[0]
	}
	return queue.words[0]
}

// pop takes the next word from the queue.
func (queue *wordQueue) pop() {
	if len(queue.parts) > 0 {
		queue.parts = queue.parts[1:]
	} else {
		queue.words = queue.words[1:]
	}
}

func (queue *wordQueue) pushFront(parts []word) {
	queue.parts = append(parts, queue.parts...)
}

// splitWord splits a word before its rune at offset at, counted from its start: it returns the
// start, ending with breakAfter, and the rest. A fragment cut in two is measured again in both
// halves, as kerning or shaping across the cut may change their widths.
func splitWord(unsplit word, at int, breakAfter breakKind, measure measurer) []word {
	var head, tail word
	offset := 0 // the runes of the word before fragment
	for _, fragment := range unsplit.fragments {
		switch {
		case offset+fragment.runes <= at:
			head.add(fragment)
		case offset >= at:
			tail.add(fragment)
		default:
			start, rest := cutFragment(fragment, at-offset, measure)
			head.add(start)
			tail.add(rest)
		}
		offset += fragment.runes
	}
	head.fragments[len(head.fragments)-1].breakAfter = breakAfter
	return []word{head, tail}
}

// cutFragment cuts fragment after its first runes runes.
func cutFragment(fragment fragment, runes int, measure measurer) (head, tail fragment) {
	index := 0
	for range runes {
		_, size := utf8.DecodeRuneInString(fragment.text[index:])
		index += size
	}
	head, tail = fragment, fragment
	head.text, head.runes, head.breakAfter = fragment.text[:index], runes, breakNone
	tail.text, tail.runes, tail.start = fragment.text[index:], fragment.runes-runes, fragment.start+runes
	head.width, tail.width = measure(head.text, fragment.format), measure(tail.text, fragment.format)
	return head, tail
}

// cutWord cuts a word too wide for a line in two, between characters: the longest start that
// fits in width (one character at least, so that the layout always moves on), and the rest. A
// word of one character is returned whole.
func cutWord(tooWide word, width int, measure measurer) []word {
	runes := tooWide.runes()
	if runes < 2 {
		return []word{tooWide}
	}
	fitting := 1
	for fitting+1 < runes && splitWord(tooWide, fitting+1, breakDirect, measure)[0].width <= width {
		fitting++
	}
	return splitWord(tooWide, fitting, breakDirect, measure)
}

// firstRune returns the first rune of word, or -1 for an image.
func firstRune(word word) rune {
	first := word.fragments[0]
	if first.format.image != "" {
		return -1
	}
	character, _ := utf8.DecodeRuneInString(first.text)
	return character
}

// lastRune returns the last rune of word, or -1 for an image.
func lastRune(word word) rune {
	last := word.fragments[len(word.fragments)-1]
	if last.format.image != "" {
		return -1
	}
	character, _ := utf8.DecodeLastRuneInString(last.text)
	return character
}
