package hyphen

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// hyphenated returns word with a "-" at each of its points.
func hyphenated(hyphenator *Hyphenator, word string) string {
	points := hyphenator.Points(word)
	var result strings.Builder
	offset := 0
	for _, letter := range word {
		if slices.Contains(points, offset) {
			result.WriteByte('-')
		}
		result.WriteRune(letter)
		offset++
	}
	return result.String()
}

// The expected words are TeX's output with the same patterns (checked with a plain
// implementation of Liang's algorithm), but where an exception applies.
func TestPoints(t *testing.T) {
	tests := []struct {
		language string
		words    []string
	}{
		{"en", []string{"hy-phen-ation", "com-puter", "al-go-rithm", "type-set-ting", "Hy-phen-ation"}},
		{"en-GB", []string{"hy-phen-a-tion", "col-our"}},
		{"fr", []string{"or-di-na-teur", "hy-phé-na-tion", "an-ti-cons-ti-tu-tion-nel-le-ment"}},
		{"de", []string{"Sil-ben-tren-nung", "Do-nau-dampf-schiff-fahrt", "Recht-schrei-bung"}},
		{"es", []string{"or-de-na-dor", "bi-blio-te-ca"}},
		{"it", []string{"cal-co-la-to-re", "bi-blio-te-ca"}},
		{"nl", []string{"woor-den-boek", "com-pu-ter"}},
		{"pt", []string{"com-pu-ta-dor", "bi-bli-o-teca"}},
	}
	for _, test := range tests {
		hyphenator := For(test.language)
		for _, want := range test.words {
			word := strings.ReplaceAll(want, "-", "")
			if got := hyphenated(hyphenator, word); got != want {
				t.Errorf("%s %q: got %q, want %q", test.language, word, got, want)
			}
		}
	}
}

func TestExceptions(t *testing.T) {
	tests := []struct {
		language, want string
	}{
		{"en", "as-so-ciate"}, // the patterns say as-so-ci-ate
		{"en", "present"},     // the patterns say pre-sent
		{"en", "ta-ble"},
		{"en-gb", "uni-ver-sity"}, // the patterns say uni-versity
		{"nl", "aan-dachts-trek-ker"},
	}
	for _, test := range tests {
		word := strings.ReplaceAll(test.want, "-", "")
		if got := hyphenated(For(test.language), word); got != test.want {
			t.Errorf("%s %q: got %q, want %q", test.language, word, got, test.want)
		}
	}
}

func TestMinimumLengths(t *testing.T) {
	for _, word := range []string{"hyphenation", "algorithm", "ordinateur", "Silbentrennung"} {
		for _, language := range []string{"en", "fr", "de"} {
			hyphenator := For(language)
			for _, point := range hyphenator.Points(word) {
				if point < hyphenator.leftMin || point > utf8.RuneCountInString(word)-hyphenator.rightMin {
					t.Errorf("%s %q: point %d breaks the minimum lengths", language, word, point)
				}
			}
		}
	}
	for _, word := range []string{"a", "an", "the", "into"} {
		if points := For("en").Points(word); points != nil {
			t.Errorf("%q: got points %v in a short word", word, points)
		}
	}
}

func TestWordShapes(t *testing.T) {
	tests := []struct {
		language, word string
		want           []int
	}{
		{"fr", "l'ordinateur", []int{4, 6, 8}}, // l'or-di-na-teur
		{"fr", "l’ordinateur", []int{4, 6, 8}},
		{"en", "computer's", []int{3}},        // com-puter's
		{"en", "“hyphenation,”", []int{3, 7}}, // the quotes and the comma are skipped
		{"en", "hyphenation2", nil},           // a digit
		{"en", "hyphen-ation", nil},           // a hyphen already
		{"en", "hyphen\u00adation", nil},      // a soft hyphen already
		{"en", "", nil},
		{"en", "...", nil},
	}
	for _, test := range tests {
		if got := For(test.language).Points(test.word); !slices.Equal(got, test.want) {
			t.Errorf("%s %q: got %v, want %v", test.language, test.word, got, test.want)
		}
	}
}

func TestFor(t *testing.T) {
	tests := []struct {
		tag, file string
	}{
		{"en", "hyph-en-us"},
		{"en-US", "hyph-en-us"},
		{"en-AU", "hyph-en-us"},
		{"en_GB", "hyph-en-gb"},
		{"fr-FR", "hyph-fr"},
		{"de-DE-1996", "hyph-de-1996"},
		{"DE", "hyph-de-1996"},
		{"pt-BR", "hyph-pt"},
		{"ja", ""},
		{"", ""},
		{"und", ""},
	}
	for _, test := range tests {
		hyphenator := For(test.tag)
		file := ""
		if hyphenator != nil {
			file = hyphenator.file
		}
		if file != test.file {
			t.Errorf("For(%q): got %q, want %q", test.tag, file, test.file)
		}
	}
	if points := For("ja").Points("hyphenation"); points != nil {
		t.Errorf("no patterns: got points %v", points)
	}
}

// TestPatternFiles checks that every language's files parse.
func TestPatternFiles(t *testing.T) {
	for tag, hyphenator := range hyphenators {
		hyphenator.once.Do(hyphenator.load)
		if len(hyphenator.patterns.nodes) < 100 {
			t.Errorf("%s: %d trie nodes", tag, len(hyphenator.patterns.nodes))
		}
	}
}

// BenchmarkPoints hyphenates the words of a paragraph, in English and in German, whose patterns
// are the most.
func BenchmarkPoints(b *testing.B) {
	paragraphs := map[string]string{
		"en": `It was the best of times, it was the worst of times, it was the age of wisdom, it
			was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity,
			it was the season of Light, it was the season of Darkness, it was the spring of hope, it
			was the winter of despair, we had everything before us, we had nothing before us.`,
		"de": `Als Gregor Samsa eines Morgens aus unruhigen Träumen erwachte, fand er sich in
			seinem Bett zu einem ungeheueren Ungeziefer verwandelt. Er lag auf seinem panzerartig
			harten Rücken und sah, wenn er den Kopf ein wenig hob, seinen gewölbten, braunen, von
			bogenförmigen Versteifungen geteilten Bauch.`,
	}
	for language, paragraph := range paragraphs {
		b.Run(language, func(b *testing.B) {
			words := strings.Fields(paragraph)
			hyphenator := For(language)
			hyphenator.Points("warm")
			for b.Loop() {
				for _, word := range words {
					hyphenator.Points(word)
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(words)), "ns/word")
		})
	}
}

// BenchmarkLoad parses the German patterns, the largest file.
func BenchmarkLoad(b *testing.B) {
	for b.Loop() {
		hyphenator := Hyphenator{file: "hyph-de-1996"}
		hyphenator.load()
	}
}
