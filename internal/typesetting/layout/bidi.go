package layout

import (
	"slices"
	"unicode/utf8"

	"github.com/go-text/typesetting/bidi"
	unicodebidi "golang.org/x/text/unicode/bidi"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// containsShaped reports whether text holds a character of a script that needs shaping.
func containsShaped(text string) bool {
	for _, character := range text {
		if document.NeedsShaping(character) {
			return true
		}
	}
	return false
}

// bidiLevels returns the embedding level of each rune of block (from its start), by the Unicode
// bidirectional algorithm, and whether its paragraph is right to left. Levels are nil for a
// block of left-to-right text only, which is most blocks.
//
// The runs of an Isolate (an inline element with its own direction) are worked out apart, as the
// isolate controls of plain text would make them: in the paragraph they count as one neutral
// character; inside, they are a paragraph of their own, one or two levels up. An override puts
// all of its text at that level, whatever its characters.
func bidiLevels(block document.Block, rightToLeftText bool) ([]int8, bool) {
	isolated := slices.ContainsFunc(block.Runs, func(run document.Run) bool { return run.Isolate != document.NoIsolate })
	if !rightToLeftText && !isolated && block.Direction != document.RightToLeft {
		return nil, false
	}
	// The paragraph's text, each isolate standing as an object replacement character, and the
	// isolates' own texts.
	type isolateText struct {
		isolate document.Isolate
		text    []rune
	}
	paragraph := make([]rune, 0, block.Length())
	var isolates []isolateText
	for index, run := range block.Runs {
		text := []rune(run.Text)
		if run.Image != "" {
			text = []rune{objectReplacement}
		}
		switch {
		case run.Isolate == document.NoIsolate:
			paragraph = append(paragraph, text...)
		case index > 0 && block.Runs[index-1].Isolate == run.Isolate:
			last := &isolates[len(isolates)-1]
			last.text = append(last.text, text...)
		default:
			paragraph = append(paragraph, objectReplacement)
			isolates = append(isolates, isolateText{isolate: run.Isolate, text: text})
		}
	}
	rightToLeft := block.Direction == document.RightToLeft
	if block.Direction == document.DirectionAuto {
		rightToLeft = firstStrongIsRightToLeft(paragraph)
	}
	paragraphLevels := segmentLevels(paragraph, rightToLeft)
	if len(isolates) == 0 {
		return paragraphLevels, rightToLeft
	}
	levels := make([]int8, 0, block.Length())
	position, isolateIndex := 0, 0 // where the next run is in paragraph and isolates
	for index, run := range block.Runs {
		switch {
		case run.Isolate == document.NoIsolate:
			runes := utf8.RuneCountInString(run.Text)
			if run.Image != "" {
				runes = 1
			}
			levels = append(levels, paragraphLevels[position:position+runes]...)
			position += runes
		case index > 0 && block.Runs[index-1].Isolate == run.Isolate:
			// Its levels came with the first run of its isolate.
		default:
			isolate := isolates[isolateIndex]
			levels = append(levels, isolateLevels(isolate.isolate, isolate.text, paragraphLevels[position])...)
			position++
			isolateIndex++
		}
	}
	return levels, rightToLeft
}

// isolateLevels returns the levels of the text of an isolate that stands at level outer in its
// paragraph.
func isolateLevels(isolate document.Isolate, text []rune, outer int8) []int8 {
	rightToLeft := isolate == document.IsolateRightToLeft || isolate == document.OverrideRightToLeft ||
		isolate == document.IsolateAuto && firstStrongIsRightToLeft(text)
	// The next level up in the isolate's direction: odd levels are right to left, even ones left
	// to right.
	level := outer + 1
	if (level%2 == 1) != rightToLeft {
		level++
	}
	levels := make([]int8, len(text))
	if isolate == document.OverrideLeftToRight || isolate == document.OverrideRightToLeft {
		for index := range levels {
			levels[index] = level
		}
		return levels
	}
	base := int8(0)
	if rightToLeft {
		base = 1
	}
	for index, inner := range segmentLevels(text, rightToLeft) {
		levels[index] = inner - base + level
	}
	return levels
}

// segmentLevels returns the level of each rune of text, a paragraph in a direction, by the
// bidirectional algorithm (go-text's). go-text gives the levels of runs of one direction, so a
// run that mixes two levels of one direction (Arabic digits after Latin text) takes its first's.
func segmentLevels(text []rune, rightToLeft bool) []int8 {
	direction := bidi.LeftToRight
	if rightToLeft {
		direction = bidi.RightToLeft
	}
	var paragraph bidi.Paragraph
	runs := paragraph.Segment(text, direction)
	levels := make([]int8, len(text))
	for index := range runs.NumRuns() {
		run := runs.Run(index)
		for position := run.Start; position < run.End; position++ {
			levels[position] = int8(run.Level)
		}
	}
	return levels
}

// firstStrongIsRightToLeft reports whether the first strong character of text (a letter, which
// has a direction), out of isolates, is right to left: rules P2 and P3 of the bidirectional
// algorithm, which give a paragraph its direction.
func firstStrongIsRightToLeft(text []rune) bool {
	depth := 0
	for _, character := range text {
		properties, _ := unicodebidi.LookupRune(character)
		switch properties.Class() {
		case unicodebidi.LRI, unicodebidi.RLI, unicodebidi.FSI:
			depth++
		case unicodebidi.PDI:
			depth = max(depth-1, 0)
		case unicodebidi.L:
			if depth == 0 {
				return false
			}
		case unicodebidi.R, unicodebidi.AL:
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// reorder puts items in visual order, from left to right, as rule L2 of the bidirectional
// algorithm does: from the highest level down to the lowest odd one, it reverses every run of
// items at that level or higher. Text at level 1 is reversed once (right to left), digits at
// level 2 inside it twice (left to right again).
func reorder[Item any](items []Item, level func(item Item) int8) {
	highest, lowestOdd := int8(0), int8(127)
	for _, item := range items {
		itemLevel := level(item)
		highest, lowestOdd = max(highest, itemLevel), min(lowestOdd, itemLevel|1)
	}
	for current := highest; current >= lowestOdd; current-- {
		for start := 0; start < len(items); {
			if level(items[start]) < current {
				start++
				continue
			}
			end := start
			for end < len(items) && level(items[end]) >= current {
				end++
			}
			slices.Reverse(items[start:end])
			start = end
		}
	}
}
