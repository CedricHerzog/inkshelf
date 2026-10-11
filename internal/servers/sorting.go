package servers

import (
	"cmp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/CedricHerzog/inkshelf/internal/naturalorder"
)

// ligatures are the letters that folding spells out.
var ligatures = strings.NewReplacer("œ", "oe", "Œ", "Oe", "æ", "ae", "Æ", "Ae", "ß", "ss")

// Fold returns title without its accents and ligatures ("Œufs brouillés" → "Oeufs brouilles").
func Fold(title string) string {
	var folded strings.Builder
	for _, letter := range norm.NFD.String(ligatures.Replace(title)) {
		if !unicode.Is(unicode.Mn, letter) {
			folded.WriteRune(letter)
		}
	}
	return folded.String()
}

// punctuation spells the apostrophes and dashes of titles one way, and their no-break spaces
// as spaces, so that "Dune – Tome 2" sorts between "Dune - Tome 1" and "Dune - Tome 3".
var punctuation = strings.NewReplacer(
	"’", "'", "‘", "'", "ʼ", "'",
	"‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "−", "-",
	" ", " ", " ", " ",
)

// articles are the leading articles that sorting skips, French then English, in lower case and
// followed by the space or apostrophe that ends them. "a " is not "à ": an accented A is a
// preposition, kept.
var articles = []string{"le ", "la ", "les ", "l'", "un ", "une ", "des ", "the ", "a ", "an "}

// frenchWords start a title whose "A" is a French "à" written without its accent ("A la
// croisée des mondes"): such an A is kept.
var frenchWords = []string{"la ", "le ", "les ", "l'", "un ", "une ", "des ", "du "}

// SortKey returns what title sorts and is filed by in the A–Z jump (as Series.SortTitle):
// folded, its apostrophes and dashes spelled one way, without a leading article ("Le Trône de
// Fer" → "Trone de Fer", "L'Écume des jours" → "Ecume des jours", "The Hobbit" → "Hobbit").
func SortKey(title string) string {
	return Fold(withoutArticle(strings.TrimSpace(punctuation.Replace(title))))
}

// withoutArticle returns title without its leading article, or title if it has none or is
// only one. An article is matched in any case and must be followed by a space, or for L' by a
// vowel or an h.
func withoutArticle(title string) string {
	for _, article := range articles {
		if len(title) <= len(article) || !strings.EqualFold(title[:len(article)], article) {
			continue
		}
		rest := strings.TrimLeft(title[len(article):], " ")
		if rest == "" {
			return title
		}
		if article == "l'" && !startsWithVowelOrH(rest) {
			return title
		}
		if article == "a " && startsWithAny(rest, frenchWords) {
			return title
		}
		return rest
	}
	return title
}

// startsWithVowelOrH reports whether text starts with a vowel, accented or not, or an h:
// where French elides its article.
func startsWithVowelOrH(text string) bool {
	_, size := utf8.DecodeRuneInString(text)
	letter, _ := utf8.DecodeRuneInString(Fold(text[:size]))
	return strings.ContainsRune("aeiouyh", unicode.ToLower(letter))
}

// startsWithAny reports whether text starts with one of words, in any case.
func startsWithAny(text string, words []string) bool {
	for _, word := range words {
		if len(text) > len(word) && strings.EqualFold(text[:len(word)], word) {
			return true
		}
	}
	return false
}

// CompareTitles orders two titles alphabetically by their SortKey, ignoring case, their runs
// of digits compared as numbers ("Tome 2" before "Tome 10"); titles of the same key by their
// full text.
func CompareTitles(first, second string) int {
	return cmp.Or(
		naturalorder.Compare(strings.ToLower(SortKey(first)), strings.ToLower(SortKey(second))),
		strings.Compare(first, second),
	)
}
