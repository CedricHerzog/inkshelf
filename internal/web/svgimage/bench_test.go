package svgimage

import (
	"fmt"
	"image"
	"strings"
	"testing"
)

// benchmarkSVG returns an illustration as books carry them: a stylesheet, gradients, groups
// with transforms, a few dozen curved paths and labels in text.
func benchmarkSVG() []byte {
	var markup strings.Builder
	markup.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 600" width="800" height="600">
<style>.label { font: 14px sans-serif; fill: #333 } .axis { stroke: black; stroke-width: 2; fill: none }</style>
<defs>
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#eee"/><stop offset="1" stop-color="#888"/></linearGradient>
<radialGradient id="sun" cx="0.5" cy="0.5" r="0.5"><stop offset="0" stop-color="white"/><stop offset="1" stop-color="#444"/></radialGradient>
</defs>
<rect width="800" height="600" fill="url(#sky)"/>
<circle cx="650" cy="120" r="70" fill="url(#sun)"/>
<path class="axis" d="M 50 550 L 750 550 M 50 550 L 50 50"/>
`)
	for index := range 40 {
		fmt.Fprintf(&markup, `<g transform="translate(%d %d) rotate(%d) scale(1.2)">`, 60+index*17, 300+(index%7)*30, index*9)
		fmt.Fprintf(&markup, `<path d="M 0 0 C 20 -40, 60 -40, 80 0 S 140 40, 160 0 Q 120 60 80 50 Z" fill="#%02x%02x%02x" stroke="black" stroke-width="1.5"/>`,
			40+index*5, 80+index*3, 200-index*4)
		markup.WriteString(`<ellipse cx="40" cy="10" rx="12" ry="6" fill="url(#sun)" opacity="0.7"/></g>`)
	}
	for index := range 10 {
		fmt.Fprintf(&markup, `<text class="label" x="%d" y="575" text-anchor="middle">Année %d</text>`, 80+index*70, 1990+index*3)
	}
	markup.WriteString(`<g transform="rotate(-90 25 300)"><text class="label" x="25" y="300">Production <tspan font-weight="bold">(tonnes)</tspan></text></g>`)
	markup.WriteString(`</svg>`)
	return []byte(markup.String())
}

// Rasterize an illustration half the InkPad 3's screen wide, its text drawn as boxes: the cost
// of an SVG figure in a book, all but the glyphs. Gradient fills dominate it.
func BenchmarkRasterize(b *testing.B) {
	svg := benchmarkSVG()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Rasterize(svg, image.Pt(702, 527), Options{Text: &fakeText{}}); err != nil {
			b.Fatal(err)
		}
	}
}

// Size reads the root element only: pagination calls it for every SVG image of a chapter.
func BenchmarkSize(b *testing.B) {
	svg := benchmarkSVG()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Size(svg); err != nil {
			b.Fatal(err)
		}
	}
}
