package css

import "strings"

// Stylesheet is a parsed stylesheet.
type Stylesheet struct {
	// Imports are the URLs of its @import rules whose media match, as written: the caller
	// resolves them against the stylesheet's URL and adds them to a Cascade before it.
	Imports   []string
	FontFaces []FontFace
	rules     []rule
}

// FontFace is an @font-face rule.
type FontFace struct {
	Family  string
	Sources []FontSource // the url()s of src, in order; local() fonts are left out
	Weight  string       // as written, "" if not given
	Style   string
}

// FontSource is a font file of an @font-face rule.
type FontSource struct {
	URL    string // as written
	Format string // what format() says, "" if not given
}

// rule is a style rule: its selectors, and its declarations, shorthands expanded.
type rule struct {
	selectors    []selector
	declarations []declaration
}

// Parse parses the stylesheet css. The rules of @media rules that don't match media are
// dropped, as are rules with no selector the package follows.
func Parse(css string, media Media) *Stylesheet {
	sheet := &Stylesheet{}
	sheet.parseRules(tokenize(css), media)
	return sheet
}

// parseRules parses the rules of tokens: a stylesheet, or the block of an @media rule.
func (sheet *Stylesheet) parseRules(tokens []token, media Media) {
	for index := 0; index < len(tokens); {
		switch tokens[index].kind {
		case whitespaceToken, cdoToken, cdcToken, semicolonToken:
			index++
		case atKeywordToken:
			index = sheet.parseAtRule(tokens, index, media)
		default:
			prelude, block, hasBlock, end := splitRule(tokens, index, false)
			index = end
			if !hasBlock {
				continue // a rule cut by the end of the stylesheet
			}
			if selectors := parseSelectorList(prelude); len(selectors) > 0 {
				sheet.rules = append(sheet.rules, rule{selectors: selectors, declarations: parseDeclarations(block)})
			}
		}
	}
}

// parseAtRule parses the at-rule at index and returns the index after it. @import, @media and
// @font-face are followed; other at-rules (@page, @namespace, @supports…) are skipped.
func (sheet *Stylesheet) parseAtRule(tokens []token, index int, media Media) int {
	name := strings.ToLower(tokens[index].value)
	prelude, block, hasBlock, end := splitRule(tokens, index+1, true)
	switch {
	case name == "import" && !hasBlock:
		if address, queries, ok := importOf(prelude); ok && media.matchList(queries) {
			sheet.Imports = append(sheet.Imports, address)
		}
	case name == "media" && hasBlock:
		if media.matchList(prelude) {
			sheet.parseRules(block, media)
		}
	case name == "font-face" && hasBlock:
		sheet.FontFaces = append(sheet.FontFaces, fontFaceOf(parseDeclarations(block)))
	}
	return end
}

// splitRule returns the prelude of the rule starting at start, the contents of its {} block
// if it has one (not an at-rule ended by ";", nor a rule cut by the end), and the index after
// the rule.
func splitRule(tokens []token, start int, atRule bool) (prelude, block []token, hasBlock bool, end int) {
	for index := start; index < len(tokens); index = componentEnd(tokens, index) {
		switch tokens[index].kind {
		case openCurlyToken:
			end := componentEnd(tokens, index)
			blockEnd := end
			if end > index+1 && tokens[end-1].kind == closeCurlyToken {
				blockEnd = end - 1
			}
			return tokens[start:index], tokens[index+1 : blockEnd], true, end
		case semicolonToken:
			if atRule {
				return tokens[start:index], nil, false, index + 1
			}
		}
	}
	return tokens[start:], nil, false, len(tokens)
}

// importOf returns the URL and the media queries of an @import rule's prelude.
func importOf(prelude []token) (address string, queries []token, ok bool) {
	prelude = trimWhitespace(prelude)
	if len(prelude) == 0 {
		return "", nil, false
	}
	end := componentEnd(prelude, 0)
	address, ok = urlOf(prelude[:end])
	return address, prelude[end:], ok
}

// urlOf returns the URL of a component value: a url(…), or a string.
func urlOf(component []token) (string, bool) {
	switch first := component[0]; first.kind {
	case urlToken, stringToken:
		return first.value, true
	case functionToken:
		if strings.EqualFold(first.value, "url") {
			for _, argument := range component[1:] {
				if argument.kind == stringToken {
					return argument.value, true
				}
			}
		}
	}
	return "", false
}

// fontFaceOf returns the font face that the descriptors of an @font-face rule describe.
func fontFaceOf(descriptors []declaration) FontFace {
	var face FontFace
	for _, descriptor := range descriptors {
		switch descriptor.property {
		case "font-family":
			if families := Families(descriptor.value); len(families) > 0 {
				face.Family = families[0]
			}
		case "font-weight":
			face.Weight = descriptor.value
		case "font-style":
			face.Style = descriptor.value
		case "src":
			face.Sources = nil
			for _, source := range splitAt(tokenize(descriptor.value), commaToken) {
				if parsed, ok := fontSourceOf(components(source)); ok {
					face.Sources = append(face.Sources, parsed)
				}
			}
		}
	}
	return face
}

// fontSourceOf returns the font file of one source of src: a url() and maybe a format().
func fontSourceOf(parts [][]token) (FontSource, bool) {
	if len(parts) == 0 {
		return FontSource{}, false
	}
	address, ok := urlOf(parts[0])
	if !ok || parts[0][0].kind == stringToken {
		return FontSource{}, false
	}
	source := FontSource{URL: address}
	for _, part := range parts[1:] {
		if part[0].kind != functionToken || !strings.EqualFold(part[0].value, "format") {
			continue
		}
		for _, argument := range part[1:] {
			if argument.kind == stringToken || argument.kind == identToken {
				source.Format = argument.value
				break
			}
		}
	}
	return source, true
}
