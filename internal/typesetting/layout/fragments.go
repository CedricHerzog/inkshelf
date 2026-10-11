package layout

import (
	"strings"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// The characters that line breaking treats apart from the rest of the text.
const (
	softHyphen     = '\u00ad' // a break point, drawn as a hyphen only when the line breaks there
	zeroWidthSpace = '\u200b' // a break point that is never drawn
	narrowNBSP     = '\u202f' // the space French puts before ; : ! ?, never a break point
	nbsp           = '\u00a0'
)

// breakKind is what lies after a fragment of text: whether a line may break there, and what the
// break leaves on the page.
type breakKind int

const (
	// breakNone continues the word: the next fragment is the same word in another style.
	breakNone breakKind = iota
	// breakSpace is a space: a line may break there, and justification stretches it.
	breakSpace
	// breakSoft is a soft hyphen: a line may break there, and then ends with a hyphen.
	breakSoft
	// breakDirect follows a dash, a zero-width space or an ideograph: a line may break there,
	// with no space between the two sides.
	breakDirect
)

// fragment is a stretch of text with no break point inside, in one format, or an inline image.
// Offsets count the runes of the document, so a fragment knows where it came from whatever the
// layout.
type fragment struct {
	text       string
	format     *runFormat
	start      int // the offset of its first rune
	runes      int
	width      int
	breakAfter breakKind
}

// word is fragments that stay together on a line. It is usually one fragment; a word that changes
// style in its middle ("un*believ*able") is several.
type word struct {
	fragments []fragment
	width     int
	// whole is set on a word that hyphenation may still split: one that was neither cut
	// already nor written with a soft hyphen, which says where the book wants it broken.
	whole bool
}

func (word word) breakAfter() breakKind { return word.fragments[len(word.fragments)-1].breakAfter }

func (word *word) add(fragment fragment) {
	word.fragments = append(word.fragments, fragment)
	word.width += fragment.width
}

func (word word) runes() int {
	runes := 0
	for _, fragment := range word.fragments {
		runes += fragment.runes
	}
	return runes
}

func (word word) text() string {
	if len(word.fragments) == 1 {
		return word.fragments[0].text
	}
	var text strings.Builder
	for _, fragment := range word.fragments {
		text.WriteString(fragment.text)
	}
	return text.String()
}

// splitFragments cuts the runs of block into fragments, each measured in its run's format.
//
// Spaces, soft hyphens and zero-width spaces end a fragment and are not drawn, but they still
// count in the offsets. Alphabetic text also breaks after dashes; text with Chinese or Japanese
// (text.breaks is set) breaks where the Unicode line breaking algorithm allows instead. A
// fragment never mixes two bidirectional levels, so that it can be drawn in one direction: a
// right-to-left fragment, or one in a script whose letters join, is measured and drawn shaped.
//
// This is the hottest loop of the layout, which is why it stays one function with closures: it
// runs once per character of every chapter laid out.
func splitFragments(block *document.Block, formats []*runFormat, faces *Faces, text *blockText) []fragment {
	var fragments []fragment
	offset := block.Start
	opportunities, levels := text.breaks, text.levels
	// markBreakAfterLast puts a break point after the last fragment, if it has none. A space
	// also replaces the break after an image or a dash, so that "a – b" keeps its spaces.
	markBreakAfterLast := func(kind breakKind) {
		if last := len(fragments) - 1; last >= 0 && (fragments[last].breakAfter == breakNone || kind == breakSpace && (fragments[last].breakAfter == breakDirect || fragments[last].format.image != "")) {
			fragments[last].breakAfter = kind
		}
	}
	for index, run := range block.Runs {
		format := formats[index]
		if run.Image != "" {
			markBreakAfterLast(breakDirect)
			fragments = append(fragments, fragment{format: format, start: offset, runes: 1, width: format.imageSize.X, breakAfter: breakDirect})
			offset++
			continue
		}
		var characters strings.Builder
		start, runes := offset, 0
		endFragment := func(kind breakKind) {
			if runes > 0 {
				content := characters.String()
				fragmentFormat := format
				if levels != nil || text.shaped {
					if rightToLeft := text.level(start)%2 == 1; rightToLeft || text.shaped && containsShaped(content) {
						fragmentFormat = format.shapedFormat(rightToLeft, text.languageOf(run))
					}
				}
				fragments = append(fragments, fragment{text: content, format: fragmentFormat, start: start, runes: runes, width: fragmentFormat.width(faces, content), breakAfter: kind})
			} else if kind != breakNone {
				markBreakAfterLast(kind) // the run starts with a break point: it goes after the previous run
			}
			characters.Reset()
			runes = 0
		}
		for _, character := range run.Text {
			switch character {
			case ' ', '\t', '\n', '\r':
				endFragment(breakSpace)
			case softHyphen:
				endFragment(breakSoft)
			case zeroWidthSpace:
				endFragment(breakDirect)
			default:
				if levels != nil && runes > 0 && text.level(offset) != text.level(start) {
					endFragment(breakNone)
				}
				if runes == 0 {
					start = offset
				}
				if character == narrowNBSP && !format.font.Has(narrowNBSP) {
					character = nbsp // many fonts lack the narrow one, and it must not break either
				}
				characters.WriteRune(character)
				runes++
				if opportunities != nil {
					if opportunities[offset-block.Start] {
						endFragment(breakDirect)
					}
				} else if character == '-' || character == '–' || character == '—' {
					endFragment(breakDirect)
				}
			}
			offset++
		}
		endFragment(breakNone)
	}
	return fragments
}

// groupWords groups fragments into words, which a line may only break after. The words share the
// fragments' array rather than copy it.
func groupWords(fragments []fragment) []word {
	grouped := make([]word, 0, len(fragments))
	first, width, hasImage, afterSoft := 0, 0, false, false
	for index, fragment := range fragments {
		width += fragment.width
		hasImage = hasImage || fragment.format.image != ""
		if fragment.breakAfter != breakNone || index == len(fragments)-1 {
			whole := !hasImage && !afterSoft && fragment.breakAfter != breakSoft
			grouped = append(grouped, word{fragments: fragments[first : index+1 : index+1], width: width, whole: whole})
			first, width, hasImage, afterSoft = index+1, 0, false, fragment.breakAfter == breakSoft
		}
	}
	return grouped
}

// measurer returns the width of text in format, in pixels.
type measurer func(text string, format *runFormat) int
