package epub

import (
	"slices"
	"strconv"
	"strings"
)

// listMarker returns the marker of a list item, counting it in its list.
func listMarker(item *element) string {
	item.parent.counter++
	number := item.parent.counter
	switch item.style.listStyle {
	case "none":
		return ""
	case "decimal":
		return strconv.Itoa(number) + ". "
	case "lower-roman":
		return strings.ToLower(roman(number)) + ". "
	case "upper-roman":
		return roman(number) + ". "
	case "lower-alpha", "lower-latin":
		return strings.ToLower(alphabetic(number)) + ". "
	case "upper-alpha", "upper-latin":
		return alphabetic(number) + ". "
	case "circle":
		return "◦ "
	case "square":
		return "▪ "
	}
	return "• "
}

// romanNumerals are the values of Roman numerals, the largest first, with the subtractive
// pairs.
var romanNumerals = []struct {
	value   int
	numeral string
}{
	{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
	{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
}

// roman returns number in uppercase Roman numerals, in decimal if out of 1 to 3999.
func roman(number int) string {
	if number < 1 || number > 3999 {
		return strconv.Itoa(number)
	}
	var numerals strings.Builder
	for _, numeral := range romanNumerals {
		for ; number >= numeral.value; number -= numeral.value {
			numerals.WriteString(numeral.numeral)
		}
	}
	return numerals.String()
}

// alphabetic returns number in uppercase letters: A to Z, then AA, AB…; in decimal if not
// positive.
func alphabetic(number int) string {
	if number < 1 {
		return strconv.Itoa(number)
	}
	var letters []byte
	for ; number > 0; number = (number - 1) / 26 {
		letters = append(letters, byte('A'+(number-1)%26))
	}
	slices.Reverse(letters)
	return string(letters)
}
