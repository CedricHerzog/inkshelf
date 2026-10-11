package css

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// declaration is a property and its value, as written with whitespace collapsed.
type declaration struct {
	property  string // lowercase
	value     string
	important bool
}

// parseDeclarations parses the declarations of a rule's block or a style attribute, with
// shorthands expanded. Declarations that cannot be parsed, and nested rules, are skipped.
func parseDeclarations(tokens []token) []declaration {
	var declarations []declaration
	for _, part := range splitAt(tokens, semicolonToken) {
		part = trimWhitespace(part)
		if len(part) < 2 || part[0].kind != identToken {
			continue
		}
		property := strings.ToLower(part[0].value)
		value := trimWhitespace(part[1:])
		if len(value) == 0 || value[0].kind != colonToken {
			continue
		}
		value, important := cutImportant(trimWhitespace(value[1:]))
		if len(value) > 0 && !slices.ContainsFunc(value, isBad) {
			declarations = append(declarations, expand(property, value, important)...)
		}
	}
	return declarations
}

// ParseDeclarations returns the values of a style attribute's declarations, by property
// (lowercase), as Compute gives them: shorthands expanded, the last declaration of a property
// winning, an !important one above the others.
func ParseDeclarations(style string) map[string]string {
	values := map[string]string{}
	declarations := parseDeclarations(tokenize(style))
	for _, important := range []bool{false, true} {
		for _, declared := range declarations {
			if declared.important == important {
				values[declared.property] = declared.value
			}
		}
	}
	return values
}

// isBad reports whether current is a bad string or URL, which makes its declaration invalid.
func isBad(current token) bool {
	return current.kind == badStringToken || current.kind == badURLToken
}

// cutImportant returns value without its ending "!important", and whether it had one.
func cutImportant(value []token) ([]token, bool) {
	if length := len(value); length == 0 || !isIdent(value[length-1], "important") {
		return value, false
	}
	rest := trimWhitespace(value[:len(value)-1])
	if length := len(rest); length > 0 && isDelim(rest[length-1], "!") {
		return trimWhitespace(rest[:length-1]), true
	}
	return value, false
}

// boxSides are the sides of margin, padding and border, in the order of their shorthands.
var boxSides = [4]string{"top", "right", "bottom", "left"}

// expand returns the longhands of a shorthand property, or the declaration itself. A
// shorthand whose value cannot be parsed gives nothing.
func expand(property string, value []token, important bool) []declaration {
	var longhands map[string]string
	switch property {
	case "margin", "padding":
		longhands = boxLonghands(property+"-%s", value)
	case "border-width", "border-style", "border-color":
		longhands = boxLonghands("border-%s-"+strings.TrimPrefix(property, "border-"), value)
	case "border":
		longhands = borderLonghands(boxSides[:], value)
	case "border-top", "border-right", "border-bottom", "border-left":
		longhands = borderLonghands([]string{strings.TrimPrefix(property, "border-")}, value)
	case "font":
		longhands = fontLonghands(value)
	case "list-style":
		longhands = listStyleLonghands(value)
	case "text-decoration":
		longhands = textDecorationLonghands(value)
	default:
		return []declaration{{property: property, value: serialize(value), important: important}}
	}
	declarations := make([]declaration, 0, len(longhands))
	for longhand, longhandValue := range longhands {
		declarations = append(declarations, declaration{property: longhand, value: longhandValue, important: important})
	}
	return declarations
}

// boxLonghands expands the one to four values of a box shorthand (top, right, bottom, left,
// as for margin) into the properties that format gives with each side.
func boxLonghands(format string, value []token) map[string]string {
	parts := components(value)
	if len(parts) == 0 || len(parts) > 4 {
		return nil
	}
	// The sides that the values give, by count of values: top, right, bottom, left.
	sources := [5][4]int{1: {0, 0, 0, 0}, 2: {0, 1, 0, 1}, 3: {0, 1, 2, 1}, 4: {0, 1, 2, 3}}[len(parts)]
	longhands := map[string]string{}
	for index, side := range boxSides {
		longhands[fmt.Sprintf(format, side)] = serialize(parts[sources[index]])
	}
	return longhands
}

// borderStyles are the values of border-style.
var borderStyles = map[string]bool{
	"none": true, "hidden": true, "dotted": true, "dashed": true, "solid": true, "double": true,
	"groove": true, "ridge": true, "inset": true, "outset": true,
}

// borderLonghands expands a border shorthand (width, style and color, in any order) into the
// width, style and color of sides; what is not given is reset.
func borderLonghands(sides []string, value []token) map[string]string {
	width, style, color := "medium", "none", "currentcolor"
	var given [3]bool
	for _, part := range components(value) {
		text := serialize(part)
		keyword := strings.ToLower(text)
		field := 2 // color
		if borderStyles[keyword] {
			field = 1
		} else if _, isLength := ParseLength(text); isLength || keyword == "thin" || keyword == "medium" || keyword == "thick" {
			field = 0
		}
		if given[field] {
			return nil
		}
		given[field] = true
		*[3]*string{&width, &style, &color}[field] = text
	}
	longhands := map[string]string{}
	for _, side := range sides {
		longhands["border-"+side+"-width"] = width
		longhands["border-"+side+"-style"] = style
		longhands["border-"+side+"-color"] = color
	}
	return longhands
}

