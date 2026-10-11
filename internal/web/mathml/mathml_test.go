package mathml

import (
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/graphics/fonts"
)

// testSize is the size of the text around the formulas of the tests, in pixels.
const testSize = 40

// testFonts are Go's fonts, off the reader, opened as asked.
type testFonts struct {
	tb    testing.TB
	fonts map[[2]int]*fonts.Font
}

func newTestFonts(tb testing.TB) *testFonts {
	set := &testFonts{tb: tb, fonts: map[[2]int]*fonts.Font{}}
	tb.Cleanup(func() {
		for _, opened := range set.fonts {
			opened.Close()
		}
	})
	return set
}

func (set *testFonts) font(variant Variant, size int) *fonts.Font {
	key := [2]int{int(variant), size}
	if opened, ok := set.fonts[key]; ok {
		return opened
	}
	style := fonts.Regular
	switch variant {
	case Bold:
		style = fonts.Bold
	case Italic:
		style = fonts.Italic
	case Bold | Italic:
		style = fonts.BoldItalic
	}
	opened, err := fonts.Open(fonts.Serif, style, size)
	if err != nil {
		set.tb.Fatal(err)
	}
	set.fonts[key] = opened
	return opened
}

func (set *testFonts) Measure(text string, variant Variant, size int) (width int, ink image.Rectangle) {
	font := set.font(variant, size)
	return font.Width(text), font.Ink(text)
}

func (set *testFonts) Draw(dst *image.Gray, text string, variant Variant, size int, origin image.Point) {
	font := set.font(variant, size)
	font.DrawTo(dst, origin.Sub(image.Pt(0, font.Ascent())), text, color.Gray{Y: black})
}

// layOut parses markup and lays it out at testSize.
func layOut(t *testing.T, markup string) *Formula {
	t.Helper()
	root, err := Parse(markup)
	if err != nil {
		t.Fatalf("%s: %v", markup, err)
	}
	return Layout(root, newTestFonts(t), testSize)
}

// textItems returns the texts a formula draws, by text: the last one of each.
func textItems(formula *Formula) map[string]item {
	found := map[string]item{}
	for _, placed := range formula.items {
		if placed.kind == textItem || placed.kind == stretchItem {
			found[placed.text] = placed
		}
	}
	return found
}

// count returns how many items of kind a formula draws.
func count(formula *Formula, kind itemKind) int {
	total := 0
	for _, placed := range formula.items {
		if placed.kind == kind {
			total++
		}
	}
	return total
}

func TestParse(t *testing.T) {
	decoder := xml.NewDecoder(strings.NewReader(`<m:math xmlns:m="http://www.w3.org/1998/Math/MathML" display="block" class="x">
<m:mi mathvariant="bold"> x </m:mi><m:mo>&amp;</m:mo><m:mtext>a  b</m:mtext><m:annotation encoding="TeX">x</m:annotation></m:math><p>after</p>`))
	root, err := Read(decoder)
	if err != nil {
		t.Fatal(err)
	}
	want := `<math display="block"><mi mathvariant="bold">x</mi><mo>&amp;</mo><mtext>a b</mtext><annotation></annotation></math>`
	if markup := root.Markup(); markup != want {
		t.Errorf("markup %s, want %s", markup, want)
	}
	if !root.Display() {
		t.Errorf("not displayed as a block")
	}
	// Read stops at the end of the formula.
	if token, _ := decoder.Token(); token.(xml.StartElement).Name.Local != "p" {
		t.Errorf("next token %v, want <p>", token)
	}
	again, err := FromPath(Path(root))
	if err != nil || again.Markup() != want {
		t.Errorf("from its path: %v, %v", again, err)
	}
	// MathML's named characters that HTML lacks, left in the text by a lenient decoder.
	lenient := xml.NewDecoder(strings.NewReader(`<math><mi>f</mi><mo>&ApplyFunction;</mo><mi>x</mi></math>`))
	lenient.Strict = false
	if root, err := Read(lenient); err != nil || root.Children[1].Text != "\u2061" {
		t.Errorf("entity: %v, %v", root, err)
	}
	for _, markup := range []string{
		"<svg></svg>", "<math><mi>x</mi>", "",
	} {
		if _, err := Parse(markup); err == nil {
			t.Errorf("%q: no error", markup)
		}
	}
	for _, markup := range []string{
		"<math>" + strings.Repeat("<mrow>", maxDepth) + strings.Repeat("</mrow>", maxDepth) + "</math>",
		"<math>" + strings.Repeat("<mi>x</mi>", maxNodes) + "</math>",
		"<math><mtext>" + strings.Repeat("x", maxText+1) + "</mtext></math>",
	} {
		if _, err := Parse(markup); !errors.Is(err, ErrLimit) {
			t.Errorf("%.40q…: %v, want ErrLimit", markup, err)
		}
	}
}

