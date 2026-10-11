package fakecalibreweb

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The fake library's pictures are plain gray shapes with the book's title in Go Bold, so that
// a shot shows which book is which.

// The sizes of the generated images.
var (
	coverSize     = image.Pt(600, 900)
	comicPageSize = image.Pt(1200, 1800) // a CBZ page
	plateSize     = image.Pt(800, 500)   // the illustration of an EPUB
)

// jpegQuality is the quality of the covers and CBZ pages.
const jpegQuality = 85

// drawCover returns the cover of a book: its title on a gray band, as a JPEG.
func drawCover(item book) ([]byte, error) {
	img := image.NewRGBA(image.Rectangle{Max: coverSize})
	shade := uint8(60 + item.ID*7%120)
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Gray{Y: shade}), image.Point{}, draw.Src)
	band := image.Rect(0, coverSize.Y/3, coverSize.X, coverSize.Y*2/3)
	draw.Draw(img, band, image.White, image.Point{}, draw.Src)
	if err := drawLines(img, band, 56, strings.Split(item.Title, ", ")); err != nil {
		return nil, err
	}
	return encodeJPEG(img)
}

// drawPlate returns the illustration of an EPUB, a checkerboard of two grays, as a PNG.
func drawPlate() ([]byte, error) {
	img := image.NewGray(image.Rectangle{Max: plateSize})
	for index := range img.Pix {
		column, row := index%plateSize.X, index/plateSize.X
		img.Pix[index] = uint8((column/40+row/40)%2*160 + 60)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// drawLines draws lines centred in rect, size pixels high, in black.
func drawLines(img *image.RGBA, rect image.Rectangle, size float64, lines []string) error {
	parsed, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	drawer := font.Drawer{Dst: img, Src: image.Black, Face: face}
	lineHeight := int(size * 1.3)
	top := rect.Min.Y + (rect.Dy()-lineHeight*len(lines))/2
	for index, text := range lines {
		width := drawer.MeasureString(text).Ceil()
		drawer.Dot = fixed.P(rect.Min.X+(rect.Dx()-width)/2, top+index*lineHeight+int(size))
		drawer.DrawString(text)
	}
	return nil
}

// encodeJPEG returns img as a JPEG.
func encodeJPEG(img image.Image) ([]byte, error) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
