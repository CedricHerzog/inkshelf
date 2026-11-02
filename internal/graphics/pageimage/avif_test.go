package pageimage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
	"testing"

	"github.com/gen2brain/gav1d/avif"
)

// The AVIF images of these tests are made in code with gav1d's own encoder, which writes 8-bit
// images only: 4:2:0 color, monochrome, and color with an alpha item.

// encodeAVIF returns img as an AVIF of quality.
func encodeAVIF(tb testing.TB, img image.Image, quality int) []byte {
	tb.Helper()
	var encoded bytes.Buffer
	if err := avif.Encode(&encoded, img, avif.EncodeOptions{Quality: quality}); err != nil {
		tb.Fatal(err)
	}
	return encoded.Bytes()
}

// colorPage returns a 64×48 image, its top half black, its bottom half white with a red box
// (#c82828) at (40, 30)–(60, 44), as page.jxl.
func colorPage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	draw.Draw(img, image.Rect(0, 0, 64, 24), image.Black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 24, 64, 48), image.White, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(40, 30, 60, 44), image.NewUniform(color.NRGBA{R: 0xc8, G: 0x28, B: 0x28, A: 0xff}), image.Point{}, draw.Src)
	return img
}

// halfTransparent returns a 32×16 image, its left half black, its right half transparent, as
// alpha.jxl.
func halfTransparent() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	draw.Draw(img, image.Rect(0, 0, 16, 16), image.Black, image.Point{}, draw.Src)
	return img
}

// grayRamp returns a 48×32 gray image, black on its left to white on its right.
func grayRamp() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 48, 32))
	for row := range 32 {
		for column := range 48 {
			img.Pix[row*img.Stride+column] = uint8(column * 255 / 47)
		}
	}
	return img
}

// withSize returns data, an AVIF, whose ispe property says its image is width×height.
func withSize(tb testing.TB, data []byte, width, height uint32) []byte {
	tb.Helper()
	data = bytes.Clone(data)
	// The box's type, then its version and flags, then the width and the height.
	at := bytes.Index(data, []byte("ispe"))
	if at < 0 {
		tb.Fatal("no ispe property")
	}
	binary.BigEndian.PutUint32(data[at+8:], width)
	binary.BigEndian.PutUint32(data[at+12:], height)
	return data
}

func TestDecodeGrayAVIF(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		size   image.Point
		pixels map[image.Point][2]uint8 // the range of gray each pixel may have
	}{
		{
			name: "4:2:0 color",
			data: encodeAVIF(t, colorPage(), 90),
			size: image.Pt(64, 48),
			pixels: map[image.Point][2]uint8{
				{10, 10}: {0, 10},    // black
				{10, 40}: {245, 255}, // white
				{50, 37}: {75, 101},  // red: 88 in gray
			},
		},
		{
			name: "monochrome",
			data: encodeAVIF(t, grayRamp(), 90),
			size: image.Pt(48, 32),
			pixels: map[image.Point][2]uint8{
				{0, 16}:  {0, 12},
				{24, 16}: {118, 142},
				{47, 16}: {243, 255},
			},
		},
		{
			name: "alpha",
			data: encodeAVIF(t, halfTransparent(), 100),
			size: image.Pt(32, 16),
			pixels: map[image.Point][2]uint8{
				{4, 8}:  {0, 10},    // black
				{24, 8}: {245, 255}, // transparent: white, the page
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !isAVIF(test.data) {
				t.Fatal("not recognized as AVIF")
			}
			gray, err := DecodeGray(test.data)
			if err != nil {
				t.Fatal(err)
			}
			if got := gray.Bounds().Size(); got != test.size {
				t.Fatalf("size %v, want %v", got, test.size)
			}
			for point, want := range test.pixels {
				if got := grayAt(gray, point.X, point.Y); got < want[0] || got > want[1] {
					t.Errorf("gray at %v: %d, want %d to %d", point, got, want[0], want[1])
				}
			}
			config, err := DecodeConfig(bytes.NewReader(test.data))
			if err != nil || image.Pt(config.Width, config.Height) != test.size {
				t.Errorf("DecodeConfig = %v×%v, %v; want %v", config.Width, config.Height, err, test.size)
			}
		})
	}
}

// An AVIF whose header gives more than maxAVIFPixels is refused before any decoding.
func TestDecodeAVIFTooLarge(t *testing.T) {
	data := withSize(t, encodeAVIF(t, grayRamp(), 60), 5000, 4000)
	if _, err := DecodeGray(data); !errors.Is(err, errAVIFTooLarge) {
		t.Fatalf("error %v, want %v", err, errAVIFTooLarge)
	}
}

// Malformed AVIF images give an error, never a panic: run with -fuzz=FuzzDecodeAVIF.
func FuzzDecodeAVIF(f *testing.F) {
	f.Add(encodeAVIF(f, colorPage(), 90))
	f.Add(encodeAVIF(f, grayRamp(), 60))
	f.Add(encodeAVIF(f, halfTransparent(), 100))
	f.Fuzz(func(t *testing.T, data []byte) {
		if config, err := DecodeConfig(bytes.NewReader(data)); err == nil && int64(config.Width)*int64(config.Height) > maxAVIFPixels {
			return // not decoded: refused from its header
		}
		gray, err := DecodeGray(data)
		if err == nil && gray == nil {
			t.Fatal("no image and no error")
		}
	})
}

// BenchmarkDecodeAVIF decodes a full page, 1404×1872, of gray noise over gradients, so that it
// does not compress to nothing.
func BenchmarkDecodeAVIF(b *testing.B) {
	page := image.NewNRGBA(image.Rect(0, 0, 1404, 1872))
	random := rand.New(rand.NewPCG(1, 2))
	for row := range 1872 {
		for column := range 1404 {
			offset := page.PixOffset(column, row)
			base := (column + row) * 255 / (1404 + 1872)
			noise := random.IntN(48)
			page.Pix[offset] = uint8(min(base+noise, 255))
			page.Pix[offset+1] = uint8(min(row*255/1872+noise, 255))
			page.Pix[offset+2] = uint8(min(column*255/1404+noise, 255))
			page.Pix[offset+3] = 0xff
		}
	}
	data := encodeAVIF(b, page, 70)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := DecodeGray(data); err != nil {
			b.Fatal(err)
		}
	}
}
