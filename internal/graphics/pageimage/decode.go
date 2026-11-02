package pageimage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // registers the JPEG decoder with image.Decode
	_ "image/png"  // registers the PNG decoder with image.Decode
	"io"

	"github.com/gen2brain/gav1d/avif" // also registers the AVIF decoder with image.Decode, unguarded
	"github.com/gen2brain/jxl"        // also registers the JPEG XL decoder with image.Decode, unguarded
	_ "golang.org/x/image/webp"       // registers the WebP decoder with image.Decode

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// The signatures of a JPEG XL file: a bare codestream, or the ISO BMFF container.
const (
	jxlCodestream = "\xff\x0a"
	jxlContainer  = "\x00\x00\x00\x0cJXL \r\n\x87\n"
)

// maxJXLPixels is the largest JPEG XL image decoded, in pixels (8.4 M, a page scanned at
// 300 dpi): the decoder works in float32 planes, some 55 bytes allocated a pixel, and a
// header may ask for any size.
const maxJXLPixels = 1 << 23

// errJXLTooLarge is returned for a JPEG XL image larger than maxJXLPixels.
var errJXLTooLarge = errors.New("jxl: image too large")

// The brands of an AVIF file, its bytes 4 to 12: a still image, or an image sequence.
const (
	avifStill    = "ftypavif"
	avifSequence = "ftypavis"
)

// maxAVIFPixels is the largest AVIF image decoded, in pixels: JPEG XL's 8.4 M. An AV1 decoder
// keeps its reference planes and works a picture in 16-bit samples, and a header may ask for any
// size.
const maxAVIFPixels = maxJXLPixels

// errAVIFTooLarge is returned for an AVIF image larger than maxAVIFPixels.
var errAVIFTooLarge = errors.New("avif: image too large")

// avifOnCaller is set once gav1d can decode on the calling goroutine. It cannot yet: it decodes
// a frame and its tiles on goroutines of its own whenever the reader has more than one core, and
// a panic there on a crafted image would end the app, which no recover can stop. Until then an
// AVIF is refused and the book's manifest fallback shows instead (the container no longer counts
// AVIF as an image it can show), or the image's text alternative.
const avifOnCaller = false

// ErrRefused is the error, wrapped, of an image of a format the app can read but chooses not to
// decode: its text alternative is shown in its place, as for one that is not ready in time.
var ErrRefused = errors.New("image refused")

// errAVIFOffCaller is returned for an AVIF image while avifOnCaller is not set.
var errAVIFOffCaller = fmt.Errorf("%w: avif: its decoder would run off the calling goroutine", ErrRefused)

// maxPixels is the largest image of another format decoded, in pixels (33.5 M, a double page
// scanned at 400 dpi): decoded, it takes up to 4 bytes a pixel, and a header may ask for any
// size, which a book could use to take the reader's memory.
const maxPixels = 1 << 25

// errTooLarge is returned for an image larger than maxPixels.
var errTooLarge = errors.New("image too large")

// DecodeGray decodes a JPEG, PNG, WebP or JPEG XL image to grayscale, what is transparent in
// it composited over white, the page.
func DecodeGray(data []byte) (*image.Gray, error) {
	img, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return toGray(img), nil
}

// Decode decodes an image of any registered format, a JPEG XL one through decodeJXL, and refuses
// an AVIF one until avifOnCaller is set. It is what decodes the images of books and servers: the
// decoders that image.Decode picks for those have no size cap of ours, and a panic in their
// goroutines could not be recovered. An image whose header gives more than maxPixels is not
// decoded, and a decoder's panic is returned as an error.
func Decode(data []byte) (img image.Image, err error) {
	switch {
	case isJXL(data):
		return decodeJXL(data)
	case isAVIF(data):
		if !avifOnCaller {
			return nil, errAVIFOffCaller
		}
		return decodeAVIF(data)
	}
	defer recoverError(&err)
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if int64(config.Width)*int64(config.Height) > maxPixels {
		return nil, fmt.Errorf("%w: %d×%d", errTooLarge, config.Width, config.Height)
	}
	img, _, err = image.Decode(bytes.NewReader(data))
	return img, err
}

// DecodeConfig is image.DecodeConfig, a decoder's panic (on a malformed JPEG XL header)
// returned as an error.
func DecodeConfig(reader io.Reader) (config image.Config, err error) {
	defer recoverError(&err)
	config, _, err = image.DecodeConfig(reader)
	return config, err
}

