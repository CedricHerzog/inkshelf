package css

import (
	"slices"
	"strings"
	"testing"
)

// testElement is an element of a tree built in tests.
type testElement struct {
	label      string // what tests call it
	name       string
	attributes map[string]string
	classes    []string
	parent     *testElement
	previous   *testElement
	children   []*testElement
}

// add adds to parent (nil for the root) a child of name, labeled, with attributes given as
// name and value pairs, and returns it.
func add(parent *testElement, label, name string, attributes ...string) *testElement {
	added := &testElement{label: label, name: name, attributes: map[string]string{}, parent: parent}
	for index := 0; index < len(attributes); index += 2 {
		added.attributes[attributes[index]] = attributes[index+1]
	}
	added.classes = strings.Fields(added.attributes["class"])
	if parent != nil {
		if count := len(parent.children); count > 0 {
			added.previous = parent.children[count-1]
		}
		parent.children = append(parent.children, added)
	}
	return added
}

func (element *testElement) Name() string      { return element.name }
func (element *testElement) ID() string        { return element.attributes["id"] }
func (element *testElement) Classes() []string { return element.classes }

func (element *testElement) Attribute(name string) (string, bool) {
	value, ok := element.attributes[name]
	return value, ok
}

func (element *testElement) Parent() Element {
	if element.parent == nil {
		return nil
	}
	return element.parent
}

func (element *testElement) PreviousSibling() Element {
	if element.previous == nil {
		return nil
	}
	return element.previous
}

// all returns element and its descendants, in document order.
func (element *testElement) all() []*testElement {
	elements := []*testElement{element}
	for _, child := range element.children {
		elements = append(elements, child.all()...)
	}
	return elements
}

// testTree returns the root of:
//
//	html
//	  body
//	    div#main.chapter lang="en-US"
//	      h1.title
//	      p.first.intro type="bodymatter chapter"
//	      p (p3)
//	      blockquote
//	        p.quote
//	      p data="x-y" (p5)
//	    a href (link)
func testTree() *testElement {
	html := add(nil, "html", "html")
	body := add(html, "body", "body")
	main := add(body, "main", "div", "id", "main", "class", "chapter", "lang", "en-US")
	add(main, "h1", "h1", "class", "title")
	add(main, "intro", "p", "class", "first intro", "type", "bodymatter chapter")
	add(main, "p3", "p")
	quote := add(main, "blockquote", "blockquote")
	add(quote, "quote", "p", "class", "quote")
	add(main, "p5", "p", "data", "x-y")
	add(body, "link", "a", "href", "#")
	return html
}

func TestSelectorMatching(t *testing.T) {
	tree := testTree()
	for _, test := range []struct {
		selector string
		want     string // the labels of the matching elements
	}{
		{"p", "intro p3 quote p5"},
		{"P", "intro p3 quote p5"},
		{"*", "html body main h1 intro p3 blockquote quote p5 link"},
		{".intro", "intro"},
		{"p.first.intro", "intro"},
		{"p.first.other", ""},
		{"#main", "main"},
		{"div#main.chapter", "main"},
		{"[data]", "p5"},
		{"[data=x-y]", "p5"},
		{`[data="x"]`, ""},
		{`[type~="chapter"]`, "intro"},
		{`[type~="chap"]`, ""},
		{`[lang|="en"]`, "main"},
		{`[lang|=en-US]`, "main"},
		{`[data^="x-"]`, "p5"},
		{`[data$="-y"]`, "p5"},
		{`[data*="-"]`, "p5"},
		{`[data*=""]`, ""},
		{`[lang="EN-us" i]`, "main"},
		{`[lang="EN-us"]`, ""},
		{`[epub|type~="bodymatter"]`, "intro"},
		{`[*|type]`, "intro"},
		{`xhtml|p.quote`, "quote"},
		{`*|*#main`, "main"},
		{"div p", "intro p3 quote p5"},
		{"div > p", "intro p3 p5"},
		{"body p", "intro p3 quote p5"},
		{"html > div p", ""},
		{"html > body > div > blockquote > p", "quote"},
		{"h1 + p", "intro"},
		{"h1 ~ p", "intro p3 p5"},
		{"blockquote + p", "p5"},
		{"h1+p", "intro"},
		{"div>p.first", "intro"},
		{"h1 ~ p + p", "p3"},
		{".chapter blockquote p", "quote"},
		{"p:first-child", "quote"},
		{":first-child", "html body main h1 quote"},
		{"p:first-of-type", "intro quote"},
		{"p:nth-child(2)", "intro"},
		{":nth-child(odd)", "html body main h1 p3 quote p5"},
		{"div > :nth-child(2n)", "intro blockquote"},
		{"div > :nth-child(-n+2)", "h1 intro"},
		{"div > :nth-child(n+4)", "blockquote p5"},
		{"div > :nth-child( 3n + 2 )", "intro p5"},
		{":root", "html"},
		{":link", "link"},
		{"p:not(.intro)", "p3 quote p5"},
		{"p:not(.intro, [data])", "p3 quote"},
		{"div :not(p)", "h1 blockquote"},
		{"p::first-letter", "intro p3 quote p5"},
		{"p:first-line", "intro p3 quote p5"},
		{".\\69ntro", "intro"},
	} {
		selectors := parseSelectorList(tokenize(test.selector))
		if len(selectors) != 1 {
			t.Errorf("%q: %d selectors, want 1", test.selector, len(selectors))
			continue
		}
		var matched []string
		for _, element := range tree.all() {
			if selectors[0].matches(element) {
				matched = append(matched, element.label)
			}
		}
		if got := strings.Join(matched, " "); got != test.want {
			t.Errorf("%q matches %q, want %q", test.selector, got, test.want)
		}
	}
}

