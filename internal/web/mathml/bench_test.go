package mathml

import (
	"image"
	"testing"
)

// benchmarkMarkup is a display formula as a textbook writes one: the quadratic formula, a sum
// with limits, a cube root and a two by two matrix in stretched parentheses.
const benchmarkMarkup = `<math display="block"><mrow>
<mi>x</mi><mo>=</mo>
<mfrac><mrow><mo>−</mo><mi>b</mi><mo>±</mo><msqrt><mrow><msup><mi>b</mi><mn>2</mn></msup><mo>−</mo><mn>4</mn><mi>a</mi><mi>c</mi></mrow></msqrt></mrow>
<mrow><mn>2</mn><mi>a</mi></mrow></mfrac>
<mo>+</mo>
<munderover><mo>∑</mo><mrow><mi>k</mi><mo>=</mo><mn>0</mn></mrow><mi>n</mi></munderover>
<msubsup><mi>a</mi><mi>k</mi><mn>2</mn></msubsup>
<mo>+</mo>
<mroot><mrow><mi>y</mi><mo>+</mo><mn>1</mn></mrow><mn>3</mn></mroot>
<mo>⋅</mo>
<mrow><mo>(</mo><mtable>
<mtr><mtd><mfrac><mn>1</mn><mn>2</mn></mfrac></mtd><mtd><msub><mi>x</mi><mn>12</mn></msub></mtd></mtr>
<mtr><mtd><msup><mi>e</mi><mrow><mi>i</mi><mi>θ</mi></mrow></msup></mtd><mtd><msqrt><mi>z</mi></msqrt></mtd></mtr>
</mtable><mo>)</mo></mrow>
</mrow></math>`

// Parse the formula's markup into its tree, as a chapter with formulas does once per formula.
func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(benchmarkMarkup); err != nil {
			b.Fatal(err)
		}
	}
}

// Lay the parsed formula out at the reader's text size, its fonts already open.
func BenchmarkLayout(b *testing.B) {
	root, err := Parse(benchmarkMarkup)
	if err != nil {
		b.Fatal(err)
	}
	set := newTestFonts(b)
	Layout(root, set, 44) // opens the fonts outside the loop
	b.ReportAllocs()
	for b.Loop() {
		Layout(root, set, 44)
	}
}

// Draw the laid out formula onto a canvas cleared to white each time, as a page draws it.
func BenchmarkDraw(b *testing.B) {
	root, err := Parse(benchmarkMarkup)
	if err != nil {
		b.Fatal(err)
	}
	formula := Layout(root, newTestFonts(b), 44)
	canvas := image.NewGray(image.Rect(0, 0, formula.Width, formula.Ascent+formula.Descent))
	b.ReportAllocs()
	for b.Loop() {
		for index := range canvas.Pix {
			canvas.Pix[index] = white
		}
		formula.Draw(canvas, image.Pt(0, formula.Ascent))
	}
}
