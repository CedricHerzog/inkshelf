package css

import (
	"slices"
	"strconv"
	"strings"
)

// selector is a complex selector, such as "div.note > p:first-child::first-letter".
type selector struct {
	compounds     []compound // right to left: compounds[0] is the element's
	pseudoElement string     // "first-letter", "first-line", "before", "after" or ""
	specificity   int
}

// combinator relates an element to another, to its left in a selector.
type combinator int

const (
	descendant        combinator = iota // "a b"
	child                               // "a > b"
	nextSibling                         // "a + b"
	subsequentSibling                   // "a ~ b"
)

// compound is the simple selectors that one element must match, such as "p.note[lang]".
type compound struct {
	name          string // the type, lowercase, "" for any
	id            string
	classes       []string
	attributes    []attributeTest
	pseudoClasses []pseudoClass
	// combinator relates the element to the one the next compound (to its left) matches.
	combinator combinator
}

// attributeTest is an attribute selector, such as [lang|="en"].
type attributeTest struct {
	name       string // the local name, lowercase
	operator   string // "" (the attribute is there), "=", "~=", "|=", "^=", "$=" or "*="
	value      string
	ignoreCase bool // the "i" flag
}

// pseudoClassKind is a pseudo-class the package follows.
type pseudoClassKind int

const (
	firstChild pseudoClassKind = iota
	firstOfType
	nthChild // :nth-child(step n + offset)
	root
	link // an a element with an href
	not
)

// pseudoClass is a pseudo-class of a compound.
type pseudoClass struct {
	kind         pseudoClassKind
	step, offset int        // of nthChild
	negated      []compound // the arguments of :not, none of which may match
}

// Specificities are counted in one int: ids, then classes (attributes and pseudo-classes
// too), then types (and pseudo-elements), each count below 1024.
const (
	idSpecificity    = 1 << 20
	classSpecificity = 1 << 10
	typeSpecificity  = 1
)

// pseudoClassKinds are the pseudo-classes without arguments the package follows.
var pseudoClassKinds = map[string]pseudoClassKind{"first-child": firstChild, "first-of-type": firstOfType, "root": root, "link": link}

// pseudoElements are the pseudo-elements the package follows; others drop their selector.
var pseudoElements = map[string]bool{"first-letter": true, "first-line": true, "before": true, "after": true}

// parseSelectorList returns the selectors of a comma-separated list, those that cannot be
// parsed or followed left out. Pseudo-classes that need what follows an element (:last-child,
// :only-child, :empty, :last-of-type…) or user actions (:hover…) are not followed: a builder
// reading a stream does not know them when it computes an element's style.
func parseSelectorList(tokens []token) []selector {
	var selectors []selector
	for _, part := range splitAt(tokens, commaToken) {
		if parsed, ok := parseSelector(trimWhitespace(part)); ok {
			selectors = append(selectors, parsed)
		}
	}
	return selectors
}

// parseSelector parses one complex selector.
func parseSelector(tokens []token) (selector, bool) {
	parser := selectorParser{tokens: tokens}
	var parsed selector
	nextCombinator := descendant
	for {
		current, pseudoElement, ok := parser.compound()
		if !ok {
			return selector{}, false
		}
		current.combinator = nextCombinator
		parsed.compounds = append([]compound{current}, parsed.compounds...)
		parsed.specificity += current.specificity()
		if pseudoElement != "" {
			parsed.pseudoElement = pseudoElement
			parsed.specificity += typeSpecificity
			return parsed, parser.done() // nothing may follow a pseudo-element
		}
		if parser.done() {
			return parsed, true
		}
		if nextCombinator, ok = parser.combinator(); !ok || parser.done() {
			return selector{}, false
		}
	}
}

// specificity returns the specificity of a compound.
func (part compound) specificity() int {
	specificity := (len(part.classes) + len(part.attributes)) * classSpecificity
	if part.name != "" {
		specificity += typeSpecificity
	}
	if part.id != "" {
		specificity += idSpecificity
	}
	for _, pseudo := range part.pseudoClasses {
		if pseudo.kind != not {
			specificity += classSpecificity
			continue
		}
		highest := 0 // :not counts as its most specific argument
		for _, argument := range pseudo.negated {
			highest = max(highest, argument.specificity())
		}
		specificity += highest
	}
	return specificity
}

