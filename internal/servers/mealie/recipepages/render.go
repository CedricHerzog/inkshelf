package recipepages

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg" // registers the JPEG decoder with image.Decode: photos may be JPEG
	"image/png"    // also registers the PNG decoder with image.Decode

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp" // registers the WebP decoder with image.Decode: Mealie's photos
)

// pngEncoder encodes the pages fast: they are decoded again at once, on the reader.
var pngEncoder = png.Encoder{CompressionLevel: png.BestSpeed}

// Render draws a page in black on white, with photo (nil if none) fitted in its box on the
// first page, and returns it as a PNG. A zero Page is a blank page.
func Render(laidOut Page, photo image.Image) ([]byte, error) {
	canvas := image.NewGray(image.Rect(0, 0, Width, Height))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	if laidOut.photo && photo != nil {
		box := image.Rect(margin, margin, Width-margin, margin+photoHeight)
		draw.CatmullRom.Scale(canvas, fitIn(photo.Bounds().Size(), box), photo, photo.Bounds(), draw.Src, nil)
	}
	drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(color.Black)}
	pageFaces := newFaces()
	for _, placed := range laidOut.lines {
		drawer.Face = pageFaces[placed.textStyle]
		ascent := drawer.Face.Metrics().Ascent
		drawer.Dot = fixed.Point26_6{X: fixed.I(placed.x), Y: fixed.I(placed.y) + ascent}
		drawer.DrawString(placed.text)
	}
	var encoded bytes.Buffer
	if err := pngEncoder.Encode(&encoded, canvas); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// fitIn returns the largest rectangle of size's proportions that fits in box, centred in it.
func fitIn(size image.Point, box image.Rectangle) image.Rectangle {
	if size.X <= 0 || size.Y <= 0 {
		return box
	}
	width, height := box.Dx(), box.Dx()*size.Y/size.X
	if height > box.Dy() {
		width, height = box.Dy()*size.X/size.Y, box.Dy()
	}
	corner := box.Min.Add(image.Pt((box.Dx()-width)/2, (box.Dy()-height)/2))
	return image.Rectangle{Min: corner, Max: corner.Add(image.Pt(width, height))}
}

// DecodePhoto decodes a recipe's photo, in JPEG, PNG or WebP, for Render.
func DecodePhoto(data []byte) (image.Image, error) {
	photo, _, err := image.Decode(bytes.NewReader(data))
	return photo, err
}
