package css

import (
	"slices"
	"strings"
)

// Element is an element that selectors are matched against. Parent and PreviousSibling
// return a nil Element (not a typed nil pointer in an Element) when there is none.
//
// A builder reading XHTML as a stream implements it with its stack of open elements, each
// keeping its parent and the last child closed so far: when an element opens, that child is
// its previous sibling. What follows an element is never asked for, which is why :last-child,
// :only-child, :empty and the like are not followed.
type Element interface {
	Name() string // the local name, lowercase
	ID() string
	Classes() []string
	// Attribute returns the value of the attribute of a local name, lowercase. Compute reads
	// the style attribute through it.
	Attribute(name string) (string, bool)
	Parent() Element
	PreviousSibling() Element // the previous element among the parent's children
}

// Cascade is stylesheets in order, their rules indexed by what their selectors' rightmost
// compound needs: an id, else a class, else a type.
type Cascade struct {
	byID, byClass, byName map[string][]indexedSelector
	universal             []indexedSelector
	rules                 int // the rules added so far, which orders them
}

// indexedSelector is a selector of a rule, with the rule's declarations and order.
type indexedSelector struct {
	selector     selector
	declarations []declaration
	order        int
}

// NewCascade returns an empty cascade.
func NewCascade() *Cascade {
	return &Cascade{byID: map[string][]indexedSelector{}, byClass: map[string][]indexedSelector{}, byName: map[string][]indexedSelector{}}
}

// Add adds the rules of sheet after those added before: they win over them at the same
// specificity. The sheets that sheet imports go first.
func (cascade *Cascade) Add(sheet *Stylesheet) {
	for _, added := range sheet.rules {
		for _, parsed := range added.selectors {
			indexed := indexedSelector{selector: parsed, declarations: added.declarations, order: cascade.rules}
			switch rightmost := parsed.compounds[0]; {
			case rightmost.id != "":
				cascade.byID[rightmost.id] = append(cascade.byID[rightmost.id], indexed)
			case len(rightmost.classes) > 0:
				cascade.byClass[rightmost.classes[0]] = append(cascade.byClass[rightmost.classes[0]], indexed)
			case rightmost.name != "":
				cascade.byName[rightmost.name] = append(cascade.byName[rightmost.name], indexed)
			default:
				cascade.universal = append(cascade.universal, indexed)
			}
		}
		cascade.rules++
	}
}

// Compute returns the values declared for element, by property (lowercase), or for its
// pseudo-element ("first-letter", "first-line", "before" or "after") if not "". Values are
// as written, whitespace collapsed; margin, padding, font, border, border-*, list-style and
// text-decoration are expanded into their longhands. Inheritance is left to the caller.
//
// The cascade: rules by specificity, then order; the style attribute (for the element
// itself) above them; then !important declarations, in the same order. The map is nil when
// nothing applies.
func (cascade *Cascade) Compute(element Element, pseudoElement string) map[string]string {
	layers := cascade.matching(element, pseudoElement)
	var inline []declaration
	if style, ok := element.Attribute("style"); ok && pseudoElement == "" {
		inline = parseDeclarations(tokenize(style))
	}
	if len(layers) == 0 && len(inline) == 0 {
		return nil
	}
	layers = append(layers, inline) // the style attribute above the rules
	values := map[string]string{}
	for _, important := range []bool{false, true} {
		for _, declarations := range layers {
			for _, declared := range declarations {
				if declared.important == important {
					values[declared.property] = declared.value
				}
			}
		}
	}
	return values
}

