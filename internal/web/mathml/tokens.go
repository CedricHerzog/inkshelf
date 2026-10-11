package mathml

import (
	"cmp"
	"image"
	"strings"
	"unicode"
)

// token lays out an <mi>, <mn>, <mtext> or <ms>.
func (layout *layouter) token(node *Node, current style) *box {
	text := node.Text
	if node.Name == "ms" {
		text = cmp.Or(node.Attributes["lquote"], `"`) + text + cmp.Or(node.Attributes["rquote"], `"`)
	}
	variant, text := variantOf(node, current, text)
	return layout.text(text, variant, current.size, variant&Italic != 0 && runeCount(text) == 1)
}

// text lays out text in variant at size; italic marks a letter in italics.
func (layout *layouter) text(text string, variant Variant, size int, italic bool) *box {
	if text == "" {
		return &box{}
	}
	width, ink := layout.fonts.Measure(text, variant, size)
	// Room for the ink out of the advance: an italic letter's overhang.
	left := max(-ink.Min.X, 0)
	return &box{width: left + max(width, ink.Max.X), ascent: -ink.Min.Y, descent: ink.Max.Y, italic: italic,
		items: []item{{kind: textItem, at: image.Pt(left, 0), text: text, variant: variant, size: size}}}
}

// variantOf returns how a token's text is drawn, and its text: with mathvariant's letters, and
// the letters of Unicode's mathematical alphabets, which fonts lack, as plain ones in a variant.
func variantOf(node *Node, current style, text string) (Variant, string) {
	mathVariant := cmp.Or(node.Attributes["mathvariant"], current.variant)
	if mathVariant == "" && node.Name == "mi" && runeCount(text) == 1 && isItalicLetter(firstRune(text)) {
		mathVariant = "italic"
	}
	var variant Variant
	if strings.Contains(mathVariant, "bold") || node.Attributes["fontweight"] == "bold" {
		variant |= Bold
	}
	if strings.Contains(mathVariant, "italic") || node.Attributes["fontstyle"] == "italic" {
		variant |= Italic
	}
	if node.Attributes["fontstyle"] == "normal" {
		variant &^= Italic
	}
	letters := letterlike[strings.TrimPrefix(mathVariant, "bold-")]
	var plain strings.Builder
	for _, character := range text {
		if mapped, ok := letters[character]; ok {
			plain.WriteRune(mapped)
			continue
		}
		mapped, alphabet := plainLetter(character)
		variant |= alphabet
		plain.WriteRune(mapped)
	}
	return variant, plain.String()
}

// isItalicLetter reports whether a lone letter in an <mi> is in italics: a Latin or a Greek
// one, as in MathML's default.
func isItalicLetter(character rune) bool {
	return 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z' ||
		unicode.Is(unicode.Greek, character) && unicode.IsLetter(character) || character == 'ı' || character == 'ȷ'
}

// letterlike are the letters of mathvariants that fonts have, in Letterlike Symbols, by
// mathvariant: the others are drawn as plain letters.
var letterlike = map[string]map[rune]rune{
	"double-struck": {'C': 'ℂ', 'H': 'ℍ', 'N': 'ℕ', 'P': 'ℙ', 'Q': 'ℚ', 'R': 'ℝ', 'Z': 'ℤ'},
	"script":        {'B': 'ℬ', 'E': 'ℰ', 'F': 'ℱ', 'H': 'ℋ', 'I': 'ℐ', 'L': 'ℒ', 'M': 'ℳ', 'R': 'ℛ', 'e': 'ℯ', 'g': 'ℊ', 'o': 'ℴ'},
	"fraktur":       {'C': 'ℭ', 'H': 'ℌ', 'I': 'ℑ', 'R': 'ℜ', 'Z': 'ℨ'},
}

// The Mathematical Alphanumeric Symbols: 13 alphabets of 52 letters from mathLetters, then 5
// sets of 10 digits from mathDigits.
const (
	mathLetters, mathLettersEnd = 0x1D400, 0x1D6A4
	mathDigits, mathDigitsEnd   = 0x1D7CE, 0x1D800
)

// alphabetVariants are the variants of the 13 alphabets: bold, italic, bold italic, script,
// bold script, fraktur, double-struck, bold fraktur, sans-serif and its bold, italic and bold
// italic, monospace.
var alphabetVariants = []Variant{Bold, Italic, Bold | Italic, 0, Bold, 0, 0, Bold, 0, Bold, Italic, Bold | Italic, 0}

// plainLetter returns the plain letter or digit of a mathematical one, and its variant; another
// character as it is.
func plainLetter(character rune) (rune, Variant) {
	switch {
	case character == 'ℎ': // the italic h, in Letterlike Symbols
		return 'h', Italic
	case mathLetters <= character && character < mathLettersEnd:
		offset := int(character - mathLetters)
		letter := offset % 52
		base := rune('A' + letter)
		if letter >= 26 {
			base = rune('a' + letter - 26)
		}
		return base, alphabetVariants[offset/52]
	case mathDigits <= character && character < mathDigitsEnd:
		offset := int(character - mathDigits)
		variant := Variant(0)
		if offset/10 == 0 || offset/10 == 3 {
			variant = Bold
		}
		return rune('0' + offset%10), variant
	}
	return character, 0
}
