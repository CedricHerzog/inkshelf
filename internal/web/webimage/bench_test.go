package webimage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
)

// photograph returns a 1200 by 900 picture with gradients and fine detail, so that its JPEG
// and PNG encodings are as costly to decode as a book's illustrations.
func photograph() *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	for row := range 900 {
		for column := range 1200 {
			detail := uint8((column*column + row*7) % 61)
			picture.SetRGBA(column, row, color.RGBA{
				R: uint8(column*255/1200) ^ detail, G: uint8(row * 255 / 900), B: uint8((column+row)%256) / 2, A: 255,
			})
		}
	}
	return picture
}

// benchmarkSVG is a diagram of shapes, a gradient and transforms, without text.
const benchmarkSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 300" width="400" height="300">
<defs><linearGradient id="shade"><stop offset="0" stop-color="white"/><stop offset="1" stop-color="#555"/></linearGradient></defs>
<rect x="10" y="10" width="380" height="280" rx="20" fill="url(#shade)" stroke="black" stroke-width="3"/>
<g transform="translate(200 150) rotate(30)">
<path d="M -120 0 C -80 -90, 80 -90, 120 0 S 80 90, 0 60 Z" fill="#888" stroke="black" stroke-width="2"/>
<circle r="30" fill="white" stroke="#222" stroke-width="4"/>
</g>
<polyline points="20,280 80,200 140,240 200,120 260,180 320,60 380,100" fill="none" stroke="black" stroke-width="5"/>
</svg>`

// Prepare an image for a page of the InkPad 3, fitted in a box most of the screen's width:
// decoded or rasterized, toned, dithered to 16 grays and unpacked again.
func BenchmarkPrepare(b *testing.B) {
	var jpegData, pngData bytes.Buffer
	picture := photograph()
	if err := jpeg.Encode(&jpegData, picture, &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	if err := png.Encode(&pngData, picture); err != nil {
		b.Fatal(err)
	}
	size := image.Pt(1224, 918)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"jpeg", jpegData.Bytes()},
		{"png", pngData.Bytes()},
		{"svg", []byte(benchmarkSVG)},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Prepare(test.data, size, svgimage.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