// matching returns the declarations of the rules whose selectors match element and
// pseudoElement, from the least specific to the most.
func (cascade *Cascade) matching(element Element, pseudoElement string) [][]declaration {
	var matched []*indexedSelector // pointers, which sort faster than selectors
	collect := func(candidates []indexedSelector) {
		for index := range candidates {
			candidate := &candidates[index]
			if candidate.selector.pseudoElement == pseudoElement && candidate.selector.matches(element) {
				matched = append(matched, candidate)
			}
		}
	}
	if id := element.ID(); id != "" {
		collect(cascade.byID[id])
	}
	classes := element.Classes()
	for index, class := range classes {
		if !slices.Contains(classes[:index], class) { // class="a a" matches a's rules once
			collect(cascade.byClass[class])
		}
	}
	collect(cascade.byName[element.Name()])
	collect(cascade.universal)
	slices.SortFunc(matched, func(first, second *indexedSelector) int {
		if first.selector.specificity != second.selector.specificity {
			return first.selector.specificity - second.selector.specificity
		}
		return first.order - second.order
	})
	declarations := make([][]declaration, len(matched))
	for index, candidate := range matched {
		declarations[index] = candidate.declarations
	}
	return declarations
}

// matches reports whether element matches the selector (its pseudo-element aside).
func (parsed selector) matches(element Element) bool {
	return parsed.matchesFrom(0, element)
}

// matchesFrom reports whether element matches compounds[index], and its relatives the
// compounds to its left.
func (parsed selector) matchesFrom(index int, element Element) bool {
	current := parsed.compounds[index]
	if !current.matches(element) {
		return false
	}
	if index == len(parsed.compounds)-1 {
		return true
	}
	switch current.combinator {
	case child:
		parent := element.Parent()
		return parent != nil && parsed.matchesFrom(index+1, parent)
	case descendant:
		for ancestor := element.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
			if parsed.matchesFrom(index+1, ancestor) {
				return true
			}
		}
	case nextSibling:
		previous := element.PreviousSibling()
		return previous != nil && parsed.matchesFrom(index+1, previous)
	case subsequentSibling:
		for previous := element.PreviousSibling(); previous != nil; previous = previous.PreviousSibling() {
			if parsed.matchesFrom(index+1, previous) {
				return true
			}
		}
	}
	return false
}

// matches reports whether element matches all the simple selectors of the compound.
func (part compound) matches(element Element) bool {
	if part.name != "" && element.Name() != part.name {
		return false
	}
	if part.id != "" && element.ID() != part.id {
		return false
	}
	for _, class := range part.classes {
		if !slices.Contains(element.Classes(), class) {
			return false
		}
	}
	for _, test := range part.attributes {
		if !test.matches(element) {
			return false
		}
	}
	for _, pseudo := range part.pseudoClasses {
		if !pseudo.matches(element) {
			return false
		}
	}
	return true
}

// matches reports whether element has the attribute that the test asks for.
func (test attributeTest) matches(element Element) bool {
	value, ok := element.Attribute(test.name)
	if !ok {
		return false
	}
	wanted := test.value
	if test.ignoreCase {
		value, wanted = strings.ToLower(value), strings.ToLower(wanted)
	}
	switch test.operator {
	case "=":
		return value == wanted
	case "~=":
		return wanted != "" && slices.Contains(strings.Fields(value), wanted)
	case "|=":
		return value == wanted || strings.HasPrefix(value, wanted+"-")
	case "^=":
		return wanted != "" && strings.HasPrefix(value, wanted)
	case "$=":
		return wanted != "" && strings.HasSuffix(value, wanted)
	case "*=":
		return wanted != "" && strings.Contains(value, wanted)
	}
	return true
}

// matches reports whether element matches the pseudo-class.
func (pseudo pseudoClass) matches(element Element) bool {
	switch pseudo.kind {
	case firstChild:
		return element.PreviousSibling() == nil
	case firstOfType:
		for previous := element.PreviousSibling(); previous != nil; previous = previous.PreviousSibling() {
			if previous.Name() == element.Name() {
				return false
			}
		}
		return true
	case nthChild:
		position := 1
		for previous := element.PreviousSibling(); previous != nil; previous = previous.PreviousSibling() {
			position++
		}
		if pseudo.step == 0 {
			return position == pseudo.offset
		}
		steps := position - pseudo.offset
		return steps%pseudo.step == 0 && steps/pseudo.step >= 0
	case root:
		return element.Parent() == nil
	case link:
		_, hasHref := element.Attribute("href")
		return element.Name() == "a" && hasHref
	case not:
		for _, negated := range pseudo.negated {
			if negated.matches(element) {
				return false
			}
		}
		return true
	}
	return false
}
