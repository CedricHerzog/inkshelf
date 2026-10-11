package epub

import (
	"fmt"
	"image"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"

	"golang.org/x/image/font/gofont/gobold"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
	"github.com/CedricHerzog/inkshelf/internal/web/webfont"
)

// openStyled opens a book of one chapter whose body is body, linking OEBPS/style.css, which
// is css, and of files (paths in the zip). The book is closed with the test.
func openStyled(t testing.TB, css, body string, files map[string]string) *Book {
	t.Helper()
	if files == nil {
		files = map[string]string{}
	}
	files["OEBPS/style.css"] = css
	files["OEBPS/chapter.xhtml"] = `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><link rel="stylesheet" type="text/css" href="style.css"/></head><body>` + body + `</body></html>`
	return openPackage(t, `unique-identifier="uid"`, `<dc:identifier id="uid">urn:uuid:1234</dc:identifier><dc:language>en</dc:language>`,
		`<item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>`, `><itemref idref="chapter"/>`, files)
}

// styledChapter returns the chapter of openStyled.
func styledChapter(t *testing.T, css, body string) *document.Document {
	t.Helper()
	doc, err := openStyled(t, css, body, nil).Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// runOf returns the run of doc whose text, trimmed, is text.
func runOf(t *testing.T, doc *document.Document, text string) document.Run {
	t.Helper()
	for _, block := range doc.Blocks {
		for _, run := range block.Runs {
			if strings.TrimSpace(run.Text) == text {
				return run
			}
		}
	}
	t.Fatalf("no run %q in %q", text, texts(doc))
	return document.Run{}
}

func TestInheritance(t *testing.T) {
	doc := styledChapter(t, `
div.box { font-size: 1.5em; text-align: center; font-weight: bold; text-indent: 1em }
div.box p { margin: 2em 1em 0 0; line-height: 1.4 }
.wide { letter-spacing: 0.1em; font-weight: normal }
.half { font-size: 50% }`,
		`<div class="box"><p>Big <span class="wide">wide</span> <span class="half">half</span></p></div><p>After</p>`)
	big := doc.Blocks[0]
	if big.FontScale != 150 || big.Align != document.AlignCenter || big.Indent != 150 || big.MarginTop != 300 || big.Right != 150 || big.LineHeight != 140 {
		t.Errorf("the block inherits size, alignment and indent; em are its own size's: %+v", big)
	}
	if run := runOf(t, doc, "Big"); run.Style != document.Bold || run.FontScale != 0 {
		t.Errorf("Big: %+v", run)
	}
	if run := runOf(t, doc, "wide"); run.Style != 0 || run.LetterSpacing != 10 {
		t.Errorf("wide: %+v, want 10 hundredths of its em apart", run)
	}
	if run := runOf(t, doc, "half"); run.FontScale != 50 || run.Style != document.Bold {
		t.Errorf("half: %+v, want half the block's size", run)
	}
	if after := doc.Blocks[1]; after.FontScale != 0 || after.Align != document.AlignDefault || after.Runs[0].Style != 0 {
		t.Errorf("a sibling inherits nothing: %+v", after)
	}
}

func TestSelectors(t *testing.T) {
	doc := styledChapter(t, `
section.notes p { text-indent: 3em }
p + p { font-style: italic }
#last { text-align: right }
p:first-child { font-weight: bold }`,
		`<section class="notes"><p>One</p><p>Two</p></section><p id="last">Three</p>`)
	one, two, three := doc.Blocks[0], doc.Blocks[1], doc.Blocks[2]
	if one.Indent != 300 || two.Indent != 300 || three.Indent != 0 {
		t.Errorf("indents %d, %d, %d; want a descendant selector to apply", one.Indent, two.Indent, three.Indent)
	}
	if one.Runs[0].Style != document.Bold || two.Runs[0].Style != document.Italic || three.Runs[0].Style != 0 {
		t.Errorf("styles %v, %v, %v", one.Runs[0].Style, two.Runs[0].Style, three.Runs[0].Style)
	}
	if three.Align != document.AlignRight {
		t.Errorf("align by id: %v", three.Align)
	}
}

func TestRuns(t *testing.T) {
	doc := styledChapter(t, `
.big { font-size: 2em } .up { vertical-align: 0.5em } .sc { font-variant: small-caps }
.upper { text-transform: uppercase } .cap { text-transform: capitalize } .serif { font-family: Georgia, serif }
.plain { text-decoration: none } .small-sup { font-size: 60% } .wide { text-transform: -epub-fullwidth }`,
		`<p>a<sup>1</sup> <sub>2</sub> <sup class="small-sup">3</sup> <span class="big">B</span> <span class="up">c</span>
<u>under <b>bold</b> <span class="plain">plain</span></u> <s>gone</s> <span class="sc">caps</span>
<span class="upper">straße</span> <span class="cap">l'été dit-il</span> <code>x</code> <span class="serif">g</span>
<span class="wide">12 ab</span></p>`)
	for _, test := range []struct {
		text string
		want document.Run
	}{
		{"1", document.Run{Style: document.Small, Shift: 33}},
		{"2", document.Run{Style: document.Small, Shift: -20}},
		{"3", document.Run{FontScale: 60, Shift: 33}}, // smaller already: not Small
		{"B", document.Run{FontScale: 200}},
		{"c", document.Run{Shift: 50}},
		{"under", document.Run{Style: document.Underline}},
		{"bold", document.Run{Style: document.Underline | document.Bold}},
		{"plain", document.Run{Style: document.Underline}}, // a decoration is drawn through descendants
		{"gone", document.Run{Style: document.Strike}},
		{"caps", document.Run{Style: document.SmallCaps}},
		{"x", document.Run{Family: "monospace"}},
		{"g", document.Run{Family: "serif"}},
	} {
		got := runOf(t, doc, test.text)
		got.Text = ""
		if got != test.want {
			t.Errorf("%s: %+v, want %+v", test.text, got, test.want)
		}
	}
	// Transformed one rune for one: ß has no capital of its own.
	if got := texts(doc)[0]; !strings.Contains(got, " STRAßE L'été Dit-Il ") {
		t.Errorf("text-transform: %q", got)
	}
	if got := texts(doc)[0]; !strings.Contains(got, "１２ ａｂ") {
		t.Errorf("full-width: %q", got)
	}
}

func TestLinks(t *testing.T) {
	doc := styledChapter(t, "", `<p><a href="notes.xhtml#n1">note</a> <a href="#here">here</a> <a href="https://example.org/">web</a>
<a href="javascript:run()">script</a> <a href="../Other%20Dir/x.xhtml">other</a> <a id="here">anchor</a></p>`)
	for text, want := range map[string]string{
		"note": "OEBPS/notes.xhtml#n1", "here": "OEBPS/chapter.xhtml#here", "web": "https://example.org/",
		"script": "", "other": "Other Dir/x.xhtml", "anchor": "",
	} {
		if run := runOf(t, doc, text); run.Link != want || run.Style != 0 {
			t.Errorf("%s: link %q, style %v; want %q, not underlined", text, run.Link, run.Style, want)
		}
	}
}

func TestFootnotes(t *testing.T) {
	doc := styledChapter(t, "", `<p>Body<a epub:type="noteref" href="#n1">1</a></p>
<aside epub:type="footnote" id="n1"><p>Note one</p><p>More</p></aside>
<div epub:type="rearnote"><p id="n2">Two</p></div>
<ol><li epub:type="endnote" id="n3">Three</li></ol>`)
	var got []string
	for _, block := range doc.Blocks {
		got = append(got, block.Footnote)
	}
	if want := []string{"", "n1", "n1", "n2", "n3"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("footnotes %q, want %q", got, want)
	}
}

func TestImages(t *testing.T) {
	book := openStyled(t, `.glyph { height: 1em } .half { width: 50% } .capped { max-width: 300px } p.drop { margin-top: 2em }`,
		`<p>A <img class="glyph" src="big.png"/> glyph, an icon <img src="icon.png"/>.</p>
<p class="drop"><img src="icon.png"/></p>
<p>Text <img src="big.png"/> more</p>
<div><img class="half" src="big.png"/></div>
<img class="capped" src="big.png"/>
<img width="150" src="big.png"/>`,
		map[string]string{"OEBPS/big.png": pngOf(t, image.Pt(1200, 900)), "OEBPS/icon.png": pngOf(t, image.Pt(16, 16))})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"A [image OEBPS/big.png] glyph, an icon [image OEBPS/icon.png].",
		"[image OEBPS/icon.png]", // alone: a block
		"Text", "[image OEBPS/big.png]", "more",
		"[image OEBPS/big.png]", "[image OEBPS/big.png]", "[image OEBPS/big.png]",
	}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	if glyph := doc.Blocks[0].Runs[1]; glyph.ImageHeight != 100 || doc.Blocks[0].Runs[3].ImageHeight != 0 {
		t.Errorf("an image 1em high: %+v; one of its own size: %+v", glyph, doc.Blocks[0].Runs[3])
	}
	if icon := doc.Blocks[1]; icon.MarginTop != 200 {
		t.Errorf("a lone small image keeps its paragraph's margin: %+v", icon)
	}
	for index, wantWidth := range map[int]int{3: 0, 5: 50, 6: 50, 7: 25} {
		if width := doc.Blocks[index].ImageWidth; width != wantWidth {
			t.Errorf("block %d: width %d%%, want %d%%", index, width, wantWidth)
		}
	}
}

