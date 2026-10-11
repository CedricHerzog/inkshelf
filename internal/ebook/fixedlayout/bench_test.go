package fixedlayout

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// BenchmarkPageRender draws a comic's page for the InkPad 3's screen: a gradient behind a
// picture that fills the 1200x1800 page, decoded and fitted, an SVG figure in a corner, and a
// caption of a few lines laid out at the page's bottom.
func BenchmarkPageRender(b *testing.B) {
	pageSize := image.Pt(1200, 1800)
	gradient, ok := css.ParseLinearGradient("linear-gradient(to bottom, white, #444)", css.Black)
	if !ok {
		b.Fatal("the gradient does not parse")
	}
	caption := &document.Document{}
	for range 3 {
		caption.Add(document.Block{Runs: []document.Run{{Text: "Il était une fois, au bord de la rivière, un château dont les fenêtres " +
			"restaient éclairées bien après minuit, et personne au village ne savait pourquoi."}}})
	}
	files := files{
		"page.png": scannedPicture(b, pageSize),
		"figure.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="300" viewBox="0 0 400 300">
<rect x="10" y="10" width="380" height="280" rx="20" fill="#ddd" stroke="black" stroke-width="4"/>
<circle cx="120" cy="150" r="70" fill="#888"/><path d="M 220 220 L 300 80 L 360 220 Z" fill="none" stroke="black" stroke-width="6"/></svg>`),
	}
	page := &Page{Size: pageSize, EmPixels: 40, Regions: []*Region{
		{Rect: image.Rectangle{Max: pageSize}, Gradient: &gradient, Content: imageDocument("page.png")},
		{Rect: image.Rect(840, 60, 1140, 360), Content: imageDocument("figure.svg")},
		{Rect: image.Rect(100, 1450, 1100, 1750), Color: css.Color{Gray: 240, Alpha: 255}, Content: caption},
	}}
	screen := image.Pt(1404, 1872)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := page.Render(screen, files); err != nil {
			b.Fatal(err)
		}
	}
}

// scannedPicture returns a PNG of size in bands of gray crossed by a grid: neither flat nor
// noise, so that it compresses and decodes as a scanned page would.
func scannedPicture(b *testing.B, size image.Point) []byte {
	b.Helper()
	picture := image.NewGray(image.Rectangle{Max: size})
	for row := range size.Y {
		for column := range size.X {
			shade := uint8(255 - (row*160/size.Y + column*60/size.X))
			if row%50 < 3 || column%50 < 3 {
				shade /= 3
			}
			picture.Pix[row*picture.Stride+column] = shade
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		b.Fatal(err)
	}
	return encoded.Bytes()
}
