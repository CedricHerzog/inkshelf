package epub

import (
	"image"
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
	"github.com/CedricHerzog/inkshelf/internal/web/uri"
)

// backgroundLayers returns the layers of the background an element declares, currentColor
// being current, their lengths in a box's units: units converts a length that is neither a
// percentage nor auto, false when it cannot. An image is found as backgroundImage finds it; one
// the book lacks (a remote one, for instance) leaves its layer out.
func (parser *builder) backgroundLayers(declared map[string]string, current css.Color, units func(css.Length) (float64, bool)) []document.BackgroundLayer {
	var layers []document.BackgroundLayer
	for _, layer := range css.Backgrounds(declared, current) {
		placed := document.BackgroundLayer{BackgroundLayer: layer}
		if layer.Image != "" {
			imagePath := parser.backgroundImage(layer.Image)
			if imagePath == "" {
				continue
			}
			placed.Image = imagePath
			if size, err := parser.book.ImageSize(imagePath); err == nil {
				width, _ := units(css.Length{Value: float64(size.X), Unit: "px"})
				height, _ := units(css.Length{Value: float64(size.Y), Unit: "px"})
				placed.ImageSize = image.Pt(int(width+0.5), int(height+0.5))
			}
		}
		normalizeLayer(&placed.BackgroundLayer, units)
		layers = append(layers, placed)
	}
	return layers
}

// backgroundImage returns the path in the book of a background image named by address, the
// first of its manifest fallbacks the book can show, "" if the book has none. Its URL is
// relative to the stylesheet that declares it, which the cascade forgets: it is looked for from
// the chapter, then from each of its stylesheets. A data: URI is its own path.
func (parser *builder) backgroundImage(address string) string {
	if uri.IsData(address) {
		return address
	}
	bases := append([]string{parser.chapterPath}, slices.Sorted(maps.Keys(parser.sheetsAdded))...)
	for _, base := range bases {
		if imagePath := resolve(base, address); parser.book.HasFile(imagePath) {
			if fallback, ok := parser.book.SupportedImage(imagePath); ok {
				return fallback
			}
			return imagePath
		}
	}
	return ""
}

// normalizeLayer puts a layer's lengths in a box's units, through units, with the Unit "px"
// (document.BackgroundLayer's convention), its gradient's included. A length units cannot
// convert takes CSS's initial value: a position 0, a size auto, a radius 0 (which draws nothing),
// a centre in the middle; a stop is spaced as if it had no position.
func normalizeLayer(layer *css.BackgroundLayer, units func(css.Length) (float64, bool)) {
	convert := func(length css.Length, otherwise css.Length) css.Length {
		if length.Unit == "%" || length.IsAuto() {
			return length
		}
		value, ok := units(length)
		if !ok {
			return otherwise
		}
		return css.Length{Value: value, Unit: "px"}
	}
	zero := css.Length{Unit: "%"}
	layer.X, layer.Y = convert(layer.X, zero), convert(layer.Y, zero)
	layer.Size.Width, layer.Size.Height = convert(layer.Size.Width, css.Auto), convert(layer.Size.Height, css.Auto)
	if layer.Gradient == nil {
		return
	}
	gradient := *layer.Gradient
	middle := css.Length{Value: 50, Unit: "%"}
	gradient.RadiusX, gradient.RadiusY = convert(gradient.RadiusX, css.Length{}), convert(gradient.RadiusY, css.Length{})
	gradient.CenterX, gradient.CenterY = convert(gradient.CenterX, middle), convert(gradient.CenterY, middle)
	gradient.Stops = slices.Clone(gradient.Stops)
	for index, stop := range gradient.Stops {
		if !stop.InPixels {
			continue
		}
		position, ok := units(css.Length{Value: stop.Position, Unit: "px"})
		if !ok {
			position = math.NaN() // no position
		}
		gradient.Stops[index].Position = position
	}
	layer.Gradient = &gradient
}

// hundredthsOf converts a length to hundredths of an em of an element whose font size is
// fontSize (in hundredths of the reader's): the units of a reflowable chapter's boxes.
func hundredthsOf(fontSize int) func(css.Length) (float64, bool) {
	return func(length css.Length) (float64, bool) {
		hundredths, ok := lengthOf(strconv.FormatFloat(length.Value, 'g', -1, 64)+length.Unit, fontSize, 0)
		return float64(hundredths), ok
	}
}