func TestBlocks(t *testing.T) {
	doc := styledChapter(t, `
.inline { display: inline } .block { display: block } .gone { display: none }
.chapter { page-break-before: always } .end { page-break-after: always }
.boxed { border-top: 1px solid black; border-bottom: 2px dotted; padding-left: 1em; margin-left: 1em; margin-right: 10% }
.hang { padding-left: 2em; text-indent: -3em } .wide { margin-left: 50% }
.auto { -epub-hyphens: auto } .keep { break-after: avoid }
.lines { white-space: pre-line }`,
		`<p class="inline">one</p> <p class="inline">two</p><span class="block">three</span><p class="gone" id="gone">hidden</p>
<p class="chapter">four</p><div class="end"><p>five</p></div><p>six</p>
<p class="boxed">seven</p><p class="hang">eight</p><p class="wide">nine</p><p class="auto">ten</p><h2 class="keep">eleven</h2>
<p class="lines">a  b
c</p><p hidden="">hidden too</p><p>He said <q>yes</q>.</p>`)
	want := []string{"one two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "a b", "c", "He said “yes”."}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("blocks\n%q\nwant\n%q", got, want)
	}
	if _, ok := doc.IDs["gone"]; ok {
		t.Error("a hidden element's id is an anchor")
	}
	blocks := doc.Blocks
	if !blocks[2].PageBreak || !blocks[4].PageBreak || blocks[3].PageBreak {
		t.Errorf("page breaks %v, %v, %v; want before four and six", blocks[2].PageBreak, blocks[3].PageBreak, blocks[4].PageBreak)
	}
	if boxed := blocks[5]; boxed.BorderTop || boxed.BorderBottom || boxed.Left != 200 || boxed.Right != 375 {
		t.Errorf("boxed %+v", boxed)
	}
	wantBox := document.BorderBox{
		Start: blocks[5].Start, End: blocks[6].Start, Top: document.Border{Style: document.Solid, Width: 6},
		Bottom: document.Border{Style: document.Dotted, Width: 13}, MarginTop: 100, MarginBottom: 100, LeftEdge: 100, RightEdge: 375,
	}
	if len(doc.Boxes) != 1 || doc.Boxes[0] != wantBox {
		t.Errorf("boxes %+v, want %+v", doc.Boxes, wantBox)
	}
	if hang := blocks[6]; hang.Left != 200 || hang.Indent != -200 {
		t.Errorf("a hanging indent: %+v, want left 200, indent -200", hang)
	}
	if wide := blocks[7]; wide.Left != maxMargin {
		t.Errorf("a wide margin: %d, want %d", wide.Left, maxMargin)
	}
	if !blocks[8].Hyphenate || blocks[7].Hyphenate {
		t.Error("hyphens: auto")
	}
	if !blocks[9].KeepWithNext || blocks[9].Kind != document.Heading || blocks[9].Level != 2 {
		t.Errorf("heading %+v", blocks[9])
	}
}

