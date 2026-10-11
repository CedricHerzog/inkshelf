package css

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// screen is the media of the tests: an InkPad 3 in portrait.
var screen = Media{Type: "screen", Width: 1404, Height: 1872}

// computeIn returns the values that css declares for the element labeled in testTree.
func computeIn(css, label, pseudoElement string) map[string]string {
	cascade := NewCascade()
	cascade.Add(Parse(css, screen))
	for _, element := range testTree().all() {
		if element.label == label {
			return cascade.Compute(element, pseudoElement)
		}
	}
	panic("no element " + label)
}

func TestCascade(t *testing.T) {
	for _, test := range []struct {
		name, css, label string
		want             string // the value of color
	}{
		{"later wins", "p { color: red } p { color: blue }", "intro", "blue"},
		{"class over type", ".intro { color: red } p { color: blue }", "intro", "red"},
		{"id over classes", "#main { color: red } div.chapter.chapter { color: blue }", "main", "red"},
		{"descendant adds", "div p { color: red } p { color: blue }", "intro", "red"},
		{"important over specificity", "p { color: red !important } #main p.intro { color: blue }", "intro", "red"},
		{"important, spaced and uppercase", "p { color: red ! IMPORTANT } p.intro { color: blue }", "intro", "red"},
		{"important by specificity", "p.intro { color: red !important } p { color: blue !important }", "intro", "red"},
		{"no match", "h1 { color: red }", "intro", ""},
		{"universal", "* { color: red }", "intro", "red"},
		{"same rule, two selectors", "p, .intro { color: red }", "intro", "red"},
		{"last declaration wins", "p { color: red; color: blue }", "intro", "blue"},
		{"case of the property", "p { COLOR: red }", "intro", "red"},
		{"value as written", "p { color: RGB( 1, 2, 3 ) }", "intro", "RGB( 1, 2, 3 )"},
		{"pseudo-element apart", "p::first-letter { color: red }", "intro", ""},
	} {
		got := computeIn(test.css, test.label, "")["color"]
		if got != test.want {
			t.Errorf("%s: color %q, want %q", test.name, got, test.want)
		}
	}
}

func TestInlineStyle(t *testing.T) {
	element := add(nil, "p", "p", "class", "x", "style", "color: green; margin-left: 2em")
	for _, test := range []struct{ css, want string }{
		{"p.x { color: red }", "green"},
		{"#y, p.x { color: red !important }", "red"},
		{"p { color: red !important }", "red"},
	} {
		cascade := NewCascade()
		cascade.Add(Parse(test.css, screen))
		values := cascade.Compute(element, "")
		if values["color"] != test.want || values["margin-left"] != "2em" {
			t.Errorf("%q: %v, want color %q", test.css, values, test.want)
		}
	}
	important := add(nil, "p", "p", "style", "color: green !important")
	cascade := NewCascade()
	cascade.Add(Parse("p { color: red !important }", screen))
	if got := cascade.Compute(important, "")["color"]; got != "green" {
		t.Errorf("important style attribute: %q, want green", got)
	}
	if got := cascade.Compute(important, "first-letter"); got != nil {
		t.Errorf("a pseudo-element got the style attribute: %v", got)
	}
}

func TestPseudoElements(t *testing.T) {
	css := "p::first-letter { float: left } p:first-letter { font-size: 3em } .intro::before { content: '§' }"
	if got := computeIn(css, "intro", "first-letter"); !maps.Equal(got, map[string]string{"float": "left", "font-size": "3em"}) {
		t.Errorf("first-letter: %v", got)
	}
	if got := computeIn(css, "intro", "before"); got["content"] != "'§'" {
		t.Errorf("before: %v", got)
	}
	if got := computeIn(css, "p3", "before"); got != nil {
		t.Errorf("p3 before: %v", got)
	}
}

func TestCascadeOrderAcrossSheets(t *testing.T) {
	cascade := NewCascade()
	cascade.Add(Parse("p { color: red; margin-top: 1em }", screen))
	cascade.Add(Parse("p { color: blue }", screen))
	values := cascade.Compute(add(nil, "p", "p"), "")
	if values["color"] != "blue" || values["margin-top"] != "1em" {
		t.Errorf("got %v", values)
	}
}