func TestSelectorsNotFollowed(t *testing.T) {
	for _, selector := range []string{
		"p:hover", "p:last-child", "li:only-child", "p:empty", "::selection", "p::marker",
		"p::before span", "p::after:hover", "a >", "> a", "a > > b", "p..x", "p.", "[", "[x", "p[=x]",
		"[x=]", "[x=y z]", "[x~y]", ":not(p span)", ":not()", ":not(p", ":nth-child(foo)",
		":nth-child(2n+)", ":nth-child(n 1)", "|", "p|", "#", "p:", "::", "p !", "{}", "p:lang(en)",
	} {
		if selectors := parseSelectorList(tokenize(selector)); len(selectors) != 0 {
			t.Errorf("%q gave %d selectors, want none", selector, len(selectors))
		}
	}
	// One selector of a list that is not followed leaves the others.
	if selectors := parseSelectorList(tokenize("p:hover, p.note , h1")); len(selectors) != 2 {
		t.Errorf("got %d selectors, want 2", len(selectors))
	}
}

func TestSpecificity(t *testing.T) {
	for _, test := range []struct {
		selector              string
		ids, classes, types   int
		pseudoElementExpected string
	}{
		{"*", 0, 0, 0, ""},
		{"p", 0, 0, 1, ""},
		{"div p", 0, 0, 2, ""},
		{".a", 0, 1, 0, ""},
		{"p.a.b", 0, 2, 1, ""},
		{"#x", 1, 0, 0, ""},
		{"#x #y .a p", 2, 1, 1, ""},
		{"[lang]", 0, 1, 0, ""},
		{"a:first-child", 0, 1, 1, ""},
		{"p::first-letter", 0, 0, 2, "first-letter"},
		{"p:before", 0, 0, 2, "before"},
		{"p:not(.a)", 0, 1, 1, ""},
		{"p:not(#x, .a)", 1, 0, 1, ""},
		{"li:nth-child(2n+1)", 0, 1, 1, ""},
		{"html|*", 0, 0, 0, ""},
	} {
		selectors := parseSelectorList(tokenize(test.selector))
		if len(selectors) != 1 {
			t.Errorf("%q: %d selectors, want 1", test.selector, len(selectors))
			continue
		}
		want := test.ids*idSpecificity + test.classes*classSpecificity + test.types*typeSpecificity
		if selectors[0].specificity != want || selectors[0].pseudoElement != test.pseudoElementExpected {
			t.Errorf("%q: specificity %#x and pseudo-element %q, want %#x and %q", test.selector,
				selectors[0].specificity, selectors[0].pseudoElement, want, test.pseudoElementExpected)
		}
	}
}

func TestParseNth(t *testing.T) {
	for _, test := range []struct {
		argument     string
		step, offset int
	}{
		{"odd", 2, 1}, {"EVEN", 2, 0}, {"3", 0, 3}, {"-1", 0, -1}, {"n", 1, 0}, {"-n+3", -1, 3},
		{"+n-1", 1, -1}, {"2n+1", 2, 1}, {"10n", 10, 0},
	} {
		step, offset, ok := parseNth(test.argument)
		if !ok || step != test.step || offset != test.offset {
			t.Errorf("%q: %d, %d, %v; want %d, %d", test.argument, step, offset, ok, test.step, test.offset)
		}
	}
	for _, argument := range []string{"", "x", "2n1", "n+", "2.5n", "nn"} {
		if _, _, ok := parseNth(argument); ok {
			t.Errorf("%q parsed", argument)
		}
	}
}

func TestTokenizerEscapesAndStrings(t *testing.T) {
	for _, test := range []struct {
		css   string
		kinds []tokenKind
		value string // of the first token
	}{
		{`\31 0abc`, []tokenKind{identToken}, "10abc"},
		{`"a\"b"`, []tokenKind{stringToken}, `a"b`},
		{"'a\\\nb'", []tokenKind{stringToken}, "ab"},
		{"'a\nb'", []tokenKind{badStringToken, whitespaceToken, identToken, stringToken}, ""},
		{`"\263A"`, []tokenKind{stringToken}, "☺"},
		{`url( a.png )`, []tokenKind{urlToken}, "a.png"},
		{`url(a b)`, []tokenKind{badURLToken}, ""},
		{`URL("a b")`, []tokenKind{functionToken, stringToken, closeParenToken}, "URL"},
		{`#a-b`, []tokenKind{hashToken}, "a-b"},
		{`-1.5e3px`, []tokenKind{dimensionToken}, "-1.5e3"},
		{`50%`, []tokenKind{percentageToken}, "50"},
		{`.5`, []tokenKind{numberToken}, ".5"},
		{`-->x`, []tokenKind{cdcToken, identToken}, ""},
		{`a/* comment */b`, []tokenKind{identToken, identToken}, "a"},
		{`a /* unclosed`, []tokenKind{identToken, whitespaceToken}, "a"},
		{"@media", []tokenKind{atKeywordToken}, "media"},
		{"--custom", []tokenKind{identToken}, "--custom"},
		{"é", []tokenKind{identToken}, "é"},
	} {
		tokens := tokenize(test.css)
		var kinds []tokenKind
		for _, current := range tokens {
			kinds = append(kinds, current.kind)
		}
		if !slices.Equal(kinds, test.kinds) || tokens[0].value != test.value {
			t.Errorf("%q: kinds %v, value %q; want %v, %q", test.css, kinds, tokens[0].value, test.kinds, test.value)
		}
	}
}
