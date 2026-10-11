package mathml

import (
	"encoding/xml"
	"image"
	"strings"
	"testing"
)

// fuzzSeeds are formulas of every element, and some broken ones.
var fuzzSeeds = []string{
	`<math><mi>x</mi><mo>=</mo><mfrac><mrow><mo>−</mo><mi>b</mi><mo>±</mo><msqrt><msup><mi>b</mi><mn>2</mn></msup><mo>−</mo><mn>4</mn><mi>a</mi><mi>c</mi></msqrt></mrow><mrow><mn>2</mn><mi>a</mi></mrow></mfrac></math>`,
	`<math display="block"><munderover><mo>∑</mo><mrow><mi>i</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover><msubsup><mi>x</mi><mi>i</mi><mn>2</mn></msubsup></math>`,
	`<math><mrow><mo>(</mo><mtable rowlines="solid" columnlines="solid" frame="solid"><mtr><mtd><mn>1</mn></mtd><mtd><mn>0</mn></mtd></mtr><mlabeledtr><mtd><mtext>(1)</mtext></mtd><mtd><mn>0</mn></mtd></mlabeledtr></mtable><mo>)</mo></mrow></math>`,
	`<math><mroot><mi>x</mi><mn>3</mn></mroot><mfenced open="[" close="]" separators=";"><mi>a</mi><mi>b</mi></mfenced><mspace width="2em" height="1ex" depth="-3px"/></math>`,
	`<math><mstyle scriptlevel="-9" mathsize="900%" displaystyle="true"><mpadded width="+2width" height="-50%" lspace="1em" voffset="-3em"><mphantom><mi>x</mi></mphantom></mpadded></mstyle></math>`,
	`<math><menclose notation="box circle longdiv updiagonalstrike downdiagonalstrike verticalstrike horizontalstrike radical"><mi>x</mi></menclose><mmultiscripts><mi>R</mi><mi>i</mi><none/><mprescripts/><mn>1</mn></mmultiscripts></math>`,
	`<math><semantics><mover accent="true"><mi>x</mi><mo>^</mo></mover><annotation>x</annotation></semantics><munder><mi>ab</mi><mo>⏟</mo></munder><maction selection="7"><mi>a</mi></maction><ms>s</ms></math>`,
	`<math><mfrac linethickness="1000000em"><mi>a</mi></mfrac><msub/><msup><mi>x</mi></msup><mo lspace="-100em" stretchy="true">|</mo></math>`,
	`<math><mi>𝑥</mi><mi mathvariant="double-struck">Rz</mi><mo>&ApplyFunction;</mo>`,
	`<math><mover></mover><munderover><mo>∑</mo></munderover><mroot/><mfrac/></math>`,
	`<mAth><A:0></A>`,
	`<math><mi>x</mi></math>`,
}

// Any formula that reads lays out and draws, in its box and in an image of bounded size.
func FuzzMathML(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	fonts := newTestFonts(f)
	f.Fuzz(func(t *testing.T, markup string) {
		decoder := xml.NewDecoder(strings.NewReader(markup))
		decoder.Strict = false
		root, err := Read(decoder)
		if err != nil {
			return
		}
		again, err := Parse(root.Markup())
		if err != nil || again.Markup() != root.Markup() {
			t.Fatalf("markup %q does not read again: %v", root.Markup(), err)
		}
		formula := Layout(again, fonts, 20)
		if formula.Width < 0 || formula.Ascent < 0 || formula.Descent < 0 {
			t.Fatalf("formula %d×%d+%d", formula.Width, formula.Ascent, formula.Descent)
		}
		canvas := image.NewGray(image.Rect(0, 0, 200, 200))
		formula.Draw(canvas, image.Pt(10, 100))
		if formula.Width*(formula.Ascent+formula.Descent) <= 1<<20 {
			formula.Image()
		}
	})
}