// fontSizeKeywords are the keywords of font-size.
var fontSizeKeywords = map[string]bool{
	"xx-small": true, "x-small": true, "small": true, "medium": true, "large": true, "x-large": true,
	"xx-large": true, "xxx-large": true, "smaller": true, "larger": true,
}

// fontLonghands expands font: [style || variant || weight || stretch] size [/ line-height]
// family. What is not given is reset to normal. System fonts (font: caption) are not followed.
func fontLonghands(value []token) map[string]string {
	longhands := map[string]string{
		"font-style": "normal", "font-variant": "normal", "font-weight": "normal", "line-height": "normal",
	}
	index := 0
	next := func() token { // the next token that is not whitespace
		for index < len(value) && value[index].kind == whitespaceToken {
			index++
		}
		if index == len(value) {
			return token{kind: whitespaceToken}
		}
		index++
		return value[index-1]
	}
	current := next()
	for setFontKeyword(longhands, current) {
		current = next()
	}
	isSize := current.kind == dimensionToken || current.kind == percentageToken ||
		current.kind == numberToken && current.value == "0" || current.kind == identToken && fontSizeKeywords[strings.ToLower(current.value)]
	if !isSize {
		return nil
	}
	longhands["font-size"] = current.raw
	family := index
	if slash := next(); isDelim(slash, "/") {
		lineHeight := next()
		if lineHeight.kind != numberToken && lineHeight.kind != dimensionToken && lineHeight.kind != percentageToken && !isIdent(lineHeight, "normal") {
			return nil
		}
		longhands["line-height"] = lineHeight.raw
		family = index
	}
	longhands["font-family"] = serialize(value[family:])
	if longhands["font-family"] == "" {
		return nil
	}
	return longhands
}

// setFontKeyword sets the longhand of font that current gives, if a style, variant, weight
// or stretch (which is not kept), and reports whether it did.
func setFontKeyword(longhands map[string]string, current token) bool {
	keyword := strings.ToLower(current.value)
	switch {
	case current.kind == numberToken:
		if isFontWeight(current.value) {
			longhands["font-weight"] = current.value
			return true
		}
		return false
	case current.kind != identToken:
		return false
	case keyword == "italic" || keyword == "oblique":
		longhands["font-style"] = current.value
	case keyword == "small-caps":
		longhands["font-variant"] = current.value
	case keyword == "bold" || keyword == "bolder" || keyword == "lighter":
		longhands["font-weight"] = current.value
	case keyword == "normal" || strings.HasSuffix(keyword, "condensed") || strings.HasSuffix(keyword, "expanded"):
	default:
		return false
	}
	return true
}

// isFontWeight reports whether number is a font weight: 1 to 1000.
func isFontWeight(number string) bool {
	weight, err := strconv.ParseFloat(number, 64)
	return err == nil && weight >= 1 && weight <= 1000
}

// listStyleLonghands expands list-style: type, position and image in any order.
func listStyleLonghands(value []token) map[string]string {
	longhands := map[string]string{}
	nones := 0
	for _, part := range components(value) {
		text := serialize(part)
		switch keyword := strings.ToLower(text); {
		case keyword == "inside" || keyword == "outside":
			longhands["list-style-position"] = text
		case keyword == "none":
			nones++ // the type's or the image's
		case part[0].kind == urlToken || part[0].kind == functionToken:
			longhands["list-style-image"] = text
		default:
			longhands["list-style-type"] = text
		}
	}
	defaults := map[string]string{"list-style-position": "outside", "list-style-image": "none", "list-style-type": "disc"}
	if nones > 0 {
		defaults["list-style-type"] = "none"
	}
	for longhand, initial := range defaults {
		if _, given := longhands[longhand]; !given {
			longhands[longhand] = initial
		}
	}
	return longhands
}

// textDecorationLines are the values of text-decoration-line.
var textDecorationLines = map[string]bool{"none": true, "underline": true, "overline": true, "line-through": true, "blink": true}

// textDecorationStyles are the values of text-decoration-style.
var textDecorationStyles = map[string]bool{"solid": true, "double": true, "dotted": true, "dashed": true, "wavy": true}

// textDecorationLonghands expands text-decoration: lines, style and color in any order.
func textDecorationLonghands(value []token) map[string]string {
	var lines []string
	longhands := map[string]string{"text-decoration-style": "solid", "text-decoration-color": "currentcolor"}
	for _, part := range components(value) {
		text := serialize(part)
		switch keyword := strings.ToLower(text); {
		case textDecorationLines[keyword]:
			lines = append(lines, text)
		case textDecorationStyles[keyword]:
			longhands["text-decoration-style"] = text
		default:
			longhands["text-decoration-color"] = text
		}
	}
	longhands["text-decoration-line"] = "none"
	if len(lines) > 0 {
		longhands["text-decoration-line"] = strings.Join(lines, " ")
	}
	return longhands
}