// A <br> stays a line break and an <hr> a rule whatever their display, as Calibre makes them
// blocks: no empty line, no borders around the rule nor on the next block.
func TestBreaksAndRulesAsBlocks(t *testing.T) {
	for _, test := range []struct{ name, css, plain, styled string }{
		{"br", `.b { display: block }`, `<p>a<br/>b</p><p>c</p>`, `<p>a<br class="b"/>b</p><p>c</p>`},
		{"hr", `.h { display: block }`, `<p>a</p><hr/><p>b</p>`, `<p>a</p><hr class="h"/><p>b</p>`},
		{"bordered hr", `.h { display: block; height: 2px; margin: 0.2em 0; border: #000 solid 1px }`,
			`<p>a</p><hr/><p>b</p>`, `<p>a</p><hr class="h"/><p>b</p>`},
		{"bordered inline hr", `.h { border-top: 1px solid; border-bottom: 1px solid }`,
			`<p>a</p><hr/><p>b</p>`, `<p>a</p><hr class="h"/><p>b</p>`},
	} {
		plain, styled := styledChapter(t, test.css, test.plain), styledChapter(t, test.css, test.styled)
		if !reflect.DeepEqual(plain.Blocks, styled.Blocks) {
			t.Errorf("%s:\n%+v\nwant\n%+v", test.name, styled.Blocks, plain.Blocks)
		}
	}
	doc := styledChapter(t, `.b { display: block }`, `<p>a<br class="b"/>b</p><hr class="b"/>`)
	if blocks := doc.Blocks; len(blocks) != 3 || blocks[1].MarginTop != 0 || blocks[2].Kind != document.Rule {
		t.Errorf("a block <br> continues its paragraph, a block <hr> is a rule: %+v", blocks)
	}
}