// A lone letter is in italics, a word or a number upright; the letters of Unicode's
// mathematical alphabets are plain ones in their variant.
func TestTokens(t *testing.T) {
	formula := layOut(t, `<math><mi>x</mi><mi>sin</mi><mn>2</mn><mi mathvariant="normal">d</mi><mi>𝐁</mi><mi mathvariant="double-struck">R</mi></math>`)
	items := textItems(formula)
	for text, want := range map[string]Variant{"x": Italic, "sin": 0, "2": 0, "d": 0, "B": Bold, "ℝ": 0} {
		if placed, ok := items[text]; !ok || placed.variant != want {
			t.Errorf("%s: %+v, want variant %d", text, placed, want)
		}
	}
	if items["sin"].at.X <= items["x"].at.X || formula.Ascent <= 0 || formula.Width <= 0 {
		t.Errorf("tokens not in a row: %+v", items)
	}
	// Operators are spaced: relations more than binary operators, a sign not at all.
	spaced := func(markup string) int { return layOut(t, markup).Width }
	plain := spaced(`<math><mi>a</mi><mi>b</mi></math>`)
	equals, plus, minus := spaced(`<math><mi>a</mi><mo>=</mo><mi>b</mi></math>`), spaced(`<math><mi>a</mi><mo>+</mo><mi>b</mi></math>`), spaced(`<math><mo>−</mo><mi>b</mi></math>`)
	width, _ := newTestFonts(t).Measure("=", 0, testSize)
	minusWidth, _ := newTestFonts(t).Measure("−", 0, testSize)
	if equals-plain-width != 10*testSize/18 || plus <= plain || minus > minusWidth+plain/2+1 {
		t.Errorf("widths: a b %d, a = b %d (= is %d), a + b %d, −b %d", plain, equals, width, plus, minus)
	}
}

// A fraction stands taller than text, its numerator over the bar, its denominator under; a
// fraction in a line has smaller parts.
func TestFraction(t *testing.T) {
	letter := layOut(t, `<math><mi>x</mi></math>`)
	fraction := layOut(t, `<math><mfrac><mi>a</mi><mi>b</mi></mfrac></math>`)
	if fraction.Ascent <= letter.Ascent || fraction.Descent <= testSize/4 {
		t.Errorf("fraction %d/%d, letter %d/%d: want it taller, below the baseline", fraction.Ascent, fraction.Descent, letter.Ascent, letter.Descent)
	}
	items := textItems(fraction)
	var bar image.Rectangle
	for _, placed := range fraction.items {
		if placed.kind == ruleItem {
			bar = placed.rect
		}
	}
	if bar.Empty() || items["a"].at.Y >= bar.Min.Y || items["b"].at.Y <= bar.Max.Y || bar.Max.Y > 0 {
		t.Errorf("a at %v, b at %v, bar %v: want a over the bar, b under it, the bar above the baseline", items["a"].at, items["b"].at, bar)
	}
	if items["a"].size >= testSize {
		t.Errorf("numerator at %d pixels, want smaller in a line", items["a"].size)
	}
	display := layOut(t, `<math display="block"><mfrac><mi>a</mi><mi>b</mi></mfrac></math>`)
	if textItems(display)["a"].size != testSize || display.Ascent <= fraction.Ascent {
		t.Errorf("display fraction: %+v, ascent %d", textItems(display)["a"], display.Ascent)
	}
	// linethickness 0: a binomial coefficient, no bar.
	if binomial := layOut(t, `<math><mfrac linethickness="0"><mi>n</mi><mi>k</mi></mfrac></math>`); count(binomial, ruleItem) != 0 {
		t.Errorf("a bar of thickness 0")
	}
}