func TestMediaQueries(t *testing.T) {
	for _, test := range []struct {
		query string
		want  bool
	}{
		{"", true},
		{"screen", true},
		{"SCREEN", true},
		{"all", true},
		{"print", false},
		{"amzn-kf8", false},
		{"amzn-mobi", false},
		{"not amzn-mobi", true},
		{"not screen", false},
		{"only screen", true},
		{"print, screen", true},
		{"print, amzn-kf8", false},
		{"screen and (min-width: 600px)", true},
		{"screen and (max-width: 600px)", false},
		{"screen and (min-width: 1404px) and (max-width: 1404px)", true},
		{"(width: 1404px)", true},
		{"(orientation: portrait)", true},
		{"(orientation: landscape)", false},
		{"not screen and (orientation: landscape)", true},
		{"(min-width: 40em)", true},
		{"(max-width: 80em)", false},
		{"(max-device-width: 2000px)", true},
		{"(min-height: 20in)", false},
		{"(min-width: 50%)", false},
		{"(width)", true},
		{"(color)", false},
		{"(amzn-kf8)", false},
		{"screen and (-webkit-min-device-pixel-ratio: 2)", false},
		{"not (color)", true},
		{"screen (min-width: 1px)", false},
		{"screen and", false},
		{"and (width)", false},
		{"screen and (min-width: 1px", false},
		{"(width > 100px)", false},
	} {
		if got := screen.Matches(test.query); got != test.want {
			t.Errorf("%q: %v, want %v", test.query, got, test.want)
		}
	}
}

func TestAtRules(t *testing.T) {
	css := `@charset "utf-8";
@import url(base.css);
@import "print.css" print;
@import url("fonts.css") screen and (min-width: 100px);
@namespace epub "http://www.idpf.org/2007/ops";
@font-face {
	font-family: "Libre Baskerville";
	font-weight: bold;
	font-style: italic;
	src: local("Baskerville"), url(../fonts/LibreBaskerville-Bold.woff2) format("woff2"), url('../fonts/b.ttf') format(truetype);
}
@font-face { font-family: Charis SIL; src: url(charis.otf) }
@media amzn-kf8 { p { color: red } }
@media screen { @media (min-width: 100px) { p { color: green } } }
@page { margin: 0 }
@supports (display: flex) { p { color: purple } }
@-webkit-keyframes spin { from { color: red } to { color: blue } }
p { margin-top: 1em }`
	sheet := Parse(css, screen)
	if want := []string{"base.css", "fonts.css"}; !slices.Equal(sheet.Imports, want) {
		t.Errorf("imports %q, want %q", sheet.Imports, want)
	}
	want := []FontFace{
		{Family: "Libre Baskerville", Weight: "bold", Style: "italic", Sources: []FontSource{
			{URL: "../fonts/LibreBaskerville-Bold.woff2", Format: "woff2"},
			{URL: "../fonts/b.ttf", Format: "truetype"},
		}},
		{Family: "Charis SIL", Sources: []FontSource{{URL: "charis.otf"}}},
	}
	if fmt.Sprint(sheet.FontFaces) != fmt.Sprint(want) {
		t.Errorf("font faces %+v,\nwant %+v", sheet.FontFaces, want)
	}
	cascade := NewCascade()
	cascade.Add(sheet)
	values := cascade.Compute(add(nil, "p", "p"), "")
	if !maps.Equal(values, map[string]string{"color": "green", "margin-top": "1em"}) {
		t.Errorf("got %v", values)
	}
}