func TestLists(t *testing.T) {
	doc := styledChapter(t, `ol.roman { list-style-type: upper-roman } ul.plain { list-style: none } .alpha li { list-style-type: lower-alpha }`,
		`<ol start="3"><li>c</li><li>d</li></ol><ol class="roman"><li>one</li><li>two</li><li>three</li><li>four</li></ol>
<ul class="plain"><li>none</li></ul><ul><li><p>nested</p><ul><li>deep</li></ul></li></ul><ol class="alpha"><li>a</li><li>b</li></ol>`)
	want := []string{"3. c", "4. d", "I. one", "II. two", "III. three", "IV. four", "none", "• nested", "• deep", "a. a", "b. b"}
	if got := texts(doc); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("lists\n%q\nwant\n%q", got, want)
	}
	if roman(1994) != "MCMXCIV" || alphabetic(28) != "AB" {
		t.Errorf("roman(1994) = %s, alphabetic(28) = %s", roman(1994), alphabetic(28))
	}
}

// Stylesheets from links (if their media match), <style>, @import (cycles end) and @media.
func TestStylesheets(t *testing.T) {
	book := openPackage(t, "", `<dc:language>fr</dc:language>`,
		`<item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>`,
		`><itemref idref="one"/><itemref idref="two"/>`, map[string]string{
			"OEBPS/css/main.css":       `@import url("parts/base.css"); .b { font-weight: bold } @media (min-width: 2000px) { .b { font-style: italic } }`,
			"OEBPS/css/parts/base.css": `@import "../main.css"; .i { font-style: italic } .b { font-weight: normal }`,
			"OEBPS/css/print.css":      `p { text-align: center }`,
			"OEBPS/text/one.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml" lang="de"><head>
<link rel="stylesheet" href="../css/main.css"/><link rel="stylesheet" media="print" href="../css/print.css"/>
<link rel="alternate stylesheet" href="../css/print.css"/>
<style>.u { text-decoration: underline }</style><style media="print">.u { font-weight: bold }</style></head>
<body><p class="b">bold</p><p class="i">italic</p><p class="u">under</p></body></html>`,
			"OEBPS/text/two.xhtml": xhtml(`<p>Zwei</p>`),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	styles := []document.Style{document.Bold, document.Italic, document.Underline}
	for index, want := range styles {
		if block := doc.Blocks[index]; block.Runs[0].Style != want || block.Align != document.AlignDefault {
			t.Errorf("block %d: style %v, align %v; want %v, the default", index, block.Runs[0].Style, block.Align, want)
		}
	}
	if doc.Language != "de" {
		t.Errorf("language %q, want the html element's", doc.Language)
	}
	two, err := book.Chapter(1)
	if err != nil {
		t.Fatal(err)
	}
	if two.Language != "fr" {
		t.Errorf("language %q, want the book's", two.Language)
	}
	if first, again := book.Stylesheet("OEBPS/css/main.css", screen), book.Stylesheet("OEBPS/css/main.css", screen); first == nil || first != again {
		t.Error("a stylesheet is parsed once")
	}
}

func TestEmbeddedFonts(t *testing.T) {
	const identifier = "urn:uuid:1234"
	obfuscated, err := webfont.Deobfuscate(gobold.TTF, webfont.IDPFAlgorithm, identifier)
	if err != nil {
		t.Fatal(err)
	}
	woff2, err := os.ReadFile("../../web/webfont/testdata/KaTeX_SansSerif-Regular.woff2")
	if err != nil {
		t.Fatal(err)
	}
	book := openStyled(t, `
@font-face { font-family: "My Font"; src: url(fonts/my.eot) format("embedded-opentype"), url(fonts/bold.ttf) format("truetype"); font-weight: 700 }
@font-face { font-family: "My Font"; src: url(fonts/sans.woff2?v=1); }
@font-face { font-family: Broken; src: url(fonts/broken.otf) }
p { font-family: "My Font", serif } .broken { font-family: Broken, sans-serif }`,
		`<p>Text <span class="broken">fallback</span></p>`,
		map[string]string{
			"OEBPS/fonts/bold.ttf":    string(obfuscated),
			"OEBPS/fonts/sans.woff2":  string(woff2),
			"OEBPS/fonts/broken.otf":  "not a font",
			"META-INF/encryption.xml": fmt.Sprintf(`<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#"><enc:EncryptedData><enc:EncryptionMethod Algorithm=%q/><enc:CipherData><enc:CipherReference URI="OEBPS/fonts/bold.ttf"/></enc:CipherData></enc:EncryptedData></encryption>`, webfont.IDPFAlgorithm),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	styles := doc.Fonts["my font"]
	if len(doc.Fonts) != 1 || styles[fonts.Bold] == nil || styles[fonts.Regular] == nil {
		t.Fatalf("fonts %v, want my font in bold and regular", doc.Fonts)
	}
	face, err := styles[fonts.Bold].Open(20)
	if err != nil || !face.Has('A') {
		t.Errorf("the de-obfuscated font: %v", err)
	}
	if run := runOf(t, doc, "Text"); run.Family != "my font" {
		t.Errorf("family %q, want the embedded font", run.Family)
	}
	if run := runOf(t, doc, "fallback"); run.Family != "sans-serif" {
		t.Errorf("family %q, want the generic family after a font that failed", run.Family)
	}
	if again, _ := book.Chapter(0); again.Fonts["my font"][fonts.Bold] != styles[fonts.Bold] {
		t.Error("a font is loaded once")
	}
}

func TestAdobeFontWithoutUUID(t *testing.T) {
	// The book's identifier, urn:uuid:1234, is not the UUID the font was obfuscated with.
	obfuscated, err := webfont.Deobfuscate(gobold.TTF, webfont.AdobeAlgorithm, "urn:uuid:a1b0d67e-2e81-4df5-9e67-a64cbe366809")
	if err != nil {
		t.Fatal(err)
	}
	book := openStyled(t, `@font-face { font-family: Lost; src: url(fonts/lost.ttf) } p { font-family: Lost }`, `<p>Text</p>`,
		map[string]string{
			"OEBPS/fonts/lost.ttf":    string(obfuscated),
			"META-INF/encryption.xml": fmt.Sprintf(`<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#"><enc:EncryptedData><enc:EncryptionMethod Algorithm=%q/><enc:CipherData><enc:CipherReference URI="OEBPS/fonts/lost.ttf"/></enc:CipherData></enc:EncryptedData></encryption>`, webfont.AdobeAlgorithm),
		})
	doc, err := book.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Fonts["lost"][fonts.Regular] == nil {
		t.Fatalf("fonts %v, want the font with its key recovered", doc.Fonts)
	}
}

// The text properties of CSS Text 3 and 4, with their -epub- and -webkit- prefixes: an
// unprefixed declaration wins.
func TestTextProperties(t *testing.T) {
	doc := styledChapter(t, `
.last { text-align: justify; text-align-last: center }
.lastjustify { -epub-text-align-last: justify }
.under { text-decoration: underline; -webkit-text-underline-position: under }
.dots { -epub-text-emphasis-style: filled dot; text-emphasis-position: under right }
.sesame { text-emphasis: open sesame red }
.string { -webkit-text-emphasis: "x" }
.none { text-emphasis-style: none }
.both { -epub-word-break: break-all; word-break: keep-all; -webkit-line-break: strict }
.anywhere { line-break: anywhere; word-break: break-all }
.rtl { direction: rtl; text-align: start }
.end { text-align: end }`,
		`<p class="last">last</p><p class="lastjustify">justified</p><p><span class="under">under</span>
<span class="dots">dots <span class="none">none</span></span> <span class="sesame">sesame</span> <span class="string">string</span></p>
<p class="both">both</p><p class="anywhere">anywhere</p><p class="rtl">rtl <span class="end">end</span></p>
<div dir="rtl"><p>attribute</p><p dir="ltr" class="end">ltr</p></div>`)
	for _, test := range []struct {
		text string
		want document.Run
	}{
		{"under", document.Run{Style: document.Underline | document.UnderlineUnder}},
		{"dots", document.Run{Emphasis: document.Emphasis{Mark: '•', Under: true}}},
		{"none", document.Run{Emphasis: document.Emphasis{Under: true}}},
		{"sesame", document.Run{Emphasis: document.Emphasis{Mark: '﹆'}}},
		{"string", document.Run{Emphasis: document.Emphasis{Mark: 'x'}}},
	} {
		got := runOf(t, doc, test.text)
		got.Text = ""
		if got != test.want {
			t.Errorf("%s: %+v, want %+v", test.text, got, test.want)
		}
	}
	blocks := map[string]document.Block{}
	for _, block := range doc.Blocks {
		blocks[strings.TrimSpace(block.Runs[0].Text)] = block
	}
	for _, test := range []struct {
		text string
		want func(block document.Block) bool
	}{
		{"last", func(block document.Block) bool {
			return block.Align == document.AlignDefault && block.AlignLast == document.AlignCenter
		}},
		{"justified", func(block document.Block) bool { return block.AlignLast == document.AlignJustify }},
		{"both", func(block document.Block) bool {
			return block.WordBreak == document.KeepAll && block.LineBreak == document.LineBreakStrict
		}},
		{"anywhere", func(block document.Block) bool {
			return block.WordBreak == document.BreakAll && block.LineBreak == document.LineBreakAnywhere
		}},
		{"rtl end", func(block document.Block) bool {
			return block.Direction == document.RightToLeft && block.Align == document.AlignStart
		}},
		{"attribute", func(block document.Block) bool { return block.Direction == document.RightToLeft }},
		{"ltr", func(block document.Block) bool {
			return block.Direction == document.LeftToRight && block.Align == document.AlignEnd
		}},
	} {
		if block, ok := blocks[test.text]; !ok || !test.want(block) {
			t.Errorf("%s: %+v", test.text, block)
		}
	}
}

// dir, <bdi>, <bdo>, direction and unicode-bidi set a block's direction and a run's isolate;
// lang, a run's language when it is not the document's.
func TestBidiMarkup(t *testing.T) {
	doc := styledChapter(t, `
.embed { direction: rtl; unicode-bidi: embed } .plain { unicode-bidi: plaintext }
.override { unicode-bidi: bidi-override } .direction { direction: rtl }`,
		`<p dir="rtl">rtl</p><p dir="AUTO">auto</p><p class="plain">plaintext</p>
<p><span dir="rtl">isolated <b>bold</b></span> <bdi>bdi</bdi> <bdo dir="rtl">bdo</bdo> <span class="embed">embed</span>
<span class="override">override</span> <span class="direction">direction</span> <span lang="ar">arabic</span> <span dir="auto">automatic</span></p>`)
	for _, test := range []struct {
		text string
		want document.Run
	}{
		{"isolated", document.Run{Isolate: document.IsolateRightToLeft}},
		{"bold", document.Run{Isolate: document.IsolateRightToLeft, Style: document.Bold}},
		{"bdi", document.Run{Isolate: document.IsolateAuto}},
		{"bdo", document.Run{Isolate: document.OverrideRightToLeft}},
		{"embed", document.Run{Isolate: document.IsolateRightToLeft}},
		{"override", document.Run{Isolate: document.OverrideLeftToRight}},
		{"direction", document.Run{}},         // direction alone moves no inline text
		{"arabic", document.Run{Language: 1}}, // Languages[0]
		{"automatic", document.Run{Isolate: document.IsolateAuto}},
	} {
		got := runOf(t, doc, test.text)
		got.Text = ""
		if got != test.want {
			t.Errorf("%s: %+v, want %+v", test.text, got, test.want)
		}
	}
	if len(doc.Languages) != 1 || doc.Languages[0] != "ar" {
		t.Errorf("languages %q, want ar", doc.Languages)
	}
	// The spaces around an isolate are outside it, where their whitespace is.
	for _, run := range doc.Blocks[3].Runs {
		if (run.Isolate == document.IsolateAuto || run.Isolate == document.OverrideRightToLeft) && strings.TrimSpace(run.Text) != run.Text {
			t.Errorf("isolate %q holds the space around it", run.Text)
		}
	}
	for index, want := range []document.Direction{document.RightToLeft, document.DirectionAuto, document.DirectionAuto, document.LeftToRight} {
		if got := doc.Blocks[index].Direction; got != want {
			t.Errorf("block %d %q: direction %d, want %d", index, doc.Blocks[index].Runs[0].Text, got, want)
		}
	}
}

// emphasisMarkOf reads the styles of text-emphasis-style, and nothing else.
func TestEmphasisMarkOf(t *testing.T) {
	for _, test := range []struct {
		value string
		mark  rune
		given bool
	}{
		{"none", 0, true},
		{"filled", '●', true},
		{"open", '○', true},
		{"triangle", '▲', true},
		{"open double-circle", '◎', true},
		{"Dot Filled", '•', true},
		{`"*"`, '*', true},
		{`''`, 0, true},
		{"red", 0, false},
		{"", 0, false},
	} {
		if mark, given := emphasisMarkOf(test.value); mark != test.mark || given != test.given {
			t.Errorf("%q: %q, %v; want %q, %v", test.value, mark, given, test.mark, test.given)
		}
	}
}

// FuzzEmphasisMarkOf checks that no value makes emphasisMarkOf panic.
func FuzzEmphasisMarkOf(f *testing.F) {
	for _, value := range []string{"none", "filled sesame", `"x"`, `'`, `"`, "open red"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		emphasisMarkOf(value)
	})
}

// BenchmarkChapter lays out a chapter of 5,000 paragraphs styled by a stylesheet of 200 rules.
func BenchmarkChapter(b *testing.B) {
	var css, body strings.Builder
	for index := range 50 {
		fmt.Fprintf(&css, ".c%d { margin-top: %dpx; font-size: 1.%dem }\n", index, index, index)
		fmt.Fprintf(&css, "div.s%d > p { text-indent: 1em }\n", index)
		fmt.Fprintf(&css, "#id%d p span { font-weight: bold }\n", index)
		fmt.Fprintf(&css, "p + p.c%d, h2 ~ p.c%d { font-style: italic !important }\n", index, index)
	}
	for section := range 50 {
		fmt.Fprintf(&body, `<div class="s%d" id="id%d"><h2 class="c%d">Section %d</h2>`, section, section, section, section)
		for paragraph := range 100 {
			fmt.Fprintf(&body, `<p class="c%d">Paragraph %d of the section, <span class="c%d">with a span</span> and <em>emphasis</em> in it.</p>`,
				paragraph%60, paragraph, paragraph%50)
		}
		body.WriteString("</div>")
	}
	book := openStyled(b, css.String(), body.String(), nil)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := book.Chapter(0); err != nil {
			b.Fatal(err)
		}
	}
}
