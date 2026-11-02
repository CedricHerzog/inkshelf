package fixedlayout

import (
	"bytes"
	"image"
	"image/draw"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/layout"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// maxTransparentPixels caps the size of a background image composited with its transparency:
// a larger one is drawn as other images are, over white.
const maxTransparentPixels = 16 << 20

// fillColor blends color over area of canvas, row by row: image/draw has no fast path for
// gray images.
func fillColor(canvas *image.Gray, area image.Rectangle, color css.Color) {
	area = area.Intersect(canvas.Bounds())
	if color.Alpha == 0 || area.Empty() {
		return
	}
	alpha, source := uint32(color.Alpha), uint32(color.Gray)*uint32(color.Alpha)
	for row := area.Min.Y; row < area.Max.Y; row++ {
		pixels := canvas.Pix[canvas.PixOffset(area.Min.X, row):canvas.PixOffset(area.Max.X, row)]
		for index, pixel := range pixels {
			pixels[index] = uint8((source + uint32(pixel)*(255-alpha) + 127) / 255)
		}
	}
}

// backgroundTile returns the image at imagePath (a data: URI too) stretched to size, as a
// background layer's tile: with its transparency if it has some, so that what is under it shows.
func backgroundTile(imagePath string, size image.Point, files Files, faces *layout.Faces) (layout.Tile, error) {
	data, err := imageData(imagePath, files)
	if err != nil {
		return layout.Tile{}, err
	}
	if svgimage.IsSVG(data) {
		picture, err := rasterize(data, imagePath, size, files, faces, false)
		return layout.Tile{Gray: picture}, err
	}
	if tile, ok := transparentTile(data, size); ok {
		return tile, nil
	}
	decoded, err := pageimage.DecodeGray(data)
	if err != nil {
		return layout.Tile{}, err
	}
	return layout.Tile{Gray: stretch(pageimage.Fit(decoded, size.X, size.Y), size)}, nil
}

// transparentTile returns a raster image with transparency stretched to size: its gray
// premultiplied by its opacity, and its opacity. It returns false if the image is opaque or
// cannot be decoded, for backgroundTile to draw it as other images are.
func transparentTile(data []byte, size image.Point) (layout.Tile, bool) {
	// pageimage's decoders, which recover from a malformed JPEG XL's panic.
	config, err := pageimage.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width*config.Height > maxTransparentPixels {
		return layout.Tile{}, false
	}
	decoded, err := pageimage.Decode(data)
	if err != nil {
		return layout.Tile{}, false
	}
	if opaque, ok := decoded.(interface{ Opaque() bool }); !ok || opaque.Opaque() {
		return layout.Tile{}, false
	}
	bounds := decoded.Bounds()
	premultiplied := image.NewRGBA(bounds)
	draw.Draw(premultiplied, bounds, decoded, bounds.Min, draw.Src)
	// Gray times alpha, and alpha: both scaled as they are, which keeps the edges right.
	gray, alpha := image.NewGray(bounds), image.NewGray(bounds)
	for index := range gray.Pix {
		pixel := premultiplied.Pix[4*index : 4*index+4 : 4*index+4]
		red, green, blue := uint32(pixel[0]), uint32(pixel[1]), uint32(pixel[2])
		gray.Pix[index] = uint8((19595*red + 38470*green + 7471*blue + 1<<15) >> 16)
		alpha.Pix[index] = pixel[3]
	}
	return layout.Tile{
		Gray:  stretch(pageimage.Fit(gray, size.X, size.Y), size),
		Alpha: stretch(pageimage.Fit(alpha, size.X, size.Y), size),
	}, true
}

// imageData returns the data of the image at imagePath: a file of the book, or a data: URI.
func imageData(imagePath string, files Files) ([]byte, error) {
	if uri.IsData(imagePath) {
		return uri.DecodeData(imagePath)
	}
	return files.Resource(imagePath)
}