func TestBrokenCSS(t *testing.T) {
	for _, test := range []struct{ name, css, want string }{
		{"missing semicolon", "p { color: red margin-top: 1em } p { text-indent: 0 }", "color=red margin-top: 1em text-indent=0"},
		{"stray semicolons", ";; p { ;color: red;; } ;", "color=red"},
		{"bad declaration", "p { color; text-indent: 0; : x; 5: y; margin-top: }", "text-indent=0"},
		{"bad string", "p { content: 'a\n; color: red } p { text-indent: 0 }", "color=red text-indent=0"},
		{"braces in strings", `p { content: "}" ; color: red }`, `color=red content="}"`},
		{"nested block", "p { color: red; &:hover { color: blue } text-indent: 0 }", "color=red"},
		{"block in a value", "p { color: {red}; text-indent: 0 }", "color={red} text-indent=0"},
		{"stray brace", "} p { color: red }", ""},
		{"unclosed rule", "p { color: red", "color=red"},
		{"unclosed comment", "p { color: red } /* p { color: blue }", "color=red"},
		{"rule without block", "p { color: red } h1", "color=red"},
		{"unknown at-rule", "@foo bar { p { color: blue } } p { color: red }", "color=red"},
		{"at-rule without block", "@foo bar; p { color: red }", "color=red"},
		{"html comments", "<!-- p { color: red } -->", "color=red"},
		{"bad selector", "p:hover { color: blue } p! { color: blue } p { color: red }", "color=red"},
		{"brackets in values", "p { font-family: a(b; c) ; color: red }", "color=red font-family=a(b; c)"},
		{"escaped property", `p { c\olor: red }`, "color=red"},
		{"bad url", "p { background: url(a b); color: red }", "color=red"},
		{"empty", "", ""},
	} {
		values := computeIn(test.css, "p3", "")
		var got []string
		for _, property := range slices.Sorted(maps.Keys(values)) {
			got = append(got, property+"="+values[property])
		}
		if strings.Join(got, " ") != test.want {
			t.Errorf("%s: %q, want %q", test.name, strings.Join(got, " "), test.want)
		}
	}
}

func TestShorthands(t *testing.T) {
	for _, test := range []struct{ declaration, want string }{
		{"margin: 1em", "margin-bottom=1em margin-left=1em margin-right=1em margin-top=1em"},
		{"margin: 1em auto", "margin-bottom=1em margin-left=auto margin-right=auto margin-top=1em"},
		{"padding: 1px 2px 3px", "padding-bottom=3px padding-left=2px padding-right=2px padding-top=1px"},
		{"margin: 1px 2px 3px 4px", "margin-bottom=3px margin-left=4px margin-right=2px margin-top=1px"},
		{"margin: 1px 2px 3px 4px 5px", ""},
		{"margin: calc(1em + 2px) 0", "margin-bottom=calc(1em + 2px) margin-left=0 margin-right=0 margin-top=calc(1em + 2px)"},
		{"margin: 0 !important; margin-top: 1em", "margin-bottom=0 margin-left=0 margin-right=0 margin-top=0"},
		{"border-width: thin 2px", "border-bottom-width=thin border-left-width=2px border-right-width=2px border-top-width=thin"},
		{"border: 1px solid black", "border-bottom-color=black border-bottom-style=solid border-bottom-width=1px " +
			"border-left-color=black border-left-style=solid border-left-width=1px " +
			"border-right-color=black border-right-style=solid border-right-width=1px " +
			"border-top-color=black border-top-style=solid border-top-width=1px"},
		{"border-top: double", "border-top-color=currentcolor border-top-style=double border-top-width=medium"},
		{"border-bottom: #ccc 0.1em dotted", "border-bottom-color=#ccc border-bottom-style=dotted border-bottom-width=0.1em"},
		{"border-left: solid dotted", ""},
		{"font: italic bold 12px/1.5 Georgia, 'Times New Roman', serif", "font-family=Georgia, 'Times New Roman', serif " +
			"font-size=12px font-style=italic font-variant=normal font-weight=bold line-height=1.5"},
		{"font: 700 small-caps 1em / 120% serif", "font-family=serif font-size=1em font-style=normal font-variant=small-caps " +
			"font-weight=700 line-height=120%"},
		{"font: larger sans-serif", "font-family=sans-serif font-size=larger font-style=normal font-variant=normal " +
			"font-weight=normal line-height=normal"},
		{"font: normal condensed 80% monospace", "font-family=monospace font-size=80% font-style=normal font-variant=normal " +
			"font-weight=normal line-height=normal"},
		{"font: 12px", ""},
		{"font: caption", ""},
		{"font: bold serif", ""},
		{"list-style: none", "list-style-image=none list-style-position=outside list-style-type=none"},
		{"list-style: upper-roman inside", "list-style-image=none list-style-position=inside list-style-type=upper-roman"},
		{"list-style: url(dot.png) square", "list-style-image=url(dot.png) list-style-position=outside list-style-type=square"},
		{"text-decoration: underline", "text-decoration-color=currentcolor text-decoration-line=underline text-decoration-style=solid"},
		{"text-decoration: underline overline wavy red", "text-decoration-color=red text-decoration-line=underline overline text-decoration-style=wavy"},
		{"text-decoration: none", "text-decoration-color=currentcolor text-decoration-line=none text-decoration-style=solid"},
		{"margin-top: 1em", "margin-top=1em"},
	} {
		cascade := NewCascade()
		cascade.Add(Parse("p {"+test.declaration+"}", screen))
		values := cascade.Compute(add(nil, "p", "p"), "")
		var got []string
		for _, property := range slices.Sorted(maps.Keys(values)) {
			got = append(got, property+"="+values[property])
		}
		if strings.Join(got, " ") != test.want {
			t.Errorf("%s:\n got %q\nwant %q", test.declaration, strings.Join(got, " "), test.want)
		}
	}
}