// selectorParser reads a selector's tokens.
type selectorParser struct {
	tokens   []token
	position int
}

func (parser *selectorParser) done() bool { return parser.position >= len(parser.tokens) }

// peek returns the token at position, a whitespace past the end.
func (parser *selectorParser) peek() token {
	if parser.done() {
		return token{kind: whitespaceToken}
	}
	return parser.tokens[parser.position]
}

// combinator reads a combinator and the whitespace around it.
func (parser *selectorParser) combinator() (combinator, bool) {
	found := descendant
	sawWhitespace := false
	for !parser.done() {
		current := parser.peek()
		switch {
		case current.kind == whitespaceToken:
			sawWhitespace = true
		case found == descendant && current.kind == delimToken && slices.Contains([]string{">", "+", "~"}, current.value):
			found = map[string]combinator{">": child, "+": nextSibling, "~": subsequentSibling}[current.value]
		default:
			return found, sawWhitespace || found != descendant
		}
		parser.position++
	}
	return found, sawWhitespace || found != descendant
}

// compound reads a compound selector and the pseudo-element that may end it.
func (parser *selectorParser) compound() (parsed compound, pseudoElement string, ok bool) {
	name, hasType, ok := parser.typeSelector()
	if !ok {
		return compound{}, "", false
	}
	parsed.name = name
	read := hasType
	for !parser.done() && pseudoElement == "" {
		current := parser.peek()
		switch {
		case current.kind == hashToken:
			parsed.id = current.value
			parser.position++
		case isDelim(current, "."):
			parser.position++
			class := parser.peek()
			if class.kind != identToken {
				return compound{}, "", false
			}
			parsed.classes = append(parsed.classes, class.value)
			parser.position++
		case current.kind == openSquareToken:
			end := componentEnd(parser.tokens, parser.position)
			if end-1 == parser.position || parser.tokens[end-1].kind != closeSquareToken {
				return compound{}, "", false // not closed
			}
			test, ok := parseAttributeTest(parser.tokens[parser.position+1 : end-1])
			if !ok {
				return compound{}, "", false
			}
			parsed.attributes = append(parsed.attributes, test)
			parser.position = end
		case current.kind == colonToken:
			pseudo, element, ok := parser.pseudo()
			if !ok {
				return compound{}, "", false
			}
			if element != "" {
				pseudoElement = element
			} else {
				parsed.pseudoClasses = append(parsed.pseudoClasses, pseudo)
			}
		default:
			return parsed, "", read
		}
		read = true
	}
	return parsed, pseudoElement, read
}

// typeSelector reads the type or universal selector that may start a compound, with its
// namespace prefix ("ns|name", "*|name", "|name"), which is ignored.
func (parser *selectorParser) typeSelector() (name string, found, ok bool) {
	if isDelim(parser.peek(), "|") {
		parser.position++
		name, found = parser.nameOrStar()
		return name, found, found
	}
	name, found = parser.nameOrStar()
	if found && isDelim(parser.peek(), "|") {
		parser.position++
		name, found = parser.nameOrStar()
		return name, found, found
	}
	return name, found, true
}

// nameOrStar reads an element name, lowercase, or "*" as "".
func (parser *selectorParser) nameOrStar() (string, bool) {
	switch current := parser.peek(); {
	case current.kind == identToken:
		parser.position++
		return strings.ToLower(current.value), true
	case isDelim(current, "*"):
		parser.position++
		return "", true
	}
	return "", false
}