// isJXL reports whether data starts with a JPEG XL signature.
func isJXL(data []byte) bool {
	return bytes.HasPrefix(data, []byte(jxlCodestream)) || bytes.HasPrefix(data, []byte(jxlContainer))
}

// decodeJXL decodes a JPEG XL image whose header gives at most maxJXLPixels, on the calling
// goroutine, so that a panic of the decoder on malformed data comes back as an error. It
// gives *image.NRGBA, or *image.Gray without color or alpha, which toGray converts fast;
// above 8 bits, *image.NRGBA64 or *image.Gray16, which go through img.At.
func decodeJXL(data []byte) (img image.Image, err error) {
	defer recoverError(&err)
	config, err := jxl.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if int64(config.Width)*int64(config.Height) > maxJXLPixels {
		return nil, fmt.Errorf("%w: %d×%d", errJXLTooLarge, config.Width, config.Height)
	}
	// One thread: a panic in the decoder's own goroutines would end the app. Pages are
	// decoded in parallel already.
	return jxl.Decode(bytes.NewReader(data), jxl.Options{Threads: 1, FrameSizeLimit: maxJXLPixels})
}

// isAVIF reports whether data is an AVIF file: an ISO BMFF file of an AVIF brand.
func isAVIF(data []byte) bool {
	return len(data) >= 12 && (string(data[4:12]) == avifStill || string(data[4:12]) == avifSequence)
}

// decodeAVIF decodes an AVIF image whose header gives at most maxAVIFPixels, its frame limited
// to the same, a panic on the calling goroutine returned as an error. gav1d decodes an image's
// frame and tiles on goroutines of its own and offers no way to keep them on the caller, so a
// panic there is not recovered: Decode calls it only once avifOnCaller is set, and fuzzing
// (FuzzDecodeAVIF) keeps it ready for then. It gives *image.NRGBA,
// or *image.NRGBA64 above 8 bits, which goes through img.At.
func decodeAVIF(data []byte) (img image.Image, err error) {
	span := timing.Start(context.Background(), "decode avif")
	defer span.End()
	defer recoverError(&err)
	config, err := avif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if int64(config.Width)*int64(config.Height) > maxAVIFPixels {
		return nil, fmt.Errorf("%w: %d×%d", errAVIFTooLarge, config.Width, config.Height)
	}
	return avif.Decode(bytes.NewReader(data), avif.Options{FrameSizeLimit: maxAVIFPixels})
}

// recoverError puts a panic, as an error, in *err; it is deferred.
func recoverError(err *error) { //nolint:gocritic // ptrToRefParam: it sets the deferring function's named result
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("image decoder panic: %v", recovered)
	}
}

// toGray converts a decoded image to gray, composited over white (transparent pixels are
// often stored black), with a fast path for each kind of image the decoders return, and img.At
// for any other.
func toGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	switch img := img.(type) {
	case *image.Gray: // gray PNG and JPEG, JPEG XL without color or alpha
		return img
	case *image.YCbCr: // JPEG and lossy WebP: the luma plane already is the gray image
		return &image.Gray{Pix: img.Y, Stride: img.YStride, Rect: bounds}
	case *image.NYCbCrA: // lossy WebP with alpha
		return lumaAlphaToGray(img)
	case *image.RGBA: // opaque color PNG, a drawn SVG
		return rgbaToGray(img.Pix, img.Stride, bounds, false)
	case *image.NRGBA: // PNG with transparency (gray too), lossless WebP, JPEG XL
		return rgbaToGray(img.Pix, img.Stride, bounds, true)
	case *image.Paletted: // paletted PNG (its tRNS as the colors' alpha), GIF, the PNG pages of older versions
		return palettedToGray(img)
	}
	gray := image.NewGray(bounds)
	for row := range bounds.Dy() {
		for column := range bounds.Dx() {
			// The pixels are numbered from bounds.Min, which is not always (0, 0).
			gray.Pix[row*gray.Stride+column] = grayOverWhite(img.At(bounds.Min.X+column, bounds.Min.Y+row))
		}
	}
	return gray
}

