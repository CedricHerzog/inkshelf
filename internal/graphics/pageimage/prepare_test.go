package pageimage

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

// encodePNG returns img encoded as a PNG.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestPrepare(t *testing.T) {
	// A horizontal gradient from dark gray to light gray, 400x300.
	src := image.NewGray(image.Rect(0, 0, 400, 300))
	for row := range 300 {
		for column := range 400 {
			src.Pix[row*src.Stride+column] = uint8(40 + column*160/400)
		}
	}
	packed, err := Prepare(encodePNG(t, src), 200, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	gray, err := Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fits the screen", func(t *testing.T) {
		if bounds := gray.Bounds(); bounds.Dx() != 200 || bounds.Dy() != 150 {
			t.Fatalf("size %v, want 200x150", bounds)
		}
	})
	t.Run("stretches the levels", func(t *testing.T) {
		// The ends of the gradient become black and white.
		if left, right := gray.GrayAt(0, 75).Y, gray.GrayAt(199, 75).Y; left != 0 || right != 255 {
			t.Fatalf("ends %d, %d; want 0, 255", left, right)
		}
	})
	t.Run("uses the panel's grays", func(t *testing.T) {
		for _, value := range gray.Pix {
			if value%grayStep != 0 {
				t.Fatalf("gray %d is not one of the panel's", value)
			}
		}
	})
}

// Prepare fits whatever size the decoder returns.
func TestPrepareFitsDecodedImage(t *testing.T) {
	decode := func(data []byte) (*image.Gray, error) {
		return image.NewGray(image.Rect(0, 0, 300, 600)), nil
	}
	packed, err := prepare([]byte("not an image"), 200, 200, decode, nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if bounds := img.Bounds(); bounds.Dx() != 100 || bounds.Dy() != 200 {
		t.Fatalf("size %v, want 100x200", bounds)
	}
}

// Prepare counts the time it spends in each stage.
func TestPrepareTimes(t *testing.T) {
	var times Times
	if _, err := Prepare(encodePNG(t, image.NewGray(image.Rect(0, 0, 400, 300))), 200, 200, &times); err != nil {
		t.Fatal(err)
	}
	if times.Decode.Load() <= 0 || times.Scale.Load() <= 0 || times.Dither.Load() <= 0 {
		t.Fatalf("times %d, %d, %d ns; want all above 0", times.Decode.Load(), times.Scale.Load(), times.Dither.Load())
	}
}