// Scripts are raised and lowered, smaller, down to half the size.
func TestScripts(t *testing.T) {
	formula := layOut(t, `<math><msubsup><mi>x</mi><mi>i</mi><mn>2</mn></msubsup></math>`)
	items := textItems(formula)
	if items["2"].at.Y >= -testSize/4 || items["i"].at.Y <= 0 || items["2"].at.X <= items["x"].at.X {
		t.Errorf("x at %v, sub i at %v, sup 2 at %v: want 2 raised, i lowered, after x", items["x"].at, items["i"].at, items["2"].at)
	}
	if items["2"].size != testSize*71/100 {
		t.Errorf("script at %d pixels, want %d", items["2"].size, testSize*71/100)
	}
	nested := layOut(t, `<math><msup><mi>e</mi><msup><mi>x</mi><msup><mi>y</mi><mi>z</mi></msup></msup></msup></math>`)
	if size := textItems(nested)["z"].size; size != testSize/2 {
		t.Errorf("third script level at %d pixels, want the minimum, %d", size, testSize/2)
	}
	// A superscript on a tall base follows its top.
	tall := layOut(t, `<math display="block"><msup><mrow><mfrac><mi>a</mi><mi>b</mi></mfrac></mrow><mn>2</mn></msup></math>`)
	if textItems(tall)["2"].at.Y >= items["2"].at.Y {
		t.Errorf("superscript of a fraction at %v, of a letter at %v: want it higher", textItems(tall)["2"].at, items["2"].at)
	}
}

// Fences stretch to their row, roots and enclosures draw around their content.
func TestStretchAndRoots(t *testing.T) {
	fenced := layOut(t, `<math display="block"><mo>(</mo><mfrac><mi>a</mi><mi>b</mi></mfrac><mo>)</mo></math>`)
	open := textItems(fenced)["("]
	if open.kind != stretchItem || !open.vertical || open.rect.Dy() < fenced.Ascent+fenced.Descent-2 {
		t.Errorf("fence %+v, formula %d/%d: want it stretched to the fraction", open, fenced.Ascent, fenced.Descent)
	}
	if plain := textItems(layOut(t, `<math><mo>(</mo><mi>a</mi><mo>)</mo></math>`))["("]; plain.kind != textItem {
		t.Errorf("fence around a letter %+v, want a glyph", plain)
	}
	if mfenced := textItems(layOut(t, `<math display="block"><mfenced><mfrac><mi>a</mi><mi>b</mi></mfrac><mi>c</mi></mfenced></math>`)); mfenced[")"].kind != stretchItem || mfenced[","].text != "," {
		t.Errorf("mfenced: %+v", mfenced)
	}
	letter := layOut(t, `<math><mi>x</mi></math>`)
	root := layOut(t, `<math><msqrt><mi>x</mi></msqrt></math>`)
	if count(root, strokeItem) != 3 || count(root, ruleItem) != 1 || root.Ascent <= letter.Ascent || root.Width <= letter.Width+testSize/3 {
		t.Errorf("square root %d/%d, %d wide: want a sign and a bar over x", root.Ascent, root.Descent, root.Width)
	}
	cube := layOut(t, `<math><mroot><mi>x</mi><mn>3</mn></mroot></math>`)
	if index := textItems(cube)["3"]; index.size >= testSize*71/100 || index.at.Y >= 0 {
		t.Errorf("index %+v, want small and raised", index)
	}
	if boxed := layOut(t, `<math><menclose notation="box updiagonalstrike"><mi>x</mi></menclose></math>`); count(boxed, ruleItem) != 4 || count(boxed, strokeItem) != 1 {
		t.Errorf("enclosed: %d rules, %d strokes", count(boxed, ruleItem), count(boxed, strokeItem))
	}
}

