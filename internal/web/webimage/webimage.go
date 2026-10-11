// Package webimage prepares the images of a book's markup for the panel: raster images
// (JPEG, PNG, GIF, BMP, WebP, JPEG XL) and SVGs, read from the book's files or from data: URIs.
package webimage

import (
	"bytes"
	"image"
	_ "image/gif" // registers the GIF decoder: EPUBs may hold GIF illustrations

	_ "golang.org/x/image/bmp" // registers the BMP decoder: some EPUBs hold BMP illustrations

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/web/svgimage"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// Loader returns an image of a book prepared for the panel, fitted in size.
type Loader func(path string, size image.Point) (*image.Gray, error)

// NewLoader returns the Loader of a book whose files read returns. An image's path may also
// be a data: URI, and an SVG reads the images it links to from the files next to it, and
// draws its text with text (nil leaves it out), from the goroutine that draws the page.
func NewLoader(read func(path string) ([]byte, error), text svgimage.TextDrawer) Loader {
	options := svgimage.Options{Text: text}
	return func(imagePath string, size image.Point) (*image.Gray, error) {
		if uri.IsData(imagePath) {
			data, err := uri.DecodeData(imagePath)
			if err != nil {
				return nil, err
			}
			return Prepare(data, size, options)
		}
		data, err := read(imagePath)
		if err != nil {
			return nil, err
		}
		linked := options
		linked.Resolve = func(href string) ([]byte, error) {
			return read(uri.Resolve(imagePath, href))
		}
		return Prepare(data, size, linked)
	}
}

// Prepare decodes an image's data, or draws an SVG's, and prepares it for the panel,
// fitted in size, as the pages of comics are: its tones stretched for e-ink, then dithered
// to 16 grays. An SVG is
// drawn with options: the images it links to, its text.
func Prepare(data []byte, size image.Point, options svgimage.Options) (*image.Gray, error) {
	var packed []byte
	if svgimage.IsSVG(data) {
		// Drawn at the size of its box, which was laid out from its own aspect ratio.
		drawn, err := svgimage.Rasterize(data, size, options)
		if err != nil {
			return nil, err
		}
		packed = pageimage.PrepareDecoded(drawn, size.X, size.Y)
	} else {
		var err error
		if packed, err = pageimage.Prepare(data, size.X, size.Y, nil); err != nil {
			return nil, err
		}
	}
	return pageimage.Unpack(packed)
}

// Size returns the size of an image from its data, without decoding its pixels.
func Size(data []byte) (image.Point, error) {
	if svgimage.IsSVG(data) {
		return svgimage.Size(data)
	}
	config, err := pageimage.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Point{}, err
	}
	return image.Pt(config.Width, config.Height), nil
}
