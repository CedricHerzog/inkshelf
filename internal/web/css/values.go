package css

import (
	"strconv"
	"strings"
)

// Length is a number and its unit: lowercase, "%" for a percentage, "" for a bare number.
type Length struct {
	Value float64
	Unit  string
}

// lengthUnits are the units ParseLength accepts, with the size of each in CSS pixels when it
// has one that does not depend on the element or the screen (0 otherwise).
var lengthUnits = map[string]float64{
	"px": 1, "pt": 96.0 / 72, "pc": 16, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4, "q": 96 / 101.6,
	"em": 0, "rem": 0, "ex": 0, "ch": 0, "%": 0, "vw": 0, "vh": 0, "vmin": 0, "vmax": 0, "": 0,
}

// ParseLength parses a length, a percentage or a number, such as "1.5em", "-10%" or "0".
func ParseLength(value string) (Length, bool) {
	tokens := trimWhitespace(tokenize(value))
	if len(tokens) != 1 {
		return Length{}, false
	}
	parsed := Length{Unit: strings.ToLower(tokens[0].unit)}
	switch tokens[0].kind {
	case percentageToken:
		parsed.Unit = "%"
	case numberToken, dimensionToken:
	default:
		return Length{}, false
	}
	if _, known := lengthUnits[parsed.Unit]; !known {
		return Length{}, false
	}
	number, err := strconv.ParseFloat(tokens[0].value, 64)
	parsed.Value = number
	return parsed, err == nil
}

// Pixels returns the length in CSS pixels (96 to the inch), and false when it is relative to
// something (em, %…).
func (length Length) Pixels() (float64, bool) {
	if length.Unit == "" && length.Value == 0 {
		return 0, true
	}
	pixels := lengthUnits[length.Unit]
	return length.Value * pixels, pixels > 0
}

// Families returns the font families of a font-family value, unquoted: "'Foo Bar', serif"
// gives "Foo Bar" and "serif".
func Families(value string) []string {
	var families []string
	for _, part := range splitAt(tokenize(value), commaToken) {
		var words []string
		for _, current := range part {
			if current.kind == identToken || current.kind == stringToken {
				words = append(words, current.value)
			}
		}
		if len(words) > 0 {
			families = append(families, strings.Join(words, " "))
		}
	}
	return families
}

// URL returns the first url(…) of a value, such as a background's: "url(a.png) no-repeat"
// gives "a.png". It returns false if value has none.
func URL(value string) (string, bool) {
	tokens := tokenize(value)
	for index := 0; index < len(tokens); {
		end := componentEnd(tokens, index)
		if current := tokens[index]; current.kind == urlToken || current.kind == functionToken && strings.EqualFold(current.value, "url") {
			return urlOf(tokens[index:end])
		}
		index = end
	}
	return "", false
}