// A sum's limits go under and over it in display style, as scripts after it in a line.
func TestLimits(t *testing.T) {
	sum := `<munderover><mo>∑</mo><mrow><mi>i</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover><mi>i</mi>`
	display := textItems(layOut(t, `<math display="block">`+sum+`</math>`))
	if display["n"].at.Y >= display["∑"].at.Y-testSize/2 || display["1"].at.Y <= display["∑"].at.Y || display["∑"].size <= testSize {
		t.Errorf("display: ∑ %+v, n %v, 1 %v: want n over a larger ∑, 1 under it", display["∑"], display["n"].at, display["1"].at)
	}
	inline := textItems(layOut(t, `<math>`+sum+`</math>`))
	if inline["n"].at.X <= inline["∑"].at.X || inline["1"].at.X <= inline["∑"].at.X {
		t.Errorf("in a line: ∑ at %v, n at %v, 1 at %v: want the limits after it", inline["∑"].at, inline["n"].at, inline["1"].at)
	}
	// An accent sits close over its base, at its size; a bar stretches over it as a rule.
	hat := textItems(layOut(t, `<math><mover><mi>x</mi><mo>^</mo></mover></math>`))
	if hat["^"].size != testSize || hat["^"].at.Y >= 0 {
		t.Errorf("accent %+v", hat["^"])
	}
	if bar := layOut(t, `<math><mover><mrow><mi>a</mi><mi>b</mi></mrow><mo>¯</mo></mover></math>`); count(bar, ruleItem) != 1 {
		t.Errorf("overbar: %d rules", count(bar, ruleItem))
	}
}

// A matrix's cells line up in columns and rows, centred on the axis.
func TestTable(t *testing.T) {
	formula := layOut(t, `<math><mtable columnalign="left right">
<mtr><mtd><mi>a</mi></mtd><mtd><mn>1</mn></mtd></mtr>
<mtr><mtd><mi>bbbb</mi></mtd><mtd><mn>222</mn></mtd></mtr></mtable></math>`)
	items := textItems(formula)
	if items["a"].at.X != items["bbbb"].at.X || items["a"].at.Y >= items["bbbb"].at.Y {
		t.Errorf("first column: a at %v, bbbb at %v: want them left-aligned, a above", items["a"].at, items["bbbb"].at)
	}
	set := newTestFonts(t)
	oneWidth, _ := set.Measure("1", 0, testSize)
	twoWidth, _ := set.Measure("222", 0, testSize)
	if items["1"].at.X+oneWidth != items["222"].at.X+twoWidth || items["1"].at.X <= items["a"].at.X {
		t.Errorf("second column: 1 at %v, 222 at %v: want them right-aligned", items["1"].at, items["222"].at)
	}
	if middle := (formula.Descent - formula.Ascent) / 2; middle > 0 || middle < -testSize/2 {
		t.Errorf("table %d/%d: want it centred above the baseline", formula.Ascent, formula.Descent)
	}
}

// What a formula draws is inside its box.
func TestImage(t *testing.T) {
	for _, markup := range []string{
		`<math><mi>x</mi></math>`,
		`<math display="block"><mrow><mo>(</mo><mtable><mtr><mtd><mfrac><mn>1</mn><mn>2</mn></mfrac></mtd><mtd><mn>0</mn></mtd></mtr><mtr><mtd><mn>0</mn></mtd><mtd><msqrt><mi>y</mi></msqrt></mtd></mtr></mtable><mo>)</mo></mrow></math>`,
		`<math><mroot><mrow><mi>x</mi><mo>+</mo><mn>1</mn></mrow><mn>3</mn></mroot></math>`,
		`<math><munder><mrow><mi>a</mi><mi>b</mi></mrow><mo>⏟</mo></munder></math>`,
	} {
		formula := layOut(t, markup)
		drawn := formula.Image()
		if drawn.Bounds().Dy() != formula.Ascent+formula.Descent || drawn.Bounds().Dx() != formula.Width {
			t.Errorf("%s: image %v, formula %d×%d+%d", markup, drawn.Bounds(), formula.Width, formula.Ascent, formula.Descent)
		}
		// Larger, then the ink in the middle: none spills.
		margin := testSize
		larger := image.NewGray(image.Rect(0, 0, formula.Width+2*margin, formula.Ascent+formula.Descent+2*margin))
		for index := range larger.Pix {
			larger.Pix[index] = white
		}
		formula.Draw(larger, image.Pt(margin, margin+formula.Ascent))
		ink := inkBounds(larger)
		if ink.Empty() || !ink.In(image.Rect(margin-2, margin-2, margin+formula.Width+2, margin+formula.Ascent+formula.Descent+2)) {
			t.Errorf("%s: ink in %v, box %v", markup, ink.Sub(image.Pt(margin, margin)), drawn.Bounds())
		}
	}
}
