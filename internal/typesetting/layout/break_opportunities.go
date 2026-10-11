package layout

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
)

// objectReplacement stands for an inline image in the text given to the Unicode algorithms.
const objectReplacement = '\ufffc'

// breakOpportunities returns, for each rune of block (counted from its start), whether a line may
// break after it: by the Unicode line breaking algorithm (UAX #14), as the block's WordBreak and
// LineBreak adjust it. Chinese and Japanese have no spaces: a line may break between most
// ideographs, but not before a closing bracket or a small kana (kinsoku). It returns nil for a
// block of alphabetic text, which breaks at spaces and dashes only, so that it does not pay for
// the algorithm. wide is set if the block's text has Chinese, Japanese, Korean or Yi characters.
func breakOpportunities(block document.Block, wide bool) []bool {
	if block.WordBreak != document.BreakAll && block.LineBreak != document.LineBreakAnywhere && !wide {
		return nil
	}
	var text strings.Builder
	for _, run := range block.Runs {
		if run.Image != "" {
			text.WriteRune(objectReplacement)
			continue
		}
		text.WriteString(run.Text)
	}
	runes := []rune(text.String())
	breaks := make([]bool, len(runes))
	rest, state, offset := text.String(), -1, 0
	for rest != "" {
		var segment string
		segment, rest, _, state = uniseg.FirstLineSegmentInString(rest, state)
		offset += utf8.RuneCountInString(segment)
		if rest != "" {
			breaks[offset-1] = true
		}
	}
	for index := range len(runes) - 1 {
		previous, next := runes[index], runes[index+1]
		switch {
		case block.LineBreak == document.LineBreakAnywhere:
			breaks[index] = true
		case block.WordBreak == document.BreakAll && isLetterOrDigit(previous) && isLetterOrDigit(next):
			breaks[index] = true
		case block.WordBreak == document.KeepAll && isLetterOrDigit(previous) && isLetterOrDigit(next):
			breaks[index] = false
		case block.LineBreak != document.LineBreakStrict && isSmallKana(next) && document.IsWide(previous) && isLetterOrDigit(previous):
			// CSS breaks before small kana and the prolonged sound mark unless strict, which
			// UAX #14 alone does not.
			breaks[index] = true
		}
	}
	return breaks
}

func isLetterOrDigit(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character)
}

// isSmallKana reports whether character is a small kana or the prolonged sound mark: the
// conditional Japanese starters (class CJ) of UAX #14.
func isSmallKana(character rune) bool {
	switch character {
	case 'ぁ', 'ぃ', 'ぅ', 'ぇ', 'ぉ', 'っ', 'ゃ', 'ゅ', 'ょ', 'ゎ', 'ゕ', 'ゖ',
		'ァ', 'ィ', 'ゥ', 'ェ', 'ォ', 'ッ', 'ャ', 'ュ', 'ョ', 'ヮ', 'ヵ', 'ヶ', 'ー':
		return true
	}
	return character >= 0x31f0 && character <= 0x31ff || character >= 0xff67 && character <= 0xff70
}
