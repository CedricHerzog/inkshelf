package document

import "unicode/utf8"

// The bidirectional controls that start right-to-left text.
const (
	rightToLeftIsolate  = '\u2067'
	firstStrongIsolate  = '\u2068'
	rightToLeftOverride = '\u202e'
)

// Scripts are what the characters of a block's text need beyond what Latin text needs: lines
// broken at spaces, and words drawn character by character from left to right.
type Scripts struct {
	Wide        bool // Chinese, Japanese, Korean or Yi: lines break between characters
	RightToLeft bool // Hebrew, Arabic… or a right-to-left control: bidirectional levels
	Shaped      bool // a script whose letters join or combine: shaping
	// Scanned is set by ScanScripts: the other fields are known, not just false.
	Scanned bool
}

// ScanScripts returns what the characters of runs need. Latin, Greek and Cyrillic text costs one
// comparison per byte: every script it looks for starts at U+0590 or above, whose UTF-8 encoding
// starts with a byte of 0xD6 or more.
func ScanScripts(runs []Run) Scripts {
	found := Scripts{Scanned: true}
	for _, run := range runs {
		text := run.Text
		for index := 0; index < len(text); index++ {
			if text[index] < 0xd6 {
				continue
			}
			character, size := utf8.DecodeRuneInString(text[index:])
			found.Wide = found.Wide || IsWide(character)
			found.RightToLeft = found.RightToLeft || isRightToLeft(character)
			found.Shaped = found.Shaped || NeedsShaping(character)
			index += size - 1
		}
	}
	return found
}

// isRightToLeft reports whether character is of a right-to-left script (Hebrew, Arabic, Syriac,
// Thaana, N'Ko…) or a control that starts right-to-left text.
func isRightToLeft(character rune) bool {
	switch {
	case character < 0x0590:
		return false
	case character <= 0x08ff, character >= 0xfb1d && character <= 0xfdff, character >= 0xfe70 && character <= 0xfeff:
		return true
	case character == '\u200f', character == '\u202b', character == rightToLeftOverride, character == rightToLeftIsolate, character == firstStrongIsolate:
		return true
	}
	return character >= 0x10800 && character <= 0x10fff || character >= 0x1e800 && character <= 0x1efff
}

// NeedsShaping reports whether character is of a script whose letters join, reorder or carry
// marks placed by the font: Hebrew's points, Arabic, the scripts of India and of South-East
// Asia, Tibetan, Mongolian.
func NeedsShaping(character rune) bool {
	switch {
	case character < 0x0590:
		return false
	case character <= 0x109f: // Hebrew to Myanmar
		return true
	case character >= 0x1780 && character <= 0x18af: // Khmer, Mongolian
		return true
	}
	return character >= 0xfb1d && character <= 0xfdff || character >= 0xfe70 && character <= 0xfeff ||
		character >= 0x10800 && character <= 0x10fff || character >= 0x1e800 && character <= 0x1efff
}

// IsWide reports whether character is Chinese, Japanese, Korean or Yi: an ideograph, kana,
// hangul, CJK punctuation or a fullwidth form.
func IsWide(character rune) bool {
	switch {
	case character < 0x2e80:
		return false
	case character <= 0xa4cf: // CJK radicals to Yi
		return true
	case character >= 0xac00 && character <= 0xd7af: // hangul syllables
		return true
	case character >= 0xf900 && character <= 0xfaff: // compatibility ideographs
		return true
	case character >= 0xfe30 && character <= 0xfe4f: // CJK compatibility forms
		return true
	case character >= 0xff00 && character <= 0xffef: // halfwidth and fullwidth forms
		return true
	}
	return character >= 0x20000 && character <= 0x3ffff // supplementary ideographs
}
