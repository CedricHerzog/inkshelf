package pageimage

import (
	"bytes"
	"errors"
	"image"
	"os"
	"testing"
)

// The JPEG XL files of testdata, made with cjxl 0.12 from images drawn by ImageMagick:
// page.jxl, a lossy (VarDCT) bare codestream, 64×48, its top half black, its bottom half white
// with a red box (#c82828) at (40, 30)–(60, 44); alpha.jxl, a lossless (modular) file in the
// container, 32×16, its left half black, its right half transparent; large-header.jxl, the
// first 64 bytes of a 5000×4000 white image, its header whole.

func readTestdata(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// grayAt returns the gray of a pixel of a decoded page.
func grayAt(gray *image.Gray, column, row int) uint8 {
	return gray.Pix[(row-gray.Rect.Min.Y)*gray.Stride+column-gray.Rect.Min.X]
}

func TestDecodeGrayJXL(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		size   image.Point
		pixels map[image.Point][2]uint8 // the range of gray each pixel may have
	}{
		{
			name: "lossy codestream",
			file: "page.jxl",
			size: image.Pt(64, 48),
			pixels: map[image.Point][2]uint8{
				{10, 10}: {0, 8},     // black
				{10, 40}: {247, 255}, // white
				{50, 37}: {78, 98},   // red: 88 in gray
			},
		},
		{
			name: "lossless container with alpha",
			file: "alpha.jxl",
			size: image.Pt(32, 16),
			pixels: map[image.Point][2]uint8{
				{4, 8}:  {0, 0},     // black
				{24, 8}: {255, 255}, // transparent: white, the page
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gray, err := DecodeGray(readTestdata(t, test.file))
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
		})
	}
}

// The size of a JPEG XL image comes from its header, through image.DecodeConfig as epub's
// ImageSize and layout's ask for it.
func TestDecodeConfigJXL(t *testing.T) {
	tests := []struct {
		file string
		size image.Point
	}{
		{file: "page.jxl", size: image.Pt(64, 48)},
		{file: "alpha.jxl", size: image.Pt(32, 16)},
		{file: "large-header.jxl", size: image.Pt(5000, 4000)},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			config, err := DecodeConfig(bytes.NewReader(readTestdata(t, test.file)))
			if err != nil {
				t.Fatal(err)
			}
			if got := image.Pt(config.Width, config.Height); got != test.size {
				t.Fatalf("size %v, want %v", got, test.size)
			}
		})
	}
}

// An image larger than maxJXLPixels is refused from its header, before any decoding.
func TestDecodeJXLTooLarge(t *testing.T) {
	_, err := DecodeGray(readTestdata(t, "large-header.jxl"))
	if !errors.Is(err, errJXLTooLarge) {
		t.Fatalf("error %v, want %v", err, errJXLTooLarge)
	}
}

// Malformed images give an error, never a panic: run with -fuzz=FuzzDecodeGray.
func FuzzDecodeGray(f *testing.F) {
	for _, name := range []string{"page.jxl", "alpha.jxl", "large-header.jxl"} {
		f.Add(readTestdata(f, name))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if config, err := DecodeConfig(bytes.NewReader(data)); err == nil && int64(config.Width)*int64(config.Height) > maxJXLPixels {
			return // not decoded: refused from its header
		}
		gray, err := DecodeGray(data)
		if err == nil && gray == nil {
			t.Fatal("no image and no error")
		}
	})
}

func BenchmarkDecodeGrayJXL(b *testing.B) {
	data := readTestdata(b, "page.jxl")
	for b.Loop() {
		if _, err := DecodeGray(data); err != nil {
			b.Fatal(err)
		}
	}
}
