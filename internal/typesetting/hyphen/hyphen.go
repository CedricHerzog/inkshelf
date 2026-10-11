package hyphen

import (
	"strings"
	"sync"
	"unicode"
)

// Hyphenator hyphenates the words of one language. Its patterns are parsed on first use. A nil
// Hyphenator (a language without patterns) finds no points.
type Hyphenator struct {
	file     string // the pattern files' name in patterns/, without .pat.txt or .hyp.txt
	leftMin  int    // the fewest letters before a hyphen (TeX's \lefthyphenmin)
	rightMin int    // the fewest letters after it (\righthyphenmin)

	once       sync.Once
	patterns   trie
	exceptions map[string][]int // a word in lowercase → its points
}

// The languages with patterns, by lowercase language tag. The minimum lengths are hyph-utf8's,
// but French's, which are babel's.
var hyphenators = func() map[string]*Hyphenator {
	americanEnglish := &Hyphenator{file: "hyph-en-us", leftMin: 2, rightMin: 3}
	german := &Hyphenator{file: "hyph-de-1996", leftMin: 2, rightMin: 2}
	return map[string]*Hyphenator{
		"en":      americanEnglish,
		"en-us":   americanEnglish,
		"en-gb":   {file: "hyph-en-gb", leftMin: 2, rightMin: 3},
		"fr":      {file: "hyph-fr", leftMin: 2, rightMin: 3},
		"de":      german,
		"de-1996": german,
		"es":      {file: "hyph-es", leftMin: 2, rightMin: 2},
		"it":      {file: "hyph-it", leftMin: 2, rightMin: 2},
		"nl":      {file: "hyph-nl", leftMin: 2, rightMin: 2},
		"pt":      {file: "hyph-pt", leftMin: 2, rightMin: 3},
	}
}()

// For returns the hyphenator of a language tag ("en", "en-US", "fr-FR", "de-DE-1996"…): the
// patterns of the whole tag, else of the tag without its last subtags, one at a time. It returns
// nil when the language has none.
func For(language string) *Hyphenator {
	tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(language), "_", "-"))
	for tag != "" {
		if hyphenator, found := hyphenators[tag]; found {
			return hyphenator
		}
		last := strings.LastIndexByte(tag, '-')
		if last < 0 {
			break
		}
		tag = tag[:last]
	}
	return nil
}

// longWord is the length up to which Points keeps a word's runes on the stack rather than
// allocate them: nearly every word is shorter.
const longWord = 64

// Points returns the rune offsets in word where a hyphen may go: before the rune at each
// offset. Punctuation around the word (quotes, a comma) is skipped. An apostrophe splits the word
// and each part is hyphenated alone (l'ordinateur: l', then or-di-na-teur). A word with
// anything else than letters inside (digits, a hyphen, a soft hyphen) has no points.
func (hyphenator *Hyphenator) Points(word string) []int {
	if hyphenator == nil {
		return nil
	}
	hyphenator.once.Do(hyphenator.load)
	var buffer [longWord]rune
	runes := buffer[:0]
	for _, letter := range word {
		runes = append(runes, unicode.ToLower(letter))
	}
	first, last := 0, len(runes)
	for first < last && isPunctuation(runes[first]) {
		first++
	}
	for last > first && isPunctuation(runes[last-1]) {
		last--
	}
	var points []int
	partStart := first
	for index := first; index <= last; index++ {
		if index < last && isLetter(runes[index]) {
			continue
		}
		if index < last && !isApostrophe(runes[index]) {
			return nil
		}
		points = hyphenator.hyphenate(runes[partStart:index], partStart, points)
		partStart = index + 1
	}
	return points
}

// hyphenate appends to points the points of letters, a lowercase word without apostrophes, plus
// offset.
func (hyphenator *Hyphenator) hyphenate(letters []rune, offset int, points []int) []int {
	count := len(letters)
	if count < hyphenator.leftMin+hyphenator.rightMin {
		return points
	}
	if len(hyphenator.exceptions) > 0 {
		if exception, found := hyphenator.exceptions[string(letters)]; found {
			for _, point := range exception {
				if point >= hyphenator.leftMin && point <= count-hyphenator.rightMin {
					points = append(points, offset+point)
				}
			}
			return points
		}
	}
	// Patterns match the word between dots, which they use for its ends. levels[gap] is the
	// highest value they give the gap before dotted[gap]: odd allows a hyphen, even forbids it.
	var dottedBuffer [longWord + 2]rune
	dotted := append(append(append(dottedBuffer[:0], '.'), letters...), '.')
	var levelBuffer [longWord + 3]uint8
	levels := levelBuffer[:0]
	for range len(dotted) + 1 {
		levels = append(levels, 0)
	}
	for start := range dotted {
		hyphenator.patterns.match(dotted[start:], levels[start:])
	}
	// A hyphen after letters[:point] is in the gap before dotted[point+1].
	for point := hyphenator.leftMin; point <= count-hyphenator.rightMin; point++ {
		if levels[point+1]%2 == 1 {
			points = append(points, offset+point)
		}
	}
	return points
}

// isLetter reports whether a lowercase rune belongs in a word: a letter or a combining mark.
func isLetter(letter rune) bool {
	return unicode.IsLetter(letter) || unicode.Is(unicode.Mn, letter)
}

// isPunctuation reports whether a rune may surround a word: a quote, a bracket, a comma…
func isPunctuation(letter rune) bool { return unicode.IsPunct(letter) || unicode.IsSymbol(letter) }

func isApostrophe(letter rune) bool { return letter == '\'' || letter == '’' }