// pseudo reads a pseudo-class, or the name of a pseudo-element.
func (parser *selectorParser) pseudo() (pseudoClass, string, bool) {
	parser.position++ // the colon
	double := parser.peek().kind == colonToken
	if double {
		parser.position++
	}
	current := parser.peek()
	parser.position++
	name := strings.ToLower(current.value)
	if current.kind == identToken && pseudoElements[name] {
		return pseudoClass{}, name, true // with one colon too, as CSS 2 wrote them
	}
	if double {
		return pseudoClass{}, "", false
	}
	if current.kind == identToken {
		kind, ok := pseudoClassKinds[name]
		return pseudoClass{kind: kind}, "", ok
	}
	if current.kind != functionToken {
		return pseudoClass{}, "", false
	}
	end := componentEnd(parser.tokens, parser.position-1)
	if end-1 < parser.position || parser.tokens[end-1].kind != closeParenToken {
		return pseudoClass{}, "", false // not closed
	}
	arguments := parser.tokens[parser.position : end-1]
	parser.position = end
	switch name {
	case "nth-child":
		step, offset, ok := parseNth(serialize(arguments))
		return pseudoClass{kind: nthChild, step: step, offset: offset}, "", ok
	case "not":
		negated, ok := parseCompoundList(arguments)
		return pseudoClass{kind: not, negated: negated}, "", ok
	}
	return pseudoClass{}, "", false
}

// parseCompoundList parses the arguments of :not: compound selectors without pseudo-elements.
func parseCompoundList(tokens []token) ([]compound, bool) {
	var compounds []compound
	for _, part := range splitAt(tokens, commaToken) {
		parser := selectorParser{tokens: trimWhitespace(part)}
		parsed, pseudoElement, ok := parser.compound()
		if !ok || pseudoElement != "" || !parser.done() {
			return nil, false
		}
		compounds = append(compounds, parsed)
	}
	return compounds, true
}

// parseNth parses the argument of :nth-child: "odd", "even", "b", or "an+b" (a and b
// optional).
func parseNth(argument string) (step, offset int, ok bool) {
	argument = strings.ToLower(strings.ReplaceAll(argument, " ", ""))
	switch argument {
	case "odd":
		return 2, 1, true
	case "even":
		return 2, 0, true
	}
	before, after, hasN := strings.Cut(argument, "n")
	if !hasN {
		offset, err := strconv.Atoi(argument)
		return 0, offset, err == nil
	}
	switch before {
	case "", "+":
		step = 1
	case "-":
		step = -1
	default:
		var err error
		if step, err = strconv.Atoi(before); err != nil {
			return 0, 0, false
		}
	}
	if after == "" {
		return step, 0, true
	}
	if after[0] != '+' && after[0] != '-' {
		return 0, 0, false
	}
	offset, err := strconv.Atoi(after)
	return step, offset, err == nil
}

// parseAttributeTest parses the inside of an attribute selector's brackets.
func parseAttributeTest(tokens []token) (attributeTest, bool) {
	parts := tokens[:0:0]
	for _, current := range tokens {
		if current.kind != whitespaceToken {
			parts = append(parts, current)
		}
	}
	// A namespace prefix: "|name", "ns|name" or "*|name" (not "name|=…").
	if len(parts) > 0 && isDelim(parts[0], "|") {
		parts = parts[1:]
	} else if len(parts) > 2 && (parts[0].kind == identToken || isDelim(parts[0], "*")) && isDelim(parts[1], "|") && parts[2].kind == identToken {
		parts = parts[2:]
	}
	if len(parts) == 0 || parts[0].kind != identToken {
		return attributeTest{}, false
	}
	test := attributeTest{name: strings.ToLower(parts[0].value)}
	parts = parts[1:]
	if len(parts) == 0 {
		return test, true
	}
	switch {
	case isDelim(parts[0], "="):
		test.operator = "="
		parts = parts[1:]
	case len(parts) > 1 && parts[0].kind == delimToken && slices.Contains([]string{"~", "|", "^", "$", "*"}, parts[0].value) && isDelim(parts[1], "="):
		test.operator = parts[0].value + "="
		parts = parts[2:]
	default:
		return attributeTest{}, false
	}
	if len(parts) == 0 || (parts[0].kind != identToken && parts[0].kind != stringToken) {
		return attributeTest{}, false
	}
	test.value = parts[0].value
	parts = parts[1:]
	if len(parts) == 1 && (isIdent(parts[0], "i") || isIdent(parts[0], "s")) {
		test.ignoreCase = isIdent(parts[0], "i")
		parts = parts[1:]
	}
	return test, len(parts) == 0
}