// grayOverWhite returns the gray of a color composited over white.
func grayOverWhite(pixel color.Color) uint8 {
	red, green, blue, alpha := pixel.RGBA() // premultiplied
	white := 0xffff - alpha
	return luma(red+white, green+white, blue+white)
}

// lumaAlphaToGray converts a luma plane with alpha (not premultiplied) to gray over white,
// rounded.
func lumaAlphaToGray(img *image.NYCbCrA) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	width := bounds.Dx()
	for row := range bounds.Dy() {
		lumas := img.Y[row*img.YStride : row*img.YStride+width]
		alphas := img.A[row*img.AStride : row*img.AStride+width]
		dst := gray.Pix[row*gray.Stride : row*gray.Stride+width]
		for column, alpha := range alphas {
			dst[column] = uint8((uint32(lumas[column])*uint32(alpha) + 0xff*(0xff-uint32(alpha)) + 0x7f) / 0xff)
		}
	}
	return gray
}

// palettedToGray converts a paletted image to gray through a table of its palette's grays,
// over white.
func palettedToGray(img *image.Paletted) *image.Gray {
	var paletteGrays [256]uint8
	for index, paletteColor := range img.Palette {
		paletteGrays[index] = grayOverWhite(paletteColor)
	}
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	width := bounds.Dx()
	for row := range bounds.Dy() {
		src := img.Pix[row*img.Stride : row*img.Stride+width]
		dst := gray.Pix[row*gray.Stride : row*gray.Stride+width]
		for column, index := range src {
			dst[column] = paletteGrays[index]
		}
	}
	return gray
}

// rgbaToGray converts 8-bit RGBA pixels to gray over white exactly as grayOverWhite does,
// without the cost of img.At and a color conversion per pixel. A row is first converted as if
// opaque, which nearly all rows are; one with a pixel that is not is converted again by
// compositeRow.
//
// HOT PATH: PNG and WebP pages, and drawn SVGs, all come through here. It was tuned against
// the ARM assembly: its helpers are inlined, and its slicing lets the compiler drop the bounds
// checks. Run BenchmarkRGBAToGray and check the inlining with GOARCH=arm before and after
// changing it.
func rgbaToGray(pix []uint8, stride int, bounds image.Rectangle, straight bool) *image.Gray {
	gray := image.NewGray(bounds)
	width := bounds.Dx()
	for row := range bounds.Dy() {
		src, dst := pix[row*stride:row*stride+4*width], gray.Pix[row*gray.Stride:row*gray.Stride+width]
		rowAlpha := uint8(0xff) // the alphas of the row ANDed: 0xff if all are opaque
		for column := range dst {
			pixel := src[4*column : 4*column+4 : 4*column+4] // red, green, blue, alpha
			rowAlpha &= pixel[3]
			// The 16-bit values that color.RGBA.RGBA and color.NRGBA.RGBA return when opaque.
			dst[column] = luma(to16Bits(pixel[0]), to16Bits(pixel[1]), to16Bits(pixel[2]))
		}
		if rowAlpha != 0xff {
			compositeRow(src, dst, straight)
		}
	}
	return gray
}

// compositeRow converts a row of 8-bit RGBA pixels to gray, each composited over white. If
// straight is set (NRGBA), the colors are first multiplied by their alpha, as
// color.NRGBA.RGBA does.
func compositeRow(src, dst []uint8, straight bool) {
	for column := range dst {
		pixel := src[4*column : 4*column+4 : 4*column+4] // red, green, blue, alpha
		red, green, blue, alpha := to16Bits(pixel[0]), to16Bits(pixel[1]), to16Bits(pixel[2]), to16Bits(pixel[3])
		if straight {
			red, green, blue = red*alpha/0xffff, green*alpha/0xffff, blue*alpha/0xffff
		}
		white := 0xffff - alpha
		dst[column] = luma(red+white, green+white, blue+white)
	}
}

// to16Bits widens an 8-bit color value to 16 bits, 0xff becoming 0xffff.
func to16Bits(value uint8) uint32 { return uint32(value) * 0x101 }

// luma returns the gray of 16-bit red, green and blue values as color.GrayModel does: with
// weights 0.299, 0.587 and 0.114 in 16.16 fixed point, rounded, keeping the high 8 bits.
func luma(red, green, blue uint32) uint8 {
	return uint8((19595*red + 38470*green + 7471*blue + 1<<15) >> 24)
}
