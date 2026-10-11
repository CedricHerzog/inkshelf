package fixedlayout

import (
	"bytes"
	"image"
	"image/draw"
	"math"

	"github.com/CedricHerzog/inkshelf/internal/graphics/pageimage"
	"github.com/CedricHerzog/inkshelf/internal/web/css"
)

// gradientSamples is how many colors a gradient is sampled into along its line.
const gradientSamples = 1024

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

// fillGradient blends a linear gradient over box, the box in canvas's pixels: CSS's gradient
// line through the box's middle, as long as the box's corners are apart along it.
func fillGradient(canvas *image.Gray, box image.Rectangle, gradient *css.Gradient) {
	area := box.Intersect(canvas.Bounds())
	if area.Empty() || len(gradient.Stops) < 2 {
		return
	}
	width, height := float64(box.Dx()), float64(box.Dy())
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
	if lineLength == 0 {
		return
	}
	samples := sampleGradient(gradient.Stops)
	centreX, centreY := float64(box.Min.X)+width/2, float64(box.Min.Y)+height/2
	step := directionX / lineLength // along a row
	for row := area.Min.Y; row < area.Max.Y; row++ {
		// At the centre of the row's first pixel.
		position := 0.5 + ((float64(area.Min.X)+0.5-centreX)*directionX+(float64(row)+0.5-centreY)*directionY)/lineLength
		pixels := canvas.Pix[canvas.PixOffset(area.Min.X, row):canvas.PixOffset(area.Max.X, row)]
		for index, pixel := range pixels {
			sample := samples[int(min(max(position, 0), 1)*(gradientSamples-1)+0.5)]
			alpha := uint32(sample.Alpha)
			pixels[index] = uint8((uint32(sample.Gray)*alpha + uint32(pixel)*(255-alpha) + 127) / 255)
			position += step
		}
	}
}

// sampleGradient returns the colors of a gradient at gradientSamples points from 0 to 1: each
// interpolated between the stops around it, in premultiplied alpha.
func sampleGradient(stops []css.GradientStop) []css.Color {
	samples := make([]css.Color, gradientSamples)
	next := 0 // the first stop past the sample
	for index := range samples {
		position := float64(index) / (gradientSamples - 1)
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
	return samples
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

// drawTransparent draws a raster image with transparency stretched over area of canvas,
// composited over what is there. It returns false if the image is opaque or cannot be decoded,
// for drawImage to draw it.
func drawTransparent(canvas *image.Gray, data []byte, area image.Rectangle) bool {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width*config.Height > maxTransparentPixels {
		return false
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	if opaque, ok := decoded.(interface{ Opaque() bool }); !ok || opaque.Opaque() {
		return false
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
	gray, alpha = stretch(pageimage.Fit(gray, area.Dx(), area.Dy()), area.Size()), stretch(pageimage.Fit(alpha, area.Dx(), area.Dy()), area.Size())
	target := area.Intersect(canvas.Bounds())
	for row := target.Min.Y; row < target.Max.Y; row++ {
		for column := target.Min.X; column < target.Max.X; column++ {
			at := gray.PixOffset(column-area.Min.X, row-area.Min.Y)
			pixel := &canvas.Pix[canvas.PixOffset(column, row)]
			*pixel = uint8(min(uint32(gray.Pix[at])+(uint32(*pixel)*(255-uint32(alpha.Pix[at]))+127)/255, 255))
		}
	}
	return true
}
