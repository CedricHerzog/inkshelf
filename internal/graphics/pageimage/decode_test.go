package pageimage

import (
	"bytes"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

// The RGBA and NRGBA fast paths of toGray give what color.GrayModel gives over white.
func TestToGrayRGBA(t *testing.T) {
	bounds := image.Rect(3, 5, 40, 30) // not at the origin
	rgba, nrgba := image.NewRGBA(bounds), image.NewNRGBA(bounds)
	for index := range rgba.Pix {
		rgba.Pix[index] = uint8(rand.IntN(256))
	}
	for index := 0; index < len(rgba.Pix); index += 4 { // premultiplied: no color above its alpha
		for channel := range 3 {
			rgba.Pix[index+channel] = min(rgba.Pix[index+channel], rgba.Pix[index+3])
		}
	}
	for index := range nrgba.Pix {
		nrgba.Pix[index] = uint8(rand.IntN(256))
	}
	tests := []struct {
		name string
		img  image.Image
	}{
		{name: "RGBA", img: rgba},
		{name: "NRGBA", img: nrgba},
		{name: "RGBA sub-image", img: rgba.SubImage(image.Rect(10, 10, 20, 20))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := toGray(test.img)
			want := grayModelReference(test.img)
			if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("%v: differs from color.GrayModel over white", test.img.Bounds())
			}
		})
	}
}

// A paletted image goes through its palette's grays, a transparent index (a PNG's tRNS, a
// GIF's) being white.
func TestToGrayPaletted(t *testing.T) {
	palette := color.Palette{color.Gray{Y: 0}, color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{}}
	paletted := image.NewPaletted(image.Rect(0, 0, 4, 2), palette)
	paletted.Pix[1], paletted.Pix[2] = 1, 2
	got := toGray(paletted)
	if want := grayModelReference(paletted); !bytes.Equal(got.Pix, want.Pix) {
		t.Fatalf("pixels %v, want %v", got.Pix, want.Pix)
	}
	if want := []uint8{0, 255, 255, 0}; !bytes.Equal(got.Pix[:4], want) {
		t.Fatalf("first row %v, want %v", got.Pix[:4], want)
	}
}

// Transparent pixels are composited over white, whatever their color: the transparent black
// of PNG illustrations was drawn as black boxes.
func TestToGrayTransparent(t *testing.T) {
	bounds := image.Rect(0, 0, 2, 1)
	tests := []struct {
		name string
		img  image.Image
		want []uint8
	}{
		{
			name: "NRGBA transparent black",
			img:  &image.NRGBA{Pix: []uint8{0, 0, 0, 0, 0, 0, 0, 255}, Stride: 8, Rect: bounds},
			want: []uint8{255, 0},
		},
		{
			name: "NRGBA half-transparent",
			img:  &image.NRGBA{Pix: []uint8{0, 0, 0, 128, 255, 255, 255, 128}, Stride: 8, Rect: bounds},
			want: []uint8{127, 255},
		},
		{
			name: "RGBA half-transparent", // premultiplied
			img:  &image.RGBA{Pix: []uint8{0, 0, 0, 128, 128, 128, 128, 128}, Stride: 8, Rect: bounds},
			want: []uint8{127, 255},
		},
		{
			name: "NRGBA64 transparent black", // a 16-bit PNG, through img.At
			img:  &image.NRGBA64{Pix: make([]uint8, 16), Stride: 16, Rect: bounds},
			want: []uint8{255, 255},
		},
		{
			name: "NYCbCrA", // a lossy WebP with alpha
			img: &image.NYCbCrA{
				Y: []uint8{0, 0}, Cb: []uint8{128}, Cr: []uint8{128}, YStride: 2, CStride: 1,
				SubsampleRatio: image.YCbCrSubsampleRatio420, Rect: bounds,
				A: []uint8{0, 128}, AStride: 2,
			},
			want: []uint8{255, 127},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := toGray(test.img); !bytes.Equal(got.Pix, test.want) {
				t.Fatalf("pixels %v, want %v", got.Pix, test.want)
			}
		})
	}
}

// An opaque image is converted as color.GrayModel converts it: compositing changes nothing.
func TestToGrayOpaque(t *testing.T) {
	bounds := image.Rect(0, 0, 37, 21)
	rgba, nrgba, rgba64 := image.NewRGBA(bounds), image.NewNRGBA(bounds), image.NewRGBA64(bounds)
	for index := range rgba.Pix {
		value := uint8(rand.IntN(256))
		if index%4 == 3 { // alpha
			value = 255
		}
		rgba.Pix[index], nrgba.Pix[index] = value, value
	}
	for index := range rgba64.Pix {
		value := uint8(rand.IntN(256))
		if index%8 >= 6 { // alpha
			value = 255
		}
		rgba64.Pix[index] = value
	}
	for _, img := range []image.Image{rgba, nrgba, rgba64} {
		got := toGray(img)
		want := image.NewGray(bounds)
		for row := range bounds.Dy() {
			for column := range bounds.Dx() {
				want.Set(column, row, color.GrayModel.Convert(img.At(column, row)))
			}
		}
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("%T: differs from color.GrayModel", img)
		}
	}
}

// grayModelReference converts img to gray pixel by pixel with color.GrayModel, each pixel
// first composited over white.
func grayModelReference(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for row := range bounds.Dy() {
		for column := range bounds.Dx() {
			x, y := bounds.Min.X+column, bounds.Min.Y+row
			red, green, blue, alpha := img.At(x, y).RGBA() // premultiplied
			white := 0xffff - alpha
			composited := color.RGBA64{R: uint16(red + white), G: uint16(green + white), B: uint16(blue + white), A: 0xffff}
			gray.Set(x, y, color.GrayModel.Convert(composited))
		}
	}
	return gray
}
