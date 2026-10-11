package pageimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand/v2"
	"testing"
)

// benchmarkPage returns a synthetic color manga-sized page encoded as a JPEG: a gradient with
// noise, so that every stage of Prepare has real work to do.
func benchmarkPage(b *testing.B) []byte {
	b.Helper()
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, benchmarkPageImage(), &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	return encoded.Bytes()
}

// benchmarkPageImage returns the image of benchmarkPage, before it is encoded.
func benchmarkPageImage() *image.RGBA {
	const width, height = 1600, 2400
	random := rand.New(rand.NewPCG(1, 2))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for row := range height {
		for column := range width {
			level := uint8(column*200/width + random.IntN(56))
			img.Set(column, row, color.RGBA{level, level, uint8(row * 255 / height), 255})
		}
	}
	return img
}

func BenchmarkPrepare(b *testing.B) {
	data := benchmarkPage(b)
	for b.Loop() {
		if _, err := Prepare(data, 1404, 1872, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFitDither(b *testing.B) {
	src, err := DecodeGray(benchmarkPage(b))
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		gray, histogram := fit(src, 1404, 1872)
		ditherAndPack(gray, toneTable(histogram))
	}
}

func BenchmarkUnpack(b *testing.B) {
	data, err := Prepare(benchmarkPage(b), 1404, 1872, nil)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := Unpack(data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRGBAToGray converts opaque noise, as the color pages of comics are: the fast
// paths must cost no more for compositing what is transparent over white.
func BenchmarkRGBAToGray(b *testing.B) {
	bounds := image.Rect(0, 0, 1600, 2400)
	random := rand.New(rand.NewPCG(1, 2))
	rgba, nrgba := image.NewRGBA(bounds), image.NewNRGBA(bounds)
	for index := range rgba.Pix {
		value := uint8(random.IntN(256))
		if index%4 == 3 {
			value = 0xff
		}
		rgba.Pix[index], nrgba.Pix[index] = value, value
	}
	for _, img := range []image.Image{rgba, nrgba} {
		b.Run(fmt.Sprintf("%T", img), func(b *testing.B) {
			for b.Loop() {
				toGray(img)
			}
		})
	}
}