func TestParseLength(t *testing.T) {
	for _, test := range []struct {
		value string
		want  Length
		ok    bool
	}{
		{"1.5em", Length{1.5, "em"}, true},
		{" -10% ", Length{-10, "%"}, true},
		{"0", Length{0, ""}, true},
		{"1.2", Length{1.2, ""}, true},
		{"12PX", Length{12, "px"}, true},
		{"2rem", Length{2, "rem"}, true},
		{"3Q", Length{3, "q"}, true},
		{".5vw", Length{0.5, "vw"}, true},
		{"1e1pt", Length{10, "pt"}, true},
		{"auto", Length{}, false},
		{"1 em", Length{}, false},
		{"12dpi", Length{}, false},
		{"", Length{}, false},
	} {
		got, ok := ParseLength(test.value)
		if got != test.want || ok != test.ok {
			t.Errorf("%q: %v, %v; want %v, %v", test.value, got, ok, test.want, test.ok)
		}
	}
	for _, test := range []struct {
		length Length
		pixels float64
		ok     bool
	}{
		{Length{12, "pt"}, 16, true}, {Length{1, "in"}, 96, true}, {Length{0, ""}, 0, true},
		{Length{1, "em"}, 0, false}, {Length{50, "%"}, 0, false}, {Length{1.5, ""}, 0, false},
	} {
		if pixels, ok := test.length.Pixels(); pixels != test.pixels || ok != test.ok {
			t.Errorf("%v: %v px, %v; want %v, %v", test.length, pixels, ok, test.pixels, test.ok)
		}
	}
}

func TestFamilies(t *testing.T) {
	for _, test := range []struct {
		value string
		want  []string
	}{
		{"serif", []string{"serif"}},
		{`"Times New Roman", Times,serif`, []string{"Times New Roman", "Times", "serif"}},
		{"Gentium  Book Basic , 'x'", []string{"Gentium Book Basic", "x"}},
		{"", nil},
	} {
		if got := Families(test.value); !slices.Equal(got, test.want) {
			t.Errorf("%q: %q, want %q", test.value, got, test.want)
		}
	}
}

// BenchmarkCompute computes the values of an element 20 levels deep against 400 rules, as a
// book's stylesheet may have.
func BenchmarkCompute(b *testing.B) {
	var css strings.Builder
	for index := range 100 {
		fmt.Fprintf(&css, ".c%d { margin-top: %dpx }\n", index, index)
		fmt.Fprintf(&css, "div.c%d > p { color: red }\n", index)
		fmt.Fprintf(&css, "#id%d p span { font-weight: bold }\n", index)
		fmt.Fprintf(&css, "p + p.c%d, div * { text-indent: 1em !important }\n", index)
	}
	cascade := NewCascade()
	cascade.Add(Parse(css.String(), screen))
	parent := add(nil, "html", "html")
	for depth := range 20 {
		parent = add(parent, "div", "div", "class", fmt.Sprintf("c%d c%d", depth, depth+50), "id", fmt.Sprintf("id%d", depth))
	}
	add(parent, "previous", "p")
	element := add(parent, "p", "p", "class", "c7 c19 c80", "style", "text-align: center")
	b.ResetTimer()
	for b.Loop() {
		cascade.Compute(element, "")
	}
}
