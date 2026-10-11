package css

import "strings"

// Media is what media queries are evaluated against.
type Media struct {
	Type          string  // "screen", "print"…: the queries for it or for "all" match
	Width, Height float64 // in CSS pixels
}

// Matches reports whether media matches a media query list, such as the media attribute of a
// link element. An empty list matches.
func (media Media) Matches(queryList string) bool {
	return media.matchList(tokenize(queryList))
}

// matchList reports whether media matches one of the comma-separated queries of tokens.
// Media Queries 3 are followed: "[not|only] type [and (feature)]…" or "(feature) [and
// (feature)]…". Features other than width, height (min-, max-, device-) and orientation, such
// as Kindle's amzn-kf8, are false; a query that cannot be parsed is false.
func (media Media) matchList(tokens []token) bool {
	if len(trimWhitespace(tokens)) == 0 {
		return true
	}
	for _, query := range splitAt(tokens, commaToken) {
		if media.matchQuery(components(query)) {
			return true
		}
	}
	return false
}

// matchQuery reports whether media matches a query, given as its component values.
func (media Media) matchQuery(parts [][]token) bool {
	negated := false
	if len(parts) > 0 && (isIdent(parts[0][0], "not") || isIdent(parts[0][0], "only")) {
		negated = isIdent(parts[0][0], "not")
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return false
	}
	matched := true
	needsAnd := false // between a type and a feature, and between features
	if first := parts[0][0]; first.kind == identToken {
		mediaType := strings.ToLower(first.value)
		matched = mediaType == "all" || strings.EqualFold(mediaType, media.Type)
		parts = parts[1:]
		needsAnd = true
	}
	for len(parts) > 0 {
		if needsAnd {
			if !isIdent(parts[0][0], "and") {
				return false
			}
			parts = parts[1:]
		}
		if len(parts) == 0 || parts[0][0].kind != openParenToken {
			return false
		}
		matched = media.matchFeature(parts[0]) && matched
		parts = parts[1:]
		needsAnd = true
	}
	return matched != negated
}

// matchFeature reports whether media has a feature: "(name)" or "(name: value)", given with
// its parentheses.
func (media Media) matchFeature(feature []token) bool {
	if len(feature) < 2 || feature[len(feature)-1].kind != closeParenToken {
		return false
	}
	parts := components(feature[1 : len(feature)-1])
	if len(parts) == 0 || parts[0][0].kind != identToken {
		return false
	}
	name := strings.ToLower(parts[0][0].value)
	limit := ""
	for _, prefix := range []string{"min-", "max-"} {
		if trimmed, ok := strings.CutPrefix(name, prefix); ok {
			limit, name = prefix, trimmed
		}
	}
	name = strings.TrimPrefix(name, "device-")
	size := map[string]float64{"width": media.Width, "height": media.Height}
	if len(parts) == 1 {
		switch name {
		case "width", "height":
			return limit == "" && size[name] > 0
		case "orientation":
			return limit == ""
		}
		return false
	}
	if len(parts) != 3 || parts[1][0].kind != colonToken {
		return false
	}
	value := serialize(parts[2])
	if name == "orientation" {
		orientation := "landscape"
		if media.Height >= media.Width {
			orientation = "portrait"
		}
		return limit == "" && strings.EqualFold(value, orientation)
	}
	wanted, ok := ParseLength(value)
	pixels, isAbsolute := wanted.Pixels()
	if wanted.Unit == "em" || wanted.Unit == "rem" {
		pixels, isAbsolute = wanted.Value*16, true // as browsers do
	}
	if _, isSize := size[name]; !isSize || !ok || !isAbsolute {
		return false
	}
	switch limit {
	case "min-":
		return size[name] >= pixels
	case "max-":
		return size[name] <= pixels
	}
	return size[name] == pixels
}
