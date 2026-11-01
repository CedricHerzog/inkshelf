package layout

import (
	"context"
	"image"
	"math"

	"github.com/CedricHerzog/inkshelf/internal/timing"
	"github.com/CedricHerzog/inkshelf/internal/typesetting/document"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// maxBackgroundTiles caps the tiles a background layer draws: a layer that would draw more
// (a tiny pattern over a large box) is left out, and the box keeps its color.
const maxBackgroundTiles = 4096

// maxTilePixels caps the size of a layer's tile, which is drawn whole: a layer with a larger
// one is left out.
const maxTilePixels = 16 << 20

// gradientSamples is how many colors a gradient is sampled into, from its first stop to its last.
const gradientSamples = 1024

// Tile is an image of a background layer prepared at the size of a tile: its gray, and its
// opacity (nil when opaque). The gray is premultiplied by the opacity.
type Tile struct {
	Gray, Alpha *image.Gray
}

// TileLoader returns a book's image prepared at size, for a background layer.
type TileLoader func(path string, size image.Point) (Tile, error)

// PaintBackground paints a box's background layers over box in dst, the bottom layer first,
// clipped to dst. Their lengths in the box's units are multiplied by unit to give pixels.
// An image tiles cannot load, and a layer of more than maxBackgroundTiles tiles or with a
// tile over maxTilePixels, are left out.
func PaintBackground(dst *image.Gray, box image.Rectangle, layers []document.BackgroundLayer, unit float64, tiles TileLoader) {
	clip := box.Intersect(dst.Bounds())
	if len(layers) == 0 || clip.Empty() || unit <= 0 {
		return
	}
	span := timing.Start(context.Background(), "paint background")
	defer span.End()
	for index := len(layers) - 1; index >= 0; index-- {
		paintLayer(dst, box, clip, &layers[index], unit, tiles)
	}
}

// paintLayer paints one layer over box, clipped to clip.
func paintLayer(dst *image.Gray, box, clip image.Rectangle, layer *document.BackgroundLayer, unit float64, tiles TileLoader) {
	size := tileSize(box.Size(), layer, unit)
	if size.X <= 0 || size.Y <= 0 || size.X*size.Y > maxTilePixels {
		return
	}
	columns, columnStep, firstColumn := tilePlaces(box.Dx(), size.X, length(layer.X, box.Dx()-size.X, unit), layer.RepeatX)
	rows, rowStep, firstRow := tilePlaces(box.Dy(), size.Y, length(layer.Y, box.Dy()-size.Y, unit), layer.RepeatY)
	if columns == 0 || rows == 0 || columns*rows > maxBackgroundTiles {
		return
	}
	var tile Tile
	if layer.Gradient != nil {
		shade, ok := newGradientShade(layer.Gradient, size, unit)
		if !ok {
			return
		}
		if columns*rows == 1 {
			blendGradient(dst, clip, shade, box.Min.Add(image.Pt(firstColumn, firstRow)), size)
			return
		}
		tile = gradientTile(shade, size)
	} else {
		loaded, err := tiles(layer.Image, size)
		if err != nil || loaded.Gray == nil {
			return
		}
		tile = loaded
	}
	for row := range rows {
		for column := range columns {
			at := box.Min.Add(image.Pt(firstColumn+column*columnStep, firstRow+row*rowStep))
			compositeTile(dst, clip, tile, at)
		}
	}
}

// tileSize returns the size in pixels of a layer's tile in a box of size boxSize: CSS's
// background-size, cover and contain keeping the image's aspect ratio, an auto side from the
// other at that ratio or else the image's own size (a gradient's own size is the box's), and
// round shrinking or stretching the tile to a whole number in the box.
func tileSize(boxSize image.Point, layer *document.BackgroundLayer, unit float64) image.Point {
	own := image.Pt(int(math.Round(float64(layer.ImageSize.X)*unit)), int(math.Round(float64(layer.ImageSize.Y)*unit)))
	if layer.Gradient != nil || own.X <= 0 || own.Y <= 0 {
		own = boxSize
	}
	if own.X <= 0 || own.Y <= 0 {
		return image.Point{}
	}
	width, height := float64(own.X), float64(own.Y)
	size := layer.Size
	switch {
	case size.Cover || size.Contain:
		scaleX, scaleY := float64(boxSize.X)/width, float64(boxSize.Y)/height
		scale := min(scaleX, scaleY)
		if size.Cover {
			scale = max(scaleX, scaleY)
		}
		width, height = width*scale, height*scale
	case size.Width.IsAuto() && size.Height.IsAuto():
	case size.Width.IsAuto():
		height = float64(length(size.Height, boxSize.Y, unit))
		width = height * float64(own.X) / float64(own.Y)
		if layer.Gradient != nil {
			width = float64(boxSize.X)
		}
	case size.Height.IsAuto():
		width = float64(length(size.Width, boxSize.X, unit))
		height = width * float64(own.Y) / float64(own.X)
		if layer.Gradient != nil {
			height = float64(boxSize.Y)
		}
	default:
		width, height = float64(length(size.Width, boxSize.X, unit)), float64(length(size.Height, boxSize.Y, unit))
	}
	if layer.RepeatX == css.Rounded && width > 0 {
		width = float64(boxSize.X) / max(math.Round(float64(boxSize.X)/width), 1)
	}
	if layer.RepeatY == css.Rounded && height > 0 {
		height = float64(boxSize.Y) / max(math.Round(float64(boxSize.Y)/height), 1)
	}
	if !(width >= 0.5 && height >= 0.5) || width > maxBackgroundLength || height > maxBackgroundLength {
		return image.Point{}
	}
	return image.Pt(max(int(math.Round(width)), 1), max(int(math.Round(height)), 1))
}

// maxBackgroundLength caps a background's lengths, in pixels either way: a book's absurd one
// stays far from overflowing an int on the reader, which has 32 bits.
const maxBackgroundLength = 1 << 20

// length returns a layer's length in pixels: a percentage of hundred, auto 0.
func length(value css.Length, hundred int, unit float64) int {
	pixels := value.Value * unit
	switch value.Unit {
	case "%":
		pixels = value.Value * float64(hundred) / 100
	case "auto":
		return 0
	}
	if math.IsNaN(pixels) {
		return 0
	}
	return int(math.Round(min(max(pixels, -maxBackgroundLength), maxBackgroundLength)))
}

// tilePlaces returns how many tiles of a layer go along an axis of a box extent pixels long,
// the step from one to the next, and where the first is, from the box's start: one at offset
// for no-repeat; for repeat and round, enough to cover the box with one at offset; for space,
// as many whole tiles as fit, the first and last at the box's edges and the rest spread between
// (one at offset if only one fits).
func tilePlaces(extent, tile, offset int, repeat css.Repeat) (count, step, first int) {
	switch repeat {
	case css.NoRepeat:
		return 1, tile, offset
	case css.Spaced:
		fitting := extent / tile
		if fitting <= 1 {
			return 1, tile, offset
		}
		return fitting, tile + (extent-fitting*tile)/(fitting-1), 0
	}
	first = offset % tile
	if first > 0 {
		first -= tile
	}
	return (extent - first + tile - 1) / tile, tile, first
}

// compositeTile draws tile with its top left at at, over what dst has, inside clip.
func compositeTile(dst *image.Gray, clip image.Rectangle, tile Tile, at image.Point) {
	area := tile.Gray.Bounds().Sub(tile.Gray.Bounds().Min).Add(at).Intersect(clip)
	if area.Empty() {
		return
	}
	from := tile.Gray.Bounds().Min.Sub(at)
	for row := area.Min.Y; row < area.Max.Y; row++ {
		target := dst.Pix[dst.PixOffset(area.Min.X, row):dst.PixOffset(area.Max.X, row)]
		start := tile.Gray.PixOffset(area.Min.X+from.X, row+from.Y)
		grays := tile.Gray.Pix[start : start+len(target)]
		if tile.Alpha == nil {
			copy(target, grays)
			continue
		}
		start = tile.Alpha.PixOffset(area.Min.X+from.X, row+from.Y)
		alphas := tile.Alpha.Pix[start : start+len(target)]
		for index, pixel := range target {
			target[index] = uint8(min(uint32(grays[index])+(uint32(pixel)*(255-uint32(alphas[index]))+127)/255, 255))
		}
	}
}

// gradientShade is a gradient laid over a tile of a given size, ready to give its colors row by
// row: a linear one along CSS's gradient line through the tile's middle (as long as the tile's
// corners are apart along it), a radial one along rays out of its centre to its ending shape.
type gradientShade struct {
	samples   []css.Color
	low       float64 // the position of samples[0]
	period    float64 // from samples[0] to the last sample: never 0
	repeating bool
	radial    bool
	// A linear gradient: the position at the tile's top left pixel's centre, and its steps to
	// the next pixel across and down.
	origin, stepX, stepY float64
	// A radial gradient: its centre and radii, in pixels.
	centreX, centreY, radiusX, radiusY float64
}

// newGradientShade lays gradient over a tile of size pixels, its lengths in the box's units
// multiplied by unit. It returns false when the gradient draws nothing: a line or a radius of no
// length.
func newGradientShade(gradient *css.Gradient, size image.Point, unit float64) (*gradientShade, bool) {
	shade := &gradientShade{radial: gradient.Radial}
	var lineLength float64
	if gradient.Radial {
		lineLength = shade.layRadial(gradient, size, unit)
	} else {
		lineLength = shade.layLinear(gradient, size)
	}
	if !(lineLength > 0) {
		return nil, false
	}
	stops := gradient.StopsAlong(lineLength / unit)
	// Stops all at one point repeat nothing: such a gradient is drawn as if not repeating.
	shade.repeating = gradient.Repeating && stops[len(stops)-1].Position > stops[0].Position
	var high float64
	shade.samples, shade.low, high = sampleGradient(stops, shade.repeating)
	shade.period = high - shade.low // past the stops' span, one that doesn't repeat spans 0 to 1
	return shade, true
}

// layLinear sets a linear gradient's positions over a tile of size and returns the length of
// its line, in pixels.
func (shade *gradientShade) layLinear(gradient *css.Gradient, size image.Point) float64 {
	width, height := float64(size.X), float64(size.Y)
	var directionX, directionY float64 // towards the gradient's end, y down
	if gradient.ToCorner {
		// Perpendicular to the diagonal between the two other corners.
		directionX, directionY = float64(gradient.CornerX)*height, float64(gradient.CornerY)*width
		length := math.Hypot(directionX, directionY)
		directionX, directionY = directionX/length, directionY/length
	} else {
		sine, cosine := math.Sincos(gradient.Angle * math.Pi / 180)
		directionX, directionY = sine, -cosine
	}
	lineLength := math.Abs(width*directionX) + math.Abs(height*directionY)
	if lineLength > 0 {
		shade.stepX, shade.stepY = directionX/lineLength, directionY/lineLength
		shade.origin = 0.5 + ((0.5-width/2)*directionX+(0.5-height/2)*directionY)/lineLength
	}
	return lineLength
}

// layRadial sets a radial gradient's centre and radii over a tile of size and returns the
// length of its ray, in pixels: the ending shape's horizontal radius.
func (shade *gradientShade) layRadial(gradient *css.Gradient, size image.Point, unit float64) float64 {
	width, height := float64(size.X), float64(size.Y)
	centreX, centreY := float64(length(gradient.CenterX, size.X, unit)), float64(length(gradient.CenterY, size.Y, unit))
	// The distances from the centre to the nearest and farthest sides, along each axis.
	nearX, nearY := min(math.Abs(centreX), math.Abs(width-centreX)), min(math.Abs(centreY), math.Abs(height-centreY))
	farX, farY := max(math.Abs(centreX), math.Abs(width-centreX)), max(math.Abs(centreY), math.Abs(height-centreY))
	var radiusX, radiusY float64
	switch gradient.Extent {
	case "closest-side":
		radiusX, radiusY = nearX, nearY
		if gradient.Circle {
			radiusX = min(nearX, nearY)
			radiusY = radiusX
		}
	case "farthest-side":
		radiusX, radiusY = farX, farY
		if gradient.Circle {
			radiusX = max(farX, farY)
			radiusY = radiusX
		}
	case "closest-corner", "farthest-corner":
		// Through the corner, with the aspect ratio the side's distances give.
		cornerX, cornerY := nearX, nearY
		if gradient.Extent == "farthest-corner" {
			cornerX, cornerY = farX, farY
		}
		radiusX, radiusY = cornerX*math.Sqrt2, cornerY*math.Sqrt2
		if gradient.Circle {
			radiusX = math.Hypot(cornerX, cornerY)
			radiusY = radiusX
		}
	default:
		radiusX, radiusY = float64(length(gradient.RadiusX, size.X, unit)), float64(length(gradient.RadiusY, size.Y, unit))
	}
	if radiusX <= 0 || radiusY <= 0 {
		return 0
	}
	shade.centreX, shade.centreY, shade.radiusX, shade.radiusY = centreX, centreY, radiusX, radiusY
	return radiusX
}

// row sets colors to the gradient's colors at pixels from, from+1… of the tile's row y.
func (shade *gradientShade) row(y, from int, colors []css.Color) {
	scale := float64(gradientSamples-1) / shade.period
	if shade.radial {
		offsetY := (float64(y) + 0.5 - shade.centreY) / shade.radiusY
		for index := range colors {
			colors[index] = shade.sample(math.Hypot((float64(from+index)+0.5-shade.centreX)/shade.radiusX, offsetY), scale)
		}
		return
	}
	at := shade.origin + float64(from)*shade.stepX + float64(y)*shade.stepY
	if shade.repeating {
		for index := range colors {
			colors[index] = shade.sample(at, scale)
			at += shade.stepX
		}
		return
	}
	// The common case, a gradient across a page: no repeat, a position that steps along.
	samples, low, period := shade.samples, shade.low, shade.period
	for index := range colors {
		colors[index] = samples[int(min(max(at-low, 0), period)*scale+0.5)]
		at += shade.stepX
	}
}

// sample returns the gradient's color at position at along its line or ray, scale being
// samples per unit of position.
func (shade *gradientShade) sample(at, scale float64) css.Color {
	if shade.repeating {
		at = shade.low + math.Mod(math.Mod(at-shade.low, shade.period)+shade.period, shade.period)
	}
	if math.IsNaN(at) {
		return shade.samples[0]
	}
	return shade.samples[int(min(max(at-shade.low, 0), shade.period)*scale+0.5)]
}

// gradientTile draws a gradient into a tile of size pixels, through shade.
func gradientTile(shade *gradientShade, size image.Point) Tile {
	tile := Tile{Gray: image.NewGray(image.Rectangle{Max: size}), Alpha: image.NewGray(image.Rectangle{Max: size})}
	colors := make([]css.Color, size.X)
	for y := range size.Y {
		shade.row(y, 0, colors)
		grays, alphas := tile.Gray.Pix[y*tile.Gray.Stride:][:size.X], tile.Alpha.Pix[y*tile.Alpha.Stride:][:size.X]
		for index, color := range colors {
			grays[index] = uint8((uint32(color.Gray)*uint32(color.Alpha) + 127) / 255)
			alphas[index] = color.Alpha
		}
	}
	return tile
}

// blendGradient draws a gradient through shade, its tile's top left at at, over what dst has
// inside clip: a layer of one tile needs no tile drawn first.
func blendGradient(dst *image.Gray, clip image.Rectangle, shade *gradientShade, at, size image.Point) {
	area := image.Rectangle{Min: at, Max: at.Add(size)}.Intersect(clip)
	if area.Empty() {
		return
	}
	colors := make([]css.Color, area.Dx())
	scale := float64(gradientSamples-1) / shade.period
	for row := area.Min.Y; row < area.Max.Y; row++ {
		pixels := dst.Pix[dst.PixOffset(area.Min.X, row):dst.PixOffset(area.Max.X, row)]
		if !shade.radial && !shade.repeating {
			// The common case, a gradient across a page, in one pass.
			samples, low, period, step := shade.samples, shade.low, shade.period, shade.stepX
			position := shade.origin + float64(area.Min.X-at.X)*step + float64(row-at.Y)*shade.stepY
			for index, pixel := range pixels {
				color := samples[int(min(max(position-low, 0), period)*scale+0.5)]
				alpha := uint32(color.Alpha)
				pixels[index] = uint8((uint32(color.Gray)*alpha + uint32(pixel)*(255-alpha) + 127) / 255)
				position += step
			}
			continue
		}
		shade.row(row-at.Y, area.Min.X-at.X, colors)
		for index, pixel := range pixels {
			alpha := uint32(colors[index].Alpha)
			pixels[index] = uint8((uint32(colors[index].Gray)*alpha + uint32(pixel)*(255-alpha) + 127) / 255)
		}
	}
}

// sampleGradient returns the colors of stops at gradientSamples points from the first stop's
// position to the last's (low and high), each interpolated between the stops around it in
// premultiplied alpha. A repeating gradient repeats that span; another keeps the end colors
// past it.
func sampleGradient(stops []css.GradientStop, repeating bool) (samples []css.Color, low, high float64) {
	low, high = stops[0].Position, stops[len(stops)-1].Position
	if !repeating {
		low, high = min(low, 0), max(high, 1)
	}
	samples = make([]css.Color, gradientSamples)
	next := 0 // the first stop past the sample
	for index := range samples {
		position := low + (high-low)*float64(index)/(gradientSamples-1)
		for next < len(stops) && stops[next].Position <= position {
			next++
		}
		switch {
		case next == 0:
			samples[index] = stops[0].Color
		case next == len(stops):
			samples[index] = stops[len(stops)-1].Color
		default:
			before, after := stops[next-1], stops[next]
			fraction := 0.0
			if span := after.Position - before.Position; span > 0 {
				fraction = (position - before.Position) / span
			}
			samples[index] = mix(before.Color, after.Color, fraction)
		}
	}
	return samples, low, high
}

// mix returns the color fraction of the way from first to second, in premultiplied alpha.
func mix(first, second css.Color, fraction float64) css.Color {
	firstAlpha, secondAlpha := float64(first.Alpha), float64(second.Alpha)
	alpha := firstAlpha + (secondAlpha-firstAlpha)*fraction
	if alpha == 0 {
		return css.Color{Gray: 255}
	}
	premultiplied := float64(first.Gray)*firstAlpha + (float64(second.Gray)*secondAlpha-float64(first.Gray)*firstAlpha)*fraction
	return css.Color{Gray: uint8(math.Round(min(max(premultiplied/alpha, 0), 255))), Alpha: uint8(math.Round(alpha))}
}
