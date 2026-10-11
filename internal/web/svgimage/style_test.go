package svgimage

import (
	"image"
	"strings"
	"testing"
	"unicode/utf8"
)

// rasterizeGray draws svg at size with options, failing the test on an error.
func rasterizeGray(t *testing.T, svg string, size image.Point, options Options) *image.Gray {
	t.Helper()
	canvas, err := Rasterize([]byte(svg), size, options)
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	return canvas
}

// Each case is a 10×10 square filling a 10×10 SVG: its gray in the middle.
func TestStyles(t *testing.T) {
	for _, test := range []struct {
		name, svg string
		css       string
		want      uint8
	}{
		{"class rule", `<style>.dark { fill: #000 } rect { fill: white }</style><rect class="dark" width="10" height="10"/>`, "", 0},
		{"rule over attribute", `<style>rect { fill: #808080 }</style><rect fill="black" width="10" height="10"/>`, "", 128},
		{"style over rule", `<style>#square { fill: black }</style><rect id="square" style="fill: #808080" width="10" height="10"/>`, "", 128},
		{"important rule", `<style>rect { fill: #808080 !important }</style><rect style="fill: black" width="10" height="10"/>`, "", 128},
		{"CDATA and comments", `<defs><style type="text/css"><![CDATA[ /* a */ g > rect { fill: gray } ]]></style></defs><g><rect width="10" height="10"/></g>`, "", 128},
		{"rgba", `<rect fill="rgba(0, 0, 0, 0.5)" width="10" height="10"/>`, "", 128},
		{"currentColor", `<g color="#808080"><rect fill="currentColor" width="10" height="10"/></g>`, "", 128},
		{"currentColor by a rule", `<style>g { color: rgb(128 128 128) } rect { fill: currentColor }</style><g><rect width="10" height="10"/></g>`, "", 128},
		{"inherit", `<g fill="#808080"><rect fill="inherit" width="10" height="10"/></g>`, "", 128},
		{"bad value dropped", `<rect fill="nonsense" stroke-width="wide" opacity="half" width="10" height="10"/>`, "", 0},
		{"hsl", `<rect style="fill: hsl(0, 0%, 50%)" width="10" height="10"/>`, "", 128},
		{"display none", `<style>.gone { display: none }</style><rect class="gone" width="10" height="10"/>`, "", 255},
		{"visibility hidden", `<rect visibility="hidden" width="10" height="10"/>`, "", 255},
		{"extra CSS under the sheet", `<style>rect { fill: #808080 }</style><rect width="10" height="10"/>`, "rect { fill: white }", 128},
		{"extra CSS", `<rect width="10" height="10"/>`, "svg rect { fill: #808080 }", 128},
		{"document style", `<style>rect { stroke: none }</style><rect fill="black" ` + DocumentStyleAttribute + `="fill: #808080" width="10" height="10"/>`, "", 128},
		{"missing paint server", `<rect fill="url(#nothing) #808080" width="10" height="10"/>`, "", 128},
		{"missing paint server, no fallback", `<rect fill="url(#nothing)" width="10" height="10"/>`, "", 255},
		{"one-number scale", `<g transform="scale(2)"><rect width="5" height="5"/></g>`, "", 0},
		{"transform with commas", `<g transform="translate(10, 0), translate(-10 0)"><rect width="10" height="10"/></g>`, "", 0},
		{"clipPath not drawn", `<clipPath id="clip"><rect width="10" height="10"/></clipPath>`, "", 255},
	} {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` + test.svg + `</svg>`
		canvas := rasterizeGray(t, svg, image.Pt(10, 10), Options{CSS: test.css})
		if gray := canvas.GrayAt(5, 5).Y; int(gray)-int(test.want) > 2 || int(test.want)-int(gray) > 2 {
			t.Errorf("%s: gray %d, want %d", test.name, gray, test.want)
		}
	}
}

// A stop's color from a rule reaches oksvg, which reads it from attributes only.
func TestStopColorRule(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><style>stop { stop-color: #808080 }</style>` +
		`<linearGradient id="flat"><stop offset="0"/><stop offset="1"/></linearGradient>` +
		`<rect fill="url(#flat)" width="10" height="10"/></svg>`
	if gray := rasterizeGray(t, svg, image.Pt(10, 10), Options{}).GrayAt(5, 5).Y; gray < 126 || gray > 130 {
		t.Errorf("gray %d, want 128", gray)
	}
}

// fakeText measures each character as half the size wide and draws text as a box from the
// baseline up, size × 0.7 high.
type fakeText struct{ drawn []string }

func (fake *fakeText) Width(style TextStyle, text string) int {
	return utf8.RuneCountInString(text) * style.Size / 2
}

func (fake *fakeText) Draw(dst *image.Gray, style TextStyle, origin image.Point, text string) error {
	fake.drawn = append(fake.drawn, text)
	area := image.Rect(origin.X, origin.Y-style.Size*7/10, origin.X+fake.Width(style, text), origin.Y).Intersect(dst.Bounds())
	for row := area.Min.Y; row < area.Max.Y; row++ {
		for column := area.Min.X; column < area.Max.X; column++ {
			dst.Pix[dst.PixOffset(column, row)] = style.Gray
		}
	}
	return nil
}

func TestText(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50" width="200" height="100">
<style>.title { font: bold 20px serif; fill: #808080 }</style>
<text class="title" x="50" y="40" text-anchor="middle">
   AB  <tspan fill="black">CD</tspan>
</text>
<defs><text id="hidden">no</text></defs>
<g transform="translate(0 10)"><text x="0" y="0" font-size="10">E</text></g>
</svg>`
	fake := &fakeText{}
	canvas := rasterizeGray(t, svg, image.Pt(200, 100), Options{Text: fake})
	if got := strings.Join(fake.drawn, "|"); got != "AB |CD|E" {
		t.Errorf("drawn %q, want \"AB |CD|E\"", got)
	}
	// 20 user units at scale 2 is 40 px: "AB CD" is 5 × 20 = 100 px wide, centred on x 100,
	// so from 50 to 150, its baseline at y 80; "AB " gray, "CD" from 110 black.
	for _, check := range []struct {
		point image.Point
		want  uint8
	}{
		{image.Pt(55, 75), 128}, {image.Pt(105, 75), 128}, {image.Pt(115, 75), 0}, {image.Pt(145, 75), 0},
		{image.Pt(45, 75), 255}, {image.Pt(155, 75), 255}, {image.Pt(100, 85), 255},
		{image.Pt(5, 18), 0}, // E: 20 px at (0, 20)
	} {
		if gray := canvas.GrayAt(check.point.X, check.point.Y).Y; gray != check.want {
			t.Errorf("gray at %v = %d, want %d", check.point, gray, check.want)
		}
	}
}

func TestTextLeftOutWithoutDrawer(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><text x="0" y="10" font-size="10">A</text></svg>`
	canvas := rasterizeGray(t, svg, image.Pt(10, 10), Options{})
	if gray := canvas.GrayAt(2, 8).Y; gray != 255 {
		t.Errorf("gray %d, want white", gray)
	}
}

func TestParseTransform(t *testing.T) {
	for _, test := range []struct {
		value string
		want  matrix
		ok    bool
	}{
		{"translate(10)", matrix{a: 1, d: 1, e: 10}, true},
		{"scale(2) translate(1,1)", matrix{a: 2, d: 2, e: 2, f: 2}, true},
		{"matrix(1 0 0 1 -5.5-2)", matrix{a: 1, d: 1, e: -5.5, f: -2}, true},
		{"rotate(90)", matrix{a: 0, b: 1, c: -1, d: 0}, true},
		{"", identity, true},
		{"spin(3)", identity, false},
		{"translate(1", identity, false},
		{"scale(1 2 3)", identity, false},
	} {
		got, ok := parseTransform(test.value)
		if ok != test.ok || !closeMatrices(got, test.want) {
			t.Errorf("parseTransform(%q) = %v, %v, want %v, %v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func closeMatrices(first, second matrix) bool {
	near := func(one, other float64) bool { return one-other < 1e-9 && other-one < 1e-9 }
	return near(first.a, second.a) && near(first.b, second.b) && near(first.c, second.c) &&
		near(first.d, second.d) && near(first.e, second.e) && near(first.f, second.f)
}

// Too many elements fail; too deep ones are left out.
func TestLimits(t *testing.T) {
	many := `<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g/>", maxElements+1) + `</svg>`
	if _, err := Rasterize([]byte(many), image.Pt(10, 10), Options{}); err == nil {
		t.Error("too many elements: no error")
	}
	deep := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` + strings.Repeat("<g>", maxDepth+10) +
		`<rect width="10" height="10"/>` + strings.Repeat("</g>", maxDepth+10) + `</svg>`
	if gray := rasterizeGray(t, deep, image.Pt(10, 10), Options{}).GrayAt(5, 5).Y; gray != 255 {
		t.Errorf("too deep: gray %d, want white", gray)
	}
}

// FuzzStyles fuzzes the content and stylesheet of a well-formed SVG, which the styler reads
// further than random bytes.
func FuzzStyles(f *testing.F) {
	f.Add(`<rect class="a" width="5" height="5" style="stroke-dasharray: 1 2em"/>`, `.a { fill: hsl(10 20% 30% / .5) } rect + rect { stroke: currentColor }`)
	f.Add(`<g transform="skewX(10) rotate(5 1 1)"><text dx="2" font-size="200%">A<tspan y="3 4">B</tspan></text></g>`, `text { font: italic bold 12px serif }`)
	f.Add(`<pattern id="p" href="#p"/><circle r="3" fill="url(#p) red"/>`, `circle:first-child { fill: inherit }`)
	f.Fuzz(func(t *testing.T, content, sheet string) {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>` + sheet + `</style>` + content + `</svg>`
		_, _ = Rasterize([]byte(svg), image.Pt(12, 12), Options{Text: &fakeText{}, CSS: sheet}) // must not panic nor hang
	})
}

func FuzzRasterize(f *testing.F) {
	for _, seed := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><style>.a{fill:rgba(1,2,3,.5)}</style><rect class="a" width="5" height="5"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><g transform="rotate(30 5 5) scale(2)"><text x="1" y="5">Hi <tspan dy="2">there</tspan></text></g></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><pattern id="p"><circle r="2" fill="url(#p)"/></pattern><rect fill="url(#p)" width="9" height="9"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 9 9"><pattern id="a" width=".3" height="40%" viewBox="0 0 2 2" patternTransform="rotate(20)"><rect width="1" height="1" fill="url(#g)"/></pattern>` +
			`<linearGradient id="g"><stop stop-color="red"/></linearGradient><pattern id="b" href="#a" patternContentUnits="objectBoundingBox" x="1"/>` +
			`<g transform="skewX(10)" fill="url(#b)" fill-opacity=".5"><circle cx="4" cy="4" r="3" stroke="url(#a)"/><path d="M0 0Q9 0 9 9C0 9 0 0 0 0z"/></g><rect fill="url(#a)" width="9" height="0"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><linearGradient id="g"><stop style="stop-color:currentColor"/></linearGradient><path d="M0 0L5 5" stroke="url(#g)" stroke-dasharray="1em 2"/></svg>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, svg string) {
		canvas, err := Rasterize([]byte(svg), image.Pt(16, 16), Options{Text: &fakeText{}, CSS: "rect { fill: gray }"})
		if err == nil && canvas.Bounds() != image.Rect(0, 0, 16, 16) {
			t.Fatalf("bounds %v", canvas.Bounds())
		}
	})
}
